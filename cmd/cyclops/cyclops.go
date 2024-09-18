package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/coreos/go-systemd/daemon"
	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/server"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/vpn"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/safewg/wgroot"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// This is purely for documentation of the cmd-line args
	nominalDefaultDB := "$HOME/cyclops/config.sqlite"

	//_, foo := os.ReadFile("/proc/self/auxv")
	//fmt.Printf("Read from /proc/self/auxv: %v\n", foo)

	// Certain parameters are scrubbed when dropping privileges, so we specify them as constants
	const pnVPN = "vpn"
	const pnUsername = "username"

	parser := argparse.NewParser("cyclops", "Camera security system")
	configFile := parser.String("c", "config", &argparse.Options{Help: "Configuration database file", Default: nominalDefaultDB})
	enableVPN := parser.Flag("", pnVPN, &argparse.Options{Help: "Enable automatic VPN", Default: false})
	username := parser.String("", pnUsername, &argparse.Options{Help: "After launching as root, change identity to this user (for dropping privileges of the main process)", Default: ""})
	hotReloadWWW := parser.Flag("", "hot", &argparse.Options{Help: "Hot reload www instead of embedding into binary", Default: false})
	ownIPStr := parser.String("", "ip", &argparse.Options{Help: "IP address of this machine (for network scanning)", Default: ""}) // eg for dev time, and server is running inside a NAT'ed VM such as WSL.
	privateKey := parser.String("", "privatekey", &argparse.Options{Help: "Change private key of system (e.g. for recreating a system using a prior identity)", Default: ""})
	disableHailo := parser.Flag("", "nohailo", &argparse.Options{Help: "Disable Hailo neural network accelerator support", Default: false})
	nnModelName := parser.String("", "nn", &argparse.Options{Help: "Specify the neural network for object detection", Default: "yolov8m"})
	elevated := parser.Flag("", "elevated", &argparse.Options{Help: "Maintain elevated permissions, instead of setuid(username)", Default: false})
	kernelWG := parser.Flag("", "kernelwg", &argparse.Options{Help: "(Internal) Run the kernel-mode wireguard interface", Default: false})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	if *kernelWG {
		// The main cyclops process has launched us, and our role is to control the wireguard interface.
		wgroot.Main()
		return
	}

	logger, err := logs.NewLog()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		// I don't know how this would happen in practice.. maybe some kind of system account.
		// But anyway, it's usual for this to be overridden with the --config option, so this
		// default is not very important.
		home = "/var/lib"
	}

	kernelWGSecret := ""

	if *enableVPN {
		// We are running as the cyclops server, and our first step is to launch the kernel-mode wireguard sub-process.
		if err, kernelWGSecret = wgroot.LaunchRootModeSubProcess(); err != nil {
			logger.Errorf("Error launching wireguard management sub-process: %v", err)
			os.Exit(1)
		}
		logger.Infof("Wireguard management sub-process launched")
	}

	// This auto SUDO privilege drop is too niche. I'm rather leave dropping privileges as an explicit option.
	//if *username == "" && os.Getenv("SUDO_USER") != "" {
	//	*username = os.Getenv("SUDO_USER")
	//}

	sslCertDirectory := filepath.Join(home, ".local", "share", "certmagic")

	// We must initialize ncnn before dropping privileges, because once we do that,
	// we're unable to read from /proc/self/auxv to detect CPU features.
	ncnn.Initialize()

	var privilegeLimiter *wgroot.PrivilegeLimiter

	// Check if we need to drop privileges to a different user ('username')
	if *username != "" && !wgroot.IsRunningAsUser(*username) {
		// Drop privileges
		privilegeLimiter, err = wgroot.NewPrivilegeLimiter(*username, wgroot.PrivilegeLimiterFlagSetEnvVars)
		if err != nil {
			logger.Errorf("Error creating privilege limiter: %v", err)
			os.Exit(1)
		}

		// Update home directory to lower privilege user
		home = privilegeLimiter.LoweredHome
		logger.Infof("Privileges dropped to user '%v'. Home directory is now '%v'", *username, home)

		// This is necessary for hailo, which tries to write logs into "./hailort.log" (or something like that)
		// If we are launched as root, then cwd is "/root"
		// However, if we are launched from "/home/developer/work/cyclops", then we want to leave that as-is.
		cwd, _ := os.Getwd()
		if !strings.HasPrefix(cwd, home) {
			os.Chdir(home)
		}

		sslCertDirectory = filepath.Join(home, ".local", "share", "certmagic")
	}

	actualDefaultConfigDB := filepath.Join(home, "cyclops", "config.sqlite")
	if *configFile == nominalDefaultDB {
		*configFile = actualDefaultConfigDB
	}

	var ownIP net.IP
	if *ownIPStr != "" {
		ownIP = net.ParseIP(*ownIPStr)
		if ownIP == nil {
			logger.Errorf("Invalid IP address: %v", *ownIPStr)
			os.Exit(1)
		}
	}

	// Dynamically load shared libraries (which are optional) that accelerate neural network inference.
	enableHailo := !*disableHailo
	nnload.LoadAccelerators(logger, enableHailo)

	// Right now Hailo dies if we use seteuid(), so we need to disable privilege drop when we have a Hailo device.
	if nnload.HaveHailo() {
		*elevated = true
	}

	if privilegeLimiter != nil && *elevated {
		logger.Infof("Elevating privileges back up")
		privilegeLimiter.Elevate()
		privilegeLimiter = nil
	}

	// Load/create the configuration database
	configDB, err := configdb.NewConfigDB(logger, *configFile, *privateKey)
	if err != nil {
		logger.Errorf("Failed to open config database: %v", err)
		os.Exit(1)
	}
	logger.Infof("Public key: %v (short hex %v)", configDB.PublicKey, hex.EncodeToString(configDB.PublicKey[:vpn.ShortPublicKeyLen]))

	var vpnClient *vpn.VPN
	vpnShutdown := make(chan bool)

	// Connect to our wireguard privileged process once, and never disconnect until we exit.
	// The privileged process only accepts its socket connection once, and then dies.
	// This is an intentional design decision to lower the odds of an attacker connecting to it.
	// We can only create the VPN client after we've loaded the private key out of the config database.
	// Note:
	// We must start the VPN before we start the HTTPS listener, because the HTTPS listener will try
	// to get a certificate from Let's Encrypt, and that will fail if the VPN is not running.
	enableSSL := false
	if kernelWGSecret != "" {
		// Setup VPN and register with proxy.
		logger.Infof("Starting VPN")
		vpnClient, err = server.StartVPN(logger, configDB.PrivateKey, kernelWGSecret)
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		vpnClient.RunRegisterLoop(vpnShutdown)
		configDB.VpnAllowedIPs = vpnClient.AllowedIPs
		enableSSL = true
	}

	// Run in a continuous loop, so that the server can restart itself
	// due to major configuration changes.
	// TODO: Get rid of this restart loop. Rather rely on systemd to restart us.
	// That's much cleaner, and simplifies privilege dropping, and also improves security.
	for {
		flags := 0
		if *hotReloadWWW {
			flags |= server.ServerFlagHotReloadWWW
		}
		srv, err := server.NewServer(logger, configDB, flags, *nnModelName)
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		if ownIP != nil {
			srv.OwnIP = ownIP
		}
		srv.ListenForKillSignals()

		//logger.Warnf("Sleeping for 1 hour")
		//time.Sleep(time.Hour)

		// Tell systemd that we're alive.
		// We might also want to implement a liveness ping.
		// See this article for more details: https://vincent.bernat.ch/en/blog/2017-systemd-golang
		daemon.SdNotify(false, daemon.SdNotifyReady)

		if enableSSL {
			err = srv.ListenHTTPS(sslCertDirectory, privilegeLimiter)
			if err != nil {
				logger.Infof("ListenHTTPS returned: %v", err)
				if !srv.MustRestart {
					break
				}
			}
		} else {
			// SYNC-SERVER-PORT
			err = srv.ListenHTTP(":8080")
			if err != nil {
				logger.Infof("ListenHTTP returned: %v", err)
				if !srv.MustRestart {
					break
				}
			}
		}

		err = <-srv.ShutdownComplete
		//fmt.Printf("Server sent ShutdownComplete: %v", err)
		if !srv.MustRestart {
			break
		}
	}

	close(vpnShutdown)
}

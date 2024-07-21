package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
	"github.com/coreos/go-systemd/daemon"
	"github.com/cyclopcam/cyclops/pkg/kernelwg"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/server"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// This is purely for documentation of the cmd-line args
	nominalDefaultDB := "~home/cyclops/config.sqlite"

	parser := argparse.NewParser("cyclops", "A teachable camera security system")
	configFile := parser.String("c", "config", &argparse.Options{Help: "Configuration database file", Default: nominalDefaultDB})
	disableVPN := parser.Flag("", "novpn", &argparse.Options{Help: "Disable automatic VPN", Default: false})
	hotReloadWWW := parser.Flag("", "hot", &argparse.Options{Help: "Hot reload www instead of embedding into binary", Default: false})
	ownIPStr := parser.String("", "ip", &argparse.Options{Help: "IP address of this machine (for network scanning)", Default: ""}) // eg for dev time, and server is running inside a NAT'ed VM such as WSL.
	privateKey := parser.String("", "privatekey", &argparse.Options{Help: "Change private key of system (e.g. for recreating a system using a prior identity)", Default: ""})
	kernelWG := parser.Flag("", "kernelwg", &argparse.Options{Help: "Run the kernel-mode wireguard interface", Default: false})
	username := parser.String("", "username", &argparse.Options{Help: "After launching as root, change identity to this user (for dropping privileges of the main process)", Default: ""})
	disableHailo := parser.Flag("", "nohailo", &argparse.Options{Help: "Disable Hailo neural network accelerator support", Default: false})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	if *kernelWG {
		// The main cyclops process has launched us, and our role is to control the wireguard interface.
		kernelwg.Main()
		return
	}

	logger, err := log.NewLog()
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

	if !*disableVPN {
		// We are running as the cyclops server, and our first step is to launch the kernel-mode wireguard sub-process.
		if err, kernelWGSecret = kernelwg.LaunchRootModeSubProcess(); err != nil {
			logger.Errorf("Error launching kernel wireguard sub-process: %v", err)
			logger.Errorf("You can use --novpn to disable the automatic VPN system.")
			os.Exit(1)
		}
		if *username == "" && os.Getenv("SUDO_USER") != "" {
			*username = os.Getenv("SUDO_USER")
		}
		if err, home = kernelwg.DropPrivileges(*username); err != nil {
			logger.Errorf("Error dropping privileges to username '%v': %v", *username, err)
			logger.Errorf("You can use --novpn to disable the automatic VPN system.")
			os.Exit(1)
		}
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

	// Here we dynamically optional load shared libraries that accelerate neural network
	// inference. So we only do this once, during process startup.
	enableHailo := !*disableHailo
	nnload.LoadAccelerators(logger, enableHailo)

	// Run in a continuous loop, so that the server can restart itself
	// due to major configuration changes.
	for {
		flags := 0
		if *disableVPN {
			flags |= server.ServerFlagDisableVPN
		}
		if *hotReloadWWW {
			flags |= server.ServerFlagHotReloadWWW
		}
		srv, err := server.NewServer(logger, *configFile, flags, *privateKey, kernelWGSecret)
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		if ownIP != nil {
			srv.OwnIP = ownIP
		}
		srv.ListenForInterruptSignal()

		// Tell systemd that we're alive.
		// We might also want to implement a liveness ping.
		// See this article for more details: https://vincent.bernat.ch/en/blog/2017-systemd-golang
		daemon.SdNotify(false, daemon.SdNotifyReady)

		// SYNC-SERVER-PORT
		err = srv.ListenHTTP(":8080")
		if err != nil {
			logger.Infof("ListenHTTP returned: %v\n", err)
		}
		err = <-srv.ShutdownComplete
		//fmt.Printf("Server sent ShutdownComplete: %v\n", err)
		if !srv.MustRestart {
			break
		}
	}
}

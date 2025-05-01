package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/coreos/go-systemd/daemon"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/pkg/osutil"
	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/pkg/requests"
	"github.com/cyclopcam/cyclops/server"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/vpn"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/safewg/wgroot"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func isWSL2() bool {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	release := strings.ToLower(string(data))
	return strings.Contains(release, "microsoft") && strings.Contains(release, "wsl2")
}

// The exit values look nominally wrong, since usually a zero exit code means we
// exited cleanly. The reason we flip the codes around, is so that we can use
// systemd restart=on-failure. on-failure will capture all exit conditions except
// for the one where we explicitly tell it "don't try". The only way to achieve
// this is by flipping the meaning around of zero vs non-zero exit codes.

// Exit and tell systemd that it should not attempt a restart
func ExitNoRestart() {
	fmt.Printf("Exiting and telling systemd not to restart us (exit code 0)\n")
	os.Exit(0)
}

// Exit and tell systemd that it should restart us.
func ExitAndRestart() {
	fmt.Printf("Exiting and telling systemd to restart us (exit code 1)\n")
	os.Exit(1)
}

func main() {
	// These are purely for documentation of the cmd-line args
	nominalDefaultDB := "$HOME/cyclops/config.sqlite"
	nominalModelsDir := "$HOME/cyclops/models"

	//_, foo := os.ReadFile("/proc/self/auxv")
	//fmt.Printf("Read from /proc/self/auxv: %v\n", foo)

	defaultVpnNetwork := "IPv6"
	if isWSL2() {
		// As of 2024-10-01, Wireguard IPv6 doesn't work on WSL2 in NAT mode.
		// Apparently it might work in Bridge mode, but I haven't tried that.
		defaultVpnNetwork = "IPv4"
	}

	// SYNC-SERVER-PORT
	httpListenPort := 8080

	// Certain parameters are scrubbed from the child processes's args when dropping privileges, so we specify them as constants
	const pnVPN = "vpn"
	const pnUsername = "username"

	parser := argparse.NewParser("cyclops", "Camera security system")
	configFile := parser.String("c", "config", &argparse.Options{Help: "Configuration database file", Default: nominalDefaultDB})
	enableVPN := parser.Flag("", pnVPN, &argparse.Options{Help: "Enable automatic VPN", Default: false})
	vpnNetwork := parser.String("", "vpn-network", &argparse.Options{Help: "Either IPv4 or IPv6 for VPN (IPv4 is necessary on WSL2 in NAT mode)", Default: defaultVpnNetwork})
	username := parser.String("", pnUsername, &argparse.Options{Help: "After launching as root, change identity to this user (for dropping privileges of the main process)", Default: ""})
	hotReloadWWW := parser.Flag("", "hot", &argparse.Options{Help: "Hot reload www instead of embedding into binary", Default: false})
	ownIPStr := parser.String("", "ip", &argparse.Options{Help: "IP address of this machine (for network scanning)", Default: ""}) // eg for dev time, and server is running inside a NAT'ed VM such as WSL.
	privateKey := parser.String("", "privatekey", &argparse.Options{Help: "Change private key of system (e.g. for recreating a system using a prior identity)", Default: ""})
	disableHailo := parser.Flag("", "nohailo", &argparse.Options{Help: "Disable Hailo neural network accelerator support", Default: false})
	modelsDir := parser.String("", "models", &argparse.Options{Help: "Neural network models directory", Default: nominalModelsDir})
	nnModelName := parser.String("", "nn", &argparse.Options{Help: "Specify the neural network for object detection", Default: ""})
	elevated := parser.Flag("", "elevated", &argparse.Options{Help: "Maintain elevated permissions, instead of setuid(username)", Default: false})
	kernelWG := parser.Flag("", "kernelwg", &argparse.Options{Help: "(Internal) Run the kernel-mode wireguard interface", Default: false})
	resetUser := parser.Flag("", "reset-user", &argparse.Options{Help: "Interactively ensure an admin user exists (to recover a system that you're locked out of)", Default: false})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		ExitNoRestart()
	}

	if *vpnNetwork != "IPv4" && *vpnNetwork != "IPv6" {
		fmt.Printf("Invalid VPN network: '%v'. Valid values are IPv4, IPv6\n", *vpnNetwork)
		ExitNoRestart()
	}

	if *kernelWG {
		// The main cyclops process has launched us, and our role is to control the wireguard interface.
		wgroot.Main()
		return
	}

	logger, err := logs.NewLog()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		ExitNoRestart()
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
			ExitNoRestart()
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
		userExists, err := osutil.UserExists(*username)
		if err != nil {
			logger.Errorf("Error checking if user '%v' exists: %v", *username, err)
			ExitNoRestart()
		}
		if !userExists {
			logger.Infof("Creating user '%v'", *username)
			if err := osutil.AddUser(*username, "", "/var/lib/cyclops"); err != nil {
				logger.Errorf("Error creating user '%v': %v", *username, err)
				ExitNoRestart()
			}
		}

		// Drop privileges
		privilegeLimiter, err = wgroot.NewPrivilegeLimiter(*username, wgroot.PrivilegeLimiterFlagSetEnvVars)
		if err != nil {
			logger.Errorf("Error creating privilege limiter: %v", err)
			ExitNoRestart()
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

	// For the case where cyclops is being run as a user with a regular home directory (eg /home/mike),
	// then we want our files stored in /home/mike/cyclops.
	// But if we're running as a system user (eg user "cyclops"), and our home directory is /var/lib/cyclops,
	// then we want our files stored in /var/lib/cyclops.
	// Without this extra logic, we end up with out files stored inside "/var/lib/cyclops/cyclops".
	defaultRoot := filepath.Join(home, "cyclops")
	if strings.HasPrefix(home, "/var") {
		defaultRoot = home
	}

	actualDefaultConfigDB := filepath.Join(defaultRoot, "config.sqlite")
	if *configFile == nominalDefaultDB {
		*configFile = actualDefaultConfigDB
	}

	actualDefaultModelsDir := filepath.Join(defaultRoot, "models")
	if *modelsDir == nominalModelsDir {
		*modelsDir = actualDefaultModelsDir
	}

	var ownIP net.IP
	if *ownIPStr != "" {
		ownIP = net.ParseIP(*ownIPStr)
		if ownIP == nil {
			logger.Errorf("Invalid IP address: %v", *ownIPStr)
			ExitNoRestart()
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
		ExitNoRestart()
	}
	logger.Infof("Public key: %v (short hex %v)", configDB.PublicKey, hex.EncodeToString(configDB.PublicKey[:vpn.ShortPublicKeyLen]))

	if *resetUser {
		userReset(configDB)
		return
	}

	if *enableVPN {
		logger.Infof("VPN network %v", *vpnNetwork)
	}

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
		forceIPv4 := *vpnNetwork == "IPv4"
		vpnClient, err = server.StartVPN(logger, configDB.PrivateKey, kernelWGSecret, forceIPv4)
		if err != nil {
			logger.Errorf("%v", err)
			httpError := &requests.ResponseError{}
			if errors.As(err, &httpError) && httpError.StatusCode == http.StatusForbidden {
				// This error means the proxy is refusing to allow us to register, because we're still waiting
				// for an authorized user to login to this server. Authorized means any active user on accounts.cyclopcam.org.
				logger.Errorf("Not authorized to use VPN")
				// We can't listen on 443, but we still want to listen on 80
				// SYNC-SERVER-PORT
				httpListenPort = 80
			} else {
				// Add a pause, otherwise we very quickly exhaust our restart timer.
				logger.Infof("Waiting 5 seconds before restarting")
				time.Sleep(5 * time.Second)
				ExitAndRestart()
			}
		} else {
			vpnClient.RunRegisterLoop(vpnShutdown)
			configDB.VpnAllowedIP = vpnClient.AllowedIP
			enableSSL = true
		}
	}

	////////////////////////////////////////////////////////////////////////////////////////
	// This used to be the start of a continuous run-restart loop, but now we rather rely
	// on systemd to restart us if necessary. This change was brought about to simplify
	// our privilege dropping behaviour. If we need to be able to restart indefinitely,
	// then it means we need to keep our elevated privileges. All the pain around reading
	// from /proc/self/auxv, listening on low ports, hailo, etc, brings this about.
	flags := 0
	if *hotReloadWWW {
		flags |= server.ServerFlagHotReloadWWW
	}
	srv, err := server.NewServer(logger, configDB, flags, *modelsDir, *nnModelName)
	if err != nil {
		logger.Errorf("%v", err)
		ExitNoRestart()
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
		}
	} else {
		err = srv.ListenHTTP(httpListenPort, privilegeLimiter)
		if err != nil {
			logger.Infof("ListenHTTP returned: %v", err)
		}
	}
	if !errors.Is(err, http.ErrServerClosed) {
		// typical cause would be that Listen() failed
		if !gen.IsChannelClosed(srv.ShutdownStarted) {
			srv.Shutdown(false)
		}
	}

	err = <-srv.ShutdownComplete
	//fmt.Printf("Server sent ShutdownComplete: %v", err)
	// This was the end of the original run-restart loop, mentioned above.
	////////////////////////////////////////////////////////////////////////////////////////

	close(vpnShutdown)

	if srv.MustRestart {
		ExitAndRestart()
	} else {
		ExitNoRestart()
	}
}

// This can be used to create an admin user in a database where you've somehow lost that permission
func userReset(configDB *configdb.ConfigDB) {
	fmt.Printf("\nThis will create a user with a name of your choice.\n")
	fmt.Printf("It's OK if that user already exists.\n")
	fmt.Printf("We'll make sure that the user has admin permissions.\n")
	fmt.Printf("You can choose the password for the user.\n\n")
	fmt.Printf("Enter the username: ")
	var username string
	fmt.Scanln(&username)
	fmt.Printf("Enter the password: ")
	var password string
	fmt.Scanln(&password)

	user := configdb.User{}
	configDB.DB.Where("username_normalized = ?", configdb.NormalizeUsername(username)).First(&user)
	if user.ID == 0 {
		fmt.Printf("Creating new user '%v'\n", username)
		user.Username = username
		user.UsernameNormalized = configdb.NormalizeUsername(username)
		user.Name = username
		user.Permissions = string(configdb.UserPermissionAdmin)
		user.CreatedAt = dbh.MakeIntTime(time.Now())
		user.Password = pwdhash.HashPasswordBase64(password)
		if err := configDB.DB.Create(&user).Error; err != nil {
			fmt.Printf("Error creating user: %v\n", err)
			return
		}
	} else {
		fmt.Printf("Updating existing user '%v'\n", username)
		if !strings.Contains(user.Permissions, string(configdb.UserPermissionAdmin)) {
			user.Permissions += string(configdb.UserPermissionAdmin)
		}
		user.Password = pwdhash.HashPasswordBase64(password)
		if err := configDB.DB.Save(&user).Error; err != nil {
			fmt.Printf("Error saving user: %v\n", err)
			return
		}
	}
	fmt.Printf("User '%v' created/updated successfully\n", username)
}

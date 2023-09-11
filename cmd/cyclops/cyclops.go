package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cyclops/pkg/kernelwg"
	"github.com/bmharper/cyclops/server"
	"github.com/coreos/go-systemd/daemon"
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
			fmt.Printf("Error launching kernel wireguard sub-process: %v\n", err)
			fmt.Printf("You can use --novpn to disable the automatic VPN system.\n")
			os.Exit(1)
		}
		if *username == "" && os.Getenv("SUDO_USER") != "" {
			*username = os.Getenv("SUDO_USER")
		}
		if err, home = kernelwg.DropPrivileges(*username); err != nil {
			fmt.Printf("Error dropping privileges to username '%v': %v\n", *username, err)
			fmt.Printf("You can use --novpn to disable the automatic VPN system.\n")
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
			fmt.Printf("Invalid IP address: %v\n", *ownIPStr)
			os.Exit(1)
		}
	}

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
		srv, err := server.NewServer(*configFile, flags, *privateKey, kernelWGSecret)
		if err != nil {
			fmt.Printf("%v\n", err)
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

		srv.RunBackgroundRecorderLoop()

		// SYNC-SERVER-PORT
		err = srv.ListenHTTP(":8080")
		if err != nil {
			fmt.Printf("ListenHTTP returned: %v\n", err)
		}
		err = <-srv.ShutdownComplete
		//fmt.Printf("Server sent ShutdownComplete: %v\n", err)
		if !srv.MustRestart {
			break
		}
	}
}

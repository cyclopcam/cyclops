package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cyclops/server"
	"github.com/coreos/go-systemd/daemon"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	home, _ := os.UserHomeDir()
	if home == "" {
		// I don't know how this would happen in practice.. maybe some kind of system account.
		// But anyway, it's usual for this to be overridden with the --config option, so this
		// default is not very important.
		home = "/var/lib"
	}

	parser := argparse.NewParser("cyclops", "A teachable camera security system")
	configFile := parser.String("c", "config", &argparse.Options{Help: "Configuration database file", Default: filepath.Join(home, "cyclops", "config.sqlite")})
	disableVPN := parser.Flag("", "novpn", &argparse.Options{Help: "Disable VPN", Default: false})
	hotReloadWWW := parser.Flag("", "hot", &argparse.Options{Help: "Hot reload www instead of embedding into binary", Default: false})
	ownIPStr := parser.String("", "ip", &argparse.Options{Help: "IP address of this machine (for network scanning)", Default: ""}) // eg for dev time, and server is running inside a NAT'ed VM such as WSL.
	privateKey := parser.String("", "privatekey", &argparse.Options{Help: "Change private key of system (e.g. for recreating a system while maintaining a prior identity)", Default: ""})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
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
		srv, err := server.NewServer(*configFile, flags, *privateKey)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		if ownIP != nil {
			srv.OwnIP = ownIP
		}
		srv.ListenForInterruptSignal()

		// Tell systemd that we're alive
		daemon.SdNotify(false, daemon.SdNotifyReady)

		check(srv.StartAllCameras())

		srv.RunBackgroundRecorderLoop()

		// SYNC-SERVER-PORT
		err = srv.ListenHTTP(":8080")
		fmt.Printf("Server exited: %v\n", err)
		err = <-srv.ShutdownComplete
		//fmt.Printf("Server sent ShutdownComplete: %v\n", err)
		if !srv.MustRestart {
			break
		}
	}
}

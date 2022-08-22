package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cyclops/server"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	home, _ := os.UserHomeDir()
	if home == "" {
		// Don't know how this would happen in practice.. maybe some kind of system account
		home = "/var/lib"
	}

	parser := argparse.NewParser("cyclops", "A teachable camera security system")
	config := parser.String("c", "config", &argparse.Options{Help: "Configuration database file", Default: filepath.Join(home, "cyclops", "config.sqlite")})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	// Run in a continuous loop, so that the server can restart itself
	// due to major configuration changes.
	for {
		srv, err := server.NewServer(*config)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		srv.ListenForInterruptSignal()
		check(srv.StartAllCameras())

		srv.RunBackgroundRecorderLoop()

		err = srv.ListenHTTP(":8080")
		fmt.Printf("Server exited: %v\n", err)
		err = <-srv.ShutdownComplete
		//fmt.Printf("Server sent ShutdownComplete: %v\n", err)
		if !srv.MustRestart {
			break
		}
	}
}

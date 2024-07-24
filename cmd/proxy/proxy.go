package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/kernelwg"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/proxy"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	parser := argparse.NewParser("cyclopsproxy", "Tunnel into cyclops systems")
	kernelWG := parser.Flag("", "kernelwg", &argparse.Options{Help: "Run the kernel-mode wireguard interface", Default: false})
	username := parser.String("", "username", &argparse.Options{Help: "After launching as root, change identity to this user (for dropping privileges of the main process)", Default: ""})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	logger, err := log.NewLog()
	check(err)
	logger = log.NewPrefixLogger(logger, "proxy")

	pgHost := os.Getenv("CYCLOPS_POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "127.0.0.1"
	}

	pgPassword := os.Getenv("CYCLOPS_POSTGRES_PASSWORD")
	if pgPassword == "" {
		// dev time (for initial DB creation, this must match the POSTGRES_PASSWORD in scripts/proxy/docker-compose.yml)
		pgPassword = "lol"
	}

	adminPassword := os.Getenv("CYCLOPS_ADMIN_PASSWORD")

	if *kernelWG {
		// The main proxy process has launched us, and our role is to control the wireguard interface.
		// We run with elevated permissions.
		kernelwg.Main()
		return
	}

	// This is the secret that we use to authenticate ourselves to the kernel-mode wireguard interface.
	kernelWGSecret := ""

	// We are running as the HTTPS proxy server, and our first step is to launch the kernel-mode wireguard sub-process.
	if err, kernelWGSecret = kernelwg.LaunchRootModeSubProcess(); err != nil {
		fmt.Printf("Error launching root mode wireguard sub-process: %v\n", err)
		os.Exit(1)
	}
	if *username == "" && os.Getenv("SUDO_USER") != "" {
		*username = os.Getenv("SUDO_USER")
	}
	if err = kernelwg.DropPrivileges(*username); err != nil {
		fmt.Printf("Error dropping privileges to username '%v': %v\n", *username, err)
		os.Exit(1)
	}

	p := proxy.NewProxy()

	cfg := proxy.ProxyConfig{
		Log: logger,
		DB: dbh.DBConfig{
			Driver:   dbh.DriverPostgres,
			Host:     pgHost,
			Database: "proxy",
			Username: "postgres",
			Password: pgPassword,
		},
		KernelWGSecret: kernelWGSecret,
		AdminPassword:  adminPassword,
	}

	check(p.Start(cfg))
}

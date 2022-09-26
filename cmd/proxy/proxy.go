package main

import (
	"os"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/proxy"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
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

	kernelwgHost := os.Getenv("CYCLOPS_KERNELWG_HOST")
	if kernelwgHost == "" {
		kernelwgHost = "127.0.0.1"
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
		KernelWGHost: kernelwgHost,
	}

	check(p.Start(cfg))
}

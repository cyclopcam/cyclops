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

	pgPassword := os.Getenv("CYCLOPS_PGPASSWORD")
	if pgPassword == "" {
		// dev time (for initial DB creation, this must match the POSTGRES_PASSWORD in scripts/proxy/docker-compose.yml)
		pgPassword = "lol"
	}

	p := proxy.NewProxy()

	cfg := proxy.ProxyConfig{
		Log: logger,
		DB: dbh.DBConfig{
			Driver:   dbh.DriverPostgres,
			Host:     "localhost",
			Database: "proxy",
			Username: "postgres",
			Password: pgPassword,
		},
	}

	check(p.Start(cfg))
}

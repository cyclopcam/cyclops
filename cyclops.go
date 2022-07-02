package main

import (
	"github.com/bmharper/cyclops/server"
	"github.com/bmharper/cyclops/server/config"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	cfg, err := config.LoadConfig("")
	check(err)
	srv := server.NewServer()
	check(srv.LoadConfig(*cfg))
	check(srv.StartAll())

	srv.SetupHTTP(":8080")
}

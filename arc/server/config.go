package server

import "github.com/bmharper/cyclops/pkg/dbh"

type Config struct {
	DB dbh.DBConfig `json:"db"`
}

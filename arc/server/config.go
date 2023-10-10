package server

import "github.com/cyclopcam/cyclops/pkg/dbh"

type Config struct {
	DB               dbh.DBConfig `json:"db"`
	StoragePath      string       `json:"storagePath"`
	StorageCachePath string       `json:"storageCachePath"`
}

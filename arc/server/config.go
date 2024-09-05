package server

import "github.com/cyclopcam/dbh"

type Config struct {
	DB           dbh.DBConfig  `json:"db"`
	VideoStorage StorageConfig `json:"videoStorage"`
	VideoCache   string        `json:"videoCache"` // Path to the cache directory
}

// One of the storage options must be configured (i.e. either 'filesystem' or 'gcs')
type StorageConfig struct {
	Filesystem *StorageConfigFS  `json:"filesystem"`
	GCS        *StorageConfigGCS `json:"gcs"`
}

type StorageConfigFS struct {
	Root string `json:"root"` // Path to the root of the filesystem
}

type StorageConfigGCS struct {
	Bucket string `json:"bucket"` // Name of the GCS bucket
	Public bool   `json:"public"` // Whether the bucket is public. This allows us to give clients direct URLs into GCS, instead of passing the data through our service
}

package proxy

import (
	"github.com/BurntSushi/migration"
	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/log"
)

func Migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE server(
			id BIGSERIAL PRIMARY KEY,
			public_key BYTEA NOT NULL,
			vpn_ip TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			last_register_at TIMESTAMP NOT NULL,
			last_traffic_at TIMESTAMP
		);

		CREATE UNIQUE INDEX idx_server_public_key ON server(public_key);

		CREATE TABLE ip_free_pool(
			vpn_ip TEXT NOT NULL
		);

	`))

	return migs
}

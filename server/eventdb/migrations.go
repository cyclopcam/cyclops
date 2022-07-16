package eventdb

import (
	"github.com/BurntSushi/migration"
	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/log"
)

func Migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE recording(
			id INTEGER PRIMARY KEY,
			random_id TEXT NOT NULL,
			start_time INT NOT NULL,
			bar_time INT,
			format TEXT NOT NULL,
			labels BLOB
		);

		`))

	return migs
}

/*
	CREATE TABLE recording(
		id BIGSERIAL PRIMARY KEY,
		random_id TEXT NOT NULL,
		start_time BIGINT NOT NULL,
		foo_time BIGINT,
		bar_time BIGINT,
		format TEXT NOT NULL,
		labels JSONB,
		labels2 JSONB
	);
*/

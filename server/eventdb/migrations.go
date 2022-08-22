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
			record_type TEXT NOT NULL,
			origin TEXT NOT NULL,
			parent_id INT,
			format_hd TEXT,
			format_ld TEXT,
			labels BLOB,
			ontology_id INT,
			bytes INT,
			dimensions_hd TEXT,
			dimensions_ld TEXT,
			camera_id INT
		);

		CREATE TABLE ontology(
			id INTEGER PRIMARY KEY,
			created_at INT NOT NULL,
			definition BLOB NOT NULL
		);

		`))

	return migs
}

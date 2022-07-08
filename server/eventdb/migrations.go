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
			format TEXT NOT NULL
		);

		`))

	return migs
}

package videodb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
)

func Migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	// We don't use "WITHOUT ROWID" on event_tile, because our rows will tend to be large,
	// and if you read the Sqlite docs (https://www.sqlite.org/withoutrowid.html), you'll see
	// that WITHOUT ROWID tables store all their data in a classic BTree, so the large blobs
	// will make seeking slow.
	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE event(
			id INTEGER PRIMARY KEY,
			camera INT NOT NULL,
			time INT NOT NULL,
			duration INT NOT NULL,
			detections TEXT
		);

		CREATE INDEX idx_event_camera_time ON event (camera, time);

		CREATE TABLE strings(
			id INTEGER PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE UNIQUE INDEX idx_strings_value ON strings(value);

		CREATE TABLE event_tile(
			camera INT NOT NULL,
			level INT NOT NULL,
			start INT NOT NULL,
			tile BLOB,
			PRIMARY KEY (camera, level, start)
		);

		CREATE TABLE kv(key TEXT PRIMARY KEY, value TEXT);
	`))

	return migs
}

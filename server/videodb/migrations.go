package videodb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
)

func Migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE event(
			id INTEGER PRIMARY KEY,
			camera TEXT NOT NULL,
			time INT NOT NULL,
			duration INT NOT NULL,
			detections TEXT
		);

		CREATE INDEX idx_event_camera_time ON event (camera, time);

		CREATE TABLE event_summary(
			camera TEXT NOT NULL,
			time INT NOT NULL,
			classes TEXT NOT NULL,
			PRIMARY KEY (camera, time)
		) WITHOUT ROWID;

	`))

	return migs
}

package eventdb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
)

func Migrations(log logs.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	/*
		CREATE TABLE arm(
			id INTEGER PRIMARY KEY,
			time INT NOT NULL,
			user_id INT NOT NULL,
			state TEXT NOT NULL
		);

		CREATE INDEX idx_arm_state ON arm(time);

	*/
	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE event(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			time INT NOT NULL,
			event_type TEXT NOT NULL,
			detail TEXT NOT NULL,
			in_cloud INT NOT NULL
		);

		CREATE INDEX idx_event_time ON event(time);
		CREATE INDEX idx_event_in_cloud ON event(in_cloud);
	`))

	return migs
}

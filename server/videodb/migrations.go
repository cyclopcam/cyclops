package videodb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/cyclops/pkg/log"
)

func Migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	//idx := 0

	//migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
	//	`
	//	CREATE TABLE video_segment(
	//		id INTEGER PRIMARY KEY,
	//		camera_id INT NOT NULL,
	//		start_time INT NOT NULL,
	//		end_time INT NOT NULL
	//	);
	//`))

	return migs
}

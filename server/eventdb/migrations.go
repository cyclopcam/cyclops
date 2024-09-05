package eventdb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
)

func Migrations(log logs.Log) []migration.Migrator {
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

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE INDEX idx_recording_parent_id ON recording(parent_id);
		CREATE INDEX idx_recording_camera_id ON recording(camera_id);
	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE INDEX idx_recording_start_time ON recording(start_time);
	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		ALTER TABLE ontology ADD COLUMN modified_at INT NOT NULL;
		CREATE INDEX idx_ontology_modified_at ON ontology(modified_at);
	`))

	// Ontologies are immutable, so there is no point in having a modified_at field
	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		DROP INDEX idx_ontology_modified_at;
		ALTER TABLE ontology DROP COLUMN modified_at;
	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		ALTER TABLE recording ADD COLUMN use_for_training INT;
	`))

	return migs
}

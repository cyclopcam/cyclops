package configdb

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
		CREATE TABLE camera(
			id INTEGER PRIMARY KEY,
			model TEXT NOT NULL,
			name TEXT NOT NULL,
			host TEXT NOT NULL,
			port INT,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			high_res_url_suffix TEXT,
			low_res_url_suffix TEXT
		);

		CREATE TABLE variable(
			key TEXT PRIMARY KEY,
			value TEXT
		);

		CREATE TABLE user(
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL,
			username_normalized TEXT NOT NULL,
			permissions TEXT NOT NULL,
			name TEXT,
			password BLOB
		);
		CREATE UNIQUE INDEX idx_user_username_normalized ON user (username_normalized);

		CREATE TABLE session(
			key BLOB NOT NULL,
			user_id INT NOT NULL,
			expires_at INT
		);

	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE record_instruction(
			id INTEGER PRIMARY KEY,
			start_at INT NOT NULL,
			finish_at INT NOT NULL
		);

	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		DELETE FROM session;
		ALTER TABLE session ADD COLUMN created_at INT NOT NULL;
	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE key(name TEXT PRIMARY KEY, value BLOB NOT NULL);
	`))

	return migs
}

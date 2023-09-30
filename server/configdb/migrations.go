package configdb

import (
	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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

	// This was really just a POC migration.. nobody was using the system except for me.
	migs = append(migs, dbh.MakeMigrationFromFunc(log, &idx, func(tx migration.LimitedTx) error {
		mainKeyBin := []byte{}
		tx.QueryRow("SELECT value FROM key WHERE name = ?", KeyMain).Scan(&mainKeyBin)
		tx.Exec("DROP TABLE key")
		tx.Exec("CREATE TABLE key(name TEXT PRIMARY KEY, value TEXT NOT NULL)")
		if len(mainKeyBin) == 32 {
			k := wgtypes.Key{}
			copy(k[:], mainKeyBin)
			_, err := tx.Exec("INSERT INTO key(name, value) VALUES (?, ?)", KeyMain, k.String())
			return err
		}
		return nil
	}))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		ALTER TABLE record_instruction RENAME TO old;
		ALTER TABLE old ADD COLUMN resolution TEXT NOT NULL DEFAULT 'LD';
		CREATE TABLE record_instruction(
			id INTEGER PRIMARY KEY,
			start_at INT NOT NULL,
			finish_at INT NOT NULL,
			resolution TEXT NOT NULL
		);
		INSERT INTO record_instruction SELECT id, start_at, finish_at, resolution FROM old;
		DROP TABLE old;
	`))

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		ALTER TABLE camera ADD COLUMN created_at INT NOT NULL DEFAULT 0;
		ALTER TABLE camera ADD COLUMN updated_at INT NOT NULL DEFAULT 0;
		UPDATE camera SET created_at = strftime('%s') * 1000, updated_at = strftime('%s') * 1000;
	`))

	return migs
}

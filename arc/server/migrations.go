package server

import (
	"time"

	"github.com/BurntSushi/migration"
	"github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/pkg/rando"
	"gorm.io/gorm"
)

func migrations(log log.Log) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0

	migs = append(migs, dbh.MakeMigrationFromSQL(log, &idx,
		`
		CREATE TABLE auth_user(id BIGSERIAL PRIMARY KEY, email TEXT, password TEXT, created_at TIMESTAMP);

		CREATE TABLE auth_session(key TEXT PRIMARY KEY, auth_user_id BIGINT, created_at TIMESTAMP, expires_at TIMESTAMP);
		CREATE INDEX idx_auth_session_auth_user_id ON auth_session(auth_user_id);
		CREATE INDEX idx_auth_session_expires_at ON auth_session(expires_at);
	`))

	return migs
}

// Open or create the DB
func openDB(log log.Log, config dbh.DBConfig) (*gorm.DB, error) {
	log.Infof("Opening arc DB")
	db, err := dbh.OpenDB(log, config, migrations(log), 0)
	if err != nil {
		return nil, err
	}
	nUsers := int64(0)
	if err := db.Table("auth_user").Count(&nUsers).Error; err != nil {
		return nil, err
	}
	if nUsers == 0 {
		pwd := rando.StrongRandomAlphaNumChars(20)
		log.Infof("auth_user table is empty, creating admin user.")
		log.Infof("Username: admin")
		log.Infof("Password: %v", pwd)
		user := model.AuthUser{
			Email:     "admin",
			Password:  pwdhash.HashPasswordBase64(pwd),
			CreatedAt: time.Now().UTC(),
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, err
		}
	}

	return db, err
}

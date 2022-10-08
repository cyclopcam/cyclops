package dbh

import (
	"database/sql"
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/bmharper/cyclops/pkg/log"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DBConnectFlags are flags passed to OpenDB.
type DBConnectFlags int

const DriverPostgres = "postgres"
const DriverSqlite = "sqlite3"

const (
	// DBConnectFlagWipeDB causes the entire DB to erased, and re-initialized from scratch (useful for unit tests).
	DBConnectFlagWipeDB DBConnectFlags = 1 << iota
)

var DBNotExistRegex *regexp.Regexp

// DBConfig is the standard database config that we expect to find on our JSON config file.
type DBConfig struct {
	Driver      string
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	SSLCert     string
	SSLKey      string
	SSLRootCert string
}

func MakeSqliteConfig(filename string) DBConfig {
	return DBConfig{
		Driver:   DriverSqlite,
		Database: filename,
	}
}

// LogSafeDescription seturn a string that is useful for debugging connection issues, but doesn't leak secrets
func (db *DBConfig) LogSafeDescription() string {
	desc := fmt.Sprintf("driver=%s host=%v database=%v username=%v", db.Driver, db.Host, db.Database, db.Username)
	if db.Port != 0 {
		desc += fmt.Sprintf(" port=%v", db.Port)
	}
	return desc
}

// DSN returns a database connection string (built for Postgres and Sqlite only).
func (db *DBConfig) DSN() string {
	if db.Driver == DriverSqlite {
		return db.Database
	}
	escape := func(s string) string {
		if s == "" {
			return "''"
		} else if !strings.ContainsAny(s, " '\\") {
			return s
		}
		e := strings.Builder{}
		e.WriteRune('\'')
		for _, r := range s {
			if r == '\\' || r == '\'' {
				e.WriteRune('\\')
			}
			e.WriteRune(r)
		}
		e.WriteRune('\'')
		return e.String()
	}
	dsn := fmt.Sprintf("host=%v user=%v password=%v dbname=%v", escape(db.Host), escape(db.Username), escape(db.Password), escape(db.Database))
	if db.Port != 0 {
		dsn += fmt.Sprintf(" port=%v", db.Port)
	}
	if db.SSLKey != "" {
		dsn += fmt.Sprintf(" sslmode=require sslcert=%v sslkey=%v sslrootcert=%v", escape(db.SSLCert), escape(db.SSLKey), escape(db.SSLRootCert))
	} else {
		dsn += fmt.Sprintf(" sslmode=disable")
	}
	return dsn
}

// MakeMigrations turns a sequence of SQL expression into burntsushi migrations.
func MakeMigrations(log log.Log, sql []string) []migration.Migrator {
	migs := []migration.Migrator{}
	idx := 0
	for _, str := range sql {
		migs = append(migs, MakeMigrationFromSQL(log, &idx, str))
	}
	return migs
}

// MakeMigrationFromSQL turns an SQL string into a burntsushi migration
func MakeMigrationFromSQL(log log.Log, migrationNumber *int, sql string) migration.Migrator {
	idx := *migrationNumber + 1
	*migrationNumber++

	return func(tx migration.LimitedTx) error {
		summary := strings.TrimSpace(sql)
		var l int
		if l = len(summary) - 1; l > 40 {
			l = 40
		}
		firstNewline := strings.IndexAny(summary, "\n\r")
		if firstNewline != -1 && firstNewline < l {
			l = firstNewline
		}
		log.Infof("Running migration %v: '%v...'", idx, summary[:l])
		_, err := tx.Exec(sql)
		return err
	}
}

// MakeMigrationFromFunc wraps a migration function with another migration that logs to our logfile
func MakeMigrationFromFunc(log log.Log, migrationNumber *int, f migration.Migrator) migration.Migrator {
	idx := *migrationNumber + 1
	*migrationNumber++

	return func(tx migration.LimitedTx) error {
		log.Infof("Running migration %v: (function)", idx)
		return f(tx)
	}
}

// OpenDB creates a new DB, or opens an existing one, and runs all the migrations before returning.
func OpenDB(log log.Log, dbc DBConfig, migrations []migration.Migrator, flags DBConnectFlags) (*gorm.DB, error) {
	if flags&DBConnectFlagWipeDB != 0 {
		if err := DropAllTables(log, dbc); err != nil {
			return nil, err
		}
	}

	// This is the common fast path, where the database has been created
	db, err := migration.Open(dbc.Driver, dbc.DSN(), migrations)
	if err == nil {
		db.Close()
		gormDB, err := gormOpen(dbc.Driver, dbc.DSN())
		//if err != nil {
		//	err = fmt.Errorf("Failed to open %v database '%v': %w", driver, dsn, err)
		//}
		return gormDB, err
	}

	// Automatically create the database if it doesn't already exist
	if !isDatabaseNotExist(err) {
		return nil, err
	}

	log.Infof("Attempting to create database %v", dbc.Database)

	cfgCreate := dbc

	if dbc.Driver == DriverPostgres {
		// connect to the 'postgres' database in order to create the new DB
		cfgCreate.Database = "postgres"
	}

	if err := createDB(dbc.Driver, cfgCreate.DSN(), dbc.Database); err != nil {
		return nil, fmt.Errorf("While trying to create database '%v': %v", dbc.Database, err)
	}
	// once again, run migrations (now that the DB has been created)
	db, err = migration.Open(dbc.Driver, dbc.DSN(), migrations)
	if err != nil {
		return nil, err
	}
	db.Close()
	// finally, open with gorm
	return gormOpen(dbc.Driver, dbc.DSN())
}

// DropAllTables delete all tables in the given database.
// If the database does not exist, returns nil.
// This function is intended to be used by unit tests.
func DropAllTables(log log.Log, dbc DBConfig) error {
	if dbc.Driver == DriverSqlite {
		filename := dbc.Database
		err := os.Remove(filename)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	// Handle Postgres
	if dbc.Driver != DriverPostgres {
		return fmt.Errorf("DropAllTables not supported on %v", dbc.Driver)
	}
	db, err := sql.Open(dbc.Driver, dbc.DSN())
	if err == nil {
		// Force delay-connect drivers to attempt a connect now
		err = db.Ping()
	}
	if isDatabaseNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	defer db.Close()
	log.Warnf("Erasing entire DB '%v'", dbc.Database)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	dropAllTablesPostgres(log, tx)
	return tx.Commit()
}

func dropAllTablesPostgres(log log.Log, tx *sql.Tx) error {
	rows, err := tx.Query(`
	SELECT table_name, table_schema
	FROM information_schema.tables
	WHERE
	table_schema <> 'pg_catalog' AND
	table_schema <> 'information_schema'`)
	if err != nil {
		return err
	}
	tables := []string{}
	for rows.Next() {
		var table, schema string
		if err := rows.Scan(&table, &schema); err != nil {
			return err
		}
		tables = append(tables, fmt.Sprintf(`"%v"."%v"`, schema, table))
	}
	for _, table := range tables {
		// Skip PostGIS views
		if table == `"public"."geography_columns"` ||
			table == `"public"."geometry_columns"` ||
			table == `"public"."spatial_ref_sys"` ||
			table == `"public"."raster_columns"` ||
			table == `"public"."raster_overviews"` {
			continue
		}
		//	log.Warnf("Dropping table %v", table)
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE %v CASCADE", table)); err != nil {
			return err
		}
	}
	return nil
}

// SQLCleanIDList turns a string such as "10,34" into the string "(10,34)", so that it can be used inside an IN clause.
// It is acceptable for the raw string to end with an extra trailing comma
func SQLCleanIDList(raw string) string {
	if len(raw) != 0 && raw[len(raw)-1] == ',' {
		// remove trailing comma
		raw = raw[:len(raw)-1]
	}
	if len(raw) == 0 {
		// Postgres doesn't like (), so at least you get an invalid SQL syntax error
		return "()"
	}
	res := strings.Builder{}
	res.WriteRune('(')
	parts := strings.Split(raw, ",")
	for i, t := range parts {
		id, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			continue
		}
		res.WriteString(strconv.FormatInt(id, 10))
		if i != len(parts)-1 {
			res.WriteRune(',')
		}
	}
	res.WriteRune(')')
	return res.String()
}

// Turn an array such as [1,2] into the string "(1,2)"
func SQLFormatIDArray(ids []int64) string {
	res := strings.Builder{}
	res.WriteRune('(')
	for i, id := range ids {
		res.WriteString(strconv.FormatInt(id, 10))
		if i != len(ids)-1 {
			res.WriteRune(',')
		}
	}
	res.WriteRune(')')
	return res.String()
}

func IsKeyViolation(err error) bool {
	em := err.Error()
	return strings.Index(em, "violates unique constraint") != -1
}

func IsKeyViolationOnIndex(err error, indexName string) bool {
	em := err.Error()
	return strings.Index(em, "violates unique constraint") != -1 &&
		strings.Index(em, indexName) != -1
}

func gormOpen(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case DriverPostgres:
		dialector = postgres.Open(dsn)
	case DriverSqlite:
		dialector = sqlite.Open(dsn)
	}

	newLogger := logger.New(
		stdlog.New(os.Stdout, "\r\n", stdlog.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // This is the primary reason we use a custom logger. Record not found is just never a loggable thing.
			Colorful:                  true,
		},
	)

	config := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			// Disable pluralization of tables.
			// This is just another thing to worry about when writing our own migrations, so rather disable it.
			SingularTable: true,
		},
		Logger: newLogger,
	}
	db, err := gorm.Open(dialector, config)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func isDatabaseNotExist(err error) bool {
	if err == nil {
		return false
	}
	return DBNotExistRegex.MatchString(err.Error())
}

// Create a database called dbCreateName, by connecting to dsn.
func createDB(driver, dsn, dbCreateName string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("CREATE DATABASE " + dbCreateName); err != nil {
		return err
	}
	return nil
}

func init() {
	// Checking for "does not exist" is definitely not sufficient, because that can get
	// hit while trying to run, for example, a database migration on an incorrect field name.
	// This is a bad false positive, and I have hit it in practice.
	//
	// True positive error examples:
	// pq: database "testx" does not exist
	DBNotExistRegex = regexp.MustCompile(`database "[^"]+" does not exist`)
}

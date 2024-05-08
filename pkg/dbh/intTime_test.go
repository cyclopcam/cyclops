package dbh

import (
	"database/sql"
	"encoding/json"
	stdlog "log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type IntTimeTester struct {
	ID     int64   `gorm:"primaryKey" json:"id"`
	MyTime IntTime `json:"myTime"`
}

func TestIntTime(t *testing.T) {
	t1 := IntTime(0)
	a := time.Date(2022, time.February, 3, 4, 5, 6, 777*1000*1000, time.UTC)
	t1.Set(a)
	b := t1.Get()
	require.Equal(t, a, b)

	db := OpenSqliteTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE int_time_tester (id INTEGER PRIMARY KEY, my_time INT)").Error)

	// Ensure that an IntTime value of zero ends up as 'null' in the database.
	null := IntTimeTester{
		ID:     1,
		MyTime: 0,
	}
	require.NoError(t, db.Save(&null).Error)
	read := IntTimeTester{}
	require.NoError(t, db.First(&read).Error)
	require.Equal(t, null, read)

	nullable := sql.NullInt64{}
	require.NoError(t, db.Raw("SELECT my_time FROM int_time_tester WHERE id = 1").Row().Scan(&nullable))
	require.Equal(t, false, nullable.Valid)

	// Check JSON representation of null IntTime
	jj, err := json.Marshal(&null)
	require.NoError(t, err)
	require.Equal(t, `{"id":1,"myTime":0}`, string(jj))

	// Ensure we get expected IsZero()
	t0 := IntTime(0)
	require.True(t, t0.IsZero())
	require.True(t, t0.Get().IsZero())
	t0_b := MakeIntTime(time.Time{})
	require.Equal(t, t0, t0_b)

	// test non-null values
	other := IntTimeTester{
		ID:     2,
		MyTime: MakeIntTime(time.Date(2022, time.February, 3, 4, 5, 6, 777*1000*1000, time.UTC)),
	}
	require.NoError(t, db.Save(&other).Error)
	other2 := IntTimeTester{}
	require.NoError(t, db.Where("id = 2").First(&other2).Error)
	require.Equal(t, other.MyTime, other2.MyTime)
}

func OpenSqliteTestDB(t *testing.T) *gorm.DB {
	os.Remove("unit-test.sqlite")
	dialector := sqlite.Open("unit-test.sqlite")

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
	require.NoError(t, err)

	return db
}

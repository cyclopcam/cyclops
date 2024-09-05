package configdb

import (
	"os"
	"testing"

	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

func createTestDB(t *testing.T) *ConfigDB {
	os.Remove("test-configdb.sqlite")
	db, err := NewConfigDB(logs.NewTestingLog(t), "test-configdb.sqlite", "")
	require.NoError(t, err)
	return db
}

func TestNextID(t *testing.T) {
	db := createTestDB(t)
	for i := 0; i < 3; i++ {
		tx := db.DB.Begin()
		id, err := db.GenerateNewID(tx, "camera")
		require.NoError(t, err)
		require.Equal(t, int64(i+1), id)
		tx.Commit()
	}
}

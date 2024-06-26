package configdb

import (
	"testing"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/stretchr/testify/require"
)

func createTestDB(t *testing.T) *ConfigDB {
	db, err := NewConfigDB(log.NewTestingLog(t), "test-configdb.sqlite", "")
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

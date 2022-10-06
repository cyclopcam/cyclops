package configdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/log"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

type ConfigDB struct {
	Log log.Log
	DB  *gorm.DB

	keyLock    sync.Mutex
	privateKey wgtypes.Key

	sharedSecretKeysLock sync.Mutex
	sharedSecretKeys     map[string][]byte // Map key is RemotePublicKey, SHA256(X25519_Shared_Secret(MyPrivateKey, RemotePublicKey)).
}

func NewConfigDB(logger log.Log, dbFilename string) (*ConfigDB, error) {
	os.MkdirAll(filepath.Dir(dbFilename), 0777)
	configDB, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbFilename), Migrations(logger), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database %v: %w", dbFilename, err)
	}
	return &ConfigDB{
		Log: logger,
		DB:  configDB,
	}, nil
}

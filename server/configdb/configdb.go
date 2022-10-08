package configdb

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/log"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

type ConfigDB struct {
	Log        log.Log
	DB         *gorm.DB
	PrivateKey wgtypes.Key
	PublicKey  wgtypes.Key

	//keyLock    sync.Mutex
	//privateKey wgtypes.Key

	//sharedSecretKeysLock sync.Mutex
	//sharedSecretKeys     map[string][]byte // Map key is RemotePublicKey, SHA256(X25519_Shared_Secret(MyPrivateKey, RemotePublicKey)).
}

func NewConfigDB(logger log.Log, dbFilename string) (*ConfigDB, error) {
	os.MkdirAll(filepath.Dir(dbFilename), 0777)
	configDB, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbFilename), Migrations(logger), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database %v: %w", dbFilename, err)
	}
	privateKey, err := readOrCreatePrivateKey(logger, configDB)
	if err != nil {
		return nil, fmt.Errorf("Failed to read or create private key: %w", err)
	}
	return &ConfigDB{
		Log:        logger,
		DB:         configDB,
		PrivateKey: privateKey,
		PublicKey:  privateKey.PublicKey(),
	}, nil
}

func readOrCreatePrivateKey(logger log.Log, db *gorm.DB) (wgtypes.Key, error) {
	k := Key{}
	db.Where("name = ?", KeyMain).First(&k)
	if len(k.Value) == 32 {
		return wgtypes.NewKey(k.Value)
	}
	// Generate key
	logger.Infof("Generating private key")
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, err
	}
	k.Name = KeyMain
	k.Value = make([]byte, 32)
	copy(k.Value, key[:])
	if err := db.Create(&k).Error; err != nil {
		return wgtypes.Key{}, err
	}
	return key, nil
}

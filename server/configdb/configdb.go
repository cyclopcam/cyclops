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

func NewConfigDB(logger log.Log, dbFilename, explicitPrivateKey string) (*ConfigDB, error) {
	os.MkdirAll(filepath.Dir(dbFilename), 0777)
	configDB, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbFilename), Migrations(logger), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database %v: %w", dbFilename, err)
	}
	// Our config DB stores secrets, so we need to make sure that nobody except for us can read it
	if err := os.Chmod(dbFilename, 0600); err != nil {
		return nil, fmt.Errorf("Failed to change permissions on database %v: %w", dbFilename, err)
	}
	privateKey, err := readOrCreatePrivateKey(logger, configDB, explicitPrivateKey)
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

func readOrCreatePrivateKey(logger log.Log, db *gorm.DB, explicitPrivateKey string) (wgtypes.Key, error) {
	var key wgtypes.Key
	var err error
	if explicitPrivateKey != "" {
		key, err = wgtypes.ParseKey(explicitPrivateKey)
		if err != nil {
			return wgtypes.Key{}, fmt.Errorf("Explicit private key invalid: %w", err)
		}
	}

	keyRecord := Key{}
	db.Where("name = ?", KeyMain).First(&keyRecord)
	if keyRecord.Value != "" {
		inDB, err := wgtypes.ParseKey(keyRecord.Value)
		if err != nil {
			return inDB, err
		}
		if explicitPrivateKey == "" {
			// This is the normal code path 99.999% of the time, when an existing system is booting up
			return inDB, nil
		}
		// If the explicit key doesn't match an existing key in the database, then print the existing
		// key to the console, because this might be the last time that the user ever has access to it.
		explicit, err := wgtypes.ParseKey(explicitPrivateKey)
		if err != nil {
			return wgtypes.Key{}, err
		}
		if explicit == inDB {
			// It's not intended that you always start the system with an explicit private key,
			// but there's nothing wrong with doing it this way.
			return inDB, nil
		} else {
			logger.Warnf("Explicit private key does not match private key in database")
			logger.Warnf("Private key from database was %v", inDB)
			logger.Warnf("I am going to overwrite the database key with the explicitly provided key.")
			logger.Warnf("This might be the last time you will ever see the key %v, so if you need to", inDB)
			logger.Warnf("preserve it, or you're not sure, then you should copy it somewhere safe now.")
			if err := db.Delete(&keyRecord).Error; err != nil {
				return wgtypes.Key{}, err
			}
		}
	}
	if explicitPrivateKey == "" {
		// Generate key
		logger.Infof("Generating private key")
		key, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return wgtypes.Key{}, err
		}
	} else {
		logger.Infof("Using explicitly specified private key")
	}
	keyRecord.Name = KeyMain
	keyRecord.Value = key.String()
	if err := db.Create(&keyRecord).Error; err != nil {
		return wgtypes.Key{}, err
	}
	return key, nil
}

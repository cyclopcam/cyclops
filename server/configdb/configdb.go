package configdb

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

type ConfigDB struct {
	Log        logs.Log
	DB         *gorm.DB
	PrivateKey wgtypes.Key
	PublicKey  wgtypes.Key

	// Addresses allowed from VPN network. Used to detect if user is connecting from LAN or VPN.
	// Injected by VPN system after it has connected. There can be two: an IPv4 and an IPv6.
	VpnAllowedIP net.IPNet

	configLock sync.Mutex // Guards all access to Config
	config     ConfigJSON // Read from system_config table at startup
}

func NewConfigDB(logger logs.Log, dbFilename, explicitPrivateKey string) (*ConfigDB, error) {
	os.MkdirAll(filepath.Dir(dbFilename), 0770)
	configDB, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbFilename), Migrations(logger), dbh.DBConnectFlagSqliteWAL)
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

	systemConfig := SystemConfig{}
	configDB.First(&systemConfig)

	cdb := &ConfigDB{
		Log:        logger,
		DB:         configDB,
		PrivateKey: privateKey,
		PublicKey:  privateKey.PublicKey(),
	}
	if systemConfig.Value != nil {
		cdb.config = systemConfig.Value.Data
	}
	return cdb, nil
}

func readOrCreatePrivateKey(logger logs.Log, db *gorm.DB, explicitPrivateKey string) (wgtypes.Key, error) {
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
			logger.Warnf("Old private key from database was %v", inDB)
			logger.Warnf("I am going to overwrite that database key with the explicitly provided key.")
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

// Generate a new ID from the 'next_id' table in the database
func (c *ConfigDB) GenerateNewID(tx *gorm.DB, key string) (int64, error) {
	if err := tx.Exec(`INSERT INTO next_id (key, value) VALUES (?, 1) ON CONFLICT(key) DO UPDATE SET value = value + 1`, key).Error; err != nil {
		return 0, err
	}
	nextid := int64(0)
	if err := tx.Raw("SELECT value FROM next_id WHERE key = $1", key).Row().Scan(&nextid); err != nil {
		return 0, err
	}
	return nextid, nil
}

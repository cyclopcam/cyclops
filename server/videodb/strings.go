package videodb

import (
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Get a database-wide unique ID for the given string.
// At some point we should implement a cleanup method that gets rid of strings that are no longer used.
// It is beneficial to keep the IDs small, because smaller numbers produce smaller DB records.
func (v *VideoDB) StringToID(s string) (uint32, error) {
	v.stringToIDLock.Lock()
	defer v.stringToIDLock.Unlock()

	// Find in cache
	if id, ok := v.stringToID[s]; ok {
		return id, nil
	}

	// Find or create in DB
	id, err := v.stringToIDFromDB(s)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Resolve multiple strings to IDs
func (v *VideoDB) StringsToID(s []string) ([]uint32, error) {
	v.stringToIDLock.Lock()
	defer v.stringToIDLock.Unlock()

	ids := make([]uint32, len(s))
	for i := 0; i < len(s); i++ {
		if id, ok := v.stringToID[s[i]]; ok {
			ids[i] = id
		} else {
			if id, err := v.stringToIDFromDB(s[i]); err != nil {
				return nil, err
			} else {
				ids[i] = id
			}
		}
	}

	return ids, nil
}

// You must be holding the stringToIDLock before calling this function.
func (v *VideoDB) stringToIDFromDB(s string) (uint32, error) {
	for iter := 0; iter < 2; iter++ {
		// Find in DB
		var id uint32
		if err := v.db.Raw("SELECT id FROM strings WHERE value = ?", s).Row().Scan(&id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
				// This is normal.
				// We'll fall through to creating a new ID, and then return back here for a 2nd pass.
			} else {
				v.log.Errorf("Unexpected error searching 'strings' table for '%v': %v", s, err)
				return 0, err
			}
		} else {
			//v.log.Infof("Found string %v -> %v", s, id)
			v.stringToID[s] = id
			return id, nil
		}

		// Create new ID
		v.log.Infof("Inserting new string '%v' into 'strings' table", s)
		if err := v.db.Exec("INSERT INTO strings (value) VALUES (?)", s).Error; err != nil {
			return 0, err
		}
	}

	return 0, fmt.Errorf("Unexpected code path in StringToID")
}

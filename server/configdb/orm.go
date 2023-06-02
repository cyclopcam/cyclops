package configdb

import (
	"strings"

	"github.com/bmharper/cyclops/pkg/dbh"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// SYNC-RECORD-CAMERA
type Camera struct {
	BaseModel
	Model            string `json:"model"`                                // eg HikVision (actually CameraModels enum)
	Name             string `json:"name"`                                 // Friendly name
	Host             string `json:"host"`                                 // Hostname such as 192.168.1.33
	Port             int    `json:"port" gorm:"default:null"`             // if 0, then default is 554
	Username         string `json:"username"`                             // RTSP username
	Password         string `json:"password"`                             // RTSP password
	HighResURLSuffix string `json:"highResURLSuffix" gorm:"default:null"` // eg Streaming/Channels/101 for HikVision. Can leave blank if Model is a known type.
	LowResURLSuffix  string `json:"lowResURLSuffix" gorm:"default:null"`  // eg Streaming/Channels/102 for HikVision. Can leave blank if Model is a known type.
}

type Variable struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

type Key struct {
	Name  string `gorm:"primaryKey"`
	Value string // normal (not URL-safe) base64 encoded (same as Wireguard)
}

// UserPermissions are single characters that are present in the user's Permissions field
type UserPermissions string

const (
	UserPermissionAdmin  UserPermissions = "a"
	UserPermissionViewer UserPermissions = "v"
)

// SYNC-RECORD-USER
type User struct {
	BaseModel
	Username           string `json:"username"`
	UsernameNormalized string `json:"username_normalized"`
	Permissions        string `json:"permissions"`
	Name               string `json:"name" gorm:"default:null"`
	Password           []byte `json:"-" gorm:"default:null"`
}

type Session struct {
	CreatedAt dbh.IntTime
	Key       []byte
	UserID    int64
	ExpiresAt dbh.IntTime `gorm:"default:null"`
}

type RecordInstruction struct {
	BaseModel
	StartAt    dbh.IntTime `json:"startAt"`
	FinishAt   dbh.IntTime `json:"finishAt"`
	Resolution string      `json:"resolution" gorm:"default:null"` // One of defs.Resolution (LD or HD)
}

func IsValidPermission(p string) bool {
	return p == string(UserPermissionAdmin) || p == string(UserPermissionViewer)
}

func (u *User) HasPermission(p UserPermissions) bool {
	if strings.Contains(u.Permissions, string(UserPermissionAdmin)) {
		return true
	}
	return strings.Index(u.Permissions, string(p)) != -1
}

func NormalizeUsername(username string) string {
	return strings.ToLower(username)
}

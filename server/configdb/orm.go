package configdb

import (
	"strings"

	"github.com/cyclopcam/dbh"
)

// GORM notes:
// GORM's automatic created_at and updated_at functionality is weird. It will ignore
// the Scan() and Value() of your custom time type (IntTime in our case), and instead
// interpret the time however it wants. Since we want unix milliseconds, we have two
// options: Either use autoCreateTime:false and autoUpdateTime:false, in which case
// our IntTime will Scan() and Value() out to unix milliseconds. Or, we can use
// autoCreateTime:milli and autoUpdateTime:milli, in which case we get the same
// numbers in the database, but we have the advantage of GORM automatically injecting
// the values for us. So we use the latter.

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// SYNC-RECORD-CAMERA
type Camera struct {
	BaseModel
	Model            string      `json:"model"`                                // eg HikVision (actually CameraModels enum)
	Name             string      `json:"name"`                                 // Friendly name
	Host             string      `json:"host"`                                 // Hostname such as 192.168.1.33
	Port             int         `json:"port" gorm:"default:null"`             // if 0, then default is 554
	Username         string      `json:"username"`                             // RTSP username
	Password         string      `json:"password"`                             // RTSP password
	HighResURLSuffix string      `json:"highResURLSuffix" gorm:"default:null"` // eg Streaming/Channels/101 for HikVision. Can leave blank if Model is a known type.
	LowResURLSuffix  string      `json:"lowResURLSuffix" gorm:"default:null"`  // eg Streaming/Channels/102 for HikVision. Can leave blank if Model is a known type.
	CreatedAt        dbh.IntTime `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt        dbh.IntTime `json:"updatedAt" gorm:"autoUpdateTime:milli"`
	DetectionZone    string      `json:"detectionZone" gorm:"default:null"` // See DetectionZone.EncodeBase64()

	// The long lived name is used to identify the camera in the storage archive.
	// If necessary, we can make this configurable.
	// At present, it is equal to the camera ID. But in future, we could allow
	// the user to override this. For example, if their system goes down, but their
	// archive is on another disk, and they want to restore all the cameras, and
	// still have the history intact, and matching up to the new (but same) cameras.
	// Or perhaps you have to replace a camera, but want to retain the logical identify.
	LongLivedName string `json:"longLivedName"`
}

// Compare the current camera config against the new camera config, and return
// true if the connection details refer to the exact same camera host and config.
func (c *Camera) EqualsConnection(newCam *Camera) bool {
	return c.Model == newCam.Model &&
		c.Host == newCam.Host &&
		c.Port == newCam.Port &&
		c.Username == newCam.Username &&
		c.Password == newCam.Password &&
		c.HighResURLSuffix == newCam.HighResURLSuffix &&
		c.LowResURLSuffix == newCam.LowResURLSuffix
}

func (c *Camera) DeepEquals(x *Camera) bool {
	// SYNC-RECORD-CAMERA
	if !c.EqualsConnection(x) {
		return false
	}
	return c.Name == x.Name &&
		c.LongLivedName == x.LongLivedName &&
		c.DetectionZone == x.DetectionZone
}

type Variable struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

// Generic key/value pairs in our database. For example, KeyMain, etc.
type Key struct {
	Name  string `gorm:"primaryKey"`
	Value string // normal (not URL-safe) base64 encoded (same as Wireguard)
}

type SystemConfig struct {
	Key   string `gorm:"primaryKey"`
	Value *dbh.JSONField[ConfigJSON]
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
	CreatedAt dbh.IntTime `gorm:"autoCreateTime:milli"`
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

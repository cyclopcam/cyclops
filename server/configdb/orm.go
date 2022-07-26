package configdb

import (
	"strings"

	"github.com/bmharper/cyclops/server/dbh"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

type Camera struct {
	BaseModel
	Model            string `json:"model"`            // eg HikVision (actually CameraModels enum)
	Name             string `json:"name"`             // Friendly name
	Host             string `json:"host"`             // Hostname such as 192.168.1.33
	Port             int    `json:"port"`             // if 0, then default is 554
	Username         string `json:"username"`         // RTSP username
	Password         string `json:"password"`         // RTSP password
	HighResURLSuffix string `json:"highResURLSuffix"` // eg Streaming/Channels/101 for HikVision. Can leave blank if Model is a known type.
	LowResURLSuffix  string `json:"lowResURLSuffix"`  // eg Streaming/Channels/102 for HikVision. Can leave blank if Model is a known type.
	//URL              string `json:"url"`              // RTSP url such as rtsp://user:password@192.168.1.33:554
}

type Variable struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

// UserPermissions are single characters that are present in the user's Permissions field
type UserPermissions string

const (
	UserPermissionAdmin  UserPermissions = "a"
	UserPermissionViewer UserPermissions = "v"
)

type User struct {
	BaseModel
	Username    string `json:"username"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	Password    []byte `json:"-"`
}

type Session struct {
	Key       []byte
	UserID    int64
	ExpiresAt dbh.IntTime
}

func (u *User) HasPermission(p UserPermissions) bool {
	return strings.Index(u.Permissions, string(p)) != -1
}

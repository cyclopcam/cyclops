package model

import "time"

type AuthUser struct {
	BaseModel
	Email           string    `json:"email"`
	Password        string    `json:"-"`
	CreatedAt       time.Time `json:"createdAt"`
	SitePermissions string    `json:"sitePermissions"`
}

// Login session (cookie-based)
type AuthSession struct {
	Key        string `gorm:"primaryKey"`
	AuthUserID int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// Same semantics as AuthSession, but intended for API keys
type AuthApiKey struct {
	Key          string `gorm:"primaryKey"`
	RawKeyPrefix string // Unhashed key prefix, e.g. "sk-xyz123"
	AuthUserID   int64
	CreatedAt    time.Time
	ExpiresAt    time.Time `gorm:"default:null"` // Can be zero, which means no expiry
}

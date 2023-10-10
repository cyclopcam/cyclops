package model

import "time"

type AuthUser struct {
	BaseModel
	Email           string    `json:"email"`
	Password        string    `json:"-"`
	CreatedAt       time.Time `json:"createdAt"`
	SitePermissions string    `json:"sitePermissions"`
}

type AuthSession struct {
	Key        string `gorm:"primaryKey"`
	AuthUserID int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

package auth

import (
	"errors"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/pkg/pwdhash"
)

func (a *AuthServer) CreateUser(email, password, sitePermissions string) error {
	if email == "" {
		return errors.New("email cannot be empty")
	}
	if err := IsPasswordOK(password); err != nil {
		return err
	}
	user := model.AuthUser{
		Email:           email,
		Password:        pwdhash.HashPasswordBase64(password),
		CreatedAt:       time.Now(),
		SitePermissions: sitePermissions,
	}
	return a.db.Create(&user).Error
}

func (a *AuthServer) AllUsers() ([]model.AuthUser, error) {
	var users []model.AuthUser
	return users, a.db.Find(&users).Error
}

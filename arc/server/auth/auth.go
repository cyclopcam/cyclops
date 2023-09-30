package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bmharper/cyclops/arc/server/model"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/pkg/pwdhash"
	"github.com/bmharper/cyclops/pkg/rando"
	"github.com/bmharper/cyclops/pkg/www"
	"gorm.io/gorm"
)

type Credentials struct {
	UserID                           int64
	AuthenticatedViaSessionCookie    string // If session was authenticated via session cookie, this is pwdhash.HashSessionTokenBase64(cookie.Value)
	AuthenticatedViaUsernamePassword bool   // If authenticated via username/password, this is true
}

type AuthServer struct {
	db                *gorm.DB
	log               log.Log
	sessionCookieName string
}

func NewAuthServer(db *gorm.DB, log log.Log, sessionCookieName string) *AuthServer {
	return &AuthServer{
		db:                db,
		log:               log,
		sessionCookieName: sessionCookieName,
	}
}

// If authorization fails, sends a response to 'w', and returns nil
// If authorization succeeds, returns a non-nil Credentials
func (a *AuthServer) AuthenticateRequest(w http.ResponseWriter, r *http.Request) *Credentials {
	cookie, _ := r.Cookie(a.sessionCookieName)
	if cookie != nil {
		hashedTokenb64 := pwdhash.HashSessionTokenBase64(cookie.Value)
		session := model.AuthSession{}
		a.db.First(&session).Where("key = ?", hashedTokenb64)
		if session.AuthUserID != 0 {
			return &Credentials{
				UserID:                        session.AuthUserID,
				AuthenticatedViaSessionCookie: hashedTokenb64,
			}
		}
	}
	if username, password, ok := r.BasicAuth(); ok {
		user := model.AuthUser{}
		a.db.First(&user).Where("email = ?", username)
		if user.ID != 0 {
			if pwdhash.VerifyHashBase64(password, user.Password) {
				return &Credentials{
					UserID:                           user.ID,
					AuthenticatedViaUsernamePassword: true,
				}
			}
		}
	}

	www.SendError(w, "Unauthorized", http.StatusUnauthorized)
	return nil
}

func (a *AuthServer) Login(w http.ResponseWriter, r *http.Request) {
	cred := a.AuthenticateRequest(w, r)
	if cred == nil {
		return
	}
	if cred.AuthenticatedViaSessionCookie != "" {
		// Already logged in
		www.SendOK(w)
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(365 * 24 * time.Hour)

	token := rando.StrongRandomAlphaNumChars(20)
	session := model.AuthSession{
		Key:        pwdhash.HashSessionTokenBase64(token),
		AuthUserID: cred.UserID,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
	}
	if err := a.db.Create(&session).Error; err != nil {
		a.log.Errorf("Error creating session: %v", err)
		www.SendError(w, "Error creating session", http.StatusInternalServerError)
		return
	}
	cookie := &http.Cookie{
		Name:    a.sessionCookieName,
		Value:   token,
		Path:    "/",
		Expires: expiresAt,
	}
	http.SetCookie(w, cookie)
	www.SendOK(w)
}

func (a *AuthServer) SetPassword(userID int64, password string) error {
	return a.db.Model(&model.AuthUser{}).Where("id = ?", userID).Update("password", pwdhash.HashPasswordBase64(password)).Error
}

// Erase all sessions except the authentication mechanism that was used to issue this API request
func (a *AuthServer) EraseAllSessionsExceptCallingSession(cred *Credentials) error {
	if err := a.eraseAllSessionsExceptCallingSession(cred); err != nil {
		a.log.Errorf("Error erasing sessions: %v", err)
		return err
	}
	return nil
}

func (a *AuthServer) eraseAllSessionsExceptCallingSession(cred *Credentials) error {
	if cred.AuthenticatedViaSessionCookie != "" {
		return a.db.Where("auth_user_id = ? AND key != ?", cred.UserID, cred.AuthenticatedViaSessionCookie).Delete(&model.AuthSession{}).Error
	} else if cred.AuthenticatedViaUsernamePassword {
		return a.db.Where("auth_user_id = ?", cred.UserID).Delete(&model.AuthSession{}).Error
	} else {
		return fmt.Errorf("Unrecognized authentication mechanism")
	}
}

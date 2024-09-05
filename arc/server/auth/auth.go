package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/pkg/rando"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/www"
	"gorm.io/gorm"
)

// Site-wide (aka some kind of superuser) permissions.
// Stored in auth_user.site_permissions
const (
	SitePermissionAdmin = "a" // You can do anything
)

type AuthType int

const (
	AuthTypeSessionCookie AuthType = 1 << iota
	AuthTypeUsernamePassword
	AuthTypeApiKey
)

type Credentials struct {
	UserID                           int64
	SitePermissions                  string
	AuthenticatedViaSessionCookie    string // If session was authenticated via session cookie, this is pwdhash.HashSessionTokenBase64(cookie.Value) - aka the value in the DB
	AuthenticatedViaApiKey           string // If session was authenticated via api key, this is pwdhash.HashSessionTokenBase64(key) - aka the value in the DB
	AuthenticatedViaUsernamePassword bool   // If authenticated via username/password, this is true
}

func (c *Credentials) IsAdmin() bool {
	return strings.Index(c.SitePermissions, SitePermissionAdmin) != -1
}

func (c *Credentials) PanicIfNotAdmin() {
	if !c.IsAdmin() {
		www.PanicForbiddenf("You must be an admin to access this resource")
	}
}

type AuthServer struct {
	db                *gorm.DB
	log               logs.Log
	sessionCookieName string
}

func NewAuthServer(db *gorm.DB, log logs.Log, sessionCookieName string) *AuthServer {
	return &AuthServer{
		db:                db,
		log:               log,
		sessionCookieName: sessionCookieName,
	}
}

// If authorization fails, sends a response to 'w', and returns nil
// If authorization succeeds, returns a non-nil Credentials
func (a *AuthServer) AuthenticateRequest(w http.ResponseWriter, r *http.Request, allowTypes AuthType) *Credentials {
	cred := a.authenticateRequest(w, r, allowTypes)
	if cred != nil {
		// Augment returned Credentials with additional information about the user
		user := model.AuthUser{}
		if err := a.db.Where("id = ?", cred.UserID).First(&user).Error; err != nil {
			a.log.Errorf("Error fetching user %v after authenticating request: %v", cred.UserID, err)
			return nil
		}
		cred.SitePermissions = user.SitePermissions
	}
	return cred
}

// Same contract as AuthenticateRequest()
func (a *AuthServer) authenticateRequest(w http.ResponseWriter, r *http.Request, allowTypes AuthType) *Credentials {
	if allowTypes&AuthTypeSessionCookie != 0 {
		cookie, _ := r.Cookie(a.sessionCookieName)
		if cookie != nil {
			hashedTokenb64 := pwdhash.HashSessionTokenBase64(cookie.Value)
			session := model.AuthSession{}
			a.db.Where("key = ?", hashedTokenb64).First(&session)
			if session.AuthUserID != 0 {
				return &Credentials{
					UserID:                        session.AuthUserID,
					AuthenticatedViaSessionCookie: hashedTokenb64,
				}
			}
		}
	}

	if allowTypes&AuthTypeApiKey != 0 {
		auth := r.Header.Get("Authorization")
		// We use "ApiKey" to avoid confusion with an OAuth token which is usually prefixed with "Bearer"
		apiKeyPrefix := "ApiKey "
		if strings.HasPrefix(auth, apiKeyPrefix) {
			keyStr := auth[len(apiKeyPrefix):]
			hashedTokenb64 := pwdhash.HashSessionTokenBase64(keyStr)
			key := model.AuthApiKey{}
			a.db.Where("key = ?", hashedTokenb64).First(&key)
			if key.AuthUserID != 0 {
				if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(time.Now()) {
					www.SendError(w, "Key has expired", http.StatusUnauthorized)
					return nil
				}
				return &Credentials{
					UserID:                 key.AuthUserID,
					AuthenticatedViaApiKey: hashedTokenb64,
				}
			}
		}
	}

	if allowTypes&AuthTypeUsernamePassword != 0 {
		if username, password, ok := r.BasicAuth(); ok {
			user := model.AuthUser{}
			a.db.Where("email = ?", username).First(&user)
			if user.ID != 0 {
				if pwdhash.VerifyHashBase64(password, user.Password) {
					return &Credentials{
						UserID:                           user.ID,
						AuthenticatedViaUsernamePassword: true,
					}
				} else {
					www.SendError(w, "Invalid password", http.StatusUnauthorized)
				}
			} else {
				www.SendError(w, "Invalid username", http.StatusUnauthorized)
			}
			return nil
		}
	}

	www.SendError(w, "Unauthorized", http.StatusUnauthorized)
	return nil
}

func (a *AuthServer) Login(w http.ResponseWriter, r *http.Request) {
	cred := a.AuthenticateRequest(w, r, AuthTypeUsernamePassword)
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

	token := rando.StrongRandomAlphaNumChars(30)
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
	a.log.Infof("User %v logged in with session cookie hash = %v", cred.UserID, session.Key)
	www.SendOK(w)
}

func (a *AuthServer) Logout(w http.ResponseWriter, r *http.Request) {
	cred := a.AuthenticateRequest(w, r, AuthTypeSessionCookie)
	if cred == nil {
		return
	}
	if cred.AuthenticatedViaSessionCookie != "" {
		a.log.Infof("User %v logout with session cookie hash = %v", cred.UserID, cred.AuthenticatedViaSessionCookie)
		a.db.Where("key = ?", cred.AuthenticatedViaSessionCookie).Delete(&model.AuthSession{})
	}
	www.SendOK(w)
}

func (a *AuthServer) CreateKey(w http.ResponseWriter, r *http.Request, cred *Credentials) {
	now := time.Now().UTC()

	token := "sk-" + rando.StrongRandomAlphaNumChars(44)
	key := model.AuthApiKey{
		Key:          pwdhash.HashSessionTokenBase64(token),
		RawKeyPrefix: token[:9],
		AuthUserID:   cred.UserID,
		CreatedAt:    now,
		ExpiresAt:    time.Time{},
	}
	if err := a.db.Create(&key).Error; err != nil {
		a.log.Errorf("Error creating API Key: %v", err)
		www.SendError(w, "Error creating API Key", http.StatusInternalServerError)
		return
	}
	a.log.Infof("User %v created auth API Key %v with hash = %v", cred.UserID, key.RawKeyPrefix, key.Key)
	type response struct {
		Key string `json:"key"`
	}
	resp := response{
		Key: token,
	}
	www.SendJSON(w, &resp)
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

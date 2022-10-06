package configdb

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/www"
)

const SessionCookie = "session"

func (c *ConfigDB) Login(w http.ResponseWriter, r *http.Request) {
	userID := c.GetUserID(r, true)
	if userID == 0 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}
	expiresAtUnixMilli := www.QueryInt64(r, "expiresAt") // 0 = no expiry
	expiresAt := time.Time{}
	if expiresAtUnixMilli != 0 {
		expiresAt = time.UnixMilli(expiresAtUnixMilli)
	}
	c.LoginInternal(w, userID, expiresAt, www.QueryValue(r, "loginMode"))
}

type loginResponseJSON struct {
	BearerToken string `json:"bearerToken"`
}

const (
	LoginModeCookie               = "Cookie"
	LoginModeBearerToken          = "BearerToken"
	LoginModeCookieAndBearerToken = "CookieAndBearerToken"
)

func (c *ConfigDB) LoginInternal(w http.ResponseWriter, userID int64, expiresAt time.Time, mode string) {
	doCookie := mode == LoginModeCookie || mode == LoginModeCookieAndBearerToken || mode == ""
	doBearer := mode == LoginModeBearerToken
	if !(doCookie || doBearer) {
		http.Error(w, "Invalid loginMode. Must be Cookie or BearerToken or CookieAndBearerToken (default is Cookie)", http.StatusBadRequest)
		return
	}

	// As of Chrome 104, max cookie duration is 400 days.
	// https://stackoverflow.com/questions/16626875/google-chrome-maximum-cookie-expiry-date
	// For a mobile app, we'll need some workaround to this, because you can't just have
	// your security system ask you for a password at some random time.
	// So this is our solution:
	// Whenever you login, you get two tokens:
	// 1. The cookie
	// 2. An X-Token header with a session token inside it.
	// The expiry date of the X-Token session has no limit to it.
	now := time.Now().UTC()
	maxCookieExpireDate := now.AddDate(0, 0, 399)

	cookieExpiresAt := expiresAt

	if cookieExpiresAt.IsZero() || cookieExpiresAt.After(maxCookieExpireDate) {
		cookieExpiresAt = maxCookieExpireDate
	}
	cookieKey := StrongRandomAlphaNumChars(30)
	bearerKey := StrongRandomBytes(32)
	if doCookie {
		cookieSession := Session{
			CreatedAt: dbh.MakeIntTime(now),
			Key:       HashSessionToken(cookieKey),
			UserID:    userID,
			ExpiresAt: dbh.MakeIntTime(cookieExpiresAt),
		}
		if err := c.DB.Create(&cookieSession).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if doBearer {
		bearerSession := Session{
			CreatedAt: dbh.MakeIntTime(now),
			Key:       HashSessionToken(string(bearerKey)),
			UserID:    userID,
			ExpiresAt: dbh.MakeIntTime(expiresAt),
		}
		if err := c.DB.Create(&bearerSession).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	c.PurgeExpiredSessions()
	c.Log.Infof("Logging %v in", userID)
	//c.Log.Infof("Logging %v in. key: %v. hashed key hex: %v", userID, key, hex.EncodeToString(HashSessionToken(key))) // only for debugging
	if doCookie {
		cookie := &http.Cookie{
			Name:    SessionCookie,
			Value:   cookieKey,
			Path:    "/",
			Expires: expiresAt,
		}
		http.SetCookie(w, cookie)
	}
	resp := &loginResponseJSON{}
	if doBearer {
		resp.BearerToken = base64.StdEncoding.EncodeToString(bearerKey)
	}
	www.SendJSON(w, resp)
}

func (c *ConfigDB) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		c.DB.Where("key = ?", HashSessionToken(cookie.Value)).Delete(&Session{})
	}
	www.SendOK(w)
}

// Returns the user id, or zero
// On failure, sends a 401 to 'w'
func (c *ConfigDB) MustGetUserID(w http.ResponseWriter, r *http.Request) int64 {
	userID := c.GetUserID(r, false)
	if userID == 0 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
	}
	return userID
}

// Returns the user or nil
func (c *ConfigDB) GetUser(r *http.Request) *User {
	userID := c.GetUserID(r, false)
	if userID == 0 {
		return nil
	}
	user := User{}
	if err := c.DB.Find(&user, userID).Error; err != nil {
		c.Log.Errorf("GetUser failed: %v", err)
		return nil
	}
	return &user
}

// Returns the user id, or zero.
// You should only set allowBasic to true if this is a rate limited endpoint.
func (c *ConfigDB) GetUserID(r *http.Request, allowBasic bool) int64 {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		session := Session{}
		c.DB.Where("key = ?", HashSessionToken(cookie.Value)).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
	}
	authorization := r.Header.Get("Authorization")
	clientPublicKey := r.Header.Get("X-PublicKey")
	clientNonce := r.Header.Get("X-Nonce")
	if strings.HasPrefix(authorization, "Bearer ") && clientPublicKey != "" && clientNonce != "" {
		// Bearer token
		tokenBase64 := authorization[7:]
		decryptedBearerToken := c.DecryptBearerToken(tokenBase64, clientPublicKey, clientNonce)
		if decryptedBearerToken != nil {
			session := Session{}
			c.DB.Where("key = ?", HashSessionToken(string(decryptedBearerToken))).Find(&session)
			if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
				return session.UserID
			}
		}
	}

	if allowBasic {
		username, password, haveBasic := r.BasicAuth()
		if haveBasic {
			user := User{}
			c.DB.Where("username_normalized = ?", NormalizeUsername(username)).Find(&user)
			if user.ID != 0 {
				if VerifyHash(password, user.Password) {
					return user.ID
				}
			}
		}
	}

	return 0
}

func (c *ConfigDB) PurgeExpiredSessions() {
	db, err := c.DB.DB()
	if err != nil {
		c.Log.Warnf("PurgeExpiredSessions failed (1): %v", err)
		return
	}
	_, err = db.Exec("DELETE FROM session WHERE expires_at < ?", time.Now().UnixMilli())
	if err != nil {
		c.Log.Warnf("PurgeExpiredSessions failed (2): %v", err)
	}
}

func (c *ConfigDB) NumAdminUsers() (int, error) {
	n := int64(0)
	if err := c.DB.Model(&User{}).Where("permissions LIKE ?", "%"+UserPermissionAdmin+"%").Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

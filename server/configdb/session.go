package configdb

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/www"
)

// SYNC-CYCLOPS-SESSION-COOKIE
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

// SYNC-LOGIN-RESPONSE-JSON
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
	doBearer := mode == LoginModeBearerToken || mode == LoginModeCookieAndBearerToken
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
	cookieToken := StrongRandomAlphaNumChars(30)
	bearerToken := StrongRandomBytes(32)
	if doCookie {
		cookieSession := Session{
			CreatedAt: dbh.MakeIntTime(now),
			Key:       HashSessionToken(cookieToken),
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
			Key:       HashSessionToken(string(bearerToken)),
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
			Value:   cookieToken,
			Path:    "/",
			Expires: expiresAt,
		}
		http.SetCookie(w, cookie)
	}
	resp := &loginResponseJSON{}
	if doBearer {
		resp.BearerToken = base64.StdEncoding.EncodeToString(bearerToken)
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
	sessionCookie := ""
	if cookie != nil {
		sessionCookie = cookie.Value
	} else {
		// Allow the session cookie to be specified as a header. This is a convenience added
		// for our Android app, which needs to check the status of it's session cookie
		// before deciding what to do next. Injecting a cookie into the webview is more
		// lines of Java code plus async callbacks, so sending a simple header is easier,
		// and that's the reason why we allow it.
		sessionCookie = r.Header.Get("X-Session-Cookie")
	}

	if sessionCookie != "" {
		session := Session{}
		c.DB.Where("key = ?", HashSessionToken(sessionCookie)).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
	}
	authorization := r.Header.Get("Authorization")
	//clientPublicKey := r.Header.Get("X-PublicKey")
	//clientNonce := r.Header.Get("X-Nonce")
	//if strings.HasPrefix(authorization, "Bearer ") && clientPublicKey != "" && clientNonce != "" {
	tokenBase64 := ""

	if strings.HasPrefix(authorization, "Bearer ") {
		// Bearer token
		tokenBase64 = authorization[7:]
	} else {
		tokenBase64 = r.URL.Query().Get("authorizationToken")
	}

	if tokenBase64 != "" {
		//decryptedBearerToken := c.DecryptBearerToken(tokenBase64, clientPublicKey, clientNonce)
		//if decryptedBearerToken != nil {
		token, _ := base64.StdEncoding.DecodeString(tokenBase64)
		session := Session{}
		c.DB.Where("key = ?", HashSessionToken(string(token))).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
		//}
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

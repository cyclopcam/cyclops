package configdb

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/pkg/rando"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/www"
)

// SYNC-CYCLOPS-SESSION-COOKIE
const SessionCookie = "session"

func (c *ConfigDB) Login(w http.ResponseWriter, r *http.Request) {
	userID := c.GetUserID(r, SpecialAuthMethodBASIC|SpecialAuthMethodIdentityToken)
	if userID == 0 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}
	expiresAtUnixMilli := www.QueryInt64(r, "expiresAt") // 0 = no expiry
	expiresAt := time.Time{}
	if expiresAtUnixMilli != 0 {
		expiresAt = time.UnixMilli(expiresAtUnixMilli)
	}
	c.LoginInternal(w, userID, expiresAt, www.QueryValue(r, "loginMode"), false)
}

// SYNC-LOGIN-RESPONSE-JSON
type loginResponseJSON struct {
	BearerToken string `json:"bearerToken"`
	NeedRestart bool   `json:"needRestart"` // Used during first login, if VPN is now available, that the server must be restarted to activate VPN.
}

const (
	LoginModeCookie               = "Cookie"
	LoginModeBearerToken          = "BearerToken"
	LoginModeCookieAndBearerToken = "CookieAndBearerToken"
)

// Special authentication methods, which take computation time on our side, or internet bandwidth.
// We want to rate limit these methods, or disallow them from the internet.
type SpecialAuthMethods int

const (
	SpecialAuthMethodBASIC         = 1 << iota // BASIC (username/password)
	SpecialAuthMethodIdentityToken             // Identity Token (validated by accounts.cyclopcam.org)
)

func (c *ConfigDB) LoginInternal(w http.ResponseWriter, userID int64, expiresAt time.Time, mode string, needRestart bool) {
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
	cookieToken := rando.StrongRandomAlphaNumChars(30)
	bearerToken := rando.StrongRandomBytes(32)
	if doCookie {
		cookieSession := Session{
			CreatedAt: dbh.MakeIntTime(now),
			Key:       pwdhash.HashSessionToken(cookieToken),
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
			Key:       pwdhash.HashSessionToken(string(bearerToken)),
			UserID:    userID,
			ExpiresAt: dbh.MakeIntTime(expiresAt),
		}
		if err := c.DB.Create(&bearerSession).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	c.PurgeExpiredSessions()
	c.Log.Infof("Logging %v in, needRestart: %v", userID, needRestart)
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
	resp := &loginResponseJSON{
		NeedRestart: needRestart,
	}
	if doBearer {
		resp.BearerToken = base64.StdEncoding.EncodeToString(bearerToken)
	}
	www.SendJSON(w, resp)
}

func (c *ConfigDB) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		c.DB.Where("key = ?", pwdhash.HashSessionToken(cookie.Value)).Delete(&Session{})
	}
	www.SendOK(w)
}

// Returns the user or nil.
func (c *ConfigDB) GetUser(r *http.Request, allowSpecial SpecialAuthMethods) *User {
	userID := c.GetUserID(r, allowSpecial)
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
func (c *ConfigDB) GetUserID(r *http.Request, allowSpecial SpecialAuthMethods) int64 {
	allowBasic := 0 != (allowSpecial & SpecialAuthMethodBASIC)
	allowIdentityToken := 0 != (allowSpecial & SpecialAuthMethodIdentityToken)

	// It is too dangerous to allow BASIC authentication from anywhere on the internet.
	if !c.IsCallerOnLAN(r) {
		allowBasic = false
	}

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
		c.DB.Where("key = ?", pwdhash.HashSessionToken(sessionCookie)).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
	}
	authorization := r.Header.Get("Authorization")
	tokenBase64 := ""

	if strings.HasPrefix(authorization, "Bearer ") {
		// Bearer token
		tokenBase64 = authorization[7:]
	} else {
		tokenBase64 = r.URL.Query().Get("authorizationToken")
	}

	if tokenBase64 != "" {
		token, _ := base64.StdEncoding.DecodeString(tokenBase64)
		session := Session{}
		c.DB.Where("key = ?", pwdhash.HashSessionToken(string(token))).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
	}

	if strings.HasPrefix(authorization, "IdentityToken ") {
		if allowIdentityToken {
			// Identity token from accounts.cyclopcam.org
			// NOTE: It's not necessarily correct to bind the user to the server just because he is logging in.
			// What we're really looking for is a way for the user to choose who is responsible for billing.
			// The initial user creation path is actually handled separately, inside httpAuthCreateUser(),
			// so we could replace this call with configdb.GetVerifiedIdentityFromToken(), and the 2nd and
			// 3rd users to login, would not be bound to the server. But maybe it's better to bind all
			// people who login, and the user with admin permissions must choose who's responsible for the
			// billing.
			verified, err := c.VerifyIdentityAndBindToServer(authorization[14:])
			if err == nil {
				user := User{}
				c.DB.Where("external_id = ?", verified.ID).Find(&user)
				return user.ID
			}
		} else {
			c.Log.Warnf("Identity token authentication is not allowed for this API call")
		}
	}

	if allowBasic {
		username, password, haveBasic := r.BasicAuth()
		if haveBasic && username != "" && password != "" {
			user := User{}
			c.DB.Where("username_normalized = ?", NormalizeUsername(username)).Find(&user)
			if user.ID != 0 {
				if pwdhash.VerifyHashBase64(password, user.Password) {
					return user.ID
				}
			}
		}
	} else if len(authorization) > 6 && strings.EqualFold(authorization[:6], "Basic ") {
		c.Log.Warnf("Basic authentication is not allowed from a VPN IP address")
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

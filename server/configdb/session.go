package configdb

import (
	"net/http"
	"time"

	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/www"
)

const SessionCookie = "session"

func (c *ConfigDB) Login(w http.ResponseWriter, r *http.Request) {
	userID := c.GetUserID(r)
	if userID == 0 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}
	expiresAtUnixMilli := www.QueryInt64(r, "expiresAt") // 0 = no expiry
	expiresAt := time.Time{}
	if expiresAtUnixMilli != 0 {
		expiresAt = time.UnixMilli(expiresAtUnixMilli)
	}
	c.LoginInternal(w, userID, expiresAt)
}

func (c *ConfigDB) LoginInternal(w http.ResponseWriter, userID int64, expiresAt time.Time) {
	key := StrongRandomAlphaNumChars(30)
	session := Session{
		Key:       HashSessionCookie(key),
		UserID:    userID,
		ExpiresAt: dbh.MakeIntTime(expiresAt),
	}
	if err := c.DB.Create(&session).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Log.Infof("Logging %v in", userID)
	//c.Log.Infof("Logging %v in. key: %v. hashed key hex: %v", userID, key, hex.EncodeToString(HashSessionCookie(key))) // only for debugging
	cookie := &http.Cookie{
		Name:  SessionCookie,
		Value: key,
		Path:  "/",
	}
	cookie.Expires = expiresAt
	http.SetCookie(w, cookie)
	c.PurgeExpiredSessions()
	www.SendOK(w)
}

func (c *ConfigDB) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		c.DB.Where("key = ?", HashSessionCookie(cookie.Value)).Delete(&Session{})
	}
	www.SendOK(w)
}

// Returns the user id, or zero
// On failure, sends a 401 to 'w'
func (c *ConfigDB) MustGetUserID(w http.ResponseWriter, r *http.Request) int64 {
	userID := c.GetUserID(r)
	if userID == 0 {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
	}
	return userID
}

// Returns the user or nil
func (c *ConfigDB) GetUser(r *http.Request) *User {
	userID := c.GetUserID(r)
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

// Returns the user id, or zero
func (c *ConfigDB) GetUserID(r *http.Request) int64 {
	cookie, _ := r.Cookie(SessionCookie)
	if cookie != nil {
		session := Session{}
		c.DB.Where("key = ?", HashSessionCookie(cookie.Value)).Find(&session)
		if session.UserID != 0 && (session.ExpiresAt.IsZero() || session.ExpiresAt.Get().After(time.Now())) {
			return session.UserID
		}
	}
	if username, password, ok := r.BasicAuth(); ok {
		user := User{}
		c.DB.Where("username_normalized = ?", NormalizeUsername(username)).Find(&user)
		if user.ID != 0 {
			if VerifyHash(password, user.Password) {
				return user.ID
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

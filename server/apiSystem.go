package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"os"
	"time"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SYNC-SYSTEM-INFO-JSON
type systemInfoJSON struct {
	ReadyError string         `json:"readyError,omitempty"` // If system is not yet ready to accept cameras, this will be populated
	Cameras    []*camInfoJSON `json:"cameras"`
}

// If this gets too bloated, then we can split it up
type constantsJSON struct {
	CameraModels []string `json:"cameraModels"`
}

type pingJSON struct {
	Greeting  string `json:"greeting"`
	Hostname  string `json:"hostname"`
	Time      int64  `json:"time"`
	PublicKey string `json:"publicKey"`
}

func (s *Server) httpSystemPing(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	hostname, _ := os.Hostname()
	ping := &pingJSON{
		Greeting:  "I am Cyclops", // This is used by the LAN scanner on our mobile app to find Cyclops servers, so it's part of our API.
		Hostname:  hostname,       // This is used by the LAN scanner on our mobile app to suggest a name
		Time:      time.Now().Unix(),
		PublicKey: s.configDB.PublicKey.String(), // This is used by the LAN scanner on our mobile app to identify a Cyclops server
	}
	www.SendJSON(w, ping)
}

// SYNC-KEYS-RESPONSE-JSON
type keysJSON struct {
	PublicKey string `json:"publicKey"`
	Proof     string `json:"proof"` // HMAC[SHA256](sharedSecret, challenge).  sharedSecret is from ECDH.
}

// This API is used to prove that we own the private key corresponding to our advertised public key.
// A client can use this before offering up the bearer token that it owns for this public key.
// Without this check, server B could impersonate server A by simply claiming the public key of server A.
// Then, a client might send through the bearer token that it knows for server A. Now server B
// has access to server A.
// Challenge is a 32 byte
func (s *Server) httpSystemKeys(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	publicKeyb64 := www.RequiredQueryValue(r, "publicKey")
	challengeb64 := www.RequiredQueryValue(r, "challenge")
	publicKey, err := wgtypes.ParseKey(publicKeyb64)
	www.Check(err)
	challenge, err := base64.StdEncoding.DecodeString(challengeb64)
	www.Check(err)

	shared := [32]byte{}
	curve25519.ScalarMult(&shared, (*[32]byte)(&s.configDB.PrivateKey), (*[32]byte)(&publicKey))

	mac := hmac.New(sha256.New, shared[:])
	mac.Write(challenge)
	hash := mac.Sum(nil)

	keys := &keysJSON{
		PublicKey: s.configDB.PublicKey.String(),
		Proof:     base64.StdEncoding.EncodeToString(hash[:]),
	}
	www.SendJSON(w, keys)
}

func (s *Server) httpSystemGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	j := systemInfoJSON{
		Cameras: make([]*camInfoJSON, 0),
	}
	if err := s.IsReady(); err != nil {
		j.ReadyError = err.Error()
	}
	cameras := []*configdb.Camera{}
	www.Check(s.configDB.DB.Find(&cameras).Error)

	for _, cfg := range cameras {
		cam := s.LiveCameras.CameraFromID(cfg.ID)
		if cam != nil {
			j.Cameras = append(j.Cameras, liveToCamInfoJSON(cam))
		} else {
			j.Cameras = append(j.Cameras, cfgToCamInfoJSON(cfg))
		}
	}
	www.SendJSON(w, &j)
}

func (s *Server) httpSystemRestart(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	www.SendText(w, "Restarting...")
	// We run the shutdown from a new goroutine so that this HTTP handler can return,
	// which tells the HTTP framework that this request is finished.
	// If we instead run Shutdown from this thread, then the system never sees us return,
	// so it thinks that we're still busy sending a response.
	go func() {
		s.Shutdown(true)
	}()
}

func (s *Server) httpSystemConstants(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	cams := []string{}
	for _, m := range camera.AllCameraModels {
		cams = append(cams, string(m))
	}
	c := &constantsJSON{
		CameraModels: cams,
	}
	www.SendJSON(w, c)
}

package proxy

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/go-chi/httprate"
	"github.com/julienschmidt/httprouter"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SYNC-CYCLOPS-SERVER-COOKIE
const CyclopsServerCookie = "CyclopsServerPublicKey"

func (p *Proxy) listenHTTP() error {
	router := httprouter.New()

	admin := func(method, route string, handle func(w http.ResponseWriter, r *http.Request), requestLimit int, windowLength time.Duration) {
		withAdmin := func(w http.ResponseWriter, r *http.Request) {
			username, password, _ := r.BasicAuth()
			h := sha256.Sum256([]byte(password))
			if username == "admin" && p.adminPasswordHash != nil && subtle.ConstantTimeCompare(p.adminPasswordHash, h[:]) == 1 {
				handle(w, r)
			} else {
				http.Error(w, "Forbidden", http.StatusForbidden)
			}
		}

		limited := httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(httprate.KeyByIP))

		www.Handle(p.log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			limited(http.HandlerFunc(withAdmin)).ServeHTTP(w, r)
		})

	}

	ratelimited := func(method, route string, handle func(w http.ResponseWriter, r *http.Request), requestLimit int, windowLength time.Duration) {
		// We don't need httprate.KeyByEndpoint, because we create a unique rate limiter for each endpoint.
		// That's not really the intended use case, but I only need it for a few endpoints, so I think this is OK.
		// The data structures in httprate looks decent, so we're hopefully not bloating up too much here.
		limited := httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(httprate.KeyByIP))

		www.Handle(p.log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			limited(http.HandlerFunc(handle)).ServeHTTP(w, r)
		})
	}

	// These are our own API entrypoints, which apply in the absense of a CyclopsServerCookie
	ratelimited("POST", "/api/register", p.httpRegister, 5, time.Minute)
	admin("POST", "/api/remove", p.httpRemove, 1, time.Second)

	front := func(w http.ResponseWriter, r *http.Request) {
		// If CyclopsServerCookie is specified, then forward request to that server
		serverCookie, _ := r.Cookie(CyclopsServerCookie)
		if serverCookie != nil {
			pubkey, err := wgtypes.ParseKey(serverCookie.Value)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid public key: %v", err), http.StatusBadRequest)
			} else {
				p.serveProxyRequest(w, r, pubkey)
			}
		} else {
			// Fall back to our own API
			router.ServeHTTP(w, r)
		}
	}

	p.log.Infof("Listening on %v", ProxyHttpPort)
	p.httpServer = &http.Server{
		Addr:    ProxyHttpPort,
		Handler: http.HandlerFunc(front),
	}
	return p.httpServer.ListenAndServe()
}

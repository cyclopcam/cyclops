package proxy

import (
	"net/http"

	"github.com/bmharper/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (p *Proxy) listenHTTP() error {
	router := httprouter.New()

	add := func(method, route string, handle httprouter.Handle) {
		www.Handle(p.log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			handle(w, r, params)
		})
	}

	// These are our own API entrypoints, which apply in the absense of a cyclopsserver cookie
	add("GET", "/api/setServer", p.httpSetServer)
	add("POST", "/api/register", p.httpRegister)

	front := func(w http.ResponseWriter, r *http.Request) {
		serverCookie, _ := r.Cookie(CyclopsServerCookie)
		if serverCookie != nil {
			pubkey, err := wgtypes.ParseKey(serverCookie.Value)
			if err == nil {
				p.serveProxyRequest(w, r, pubkey)
				return
			}
		}
		// fall back to our own API
		router.ServeHTTP(w, r)
	}

	p.log.Infof("Listening on %v", ProxyHttpPort)
	p.httpServer = &http.Server{
		Addr:    ProxyHttpPort,
		Handler: http.HandlerFunc(front),
	}
	return p.httpServer.ListenAndServe()
}

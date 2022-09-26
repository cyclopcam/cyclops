package proxy

import (
	"net/http"

	"github.com/bmharper/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
)

func (p *Proxy) listenHTTP() error {
	router := httprouter.New()

	add := func(method, route string, handle httprouter.Handle) {
		www.Handle(p.log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			handle(w, r, params)
		})
	}

	add("POST", "/api/register", p.httpRegister)

	// The NotFound handler is what actually performs the redirects for proxied calls.
	// The prefix is /proxy/
	router.NotFound = p // Point to Proxy.ServeHTTP(), which is in httpProxy.go

	p.log.Infof("Listening on %v", ProxyHttpPort)
	p.httpServer = &http.Server{
		Addr:    ProxyHttpPort,
		Handler: router,
	}
	return p.httpServer.ListenAndServe()
}

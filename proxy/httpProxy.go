package proxy

import (
	"net"
	"net/http"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const PublicKeyLen = 32

// We use a custom transport with shorter connection timeouts.
func createProxyTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
			//KeepAlive: 30 * time.Second, // Use default 15 seconds
		}).DialContext,
		MaxIdleConns:        1000,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
}

func (p *Proxy) serveProxyRequest(w http.ResponseWriter, r *http.Request, serverPublicKey wgtypes.Key) {
	vpnIP := p.getPeerIPFromCache(serverPublicKey[:])
	if vpnIP == "" {
		// SERVER_NOT_FOUND is recognized by the mobile app, and shows an appropriate error page.
		w.Header().Set("X-Cyclops-Proxy-Status", "SERVER_NOT_FOUND")
		http.Error(w, "Cyclops server not found in proxy database", http.StatusBadGateway)
		return
	}

	// Use these headers to side-load information that proxyDirector will use.
	// Also, note that the http.Request that proxyDirector sees is NOT the same
	// as the Request object that we're seeing here. Director gets a fresh Request.
	r.Header.Set("X-Cyclops-Server-IP", vpnIP)
	r.Header.Set("X-Cyclops-Path", r.URL.Path)

	//p.log.Infof("Redirecting to %v%v", vpnIP, r.URL.Path)

	// If Wireguard fails here, then the client also gets an http.StatusBadGateway (502), but the error body
	// is empty. The client interprets this differently to the above case, where we know that we haven't
	// heard from the targeted peer in a long time.
	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) proxyDirector(r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = r.Header.Get("X-Cyclops-Server-IP") + ServerHttpPort
	r.URL.Path = r.Header.Get("X-Cyclops-Path")
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	// If we get here, it means we've already found our peer in our database, so that's not the problem.
	// The only remaining problem is that we're unable to reach it.
	//p.log.Infof("errorHandler: %v", err)
	w.Header().Set("X-Cyclops-Proxy-Status", "SERVER_NOT_REACHABLE")
	w.WriteHeader(http.StatusBadGateway)
}

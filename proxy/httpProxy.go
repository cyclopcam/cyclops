package proxy

import (
	"net/http"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const PublicKeyLen = 32

func (p *Proxy) serveProxyRequest(w http.ResponseWriter, r *http.Request, serverPublicKey wgtypes.Key) {
	vpnIP := p.getPeerIPFromCache(serverPublicKey[:])
	if vpnIP == "" {
		http.Error(w, "Server not found", http.StatusBadGateway)
		return
	}

	// Use these headers to side-load information that proxyDirector will use.
	// I find it strange that proxyDirector is not allowed to return an error...
	// Also, note that the http.Request that proxyDirector sees is NOT the same
	// as the Request object that we're seeing here. Director gets a fresh Request.
	r.Header.Set("X-Cyclops-Server-IP", vpnIP)
	r.Header.Set("X-Cyclops-Path", r.URL.Path)

	//p.log.Infof("Redirecting to %v%v", vpnIP, r.URL.Path)

	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) proxyDirector(r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = r.Header.Get("X-Cyclops-Server-IP") + ServerHttpPort
	r.URL.Path = r.Header.Get("X-Cyclops-Path")
}

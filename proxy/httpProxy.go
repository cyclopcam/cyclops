package proxy

import (
	"net/http"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const PublicKeyLen = 32

func (p *Proxy) serveProxyRequest(w http.ResponseWriter, r *http.Request, serverPublicKey wgtypes.Key) {
	// Example incoming Path:
	// /proxy/ZO0qmRbISuPHSBIoZnC8sSDBkWrxsLxbiNXgGZIhKEE/api/camera/latestImage/1

	/*
		prefix := "/proxy/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Example path:
		// ZO0qmRbISuPHSBIoZnC8sSDBkWrxsLxbiNXgGZIhKEE/api/camera/latestImage/1
		path := r.URL.Path[len(prefix):]
		firstSlash := strings.IndexRune(path, '/')
		if firstSlash == -1 {
			firstSlash = len(path)
		}
		//if firstSlash == -1 {
		//	http.Error(w, "Invalid proxy path", http.StatusBadRequest)
		//	return
		//}
		publicKey := path[:firstSlash]
		key, err := base64.URLEncoding.DecodeString(publicKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid public key '%v': %v", publicKey, err), http.StatusBadRequest)
			return
		} else if len(key) != PublicKeyLen {
			http.Error(w, fmt.Sprintf("Invalid public key '%v': must be 32 bytes base64-url encoded", publicKey), http.StatusBadRequest)
			return
		}
	*/

	vpnIP := p.getPeerIPFromCache(serverPublicKey[:])
	if vpnIP == "" {
		http.Error(w, "Server not found", http.StatusBadGateway)
		return
	}

	//r.Host = vpnIP + ServerHttpPort
	// testing...
	//vpnIP = "192.168.10.11"

	// Example forwardPath:
	// /api/camera/latestImage/1
	//forwardPath := path[firstSlash:]
	//r.URL.Path = forwardPath

	forwardPath := r.URL.Path

	// Use these headers to side-load information that proxyDirector will use.
	// I find it strange that proxyDirector is not allowed to return an error...
	// Also, to be clear, the http.Request that proxyDirector sees is NOT the same
	// as the Request object that we're seeing here. Director gets a fresh Request.
	r.Header.Set("X-Cyclops-Server-IP", vpnIP)
	r.Header.Set("X-Cyclops-Path", forwardPath)

	p.log.Infof("Redirecting to %v%v", vpnIP, forwardPath)

	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) proxyDirector(r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = r.Header.Get("X-Cyclops-Server-IP") + ServerHttpPort
	r.URL.Path = r.Header.Get("X-Cyclops-Path")
	//fmt.Printf("r.URL: %v\n", r.URL)
}

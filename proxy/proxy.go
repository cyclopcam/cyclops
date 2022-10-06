package proxy

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/log"
	"gorm.io/gorm"
)

const ProxyHttpPort = "127.0.0.1:8082" // In production we run on 443, but we use Docker there, and still run on 8082 inside Docker
const ServerHttpPort = ":8080"         // SYNC-SERVER-PORT. Servers always run on 8080, but we could make this configurable

type Proxy struct {
	log          log.Log
	db           *gorm.DB
	wg           *wireGuard
	httpServer   *http.Server
	reverseProxy *httputil.ReverseProxy
	kernelwgHost string

	addPeerLock     sync.Mutex
	lastPeerAddedAt time.Time

	pubkeyToIPCacheLock sync.Mutex
	pubkeyToIPCache     map[string]string // Map from public key to VPN IP. Key is raw 32 bytes cast to string.

	ipPoolLock sync.Mutex
}

type ProxyConfig struct {
	Log          log.Log
	DB           dbh.DBConfig
	KernelWGHost string
}

func NewProxy() *Proxy {
	return &Proxy{
		pubkeyToIPCache: map[string]string{},
	}
}

// Start the proxy server
func (p *Proxy) Start(config ProxyConfig) error {
	p.log = config.Log
	p.kernelwgHost = config.KernelWGHost
	//db, err := dbh.OpenDB(config.Log, config.DB, Migrations(config.Log), dbh.DBConnectFlagWipeDB)
	db, err := dbh.OpenDB(config.Log, config.DB, Migrations(config.Log), 0)
	if err != nil {
		return err
	}
	p.db = db

	wg, err := newWireGuard(p)
	if err != nil {
		return fmt.Errorf("Error connecting to kernelwg: %w", err)
	}
	p.wg = wg
	if err := p.wg.boot(); err != nil {
		return fmt.Errorf("Error booting kernelwg: %w", err)
	}

	//printDummyKeys()

	if err := p.rebuildCache(); err != nil {
		return fmt.Errorf("Error rebuilding cache: %w", err)
	}

	p.reverseProxy = &httputil.ReverseProxy{}
	p.reverseProxy.Director = p.proxyDirector

	return p.listenHTTP()
}

func printDummyKeys() {
	foo := [32]byte{}
	for i := 0; i < 32; i++ {
		foo[i] = byte(i + 195)
		//foo[i] = byte(i + 200)
	}
	fmt.Printf("%v\n", base64.StdEncoding.EncodeToString(foo[:]))
	fmt.Printf("%v\n", base64.URLEncoding.EncodeToString(foo[:]))
}
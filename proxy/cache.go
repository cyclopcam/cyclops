package proxy

func (p *Proxy) rebuildCache() error {
	p.pubkeyToIPCacheLock.Lock()
	defer p.pubkeyToIPCacheLock.Unlock()

	servers := []Server{}
	if err := p.db.Find(&servers).Error; err != nil {
		return err
	}

	m := map[string]string{}
	for _, s := range servers {
		m[string(s.PublicKey)] = s.VpnIP
	}
	p.pubkeyToIPCache = m

	return nil
}

func (p *Proxy) addPeerToCache(publicKey []byte, vpnIP string) {
	p.pubkeyToIPCacheLock.Lock()
	defer p.pubkeyToIPCacheLock.Unlock()

	p.pubkeyToIPCache[string(publicKey)] = vpnIP
}

func (p *Proxy) getPeerIPFromCache(publicKey []byte) string {
	p.pubkeyToIPCacheLock.Lock()
	defer p.pubkeyToIPCacheLock.Unlock()

	return p.pubkeyToIPCache[string(publicKey)]
}

func (p *Proxy) removePeerFromCache(publicKey []byte) {
	p.pubkeyToIPCacheLock.Lock()
	defer p.pubkeyToIPCacheLock.Unlock()

	delete(p.pubkeyToIPCache, string(publicKey))
}

package proxy

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"

	"gorm.io/gorm"
)

// Our proxy is always at this address
const ProxyAddr = "10.6.0.0"

// Servers start at this address
const ServerStartAddr = "10.7.0.0"

// Servers end at this address (exclusive)
const ServerEndAddr = "10.255.255.255"

func (p *Proxy) findFreeIP(tx *gorm.DB) (string, error) {
	p.ipPoolLock.Lock()
	defer p.ipPoolLock.Unlock()

	free := IPFreePool{}
	err := tx.First(&free).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	if free.VpnIP != "" {
		p.log.Infof("Using IP %v from free pool", free.VpnIP)
		if err := tx.Where("vpn_ip = ?", free.VpnIP).Delete(&free).Error; err != nil {
			return "", fmt.Errorf("Failed to delete free IP: %w", err)
		}
		return free.VpnIP, nil
	}

	// Create more IPs
	p.log.Infof("Creating more free IPs")
	type result struct {
		VpnIP string
	}
	results := []result{}
	if err := tx.Raw("SELECT vpn_ip FROM server").Scan(&results).Error; err != nil {
		return "", err
	}

	// Create new IPs
	nCreate := 100

	existing := []netip.Addr{}
	for _, r := range results {
		ip, err := netip.ParseAddr(r.VpnIP)
		if err != nil {
			return "", fmt.Errorf("Invalid IP '%v' in database: %w", r.VpnIP, err)
		}
		existing = append(existing, ip)
	}
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Less(existing[j])
	})

	newFreePool := createNewIPs(nCreate, netip.MustParseAddr(ServerStartAddr), netip.MustParseAddr(ServerEndAddr), existing)
	if len(newFreePool) == 0 {
		return "", fmt.Errorf("IP address range exhausted")
	}

	// Take the first IP for the caller
	first := newFreePool[0]
	newFreePool = newFreePool[1:]
	p.log.Infof("Using new IP %v", first.String())

	// And insert the rest into the free pool, for subsequent callers
	if len(newFreePool) != 0 {
		freeRecords := []IPFreePool{}
		for _, ip := range newFreePool {
			freeRecords = append(freeRecords, IPFreePool{VpnIP: ip.String()})
		}
		if err := tx.Create(freeRecords).Error; err != nil {
			return "", err
		}
	}

	return first.String(), nil
}

// Creates new IPs, starting from 'start', and ending before 'end' (i.e. end is exclusive), and avoiding any existing IPs.
// existing must be sorted.
// If the limit is reached, then we simply return as many as we could generate.
// We assume that existing IPs are within start..end.
func createNewIPs(nCreate int, start, end netip.Addr, existing []netip.Addr) []netip.Addr {
	result := []netip.Addr{}

	// If there are no addresses, then just go for it
	if len(existing) == 0 {
		ip := start
		for i := 0; i < nCreate; i++ {
			result = append(result, ip)
			ip = ip.Next()
			if ip == end {
				break
			}
		}
		return result
	}

	// If there's a gap at the start, then fill that in
	if start.Less(existing[0]) {
		ip := start
		for len(result) < nCreate && ip != existing[0] {
			result = append(result, ip)
			ip = ip.Next()
		}
	}

	prev := existing[0]
	for i := 0; i < len(existing) && len(result) < nCreate; i++ {
		// Add gaps between previous and current
		prev = prev.Next()
		for prev.Less(existing[i]) && len(result) < nCreate {
			result = append(result, prev)
			prev = prev.Next()
		}
		prev = existing[i]
	}

	// Add fresh high addresses
	ip := existing[len(existing)-1]
	for len(result) < nCreate {
		ip = ip.Next()
		if !ip.Less(end) {
			// Out of addresses
			break
		}
		result = append(result, ip)
	}

	return result
}

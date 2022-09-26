package proxy

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarvestIP(t *testing.T) {
	do := func(nCreate int, start, end string, existing []string, expect []string) {
		ex := []netip.Addr{}
		for _, e := range existing {
			ex = append(ex, netip.MustParseAddr(e))
		}
		res := createNewIPs(nCreate, netip.MustParseAddr(start), netip.MustParseAddr(end), ex)
		assert.Equal(t, len(expect), len(res))
		for i := 0; i < len(res); i++ {
			assert.Equal(t, res[i].String(), expect[i])
		}
	}

	// empty existing set
	do(1, "10.7.0.0", "10.255.255.254", []string{}, []string{"10.7.0.0"})
	do(2, "10.7.0.0", "10.255.255.254", []string{}, []string{"10.7.0.0", "10.7.0.1"})
	do(2, "10.7.0.255", "10.255.255.254", []string{}, []string{"10.7.0.255", "10.7.1.0"})

	// gap at start
	do(1, "10.7.0.0", "10.255.255.254", []string{"10.7.0.2"}, []string{"10.7.0.0"})
	do(2, "10.7.0.0", "10.255.255.254", []string{"10.7.0.2"}, []string{"10.7.0.0", "10.7.0.1"})

	// and now we need to skip over existing
	do(3, "10.7.0.0", "10.255.255.254", []string{"10.7.0.2"}, []string{"10.7.0.0", "10.7.0.1", "10.7.0.3"})
	do(3, "10.7.0.0", "10.255.255.254", []string{"10.7.0.2", "10.7.0.3"}, []string{"10.7.0.0", "10.7.0.1", "10.7.0.4"})
	do(4, "10.7.0.0", "10.255.255.254", []string{"10.7.0.2", "10.7.0.4"}, []string{"10.7.0.0", "10.7.0.1", "10.7.0.3", "10.7.0.5"})

	do(2, "10.7.0.0", "10.255.255.254", []string{"10.7.0.0", "10.7.0.1"}, []string{"10.7.0.2", "10.7.0.3"})

	// hitting the limit (run out of addresses)
	do(10, "10.255.255.253", "10.255.255.255", []string{"10.255.255.253"}, []string{"10.255.255.254"})
}

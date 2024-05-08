package idgen

import "sync/atomic"

// Uint32 returns values 1,2,3... up to 2^32-1, then wraps around to 1.
// Zero is never generated.
type Uint32 struct {
	next atomic.Uint32
}

func (u *Uint32) Next() uint32 {
	n := u.next.Add(1)
	if n == 0 {
		n = u.next.Add(1)
	}
	return n
}

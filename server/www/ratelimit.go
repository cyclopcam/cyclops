package www

import (
	"net/http"
	"time"
)

// You can disable this when running unit tests
var EnableRateLimiting bool

//var rateLimitLock sync.Mutex
//var rateLimitGroups map[string]time.Time

// This is simple, dumb, and wrong, but at least the intention is clearer than having time.Sleep() all over the place
// The key things broken here are:
// 1. We don't pay attention to who's calling
// 2. We always sleep
// 3. It's trivial to get around this by firing off 10000 simultaneous requests
func RateLimit(groupName string, maxPerSecond float64, w http.ResponseWriter, r *http.Request) {
	if EnableRateLimiting {
		/*
			rateLimitLock.Lock()
			lastCall := rateLimitGroups[groupName]
			sleepDuration := time.Duration(0)
			if !lastCall.IsZero() {
				delay := time.Nanosecond * time.Duration(1000*1000*1000/maxPerSecond)
				nextCallAt := lastCall.Add(delay)
				now := time.Now()
				if now.Before(nextCallAt) {
					sleepDuration = nextCallAt.Sub(now)
				}
			}
			rateLimitLock.Unlock()
			if sleepDuration.Seconds() != 0 {
				fmt.Printf("Rate-limiter sleeping for %v\n", sleepDuration)
				time.Sleep(sleepDuration)
			} else {
				// delay all calls by 1/10 of the
			}
		*/
		delay := time.Nanosecond * time.Duration(1000*1000*1000/maxPerSecond)
		time.Sleep(delay)
	}
}

func init() {
	EnableRateLimiting = true
	//rateLimitGroups = map[string]time.Time{}
}

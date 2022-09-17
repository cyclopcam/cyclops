package www

import "time"

// Try to execute a function multiple times, and return the first error (or nil upon success)
// We backoff exponentially with 1s, 2s, 4s, 8s, etc.
func TryMultipleTimes(maxAttempts int, action func() error) error {
	var firstError error
	pause := time.Second
	for i := 0; i < maxAttempts; i++ {
		err := action()
		if err == nil {
			return err
		} else if i == 0 {
			firstError = err
		}
		time.Sleep(pause)
		pause *= 2
	}
	return firstError
}

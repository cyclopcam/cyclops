package auth

import "errors"

func IsPasswordOK(password string) error {
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters")
	}
	return nil
}

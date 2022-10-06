package configdb

import (
	"crypto/rand"
	"encoding/base64"
)

// This is 62 symbols, hence 5.9542 bits per character
// At 20 characters, that's 119 bits
// At 24 characters, that's 142 bits
// At 32 characters, that's 190 bits
const alphaNumChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const digitChars = "0123456789"

func StrongRandomAlphaNumChars(nchars int) string {
	buf := make([]byte, nchars)
	if n, _ := rand.Read(buf[:]); n != nchars {
		panic("Unable to read from crypto/rand")
	}
	for i := 0; i < nchars; i++ {
		buf[i] = alphaNumChars[buf[i]%byte(len(alphaNumChars))]
	}
	return string(buf)
}

func StrongRandomBytes(nbytes int) []byte {
	buf := make([]byte, nbytes)
	if n, _ := rand.Read(buf[:]); n != nbytes {
		panic("Unable to read from crypto/rand")
	}
	return buf
}

func StrongRandomBase64(nbytes int) string {
	buf := make([]byte, nbytes)
	if n, _ := rand.Read(buf[:]); n != nbytes {
		panic("Unable to read from crypto/rand")
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func StrongRandomDigits(nchars int) string {
	buf := make([]byte, nchars)
	if n, _ := rand.Read(buf[:]); n != nchars {
		panic("Unable to read from crypto/rand")
	}
	for i := 0; i < nchars; i++ {
		buf[i] = digitChars[buf[i]%byte(len(digitChars))]
	}
	return string(buf)
}

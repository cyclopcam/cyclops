package pwdhash

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

// Our hash is 1 byte of version, followed by 20 bytes of salt, followed by 32 bytes of scrypt.

// scrypt(16384,8,1) is 36 ms on a Skylake 6700K
const hashVersion1 = 1
const saltSizeV1 = 20
const scryptHashSizeV1 = 32
const scryptNV1 = 16384
const scryptrV1 = 8
const scryptpV1 = 1
const hashLenV1 = 1 + saltSizeV1 + scryptHashSizeV1

// Returns a saltSizeV1 salt
func createSalt() []byte {
	s := [saltSizeV1]byte{}
	if n, _ := rand.Read(s[:]); n != saltSizeV1 {
		panic("Error creating password salt")
	}
	return s[:]
}

// Returns a hashLenV1 byte key
func hashPasswordWithSalt(salt []byte, password string) []byte {
	dk, err := scrypt.Key([]byte(password), salt, scryptNV1, scryptrV1, scryptpV1, scryptHashSizeV1)
	if err != nil {
		panic(fmt.Sprintf("Error hashing password: %v", err))
	}
	final := [hashLenV1]byte{}
	final[0] = hashVersion1
	copy(final[1:1+saltSizeV1], salt)
	copy(final[1+saltSizeV1:1+saltSizeV1+scryptHashSizeV1], dk)
	return final[:]
}

// Create a random salt, and return fully baked hash, of length hashLenV1
func HashPassword(password string) []byte {
	return hashPasswordWithSalt(createSalt(), password)
}

// Return base64 encoding of HashPassword
func HashPasswordBase64(password string) string {
	return base64.RawStdEncoding.EncodeToString(HashPassword(password))
}

// Returns true if a plaintext password matches a stored hash
func VerifyHash(password string, hash []byte) bool {
	if len(hash) != hashLenV1 {
		return false
	}
	salt := hash[1 : 1+saltSizeV1]
	dk, _ := scrypt.Key([]byte(password), salt, scryptNV1, scryptrV1, scryptpV1, scryptHashSizeV1)
	return subtle.ConstantTimeCompare(dk, hash[1+saltSizeV1:1+saltSizeV1+scryptHashSizeV1]) == 1
}

// Returns true if a plaintext password matches a stored hash, in base64
func VerifyHashBase64(password string, hashb64 string) bool {
	raw, _ := base64.RawStdEncoding.DecodeString(hashb64)
	return VerifyHash(password, raw)
}

// Hash the session token to safeguard against timing attacks (eg in the DB's BTree lookup)
// The caller gets the plaintext value, and that is the ONLY place where the plaintext lives.
func HashSessionToken(value string) []byte {
	h := sha256.Sum256([]byte(value))
	return h[:]
}

// Return the base64 encoding of the hash of the session token
func HashSessionTokenBase64(value string) string {
	return base64.RawStdEncoding.EncodeToString(HashSessionToken(value))
}

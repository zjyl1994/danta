// Package securetoken provides cryptographically secure token encoders.
package securetoken

import (
	"crypto/rand"
	"encoding/hex"
)

const akAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// Hex returns nBytes of cryptographically secure random data as lowercase hex.
func Hex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Key returns a cloud-access-key-style secret (e.g. "DTAKq3f9...").
func Key() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = akAlphabet[int(b[i])%len(akAlphabet)]
	}
	return "DTAK" + string(b), nil
}

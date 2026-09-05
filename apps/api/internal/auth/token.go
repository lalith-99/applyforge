package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// sessionTokenBytes is the amount of entropy used for session tokens.
const sessionTokenBytes = 32

// newSessionToken generates a new random, URL-safe session token. The raw
// token is what gets set in the client's cookie; only its hash is stored.
func newSessionToken() (string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken returns a hex-encoded SHA-256 hash of a session token, suitable
// for storage/lookup without persisting the raw token value.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

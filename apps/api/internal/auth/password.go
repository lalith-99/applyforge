// Package auth implements email/password and Google OAuth authentication,
// backed by opaque, database-stored sessions (see docs/SECURITY.md).
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plaintext password for storage.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether password matches the stored hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

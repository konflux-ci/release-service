package utils

import (
	"crypto/rand"
	"math/big"
	"os"
)

// GetEnv retrieves an environment variable with a default fallback.
func GetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GenerateRandomString generates a random lowercase alphanumeric string of given length.
func GenerateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[num.Int64()]
	}
	return string(b)
}

// GetGeneratedNamespace creates a unique namespace name.
func GetGeneratedNamespace(name string) string {
	return name + "-" + GenerateRandomString(4)
}

package crypto

import (
	"encoding/base64"
	"crypto/rand"
)


func GenerateRandomKeyString(keySize int) (string, error) {
	b, err := generateRandomBytes(keySize)
	return base64.URLEncoding.EncodeToString(b), err
}

// generate random key with specific size
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}
	return b, nil
}

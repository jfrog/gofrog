package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

func GenerateRandomKeyString(keySize int) (string, error) {
	b, err := generateRandomBytes(keySize)
	return hex.EncodeToString(b), err
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

// keyId is first 6 chars of hashed(sha256) signing key
func GenerateKeyId(key string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("signing key is empty")
	}
	h := sha256.New()
	h.Write([]byte(key))
	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256[:6], nil
}

package crypto

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

var signingKey, keyId string

func init() {
	var err error
	signingKey, err = GenerateRandomKeyString(32)
	if err != nil {
		panic("Failed to generate signingKey")
	}
	h := sha256.New()
	sha256 := fmt.Sprintf("%x", (h.Sum(nil)))
	keyId = sha256[:6]
}

func TestDecryptEncryptWithValidKey(t *testing.T) {
	var text = "Text to encrypt"
	cipherText, err := Encrypt(text, signingKey, keyId)
	if err != nil {
		t.Fatal(err)
	}
	clearText, err := Decrypt(cipherText, signingKey, keyId)
	if err != nil {
		t.Fatal(err)
	}
	if clearText != "Text to encrypt" {
		t.Fatal("Expect cipher text to match 8001f1$aes256$ArMu9srTA6prKSoIYLctw87TQy7xX6tex1heE43QH7NAgGr4Z-TjA1sFrw==")
	}
}

func TestDecryptUnformattedCipherText(t *testing.T) {
	var text = "Text to encrypt"
	FormattedCipherText, err := Encrypt(text, signingKey, keyId)
	if err != nil {
		t.Fatal(err)
	}
	formatEncryption := keyId + "$" + "aes256" + "$"
	//keep cipher text only
	cipherText := strings.Replace(FormattedCipherText, formatEncryption, "", -1)
	_, err = Decrypt(cipherText, signingKey, keyId)
	if err == nil || (err != nil && err.Error() != "cipher text is not well formatted") {
		t.Fatal("Expect error Cipher text is not well formatted")
	}
}

func TestIsTextEncrypted(t *testing.T) {
	var text = "Text to encrypt with very long text"
	formatEncryption := keyId + "$" + "aes256" + "$" + text
	//keep cipher text only
	isEncrypted, err := IsTextEncrypted(formatEncryption, signingKey, keyId)
	if isEncrypted {
		t.Fatal(err)
	}
}

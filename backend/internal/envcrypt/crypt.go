package envcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Box encrypts project env values at rest.
type Box struct {
	gcm cipher.AEAD
}

func FromEnv() (*Box, error) {
	key := strings.TrimSpace(os.Getenv("ENV_ENCRYPTION_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	}
	if key == "" {
		return nil, errors.New("ENV_ENCRYPTION_KEY or SESSION_SECRET is required for project env vars")
	}
	return New(key)
}

func New(secret string) (*Box, error) {
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

func (b *Box) Seal(plain string) ([]byte, error) {
	if b == nil || b.gcm == nil {
		return nil, errors.New("encryption box is not configured")
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return b.gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (b *Box) Open(ciphertext []byte) (string, error) {
	if b == nil || b.gcm == nil {
		return "", errors.New("encryption box is not configured")
	}
	ns := b.gcm.NonceSize()
	if len(ciphertext) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := b.gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

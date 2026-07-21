package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

func Encrypt(masterKey, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return "enc:v1:" + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func Decrypt(masterKey, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) < len("enc:v1:") || value[:len("enc:v1:")] != "enc:v1:" {
		return "", errors.New("secret is not encrypted")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value[len("enc:v1:"):])
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("secret is truncated")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func Mask(value string) string {
	characters := []rune(value)
	if len(characters) <= 4 {
		return "••••"
	}
	return string(characters[:2]) + "••••" + string(characters[len(characters)-2:])
}

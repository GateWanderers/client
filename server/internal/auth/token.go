package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	tokenPrefix = "v2.local."
	tokenTTL    = 7 * 24 * time.Hour
)

// claims is the internal structure of PASETO v2 local token payload.
type claims struct {
	Sub string `json:"sub"`
	IAT int64  `json:"iat"`
	EXP int64  `json:"exp"`
}

// TokenMaker handles PASETO v2 local token creation and verification.
type TokenMaker struct {
	key []byte
}

// NewTokenMaker creates a TokenMaker with the provided 32-byte symmetric key.
func NewTokenMaker(key []byte) (*TokenMaker, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("key must be exactly %d bytes", chacha20poly1305.KeySize)
	}
	k := make([]byte, len(key))
	copy(k, key)
	return &TokenMaker{key: k}, nil
}

// CreateToken generates a new PASETO v2 local token for the given account ID.
func (m *TokenMaker) CreateToken(accountID string) (string, error) {
	now := time.Now().UTC()
	c := claims{
		Sub: accountID,
		IAT: now.Unix(),
		EXP: now.Add(tokenTTL).Unix(),
	}

	payload, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("json.Marshal claims: %w", err)
	}

	aead, err := chacha20poly1305.NewX(m.key)
	if err != nil {
		return "", fmt.Errorf("chacha20poly1305.NewX: %w", err)
	}

	nonce := make([]byte, aead.NonceSize()) // 24 bytes for XChaCha20
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand.Read nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, payload, nil)

	// Token format: "v2.local." + base64url(nonce || ciphertext)
	combined := append(nonce, ciphertext...) //nolint:gocritic
	encoded := base64.RawURLEncoding.EncodeToString(combined)
	return tokenPrefix + encoded, nil
}

// VerifyToken validates a PASETO v2 local token and returns the account ID on success.
func (m *TokenMaker) VerifyToken(token string) (string, error) {
	if !strings.HasPrefix(token, tokenPrefix) {
		return "", errors.New("invalid token format")
	}

	encoded := token[len(tokenPrefix):]
	combined, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	aead, err := chacha20poly1305.NewX(m.key)
	if err != nil {
		return "", fmt.Errorf("chacha20poly1305.NewX: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(combined) < nonceSize {
		return "", errors.New("token too short")
	}

	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	payload, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("token decryption failed")
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return "", fmt.Errorf("json.Unmarshal claims: %w", err)
	}

	if time.Now().UTC().Unix() > c.EXP {
		return "", errors.New("token has expired")
	}

	return c.Sub, nil
}

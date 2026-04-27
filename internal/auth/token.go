package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Subject   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), ttl: ttl}
}

func (s *TokenService) Issue(subject string) (string, error) {
	now := time.Now().UTC()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	payload := map[string]any{
		"sub": subject,
		"iat": now.Unix(),
		"exp": now.Add(s.ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	unsigned := encode(headerJSON) + "." + encode(payloadJSON)
	return unsigned + "." + s.sign(unsigned), nil
}

func (s *TokenService) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(unsigned))) {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var payload struct {
		Subject string `json:"sub"`
		Issued  int64  `json:"iat"`
		Expires int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, ErrInvalidToken
	}
	if payload.Subject == "" || payload.Expires == 0 {
		return nil, ErrInvalidToken
	}
	expiresAt := time.Unix(payload.Expires, 0).UTC()
	if !time.Now().UTC().Before(expiresAt) {
		return nil, ErrInvalidToken
	}
	return &Claims{
		Subject:   payload.Subject,
		IssuedAt:  time.Unix(payload.Issued, 0).UTC(),
		ExpiresAt: expiresAt,
	}, nil
}

func (s *TokenService) sign(unsigned string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(unsigned))
	return encode(mac.Sum(nil))
}

func encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

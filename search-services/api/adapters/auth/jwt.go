package auth

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Service struct {
	secret  []byte
	ttl     time.Duration
	subject string
}

func New(ttl time.Duration) (*Service, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return &Service{
		secret:  secret,
		ttl:     ttl,
		subject: "superuser",
	}, nil
}

func (s *Service) IssueToken() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   s.subject,
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *Service) ParseToken(tok string) error {
	parsed, err := jwt.ParseWithClaims(tok, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return errors.New("invalid token")
	}
	if claims.Subject != s.subject {
		return errors.New("wrong subject")
	}
	return nil
}

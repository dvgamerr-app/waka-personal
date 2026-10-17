package service

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
)

var ErrUnauthorized = errors.New("unauthorized")

type AuthService struct {
	expectedAPIKey string
}

func NewAuthService(expectedAPIKey string) *AuthService {
	return &AuthService{expectedAPIKey: expectedAPIKey}
}

func (s *AuthService) Validate(queryAPIKey, authorization string) error {
	if s.expectedAPIKey == "" {
		return nil
	}

	if queryAPIKey != "" && s.matches(queryAPIKey) {
		return nil
	}

	if authorization == "" {
		return ErrUnauthorized
	}

	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "basic") {
		return ErrUnauthorized
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return ErrUnauthorized
	}

	credentials := string(decoded)
	userPass := strings.SplitN(credentials, ":", 2)
	if s.matches(userPass[0]) {
		return nil
	}
	if len(userPass) == 2 && s.matches(userPass[1]) {
		return nil
	}
	return ErrUnauthorized
}

// matches compares a presented key with the configured one without leaking
// where the first mismatch occurs.
func (s *AuthService) matches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.expectedAPIKey)) == 1
}

package service

import (
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

	if queryAPIKey != "" && queryAPIKey == s.expectedAPIKey {
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
	if len(userPass) == 0 {
		return ErrUnauthorized
	}
	if userPass[0] == s.expectedAPIKey {
		return nil
	}
	if len(userPass) == 2 && userPass[1] == s.expectedAPIKey {
		return nil
	}
	return ErrUnauthorized
}

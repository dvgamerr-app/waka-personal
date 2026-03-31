package service

import "testing"

func TestAuthServiceValidate(t *testing.T) {
	t.Run("query api key", func(t *testing.T) {
		auth := NewAuthService("secret")
		if err := auth.Validate("secret", ""); err != nil {
			t.Fatalf("expected query key to authorize, got %v", err)
		}
	})

	t.Run("basic auth username", func(t *testing.T) {
		auth := NewAuthService("secret")
		if err := auth.Validate("", "Basic c2VjcmV0Og=="); err != nil {
			t.Fatalf("expected basic auth username to authorize, got %v", err)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		auth := NewAuthService("secret")
		if err := auth.Validate("wrong", ""); err == nil {
			t.Fatal("expected invalid key to fail")
		}
	})
}

package service

import (
	"context"
	"encoding/json"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
	"waka-personal/internal/store"
)

type ProfileService struct {
	store *store.Store
	cfg   *config.Config
}

func NewProfileService(store *store.Store, cfg *config.Config) *ProfileService {
	return &ProfileService{store: store, cfg: cfg}
}

func (s *ProfileService) EffectiveSnapshot(ctx context.Context) (*domain.ProfileSnapshot, error) {
	snapshot, err := s.store.GetProfileSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if snapshot != nil {
		return snapshot, nil
	}

	timeout := s.cfg.KeystrokeTimeoutMinutes
	writesOnly := s.cfg.WritesOnly
	profileJSON, _ := json.Marshal(map[string]any{
		"username":     s.cfg.ProfileUsername,
		"display_name": s.cfg.ProfileDisplayName,
		"full_name":    s.cfg.ProfileFullName,
		"email":        s.cfg.ProfileEmail,
		"photo":        s.cfg.ProfilePhotoURL,
		"profile_url":  s.cfg.ProfileProfileURL,
		"timezone":     s.cfg.AppTimezone,
		"timeout":      timeout,
		"writes_only":  writesOnly,
	})

	return &domain.ProfileSnapshot{
		Username:       s.cfg.ProfileUsername,
		DisplayName:    s.cfg.ProfileDisplayName,
		FullName:       s.cfg.ProfileFullName,
		Email:          s.cfg.ProfileEmail,
		Photo:          s.cfg.ProfilePhotoURL,
		ProfileURL:     s.cfg.ProfileProfileURL,
		Timezone:       s.cfg.AppTimezone,
		TimeoutMinutes: &timeout,
		WritesOnly:     &writesOnly,
		City:           []byte("null"),
		ProfileJSON:    profileJSON,
	}, nil
}

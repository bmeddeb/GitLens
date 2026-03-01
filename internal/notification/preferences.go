package notification

import (
	"context"
)

type PreferenceService interface {
	GetPreferences(ctx context.Context, userID uint64) ([]NotificationPreference, error)
	UpsertPreference(ctx context.Context, pref *NotificationPreference) error
	GetDeliveryMode(ctx context.Context, userID uint64, eventType string) (string, bool, error)
}

type preferenceService struct {
	repo Repository
}

func NewPreferenceService(repo Repository) PreferenceService {
	return &preferenceService{repo: repo}
}

func (s *preferenceService) GetPreferences(ctx context.Context, userID uint64) ([]NotificationPreference, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *preferenceService) UpsertPreference(ctx context.Context, pref *NotificationPreference) error {
	return s.repo.UpsertPreference(ctx, pref)
}

// GetDeliveryMode returns the delivery mode and suppress_self setting for a user+event.
// Falls back to wildcard (*) preference if no specific one exists.
// Defaults to instant+suppress_self=true if no preference at all.
func (s *preferenceService) GetDeliveryMode(ctx context.Context, userID uint64, eventType string) (string, bool, error) {
	pref, err := s.repo.GetPreference(ctx, userID, eventType)
	if err != nil {
		return "", false, err
	}

	if pref == nil {
		return "instant", true, nil
	}

	return pref.DeliveryMode, pref.SuppressSelf, nil
}

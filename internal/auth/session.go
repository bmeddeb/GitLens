package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionStore interface {
	Create(ctx context.Context, userID uint64, duration time.Duration) (*Session, error)
	Get(ctx context.Context, sessionID string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) error
	Touch(ctx context.Context, sessionID string) error
}

type sessionStore struct {
	db *gorm.DB
}

func NewSessionStore(db *gorm.DB) SessionStore {
	return &sessionStore{db: db}
}

func (s *sessionStore) Create(ctx context.Context, userID uint64, duration time.Duration) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:           id,
		UserID:       userID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(duration),
		LastActiveAt: now,
	}

	if err := s.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	return session, nil
}

func (s *sessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	if err := s.db.WithContext(ctx).
		Where("id = ? AND expires_at > ?", sessionID, time.Now()).
		First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("getting session: %w", err)
	}
	return &session, nil
}

func (s *sessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.db.WithContext(ctx).Delete(&Session{}, "id = ?", sessionID).Error; err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *sessionStore) DeleteExpired(ctx context.Context) error {
	if err := s.db.WithContext(ctx).
		Where("expires_at <= ?", time.Now()).
		Delete(&Session{}).Error; err != nil {
		return fmt.Errorf("deleting expired sessions: %w", err)
	}
	return nil
}

func (s *sessionStore) Touch(ctx context.Context, sessionID string) error {
	if err := s.db.WithContext(ctx).
		Model(&Session{}).
		Where("id = ?", sessionID).
		Update("last_active_at", time.Now()).Error; err != nil {
		return fmt.Errorf("touching session: %w", err)
	}
	return nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

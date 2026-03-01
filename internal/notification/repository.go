package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrNotificationNotFound = errors.New("notification not found")

type Repository interface {
	Create(ctx context.Context, n *Notification) error
	FindByID(ctx context.Context, id uint64) (*Notification, error)
	Update(ctx context.Context, n *Notification) error
	ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Notification, error)
	MarkRead(ctx context.Context, id, userID uint64) error
	MarkAllRead(ctx context.Context, userID uint64) error
	ListPendingEmails(ctx context.Context, limit int) ([]Notification, error)
	PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error)

	// Preferences
	GetPreferences(ctx context.Context, userID uint64) ([]NotificationPreference, error)
	GetPreference(ctx context.Context, userID uint64, eventType string) (*NotificationPreference, error)
	UpsertPreference(ctx context.Context, pref *NotificationPreference) error

	// Path owner rules
	ListPathOwnerRules(ctx context.Context, repoID uint64) ([]PathOwnerRule, error)
	CreatePathOwnerRule(ctx context.Context, rule *PathOwnerRule) error
	DeletePathOwnerRule(ctx context.Context, id uint64) error
	ListActiveRulesByRepo(ctx context.Context, repoID uint64) ([]PathOwnerRule, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *repository) FindByID(ctx context.Context, id uint64) (*Notification, error) {
	var n Notification
	if err := r.db.WithContext(ctx).First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}
	return &n, nil
}

func (r *repository) Update(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Save(n).Error
}

func (r *repository) ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Notification, error) {
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var notifications []Notification
	if err := query.Order("created_at DESC").Limit(limit + 1).Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *repository) MarkRead(ctx context.Context, id, userID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *repository) MarkAllRead(ctx context.Context, userID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *repository) ListPendingEmails(ctx context.Context, limit int) ([]Notification, error) {
	var notifications []Notification
	if err := r.db.WithContext(ctx).
		Where("email_state = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *repository) PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	result := r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&Notification{})
	return result.RowsAffected, result.Error
}

// Preferences
func (r *repository) GetPreferences(ctx context.Context, userID uint64) ([]NotificationPreference, error) {
	var prefs []NotificationPreference
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&prefs).Error; err != nil {
		return nil, err
	}
	return prefs, nil
}

func (r *repository) GetPreference(ctx context.Context, userID uint64, eventType string) (*NotificationPreference, error) {
	var pref NotificationPreference
	err := r.db.WithContext(ctx).Where("user_id = ? AND event_type = ?", userID, eventType).First(&pref).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Try wildcard
			err = r.db.WithContext(ctx).Where("user_id = ? AND event_type = ?", userID, "*").First(&pref).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return &pref, nil
}

func (r *repository) UpsertPreference(ctx context.Context, pref *NotificationPreference) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND event_type = ?", pref.UserID, pref.EventType).
		Assign(map[string]interface{}{
			"delivery_mode": pref.DeliveryMode,
			"suppress_self": pref.SuppressSelf,
		}).
		FirstOrCreate(pref).Error
}

// Path owner rules
func (r *repository) ListPathOwnerRules(ctx context.Context, repoID uint64) ([]PathOwnerRule, error) {
	var rules []PathOwnerRule
	if err := r.db.WithContext(ctx).Where("repo_id = ?", repoID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *repository) CreatePathOwnerRule(ctx context.Context, rule *PathOwnerRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *repository) DeletePathOwnerRule(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&PathOwnerRule{}, id).Error
}

func (r *repository) ListActiveRulesByRepo(ctx context.Context, repoID uint64) ([]PathOwnerRule, error) {
	var rules []PathOwnerRule
	if err := r.db.WithContext(ctx).
		Where("repo_id = ? AND is_active = ?", repoID, true).
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// Ensure interface
var _ Repository = (*repository)(nil)
var _ = fmt.Sprintf

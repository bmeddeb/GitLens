package auth

import "time"

type Session struct {
	ID           string    `gorm:"primaryKey;size:64" json:"id"`
	UserID       uint64    `gorm:"index:idx_sessions_user_id;not null" json:"user_id"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	ExpiresAt    time.Time `gorm:"index:idx_sessions_expires_at;not null" json:"expires_at"`
	LastActiveAt time.Time `gorm:"not null" json:"last_active_at"`
}

func (Session) TableName() string {
	return "sessions"
}

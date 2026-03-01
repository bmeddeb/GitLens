package user

import "time"

type User struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GitHubID    uint64     `gorm:"uniqueIndex:uk_users_github_id;not null" json:"github_id"`
	Username    string     `gorm:"uniqueIndex:uk_users_username;size:255;not null" json:"username"`
	DisplayName *string    `gorm:"size:255" json:"display_name,omitempty"`
	Email       *string    `gorm:"size:255" json:"email,omitempty"`
	AvatarURL   *string    `gorm:"size:512" json:"avatar_url,omitempty"`
	AccessToken []byte     `gorm:"type:varbinary(512);not null" json:"-"`
	TokenExpiry *time.Time `json:"-"`
	Role        string     `gorm:"type:enum('user','admin');default:'user';not null" json:"role"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}

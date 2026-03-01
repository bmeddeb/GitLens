package notification

import "time"

type Notification struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint64     `gorm:"not null" json:"user_id"`
	ReviewID      *uint64    `json:"review_id,omitempty"`
	CommentID     *uint64    `json:"comment_id,omitempty"`
	EventType     string     `gorm:"size:64;not null" json:"event_type"`
	Title         string     `gorm:"size:512;not null" json:"title"`
	BodyPreview   *string    `gorm:"size:512" json:"body_preview,omitempty"`
	Payload       *string    `gorm:"type:json" json:"payload,omitempty"`
	IsRead        bool       `gorm:"default:false;not null" json:"is_read"`
	EmailState    string     `gorm:"type:enum('not_applicable','pending','sent','failed','suppressed');default:'not_applicable';not null" json:"email_state"`
	EmailAttempts uint8      `gorm:"default:0;not null" json:"email_attempts"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	ReadAt        *time.Time `json:"read_at,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

type NotificationPreference struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"not null" json:"user_id"`
	EventType    string    `gorm:"size:64;not null" json:"event_type"`
	DeliveryMode string    `gorm:"type:enum('instant','digest','web_only');not null" json:"delivery_mode"`
	SuppressSelf bool      `gorm:"default:true;not null" json:"suppress_self"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }

type PathOwnerRule struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RepoID       uint64    `gorm:"not null" json:"repo_id"`
	PathGlob     string    `gorm:"size:512;not null" json:"path_glob"`
	FileExt      *string   `gorm:"size:32" json:"file_ext,omitempty"`
	TargetUserID uint64    `gorm:"not null" json:"target_user_id"`
	ActionRole   string    `gorm:"type:enum('reviewer','blocking_reviewer','subscriber');not null" json:"action_role"`
	IsActive     bool      `gorm:"default:true;not null" json:"is_active"`
	CreatedBy    uint64    `gorm:"not null" json:"created_by"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PathOwnerRule) TableName() string { return "path_owner_rules" }

// Event type constants
const (
	EventReviewCreated     = "review.created"
	EventRevisionAdded     = "revision_added"
	EventReviewerAdded     = "reviewer_added"
	EventSubscriberAdded   = "subscriber_added"
	EventBatchPublished    = "batch_published"
	EventMention           = "mention"
	EventChangesRequested  = "changes_requested"
	EventAccepted          = "accepted"
	EventResolved          = "resolved"
	EventClosed            = "closed"
)

package review

import "time"

type Review struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RepoID           uint64     `gorm:"not null" json:"repo_id"`
	Kind             string     `gorm:"type:enum('review','audit');not null" json:"kind"`
	Status           string     `gorm:"type:enum('draft','open','changes_requested','accepted','resolved','closed');default:'open';not null" json:"status"`
	AuthorID         uint64     `gorm:"not null" json:"author_id"`
	Title            string     `gorm:"size:512;not null" json:"title"`
	Summary          *string    `gorm:"type:text" json:"summary,omitempty"`
	OriginType       string     `gorm:"type:enum('manual','single_commit','commit_range','fork_change','similarity_cluster');not null" json:"origin_type"`
	OriginPayload    *string    `gorm:"type:json" json:"origin_payload,omitempty"`
	LatestRevisionID *uint64    `json:"latest_revision_id,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`

	LatestRevision *ReviewRevision     `gorm:"foreignKey:LatestRevisionID" json:"latest_revision,omitempty"`
	Participants   []ReviewParticipant `gorm:"foreignKey:ReviewID" json:"participants,omitempty"`
}

func (Review) TableName() string { return "reviews" }

type ReviewRevision struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ReviewID       uint64    `gorm:"not null" json:"review_id"`
	RevisionNumber uint      `gorm:"not null" json:"revision_number"`
	BaseRef        *string   `gorm:"size:255" json:"base_ref,omitempty"`
	HeadRef        *string   `gorm:"size:255" json:"head_ref,omitempty"`
	BaseCommitSHA  string    `gorm:"size:40;not null" json:"base_commit_sha"`
	HeadCommitSHA  string    `gorm:"size:40;not null" json:"head_commit_sha"`
	FileCount      *uint     `json:"file_count,omitempty"`
	Insertions     *uint     `json:"insertions,omitempty"`
	Deletions      *uint     `json:"deletions,omitempty"`
	CreatedBy      uint64    `gorm:"not null" json:"created_by"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ReviewRevision) TableName() string { return "review_revisions" }

type ReviewParticipant struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ReviewID    uint64    `gorm:"not null" json:"review_id"`
	UserID      uint64    `gorm:"not null" json:"user_id"`
	Role        string    `gorm:"type:enum('author','reviewer','blocking_reviewer','subscriber');not null" json:"role"`
	State       string    `gorm:"type:enum('pending','commented','changes_requested','accepted','watching');default:'pending';not null" json:"state"`
	AddedBy     uint64    `gorm:"not null" json:"added_by"`
	AddedReason string    `gorm:"type:enum('manual','path_owner_rule','mention','fork_suggestion');not null" json:"added_reason"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ReviewParticipant) TableName() string { return "review_participants" }

type ReviewComment struct {
	ID                 uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ReviewID           uint64     `gorm:"not null" json:"review_id"`
	RevisionID         uint64     `gorm:"not null" json:"revision_id"`
	AuthorID           uint64     `gorm:"not null" json:"author_id"`
	ParentCommentID    *uint64    `json:"parent_comment_id,omitempty"`
	PublishBatchID     *string    `gorm:"size:36" json:"publish_batch_id,omitempty"`
	FilePath           *string    `gorm:"size:1024" json:"file_path,omitempty"`
	Side               *string    `gorm:"type:enum('left','right')" json:"side,omitempty"`
	StartLine          *uint      `json:"start_line,omitempty"`
	EndLine            *uint      `json:"end_line,omitempty"`
	Body               string     `gorm:"type:text;not null" json:"body"`
	Visibility         string     `gorm:"type:enum('draft','published');default:'draft';not null" json:"visibility"`
	IsResolved         bool       `gorm:"default:false;not null" json:"is_resolved"`
	ContextFingerprint *string    `gorm:"size:64" json:"context_fingerprint,omitempty"`
	AnchorMethod       string     `gorm:"type:enum('original','fingerprint','offset','orphaned');default:'original';not null" json:"anchor_method"`
	OrphanedAt         *time.Time `json:"orphaned_at,omitempty"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"created_at"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
}

func (ReviewComment) TableName() string { return "review_comments" }

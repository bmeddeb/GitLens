package jobs

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type JobType string

const (
	JobTypeClone        JobType = "clone"
	JobTypeSync         JobType = "sync"
	JobTypeForkAnalysis JobType = "fork_analysis"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed || s == JobStatusCancelled
}

type Job struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Type            JobType    `gorm:"type:enum('clone','sync','fork_analysis');not null" json:"type"`
	Status          JobStatus  `gorm:"type:enum('pending','running','completed','failed','cancelled');default:'pending';not null" json:"status"`
	RepoID          uint64     `gorm:"index:idx_jobs_repo_status;not null" json:"repo_id"`
	ForkID          *uint64    `json:"fork_id,omitempty"`
	DedupKey        *string    `gorm:"uniqueIndex:uk_jobs_dedup;size:255" json:"-"`
	RequestedBy     uint64     `gorm:"index:idx_jobs_requested_by;not null" json:"requested_by"`
	ProgressPct     uint8      `gorm:"default:0;not null" json:"progress_pct"`
	ProgressMessage *string    `gorm:"size:512" json:"progress_message,omitempty"`
	ResultSummary   JSONMap    `gorm:"type:json" json:"result_summary,omitempty"`
	ErrorMessage    *string    `gorm:"type:text" json:"error_message,omitempty"`
	RetryCount      uint8      `gorm:"default:0;not null" json:"retry_count"`
	MaxRetries      uint8      `gorm:"default:3;not null" json:"max_retries"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func (Job) TableName() string {
	return "jobs"
}

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

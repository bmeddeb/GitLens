package repository

import "time"

type Repository struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GitHubID       uint64     `gorm:"uniqueIndex:uk_repos_github_id;not null" json:"github_id"`
	Owner          string     `gorm:"size:255;not null" json:"owner"`
	Name           string     `gorm:"size:255;not null" json:"name"`
	FullName       string     `gorm:"uniqueIndex:uk_repos_full_name;size:512;not null" json:"full_name"`
	Description    *string    `gorm:"type:text" json:"description,omitempty"`
	DefaultBranch  string     `gorm:"size:255;default:'main';not null" json:"default_branch"`
	CloneURL       string     `gorm:"size:512;not null" json:"clone_url"`
	IsFork         bool       `gorm:"default:false;not null" json:"is_fork"`
	IsPrivate      bool       `gorm:"default:false;not null" json:"is_private"`
	ParentGitHubID *uint64    `json:"parent_github_id,omitempty"`
	LocalPath      *string    `gorm:"size:512" json:"-"`
	CloneStatus    string     `gorm:"type:enum('pending','cloning','ready','error');default:'pending';not null" json:"clone_status"`
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Repository) TableName() string {
	return "repositories"
}

type UserRepository struct {
	ID                   uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID               uint64    `gorm:"uniqueIndex:uk_user_repos;not null" json:"user_id"`
	RepoID               uint64    `gorm:"uniqueIndex:uk_user_repos;index:idx_user_repos_repo_id;not null" json:"repo_id"`
	AddedAt              time.Time `gorm:"autoCreateTime" json:"added_at"`
	AccessState          string    `gorm:"type:enum('active','suspended');default:'active';not null" json:"access_state"`
	LastAccessVerifiedAt time.Time `gorm:"autoCreateTime" json:"last_access_verified_at"`
	Nickname             *string   `gorm:"size:255" json:"nickname,omitempty"`

	Repository *Repository `gorm:"foreignKey:RepoID" json:"repository,omitempty"`
}

func (UserRepository) TableName() string {
	return "user_repositories"
}

type Commit struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RepoID        uint64    `gorm:"uniqueIndex:uk_commits_repo_sha;not null" json:"repo_id"`
	SHA           string    `gorm:"uniqueIndex:uk_commits_repo_sha;size:40;not null" json:"sha"`
	AuthorName    *string   `gorm:"size:255" json:"author_name,omitempty"`
	AuthorEmail   *string   `gorm:"size:255" json:"author_email,omitempty"`
	CommitterName *string   `gorm:"size:255" json:"committer_name,omitempty"`
	CommittedAt   time.Time `gorm:"not null" json:"committed_at"`
	Message       *string   `gorm:"type:text" json:"message,omitempty"`
	FilesChanged  *uint     `json:"files_changed,omitempty"`
	Insertions    *uint     `json:"insertions,omitempty"`
	Deletions     *uint     `json:"deletions,omitempty"`
	ParentSHAs    JSONArray `gorm:"type:json" json:"parent_shas,omitempty"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"-"`
}

func (Commit) TableName() string {
	return "commits"
}

type BlameCache struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RepoID    uint64    `gorm:"not null" json:"repo_id"`
	FilePath  string    `gorm:"size:1024;not null" json:"file_path"`
	CommitSHA string    `gorm:"size:40;not null" json:"commit_sha"`
	BlameData []byte    `gorm:"type:mediumblob;not null" json:"-"`
	LineCount *uint     `json:"line_count,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (BlameCache) TableName() string {
	return "blame_cache"
}

type ContributorIdentity struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CanonicalEmail string    `gorm:"uniqueIndex:uk_contrib_identity_email;size:255;not null" json:"canonical_email"`
	DisplayName    *string   `gorm:"size:255" json:"display_name,omitempty"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ContributorIdentity) TableName() string {
	return "contributor_identities"
}

type ContributorAlias struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	IdentityID    uint64    `gorm:"index:idx_contrib_alias_identity;not null" json:"identity_id"`
	ObservedName  *string   `gorm:"size:255" json:"observed_name,omitempty"`
	ObservedEmail string    `gorm:"uniqueIndex:uk_contrib_alias_email;size:255;not null" json:"observed_email"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ContributorAlias) TableName() string {
	return "contributor_aliases"
}

type ContributorStat struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RepoID        uint64     `gorm:"uniqueIndex:uk_contrib_stats;not null" json:"repo_id"`
	IdentityID    uint64     `gorm:"uniqueIndex:uk_contrib_stats;not null" json:"identity_id"`
	CommitsCount  uint       `gorm:"default:0;not null" json:"commits_count"`
	Insertions    uint64     `gorm:"default:0;not null" json:"insertions"`
	Deletions     uint64     `gorm:"default:0;not null" json:"deletions"`
	FirstCommitAt *time.Time `json:"first_commit_at,omitempty"`
	LastCommitAt  *time.Time `json:"last_commit_at,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	Identity *ContributorIdentity `gorm:"foreignKey:IdentityID" json:"identity,omitempty"`
}

func (ContributorStat) TableName() string {
	return "contributor_stats"
}

// Fork models
type Fork struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceRepoID   uint64     `gorm:"index:idx_forks_source_repo;not null" json:"source_repo_id"`
	GitHubForkID   uint64     `gorm:"uniqueIndex:uk_forks_github_id;not null" json:"github_fork_id"`
	ForkOwner      string     `gorm:"size:255;not null" json:"fork_owner"`
	ForkName       string     `gorm:"size:255;not null" json:"fork_name"`
	ForkFullName   string     `gorm:"size:512;not null" json:"fork_full_name"`
	CloneURL       *string    `gorm:"size:512" json:"clone_url,omitempty"`
	LocalPath      *string    `gorm:"size:512" json:"-"`
	AheadBy        uint       `gorm:"default:0;not null" json:"ahead_by"`
	BehindBy       uint       `gorm:"default:0;not null" json:"behind_by"`
	MergeBaseSHA   *string    `gorm:"size:40" json:"merge_base_sha,omitempty"`
	DivergedAt     *time.Time `json:"diverged_at,omitempty"`
	AnalysisStatus string     `gorm:"type:enum('listed','pending','analyzing','complete','error');default:'listed';not null" json:"analysis_status"`
	LastAnalyzedAt *time.Time `json:"last_analyzed_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Fork) TableName() string {
	return "forks"
}

type ForkChange struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ForkID       uint64    `gorm:"uniqueIndex:uk_fork_changes;not null" json:"fork_id"`
	CommitSHA    string    `gorm:"uniqueIndex:uk_fork_changes;size:40;not null" json:"commit_sha"`
	FilePath     *string   `gorm:"size:1024" json:"file_path,omitempty"`
	ChangeType   string    `gorm:"type:enum('feature','bugfix','refactor','docs','chore','unknown');default:'unknown';not null" json:"change_type"`
	Confidence   float64   `gorm:"type:decimal(3,2);default:0.30;not null" json:"confidence"`
	IsCherryPick bool      `gorm:"default:false;not null" json:"is_cherry_pick"`
	IsRebase     bool      `gorm:"default:false;not null" json:"is_rebase"`
	UpstreamSHA  *string   `gorm:"size:40" json:"upstream_sha,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ForkChange) TableName() string {
	return "fork_changes"
}

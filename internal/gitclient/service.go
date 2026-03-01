package gitclient

import (
	"context"
	"time"
)

type GitService interface {
	Clone(ctx context.Context, url, destPath string) error
	Pull(ctx context.Context, repoPath string) error
	Log(ctx context.Context, repoPath string, opts LogOptions) ([]CommitInfo, error)
	CommitDetail(ctx context.Context, repoPath string, sha string) (*CommitInfo, error)
	Diff(ctx context.Context, repoPath string, baseSHA, headSHA string) (*DiffResult, error)
	Blame(ctx context.Context, repoPath string, filePath, ref string) (*BlameResult, error)
	MergeBase(ctx context.Context, repoPath string, sha1, sha2 string) (string, error)
	WalkCommits(ctx context.Context, repoPath string, fromSHA, toSHA string) ([]CommitInfo, error)
}

type LogOptions struct {
	Author  string
	Since   *time.Time
	Until   *time.Time
	Path    string
	Keyword string
	Skip    int
	Limit   int
}

type CommitInfo struct {
	SHA           string    `json:"sha"`
	AuthorName    string    `json:"author_name"`
	AuthorEmail   string    `json:"author_email"`
	CommitterName string    `json:"committer_name"`
	CommittedAt   time.Time `json:"committed_at"`
	Message       string    `json:"message"`
	FilesChanged  int       `json:"files_changed"`
	Insertions    int       `json:"insertions"`
	Deletions     int       `json:"deletions"`
	ParentSHAs    []string  `json:"parent_shas"`
}

type DiffResult struct {
	Files []FileDiff `json:"files"`
}

type FileDiff struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
	Status  string `json:"status"` // added, deleted, modified, renamed
	Hunks   []Hunk `json:"hunks"`
}

type Hunk struct {
	OldStart int        `json:"old_start"`
	OldLines int        `json:"old_lines"`
	NewStart int        `json:"new_start"`
	NewLines int        `json:"new_lines"`
	Header   string     `json:"header"`
	Lines    []DiffLine `json:"lines"`
}

type DiffLine struct {
	Type    DiffLineType `json:"type"`
	Content string       `json:"content"`
	OldNum  int          `json:"old_num,omitempty"`
	NewNum  int          `json:"new_num,omitempty"`
}

type DiffLineType string

const (
	DiffLineContext  DiffLineType = "context"
	DiffLineAdd      DiffLineType = "add"
	DiffLineDelete   DiffLineType = "delete"
)

type BlameResult struct {
	Lines []BlameLine `json:"lines"`
}

type BlameLine struct {
	SHA         string    `json:"sha"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	Date        time.Time `json:"date"`
	LineNo      int       `json:"line_no"`
	Content     string    `json:"content"`
}

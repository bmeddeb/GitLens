package analysis

import (
	"strings"

	"github.com/user/gitlens-pro/internal/gitclient"
)

type CherryPickDetector struct{}

func NewCherryPickDetector() *CherryPickDetector {
	return &CherryPickDetector{}
}

// Detect checks if the commit is a cherry-pick of any source commit.
// Returns (matched, upstream_sha).
// Match criteria: identical message body (ignoring cherry-pick trailers).
func (d *CherryPickDetector) Detect(commit gitclient.CommitInfo, sourceCommits []gitclient.CommitInfo) (bool, string) {
	normalizedMsg := normalizeMessage(commit.Message)

	for _, sc := range sourceCommits {
		if commit.SHA == sc.SHA {
			continue
		}
		if normalizeMessage(sc.Message) == normalizedMsg && normalizedMsg != "" {
			return true, sc.SHA
		}
	}

	return false, ""
}

func normalizeMessage(msg string) string {
	// Strip common cherry-pick trailers
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "(cherry picked from commit") {
			continue
		}
		if strings.HasPrefix(trimmed, "Cherry-picked-from:") {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

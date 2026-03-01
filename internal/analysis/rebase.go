package analysis

import (
	"github.com/user/gitlens-pro/internal/gitclient"
)

type RebaseDetector struct{}

func NewRebaseDetector() *RebaseDetector {
	return &RebaseDetector{}
}

// Detect checks if the commit is a rebase of any source commit.
// Match criteria: different SHA but >95% patch overlap (Levenshtein on message).
func (d *RebaseDetector) Detect(commit gitclient.CommitInfo, sourceCommits []gitclient.CommitInfo) (bool, string) {
	for _, sc := range sourceCommits {
		if commit.SHA == sc.SHA {
			continue
		}

		// Compare messages using Levenshtein ratio
		ratio := LevenshteinRatio(commit.Message, sc.Message)
		if ratio > 0.95 {
			return true, sc.SHA
		}
	}

	return false, ""
}

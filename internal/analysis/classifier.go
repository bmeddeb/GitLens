package analysis

import (
	"path/filepath"
	"strings"

	"github.com/user/gitlens-pro/internal/gitclient"
)

type Classifier struct {
	rules []ClassificationRule
}

type ClassificationRule struct {
	Name       string
	Match      func(c gitclient.CommitInfo) bool
	ChangeType string
	Confidence float64
}

func NewClassifier() *Classifier {
	return &Classifier{
		rules: []ClassificationRule{
			// Priority 1-5: Conventional commit prefixes (highest confidence 0.9)
			{Name: "conventional-feat", Match: hasPrefix("feat:", "feat("), ChangeType: "feature", Confidence: 0.9},
			{Name: "conventional-fix", Match: hasPrefix("fix:", "fix("), ChangeType: "bugfix", Confidence: 0.9},
			{Name: "conventional-refactor", Match: hasPrefix("refactor:", "refactor("), ChangeType: "refactor", Confidence: 0.9},
			{Name: "conventional-docs", Match: hasPrefix("docs:", "docs("), ChangeType: "docs", Confidence: 0.9},
			{Name: "conventional-chore", Match: hasPrefix("chore:", "chore(", "build:", "build(", "ci:", "ci("), ChangeType: "chore", Confidence: 0.9},
			// Priority 6-9: File path patterns (medium confidence)
			{Name: "path-docs", Match: pathContains("doc", "docs", "README", "CHANGELOG", "LICENSE"), ChangeType: "docs", Confidence: 0.8},
			{Name: "path-test", Match: pathContains("test", "tests", "spec", "_test.go", "_test.py"), ChangeType: "chore", Confidence: 0.7},
			{Name: "path-config", Match: pathContains(".github", ".ci", "Makefile", "Dockerfile", "docker-compose", ".yaml", ".yml", ".toml"), ChangeType: "chore", Confidence: 0.7},
			{Name: "path-feature", Match: pathContains("src/", "internal/", "lib/", "pkg/"), ChangeType: "feature", Confidence: 0.7},
			// Priority 10: Stat heuristics (lower confidence)
			{Name: "heuristic-bugfix", Match: func(c gitclient.CommitInfo) bool {
				msg := strings.ToLower(c.Message)
				return strings.Contains(msg, "fix") || strings.Contains(msg, "bug") || strings.Contains(msg, "patch") || strings.Contains(msg, "hotfix")
			}, ChangeType: "bugfix", Confidence: 0.6},
			// Priority 11: Default
			{Name: "unknown", Match: func(c gitclient.CommitInfo) bool { return true }, ChangeType: "unknown", Confidence: 0.3},
		},
	}
}

func (c *Classifier) Classify(commit gitclient.CommitInfo) (string, float64) {
	for _, rule := range c.rules {
		if rule.Match(commit) {
			return rule.ChangeType, rule.Confidence
		}
	}
	return "unknown", 0.3
}

func hasPrefix(prefixes ...string) func(gitclient.CommitInfo) bool {
	return func(c gitclient.CommitInfo) bool {
		msg := strings.TrimSpace(strings.ToLower(c.Message))
		for _, p := range prefixes {
			if strings.HasPrefix(msg, p) {
				return true
			}
		}
		return false
	}
}

func pathContains(patterns ...string) func(gitclient.CommitInfo) bool {
	return func(c gitclient.CommitInfo) bool {
		// Check message for file paths (heuristic - would normally check actual changed files)
		msg := strings.ToLower(c.Message)
		for _, p := range patterns {
			base := filepath.Base(strings.ToLower(p))
			if strings.Contains(msg, base) {
				return true
			}
		}
		return false
	}
}

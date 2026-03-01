package notification

import (
	"context"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type PathOwnerService interface {
	ListRules(ctx context.Context, repoID uint64) ([]PathOwnerRule, error)
	CreateRule(ctx context.Context, rule *PathOwnerRule) error
	DeleteRule(ctx context.Context, id uint64) error
	EvaluateRules(ctx context.Context, repoID uint64, changedPaths []string) ([]PathOwnerMatch, error)
}

type PathOwnerMatch struct {
	Rule         PathOwnerRule `json:"rule"`
	MatchedPaths []string      `json:"matched_paths"`
}

type pathOwnerService struct {
	repo   Repository
	logger *zap.Logger
}

func NewPathOwnerService(repo Repository, logger *zap.Logger) PathOwnerService {
	return &pathOwnerService{repo: repo, logger: logger}
}

func (s *pathOwnerService) ListRules(ctx context.Context, repoID uint64) ([]PathOwnerRule, error) {
	return s.repo.ListPathOwnerRules(ctx, repoID)
}

func (s *pathOwnerService) CreateRule(ctx context.Context, rule *PathOwnerRule) error {
	return s.repo.CreatePathOwnerRule(ctx, rule)
}

func (s *pathOwnerService) DeleteRule(ctx context.Context, id uint64) error {
	return s.repo.DeletePathOwnerRule(ctx, id)
}

func (s *pathOwnerService) EvaluateRules(ctx context.Context, repoID uint64, changedPaths []string) ([]PathOwnerMatch, error) {
	rules, err := s.repo.ListActiveRulesByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}

	// De-dupe matches by target_user_id
	seen := make(map[uint64]bool)
	var matches []PathOwnerMatch

	for _, rule := range rules {
		var matchedPaths []string

		for _, p := range changedPaths {
			if matchesRule(rule, p) {
				matchedPaths = append(matchedPaths, p)
			}
		}

		if len(matchedPaths) > 0 && !seen[rule.TargetUserID] {
			seen[rule.TargetUserID] = true
			matches = append(matches, PathOwnerMatch{
				Rule:         rule,
				MatchedPaths: matchedPaths,
			})
		}
	}

	return matches, nil
}

func matchesRule(rule PathOwnerRule, path string) bool {
	// Check file extension filter
	if rule.FileExt != nil && *rule.FileExt != "" {
		ext := filepath.Ext(path)
		if !strings.EqualFold(ext, "."+*rule.FileExt) && !strings.EqualFold(ext, *rule.FileExt) {
			return false
		}
	}

	// Check path glob
	matched, err := filepath.Match(rule.PathGlob, path)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Try with ** prefix handling
	if strings.Contains(rule.PathGlob, "**") {
		// Simple ** support: match any directory depth
		pattern := strings.ReplaceAll(rule.PathGlob, "**", "*")
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}

		// Also try matching just the filename
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		return matched
	}

	return false
}

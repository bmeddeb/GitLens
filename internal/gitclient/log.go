package gitclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func (s *gitService) Log(ctx context.Context, repoPath string, opts LogOptions) ([]CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	logOpts := &git.LogOptions{
		Order: git.LogOrderCommitterTime,
	}

	if opts.Path != "" {
		logOpts.PathFilter = func(p string) bool {
			return p == opts.Path || strings.HasPrefix(p, opts.Path+"/")
		}
	}

	if opts.Since != nil {
		logOpts.Since = opts.Since
	}
	if opts.Until != nil {
		logOpts.Until = opts.Until
	}

	iter, err := repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("getting log: %w", err)
	}
	defer iter.Close()

	var commits []CommitInfo
	skipped := 0
	collected := 0
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	err = iter.ForEach(func(c *object.Commit) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if opts.Author != "" && !strings.Contains(strings.ToLower(c.Author.Email), strings.ToLower(opts.Author)) {
			return nil
		}

		if opts.Keyword != "" && !strings.Contains(strings.ToLower(c.Message), strings.ToLower(opts.Keyword)) {
			return nil
		}

		if skipped < opts.Skip {
			skipped++
			return nil
		}

		if collected >= limit {
			return fmt.Errorf("limit reached")
		}

		info := commitToInfo(c)
		commits = append(commits, info)
		collected++
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	return commits, nil
}

func (s *gitService) CommitDetail(ctx context.Context, repoPath string, sha string) (*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	hash := plumbing.NewHash(sha)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("getting commit %s: %w", sha, err)
	}

	info := commitToInfo(commit)

	// Compute stats by diffing against first parent (FR-COMMIT-05)
	stats, err := commitStats(commit)
	if err == nil {
		info.FilesChanged = len(stats)
		for _, s := range stats {
			info.Insertions += s.Addition
			info.Deletions += s.Deletion
		}
	}

	return &info, nil
}

func (s *gitService) WalkCommits(ctx context.Context, repoPath string, fromSHA, toSHA string) ([]CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	toHash := plumbing.NewHash(toSHA)
	iter, err := repo.Log(&git.LogOptions{From: toHash, Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, fmt.Errorf("getting log from %s: %w", toSHA, err)
	}
	defer iter.Close()

	var commits []CommitInfo
	err = iter.ForEach(func(c *object.Commit) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if c.Hash.String() == fromSHA {
			return fmt.Errorf("stop")
		}
		commits = append(commits, commitToInfo(c))
		return nil
	})

	if err != nil && err.Error() != "stop" {
		return nil, fmt.Errorf("walking commits: %w", err)
	}

	return commits, nil
}

func commitToInfo(c *object.Commit) CommitInfo {
	info := CommitInfo{
		SHA:           c.Hash.String(),
		AuthorName:    c.Author.Name,
		AuthorEmail:   c.Author.Email,
		CommitterName: c.Committer.Name,
		CommittedAt:   c.Committer.When,
		Message:       c.Message,
	}

	for _, p := range c.ParentHashes {
		info.ParentSHAs = append(info.ParentSHAs, p.String())
	}

	return info
}

func commitStats(c *object.Commit) (object.FileStats, error) {
	if c.NumParents() == 0 {
		return c.Stats()
	}

	// Diff against first parent (merge commits)
	parent, err := c.Parent(0)
	if err != nil {
		return nil, err
	}

	parentTree, err := parent.Tree()
	if err != nil {
		return nil, err
	}

	commitTree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := parentTree.Diff(commitTree)
	if err != nil {
		return nil, err
	}

	patch, err := changes.Patch()
	if err != nil {
		return nil, err
	}

	return patch.Stats(), nil
}

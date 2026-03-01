package gitclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func (s *gitService) Blame(ctx context.Context, repoPath string, filePath, ref string) (*BlameResult, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	var hash plumbing.Hash
	if ref == "" || ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("getting HEAD: %w", err)
		}
		hash = head.Hash()
	} else {
		hash = plumbing.NewHash(ref)
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("getting commit: %w", err)
	}

	result, err := git.Blame(commit, filePath)
	if err != nil {
		return nil, fmt.Errorf("blaming %s: %w", filePath, err)
	}

	blameResult := &BlameResult{}
	for i, line := range result.Lines {
		bl := BlameLine{
			SHA:         line.Hash.String(),
			AuthorName:  line.Author,
			AuthorEmail: line.Author, // go-git blame only provides Author name
			Date:        line.Date,
			LineNo:      i + 1,
			Content:     line.Text,
		}

		// Try to get email from the commit
		if c, err := repo.CommitObject(line.Hash); err == nil {
			bl.AuthorEmail = c.Author.Email
			bl.AuthorName = c.Author.Name
		}

		blameResult.Lines = append(blameResult.Lines, bl)
	}

	return blameResult, nil
}

func (s *gitService) MergeBase(ctx context.Context, repoPath string, sha1, sha2 string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("opening repo: %w", err)
	}

	c1, err := repo.CommitObject(plumbing.NewHash(sha1))
	if err != nil {
		return "", fmt.Errorf("getting commit %s: %w", sha1, err)
	}

	c2, err := repo.CommitObject(plumbing.NewHash(sha2))
	if err != nil {
		return "", fmt.Errorf("getting commit %s: %w", sha2, err)
	}

	bases, err := c1.MergeBase(c2)
	if err != nil {
		return "", fmt.Errorf("finding merge-base: %w", err)
	}

	if len(bases) == 0 {
		return "", fmt.Errorf("no merge-base found")
	}

	return bases[0].Hash.String(), nil
}

func (s *gitService) Diff(ctx context.Context, repoPath string, baseSHA, headSHA string) (*DiffResult, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	baseCommit, err := repo.CommitObject(plumbing.NewHash(baseSHA))
	if err != nil {
		return nil, fmt.Errorf("getting base commit: %w", err)
	}

	headCommit, err := repo.CommitObject(plumbing.NewHash(headSHA))
	if err != nil {
		return nil, fmt.Errorf("getting head commit: %w", err)
	}

	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting base tree: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting head tree: %w", err)
	}

	changes, err := baseTree.Diff(headTree)
	if err != nil {
		return nil, fmt.Errorf("computing diff: %w", err)
	}

	result := &DiffResult{}

	for _, change := range changes {
		patch, err := change.Patch()
		if err != nil {
			continue
		}

		for _, filePatch := range patch.FilePatches() {
			fd := FileDiff{}

			from, to := filePatch.Files()
			if from != nil {
				fd.OldPath = from.Path()
			}
			if to != nil {
				fd.NewPath = to.Path()
			}

			switch {
			case from == nil && to != nil:
				fd.Status = "added"
			case from != nil && to == nil:
				fd.Status = "deleted"
			case from != nil && to != nil && from.Path() != to.Path():
				fd.Status = "renamed"
			default:
				fd.Status = "modified"
			}

			for _, chunk := range filePatch.Chunks() {
				lines := strings.Split(chunk.Content(), "\n")
				hunk := Hunk{}

				for _, line := range lines {
					if line == "" {
						continue
					}

					dl := DiffLine{Content: line}
					switch chunk.Type() {
					case 0: // Equal
						dl.Type = DiffLineContext
					case 1: // Add
						dl.Type = DiffLineAdd
					case 2: // Delete
						dl.Type = DiffLineDelete
					}

					hunk.Lines = append(hunk.Lines, dl)
				}

				if len(hunk.Lines) > 0 {
					fd.Hunks = append(fd.Hunks, hunk)
				}
			}

			result.Files = append(result.Files, fd)
		}
	}

	return result, nil
}

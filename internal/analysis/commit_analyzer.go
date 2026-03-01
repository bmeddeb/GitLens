package analysis

import (
	"context"
	"fmt"

	"github.com/user/gitlens-pro/internal/gitclient"
	"github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type CommitAnalyzer interface {
	ProcessNewCommits(ctx context.Context, repoID uint64, commits []gitclient.CommitInfo) error
}

type commitAnalyzer struct {
	commitRepo      repository.CommitRepository
	contributorRepo repository.ContributorRepository
	logger          *zap.Logger
}

func NewCommitAnalyzer(
	commitRepo repository.CommitRepository,
	contributorRepo repository.ContributorRepository,
	logger *zap.Logger,
) CommitAnalyzer {
	return &commitAnalyzer{
		commitRepo:      commitRepo,
		contributorRepo: contributorRepo,
		logger:          logger,
	}
}

func (a *commitAnalyzer) ProcessNewCommits(ctx context.Context, repoID uint64, commits []gitclient.CommitInfo) error {
	var dbCommits []repository.Commit

	for _, c := range commits {
		// Check if commit already exists (idempotent)
		existing, err := a.commitRepo.FindBySHA(ctx, repoID, c.SHA)
		if err != nil {
			return fmt.Errorf("checking existing commit: %w", err)
		}
		if existing != nil {
			continue
		}

		fc := uint(c.FilesChanged)
		ins := uint(c.Insertions)
		del := uint(c.Deletions)

		dbCommit := repository.Commit{
			RepoID:        repoID,
			SHA:           c.SHA,
			AuthorName:    &c.AuthorName,
			AuthorEmail:   &c.AuthorEmail,
			CommitterName: &c.CommitterName,
			CommittedAt:   c.CommittedAt,
			Message:       &c.Message,
			FilesChanged:  &fc,
			Insertions:    &ins,
			Deletions:     &del,
			ParentSHAs:    c.ParentSHAs,
		}
		dbCommits = append(dbCommits, dbCommit)

		// Resolve contributor identity
		if err := a.resolveIdentity(ctx, c.AuthorName, c.AuthorEmail, repoID, &c); err != nil {
			a.logger.Warn("failed to resolve contributor identity",
				zap.String("email", c.AuthorEmail),
				zap.Error(err),
			)
		}
	}

	if len(dbCommits) > 0 {
		if err := a.commitRepo.CreateBatch(ctx, dbCommits); err != nil {
			return fmt.Errorf("inserting commits batch: %w", err)
		}
		a.logger.Info("processed new commits",
			zap.Uint64("repo_id", repoID),
			zap.Int("count", len(dbCommits)),
		)
	}

	return nil
}

func (a *commitAnalyzer) resolveIdentity(ctx context.Context, name, email string, repoID uint64, c *gitclient.CommitInfo) error {
	if email == "" {
		return nil
	}

	// Look up by observed email
	alias, err := a.contributorRepo.FindAliasByEmail(ctx, email)
	if err != nil {
		return err
	}

	var identityID uint64

	if alias != nil {
		identityID = alias.IdentityID
	} else {
		// Check if identity exists with this canonical email
		identity, err := a.contributorRepo.FindIdentityByEmail(ctx, email)
		if err != nil {
			return err
		}

		if identity != nil {
			identityID = identity.ID
		} else {
			// Create new identity
			identity = &repository.ContributorIdentity{
				CanonicalEmail: email,
				DisplayName:    &name,
			}
			if err := a.contributorRepo.CreateIdentity(ctx, identity); err != nil {
				return err
			}
			identityID = identity.ID
		}

		// Create alias
		alias = &repository.ContributorAlias{
			IdentityID:    identityID,
			ObservedName:  &name,
			ObservedEmail: email,
		}
		if err := a.contributorRepo.CreateAlias(ctx, alias); err != nil {
			// Ignore duplicate alias errors
			a.logger.Debug("alias already exists", zap.String("email", email))
		}
	}

	// Update contributor stats
	ins := uint64(c.Insertions)
	del := uint64(c.Deletions)
	stat := &repository.ContributorStat{
		RepoID:       repoID,
		IdentityID:   identityID,
		CommitsCount: 1,
		Insertions:   ins,
		Deletions:    del,
		LastCommitAt: &c.CommittedAt,
	}
	if err := a.contributorRepo.UpsertStats(ctx, stat); err != nil {
		return fmt.Errorf("upserting stats: %w", err)
	}

	return nil
}

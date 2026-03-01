package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/user/gitlens-pro/internal/auth"
	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/githubclient"
	"github.com/user/gitlens-pro/internal/user"
	"go.uber.org/zap"
)

var repoIDRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

// JobEnqueuer abstracts job enqueueing to avoid an import cycle with the jobs package.
type JobEnqueuer interface {
	Enqueue(ctx context.Context, jobType string, repoID uint64, forkID *uint64, requesterID uint64) (jobID uint64, deduplicated bool, err error)
}

const (
	JobTypeClone = "clone"
	JobTypeSync  = "sync"
)

type RepoService interface {
	AddRepo(ctx context.Context, userID uint64, fullName string) (*Repository, uint64, bool, error)
	GetRepo(ctx context.Context, userID, repoID uint64) (*Repository, error)
	SyncRepo(ctx context.Context, userID, repoID uint64) (uint64, bool, error)
	RemoveRepo(ctx context.Context, userID, repoID uint64) error
	RecheckAccess(ctx context.Context, userID, repoID uint64) (string, error)
	ListUserRepos(ctx context.Context, userID uint64, state string, cursor uint64, limit int) ([]UserRepository, error)
	ValidateAccess(ctx context.Context, userID, repoID uint64) (*UserRepository, error)
}

type repoService struct {
	repoRepo     RepoRepository
	userRepoRepo UserRepoRepository
	ghFactory    githubclient.GitHubClientFactory
	jobEnqueuer  JobEnqueuer
	userRepo     user.Repository
	cfg          *config.AppConfig
	logger       *zap.Logger
}

func NewRepoService(
	repoRepo RepoRepository,
	userRepoRepo UserRepoRepository,
	ghFactory githubclient.GitHubClientFactory,
	jobEnqueuer JobEnqueuer,
	userRepo user.Repository,
	cfg *config.AppConfig,
	logger *zap.Logger,
) RepoService {
	return &repoService{
		repoRepo:     repoRepo,
		userRepoRepo: userRepoRepo,
		ghFactory:    ghFactory,
		jobEnqueuer:  jobEnqueuer,
		userRepo:     userRepo,
		cfg:          cfg,
		logger:       logger,
	}
}

// AddRepo returns (repo, jobID, deduplicated, error). jobID == 0 means no job was enqueued.
func (s *repoService) AddRepo(ctx context.Context, userID uint64, fullName string) (*Repository, uint64, bool, error) {
	if !repoIDRegex.MatchString(fullName) {
		return nil, 0, false, fmt.Errorf("invalid repository name: must match owner/repo format")
	}

	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, 0, false, fmt.Errorf("finding user: %w", err)
	}

	ghClient, err := s.ghFactory.NewClient(ctx, u.AccessToken)
	if err != nil {
		return nil, 0, false, fmt.Errorf("creating GitHub client: %w", err)
	}

	parts := splitFullName(fullName)
	ghRepo, _, err := ghClient.Repositories.Get(ctx, parts[0], parts[1])
	if err != nil {
		return nil, 0, false, fmt.Errorf("verifying repo on GitHub: %w", err)
	}

	// Check if repo already exists in our system
	existing, err := s.repoRepo.FindByGitHubID(ctx, uint64(ghRepo.GetID()))
	if err != nil && !errors.Is(err, ErrRepoNotFound) {
		return nil, 0, false, fmt.Errorf("checking existing repo: %w", err)
	}

	if existing != nil {
		// Ensure user association exists
		ur, err := s.userRepoRepo.FindByUserAndRepo(ctx, userID, existing.ID)
		if err != nil && !errors.Is(err, ErrUserRepoNotFound) {
			return nil, 0, false, err
		}
		if ur == nil {
			ur = &UserRepository{
				UserID: userID,
				RepoID: existing.ID,
			}
			if err := s.userRepoRepo.Create(ctx, ur); err != nil {
				return nil, 0, false, err
			}
		}

		if existing.CloneStatus == "ready" {
			return existing, 0, false, nil
		}

		// Repo exists but not ready — return existing clone job
		jobID, deduped, err := s.jobEnqueuer.Enqueue(ctx, JobTypeClone, existing.ID, nil, userID)
		return existing, jobID, deduped, err
	}

	// Create new repo
	repo := &Repository{
		GitHubID:      uint64(ghRepo.GetID()),
		Owner:         ghRepo.GetOwner().GetLogin(),
		Name:          ghRepo.GetName(),
		FullName:      ghRepo.GetFullName(),
		Description:   strPtr(ghRepo.GetDescription()),
		DefaultBranch: ghRepo.GetDefaultBranch(),
		CloneURL:      ghRepo.GetCloneURL(),
		IsFork:        ghRepo.GetFork(),
		IsPrivate:     ghRepo.GetPrivate(),
		CloneStatus:   "pending",
	}

	if ghRepo.GetParent() != nil {
		pid := uint64(ghRepo.GetParent().GetID())
		repo.ParentGitHubID = &pid
	}

	if err := s.repoRepo.Create(ctx, repo); err != nil {
		return nil, 0, false, fmt.Errorf("creating repo: %w", err)
	}

	// Create user-repo association
	ur := &UserRepository{
		UserID: userID,
		RepoID: repo.ID,
	}
	if err := s.userRepoRepo.Create(ctx, ur); err != nil {
		return nil, 0, false, fmt.Errorf("creating user_repository: %w", err)
	}

	// Enqueue clone job
	jobID, deduped, err := s.jobEnqueuer.Enqueue(ctx, JobTypeClone, repo.ID, nil, userID)
	if err != nil {
		return nil, 0, false, fmt.Errorf("enqueueing clone job: %w", err)
	}

	return repo, jobID, deduped, nil
}

func (s *repoService) GetRepo(ctx context.Context, userID, repoID uint64) (*Repository, error) {
	if _, err := s.ValidateAccess(ctx, userID, repoID); err != nil {
		return nil, err
	}

	return s.repoRepo.FindByID(ctx, repoID)
}

// SyncRepo returns (jobID, deduplicated, error).
func (s *repoService) SyncRepo(ctx context.Context, userID, repoID uint64) (uint64, bool, error) {
	if _, err := s.ValidateAccess(ctx, userID, repoID); err != nil {
		return 0, false, err
	}

	jobID, deduped, err := s.jobEnqueuer.Enqueue(ctx, JobTypeSync, repoID, nil, userID)
	if err != nil {
		return 0, false, fmt.Errorf("enqueueing sync job: %w", err)
	}

	return jobID, deduped, nil
}

func (s *repoService) RemoveRepo(ctx context.Context, userID, repoID uint64) error {
	ur, err := s.userRepoRepo.FindByUserAndRepo(ctx, userID, repoID)
	if err != nil {
		return err
	}

	return s.userRepoRepo.Delete(ctx, ur.ID)
}

func (s *repoService) RecheckAccess(ctx context.Context, userID, repoID uint64) (string, error) {
	ur, err := s.userRepoRepo.FindByUserAndRepo(ctx, userID, repoID)
	if err != nil {
		return "", err
	}

	repo, err := s.repoRepo.FindByID(ctx, repoID)
	if err != nil {
		return "", err
	}

	if !repo.IsPrivate {
		ur.AccessState = "active"
		now := time.Now()
		ur.LastAccessVerifiedAt = now
		_ = s.userRepoRepo.Update(ctx, ur)
		return "active", nil
	}

	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}

	result := s.checkGitHubAccess(ctx, u, repo)
	switch result {
	case "accessible":
		ur.AccessState = "active"
		now := time.Now()
		ur.LastAccessVerifiedAt = now
	case "denied":
		ur.AccessState = "suspended"
	case "indeterminate":
		// No change
		return ur.AccessState, nil
	}

	_ = s.userRepoRepo.Update(ctx, ur)
	return ur.AccessState, nil
}

func (s *repoService) ListUserRepos(ctx context.Context, userID uint64, state string, cursor uint64, limit int) ([]UserRepository, error) {
	if state == "" {
		state = "active"
	}
	return s.userRepoRepo.ListByUser(ctx, userID, state, cursor, limit)
}

func (s *repoService) ValidateAccess(ctx context.Context, userID, repoID uint64) (*UserRepository, error) {
	ur, err := s.userRepoRepo.FindByUserAndRepo(ctx, userID, repoID)
	if err != nil {
		if errors.Is(err, ErrUserRepoNotFound) {
			return nil, fmt.Errorf("forbidden: no access to repository")
		}
		return nil, err
	}

	if ur.AccessState == "suspended" {
		return nil, fmt.Errorf("forbidden: repository access suspended")
	}

	return ur, nil
}

func (s *repoService) checkGitHubAccess(ctx context.Context, u *user.User, repo *Repository) string {
	ghClient, err := s.ghFactory.NewClient(ctx, u.AccessToken)
	if err != nil {
		return "indeterminate"
	}

	_, resp, err := ghClient.Repositories.Get(ctx, repo.Owner, repo.Name)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return "denied"
		}
		if resp != nil && resp.StatusCode == 403 {
			return "denied"
		}
		return "indeterminate"
	}

	return "accessible"
}

func splitFullName(fullName string) [2]string {
	for i, c := range fullName {
		if c == '/' {
			return [2]string{fullName[:i], fullName[i+1:]}
		}
	}
	return [2]string{fullName, ""}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Compile-time check
var _ RepoService = (*repoService)(nil)

// Suppress unused import
var _ = (*github.Client)(nil)
var _ = auth.UserFromContext

// UserRepoService for access revalidation
type UserRepoService interface {
	RevalidatePrivateRepos(ctx context.Context, userID uint64) error
	RevalidateSingle(ctx context.Context, userID, repoID uint64) (string, error)
}

type userRepoService struct {
	userRepoRepo UserRepoRepository
	repoRepo     RepoRepository
	ghFactory    githubclient.GitHubClientFactory
	userRepo     user.Repository
	cfg          *config.AppConfig
	logger       *zap.Logger
}

func NewUserRepoService(
	userRepoRepo UserRepoRepository,
	repoRepo RepoRepository,
	ghFactory githubclient.GitHubClientFactory,
	userRepo user.Repository,
	cfg *config.AppConfig,
	logger *zap.Logger,
) UserRepoService {
	return &userRepoService{
		userRepoRepo: userRepoRepo,
		repoRepo:     repoRepo,
		ghFactory:    ghFactory,
		userRepo:     userRepo,
		cfg:          cfg,
		logger:       logger,
	}
}

func (s *userRepoService) RevalidatePrivateRepos(ctx context.Context, userID uint64) error {
	repos, err := s.userRepoRepo.ListByUser(ctx, userID, "all", 0, 1000)
	if err != nil {
		return err
	}

	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	ghClient, err := s.ghFactory.NewClient(ctx, u.AccessToken)
	if err != nil {
		return err
	}

	for _, ur := range repos {
		if ur.Repository == nil || !ur.Repository.IsPrivate {
			continue
		}

		_, resp, err := ghClient.Repositories.Get(ctx, ur.Repository.Owner, ur.Repository.Name)
		switch {
		case err == nil:
			ur.AccessState = "active"
			now := time.Now()
			ur.LastAccessVerifiedAt = now
		case resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 403):
			ur.AccessState = "suspended"
		default:
			continue // indeterminate
		}

		_ = s.userRepoRepo.Update(ctx, &ur)
	}

	return nil
}

func (s *userRepoService) RevalidateSingle(ctx context.Context, userID, repoID uint64) (string, error) {
	ur, err := s.userRepoRepo.FindByUserAndRepo(ctx, userID, repoID)
	if err != nil {
		return "", err
	}

	repo, err := s.repoRepo.FindByID(ctx, repoID)
	if err != nil {
		return "", err
	}

	if !repo.IsPrivate {
		return "active", nil
	}

	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}

	ghClient, err := s.ghFactory.NewClient(ctx, u.AccessToken)
	if err != nil {
		return "indeterminate", nil
	}

	_, resp, err := ghClient.Repositories.Get(ctx, repo.Owner, repo.Name)
	switch {
	case err == nil:
		ur.AccessState = "active"
		now := time.Now()
		ur.LastAccessVerifiedAt = now
		_ = s.userRepoRepo.Update(ctx, ur)
		return "active", nil
	case resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 403):
		ur.AccessState = "suspended"
		_ = s.userRepoRepo.Update(ctx, ur)
		return "suspended", nil
	default:
		return "indeterminate", nil
	}
}

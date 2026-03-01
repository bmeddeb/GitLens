package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrRepoNotFound     = errors.New("repository not found")
	ErrUserRepoNotFound = errors.New("user repository association not found")
)

type RepoRepository interface {
	FindByID(ctx context.Context, id uint64) (*Repository, error)
	FindByFullName(ctx context.Context, fullName string) (*Repository, error)
	FindByGitHubID(ctx context.Context, githubID uint64) (*Repository, error)
	Create(ctx context.Context, repo *Repository) error
	Update(ctx context.Context, repo *Repository) error
	Delete(ctx context.Context, id uint64) error
}

type UserRepoRepository interface {
	FindByUserAndRepo(ctx context.Context, userID, repoID uint64) (*UserRepository, error)
	ListByUser(ctx context.Context, userID uint64, state string, cursor uint64, limit int) ([]UserRepository, error)
	Create(ctx context.Context, ur *UserRepository) error
	Update(ctx context.Context, ur *UserRepository) error
	Delete(ctx context.Context, id uint64) error
}

type CommitRepository interface {
	Create(ctx context.Context, commit *Commit) error
	CreateBatch(ctx context.Context, commits []Commit) error
	FindBySHA(ctx context.Context, repoID uint64, sha string) (*Commit, error)
	List(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Commit, error)
	ListFiltered(ctx context.Context, repoID uint64, opts CommitFilterOptions) ([]Commit, error)
}

type CommitFilterOptions struct {
	Author  string
	Path    string
	Keyword string
	Cursor  uint64
	Limit   int
}

type ForkRepository interface {
	FindByID(ctx context.Context, id uint64) (*Fork, error)
	FindByGitHubForkID(ctx context.Context, githubForkID uint64) (*Fork, error)
	ListBySourceRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Fork, error)
	Create(ctx context.Context, fork *Fork) error
	Update(ctx context.Context, fork *Fork) error
	CreateForkChange(ctx context.Context, fc *ForkChange) error
	CreateForkChangeBatch(ctx context.Context, changes []ForkChange) error
	ListForkChanges(ctx context.Context, forkID uint64, cursor uint64, limit int) ([]ForkChange, error)
}

type BlameCacheRepository interface {
	Get(ctx context.Context, repoID uint64, filePath, commitSHA string) (*BlameCache, error)
	Store(ctx context.Context, bc *BlameCache) error
}

type ContributorRepository interface {
	FindIdentityByEmail(ctx context.Context, email string) (*ContributorIdentity, error)
	FindAliasByEmail(ctx context.Context, email string) (*ContributorAlias, error)
	CreateIdentity(ctx context.Context, identity *ContributorIdentity) error
	CreateAlias(ctx context.Context, alias *ContributorAlias) error
	UpsertStats(ctx context.Context, stat *ContributorStat) error
	ListStatsByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]ContributorStat, error)
}

// Implementations

type repoRepository struct{ db *gorm.DB }

func NewRepoRepository(db *gorm.DB) RepoRepository { return &repoRepository{db: db} }

func (r *repoRepository) FindByID(ctx context.Context, id uint64) (*Repository, error) {
	var repo Repository
	if err := r.db.WithContext(ctx).First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRepoNotFound
		}
		return nil, fmt.Errorf("finding repo: %w", err)
	}
	return &repo, nil
}

func (r *repoRepository) FindByFullName(ctx context.Context, fullName string) (*Repository, error) {
	var repo Repository
	if err := r.db.WithContext(ctx).Where("full_name = ?", fullName).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRepoNotFound
		}
		return nil, fmt.Errorf("finding repo by full_name: %w", err)
	}
	return &repo, nil
}

func (r *repoRepository) FindByGitHubID(ctx context.Context, githubID uint64) (*Repository, error) {
	var repo Repository
	if err := r.db.WithContext(ctx).Where("github_id = ?", githubID).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRepoNotFound
		}
		return nil, fmt.Errorf("finding repo by github_id: %w", err)
	}
	return &repo, nil
}

func (r *repoRepository) Create(ctx context.Context, repo *Repository) error {
	return r.db.WithContext(ctx).Create(repo).Error
}

func (r *repoRepository) Update(ctx context.Context, repo *Repository) error {
	return r.db.WithContext(ctx).Save(repo).Error
}

func (r *repoRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&Repository{}, id).Error
}

// UserRepo repository
type userRepoRepository struct{ db *gorm.DB }

func NewUserRepoRepository(db *gorm.DB) UserRepoRepository { return &userRepoRepository{db: db} }

func (r *userRepoRepository) FindByUserAndRepo(ctx context.Context, userID, repoID uint64) (*UserRepository, error) {
	var ur UserRepository
	if err := r.db.WithContext(ctx).Where("user_id = ? AND repo_id = ?", userID, repoID).Preload("Repository").First(&ur).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserRepoNotFound
		}
		return nil, fmt.Errorf("finding user_repository: %w", err)
	}
	return &ur, nil
}

func (r *userRepoRepository) ListByUser(ctx context.Context, userID uint64, state string, cursor uint64, limit int) ([]UserRepository, error) {
	query := r.db.WithContext(ctx).Preload("Repository").Where("user_id = ?", userID)
	if state != "" && state != "all" {
		query = query.Where("access_state = ?", state)
	}
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var repos []UserRepository
	if err := query.Order("id ASC").Limit(limit + 1).Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("listing user repos: %w", err)
	}
	return repos, nil
}

func (r *userRepoRepository) Create(ctx context.Context, ur *UserRepository) error {
	return r.db.WithContext(ctx).Create(ur).Error
}

func (r *userRepoRepository) Update(ctx context.Context, ur *UserRepository) error {
	return r.db.WithContext(ctx).Save(ur).Error
}

func (r *userRepoRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&UserRepository{}, id).Error
}

// Commit repository
type commitRepository struct{ db *gorm.DB }

func NewCommitRepository(db *gorm.DB) CommitRepository { return &commitRepository{db: db} }

func (r *commitRepository) Create(ctx context.Context, commit *Commit) error {
	return r.db.WithContext(ctx).Create(commit).Error
}

func (r *commitRepository) CreateBatch(ctx context.Context, commits []Commit) error {
	if len(commits) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(commits, 100).Error
}

func (r *commitRepository) FindBySHA(ctx context.Context, repoID uint64, sha string) (*Commit, error) {
	var c Commit
	if err := r.db.WithContext(ctx).Where("repo_id = ? AND sha = ?", repoID, sha).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *commitRepository) List(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Commit, error) {
	var commits []Commit
	query := r.db.WithContext(ctx).Where("repo_id = ?", repoID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	if err := query.Order("committed_at DESC").Limit(limit + 1).Find(&commits).Error; err != nil {
		return nil, err
	}
	return commits, nil
}

func (r *commitRepository) ListFiltered(ctx context.Context, repoID uint64, opts CommitFilterOptions) ([]Commit, error) {
	query := r.db.WithContext(ctx).Where("repo_id = ?", repoID)
	if opts.Author != "" {
		query = query.Where("author_email LIKE ?", "%"+opts.Author+"%")
	}
	if opts.Keyword != "" {
		query = query.Where("message LIKE ?", "%"+opts.Keyword+"%")
	}
	if opts.Cursor > 0 {
		query = query.Where("id > ?", opts.Cursor)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	var commits []Commit
	if err := query.Order("committed_at DESC").Limit(limit + 1).Find(&commits).Error; err != nil {
		return nil, err
	}
	return commits, nil
}

// Fork repository
type forkRepository struct{ db *gorm.DB }

func NewForkRepository(db *gorm.DB) ForkRepository { return &forkRepository{db: db} }

func (r *forkRepository) FindByID(ctx context.Context, id uint64) (*Fork, error) {
	var f Fork
	if err := r.db.WithContext(ctx).First(&f, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *forkRepository) FindByGitHubForkID(ctx context.Context, githubForkID uint64) (*Fork, error) {
	var f Fork
	if err := r.db.WithContext(ctx).Where("github_fork_id = ?", githubForkID).First(&f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *forkRepository) ListBySourceRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Fork, error) {
	query := r.db.WithContext(ctx).Where("source_repo_id = ?", repoID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var forks []Fork
	if err := query.Order("id ASC").Limit(limit + 1).Find(&forks).Error; err != nil {
		return nil, err
	}
	return forks, nil
}

func (r *forkRepository) Create(ctx context.Context, fork *Fork) error {
	return r.db.WithContext(ctx).Create(fork).Error
}

func (r *forkRepository) Update(ctx context.Context, fork *Fork) error {
	return r.db.WithContext(ctx).Save(fork).Error
}

func (r *forkRepository) CreateForkChange(ctx context.Context, fc *ForkChange) error {
	return r.db.WithContext(ctx).Create(fc).Error
}

func (r *forkRepository) CreateForkChangeBatch(ctx context.Context, changes []ForkChange) error {
	if len(changes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(changes, 100).Error
}

func (r *forkRepository) ListForkChanges(ctx context.Context, forkID uint64, cursor uint64, limit int) ([]ForkChange, error) {
	query := r.db.WithContext(ctx).Where("fork_id = ?", forkID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var changes []ForkChange
	if err := query.Order("id ASC").Limit(limit + 1).Find(&changes).Error; err != nil {
		return nil, err
	}
	return changes, nil
}

// BlameCache repository
type blameCacheRepository struct{ db *gorm.DB }

func NewBlameCacheRepository(db *gorm.DB) BlameCacheRepository {
	return &blameCacheRepository{db: db}
}

func (r *blameCacheRepository) Get(ctx context.Context, repoID uint64, filePath, commitSHA string) (*BlameCache, error) {
	var bc BlameCache
	if err := r.db.WithContext(ctx).
		Where("repo_id = ? AND file_path = ? AND commit_sha = ?", repoID, filePath, commitSHA).
		First(&bc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bc, nil
}

func (r *blameCacheRepository) Store(ctx context.Context, bc *BlameCache) error {
	return r.db.WithContext(ctx).Create(bc).Error
}

// Contributor repository
type contributorRepository struct{ db *gorm.DB }

func NewContributorRepository(db *gorm.DB) ContributorRepository {
	return &contributorRepository{db: db}
}

func (r *contributorRepository) FindIdentityByEmail(ctx context.Context, email string) (*ContributorIdentity, error) {
	var ci ContributorIdentity
	if err := r.db.WithContext(ctx).Where("canonical_email = ?", email).First(&ci).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ci, nil
}

func (r *contributorRepository) FindAliasByEmail(ctx context.Context, email string) (*ContributorAlias, error) {
	var ca ContributorAlias
	if err := r.db.WithContext(ctx).Where("observed_email = ?", email).First(&ca).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ca, nil
}

func (r *contributorRepository) CreateIdentity(ctx context.Context, identity *ContributorIdentity) error {
	return r.db.WithContext(ctx).Create(identity).Error
}

func (r *contributorRepository) CreateAlias(ctx context.Context, alias *ContributorAlias) error {
	return r.db.WithContext(ctx).Create(alias).Error
}

func (r *contributorRepository) UpsertStats(ctx context.Context, stat *ContributorStat) error {
	return r.db.WithContext(ctx).
		Where("repo_id = ? AND identity_id = ?", stat.RepoID, stat.IdentityID).
		Assign(map[string]interface{}{
			"commits_count":  gorm.Expr("commits_count + ?", stat.CommitsCount),
			"insertions":     gorm.Expr("insertions + ?", stat.Insertions),
			"deletions":      gorm.Expr("deletions + ?", stat.Deletions),
			"last_commit_at": stat.LastCommitAt,
		}).
		FirstOrCreate(stat).Error
}

func (r *contributorRepository) ListStatsByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]ContributorStat, error) {
	query := r.db.WithContext(ctx).Preload("Identity").Where("repo_id = ?", repoID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var stats []ContributorStat
	if err := query.Order("commits_count DESC").Limit(limit + 1).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

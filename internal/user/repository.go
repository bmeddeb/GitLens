package user

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type Repository interface {
	FindByGitHubID(ctx context.Context, githubID uint64) (*User, error)
	FindByID(ctx context.Context, id uint64) (*User, error)
	FindByIDs(ctx context.Context, ids []uint64) ([]User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindByGitHubID(ctx context.Context, githubID uint64) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("github_id = ?", githubID).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("finding user by github_id: %w", err)
	}
	return &u, nil
}

func (r *repository) FindByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return &u, nil
}

func (r *repository) FindByIDs(ctx context.Context, ids []uint64) ([]User, error) {
	var users []User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("finding users by ids: %w", err)
	}
	return users, nil
}

func (r *repository) Create(ctx context.Context, u *User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *repository) Update(ctx context.Context, u *User) error {
	if err := r.db.WithContext(ctx).Save(u).Error; err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

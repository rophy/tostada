package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type UserStore interface {
	GetUser(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	EnsureUser(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, username string, updates map[string]any) error
	DeleteUser(ctx context.Context, username string) error
	IsAdmin(ctx context.Context, username string) (bool, error)
}

type GormUserStore struct {
	db *gorm.DB
}

func NewGormUserStore(db *gorm.DB) *GormUserStore {
	return &GormUserStore{db: db}
}

func (s *GormUserStore) GetUser(_ context.Context, username string) (*User, error) {
	var u User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *GormUserStore) ListUsers(_ context.Context) ([]User, error) {
	var users []User
	err := s.db.Order("username").Find(&users).Error
	return users, err
}

func (s *GormUserStore) EnsureUser(_ context.Context, username string) (*User, error) {
	var u User
	result := s.db.Where("username = ?", username).First(&u)
	if result.Error != nil {
		u = User{Username: username, LastLogin: time.Now().UTC()}
		if err := s.db.Create(&u).Error; err != nil {
			return nil, err
		}
		return &u, nil
	}
	s.db.Model(&u).Update("last_login", time.Now().UTC())
	return &u, nil
}

func (s *GormUserStore) UpdateUser(_ context.Context, username string, updates map[string]any) error {
	return s.db.Model(&User{}).Where("username = ?", username).Updates(updates).Error
}

func (s *GormUserStore) DeleteUser(_ context.Context, username string) error {
	return s.db.Where("username = ?", username).Delete(&User{}).Error
}

func (s *GormUserStore) IsAdmin(_ context.Context, username string) (bool, error) {
	var u User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return false, nil
	}
	return u.IsAdmin, nil
}

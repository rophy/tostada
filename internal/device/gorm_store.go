package device

import (
	"context"
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type GormStore struct {
	db *gorm.DB
}

type Option func(*logger.LogLevel)

func WithSilentLogger() Option {
	return func(l *logger.LogLevel) { *l = logger.Silent }
}

func NewGormStore(dbPath string, opts ...Option) (*GormStore, error) {
	logLevel := logger.Warn
	for _, o := range opts {
		o(&logLevel)
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.AutoMigrate(&Device{}, &UserAccess{}); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return &GormStore{db: db}, nil
}

func (s *GormStore) DB() *gorm.DB {
	return s.db
}

func (s *GormStore) ListDevices(_ context.Context, username string) ([]Device, error) {
	var devices []Device
	err := s.db.
		Joins("JOIN user_accesses ON user_accesses.device_id = devices.id").
		Where("user_accesses.username = ?", username).
		Find(&devices).Error
	return devices, err
}

func (s *GormStore) GetDevice(_ context.Context, username string, name string) (*Device, error) {
	var d Device
	err := s.db.
		Joins("JOIN user_accesses ON user_accesses.device_id = devices.id").
		Where("user_accesses.username = ? AND devices.name = ?", username, name).
		First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

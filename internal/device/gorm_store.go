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

func (s *GormStore) ListAllDevices(_ context.Context) ([]DeviceWithGrants, error) {
	var devices []Device
	if err := s.db.Find(&devices).Error; err != nil {
		return nil, err
	}
	result := make([]DeviceWithGrants, len(devices))
	for i, d := range devices {
		var accesses []UserAccess
		s.db.Where("device_id = ?", d.ID).Find(&accesses)
		grants := make([]string, len(accesses))
		for j, a := range accesses {
			grants[j] = a.Username
		}
		result[i] = DeviceWithGrants{Device: d, Grants: grants}
	}
	return result, nil
}

func (s *GormStore) CreateDevice(_ context.Context, d *Device) error {
	return s.db.Create(d).Error
}

func (s *GormStore) UpdateDevice(_ context.Context, name string, updates map[string]any) error {
	return s.db.Model(&Device{}).Where("name = ?", name).Updates(updates).Error
}

func (s *GormStore) DeleteDevice(_ context.Context, name string) error {
	var d Device
	if err := s.db.Where("name = ?", name).First(&d).Error; err != nil {
		return err
	}
	s.db.Where("device_id = ?", d.ID).Delete(&UserAccess{})
	return s.db.Delete(&d).Error
}

func (s *GormStore) GrantAccess(_ context.Context, deviceName string, username string) error {
	var d Device
	if err := s.db.Where("name = ?", deviceName).First(&d).Error; err != nil {
		return err
	}
	return s.db.Create(&UserAccess{Username: username, DeviceID: d.ID}).Error
}

func (s *GormStore) RevokeAccess(_ context.Context, deviceName string, username string) error {
	var d Device
	if err := s.db.Where("name = ?", deviceName).First(&d).Error; err != nil {
		return err
	}
	return s.db.Where("username = ? AND device_id = ?", username, d.ID).Delete(&UserAccess{}).Error
}

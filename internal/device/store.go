package device

import "context"

type Device struct {
	ID       uint   `gorm:"primarykey" json:"-"`
	Name     string `gorm:"uniqueIndex;not null" json:"name"`
	Display  string `gorm:"not null" json:"displayName"`
	Protocol string `gorm:"not null" json:"protocol"`
	Host     string `gorm:"not null" json:"host"`
	Port     int    `gorm:"not null" json:"port"`
	Username string `gorm:"not null" json:"-"`
	Password string `gorm:"not null" json:"-"`
}

type UserAccess struct {
	ID       uint   `gorm:"primarykey"`
	Username string `gorm:"index:idx_user_device,unique;not null"`
	DeviceID uint   `gorm:"index:idx_user_device,unique;not null"`
	Device   Device `gorm:"foreignKey:DeviceID"`
}

type Store interface {
	ListDevices(ctx context.Context, username string) ([]Device, error)
	GetDevice(ctx context.Context, username string, name string) (*Device, error)
}

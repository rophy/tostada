package device

import (
	"log"

	"github.com/rophy/tostada/internal/config"
)

func (s *GormStore) SyncFromConfig(devices []config.DeviceConfig) error {
	for _, dc := range devices {
		var d Device
		result := s.db.Where("name = ?", dc.Name).First(&d)
		if result.Error != nil {
			d = Device{
				Name:     dc.Name,
				Display:  dc.DisplayName,
				Protocol: dc.Protocol,
				Host:     dc.Host,
				Port:     dc.Port,
				Username: dc.Username,
				Password: dc.Password,
			}
			if err := s.db.Create(&d).Error; err != nil {
				return err
			}
			log.Printf("Device %q created", dc.Name)
		} else {
			d.Display = dc.DisplayName
			d.Protocol = dc.Protocol
			d.Host = dc.Host
			d.Port = dc.Port
			d.Username = dc.Username
			d.Password = dc.Password
			s.db.Save(&d)
			log.Printf("Device %q updated", dc.Name)
		}

		for _, username := range dc.AllowedUsers {
			var access UserAccess
			result := s.db.Where("username = ? AND device_id = ?", username, d.ID).First(&access)
			if result.Error != nil {
				s.db.Create(&UserAccess{Username: username, DeviceID: d.ID})
			}
		}
	}
	return nil
}

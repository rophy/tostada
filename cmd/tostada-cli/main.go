package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rophy/tostada/internal/device"
	"gopkg.in/yaml.v3"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: tostada-cli <command> [args]

Commands:
  device list                                   List all devices
  device add <name> <display> <proto> <host> <port> <user> <pass>  Add a device
  device remove <name>                          Remove a device
  device grant <device> <username>              Grant user access
  device revoke <device> <username>             Revoke user access
  device import <file.yaml>                     Import devices from YAML
  device access <device>                        List users with access

Environment:
  TOSTADA_DB   Path to SQLite database (default: tostada.db)
`)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		usage()
	}

	dbPath := os.Getenv("TOSTADA_DB")
	if dbPath == "" {
		dbPath = "tostada.db"
	}

	store, err := device.NewGormStore(dbPath, device.WithSilentLogger())
	if err != nil {
		fatal("open database: %v", err)
	}

	if os.Args[1] != "device" {
		usage()
	}

	switch os.Args[2] {
	case "list":
		cmdList(store)
	case "add":
		cmdAdd(store)
	case "remove":
		cmdRemove(store)
	case "grant":
		cmdGrant(store)
	case "revoke":
		cmdRevoke(store)
	case "import":
		cmdImport(store)
	case "access":
		cmdAccess(store)
	default:
		usage()
	}
}

func cmdList(store *device.GormStore) {
	var devices []device.Device
	store.DB().Find(&devices)
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	fmt.Printf("%-20s %-25s %-8s %-20s %s\n", "NAME", "DISPLAY", "PROTO", "HOST", "PORT")
	for _, d := range devices {
		fmt.Printf("%-20s %-25s %-8s %-20s %d\n", d.Name, d.Display, d.Protocol, d.Host, d.Port)
	}
}

func cmdAdd(store *device.GormStore) {
	if len(os.Args) < 10 {
		fatal("usage: tostada-cli device add <name> <display> <proto> <host> <port> <user> <pass>")
	}
	port, err := strconv.Atoi(os.Args[7])
	if err != nil {
		fatal("invalid port: %v", err)
	}
	d := device.Device{
		Name:     os.Args[3],
		Display:  os.Args[4],
		Protocol: os.Args[5],
		Host:     os.Args[6],
		Port:     port,
		Username: os.Args[8],
		Password: os.Args[9],
	}
	if err := store.DB().Create(&d).Error; err != nil {
		fatal("add device: %v", err)
	}
	fmt.Printf("Device %q added.\n", d.Name)
}

func cmdRemove(store *device.GormStore) {
	if len(os.Args) < 4 {
		fatal("usage: tostada-cli device remove <name>")
	}
	name := os.Args[3]
	var d device.Device
	if err := store.DB().Where("name = ?", name).First(&d).Error; err != nil {
		fatal("device %q not found", name)
	}
	store.DB().Where("device_id = ?", d.ID).Delete(&device.UserAccess{})
	store.DB().Delete(&d)
	fmt.Printf("Device %q removed.\n", name)
}

func cmdGrant(store *device.GormStore) {
	if len(os.Args) < 5 {
		fatal("usage: tostada-cli device grant <device> <username>")
	}
	devName, username := os.Args[3], os.Args[4]
	var d device.Device
	if err := store.DB().Where("name = ?", devName).First(&d).Error; err != nil {
		fatal("device %q not found", devName)
	}
	var existing device.UserAccess
	if err := store.DB().Where("username = ? AND device_id = ?", username, d.ID).First(&existing).Error; err == nil {
		fmt.Printf("User %q already has access to %q.\n", username, devName)
		return
	}
	store.DB().Create(&device.UserAccess{Username: username, DeviceID: d.ID})
	fmt.Printf("Granted %q access to %q.\n", username, devName)
}

func cmdRevoke(store *device.GormStore) {
	if len(os.Args) < 5 {
		fatal("usage: tostada-cli device revoke <device> <username>")
	}
	devName, username := os.Args[3], os.Args[4]
	var d device.Device
	if err := store.DB().Where("name = ?", devName).First(&d).Error; err != nil {
		fatal("device %q not found", devName)
	}
	result := store.DB().Where("username = ? AND device_id = ?", username, d.ID).Delete(&device.UserAccess{})
	if result.RowsAffected == 0 {
		fmt.Printf("User %q has no access to %q.\n", username, devName)
		return
	}
	fmt.Printf("Revoked %q access from %q.\n", username, devName)
}

func cmdAccess(store *device.GormStore) {
	if len(os.Args) < 4 {
		fatal("usage: tostada-cli device access <device>")
	}
	devName := os.Args[3]
	var d device.Device
	if err := store.DB().Where("name = ?", devName).First(&d).Error; err != nil {
		fatal("device %q not found", devName)
	}
	var accesses []device.UserAccess
	store.DB().Where("device_id = ?", d.ID).Find(&accesses)
	if len(accesses) == 0 {
		fmt.Println("No users have access.")
		return
	}
	for _, a := range accesses {
		fmt.Println(a.Username)
	}
}

type importFile struct {
	Devices []importDevice `yaml:"devices"`
}

type importDevice struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"displayName"`
	Protocol     string   `yaml:"protocol"`
	Host         string   `yaml:"host"`
	Port         int      `yaml:"port"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	AllowedUsers []string `yaml:"allowedUsers"`
}

func cmdImport(store *device.GormStore) {
	if len(os.Args) < 4 {
		fatal("usage: tostada-cli device import <file.yaml>")
	}
	data, err := os.ReadFile(os.Args[3])
	if err != nil {
		fatal("read file: %v", err)
	}
	var f importFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		fatal("parse yaml: %v", err)
	}

	for _, dc := range f.Devices {
		var d device.Device
		result := store.DB().Where("name = ?", dc.Name).First(&d)
		if result.Error != nil {
			d = device.Device{
				Name:     dc.Name,
				Display:  dc.DisplayName,
				Protocol: dc.Protocol,
				Host:     dc.Host,
				Port:     dc.Port,
				Username: dc.Username,
				Password: dc.Password,
			}
			store.DB().Create(&d)
			fmt.Printf("Created device %q\n", dc.Name)
		} else {
			d.Display = dc.DisplayName
			d.Protocol = dc.Protocol
			d.Host = dc.Host
			d.Port = dc.Port
			d.Username = dc.Username
			d.Password = dc.Password
			store.DB().Save(&d)
			fmt.Printf("Updated device %q\n", dc.Name)
		}

		for _, username := range dc.AllowedUsers {
			var access device.UserAccess
			if store.DB().Where("username = ? AND device_id = ?", username, d.ID).First(&access).Error != nil {
				store.DB().Create(&device.UserAccess{Username: username, DeviceID: d.ID})
				fmt.Printf("  Granted %q access\n", username)
			}
		}
	}
	fmt.Printf("Import complete: %d device(s) processed.\n", len(f.Devices))
}

func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(os.Stderr, msg)
	os.Exit(1)
}

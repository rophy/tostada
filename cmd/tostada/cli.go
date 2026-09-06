package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/model"
	"gopkg.in/yaml.v3"
)

func cliUsage() {
	fmt.Fprintf(os.Stderr, `Usage: tostada <command> [args]

Commands:
  serve                                           Start the HTTP server
  device list                                     List all devices
  device add <name> <display> <proto> <host> <port> <user> <pass>  Add a device
  device remove <name>                            Remove a device
  device grant <device> <username>                Grant user access
  device revoke <device> <username>               Revoke user access
  device import <file.yaml>                       Import devices from YAML
  device access <device>                          List users with access
  user list                                       List all users
  user set-admin <username> <true|false>          Set admin flag
  user delete <username>                          Remove user

Environment:
  TOSTADA_DB   Path to SQLite database (default: tostada.db)
`)
	os.Exit(1)
}

func runCLI(args []string) {
	if len(args) < 2 {
		cliUsage()
	}

	dbPath := os.Getenv("TOSTADA_DB")
	if dbPath == "" {
		dbPath = "tostada.db"
	}

	store, err := device.NewGormStore(dbPath, device.WithSilentLogger())
	if err != nil {
		fatal("open database: %v", err)
	}
	store.DB().AutoMigrate(&model.User{})

	switch args[0] {
	case "device":
		switch args[1] {
		case "list":
			cmdList(store)
		case "add":
			cmdAdd(store, args)
		case "remove":
			cmdRemove(store, args)
		case "grant":
			cmdGrant(store, args)
		case "revoke":
			cmdRevoke(store, args)
		case "import":
			cmdImport(store, args)
		case "access":
			cmdAccess(store, args)
		default:
			cliUsage()
		}
	case "user":
		switch args[1] {
		case "list":
			cmdUserList(store)
		case "set-admin":
			cmdUserSetAdmin(store, args)
		case "delete":
			cmdUserDelete(store, args)
		default:
			cliUsage()
		}
	default:
		cliUsage()
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

func cmdAdd(store *device.GormStore, args []string) {
	if len(args) < 9 {
		fatal("usage: tostada device add <name> <display> <proto> <host> <port> <user> <pass>")
	}
	port, err := strconv.Atoi(args[6])
	if err != nil {
		fatal("invalid port: %v", err)
	}
	d := device.Device{
		Name:     args[2],
		Display:  args[3],
		Protocol: args[4],
		Host:     args[5],
		Port:     port,
		Username: args[7],
		Password: args[8],
	}
	if err := store.DB().Create(&d).Error; err != nil {
		fatal("add device: %v", err)
	}
	fmt.Printf("Device %q added.\n", d.Name)
}

func cmdRemove(store *device.GormStore, args []string) {
	if len(args) < 3 {
		fatal("usage: tostada device remove <name>")
	}
	name := args[2]
	var d device.Device
	if err := store.DB().Where("name = ?", name).First(&d).Error; err != nil {
		fatal("device %q not found", name)
	}
	store.DB().Where("device_id = ?", d.ID).Delete(&device.UserAccess{})
	store.DB().Delete(&d)
	fmt.Printf("Device %q removed.\n", name)
}

func cmdGrant(store *device.GormStore, args []string) {
	if len(args) < 4 {
		fatal("usage: tostada device grant <device> <username>")
	}
	devName, username := args[2], args[3]
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

func cmdRevoke(store *device.GormStore, args []string) {
	if len(args) < 4 {
		fatal("usage: tostada device revoke <device> <username>")
	}
	devName, username := args[2], args[3]
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

func cmdAccess(store *device.GormStore, args []string) {
	if len(args) < 3 {
		fatal("usage: tostada device access <device>")
	}
	devName := args[2]
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

func cmdImport(store *device.GormStore, args []string) {
	if len(args) < 3 {
		fatal("usage: tostada device import <file.yaml>")
	}
	data, err := os.ReadFile(args[2])
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

func cmdUserList(store *device.GormStore) {
	var users []model.User
	store.DB().Order("username").Find(&users)
	if len(users) == 0 {
		fmt.Println("No users.")
		return
	}
	fmt.Printf("%-20s %-8s %s\n", "USERNAME", "ADMIN", "LAST LOGIN")
	for _, u := range users {
		admin := "no"
		if u.IsAdmin {
			admin = "yes"
		}
		lastLogin := "never"
		if !u.LastLogin.IsZero() {
			lastLogin = u.LastLogin.Format("2006-01-02 15:04")
		}
		fmt.Printf("%-20s %-8s %s\n", u.Username, admin, lastLogin)
	}
}

func cmdUserSetAdmin(store *device.GormStore, args []string) {
	if len(args) < 4 {
		fatal("usage: tostada user set-admin <username> <true|false>")
	}
	username := args[2]
	isAdmin := args[3] == "true"

	var u model.User
	if store.DB().Where("username = ?", username).First(&u).Error != nil {
		u = model.User{Username: username, IsAdmin: isAdmin}
		store.DB().Create(&u)
		fmt.Printf("Created user %q (admin=%v).\n", username, isAdmin)
		return
	}

	store.DB().Model(&u).Update("is_admin", isAdmin)
	fmt.Printf("Updated user %q (admin=%v).\n", username, isAdmin)
}

func cmdUserDelete(store *device.GormStore, args []string) {
	if len(args) < 3 {
		fatal("usage: tostada user delete <username>")
	}
	username := args[2]
	result := store.DB().Where("username = ?", username).Delete(&model.User{})
	if result.RowsAffected == 0 {
		fmt.Printf("User %q not found.\n", username)
		return
	}
	store.DB().Where("username = ?", username).Delete(&device.UserAccess{})
	fmt.Printf("User %q deleted.\n", username)
}

func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(os.Stderr, msg)
	os.Exit(1)
}

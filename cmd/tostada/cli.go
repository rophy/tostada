package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/model"
)

func openStore() *device.GormStore {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_DSN environment variable is required")
		os.Exit(1)
	}
	store, err := device.NewGormStorePostgres(dsn, device.WithSilentLogger())
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}
	store.DB().AutoMigrate(&model.User{})
	return store
}

func deviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage devices",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all devices",
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
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
			},
		},
		&cobra.Command{
			Use:   "add <name> <display> <proto> <host> <port> <user> <pass>",
			Short: "Add a device",
			Args:  cobra.ExactArgs(7),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				port, err := strconv.Atoi(args[4])
				if err != nil {
					fmt.Fprintf(os.Stderr, "invalid port: %v\n", err)
					os.Exit(1)
				}
				d := device.Device{
					Name: args[0], Display: args[1], Protocol: args[2],
					Host: args[3], Port: port, Username: args[5], Password: args[6],
				}
				if err := store.DB().Create(&d).Error; err != nil {
					fmt.Fprintf(os.Stderr, "add device: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Device %q added.\n", d.Name)
			},
		},
		&cobra.Command{
			Use:   "remove <name>",
			Short: "Remove a device",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				var d device.Device
				if err := store.DB().Where("name = ?", args[0]).First(&d).Error; err != nil {
					fmt.Fprintf(os.Stderr, "device %q not found\n", args[0])
					os.Exit(1)
				}
				store.DB().Where("device_id = ?", d.ID).Delete(&device.UserAccess{})
				store.DB().Delete(&d)
				fmt.Printf("Device %q removed.\n", args[0])
			},
		},
		&cobra.Command{
			Use:   "grant <device> <username>",
			Short: "Grant user access to a device",
			Args:  cobra.ExactArgs(2),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				var d device.Device
				if err := store.DB().Where("name = ?", args[0]).First(&d).Error; err != nil {
					fmt.Fprintf(os.Stderr, "device %q not found\n", args[0])
					os.Exit(1)
				}
				var existing device.UserAccess
				if store.DB().Where("username = ? AND device_id = ?", args[1], d.ID).First(&existing).Error == nil {
					fmt.Printf("User %q already has access to %q.\n", args[1], args[0])
					return
				}
				store.DB().Create(&device.UserAccess{Username: args[1], DeviceID: d.ID})
				fmt.Printf("Granted %q access to %q.\n", args[1], args[0])
			},
		},
		&cobra.Command{
			Use:   "revoke <device> <username>",
			Short: "Revoke user access from a device",
			Args:  cobra.ExactArgs(2),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				var d device.Device
				if err := store.DB().Where("name = ?", args[0]).First(&d).Error; err != nil {
					fmt.Fprintf(os.Stderr, "device %q not found\n", args[0])
					os.Exit(1)
				}
				result := store.DB().Where("username = ? AND device_id = ?", args[1], d.ID).Delete(&device.UserAccess{})
				if result.RowsAffected == 0 {
					fmt.Printf("User %q has no access to %q.\n", args[1], args[0])
					return
				}
				fmt.Printf("Revoked %q access from %q.\n", args[1], args[0])
			},
		},
		&cobra.Command{
			Use:   "access <device>",
			Short: "List users with access to a device",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				var d device.Device
				if err := store.DB().Where("name = ?", args[0]).First(&d).Error; err != nil {
					fmt.Fprintf(os.Stderr, "device %q not found\n", args[0])
					os.Exit(1)
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
			},
		},
		deviceImportCmd(),
	)
	return cmd
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

func deviceImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file.yaml>",
		Short: "Import devices from YAML",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			store := openStore()
			data, err := os.ReadFile(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "read file: %v\n", err)
				os.Exit(1)
			}
			var f importFile
			if err := yaml.Unmarshal(data, &f); err != nil {
				fmt.Fprintf(os.Stderr, "parse yaml: %v\n", err)
				os.Exit(1)
			}
			for _, dc := range f.Devices {
				var d device.Device
				result := store.DB().Where("name = ?", dc.Name).First(&d)
				if result.Error != nil {
					d = device.Device{
						Name: dc.Name, Display: dc.DisplayName, Protocol: dc.Protocol,
						Host: dc.Host, Port: dc.Port, Username: dc.Username, Password: dc.Password,
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
		},
	}
}

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all users",
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
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
			},
		},
		&cobra.Command{
			Use:   "set-admin <username> <true|false>",
			Short: "Set admin flag for a user",
			Args:  cobra.ExactArgs(2),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				username := args[0]
				isAdmin := args[1] == "true"
				var u model.User
				if store.DB().Where("username = ?", username).First(&u).Error != nil {
					u = model.User{Username: username, IsAdmin: isAdmin}
					store.DB().Create(&u)
					fmt.Printf("Created user %q (admin=%v).\n", username, isAdmin)
					return
				}
				store.DB().Model(&u).Update("is_admin", isAdmin)
				fmt.Printf("Updated user %q (admin=%v).\n", username, isAdmin)
			},
		},
		&cobra.Command{
			Use:   "delete <username>",
			Short: "Remove a user",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				store := openStore()
				result := store.DB().Where("username = ?", args[0]).Delete(&model.User{})
				if result.RowsAffected == 0 {
					fmt.Printf("User %q not found.\n", args[0])
					return
				}
				store.DB().Where("username = ?", args[0]).Delete(&device.UserAccess{})
				fmt.Printf("User %q deleted.\n", args[0])
			},
		},
	)
	return cmd
}

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	OIDC       OIDCConfig       `yaml:"oidc"`
	JupyterHub JupyterHubConfig `yaml:"jupyterhub"`
	Guacamole  GuacamoleConfig  `yaml:"guacamole"`
	Database   DatabaseConfig   `yaml:"database"`
	AuditLog   AuditLogConfig   `yaml:"auditLog"`
	Workspaces []Workspace      `yaml:"workspaces"`
}

type AuditLogConfig struct {
	LogDir     string `yaml:"logDir"`
	MaxSizeMB  int    `yaml:"maxSizeMB"`
	MaxBackups int    `yaml:"maxBackups"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type OIDCConfig struct {
	IssuerURL   string `yaml:"issuerURL"`
	InternalURL string `yaml:"internalURL"`
	ClientID    string `yaml:"clientID"`
	RedirectURL string `yaml:"redirectURL"`
}

type JupyterHubConfig struct {
	APIURL   string `yaml:"apiURL"`
	ProxyURL string `yaml:"proxyURL"`
}

type GuacamoleConfig struct {
	URL string `yaml:"url"`
}

type Workspace struct {
	Name           string         `yaml:"name" json:"name"`
	DisplayName    string         `yaml:"displayName" json:"displayName"`
	Description    string         `yaml:"description" json:"description"`
	Icon           string         `yaml:"icon" json:"icon"`
	Type           string         `yaml:"type" json:"type"`
	Image          string         `yaml:"image" json:"image"`
	Port           int            `yaml:"port" json:"port"`
	Cmd            []string       `yaml:"cmd" json:"cmd"`
	RDPCredentials RDPCredentials `yaml:"rdpCredentials" json:"-"`
}

type RDPCredentials struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

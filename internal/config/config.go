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
	Workspaces []Workspace      `yaml:"workspaces"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type OIDCConfig struct {
	IssuerURL    string `yaml:"issuerURL"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectURL"`
}

type JupyterHubConfig struct {
	APIURL   string `yaml:"apiURL"`
	APIToken string `yaml:"apiToken"`
}

type GuacamoleConfig struct {
	URL           string `yaml:"url"`
	JSONSecretKey string `yaml:"jsonSecretKey"`
}

type Workspace struct {
	Name           string         `yaml:"name"`
	DisplayName    string         `yaml:"displayName"`
	Description    string         `yaml:"description"`
	Icon           string         `yaml:"icon"`
	Type           string         `yaml:"type"`
	Image          string         `yaml:"image"`
	Port           int            `yaml:"port"`
	Cmd            []string       `yaml:"cmd"`
	RDPCredentials RDPCredentials `yaml:"rdpCredentials"`
}

type RDPCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
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

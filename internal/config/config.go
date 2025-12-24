package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type DNSServerConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Proto   string `yaml:"proto"`
}

type Config struct {
	StunServers []string          `yaml:"stun_servers"`
	DNSServers  []DNSServerConfig `yaml:"dns_servers"`
}

func Default() *Config {
	return &Config{
		StunServers: []string{
			"stun3.l.google.com:19302",
			"stun.l.google.com:19302",
		},
		DNSServers: []DNSServerConfig{},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		// Try default locations
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(home, ".lnd.yaml")
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

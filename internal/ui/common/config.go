package common

import (
	"os"
	"path/filepath"

	"github.com/goiriz/kaf/internal/config"
	"gopkg.in/yaml.v3"
)

// Re-export types for convenience if needed, but better use config package directly
type Config = config.Config
type Context = config.Context

func LoadConfig() (*config.Config, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".kaf.yaml")
	
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &config.Config{Contexts: make(map[string]config.Context)}, nil
		}
		return nil, err
	}
	defer f.Close()

	var cfg config.Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(c *config.Config) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".kaf.yaml")
	
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewEncoder(f).Encode(c)
}

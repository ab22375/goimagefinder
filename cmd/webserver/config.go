package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the webserver configuration
type Config struct {
	DatabasePath string  `json:"databasePath"`
	FolderPath   string  `json:"folderPath"`
	Threshold    float64 `json:"threshold"`
	Prefix       string  `json:"prefix"`
	ForceRewrite bool    `json:"forceRewrite"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		DatabasePath: filepath.Join(homeDir, "goimagefinder.db"),
		FolderPath:   "",
		Threshold:    0.75,
		Prefix:       "",
		ForceRewrite: false,
	}
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return config, nil
		}
		return nil, err
	}
	
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	
	return config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(configPath string, config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}

// GetConfigPath returns the default config file path
func GetConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".goimagefinder", "webserver.json")
}
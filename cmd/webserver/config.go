package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config represents the webserver configuration
type Config struct {
	Port         int      `json:"port"`
	DatabasePath string   `json:"databasePath"`
	FolderPath   string   `json:"folderPath"`
	Threshold    float64  `json:"threshold"`
	Prefix       string   `json:"prefix"`
	ForceRewrite bool     `json:"forceRewrite"`
	OpenBrowser  bool     `json:"openBrowser"`
	BrowseRoot   string   `json:"browseRoot"`              // Deprecated: use BrowseRoots
	BrowseRoots  []string `json:"browseRoots,omitempty"`   // Multiple browsable root paths
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Port:         8012,
		DatabasePath: filepath.Join(homeDir, "goimagefinder.db"),
		FolderPath:   "",
		Threshold:    0.75,
		Prefix:       "",
		ForceRewrite: false,
		OpenBrowser:  true,
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

// ApplyEnvironmentOverrides applies environment variable overrides to the config.
// Environment variables take precedence over config file values.
// Supported variables:
//   - GOIMAGEFINDER_PORT: HTTP port (e.g., "8012")
//   - GOIMAGEFINDER_DATABASE_PATH: Path to SQLite database
//   - GOIMAGEFINDER_BROWSE_ROOTS: Comma-separated list of browsable paths (e.g., "/photos,/external")
//   - GOIMAGEFINDER_THRESHOLD: Similarity threshold 0-1 (e.g., "0.75")
//   - GOIMAGEFINDER_OPEN_BROWSER: Whether to open browser on start ("true" or "false")
func ApplyEnvironmentOverrides(config *Config) {
	if port := os.Getenv("GOIMAGEFINDER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			config.Port = p
		}
	}

	if dbPath := os.Getenv("GOIMAGEFINDER_DATABASE_PATH"); dbPath != "" {
		config.DatabasePath = dbPath
	}

	if browseRoots := os.Getenv("GOIMAGEFINDER_BROWSE_ROOTS"); browseRoots != "" {
		roots := strings.Split(browseRoots, ",")
		// Trim whitespace from each root
		cleanRoots := make([]string, 0, len(roots))
		for _, root := range roots {
			root = strings.TrimSpace(root)
			if root != "" {
				cleanRoots = append(cleanRoots, root)
			}
		}
		if len(cleanRoots) > 0 {
			config.BrowseRoots = cleanRoots
			// Also set BrowseRoot for backward compatibility
			config.BrowseRoot = cleanRoots[0]
		}
	}

	if threshold := os.Getenv("GOIMAGEFINDER_THRESHOLD"); threshold != "" {
		if t, err := strconv.ParseFloat(threshold, 64); err == nil && t >= 0 && t <= 1 {
			config.Threshold = t
		}
	}

	if openBrowser := os.Getenv("GOIMAGEFINDER_OPEN_BROWSER"); openBrowser != "" {
		config.OpenBrowser = strings.ToLower(openBrowser) == "true"
	}
}

// GetEffectiveBrowseRoots returns the list of browse roots to use.
// Falls back to BrowseRoot (single) if BrowseRoots is empty.
func (c *Config) GetEffectiveBrowseRoots() []string {
	if len(c.BrowseRoots) > 0 {
		return c.BrowseRoots
	}
	if c.BrowseRoot != "" {
		return []string{c.BrowseRoot}
	}
	// Default fallback
	homeDir, _ := os.UserHomeDir()
	return []string{homeDir}
}
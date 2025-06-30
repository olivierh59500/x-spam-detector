package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"x-spam-detector/internal/detector"
)

// Config represents the application configuration
type Config struct {
	// Application settings
	App AppConfig `yaml:"app"`
	
	// Detection engine configuration
	Detection detector.Config `yaml:"detection"`
	
	// GUI settings
	GUI GUIConfig `yaml:"gui"`
	
	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// AppConfig holds general application settings
type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"` // development, production
	DataDir     string `yaml:"data_dir"`
	TempDir     string `yaml:"temp_dir"`
}

// GUIConfig holds GUI-specific settings
type GUIConfig struct {
	WindowWidth     int    `yaml:"window_width"`
	WindowHeight    int    `yaml:"window_height"`
	Theme           string `yaml:"theme"`           // light, dark, auto
	RefreshInterval int    `yaml:"refresh_interval"` // seconds
	MaxDisplayItems int    `yaml:"max_display_items"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Output     string `yaml:"output"`      // stdout, file, both
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`     // days
}

// Default returns a default configuration
func Default() *Config {
	return &Config{
		App: AppConfig{
			Name:        "X Spam Detector",
			Version:     "1.0.0",
			Environment: "development",
			DataDir:     "./data",
			TempDir:     "./temp",
		},
		Detection: detector.Config{
			// LSH Parameters - optimized for spam detection
			MinHashThreshold:    0.7,  // 70% similarity threshold
			SimHashThreshold:    3,    // Hamming distance threshold
			MinHashBands:        16,   // Number of LSH bands
			MinHashHashes:       128,  // Number of hash functions
			SimHashTables:       4,    // Number of SimHash tables
			
			// Clustering Parameters
			MinClusterSize:      3,    // Minimum tweets in a spam cluster
			MaxClusterAge:       24,   // Hours before cluster expires
			
			// Detection Thresholds
			SpamScoreThreshold:  0.6,  // Minimum confidence for spam classification
			BotScoreThreshold:   0.5,  // Minimum score to flag as bot account
			CoordinationThreshold: 5,  // Minimum cluster size to check coordination
			
			// Performance Settings
			BatchSize:           1000, // Process tweets in batches
			EnableHybridMode:    true, // Use both MinHash and SimHash
		},
		GUI: GUIConfig{
			WindowWidth:     1200,
			WindowHeight:    800,
			Theme:           "auto",
			RefreshInterval: 5,
			MaxDisplayItems: 1000,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Output:     "both",
			Filename:   "spam-detector.log",
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
		},
	}
}

// Load loads configuration from a YAML file
func Load(filename string) (*Config, error) {
	// Start with defaults
	config := Default()
	
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File doesn't exist, create it with defaults
		if err := config.Save(filename); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
		return config, nil
	}
	
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return config, nil
}

// Save saves the configuration to a YAML file
func (c *Config) Save(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// Validate validates the configuration values
func (c *Config) Validate() error {
	// Validate detection parameters
	if c.Detection.MinHashThreshold < 0.0 || c.Detection.MinHashThreshold > 1.0 {
		return fmt.Errorf("minhash_threshold must be between 0.0 and 1.0")
	}
	
	if c.Detection.SimHashThreshold < 0 || c.Detection.SimHashThreshold > 64 {
		return fmt.Errorf("simhash_threshold must be between 0 and 64")
	}
	
	if c.Detection.MinHashHashes <= 0 {
		return fmt.Errorf("minhash_hashes must be positive")
	}
	
	if c.Detection.MinHashBands <= 0 {
		return fmt.Errorf("minhash_bands must be positive")
	}
	
	if c.Detection.MinHashHashes%c.Detection.MinHashBands != 0 {
		return fmt.Errorf("minhash_hashes must be divisible by minhash_bands")
	}
	
	if c.Detection.MinClusterSize < 2 {
		return fmt.Errorf("min_cluster_size must be at least 2")
	}
	
	if c.Detection.SpamScoreThreshold < 0.0 || c.Detection.SpamScoreThreshold > 1.0 {
		return fmt.Errorf("spam_score_threshold must be between 0.0 and 1.0")
	}
	
	if c.Detection.BotScoreThreshold < 0.0 || c.Detection.BotScoreThreshold > 1.0 {
		return fmt.Errorf("bot_score_threshold must be between 0.0 and 1.0")
	}
	
	// Validate GUI parameters
	if c.GUI.WindowWidth < 800 {
		return fmt.Errorf("window_width must be at least 800")
	}
	
	if c.GUI.WindowHeight < 600 {
		return fmt.Errorf("window_height must be at least 600")
	}
	
	if c.GUI.RefreshInterval < 1 {
		return fmt.Errorf("refresh_interval must be at least 1 second")
	}
	
	// Validate theme
	validThemes := map[string]bool{"light": true, "dark": true, "auto": true}
	if !validThemes[c.GUI.Theme] {
		return fmt.Errorf("theme must be one of: light, dark, auto")
	}
	
	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("log level must be one of: debug, info, warn, error")
	}
	
	// Validate log output
	validOutputs := map[string]bool{"stdout": true, "file": true, "both": true}
	if !validOutputs[c.Logging.Output] {
		return fmt.Errorf("log output must be one of: stdout, file, both")
	}
	
	return nil
}

// GetDetectionConfig returns the detection engine configuration
func (c *Config) GetDetectionConfig() detector.Config {
	return c.Detection
}

// UpdateDetectionConfig updates the detection configuration
func (c *Config) UpdateDetectionConfig(newConfig detector.Config) error {
	c.Detection = newConfig
	return c.Validate()
}

// GetOptimalLSHParameters calculates optimal LSH parameters for current settings
func (c *Config) GetOptimalLSHParameters() (bands int, hashes int) {
	// For spam detection, we want to optimize for the current threshold
	// This is a simplified calculation - in practice you might want more sophisticated optimization
	
	hashes = c.Detection.MinHashHashes
	if hashes == 0 {
		hashes = 128 // Default
	}
	
	bands = c.Detection.MinHashBands
	if bands == 0 {
		// Calculate optimal bands based on threshold
		threshold := c.Detection.MinHashThreshold
		if threshold >= 0.8 {
			bands = 32 // High precision
		} else if threshold >= 0.6 {
			bands = 16 // Medium precision
		} else {
			bands = 8 // Lower precision, higher recall
		}
		
		// Ensure hashes is divisible by bands
		for hashes%bands != 0 {
			bands--
		}
	}
	
	return bands, hashes
}

// CreateDirectories creates necessary directories based on configuration
func (c *Config) CreateDirectories() error {
	dirs := []string{c.App.DataDir, c.App.TempDir}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	return nil
}

// GetLogFilePath returns the full path to the log file
func (c *Config) GetLogFilePath() string {
	if c.App.DataDir != "" {
		return fmt.Sprintf("%s/%s", c.App.DataDir, c.Logging.Filename)
	}
	return c.Logging.Filename
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// String returns a string representation of the config (excluding sensitive data)
func (c *Config) String() string {
	return fmt.Sprintf("X Spam Detector Config (Version: %s, Environment: %s, MinHash Threshold: %.2f, Hybrid Mode: %t)",
		c.App.Version, c.App.Environment, c.Detection.MinHashThreshold, c.Detection.EnableHybridMode)
}
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"x-spam-detector/internal/api"
	"x-spam-detector/internal/autonomous"
	"x-spam-detector/internal/config"
	"x-spam-detector/internal/detector"
)

var (
	configFile = flag.String("config", "config-autonomous.yaml", "Configuration file path")
	mode       = flag.String("mode", "autonomous", "Operation mode: autonomous, gui, or hybrid")
	logLevel   = flag.String("log-level", "", "Log level override (debug, info, warn, error)")
	apiOnly    = flag.Bool("api-only", false, "Run API server only (no autonomous detection)")
	help       = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override log level if specified
	if *logLevel != "" {
		cfg.System.LogLevel = *logLevel
	}

	// Expand environment variables in configuration
	expandEnvVars(cfg)

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	log.Printf("Starting X Spam Detector in %s mode", *mode)
	log.Printf("Configuration loaded from: %s", *configFile)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	switch *mode {
	case "autonomous":
		runAutonomousMode(cfg, sigChan)
	case "api-only":
		runAPIOnlyMode(cfg, sigChan)
	case "gui":
		runGUIMode(cfg)
	case "hybrid":
		runHybridMode(cfg, sigChan)
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

func runAutonomousMode(cfg *autonomous.AutonomousConfig, sigChan <-chan os.Signal) {
	log.Println("🤖 Starting autonomous spam detection mode...")

	// Create and start autonomous engine
	engine, err := autonomous.NewAutonomousEngine(*cfg)
	if err != nil {
		log.Fatalf("Failed to create autonomous engine: %v", err)
	}
	
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start autonomous engine: %v", err)
	}

	// Start API server if enabled
	var apiServer *api.Server
	if cfg.API.Enabled {
		log.Printf("🌐 Starting API server on %s:%d", cfg.API.Host, cfg.API.Port)
		serverConfig := api.ServerConfig{
			Port:         cfg.API.Port,
			Host:         cfg.API.Host,
			EnableCORS:   cfg.API.EnableCORS,
			ReadTimeout:  cfg.API.ReadTimeout,
			WriteTimeout: cfg.API.WriteTimeout,
			EnableAuth:   cfg.API.EnableAuth,
			APIKey:       cfg.API.APIKey,
		}
		apiServer = api.NewServer(engine, serverConfig)
		go func() {
			if err := apiServer.Start(); err != nil {
				log.Printf("API server error: %v", err)
			}
		}()
		
		log.Printf("📊 Dashboard available at: http://%s:%d", cfg.API.Host, cfg.API.Port)
	}

	// Print status information
	printStartupInfo(cfg)

	// Wait for shutdown signal
	<-sigChan
	log.Println("🛑 Shutdown signal received, stopping autonomous engine...")

	// Graceful shutdown
	if err := engine.Stop(); err != nil {
		log.Printf("Error stopping autonomous engine: %v", err)
	}

	log.Println("✅ Autonomous engine stopped successfully")
}

func runAPIOnlyMode(cfg *autonomous.AutonomousConfig, sigChan <-chan os.Signal) {
	log.Println("🌐 Starting API-only mode...")

	// Create engine but don't start autonomous operations
	engine, err := autonomous.NewAutonomousEngine(*cfg)
	if err != nil {
		log.Fatalf("Failed to create autonomous engine: %v", err)
	}

	// Start API server
	serverConfig := api.ServerConfig{
		Port:         cfg.API.Port,
		Host:         cfg.API.Host,
		EnableCORS:   cfg.API.EnableCORS,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
		EnableAuth:   cfg.API.EnableAuth,
		APIKey:       cfg.API.APIKey,
	}
	apiServer := api.NewServer(engine, serverConfig)
	go func() {
		log.Printf("Starting API server on %s:%d", cfg.API.Host, cfg.API.Port)
		if err := apiServer.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	log.Printf("📊 Dashboard available at: http://%s:%d", cfg.API.Host, cfg.API.Port)

	// Wait for shutdown signal
	<-sigChan
	log.Println("🛑 Shutdown signal received")
}

func runGUIMode(cfg *autonomous.AutonomousConfig) {
	log.Println("🖥️  Starting GUI mode...")
	
	// Start GUI (this would import the original GUI code)
	log.Println("Note: GUI mode would start the original Fyne interface")
	log.Println("For now, use autonomous mode with web dashboard")
	
	// Note: GUI integration would use convertToRegularConfig(cfg) when implemented
}

func runHybridMode(cfg *autonomous.AutonomousConfig, sigChan <-chan os.Signal) {
	log.Println("🔄 Starting hybrid mode (autonomous + GUI)...")
	
	// Start autonomous engine
	engine, err := autonomous.NewAutonomousEngine(*cfg)
	if err != nil {
		log.Fatalf("Failed to create autonomous engine: %v", err)
	}
	
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start autonomous engine: %v", err)
	}

	// Start API server
	if cfg.API.Enabled {
		serverConfig := api.ServerConfig{
			Port:         cfg.API.Port,
			Host:         cfg.API.Host,
			EnableCORS:   cfg.API.EnableCORS,
			ReadTimeout:  cfg.API.ReadTimeout,
			WriteTimeout: cfg.API.WriteTimeout,
			EnableAuth:   cfg.API.EnableAuth,
			APIKey:       cfg.API.APIKey,
		}
		apiServer := api.NewServer(engine, serverConfig)
		go func() {
			log.Printf("Starting API server on %s:%d", cfg.API.Host, cfg.API.Port)
			if err := apiServer.Start(); err != nil {
				log.Printf("API server error: %v", err)
			}
		}()
	}

	// Start GUI (would need integration work)
	log.Println("Note: GUI component would start here in hybrid mode")

	// Wait for shutdown signal
	<-sigChan
	log.Println("🛑 Shutdown signal received, stopping hybrid mode...")

	if err := engine.Stop(); err != nil {
		log.Printf("Error stopping autonomous engine: %v", err)
	}
}

func loadConfig(configFile string) (*autonomous.AutonomousConfig, error) {
	// Load the configuration file
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Convert to autonomous config
	// This is a simplified conversion - in a real implementation,
	// you'd have proper config parsing for the autonomous structure
	autonomousConfig := autonomous.GetDefaultAutonomousConfig()
	
	// Override with loaded values where applicable
	autonomousConfig.System.LogLevel = cfg.Logging.Level
	
	return &autonomousConfig, nil
}

func expandEnvVars(cfg *autonomous.AutonomousConfig) {
	// Expand environment variables in sensitive fields
	cfg.CrawlerConfig.API.BearerToken = os.ExpandEnv(cfg.CrawlerConfig.API.BearerToken)
	cfg.CrawlerConfig.API.APIKey = os.ExpandEnv(cfg.CrawlerConfig.API.APIKey)
	cfg.CrawlerConfig.API.APIKeySecret = os.ExpandEnv(cfg.CrawlerConfig.API.APIKeySecret)
	cfg.CrawlerConfig.API.AccessToken = os.ExpandEnv(cfg.CrawlerConfig.API.AccessToken)
	cfg.CrawlerConfig.API.AccessSecret = os.ExpandEnv(cfg.CrawlerConfig.API.AccessSecret)
}

func validateConfig(cfg *autonomous.AutonomousConfig) error {
	// Validate Twitter API credentials
	if cfg.CrawlerConfig.API.BearerToken == "" && cfg.CrawlerConfig.API.APIKey == "" {
		return fmt.Errorf("Twitter API credentials are required (TWITTER_BEARER_TOKEN or TWITTER_API_KEY)")
	}

	// Validate monitoring configuration
	if len(cfg.CrawlerConfig.Monitoring.Keywords) == 0 && len(cfg.CrawlerConfig.Monitoring.Hashtags) == 0 {
		return fmt.Errorf("at least one keyword or hashtag must be configured for monitoring")
	}

	// Validate thresholds
	if cfg.Detection.Engine.MinHashThreshold < 0 || cfg.Detection.Engine.MinHashThreshold > 1 {
		return fmt.Errorf("minhash_threshold must be between 0 and 1")
	}

	return nil
}

func convertToRegularConfig(cfg *autonomous.AutonomousConfig) *config.Config {
	// Convert autonomous config back to regular config for GUI mode
	regularConfig := config.Default()
	regularConfig.Detection = cfg.Detection.Engine
	return regularConfig
}

func printStartupInfo(cfg *autonomous.AutonomousConfig) {
	log.Println("\n" + strings.Repeat("=", 60))
	log.Println("🔍 X SPAM DETECTOR - AUTONOMOUS MODE ACTIVE")
	log.Println(strings.Repeat("=", 60))
	
	log.Printf("📡 Monitoring Configuration:")
	log.Printf("   • Keywords: %d configured", len(cfg.CrawlerConfig.Monitoring.Keywords))
	log.Printf("   • Hashtags: %d configured", len(cfg.CrawlerConfig.Monitoring.Hashtags))
	log.Printf("   • Languages: %v", cfg.CrawlerConfig.Settings.Languages)
	log.Printf("   • Streaming: %t", cfg.CrawlerConfig.Settings.EnableStreaming)
	log.Printf("   • Poll interval: %v", cfg.CrawlerConfig.Settings.PollInterval)
	
	log.Printf("\n🎯 Detection Configuration:")
	log.Printf("   • Algorithm: %s", getAlgorithmName(cfg.Detection.Engine))
	log.Printf("   • MinHash threshold: %.2f", cfg.Detection.Engine.MinHashThreshold)
	log.Printf("   • Min cluster size: %d", cfg.Detection.Engine.MinClusterSize)
	log.Printf("   • Detection interval: %v", cfg.Detection.Interval)
	log.Printf("   • Hybrid mode: %t", cfg.Detection.Engine.EnableHybridMode)
	
	log.Printf("\n🚨 Alert Configuration:")
	log.Printf("   • Alerts enabled: %t", cfg.Alerts.Enabled)
	log.Printf("   • Slack URL: %s", cfg.Alerts.SlackURL)
	log.Printf("   • Email from: %s", cfg.Alerts.EmailFrom)
	
	if cfg.API.Enabled {
		log.Printf("\n🌐 API Server:")
		log.Printf("   • URL: http://%s:%d", cfg.API.Host, cfg.API.Port)
		log.Printf("   • Dashboard: http://%s:%d/dashboard", cfg.API.Host, cfg.API.Port)
		log.Printf("   • API docs: http://%s:%d/api/v1/", cfg.API.Host, cfg.API.Port)
	}
	
	log.Println("\n🚀 System is now autonomously monitoring for spam patterns...")
	log.Println("   Press Ctrl+C to stop")
	log.Println(strings.Repeat("=", 60) + "\n")
}

func getAlgorithmName(cfg detector.Config) string {
	if cfg.EnableHybridMode {
		return "Hybrid (MinHash + SimHash)"
	}
	return "MinHash LSH"
}

func showHelp() {
	fmt.Println(`X Spam Detector - Autonomous Twitter Spam Detection System

USAGE:
    spam-detector [OPTIONS]

OPTIONS:
    -config string
        Configuration file path (default: config-autonomous.yaml)
    
    -mode string
        Operation mode (default: autonomous)
        • autonomous: Full autonomous detection with API
        • api-only:   API server only (no detection)
        • gui:        Original GUI interface
        • hybrid:     Autonomous + GUI combined
    
    -log-level string
        Override log level (debug, info, warn, error)
    
    -api-only
        Run API server only (shortcut for -mode=api-only)
    
    -help
        Show this help message

MODES:

    Autonomous Mode (Recommended):
        • Continuously monitors Twitter using API
        • Automatically detects spam patterns
        • Sends real-time alerts
        • Provides web dashboard at http://localhost:8080

    API-Only Mode:
        • Runs web dashboard and API endpoints
        • No autonomous detection
        • Useful for manual analysis

    GUI Mode:
        • Original desktop interface
        • Manual tweet input and analysis

    Hybrid Mode:
        • Combines autonomous detection with GUI
        • Best of both worlds

CONFIGURATION:

    The system uses YAML configuration files. Copy config-autonomous.yaml
    and customize for your needs.

    Required environment variables:
        TWITTER_BEARER_TOKEN    - Twitter API Bearer Token
        TWITTER_API_KEY         - Twitter API Key
        TWITTER_API_SECRET      - Twitter API Secret

    Optional environment variables:
        TWITTER_ACCESS_TOKEN    - Twitter Access Token
        TWITTER_ACCESS_SECRET   - Twitter Access Secret
        SLACK_WEBHOOK_URL       - Slack webhook for alerts
        DISCORD_WEBHOOK_URL     - Discord webhook for alerts
        EMAIL_USERNAME          - Email username for alerts
        EMAIL_PASSWORD          - Email password for alerts

EXAMPLES:

    # Start autonomous mode with default config
    ./spam-detector

    # Start with custom config
    ./spam-detector -config my-config.yaml

    # Start API-only mode for manual analysis
    ./spam-detector -mode api-only

    # Start with debug logging
    ./spam-detector -log-level debug

    # Run GUI mode for desktop interface
    ./spam-detector -mode gui

DASHBOARD:

    When running in autonomous or hybrid mode, access the web dashboard at:
    http://localhost:8080

    API endpoints available at:
    http://localhost:8080/api/v1/

    Real-time monitoring, cluster analysis, and system control available
    through the web interface.

GETTING TWITTER API ACCESS:

    1. Create a Twitter Developer account at https://developer.twitter.com
    2. Create a new App in the Developer Portal
    3. Generate API keys and tokens
    4. Set environment variables or update config file

    Required permissions: Read-only access to tweets

MORE INFO:

    Documentation: https://github.com/your-org/x-spam-detector
    Issues: https://github.com/your-org/x-spam-detector/issues
`)
}
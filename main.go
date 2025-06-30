package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"x-spam-detector/internal/app"
	"x-spam-detector/internal/config"
	"x-spam-detector/internal/gui"
)

var (
	mode       = flag.String("mode", "gui", "Operation mode: gui, autonomous")
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	help       = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	switch *mode {
	case "gui":
		runGUIMode()
	case "autonomous":
		runAutonomousMode()
	default:
		log.Fatalf("Unknown mode: %s. Use 'gui' or 'autonomous'", *mode)
	}
}

func runGUIMode() {
	log.Println("Starting GUI mode...")

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Printf("Warning: Could not load config file, using defaults: %v", err)
		cfg = config.Default()
	}

	// Initialize the spam detection application
	spamDetector := app.New(cfg)

	// Start the GUI
	gui.Run(spamDetector)
}

func runAutonomousMode() {
	fmt.Println("For autonomous mode, please use the dedicated autonomous binary:")
	fmt.Println("")
	fmt.Println("  go run cmd/autonomous/main.go")
	fmt.Println("")
	fmt.Println("Or build and run:")
	fmt.Println("  go build -o spam-detector-autonomous cmd/autonomous/main.go")
	fmt.Println("  ./spam-detector-autonomous")
	fmt.Println("The autonomous mode provides:")
	fmt.Println("  • Real-time Twitter API monitoring")
	fmt.Println("  • Automatic spam detection")
	fmt.Println("  • Web dashboard at http://localhost:8080")
	fmt.Println("  • Alert notifications")
	fmt.Println("")
	os.Exit(1)
}

func showHelp() {
	fmt.Println(`X Spam Detector - Twitter Spam Detection System

USAGE:
    spam-detector [OPTIONS]

OPTIONS:
    -mode string
        Operation mode (default: gui)
        • gui:        Desktop GUI interface for manual analysis
        • autonomous: Redirect to autonomous mode instructions
    
    -config string
        Configuration file path (default: config.yaml)
    
    -help
        Show this help message

MODES:

    GUI Mode (Default):
        • Desktop interface using Fyne framework
        • Manual tweet input and analysis
        • Interactive spam cluster visualization
        • Real-time detection results
        • Sample data for testing

    Autonomous Mode:
        • Use the dedicated autonomous binary
        • Real-time Twitter API monitoring
        • Automatic spam detection
        • Web dashboard and API
        • Alert notifications

EXAMPLES:

    # Start GUI mode (default)
    ./spam-detector

    # Start GUI with custom config
    ./spam-detector -config my-config.yaml

    # Get autonomous mode instructions
    ./spam-detector -mode autonomous

    # Show help
    ./spam-detector -help

GUI MODE FEATURES:

    • Add individual tweets or batch import
    • Real-time spam detection using LSH algorithms
    • Interactive cluster visualization
    • Suspicious account analysis
    • Performance statistics
    • Sample data for testing

    The GUI provides tabs for:
    - Input: Add tweets and sample data
    - Tweets: View all tweets with spam status
    - Clusters: Analyze detected spam clusters
    - Accounts: Review suspicious accounts
    - Statistics: System performance metrics

AUTONOMOUS MODE:

    For autonomous operation with Twitter API monitoring,
    use the dedicated autonomous binary:

        go run cmd/autonomous/main.go

    This provides:
    • Real-time Twitter API streaming
    • Automatic keyword/hashtag monitoring  
    • Continuous spam detection
    • Web dashboard at http://localhost:8080
    • Alert notifications (Slack, email, webhooks)
    • REST API for integration

CONFIGURATION:

    GUI mode uses config.yaml for basic settings.
    Autonomous mode uses config-autonomous.yaml for full configuration.

MORE INFO:

    Documentation: README.md
    Examples: See config.yaml and config-autonomous.yaml`)
}
package autonomous

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"x-spam-detector/internal/alerts"
	"x-spam-detector/internal/crawler"
	"x-spam-detector/internal/detector"
	"x-spam-detector/internal/models"
)

// AutonomousEngine coordinates all autonomous operations
type AutonomousEngine struct {
	// Core components
	crawler      *crawler.AutonomousCrawler
	detector     *detector.SpamDetectionEngine
	alertSystem  *alerts.AlertingSystem
	
	// Configuration
	config       AutonomousConfig
	
	// State management
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mutex        sync.RWMutex
	
	// Statistics
	stats        EngineStats
}

// AutonomousConfig defines configuration for autonomous operation
type AutonomousConfig struct {
	CrawlerConfig    crawler.CrawlerConfig `yaml:"crawler"`
	DetectorConfig   detector.Config      `yaml:"detector"`
	
	// Processing settings
	ProcessingDelay  time.Duration        `yaml:"processing_delay"`
	MaxConcurrent    int                  `yaml:"max_concurrent"`
	BufferSize       int                  `yaml:"buffer_size"`
	
	// Health monitoring
	HealthCheckInterval time.Duration     `yaml:"health_check_interval"`
	MaxErrors          int               `yaml:"max_errors"`
	RestartDelay       time.Duration     `yaml:"restart_delay"`
	
	// System configuration
	System           SystemConfig          `yaml:"system"`
	
	// API configuration
	API              APIConfig             `yaml:"api"`
	
	// Detection configuration
	Detection        AutoDetectionConfig   `yaml:"auto_detection"`
	
	// Alert configuration (placeholder)
	Alerts           AlertConfig           `yaml:"alerts"`
}

// SystemConfig holds system-level configuration
type SystemConfig struct {
	MaxMemoryMB          int           `yaml:"max_memory_mb"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval"`
	MetricsInterval      time.Duration `yaml:"metrics_interval"`
	LogLevel             string        `yaml:"log_level"`
	EnableProfiling      bool          `yaml:"enable_profiling"`
	ProfilingPort        int           `yaml:"profiling_port"`
}

// APIConfig holds API server configuration
type APIConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Port         int           `yaml:"port"`
	Host         string        `yaml:"host"`
	EnableCORS   bool          `yaml:"enable_cors"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	EnableAuth   bool          `yaml:"enable_auth"`
	APIKey       string        `yaml:"api_key"`
}

// AutoDetectionConfig holds automatic detection settings
type AutoDetectionConfig struct {
	Enabled                bool          `yaml:"enabled"`
	Interval               time.Duration `yaml:"interval"`
	MinTweetsForDetection  int           `yaml:"min_tweets_for_detection"`
	AutoClearOldData       bool          `yaml:"auto_clear_old_data"`
	RetentionHours         int           `yaml:"retention_hours"`
	BatchProcessing        bool          `yaml:"batch_processing"`
	BatchSize              int           `yaml:"batch_size"`
	ContinuousMode         bool          `yaml:"continuous_mode"`
	
	// Detection engine config
	Engine       detector.Config       `yaml:"engine"`
}

// AlertConfig holds basic alert configuration (simplified)
type AlertConfig struct {
	Enabled    bool   `yaml:"enabled"`
	SlackURL   string `yaml:"slack_url"`
	EmailFrom  string `yaml:"email_from"`
	EmailTo    string `yaml:"email_to"`
}

// EngineStats tracks engine performance and health
type EngineStats struct {
	StartTime        time.Time     `json:"start_time"`
	TotalTweets      int64         `json:"total_tweets"`
	SpamDetected     int64         `json:"spam_detected"`
	ClustersFound    int64         `json:"clusters_found"`
	AlertsSent       int64         `json:"alerts_sent"`
	ErrorCount       int64         `json:"error_count"`
	LastErrorTime    time.Time     `json:"last_error_time"`
	ProcessingRate   float64       `json:"processing_rate"`
	UptimeSeconds    float64       `json:"uptime_seconds"`
}

// NewAutonomousEngine creates a new autonomous engine
func NewAutonomousEngine(config AutonomousConfig) (*AutonomousEngine, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	engine := &AutonomousEngine{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		stats: EngineStats{
			StartTime: time.Now(),
		},
	}
	
	// Initialize components
	// Initialize crawler
	engine.crawler = crawler.NewAutonomousCrawler(config.CrawlerConfig)
	
	// Initialize detector
	detectorEngine, err := detector.NewSpamDetectionEngine(config.DetectorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create spam detection engine: %w", err)
	}
	engine.detector = detectorEngine
	
	// Initialize alert system
	engine.alertSystem = alerts.NewAlertingSystem()
	
	return engine, nil
}

// Start starts the autonomous engine
func (ae *AutonomousEngine) Start() error {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	if ae.running {
		return fmt.Errorf("engine is already running")
	}
	
	log.Println("Starting autonomous engine...")
	
	// Start components
	if err := ae.crawler.Start(); err != nil {
		return fmt.Errorf("failed to start crawler: %w", err)
	}
	
	// Start processing goroutines
	ae.wg.Add(3)
	go ae.processingLoop()
	go ae.healthMonitoring()
	go ae.statsCollection()
	
	ae.running = true
	log.Println("Autonomous engine started successfully")
	
	return nil
}

// Stop stops the autonomous engine
func (ae *AutonomousEngine) Stop() error {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	if !ae.running {
		return nil
	}
	
	log.Println("Stopping autonomous engine...")
	
	// Cancel context to signal all goroutines to stop
	ae.cancel()
	
	// Stop components
	if err := ae.crawler.Stop(); err != nil {
		log.Printf("Error stopping crawler: %v", err)
	}
	
	if err := ae.alertSystem.Shutdown(); err != nil {
		log.Printf("Error stopping alert system: %v", err)
	}
	
	// Wait for all goroutines to finish
	ae.wg.Wait()
	
	ae.running = false
	log.Println("Autonomous engine stopped")
	
	return nil
}

// processingLoop handles tweet processing
func (ae *AutonomousEngine) processingLoop() {
	defer ae.wg.Done()
	
	log.Println("Starting processing loop...")
	
	// Get tweet channel from crawler
	tweetChan := ae.crawler.GetTweetChannel()
	errorChan := ae.crawler.GetErrorChannel()
	
	// Batch processing
	tweets := make([]*models.Tweet, 0, ae.config.BufferSize)
	ticker := time.NewTicker(ae.config.ProcessingDelay)
	defer ticker.Stop()
	
	for {
		select {
		case <-ae.ctx.Done():
			log.Println("Processing loop stopping...")
			// Process any remaining tweets
			if len(tweets) > 0 {
				ae.processTweets(tweets)
			}
			return
			
		case tweet := <-tweetChan:
			if tweet != nil {
				tweets = append(tweets, tweet)
				// Process batch when full
				if len(tweets) >= ae.config.BufferSize {
					ae.processTweets(tweets)
					tweets = tweets[:0] // Reset slice
				}
			}
			
		case err := <-errorChan:
			if err != nil {
				log.Printf("Crawler error: %v", err)
				ae.recordError()
			}
			
		case <-ticker.C:
			// Process accumulated tweets periodically
			if len(tweets) > 0 {
				ae.processTweets(tweets)
				tweets = tweets[:0] // Reset slice
			}
		}
	}
}

// processTweets processes a batch of tweets
func (ae *AutonomousEngine) processTweets(tweets []*models.Tweet) {
	start := time.Now()
	
	// Add tweets to detector
	if err := ae.detector.AddTweets(tweets); err != nil {
		log.Printf("Error adding tweets to detector: %v", err)
		ae.recordError()
		return
	}
	
	// Run detection
	result, err := ae.detector.DetectSpam()
	if err != nil {
		log.Printf("Error detecting spam: %v", err)
		ae.recordError()
		return
	}
	
	// Update statistics
	ae.updateStats(tweets, result, time.Since(start))
	
	// Send alerts if needed
	ae.handleDetectionResult(result)
	
	log.Printf("Processed %d tweets, found %d spam tweets in %d clusters",
		len(tweets), result.SpamTweets, len(result.SpamClusters))
}

// handleDetectionResult handles detection results and sends alerts
func (ae *AutonomousEngine) handleDetectionResult(result *models.DetectionResult) {
	// Check for high spam rate
	if result.SpamRate > 0.5 {
		log.Printf("High spam rate detected: %.2f%%", result.SpamRate*100)
		
		alert := &alerts.Alert{
			Title:   "High Spam Rate Detected",
			Message: fmt.Sprintf("Spam rate reached %.2f%% (%d spam tweets out of %d total)", result.SpamRate*100, result.SpamTweets, result.TotalTweets),
		}
		
		// Just log for now since sendAlert is private
		log.Printf("ALERT: %s - %s", alert.Title, alert.Message)
	}
	
	// Check for large clusters
	for _, cluster := range result.SpamClusters {
		if cluster.Size > 10 && cluster.Confidence > 0.8 {
			log.Printf("Large spam cluster detected: %d tweets, confidence %.2f",
				cluster.Size, cluster.Confidence)
			
			alert := &alerts.Alert{
				Title:   "Large Spam Cluster Detected",
				Message: fmt.Sprintf("Detected spam cluster with %d tweets (confidence: %.2f)", cluster.Size, cluster.Confidence),
			}
			
			// Just log for now since sendAlert is private
			log.Printf("ALERT: %s - %s", alert.Title, alert.Message)
		}
	}
}

// healthMonitoring monitors engine health
func (ae *AutonomousEngine) healthMonitoring() {
	defer ae.wg.Done()
	
	ticker := time.NewTicker(ae.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ae.ctx.Done():
			return
			
		case <-ticker.C:
			ae.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check
func (ae *AutonomousEngine) performHealthCheck() {
	// TODO: Implement comprehensive health checks
	log.Println("Performing health check...")
	
	// Check if components are responsive
	// TODO: Implement crawler health check
	log.Println("Health check completed")
	
	// Check error rate
	if ae.stats.ErrorCount > int64(ae.config.MaxErrors) {
		log.Printf("High error count: %d", ae.stats.ErrorCount)
		// TODO: Implement auto-recovery
	}
}

// statsCollection collects and updates statistics
func (ae *AutonomousEngine) statsCollection() {
	defer ae.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ae.ctx.Done():
			return
			
		case <-ticker.C:
			ae.updateUptimeStats()
		}
	}
}

// updateStats updates engine statistics
func (ae *AutonomousEngine) updateStats(tweets []*models.Tweet, result *models.DetectionResult, processingTime time.Duration) {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	ae.stats.TotalTweets += int64(len(tweets))
	ae.stats.SpamDetected += int64(result.SpamTweets)
	ae.stats.ClustersFound += int64(len(result.SpamClusters))
	
	// Calculate processing rate (tweets per second)
	if processingTime > 0 {
		rate := float64(len(tweets)) / processingTime.Seconds()
		ae.stats.ProcessingRate = (ae.stats.ProcessingRate + rate) / 2 // Simple moving average
	}
}

// updateUptimeStats updates uptime statistics
func (ae *AutonomousEngine) updateUptimeStats() {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	ae.stats.UptimeSeconds = time.Since(ae.stats.StartTime).Seconds()
}

// recordError records an error event
func (ae *AutonomousEngine) recordError() {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	ae.stats.ErrorCount++
	ae.stats.LastErrorTime = time.Now()
}

// GetStats returns current engine statistics
func (ae *AutonomousEngine) GetStats() EngineStats {
	ae.mutex.RLock()
	defer ae.mutex.RUnlock()
	
	// Update uptime before returning
	stats := ae.stats
	stats.UptimeSeconds = time.Since(ae.stats.StartTime).Seconds()
	
	return stats
}

// IsRunning returns true if the engine is running
func (ae *AutonomousEngine) IsRunning() bool {
	ae.mutex.RLock()
	defer ae.mutex.RUnlock()
	return ae.running
}

// GetConfig returns the current configuration
func (ae *AutonomousEngine) GetConfig() AutonomousConfig {
	ae.mutex.RLock()
	defer ae.mutex.RUnlock()
	return ae.config
}

// Default configuration
func DefaultAutonomousConfig() AutonomousConfig {
	return AutonomousConfig{
		CrawlerConfig:       crawler.GetDefaultCrawlerConfig(),
		DetectorConfig:      detector.Config{}, // Use defaults
		ProcessingDelay:     time.Second,
		MaxConcurrent:       10,
		BufferSize:          1000,
		HealthCheckInterval: 30 * time.Second,
		MaxErrors:          100,
		RestartDelay:       5 * time.Second,
		System: SystemConfig{
			MaxMemoryMB:         1000,
			HealthCheckInterval: 5 * time.Minute,
			MetricsInterval:     30 * time.Second,
			LogLevel:            "info",
			EnableProfiling:     false,
			ProfilingPort:       6060,
		},
		API: APIConfig{
			Enabled:      true,
			Port:         8080,
			Host:         "localhost",
			EnableCORS:   true,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			EnableAuth:   false,
			APIKey:       "",
		},
		Detection: AutoDetectionConfig{
			Enabled:               true,
			Interval:              5 * time.Minute,
			MinTweetsForDetection: 50,
			AutoClearOldData:      true,
			RetentionHours:        72,
			BatchProcessing:       true,
			BatchSize:             100,
			ContinuousMode:        false,
			Engine:                detector.Config{},
		},
		Alerts: AlertConfig{
			Enabled:   false,
			SlackURL:  "",
			EmailFrom: "",
			EmailTo:   "",
		},
	}
}

// GetDefaultAutonomousConfig returns a default autonomous configuration (alias)
func GetDefaultAutonomousConfig() AutonomousConfig {
	return DefaultAutonomousConfig()
}
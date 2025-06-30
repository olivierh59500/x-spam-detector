package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// CrawlerConfig holds configuration for the autonomous crawler
type CrawlerConfig struct {
	// API Configuration
	API TwitterAPIConfig `yaml:"api"`
	
	// Monitoring targets
	Monitoring MonitoringConfig `yaml:"monitoring"`
	
	// Crawler settings
	Settings CrawlerSettings `yaml:"settings"`
}

// MonitoringConfig defines what to monitor
type MonitoringConfig struct {
	Keywords        []string    `yaml:"keywords"`
	Hashtags        []string    `yaml:"hashtags"`
	MonitoredUsers  []string    `yaml:"monitored_users"`
	GeoLocations    []GeoBox    `yaml:"geo_locations"`
}

// CrawlerSettings defines how to crawl
type CrawlerSettings struct {
	PollInterval     time.Duration `yaml:"poll_interval"`
	EnableStreaming  bool          `yaml:"enable_streaming"`
	BatchSize        int           `yaml:"batch_size"`
	Languages        []string      `yaml:"languages"`
	MinRetweets      int           `yaml:"min_retweets"`
	MaxAge           time.Duration `yaml:"max_age"`
	MaxTweetsPerHour int           `yaml:"max_tweets_per_hour"`
}

// GeoBox defines a geographical bounding box
type GeoBox struct {
	SouthWest GeoCoordinates `yaml:"southwest"`
	NorthEast GeoCoordinates `yaml:"northeast"`
	Name      string         `yaml:"name"`
}

// GeoCoordinates represents latitude and longitude
type GeoCoordinates struct {
	Lat float64 `yaml:"lat"`
	Lng float64 `yaml:"lng"`
}

// AutonomousCrawler manages autonomous data collection from Twitter
type AutonomousCrawler struct {
	client      *TwitterClient
	config      CrawlerConfig
	tweetChan   chan *models.Tweet
	errorChan   chan error
	
	// State management
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	
	// Statistics
	stats       CrawlerStats
	
	// Rate limiting for hourly tweets
	hourlyCounter   int
	hourlyResetTime time.Time
}

// CrawlerStats holds runtime statistics
type CrawlerStats struct {
	TotalTweets       int64     `json:"total_tweets"`
	TweetsThisHour    int64     `json:"tweets_this_hour"`
	ActiveStreams     int       `json:"active_streams"`
	LastTweetTime     time.Time `json:"last_tweet_time"`
	ErrorCount        int64     `json:"error_count"`
	StartTime         time.Time `json:"start_time"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	TweetsPerSecond   float64   `json:"tweets_per_second"`
}

// NewAutonomousCrawler creates a new autonomous crawler
func NewAutonomousCrawler(config CrawlerConfig) *AutonomousCrawler {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AutonomousCrawler{
		client:      NewTwitterClient(config.API),
		config:      config,
		tweetChan:   make(chan *models.Tweet, 1000), // Buffered channel
		errorChan:   make(chan error, 100),
		ctx:         ctx,
		cancel:      cancel,
		stats: CrawlerStats{
			StartTime: time.Now(),
		},
		hourlyResetTime: time.Now().Add(time.Hour),
	}
}

// Start begins autonomous crawling
func (ac *AutonomousCrawler) Start() error {
	ac.mutex.Lock()
	if ac.running {
		ac.mutex.Unlock()
		return fmt.Errorf("crawler is already running")
	}
	ac.running = true
	ac.mutex.Unlock()

	log.Println("Starting autonomous crawler...")

	// Start error handler
	ac.wg.Add(1)
	go ac.handleErrors()

	// Start statistics updater
	ac.wg.Add(1)
	go ac.updateStats()

	// Start streaming if enabled
	if ac.config.Settings.EnableStreaming {
		log.Println("Starting real-time streaming...")
		ac.wg.Add(1)
		go ac.startStreaming()
	}

	// Start periodic search crawling
	log.Println("Starting periodic search crawling...")
	ac.wg.Add(1)
	go ac.startPeriodicCrawling()

	log.Println("Autonomous crawler started successfully")
	return nil
}

// Stop stops the crawler
func (ac *AutonomousCrawler) Stop() error {
	ac.mutex.Lock()
	if !ac.running {
		ac.mutex.Unlock()
		return fmt.Errorf("crawler is not running")
	}
	ac.running = false
	ac.mutex.Unlock()

	log.Println("Stopping autonomous crawler...")
	
	// Cancel all operations
	ac.cancel()
	
	// Wait for all goroutines to finish
	ac.wg.Wait()
	
	// Close channels
	close(ac.tweetChan)
	close(ac.errorChan)
	
	log.Println("Autonomous crawler stopped")
	return nil
}

// GetTweetChannel returns the channel for receiving tweets
func (ac *AutonomousCrawler) GetTweetChannel() <-chan *models.Tweet {
	return ac.tweetChan
}

// GetErrorChannel returns the channel for receiving errors
func (ac *AutonomousCrawler) GetErrorChannel() <-chan error {
	return ac.errorChan
}

// GetStats returns current crawler statistics
func (ac *AutonomousCrawler) GetStats() CrawlerStats {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	
	stats := ac.stats
	stats.UptimeSeconds = int64(time.Since(stats.StartTime).Seconds())
	
	if stats.UptimeSeconds > 0 {
		stats.TweetsPerSecond = float64(stats.TotalTweets) / float64(stats.UptimeSeconds)
	}
	
	return stats
}

// startStreaming starts real-time tweet streaming
func (ac *AutonomousCrawler) startStreaming() {
	defer ac.wg.Done()
	
	// Build streaming rules
	rules := ac.buildStreamingRules()
	if len(rules) == 0 {
		log.Println("No streaming rules configured, skipping streaming")
		return
	}
	
	log.Printf("Setting up %d streaming rules", len(rules))
	
	// Start streaming with retry logic
	for ac.isRunning() {
		select {
		case <-ac.ctx.Done():
			return
		default:
		}
		
		streamCtx, streamCancel := context.WithCancel(ac.ctx)
		
		ac.mutex.Lock()
		ac.stats.ActiveStreams++
		ac.mutex.Unlock()
		
		err := ac.client.StartStream(streamCtx, rules, ac.tweetChan)
		
		ac.mutex.Lock()
		ac.stats.ActiveStreams--
		ac.mutex.Unlock()
		
		streamCancel()
		
		if err != nil && ac.isRunning() {
			ac.errorChan <- fmt.Errorf("streaming error: %w", err)
			
			// Wait before retrying
			select {
			case <-time.After(30 * time.Second):
			case <-ac.ctx.Done():
				return
			}
		}
	}
}

// startPeriodicCrawling starts periodic search-based crawling
func (ac *AutonomousCrawler) startPeriodicCrawling() {
	defer ac.wg.Done()
	
	ticker := time.NewTicker(ac.config.Settings.PollInterval)
	defer ticker.Stop()
	
	// Do initial crawl
	ac.performSearchCrawl()
	
	for {
		select {
		case <-ticker.C:
			if ac.isRunning() {
				ac.performSearchCrawl()
			}
		case <-ac.ctx.Done():
			return
		}
	}
}

// performSearchCrawl performs a single round of search-based crawling
func (ac *AutonomousCrawler) performSearchCrawl() {
	if !ac.checkHourlyLimit() {
		return
	}
	
	// Search by keywords and hashtags
	query := BuildSearchQuery(
		ac.config.Monitoring.Keywords,
		ac.config.Monitoring.Hashtags,
		ac.config.Settings.Languages,
	)
	
	if query == "" {
		return
	}
	
	log.Printf("Performing search crawl with query: %s", query)
	
	tweets, err := ac.client.SearchTweets(query, ac.config.Settings.BatchSize)
	if err != nil {
		ac.errorChan <- fmt.Errorf("search crawl error: %w", err)
		return
	}
	
	// Filter and send tweets
	filtered := ac.filterTweets(tweets)
	for _, tweet := range filtered {
		if !ac.checkHourlyLimit() {
			break
		}
		
		select {
		case ac.tweetChan <- tweet:
			ac.incrementTweetCount()
		case <-ac.ctx.Done():
			return
		default:
			// Channel full, skip this tweet
		}
	}
	
	log.Printf("Search crawl completed: %d tweets found, %d after filtering", len(tweets), len(filtered))
}

// buildStreamingRules creates streaming rules from configuration
func (ac *AutonomousCrawler) buildStreamingRules() []StreamRule {
	var rules []StreamRule
	
	// Rules for keywords
	for _, keyword := range ac.config.Monitoring.Keywords {
		rules = append(rules, StreamRule{
			Value: fmt.Sprintf("\"%s\"", keyword),
			Tag:   "keyword_" + keyword,
		})
	}
	
	// Rules for hashtags
	for _, hashtag := range ac.config.Monitoring.Hashtags {
		rules = append(rules, StreamRule{
			Value: "#" + hashtag,
			Tag:   "hashtag_" + hashtag,
		})
	}
	
	// Rules for monitored users
	for _, username := range ac.config.Monitoring.MonitoredUsers {
		rules = append(rules, StreamRule{
			Value: "from:" + username,
			Tag:   "user_" + username,
		})
	}
	
	// Add language filters if specified
	if len(ac.config.Settings.Languages) > 0 {
		for i, rule := range rules {
			langFilter := ""
			for j, lang := range ac.config.Settings.Languages {
				if j > 0 {
					langFilter += " OR "
				}
				langFilter += "lang:" + lang
			}
			rules[i].Value = fmt.Sprintf("(%s) (%s)", rule.Value, langFilter)
		}
	}
	
	return rules
}

// filterTweets applies filters to tweets
func (ac *AutonomousCrawler) filterTweets(tweets []*models.Tweet) []*models.Tweet {
	var filtered []*models.Tweet
	
	for _, tweet := range tweets {
		if ac.shouldIncludeTweet(tweet) {
			filtered = append(filtered, tweet)
		}
	}
	
	return filtered
}

// shouldIncludeTweet determines if a tweet should be included
func (ac *AutonomousCrawler) shouldIncludeTweet(tweet *models.Tweet) bool {
	// Check minimum retweets
	if tweet.RetweetCount < ac.config.Settings.MinRetweets {
		return false
	}
	
	// Check age
	if ac.config.Settings.MaxAge > 0 {
		if time.Since(tweet.CreatedAt) > ac.config.Settings.MaxAge {
			return false
		}
	}
	
	// Check for spam-like patterns (basic filtering)
	if ac.isObviousSpam(tweet) {
		return true // Include obvious spam for analysis
	}
	
	// Include tweets that match our monitoring criteria
	return ac.matchesMonitoringCriteria(tweet)
}

// isObviousSpam performs basic spam detection
func (ac *AutonomousCrawler) isObviousSpam(tweet *models.Tweet) bool {
	text := tweet.Text
	
	// Check for common spam patterns
	spamIndicators := []string{
		"follow for follow",
		"f4f",
		"followback",
		"crypto deal",
		"make money",
		"click here",
		"limited time",
		"urgent",
		"verify account",
		"account suspension",
	}
	
	for _, indicator := range spamIndicators {
		if containsIgnoreCase(text, indicator) {
			return true
		}
	}
	
	// Check for excessive hashtags or mentions
	if len(tweet.Hashtags) > 5 || len(tweet.Mentions) > 3 {
		return true
	}
	
	// Check for suspicious account patterns
	if tweet.Author.BotScore > 0.7 {
		return true
	}
	
	return false
}

// matchesMonitoringCriteria checks if tweet matches our monitoring targets
func (ac *AutonomousCrawler) matchesMonitoringCriteria(tweet *models.Tweet) bool {
	text := tweet.Text
	
	// Check keywords
	for _, keyword := range ac.config.Monitoring.Keywords {
		if containsIgnoreCase(text, keyword) {
			return true
		}
	}
	
	// Check hashtags
	for _, hashtag := range ac.config.Monitoring.Hashtags {
		for _, tweetHashtag := range tweet.Hashtags {
			if equalIgnoreCase(hashtag, tweetHashtag) {
				return true
			}
		}
	}
	
	// Check monitored users
	for _, username := range ac.config.Monitoring.MonitoredUsers {
		if equalIgnoreCase(username, tweet.Author.Username) {
			return true
		}
	}
	
	return false
}

// checkHourlyLimit checks if we're within the hourly tweet limit
func (ac *AutonomousCrawler) checkHourlyLimit() bool {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	now := time.Now()
	
	// Reset counter if an hour has passed
	if now.After(ac.hourlyResetTime) {
		ac.hourlyCounter = 0
		ac.hourlyResetTime = now.Add(time.Hour)
	}
	
	// Check if we're at the limit
	if ac.config.Settings.MaxTweetsPerHour > 0 && 
	   ac.hourlyCounter >= ac.config.Settings.MaxTweetsPerHour {
		return false
	}
	
	return true
}

// incrementTweetCount increments the tweet counters
func (ac *AutonomousCrawler) incrementTweetCount() {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	ac.stats.TotalTweets++
	ac.stats.TweetsThisHour++
	ac.stats.LastTweetTime = time.Now()
	ac.hourlyCounter++
}

// updateStats periodically updates statistics
func (ac *AutonomousCrawler) updateStats() {
	defer ac.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Reset hourly counter if needed
			ac.mutex.Lock()
			now := time.Now()
			if now.After(ac.hourlyResetTime) {
				ac.stats.TweetsThisHour = 0
				ac.hourlyResetTime = now.Add(time.Hour)
			}
			ac.mutex.Unlock()
			
		case <-ac.ctx.Done():
			return
		}
	}
}

// handleErrors processes errors from the crawler
func (ac *AutonomousCrawler) handleErrors() {
	defer ac.wg.Done()
	
	for {
		select {
		case err := <-ac.errorChan:
			ac.mutex.Lock()
			ac.stats.ErrorCount++
			ac.mutex.Unlock()
			
			log.Printf("Crawler error: %v", err)
			
			// You could add more sophisticated error handling here:
			// - Send to external monitoring system
			// - Implement exponential backoff
			// - Alert administrators for critical errors
			
		case <-ac.ctx.Done():
			return
		}
	}
}

// isRunning checks if the crawler is currently running
func (ac *AutonomousCrawler) isRunning() bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.running
}

// Helper functions

// containsIgnoreCase checks if text contains substring (case insensitive)
func containsIgnoreCase(text, substring string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substring))
}

// equalIgnoreCase checks if two strings are equal (case insensitive)
func equalIgnoreCase(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}

// GetDefaultCrawlerConfig returns a default crawler configuration
func GetDefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		API: TwitterAPIConfig{
			BaseURL:   "https://api.twitter.com",
			StreamURL: "https://api.twitter.com",
			RateLimit: 300,
		},
		Monitoring: MonitoringConfig{
			Keywords: []string{
				"crypto deal",
				"follow for follow",
				"make money",
				"urgent verification",
				"account suspension",
			},
			Hashtags: []string{
				"crypto",
				"bitcoin",
				"followback",
				"f4f",
				"makemoney",
			},
		},
		Settings: CrawlerSettings{
			PollInterval:     30 * time.Second,
			EnableStreaming:  true,
			BatchSize:        100,
			Languages:        []string{"en", "fr"},
			MinRetweets:      0,
			MaxAge:           24 * time.Hour,
			MaxTweetsPerHour: 10000,
		},
	}
}
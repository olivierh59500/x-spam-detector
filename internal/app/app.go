package app

import (
	"fmt"
	"log"
	"strings"
	"time"

	"x-spam-detector/internal/config"
	"x-spam-detector/internal/detector"
	"x-spam-detector/internal/models"
)

// SpamDetectorApp is the main application struct
type SpamDetectorApp struct {
	config *config.Config
	engine *detector.SpamDetectionEngine
}

// New creates a new SpamDetectorApp instance
func New(cfg *config.Config) *SpamDetectorApp {
	// Create detection engine with config
	engine := detector.NewSpamDetectionEngine(cfg.GetDetectionConfig())
	
	app := &SpamDetectorApp{
		config: cfg,
		engine: engine,
	}
	
	// Create necessary directories
	if err := cfg.CreateDirectories(); err != nil {
		log.Printf("Warning: Failed to create directories: %v", err)
	}
	
	log.Printf("X Spam Detector initialized: %s", cfg.String())
	
	return app
}

// GetEngine returns the spam detection engine
func (app *SpamDetectorApp) GetEngine() *detector.SpamDetectionEngine {
	return app.engine
}

// GetConfig returns the application configuration
func (app *SpamDetectorApp) GetConfig() *config.Config {
	return app.config
}

// AddTweet adds a single tweet to the system
func (app *SpamDetectorApp) AddTweet(text, username string) (*models.Tweet, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("tweet text cannot be empty")
	}
	
	if strings.TrimSpace(username) == "" {
		username = fmt.Sprintf("user_%d", time.Now().Unix())
	}
	
	// Create account
	account := models.NewAccount(username, username)
	account.AccountAge = 30 + (int(time.Now().Unix()) % 365) // Random age between 30-395 days
	account.FollowersCount = 100 + (int(time.Now().Unix()) % 1000)
	account.FollowingCount = 50 + (int(time.Now().Unix()) % 500)
	account.TweetCount = int(time.Now().Unix()) % 1000
	
	// Calculate tweets per day (for bot detection)
	if account.AccountAge > 0 {
		account.TweetsPerDay = float64(account.TweetCount) / float64(account.AccountAge)
	}
	
	// Create tweet
	tweet := models.NewTweet(text, account.ID, *account)
	
	// Add to detection engine
	if err := app.engine.AddTweet(tweet); err != nil {
		return nil, fmt.Errorf("failed to add tweet to detection engine: %w", err)
	}
	
	log.Printf("Added tweet from @%s: %s", username, text[:min(50, len(text))])
	
	return tweet, nil
}

// AddTweetsFromText adds multiple tweets from text (one per line)
func (app *SpamDetectorApp) AddTweetsFromText(text string) error {
	lines := strings.Split(text, "\n")
	addedCount := 0
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Generate username
		username := fmt.Sprintf("user_%d", i+1)
		
		_, err := app.AddTweet(line, username)
		if err != nil {
			return fmt.Errorf("failed to add tweet %d: %w", i+1, err)
		}
		addedCount++
	}
	
	if addedCount == 0 {
		return fmt.Errorf("no valid tweets found in input text")
	}
	
	log.Printf("Added %d tweets from text input", addedCount)
	return nil
}

// DetectSpam runs spam detection on all tweets
func (app *SpamDetectorApp) DetectSpam() (*models.DetectionResult, error) {
	log.Println("Starting spam detection...")
	
	result, err := app.engine.DetectSpam()
	if err != nil {
		return nil, fmt.Errorf("spam detection failed: %w", err)
	}
	
	log.Printf("Spam detection completed: %d spam tweets found in %d clusters (%.1f%% spam rate)",
		result.SpamTweets, len(result.SpamClusters), result.SpamRate*100)
	
	return result, nil
}

// LoadSampleData loads sample spam data for demonstration
func (app *SpamDetectorApp) LoadSampleData() error {
	log.Println("Loading sample spam data...")
	
	// Sample spam campaigns - these represent typical spam patterns
	spamCampaigns := []struct {
		template string
		variants []string
		users    []string
	}{
		{
			template: "Check out this amazing deal on crypto!",
			variants: []string{
				"Check out this amazing deal on crypto! 🚀💰",
				"Check out this incredible deal on crypto! Don't miss out!",
				"Amazing crypto deal alert! Check it out now!",
				"Don't miss this amazing deal on cryptocurrency!",
				"Check out this unbelievable crypto opportunity!",
			},
			users: []string{"cryptobot1", "dealfinder2", "coinmaster3", "trader_pro", "crypto_alert"},
		},
		{
			template: "Follow for follow back, guaranteed!",
			variants: []string{
				"Follow for follow back, guaranteed! #FollowForFollow",
				"F4F - Follow for follow back guaranteed!",
				"Follow me and I'll follow back immediately!",
				"Guaranteed follow back! Just follow me first",
				"Following everyone back who follows me!",
				"F4F guaranteed follow back #follow4follow",
			},
			users: []string{"followback1", "f4f_user", "followme123", "socialmedia_pro", "insta_follow", "followtrain"},
		},
		{
			template: "Make $1000 a day working from home!",
			variants: []string{
				"Make $1000 a day working from home! No experience needed!",
				"Earn $1000 daily from home - easy work!",
				"Work from home and make $1000 per day guaranteed!",
				"$1000/day working from home - click link to start!",
				"Make money from home - $1000 daily possible!",
				"Home-based work: earn $1000 every single day!",
			},
			users: []string{"workfromhome1", "money_maker", "home_income", "easy_money", "daily_cash"},
		},
		{
			template: "URGENT: Your account will be suspended!",
			variants: []string{
				"URGENT: Your account will be suspended! Click here to verify",
				"Account suspension warning - immediate action required!",
				"Your account is at risk of suspension - verify now!",
				"ALERT: Account suspension in 24 hours - click to prevent",
				"Urgent account verification needed to avoid suspension",
			},
			users: []string{"security_alert", "account_support", "verification_team", "urgent_notice", "admin_alert"},
		},
	}
	
	totalTweets := 0
	
	// Add spam campaigns
	for _, campaign := range spamCampaigns {
		for i, variant := range campaign.variants {
			username := campaign.users[i%len(campaign.users)]
			
			_, err := app.AddTweet(variant, username)
			if err != nil {
				return fmt.Errorf("failed to add sample tweet: %w", err)
			}
			totalTweets++
		}
	}
	
	// Add some legitimate tweets to test false positives
	legitimateTweets := []struct {
		text     string
		username string
	}{
		{"Just had an amazing coffee this morning! ☕", "coffee_lover"},
		{"Beautiful sunset today, feeling grateful 🌅", "nature_fan"},
		{"Working on a new project, excited to share updates soon!", "developer_joe"},
		{"Reading a great book about machine learning algorithms", "ai_researcher"},
		{"Had a wonderful dinner with friends last night", "socialite_sam"},
		{"Learning Go programming language, it's quite interesting!", "gopher_dev"},
		{"The weather is perfect for a walk in the park today", "walker_mike"},
		{"Just finished watching an incredible documentary about space", "space_enthusiast"},
	}
	
	for _, tweet := range legitimateTweets {
		_, err := app.AddTweet(tweet.text, tweet.username)
		if err != nil {
			return fmt.Errorf("failed to add legitimate tweet: %w", err)
		}
		totalTweets++
	}
	
	log.Printf("Loaded %d sample tweets (%d spam campaigns + legitimate tweets)", totalTweets, len(spamCampaigns))
	return nil
}

// ClearAll removes all data from the system
func (app *SpamDetectorApp) ClearAll() {
	log.Println("Clearing all data...")
	app.engine.Clear()
	log.Println("All data cleared")
}

// UpdateConfiguration updates the application configuration
func (app *SpamDetectorApp) UpdateConfiguration(newConfig *config.Config) error {
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Update engine configuration
	app.engine.UpdateConfig(newConfig.GetDetectionConfig())
	
	// Update app configuration
	app.config = newConfig
	
	log.Printf("Configuration updated: %s", newConfig.String())
	return nil
}

// GetStatistics returns comprehensive statistics about the system
func (app *SpamDetectorApp) GetStatistics() map[string]interface{} {
	engineStats := app.engine.GetStatistics()
	clusters := app.engine.GetClusters()
	suspiciousAccounts := app.engine.GetSuspiciousAccounts()
	
	stats := map[string]interface{}{
		"total_tweets":         engineStats.TotalTweets,
		"spam_tweets":          engineStats.SpamTweets,
		"active_clusters":      engineStats.ActiveClusters,
		"suspicious_accounts":  engineStats.SuspiciousAccounts,
		"processing_time":      engineStats.LastProcessingTime.String(),
		"total_processing_time": engineStats.TotalProcessingTime.String(),
		"processed_batches":    engineStats.ProcessedBatches,
		"memory_usage_mb":      engineStats.MemoryUsage / 1024 / 1024,
	}
	
	// Add spam rate
	if engineStats.TotalTweets > 0 {
		stats["spam_rate"] = float64(engineStats.SpamTweets) / float64(engineStats.TotalTweets)
	} else {
		stats["spam_rate"] = 0.0
	}
	
	// Add cluster statistics
	if len(clusters) > 0 {
		maxClusterSize := 0
		totalClusterSize := 0
		coordinatedClusters := 0
		
		for _, cluster := range clusters {
			if cluster.Size > maxClusterSize {
				maxClusterSize = cluster.Size
			}
			totalClusterSize += cluster.Size
			if cluster.IsCoordinated {
				coordinatedClusters++
			}
		}
		
		stats["max_cluster_size"] = maxClusterSize
		stats["avg_cluster_size"] = float64(totalClusterSize) / float64(len(clusters))
		stats["coordinated_clusters"] = coordinatedClusters
	}
	
	// Add account statistics
	if len(suspiciousAccounts) > 0 {
		totalBotScore := 0.0
		highRiskAccounts := 0
		
		for _, account := range suspiciousAccounts {
			totalBotScore += account.BotScore
			if account.BotScore >= 0.7 {
				highRiskAccounts++
			}
		}
		
		stats["avg_bot_score"] = totalBotScore / float64(len(suspiciousAccounts))
		stats["high_risk_accounts"] = highRiskAccounts
	}
	
	return stats
}

// ExportResults exports detection results to a structured format
func (app *SpamDetectorApp) ExportResults() (map[string]interface{}, error) {
	clusters := app.engine.GetClusters()
	tweets := app.engine.GetTweets()
	accounts := app.engine.GetSuspiciousAccounts()
	stats := app.GetStatistics()
	
	export := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generated_at":  time.Now().Format(time.RFC3339),
			"version":       app.config.App.Version,
			"configuration": app.config.GetDetectionConfig(),
		},
		"statistics": stats,
		"clusters":   clusters,
		"tweets":     tweets,
		"accounts":   accounts,
	}
	
	return export, nil
}

// GetTweetsByCluster returns all tweets in a specific cluster
func (app *SpamDetectorApp) GetTweetsByCluster(clusterID string) ([]*models.Tweet, error) {
	clusters := app.engine.GetClusters()
	cluster, exists := clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", clusterID)
	}
	
	tweets := make([]*models.Tweet, len(cluster.Tweets))
	for i, tweet := range cluster.Tweets {
		tweets[i] = &tweet
	}
	
	return tweets, nil
}

// GetAccountActivity returns activity information for a specific account
func (app *SpamDetectorApp) GetAccountActivity(accountID string) (map[string]interface{}, error) {
	tweets := app.engine.GetTweets()
	
	var accountTweets []*models.Tweet
	var account *models.Account
	
	for _, tweet := range tweets {
		if tweet.AuthorID == accountID {
			accountTweets = append(accountTweets, tweet)
			if account == nil {
				account = &tweet.Author
			}
		}
	}
	
	if account == nil {
		return nil, fmt.Errorf("account %s not found", accountID)
	}
	
	// Calculate activity metrics
	spamTweets := 0
	clusters := make(map[string]bool)
	
	for _, tweet := range accountTweets {
		if tweet.IsSpam {
			spamTweets++
			if tweet.ClusterID != "" {
				clusters[tweet.ClusterID] = true
			}
		}
	}
	
	activity := map[string]interface{}{
		"account":           account,
		"total_tweets":      len(accountTweets),
		"spam_tweets":       spamTweets,
		"spam_rate":         float64(spamTweets) / float64(len(accountTweets)),
		"involved_clusters": len(clusters),
		"tweets":            accountTweets,
	}
	
	return activity, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
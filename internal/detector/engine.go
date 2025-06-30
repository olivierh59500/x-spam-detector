package detector

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"x-spam-detector/internal/lsh"
	"x-spam-detector/internal/models"
)

// SpamDetectionEngine is the main engine for detecting spam clusters
type SpamDetectionEngine struct {
	minHashLSH    *lsh.MinHashLSH
	simHashLSH    *lsh.SimHashLSH
	tweets        map[string]*models.Tweet
	accounts      map[string]*models.Account
	clusters      map[string]*models.SpamCluster
	
	// Configuration
	config        Config
	
	// Synchronization
	mutex         sync.RWMutex
	
	// Statistics
	stats         Statistics
}

// Config holds configuration for the spam detection engine
type Config struct {
	// LSH Parameters
	MinHashThreshold    float64 `yaml:"minhash_threshold"`
	SimHashThreshold    int     `yaml:"simhash_threshold"`
	MinHashBands        int     `yaml:"minhash_bands"`
	MinHashHashes       int     `yaml:"minhash_hashes"`
	SimHashTables       int     `yaml:"simhash_tables"`
	
	// Clustering Parameters
	MinClusterSize      int     `yaml:"min_cluster_size"`
	MaxClusterAge       int     `yaml:"max_cluster_age_hours"`
	
	// Detection Parameters
	SpamScoreThreshold  float64 `yaml:"spam_score_threshold"`
	BotScoreThreshold   float64 `yaml:"bot_score_threshold"`
	CoordinationThreshold int   `yaml:"coordination_threshold"`
	
	// Performance
	BatchSize           int     `yaml:"batch_size"`
	EnableHybridMode    bool    `yaml:"enable_hybrid_mode"`
}

// Statistics holds runtime statistics for the detection engine
type Statistics struct {
	TotalTweets         int64
	SpamTweets          int64
	ActiveClusters      int64
	SuspiciousAccounts  int64
	ProcessedBatches    int64
	LastProcessingTime  time.Duration
	TotalProcessingTime time.Duration
	MemoryUsage         int64
}

// NewSpamDetectionEngine creates a new spam detection engine
func NewSpamDetectionEngine(config Config) (*SpamDetectionEngine, error) {
	// Calculate optimal LSH parameters if not provided
	if config.MinHashBands == 0 || config.MinHashHashes == 0 {
		bands, _ := lsh.CalculateOptimalParameters(config.MinHashThreshold, 128)
		config.MinHashBands = bands
		config.MinHashHashes = 128
	}
	
	if config.SimHashTables == 0 {
		config.SimHashTables = 4
	}
	
	// Create MinHash LSH with error handling
	minHashLSH, err := lsh.NewMinHashLSH(config.MinHashHashes, config.MinHashBands, config.MinHashThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to create MinHash LSH: %w", err)
	}
	
	return &SpamDetectionEngine{
		minHashLSH: minHashLSH,
		simHashLSH: lsh.NewSimHashLSH(config.SimHashThreshold, config.SimHashTables),
		tweets:     make(map[string]*models.Tweet),
		accounts:   make(map[string]*models.Account),
		clusters:   make(map[string]*models.SpamCluster),
		config:     config,
	}, nil
}

// AddTweet adds a new tweet to the detection engine
func (e *SpamDetectionEngine) AddTweet(tweet *models.Tweet) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	// Store tweet and account
	e.tweets[tweet.ID] = tweet
	e.accounts[tweet.AuthorID] = &tweet.Author
	
	// Calculate bot score for the account
	tweet.Author.CalculateBotScore()
	
	// Add to LSH indices
	e.minHashLSH.Add(tweet.ID, tweet.Text)
	e.simHashLSH.Add(tweet.ID, tweet.Text)
	
	// Update statistics
	e.stats.TotalTweets++
	if tweet.Author.IsSuspicious {
		e.stats.SuspiciousAccounts++
	}
	
	return nil
}

// AddTweets adds multiple tweets in batch
func (e *SpamDetectionEngine) AddTweets(tweets []*models.Tweet) error {
	for _, tweet := range tweets {
		if err := e.AddTweet(tweet); err != nil {
			return fmt.Errorf("failed to add tweet %s: %w", tweet.ID, err)
		}
	}
	return nil
}

// DetectSpam performs spam detection and clustering
func (e *SpamDetectionEngine) DetectSpam() (*models.DetectionResult, error) {
	startTime := time.Now()
	
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	result := &models.DetectionResult{
		ID:             fmt.Sprintf("detection_%d", time.Now().Unix()),
		Timestamp:      startTime,
		Algorithm:      e.getAlgorithmName(),
		Threshold:      e.config.MinHashThreshold,
		MinClusterSize: e.config.MinClusterSize,
		TotalTweets:    len(e.tweets),
	}
	
	// Clear existing clusters
	e.clusters = make(map[string]*models.SpamCluster)
	
	var allClusters [][]lsh.SimilarDocument
	
	if e.config.EnableHybridMode {
		// Use hybrid approach: combine MinHash and SimHash results
		allClusters = e.hybridClustering()
	} else {
		// Use MinHash clustering by default
		allClusters = e.minHashLSH.GetAllSimilarClusters()
	}
	
	// Process clusters
	spamClusters := make([]models.SpamCluster, 0)
	spamTweetCount := 0
	suspiciousAccounts := make(map[string]*models.Account)
	
	for _, cluster := range allClusters {
		if len(cluster) >= e.config.MinClusterSize {
			spamCluster := e.createSpamCluster(cluster)
			
			// Calculate cluster confidence and pattern
			e.analyzeCluster(spamCluster)
			
			if spamCluster.Confidence >= e.config.SpamScoreThreshold {
				spamClusters = append(spamClusters, *spamCluster)
				e.clusters[spamCluster.ID] = spamCluster
				
				// Mark tweets as spam
				for _, tweetID := range spamCluster.TweetIDs {
					if tweet, exists := e.tweets[tweetID]; exists {
						tweet.IsSpam = true
						tweet.SpamScore = spamCluster.Confidence
						tweet.ClusterID = spamCluster.ID
						spamTweetCount++
					}
				}
				
				// Collect suspicious accounts
				for _, accountID := range spamCluster.AccountIDs {
					if account, exists := e.accounts[accountID]; exists {
						account.SpamTweetCount++
						suspiciousAccounts[accountID] = account
					}
				}
			}
		}
	}
	
	// Update result
	result.SpamTweets = spamTweetCount
	result.SpamClusters = spamClusters
	
	// Convert suspicious accounts map to slice
	for _, account := range suspiciousAccounts {
		result.SuspiciousAccounts = append(result.SuspiciousAccounts, *account)
	}
	
	// Calculate statistics
	e.calculateResultStatistics(result)
	
	// Update engine statistics
	processingTime := time.Since(startTime)
	e.stats.LastProcessingTime = processingTime
	e.stats.TotalProcessingTime += processingTime
	e.stats.ProcessedBatches++
	e.stats.ActiveClusters = int64(len(spamClusters))
	e.stats.SpamTweets = int64(spamTweetCount)
	
	result.ProcessingTime = processingTime
	
	log.Printf("Spam detection completed: %d spam tweets in %d clusters (processing time: %v)",
		spamTweetCount, len(spamClusters), processingTime)
	
	return result, nil
}

// hybridClustering combines MinHash and SimHash clustering results
func (e *SpamDetectionEngine) hybridClustering() [][]lsh.SimilarDocument {
	minHashClusters := e.minHashLSH.GetAllSimilarClusters()
	simHashClusters := e.simHashLSH.GetAllSimilarClusters()
	
	// Create a map to track which documents are already clustered
	clustered := make(map[string]bool)
	var hybridClusters [][]lsh.SimilarDocument
	
	// Add MinHash clusters first (they tend to be more precise)
	for _, cluster := range minHashClusters {
		if len(cluster) >= e.config.MinClusterSize {
			hybridClusters = append(hybridClusters, cluster)
			for _, doc := range cluster {
				clustered[doc.ID] = true
			}
		}
	}
	
	// Add SimHash clusters that don't overlap significantly with MinHash clusters
	for _, cluster := range simHashClusters {
		if len(cluster) >= e.config.MinClusterSize {
			// Check overlap with existing clusters
			overlapCount := 0
			for _, doc := range cluster {
				if clustered[doc.ID] {
					overlapCount++
				}
			}
			
			// If less than 50% overlap, add as new cluster
			if float64(overlapCount)/float64(len(cluster)) < 0.5 {
				hybridClusters = append(hybridClusters, cluster)
				for _, doc := range cluster {
					clustered[doc.ID] = true
				}
			}
		}
	}
	
	return hybridClusters
}

// createSpamCluster creates a SpamCluster from a similarity cluster
func (e *SpamDetectionEngine) createSpamCluster(cluster []lsh.SimilarDocument) *models.SpamCluster {
	spamCluster := models.NewSpamCluster()
	spamCluster.DetectionMethod = e.getAlgorithmName()
	spamCluster.LSHThreshold = e.config.MinHashThreshold
	
	for _, doc := range cluster {
		if tweet, exists := e.tweets[doc.ID]; exists {
			spamCluster.AddTweet(*tweet)
		}
	}
	
	return spamCluster
}

// analyzeCluster performs advanced analysis on a spam cluster
func (e *SpamDetectionEngine) analyzeCluster(cluster *models.SpamCluster) {
	// Calculate confidence based on multiple factors
	confidence := 0.0
	
	// Factor 1: Size of cluster (larger clusters are more suspicious)
	sizeScore := float64(cluster.Size) / 100.0
	if sizeScore > 1.0 {
		sizeScore = 1.0
	}
	confidence += sizeScore * 0.3
	
	// Factor 2: Temporal clustering (tweets posted in short time frame)
	if cluster.Duration.Minutes() < 60 && cluster.Size > 5 {
		confidence += 0.3
	} else if cluster.Duration.Hours() < 1 && cluster.Size > 3 {
		confidence += 0.2
	}
	
	// Factor 3: Account diversity (fewer unique accounts = more suspicious)
	accountDiversityScore := 1.0 - (float64(cluster.UniqueAccounts) / float64(cluster.Size))
	confidence += accountDiversityScore * 0.2
	
	// Factor 4: Bot scores of involved accounts
	botScoreSum := 0.0
	for _, account := range cluster.Accounts {
		botScoreSum += account.BotScore
	}
	avgBotScore := botScoreSum / float64(len(cluster.Accounts))
	confidence += avgBotScore * 0.2
	
	cluster.Confidence = confidence
	
	// Determine suspicion level
	if confidence >= 0.8 {
		cluster.SuspicionLevel = "critical"
	} else if confidence >= 0.6 {
		cluster.SuspicionLevel = "high"
	} else if confidence >= 0.4 {
		cluster.SuspicionLevel = "medium"
	} else {
		cluster.SuspicionLevel = "low"
	}
	
	// Check for coordination patterns
	cluster.IsCoordinatedPattern()
	
	// Extract common pattern/template
	cluster.Pattern = e.extractPattern(cluster.Tweets)
	cluster.Template = e.extractTemplate(cluster.Tweets)
}

// extractPattern extracts a description of the spam pattern
func (e *SpamDetectionEngine) extractPattern(tweets []models.Tweet) string {
	if len(tweets) == 0 {
		return ""
	}
	
	// Analyze common elements
	hasURLs := 0
	hasMentions := 0
	hasHashtags := 0
	avgLength := 0
	
	for _, tweet := range tweets {
		if len(tweet.URLs) > 0 {
			hasURLs++
		}
		if len(tweet.Mentions) > 0 {
			hasMentions++
		}
		if len(tweet.Hashtags) > 0 {
			hasHashtags++
		}
		avgLength += len(tweet.Text)
	}
	
	avgLength /= len(tweets)
	
	pattern := fmt.Sprintf("Text similarity cluster (avg length: %d chars)", avgLength)
	
	if float64(hasURLs)/float64(len(tweets)) > 0.5 {
		pattern += ", contains URLs"
	}
	if float64(hasMentions)/float64(len(tweets)) > 0.5 {
		pattern += ", contains mentions"
	}
	if float64(hasHashtags)/float64(len(tweets)) > 0.5 {
		pattern += ", contains hashtags"
	}
	
	return pattern
}

// extractTemplate extracts a common template from the tweets
func (e *SpamDetectionEngine) extractTemplate(tweets []models.Tweet) string {
	if len(tweets) == 0 {
		return ""
	}
	
	// For simplicity, return the shortest tweet as template
	// In a production system, this could use more sophisticated template extraction
	shortestTweet := tweets[0].Text
	for _, tweet := range tweets[1:] {
		if len(tweet.Text) < len(shortestTweet) {
			shortestTweet = tweet.Text
		}
	}
	
	// Truncate if too long
	if len(shortestTweet) > 100 {
		shortestTweet = shortestTweet[:97] + "..."
	}
	
	return shortestTweet
}

// calculateResultStatistics calculates various statistics for the detection result
func (e *SpamDetectionEngine) calculateResultStatistics(result *models.DetectionResult) {
	if result.TotalTweets > 0 {
		result.SpamRate = float64(result.SpamTweets) / float64(result.TotalTweets)
	}
	
	if len(result.SpamClusters) > 0 {
		// Find largest cluster
		maxSize := 0
		totalSize := 0
		coordinatedCount := 0
		
		for _, cluster := range result.SpamClusters {
			if cluster.Size > maxSize {
				maxSize = cluster.Size
			}
			totalSize += cluster.Size
			if cluster.IsCoordinated {
				coordinatedCount++
			}
		}
		
		result.LargestCluster = maxSize
		result.AvgClusterSize = float64(totalSize) / float64(len(result.SpamClusters))
		result.CoordinatedClusters = coordinatedCount
	}
}

// getAlgorithmName returns the name of the algorithm being used
func (e *SpamDetectionEngine) getAlgorithmName() string {
	if e.config.EnableHybridMode {
		return "hybrid"
	}
	return "minhash"
}

// GetStatistics returns current engine statistics
func (e *SpamDetectionEngine) GetStatistics() Statistics {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.stats
}

// GetClusters returns all current spam clusters
func (e *SpamDetectionEngine) GetClusters() map[string]*models.SpamCluster {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	clusters := make(map[string]*models.SpamCluster)
	for id, cluster := range e.clusters {
		clusters[id] = cluster
	}
	return clusters
}

// GetTweets returns all tweets
func (e *SpamDetectionEngine) GetTweets() map[string]*models.Tweet {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	tweets := make(map[string]*models.Tweet)
	for id, tweet := range e.tweets {
		tweets[id] = tweet
	}
	return tweets
}

// GetSuspiciousAccounts returns accounts marked as suspicious
func (e *SpamDetectionEngine) GetSuspiciousAccounts() []*models.Account {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	var suspicious []*models.Account
	for _, account := range e.accounts {
		if account.IsSuspicious || account.SpamTweetCount > 0 {
			suspicious = append(suspicious, account)
		}
	}
	
	// Sort by bot score descending
	sort.Slice(suspicious, func(i, j int) bool {
		return suspicious[i].BotScore > suspicious[j].BotScore
	})
	
	return suspicious
}

// Clear removes all data from the engine
func (e *SpamDetectionEngine) Clear() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	// Recreate MinHash LSH with error handling
	minHashLSH, err := lsh.NewMinHashLSH(e.config.MinHashHashes, e.config.MinHashBands, e.config.MinHashThreshold)
	if err != nil {
		return fmt.Errorf("failed to recreate MinHash LSH: %w", err)
	}
	
	e.minHashLSH = minHashLSH
	e.simHashLSH = lsh.NewSimHashLSH(e.config.SimHashThreshold, e.config.SimHashTables)
	e.tweets = make(map[string]*models.Tweet)
	e.accounts = make(map[string]*models.Account)
	e.clusters = make(map[string]*models.SpamCluster)
	
	// Reset statistics except cumulative ones
	e.stats.TotalTweets = 0
	e.stats.SpamTweets = 0
	e.stats.ActiveClusters = 0
	e.stats.SuspiciousAccounts = 0
	
	return nil
}

// UpdateConfig updates the engine configuration
func (e *SpamDetectionEngine) UpdateConfig(config Config) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	// Validate and create new MinHash LSH with new parameters
	minHashLSH, err := lsh.NewMinHashLSH(config.MinHashHashes, config.MinHashBands, config.MinHashThreshold)
	if err != nil {
		return fmt.Errorf("failed to create MinHash LSH with new config: %w", err)
	}
	
	e.config = config
	e.minHashLSH = minHashLSH
	e.simHashLSH = lsh.NewSimHashLSH(config.SimHashThreshold, config.SimHashTables)
	
	// Re-add all tweets to the new LSH instances
	for _, tweet := range e.tweets {
		e.minHashLSH.Add(tweet.ID, tweet.Text)
		e.simHashLSH.Add(tweet.ID, tweet.Text)
	}
	
	return nil
}
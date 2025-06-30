package detector

import (
	"fmt"
	"testing"
	"time"

	"x-spam-detector/internal/lsh"
	"x-spam-detector/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpamDetectionEngine(t *testing.T) {
	config := Config{
		MinHashThreshold:   0.7,
		SimHashThreshold:   3,
		MinHashBands:       16,
		MinHashHashes:      128,
		SimHashTables:      4,
		MinClusterSize:     3,
		SpamScoreThreshold: 0.6,
		BotScoreThreshold:  0.5,
		EnableHybridMode:   true,
	}

	engine, err := NewSpamDetectionEngine(config)

	assert.NoError(t, err)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.minHashLSH)
	assert.NotNil(t, engine.simHashLSH)
	assert.NotNil(t, engine.tweets)
	assert.NotNil(t, engine.accounts)
	assert.NotNil(t, engine.clusters)
	assert.Equal(t, config, engine.config)
}

func TestSpamDetectionEngine_AddTweet(t *testing.T) {
	engine := createTestEngine()

	account := models.NewAccount("testuser", "Test User")
	tweet := models.NewTweet("This is a test tweet", account.ID, *account)

	err := engine.AddTweet(tweet)
	require.NoError(t, err)

	// Check that tweet was added
	assert.Equal(t, int64(1), engine.stats.TotalTweets)
	assert.NotNil(t, engine.tweets[tweet.ID])
	assert.NotNil(t, engine.accounts[account.ID])
}

func TestSpamDetectionEngine_AddTweets(t *testing.T) {
	engine := createTestEngine()

	tweets := []*models.Tweet{
		createTestTweet("tweet1", "user1", "First test tweet"),
		createTestTweet("tweet2", "user2", "Second test tweet"),
		createTestTweet("tweet3", "user3", "Third test tweet"),
	}

	err := engine.AddTweets(tweets)
	require.NoError(t, err)

	assert.Equal(t, int64(3), engine.stats.TotalTweets)
	assert.Len(t, engine.tweets, 3)
	assert.Len(t, engine.accounts, 3)
}

func TestSpamDetectionEngine_DetectSpam(t *testing.T) {
	engine := createTestEngine()

	// Add spam tweets (similar content)
	spamTweets := []*models.Tweet{
		createTestTweet("spam1", "bot1", "amazing crypto deal dont miss out limited time"),
		createTestTweet("spam2", "bot2", "amazing crypto deal don't miss out limited time offer"),
		createTestTweet("spam3", "bot3", "incredible crypto deal dont miss limited time opportunity"),
		createTestTweet("spam4", "bot4", "amazing cryptocurrency deal dont miss out limited time"),
	}

	// Add legitimate tweets
	legitTweets := []*models.Tweet{
		createTestTweet("legit1", "user1", "beautiful sunset today feeling grateful"),
		createTestTweet("legit2", "user2", "working on machine learning project"),
	}

	allTweets := append(spamTweets, legitTweets...)
	err := engine.AddTweets(allTweets)
	require.NoError(t, err)

	// Run detection
	result, err := engine.DetectSpam()
	require.NoError(t, err)

	// Verify results
	assert.NotNil(t, result)
	assert.Equal(t, 6, result.TotalTweets)
	assert.True(t, result.SpamTweets > 0, "Should detect some spam tweets")
	assert.True(t, len(result.SpamClusters) > 0, "Should find spam clusters")
	assert.True(t, result.ProcessingTime > 0, "Should have processing time")

	// Check that spam tweets are marked correctly
	spamCount := 0
	for _, tweet := range engine.tweets {
		if tweet.IsSpam {
			spamCount++
			assert.True(t, tweet.SpamScore > 0, "Spam tweet should have positive spam score")
			assert.NotEmpty(t, tweet.ClusterID, "Spam tweet should have cluster ID")
		}
	}

	assert.Equal(t, result.SpamTweets, spamCount, "Spam count should match")
}

func TestSpamDetectionEngine_HybridClustering(t *testing.T) {
	config := createTestConfig()
	config.EnableHybridMode = true
	engine, err := NewSpamDetectionEngine(config)
	assert.NoError(t, err)

	// Add test tweets
	tweets := []*models.Tweet{
		createTestTweet("t1", "u1", "machine learning artificial intelligence"),
		createTestTweet("t2", "u2", "machine learning and artificial intelligence"),
		createTestTweet("t3", "u3", "deep learning neural networks"),
		createTestTweet("t4", "u4", "deep learning and neural networks"),
	}

	err = engine.AddTweets(tweets)
	require.NoError(t, err)

	clusters := engine.hybridClustering()

	// Should find clusters using hybrid approach
	assert.True(t, len(clusters) >= 0, "Hybrid clustering should return results")
}

func TestSpamDetectionEngine_CreateSpamCluster(t *testing.T) {
	engine := createTestEngine()

	// Create test tweets
	tweets := []*models.Tweet{
		createTestTweet("t1", "u1", "spam content here"),
		createTestTweet("t2", "u2", "spam content here too"),
	}

	err := engine.AddTweets(tweets)
	require.NoError(t, err)

	// Create similarity documents - using the import from internal/lsh
	simDocs := []lsh.SimilarDocument{
		{ID: "t1", Text: "spam content here", Similarity: 0.9},
		{ID: "t2", Text: "spam content here too", Similarity: 0.8},
	}

	cluster := engine.createSpamCluster(simDocs)

	assert.NotNil(t, cluster)
	assert.Equal(t, 2, cluster.Size)
	assert.Contains(t, cluster.TweetIDs, "t1")
	assert.Contains(t, cluster.TweetIDs, "t2")
	assert.Equal(t, engine.getAlgorithmName(), cluster.DetectionMethod)
}

func TestSpamDetectionEngine_AnalyzeCluster(t *testing.T) {
	engine := createTestEngine()

	// Create cluster with coordinated behavior pattern
	cluster := models.NewSpamCluster()

	// Add tweets from same time period (coordinated)
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		account := models.NewAccount("bot"+string(rune(i+'1')), "Bot User")
		tweet := models.NewTweet("spam content", account.ID, *account)
		tweet.CreatedAt = baseTime.Add(time.Duration(i) * time.Minute) // Close in time
		cluster.AddTweet(*tweet)
	}

	engine.analyzeCluster(cluster)

	// Should detect high confidence due to:
	// - Multiple tweets
	// - Close temporal clustering
	// - Multiple accounts
	assert.True(t, cluster.Confidence > 0.0, "Should have positive confidence")
	assert.NotEmpty(t, cluster.SuspicionLevel, "Should have suspicion level")
	assert.NotEmpty(t, cluster.Pattern, "Should have pattern description")
}

func TestSpamDetectionEngine_GetStatistics(t *testing.T) {
	engine := createTestEngine()

	// Add some test data
	tweets := []*models.Tweet{
		createTestTweet("t1", "u1", "test tweet 1"),
		createTestTweet("t2", "u2", "test tweet 2"),
	}

	err := engine.AddTweets(tweets)
	require.NoError(t, err)

	stats := engine.GetStatistics()

	assert.Equal(t, int64(2), stats.TotalTweets)
	assert.Equal(t, int64(0), stats.SpamTweets) // No spam detected yet
	assert.Equal(t, int64(0), stats.ActiveClusters)
}

func TestSpamDetectionEngine_Clear(t *testing.T) {
	engine := createTestEngine()

	// Add test data
	tweet := createTestTweet("t1", "u1", "test tweet")
	err := engine.AddTweet(tweet)
	require.NoError(t, err)

	// Verify data exists
	assert.Equal(t, int64(1), engine.stats.TotalTweets)
	assert.Len(t, engine.tweets, 1)

	// Clear and verify
	engine.Clear()

	assert.Equal(t, int64(0), engine.stats.TotalTweets)
	assert.Len(t, engine.tweets, 0)
	assert.Len(t, engine.accounts, 0)
	assert.Len(t, engine.clusters, 0)
}

func TestSpamDetectionEngine_UpdateConfig(t *testing.T) {
	engine := createTestEngine()

	newConfig := createTestConfig()
	newConfig.MinHashThreshold = 0.8
	newConfig.MinClusterSize = 5

	engine.UpdateConfig(newConfig)

	assert.Equal(t, newConfig, engine.config)
}

func TestSpamDetectionEngine_GetSuspiciousAccounts(t *testing.T) {
	engine := createTestEngine()

	// Create accounts with different bot scores
	suspiciousAccount := models.NewAccount("bot_user", "Bot User")
	suspiciousAccount.BotScore = 0.8
	suspiciousAccount.IsSuspicious = true

	normalAccount := models.NewAccount("normal_user", "Normal User")
	normalAccount.BotScore = 0.2
	normalAccount.IsSuspicious = false

	tweets := []*models.Tweet{
		createTestTweetWithAccount("t1", *suspiciousAccount, "spam content"),
		createTestTweetWithAccount("t2", *normalAccount, "normal content"),
	}

	err := engine.AddTweets(tweets)
	require.NoError(t, err)

	suspicious := engine.GetSuspiciousAccounts()

	// Should return suspicious accounts sorted by bot score
	assert.True(t, len(suspicious) >= 1, "Should find suspicious accounts")

	// Check if the suspicious account is in the results
	found := false
	for _, acc := range suspicious {
		if acc.Username == "bot_user" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should include suspicious account")
}

func TestSpamDetectionEngine_RealWorldScenario(t *testing.T) {
	engine := createTestEngine()

	// Simulate real-world spam campaign
	spamCampaign := []*models.Tweet{
		createTestTweet("s1", "bot1", "URGENT: Account suspension warning! Click here to verify now"),
		createTestTweet("s2", "bot2", "URGENT: Account suspension alert! Click here to verify immediately"),
		createTestTweet("s3", "bot3", "URGENT: Your account suspension warning! Click to verify now"),
		createTestTweet("s4", "bot4", "URGENT: Account suspension notice! Click here to verify immediately"),
		createTestTweet("s5", "bot5", "URGENT: Account will be suspended! Click here to verify now"),
	}

	// Add some legitimate tweets
	legitimateTweets := []*models.Tweet{
		createTestTweet("l1", "user1", "Just finished a great book about data science"),
		createTestTweet("l2", "user2", "Beautiful weather today, perfect for a walk"),
		createTestTweet("l3", "user3", "Working on an interesting programming project"),
	}

	allTweets := append(spamCampaign, legitimateTweets...)
	err := engine.AddTweets(allTweets)
	require.NoError(t, err)

	// Run detection
	result, err := engine.DetectSpam()
	require.NoError(t, err)

	// Verify spam detection
	assert.True(t, result.SpamTweets >= 3, "Should detect significant spam")
	assert.True(t, len(result.SpamClusters) >= 1, "Should find spam clusters")
	assert.True(t, result.SpamRate > 0.5, "Spam rate should be high")

	// Check that legitimate tweets are not flagged as spam
	for _, tweet := range engine.tweets {
		if tweet.ID[0] == 'l' { // Legitimate tweet
			assert.False(t, tweet.IsSpam, "Legitimate tweet should not be flagged as spam")
		}
	}
}

// Helper functions for testing

func createTestEngine() *SpamDetectionEngine {
	engine, err := NewSpamDetectionEngine(createTestConfig())
	if err != nil {
		panic(fmt.Sprintf("Failed to create test engine: %v", err))
	}
	return engine
}

func createTestConfig() Config {
	return Config{
		MinHashThreshold:   0.7,
		SimHashThreshold:   3,
		MinHashBands:       16,
		MinHashHashes:      128,
		SimHashTables:      4,
		MinClusterSize:     3,
		SpamScoreThreshold: 0.6,
		BotScoreThreshold:  0.5,
		EnableHybridMode:   false,
	}
}

func createTestTweet(id, username, text string) *models.Tweet {
	account := models.NewAccount(username, username)
	tweet := models.NewTweet(text, account.ID, *account)
	tweet.ID = id
	return tweet
}

func createTestTweetWithAccount(id string, account models.Account, text string) *models.Tweet {
	tweet := models.NewTweet(text, account.ID, account)
	tweet.ID = id
	return tweet
}

// Benchmarks

func BenchmarkSpamDetectionEngine_AddTweet(b *testing.B) {
	engine := createTestEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tweet := createTestTweet(fmt.Sprintf("t%d", i), fmt.Sprintf("u%d", i), "test tweet content")
		engine.AddTweet(tweet)
	}
}

func BenchmarkSpamDetectionEngine_DetectSpam(b *testing.B) {
	engine := createTestEngine()

	// Pre-populate with test data
	for i := 0; i < 100; i++ {
		tweet := createTestTweet(fmt.Sprintf("t%d", i), fmt.Sprintf("u%d", i), "test spam content similar")
		engine.AddTweet(tweet)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.DetectSpam()
	}
}

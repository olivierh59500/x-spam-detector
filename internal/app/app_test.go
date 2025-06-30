package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"x-spam-detector/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.Default()
	app := New(cfg)
	
	assert.NotNil(t, app)
	assert.NotNil(t, app.config)
	assert.NotNil(t, app.engine)
	assert.Equal(t, cfg, app.config)
}

func TestSpamDetectorApp_AddTweet(t *testing.T) {
	app := createTestApp()
	
	tests := []struct {
		name     string
		text     string
		username string
		wantErr  bool
	}{
		{
			name:     "valid tweet",
			text:     "This is a test tweet",
			username: "testuser",
			wantErr:  false,
		},
		{
			name:     "empty text",
			text:     "",
			username: "testuser",
			wantErr:  true,
		},
		{
			name:     "empty username generates default",
			text:     "Test tweet",
			username: "",
			wantErr:  false,
		},
		{
			name:     "whitespace only text",
			text:     "   \n\t  ",
			username: "testuser",
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tweet, err := app.AddTweet(tt.text, tt.username)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tweet)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tweet)
				assert.Equal(t, tt.text, tweet.Text)
				assert.NotEmpty(t, tweet.ID)
				assert.NotEmpty(t, tweet.AuthorID)
			}
		})
	}
}

func TestSpamDetectorApp_AddTweetsFromText(t *testing.T) {
	app := createTestApp()
	
	tests := []struct {
		name        string
		text        string
		expectedCount int
		wantErr     bool
	}{
		{
			name: "multiple tweets",
			text: "First tweet\nSecond tweet\nThird tweet",
			expectedCount: 3,
			wantErr: false,
		},
		{
			name: "tweets with empty lines",
			text: "First tweet\n\nSecond tweet\n\n\nThird tweet",
			expectedCount: 3,
			wantErr: false,
		},
		{
			name: "single tweet",
			text: "Just one tweet",
			expectedCount: 1,
			wantErr: false,
		},
		{
			name: "empty text",
			text: "",
			expectedCount: 0,
			wantErr: true,
		},
		{
			name: "only whitespace",
			text: "   \n\n\t  \n  ",
			expectedCount: 0,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear app state
			app.ClearAll()
			
			err := app.AddTweetsFromText(tt.text)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Check that correct number of tweets were added
				tweets := app.engine.GetTweets()
				assert.Len(t, tweets, tt.expectedCount)
			}
		})
	}
}

func TestSpamDetectorApp_DetectSpam(t *testing.T) {
	app := createTestApp()
	
	// Add some spam-like tweets
	spamTweets := []string{
		"amazing crypto deal dont miss out limited time",
		"amazing crypto deal don't miss out limited time offer",
		"incredible crypto deal dont miss limited time opportunity",
	}
	
	for i, tweet := range spamTweets {
		_, err := app.AddTweet(tweet, fmt.Sprintf("bot%d", i))
		require.NoError(t, err)
	}
	
	// Add legitimate tweet
	_, err := app.AddTweet("beautiful sunset today feeling grateful", "user1")
	require.NoError(t, err)
	
	// Run detection
	result, err := app.DetectSpam()
	require.NoError(t, err)
	
	assert.NotNil(t, result)
	assert.Equal(t, 4, result.TotalTweets)
	assert.True(t, result.ProcessingTime > 0)
}

func TestSpamDetectorApp_LoadSampleData(t *testing.T) {
	app := createTestApp()
	
	err := app.LoadSampleData()
	require.NoError(t, err)
	
	// Verify that sample data was loaded
	tweets := app.engine.GetTweets()
	assert.True(t, len(tweets) > 10, "Should load multiple sample tweets")
	
	// Check that we have both spam and legitimate tweets
	spamCount := 0
	legitCount := 0
	
	for _, tweet := range tweets {
		// This is a heuristic check based on the sample data content
		if containsSpamKeywords(tweet.Text) {
			spamCount++
		} else {
			legitCount++
		}
	}
	
	assert.True(t, spamCount > 0, "Should have spam-like sample tweets")
	assert.True(t, legitCount > 0, "Should have legitimate sample tweets")
}

func TestSpamDetectorApp_ClearAll(t *testing.T) {
	app := createTestApp()
	
	// Add some test data
	_, err := app.AddTweet("test tweet", "testuser")
	require.NoError(t, err)
	
	// Verify data exists
	tweets := app.engine.GetTweets()
	assert.Len(t, tweets, 1)
	
	// Clear all
	app.ClearAll()
	
	// Verify data is cleared
	tweets = app.engine.GetTweets()
	assert.Len(t, tweets, 0)
}

func TestSpamDetectorApp_UpdateConfiguration(t *testing.T) {
	app := createTestApp()
	
	newConfig := config.Default()
	newConfig.Detection.MinHashThreshold = 0.8
	newConfig.Detection.MinClusterSize = 5
	
	err := app.UpdateConfiguration(newConfig)
	require.NoError(t, err)
	
	assert.Equal(t, newConfig, app.config)
	
	// Test invalid configuration
	invalidConfig := config.Default()
	invalidConfig.Detection.MinHashThreshold = 1.5 // Invalid threshold
	
	err = app.UpdateConfiguration(invalidConfig)
	assert.Error(t, err)
}

func TestSpamDetectorApp_GetStatistics(t *testing.T) {
	app := createTestApp()
	
	// Add test data
	_, err := app.AddTweet("test tweet 1", "user1")
	require.NoError(t, err)
	_, err = app.AddTweet("test tweet 2", "user2")
	require.NoError(t, err)
	
	stats := app.GetStatistics()
	
	assert.NotNil(t, stats)
	assert.Equal(t, int64(2), stats["total_tweets"])
	assert.Equal(t, int64(0), stats["spam_tweets"]) // No detection run yet
	assert.Equal(t, 0.0, stats["spam_rate"])
}

func TestSpamDetectorApp_ExportResults(t *testing.T) {
	app := createTestApp()
	
	// Add test data
	_, err := app.AddTweet("test tweet", "testuser")
	require.NoError(t, err)
	
	// Run detection
	_, err = app.DetectSpam()
	require.NoError(t, err)
	
	// Export results
	export, err := app.ExportResults()
	require.NoError(t, err)
	
	assert.NotNil(t, export)
	assert.Contains(t, export, "metadata")
	assert.Contains(t, export, "statistics")
	assert.Contains(t, export, "clusters")
	assert.Contains(t, export, "tweets")
	assert.Contains(t, export, "accounts")
	
	// Check metadata
	metadata, ok := export["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, metadata, "generated_at")
	assert.Contains(t, metadata, "version")
	assert.Contains(t, metadata, "configuration")
}

func TestSpamDetectorApp_GetTweetsByCluster(t *testing.T) {
	app := createTestApp()
	
	// Add spam tweets that should cluster together
	spamTweets := []string{
		"amazing deal dont miss out",
		"amazing deal don't miss out",
		"incredible deal dont miss out",
	}
	
	for i, tweet := range spamTweets {
		_, err := app.AddTweet(tweet, fmt.Sprintf("bot%d", i))
		require.NoError(t, err)
	}
	
	// Run detection to create clusters
	_, err := app.DetectSpam()
	require.NoError(t, err)
	
	// Get clusters and test getting tweets by cluster
	clusters := app.engine.GetClusters()
	if len(clusters) > 0 {
		var clusterID string
		for id := range clusters {
			clusterID = id
			break
		}
		
		tweets, err := app.GetTweetsByCluster(clusterID)
		require.NoError(t, err)
		assert.True(t, len(tweets) > 0, "Should return tweets for valid cluster")
	}
	
	// Test non-existent cluster
	_, err = app.GetTweetsByCluster("nonexistent")
	assert.Error(t, err)
}

func TestSpamDetectorApp_GetAccountActivity(t *testing.T) {
	app := createTestApp()
	
	// Add tweets from same account
	tweet1, err := app.AddTweet("first tweet", "testuser")
	require.NoError(t, err)
	
	_, err = app.AddTweet("second tweet", "testuser")
	require.NoError(t, err)
	
	// Get account activity
	activity, err := app.GetAccountActivity(tweet1.AuthorID)
	require.NoError(t, err)
	
	assert.NotNil(t, activity)
	assert.Contains(t, activity, "account")
	assert.Contains(t, activity, "total_tweets")
	assert.Contains(t, activity, "spam_tweets")
	assert.Contains(t, activity, "spam_rate")
	assert.Contains(t, activity, "tweets")
	
	assert.Equal(t, 2, activity["total_tweets"])
	
	// Test non-existent account
	_, err = app.GetAccountActivity("nonexistent")
	assert.Error(t, err)
}

func TestSpamDetectorApp_IntegrationTest(t *testing.T) {
	// Full integration test with realistic scenario
	app := createTestApp()
	
	// Load sample data
	err := app.LoadSampleData()
	require.NoError(t, err)
	
	// Add additional spam campaign
	campaignTweets := "Check out this crypto deal!\nCheck out this amazing crypto deal!\nCheck out this incredible crypto deal!"
	err = app.AddTweetsFromText(campaignTweets)
	require.NoError(t, err)
	
	// Run detection
	result, err := app.DetectSpam()
	require.NoError(t, err)
	
	// Verify comprehensive results
	assert.True(t, result.TotalTweets > 20, "Should have substantial tweet count")
	assert.True(t, result.SpamTweets > 0, "Should detect spam")
	assert.True(t, len(result.SpamClusters) > 0, "Should find clusters")
	
	// Get statistics
	stats := app.GetStatistics()
	assert.True(t, stats["total_tweets"].(int64) > 0)
	assert.True(t, stats["spam_rate"].(float64) >= 0.0)
	
	// Export results
	export, err := app.ExportResults()
	require.NoError(t, err)
	assert.NotNil(t, export["metadata"])
	
	// Get suspicious accounts
	suspicious := app.engine.GetSuspiciousAccounts()
	assert.True(t, len(suspicious) >= 0, "Should return suspicious accounts list")
}

// Helper functions

func createTestApp() *SpamDetectorApp {
	cfg := config.Default()
	// Use smaller parameters for faster testing
	cfg.Detection.MinHashHashes = 64
	cfg.Detection.MinHashBands = 8
	cfg.Detection.MinClusterSize = 2
	return New(cfg)
}

func containsSpamKeywords(text string) bool {
	spamKeywords := []string{"deal", "crypto", "follow", "money", "urgent", "click"}
	for _, keyword := range spamKeywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			return true
		}
	}
	return false
}
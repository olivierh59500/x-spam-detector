package models

import (
	"time"

	"github.com/google/uuid"
)

// Tweet represents a tweet/message from X (Twitter)
type Tweet struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	AuthorID  string    `json:"author_id"`
	Author    Account   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	
	// Engagement metrics
	RetweetCount int `json:"retweet_count"`
	LikeCount    int `json:"like_count"`
	ReplyCount   int `json:"reply_count"`
	
	// Content analysis
	URLs        []string `json:"urls"`
	Mentions    []string `json:"mentions"`
	Hashtags    []string `json:"hashtags"`
	
	// Spam detection results
	SpamScore     float64           `json:"spam_score"`
	IsSpam        bool              `json:"is_spam"`
	ClusterID     string            `json:"cluster_id,omitempty"`
	SimilarTweets []SimilarityMatch `json:"similar_tweets,omitempty"`
	
	// LSH signatures
	MinHashSig *uint64 `json:"minhash_sig,omitempty"`
	SimHashSig *uint64 `json:"simhash_sig,omitempty"`
}

// Account represents a user account on X (Twitter)
type Account struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	
	// Account metrics
	FollowersCount int `json:"followers_count"`
	FollowingCount int `json:"following_count"`
	TweetCount     int `json:"tweet_count"`
	
	// Profile information
	Bio              string `json:"bio"`
	Location         string `json:"location"`
	Website          string `json:"website"`
	ProfileImageURL  string `json:"profile_image_url"`
	IsVerified       bool   `json:"is_verified"`
	IsPrivate        bool   `json:"is_private"`
	
	// Bot detection metrics
	BotScore           float64   `json:"bot_score"`
	IsSuspicious       bool      `json:"is_suspicious"`
	AccountAge         int       `json:"account_age_days"`
	TweetsPerDay       float64   `json:"tweets_per_day"`
	FollowerToFollowing float64  `json:"follower_to_following_ratio"`
	
	// Associated spam activity
	SpamTweetCount int      `json:"spam_tweet_count"`
	SpamClusters   []string `json:"spam_clusters,omitempty"`
}

// SimilarityMatch represents a similarity match between tweets
type SimilarityMatch struct {
	TweetID    string  `json:"tweet_id"`
	Similarity float64 `json:"similarity"`
	Method     string  `json:"method"` // "minhash", "simhash"
}

// SpamCluster represents a cluster of similar spam tweets
type SpamCluster struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// Cluster metadata
	Size          int     `json:"size"`
	Confidence    float64 `json:"confidence"`
	Pattern       string  `json:"pattern"`       // Description of the spam pattern
	Template      string  `json:"template"`      // Common template extracted
	
	// Tweets in cluster
	Tweets    []Tweet  `json:"tweets"`
	TweetIDs  []string `json:"tweet_ids"`
	
	// Accounts involved
	Accounts         []Account `json:"accounts"`
	AccountIDs       []string  `json:"account_ids"`
	UniqueAccounts   int       `json:"unique_accounts"`
	
	// Temporal analysis
	FirstSeen        time.Time     `json:"first_seen"`
	LastSeen         time.Time     `json:"last_seen"`
	Duration         time.Duration `json:"duration"`
	TweetsPerMinute  float64       `json:"tweets_per_minute"`
	
	// Geographic distribution
	Locations        []string `json:"locations,omitempty"`
	
	// Network analysis
	IsCoordinated    bool    `json:"is_coordinated"`
	SuspicionLevel   string  `json:"suspicion_level"` // "low", "medium", "high", "critical"
	
	// Detection method
	DetectionMethod  string  `json:"detection_method"` // "minhash", "simhash", "hybrid"
	LSHThreshold     float64 `json:"lsh_threshold"`
}

// DetectionResult represents the result of spam detection analysis
type DetectionResult struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	
	// Input parameters
	Algorithm     string  `json:"algorithm"`     // "minhash", "simhash", "hybrid"
	Threshold     float64 `json:"threshold"`
	MinClusterSize int    `json:"min_cluster_size"`
	
	// Results
	TotalTweets      int           `json:"total_tweets"`
	SpamTweets       int           `json:"spam_tweets"`
	SpamClusters     []SpamCluster `json:"spam_clusters"`
	SuspiciousAccounts []Account   `json:"suspicious_accounts"`
	
	// Performance metrics
	ProcessingTime   time.Duration `json:"processing_time"`
	MemoryUsage      int64         `json:"memory_usage_bytes"`
	
	// Statistics
	SpamRate         float64 `json:"spam_rate"`
	LargestCluster   int     `json:"largest_cluster"`
	AvgClusterSize   float64 `json:"avg_cluster_size"`
	CoordinatedClusters int  `json:"coordinated_clusters"`
}

// NewTweet creates a new Tweet with generated ID
func NewTweet(text, authorID string, author Account) *Tweet {
	return &Tweet{
		ID:        uuid.New().String(),
		Text:      text,
		AuthorID:  authorID,
		Author:    author,
		CreatedAt: time.Now(),
		IsSpam:    false,
		SpamScore: 0.0,
	}
}

// NewAccount creates a new Account with generated ID
func NewAccount(username, displayName string) *Account {
	return &Account{
		ID:          uuid.New().String(),
		Username:    username,
		DisplayName: displayName,
		CreatedAt:   time.Now(),
		IsVerified:  false,
		IsPrivate:   false,
		BotScore:    0.0,
	}
}

// NewSpamCluster creates a new SpamCluster with generated ID
func NewSpamCluster() *SpamCluster {
	now := time.Now()
	return &SpamCluster{
		ID:        uuid.New().String(),
		CreatedAt: now,
		UpdatedAt: now,
		FirstSeen: now,
		LastSeen:  now,
		Tweets:    make([]Tweet, 0),
		TweetIDs:  make([]string, 0),
		Accounts:  make([]Account, 0),
		AccountIDs: make([]string, 0),
		SuspicionLevel: "medium",
	}
}

// AddTweet adds a tweet to the spam cluster
func (sc *SpamCluster) AddTweet(tweet Tweet) {
	sc.Tweets = append(sc.Tweets, tweet)
	sc.TweetIDs = append(sc.TweetIDs, tweet.ID)
	sc.Size = len(sc.Tweets)
	sc.UpdatedAt = time.Now()
	
	// Update temporal bounds
	if tweet.CreatedAt.Before(sc.FirstSeen) {
		sc.FirstSeen = tweet.CreatedAt
	}
	if tweet.CreatedAt.After(sc.LastSeen) {
		sc.LastSeen = tweet.CreatedAt
	}
	
	// Update duration and rate
	sc.Duration = sc.LastSeen.Sub(sc.FirstSeen)
	if sc.Duration.Minutes() > 0 {
		sc.TweetsPerMinute = float64(sc.Size) / sc.Duration.Minutes()
	}
	
	// Add account if not already present
	accountExists := false
	for _, existingID := range sc.AccountIDs {
		if existingID == tweet.AuthorID {
			accountExists = true
			break
		}
	}
	
	if !accountExists {
		sc.Accounts = append(sc.Accounts, tweet.Author)
		sc.AccountIDs = append(sc.AccountIDs, tweet.AuthorID)
		sc.UniqueAccounts = len(sc.AccountIDs)
	}
}

// CalculateBotScore calculates a bot score for an account based on various metrics
func (a *Account) CalculateBotScore() float64 {
	score := 0.0
	
	// Account age factor (newer accounts are more suspicious)
	if a.AccountAge < 30 {
		score += 0.3
	} else if a.AccountAge < 90 {
		score += 0.1
	}
	
	// Follower to following ratio
	if a.FollowingCount > 0 {
		ratio := float64(a.FollowersCount) / float64(a.FollowingCount)
		if ratio < 0.1 {
			score += 0.2
		} else if ratio > 10 {
			score += 0.1
		}
	}
	
	// High tweet activity
	if a.TweetsPerDay > 50 {
		score += 0.3
	} else if a.TweetsPerDay > 20 {
		score += 0.1
	}
	
	// Profile completeness (incomplete profiles are suspicious)
	if a.Bio == "" {
		score += 0.1
	}
	if a.Location == "" {
		score += 0.05
	}
	if a.ProfileImageURL == "" {
		score += 0.1
	}
	
	// Username patterns (common bot patterns)
	if len(a.Username) > 15 && containsNumbers(a.Username) {
		score += 0.15
	}
	
	a.BotScore = score
	a.IsSuspicious = score > 0.5
	
	return score
}

// IsCoordinatedPattern determines if a cluster shows coordinated behavior
func (sc *SpamCluster) IsCoordinatedPattern() bool {
	// Multiple accounts posting very similar content in short time frame
	if sc.Size >= 5 && sc.UniqueAccounts >= 3 && sc.TweetsPerMinute > 2 {
		sc.IsCoordinated = true
		sc.SuspicionLevel = "high"
		return true
	}
	
	// Many tweets from few accounts
	if sc.Size >= 10 && sc.UniqueAccounts <= 3 {
		sc.IsCoordinated = true
		sc.SuspicionLevel = "critical"
		return true
	}
	
	return false
}

// containsNumbers checks if a string contains numeric characters
func containsNumbers(s string) bool {
	for _, char := range s {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}
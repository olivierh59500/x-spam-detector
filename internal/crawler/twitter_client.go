package crawler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// TwitterClient handles communication with Twitter API v2
type TwitterClient struct {
	config      TwitterAPIConfig
	httpClient  *http.Client
	rateLimiter *RateLimiter
	mutex       sync.RWMutex
}

// TwitterAPIConfig holds Twitter API configuration
type TwitterAPIConfig struct {
	BearerToken    string `yaml:"bearer_token"`
	APIKey         string `yaml:"api_key"`
	APIKeySecret   string `yaml:"api_key_secret"`
	AccessToken    string `yaml:"access_token"`
	AccessSecret   string `yaml:"access_token_secret"`
	BaseURL        string `yaml:"base_url"`
	StreamURL      string `yaml:"stream_url"`
	RateLimit      int    `yaml:"rate_limit"`
	RequestsPerWindow int `yaml:"requests_per_window"`
}

// RateLimiter manages API rate limiting
type RateLimiter struct {
	requests chan struct{}
	ticker   *time.Ticker
	window   time.Duration
}

// TwitterAPIResponse represents the response from Twitter API
type TwitterAPIResponse struct {
	Data     []TweetData `json:"data"`
	Includes Includes    `json:"includes"`
	Meta     Meta        `json:"meta"`
	Errors   []APIError  `json:"errors"`
}

// TweetData represents tweet data from API
type TweetData struct {
	ID               string           `json:"id"`
	Text             string           `json:"text"`
	AuthorID         string           `json:"author_id"`
	CreatedAt        string           `json:"created_at"`
	PublicMetrics    PublicMetrics    `json:"public_metrics"`
	Lang             string           `json:"lang"`
	Geo              *GeoData         `json:"geo"`
	Entities         *Entities        `json:"entities"`
	ReferencedTweets []ReferencedTweet `json:"referenced_tweets"`
}

// UserData represents user data from API
type UserData struct {
	ID              string        `json:"id"`
	Username        string        `json:"username"`
	Name            string        `json:"name"`
	CreatedAt       string        `json:"created_at"`
	Description     string        `json:"description"`
	PublicMetrics   UserMetrics   `json:"public_metrics"`
	Verified        bool          `json:"verified"`
	Protected       bool          `json:"protected"`
	ProfileImageURL string        `json:"profile_image_url"`
	Location        string        `json:"location"`
	URL             string        `json:"url"`
}

// Supporting structures
type PublicMetrics struct {
	RetweetCount int `json:"retweet_count"`
	LikeCount    int `json:"like_count"`
	ReplyCount   int `json:"reply_count"`
	QuoteCount   int `json:"quote_count"`
}

type UserMetrics struct {
	FollowersCount int `json:"followers_count"`
	FollowingCount int `json:"following_count"`
	TweetCount     int `json:"tweet_count"`
	ListedCount    int `json:"listed_count"`
}

type GeoData struct {
	PlaceID     string      `json:"place_id"`
	Coordinates Coordinates `json:"coordinates"`
}

type Coordinates struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type Entities struct {
	URLs     []URLEntity     `json:"urls"`
	Hashtags []HashtagEntity `json:"hashtags"`
	Mentions []MentionEntity `json:"mentions"`
}

type URLEntity struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	URL         string `json:"url"`
	ExpandedURL string `json:"expanded_url"`
	DisplayURL  string `json:"display_url"`
}

type HashtagEntity struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Tag   string `json:"tag"`
}

type MentionEntity struct {
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Username string `json:"username"`
	ID       string `json:"id"`
}

type ReferencedTweet struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Includes struct {
	Users []UserData `json:"users"`
}

type Meta struct {
	ResultCount int    `json:"result_count"`
	NextToken   string `json:"next_token"`
	NewestID    string `json:"newest_id"`
	OldestID    string `json:"oldest_id"`
}

type APIError struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// StreamRule represents a Twitter streaming rule
type StreamRule struct {
	Value string `json:"value"`
	Tag   string `json:"tag"`
	ID    string `json:"id,omitempty"`
}

type StreamRulesRequest struct {
	Add    []StreamRule `json:"add,omitempty"`
	Delete DeleteRules  `json:"delete,omitempty"`
}

type DeleteRules struct {
	IDs []string `json:"ids,omitempty"`
}

// NewTwitterClient creates a new Twitter API client
func NewTwitterClient(config TwitterAPIConfig) *TwitterClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.twitter.com"
	}
	if config.StreamURL == "" {
		config.StreamURL = "https://api.twitter.com"
	}
	if config.RateLimit == 0 {
		config.RateLimit = 300 // Default: 300 requests per 15 minutes
	}

	return &TwitterClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(config.RateLimit, 15*time.Minute),
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(chan struct{}, requests),
		window:   window,
	}

	// Fill the initial bucket
	for i := 0; i < requests; i++ {
		rl.requests <- struct{}{}
	}

	// Refill the bucket periodically
	rl.ticker = time.NewTicker(window / time.Duration(requests))
	go func() {
		for range rl.ticker.C {
			select {
			case rl.requests <- struct{}{}:
			default:
				// Bucket is full
			}
		}
	}()

	return rl
}

// Wait blocks until a request slot is available
func (rl *RateLimiter) Wait() {
	<-rl.requests
}

// makeRequest makes an authenticated request to Twitter API
func (tc *TwitterClient) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	tc.rateLimiter.Wait()

	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}

	// Add authentication
	req.Header.Set("Authorization", "Bearer "+tc.config.BearerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "X-Spam-Detector/1.0")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// SearchTweets searches for tweets using the search endpoint
func (tc *TwitterClient) SearchTweets(query string, maxResults int) ([]*models.Tweet, error) {
	endpoint := fmt.Sprintf("%s/2/tweets/search/recent", tc.config.BaseURL)
	
	params := url.Values{}
	params.Set("query", query)
	params.Set("max_results", fmt.Sprintf("%d", maxResults))
	params.Set("tweet.fields", "created_at,author_id,public_metrics,lang,geo,entities,referenced_tweets")
	params.Set("user.fields", "created_at,description,public_metrics,verified,protected,location,url")
	params.Set("expansions", "author_id")

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	resp, err := tc.makeRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResponse TwitterAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API errors
	if len(apiResponse.Errors) > 0 {
		return nil, fmt.Errorf("API returned errors: %v", apiResponse.Errors)
	}

	// Convert to internal format
	tweets := tc.convertToInternalFormat(apiResponse)
	return tweets, nil
}

// StartStream starts a filtered stream for real-time tweets
func (tc *TwitterClient) StartStream(ctx context.Context, rules []StreamRule, tweetChan chan<- *models.Tweet) error {
	// First, set up the stream rules
	if err := tc.setupStreamRules(rules); err != nil {
		return fmt.Errorf("failed to setup stream rules: %w", err)
	}

	// Start the stream
	endpoint := fmt.Sprintf("%s/2/tweets/search/stream", tc.config.StreamURL)
	params := url.Values{}
	params.Set("tweet.fields", "created_at,author_id,public_metrics,lang,geo,entities,referenced_tweets")
	params.Set("user.fields", "created_at,description,public_metrics,verified,protected,location,url")
	params.Set("expansions", "author_id")

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+tc.config.BearerToken)
	req.Header.Set("User-Agent", "X-Spam-Detector/1.0")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read the stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue // Skip empty lines (heartbeats)
		}

		var streamData TwitterAPIResponse
		if err := json.Unmarshal([]byte(line), &streamData); err != nil {
			continue // Skip malformed data
		}

		// Convert and send tweets
		tweets := tc.convertToInternalFormat(streamData)
		for _, tweet := range tweets {
			select {
			case tweetChan <- tweet:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return scanner.Err()
}

// setupStreamRules configures the streaming rules
func (tc *TwitterClient) setupStreamRules(rules []StreamRule) error {
	// First, get existing rules to delete them
	existingRules, err := tc.getStreamRules()
	if err != nil {
		return fmt.Errorf("failed to get existing rules: %w", err)
	}

	// Delete existing rules
	if len(existingRules) > 0 {
		if err := tc.deleteStreamRules(existingRules); err != nil {
			return fmt.Errorf("failed to delete existing rules: %w", err)
		}
	}

	// Add new rules
	if len(rules) > 0 {
		if err := tc.addStreamRules(rules); err != nil {
			return fmt.Errorf("failed to add new rules: %w", err)
		}
	}

	return nil
}

// getStreamRules retrieves current streaming rules
func (tc *TwitterClient) getStreamRules() ([]StreamRule, error) {
	endpoint := fmt.Sprintf("%s/2/tweets/search/stream/rules", tc.config.BaseURL)
	
	resp, err := tc.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Data []StreamRule `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// addStreamRules adds new streaming rules
func (tc *TwitterClient) addStreamRules(rules []StreamRule) error {
	endpoint := fmt.Sprintf("%s/2/tweets/search/stream/rules", tc.config.BaseURL)
	
	request := StreamRulesRequest{Add: rules}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	resp, err := tc.makeRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// deleteStreamRules removes streaming rules
func (tc *TwitterClient) deleteStreamRules(rules []StreamRule) error {
	endpoint := fmt.Sprintf("%s/2/tweets/search/stream/rules", tc.config.BaseURL)
	
	var ids []string
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}

	request := StreamRulesRequest{
		Delete: DeleteRules{IDs: ids},
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	resp, err := tc.makeRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// convertToInternalFormat converts Twitter API response to internal models
func (tc *TwitterClient) convertToInternalFormat(apiResponse TwitterAPIResponse) []*models.Tweet {
	var tweets []*models.Tweet

	// Create user lookup map
	userMap := make(map[string]UserData)
	for _, user := range apiResponse.Includes.Users {
		userMap[user.ID] = user
	}

	for _, tweetData := range apiResponse.Data {
		// Convert user data
		userData, exists := userMap[tweetData.AuthorID]
		if !exists {
			continue // Skip tweets without user data
		}

		account := tc.convertUserData(userData)
		
		// Convert tweet data
		tweet := tc.convertTweetData(tweetData, account)
		tweets = append(tweets, tweet)
	}

	return tweets
}

// convertUserData converts API user data to internal account model
func (tc *TwitterClient) convertUserData(userData UserData) *models.Account {
	account := &models.Account{
		ID:          userData.ID,
		Username:    userData.Username,
		DisplayName: userData.Name,
		Bio:         userData.Description,
		Location:    userData.Location,
		Website:     userData.URL,
		IsVerified:  userData.Verified,
		IsPrivate:   userData.Protected,
		ProfileImageURL: userData.ProfileImageURL,
		
		FollowersCount: userData.PublicMetrics.FollowersCount,
		FollowingCount: userData.PublicMetrics.FollowingCount,
		TweetCount:     userData.PublicMetrics.TweetCount,
	}

	// Parse creation date
	if createdAt, err := time.Parse(time.RFC3339, userData.CreatedAt); err == nil {
		account.CreatedAt = createdAt
		account.AccountAge = int(time.Since(createdAt).Hours() / 24)
	}

	// Calculate tweets per day
	if account.AccountAge > 0 {
		account.TweetsPerDay = float64(account.TweetCount) / float64(account.AccountAge)
	}

	// Calculate follower ratio
	if account.FollowingCount > 0 {
		account.FollowerToFollowing = float64(account.FollowersCount) / float64(account.FollowingCount)
	}

	// Calculate bot score
	account.CalculateBotScore()

	return account
}

// convertTweetData converts API tweet data to internal tweet model
func (tc *TwitterClient) convertTweetData(tweetData TweetData, account *models.Account) *models.Tweet {
	tweet := &models.Tweet{
		ID:       tweetData.ID,
		Text:     tweetData.Text,
		AuthorID: tweetData.AuthorID,
		Author:   *account,
		
		RetweetCount: tweetData.PublicMetrics.RetweetCount,
		LikeCount:    tweetData.PublicMetrics.LikeCount,
		ReplyCount:   tweetData.PublicMetrics.ReplyCount,
	}

	// Parse creation date
	if createdAt, err := time.Parse(time.RFC3339, tweetData.CreatedAt); err == nil {
		tweet.CreatedAt = createdAt
	}

	// Extract entities
	if tweetData.Entities != nil {
		for _, url := range tweetData.Entities.URLs {
			tweet.URLs = append(tweet.URLs, url.ExpandedURL)
		}
		
		for _, hashtag := range tweetData.Entities.Hashtags {
			tweet.Hashtags = append(tweet.Hashtags, hashtag.Tag)
		}
		
		for _, mention := range tweetData.Entities.Mentions {
			tweet.Mentions = append(tweet.Mentions, mention.Username)
		}
	}

	return tweet
}

// BuildSearchQuery builds a search query from keywords and hashtags
func BuildSearchQuery(keywords []string, hashtags []string, languages []string) string {
	var parts []string

	// Add keywords
	for _, keyword := range keywords {
		if strings.Contains(keyword, " ") {
			parts = append(parts, fmt.Sprintf("\"%s\"", keyword))
		} else {
			parts = append(parts, keyword)
		}
	}

	// Add hashtags
	for _, hashtag := range hashtags {
		parts = append(parts, "#"+hashtag)
	}

	query := strings.Join(parts, " OR ")

	// Add language filter
	if len(languages) > 0 {
		langParts := make([]string, len(languages))
		for i, lang := range languages {
			langParts[i] = "lang:" + lang
		}
		query = fmt.Sprintf("(%s) (%s)", query, strings.Join(langParts, " OR "))
	}

	// Add filters to exclude retweets and replies for cleaner data
	query += " -is:retweet -is:reply"

	return query
}
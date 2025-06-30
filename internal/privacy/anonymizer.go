package privacy

import (
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// DataAnonymizer provides comprehensive data anonymization capabilities
type DataAnonymizer struct {
	config    AnonymizationConfig
	salts     map[string][]byte
	mutex     sync.RWMutex
	
	// Pattern preserving anonymization
	patternPreserver *PatternPreserver
	
	// K-anonymity support
	kAnonymizer *KAnonymityProcessor
	
	// Differential privacy
	dpProcessor *DifferentialPrivacyProcessor
}

// AnonymizationConfig defines anonymization settings
type AnonymizationConfig struct {
	// General settings
	Enabled         bool              `json:"enabled"`
	DefaultMethod   AnonymizationMethod `json:"default_method"`
	SaltRotation    time.Duration     `json:"salt_rotation"`
	
	// Field-specific settings
	FieldConfigs    map[string]FieldConfig `json:"field_configs"`
	
	// K-anonymity settings
	KValue          int               `json:"k_value"`
	LDiversity      int               `json:"l_diversity"`
	
	// Differential privacy settings
	Epsilon         float64           `json:"epsilon"`
	Delta           float64           `json:"delta"`
	
	// Data retention
	RetentionPeriod time.Duration     `json:"retention_period"`
	
	// Audit settings
	AuditEnabled    bool              `json:"audit_enabled"`
	AuditLog        string            `json:"audit_log"`
}

// AnonymizationMethod defines different anonymization techniques
type AnonymizationMethod int

const (
	Hash AnonymizationMethod = iota
	Encrypt
	Pseudonymize
	Suppress
	Generalize
	AddNoise
	KAnonymity
	DifferentialPrivacy
)

// FieldConfig defines how specific fields should be anonymized
type FieldConfig struct {
	Method          AnonymizationMethod `json:"method"`
	PreservePattern bool               `json:"preserve_pattern"`
	PreserveLength  bool               `json:"preserve_length"`
	NoiseLevel      float64            `json:"noise_level"`
	Generalization  GeneralizationRule `json:"generalization"`
	Enabled         bool               `json:"enabled"`
}

// GeneralizationRule defines how to generalize specific field values
type GeneralizationRule struct {
	Type     GeneralizationType `json:"type"`
	Buckets  []string          `json:"buckets"`
	Ranges   []NumericRange    `json:"ranges"`
	DateFormat string          `json:"date_format"`
}

// GeneralizationType defines types of generalization
type GeneralizationType int

const (
	Categorical GeneralizationType = iota
	Numeric
	Temporal
	Geographic
)

// NumericRange defines a range for numeric generalization
type NumericRange struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Label string  `json:"label"`
}

// AnonymizedTweet represents an anonymized version of a tweet
type AnonymizedTweet struct {
	ID              string                 `json:"id"`
	Text            string                 `json:"text"`
	AuthorID        string                 `json:"author_id"`
	CreatedAt       time.Time              `json:"created_at"`
	EngagementTier  string                 `json:"engagement_tier"`
	ContentCategory string                 `json:"content_category"`
	Language        string                 `json:"language"`
	SentimentScore  float64                `json:"sentiment_score"`
	IsSpam          bool                   `json:"is_spam"`
	SpamConfidence  string                 `json:"spam_confidence"`
	ClusterID       string                 `json:"cluster_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// AnonymizedAccount represents an anonymized version of an account
type AnonymizedAccount struct {
	ID              string                 `json:"id"`
	AccountAgeTier  string                 `json:"account_age_tier"`
	FollowerTier    string                 `json:"follower_tier"`
	ActivityLevel   string                 `json:"activity_level"`
	ProfileComplete bool                   `json:"profile_complete"`
	IsVerified      bool                   `json:"is_verified"`
	BotScoreTier    string                 `json:"bot_score_tier"`
	Location        string                 `json:"location,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// NewDataAnonymizer creates a new data anonymizer
func NewDataAnonymizer(config AnonymizationConfig) (*DataAnonymizer, error) {
	da := &DataAnonymizer{
		config:           config,
		salts:            make(map[string][]byte),
		patternPreserver: NewPatternPreserver(),
		kAnonymizer:      NewKAnonymityProcessor(config.KValue, config.LDiversity),
		dpProcessor:      NewDifferentialPrivacyProcessor(config.Epsilon, config.Delta),
	}
	
	// Initialize salts for different field types
	if err := da.initializeSalts(); err != nil {
		return nil, fmt.Errorf("failed to initialize salts: %w", err)
	}
	
	return da, nil
}

// AnonymizeTweet anonymizes a tweet while preserving analytical value
func (da *DataAnonymizer) AnonymizeTweet(tweet *models.Tweet) (*AnonymizedTweet, error) {
	if !da.config.Enabled {
		return da.convertTweetDirect(tweet), nil
	}
	
	da.mutex.RLock()
	defer da.mutex.RUnlock()
	
	anonymized := &AnonymizedTweet{
		ID:        da.anonymizeField("tweet_id", tweet.ID),
		AuthorID:  da.anonymizeField("author_id", tweet.AuthorID),
		CreatedAt: da.anonymizeTimestamp(tweet.CreatedAt),
		Language:  tweet.Author.Location, // Assume language detection
		IsSpam:    tweet.IsSpam,
		Metadata:  make(map[string]interface{}),
	}
	
	// Anonymize text content while preserving patterns
	var err error
	anonymized.Text, err = da.anonymizeText(tweet.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to anonymize text: %w", err)
	}
	
	// Generalize engagement metrics
	anonymized.EngagementTier = da.generalizeEngagement(tweet.RetweetCount + tweet.LikeCount + tweet.ReplyCount)
	
	// Categorize content
	anonymized.ContentCategory = da.categorizeContent(tweet)
	
	// Add noise to sentiment if available
	anonymized.SentimentScore = da.addNoise(0.5, 0.1) // Placeholder sentiment
	
	// Generalize spam confidence
	anonymized.SpamConfidence = da.generalizeSpamScore(tweet.SpamScore)
	
	// Anonymize cluster ID if present
	if tweet.ClusterID != "" {
		anonymized.ClusterID = da.anonymizeField("cluster_id", tweet.ClusterID)
	}
	
	// Add anonymization metadata
	anonymized.Metadata["anonymized_at"] = time.Now()
	anonymized.Metadata["method"] = da.config.DefaultMethod
	
	return anonymized, nil
}

// AnonymizeAccount anonymizes an account while preserving analytical value
func (da *DataAnonymizer) AnonymizeAccount(account *models.Account) (*AnonymizedAccount, error) {
	if !da.config.Enabled {
		return da.convertAccountDirect(account), nil
	}
	
	da.mutex.RLock()
	defer da.mutex.RUnlock()
	
	anonymized := &AnonymizedAccount{
		ID:              da.anonymizeField("account_id", account.ID),
		IsVerified:      account.IsVerified,
		ProfileComplete: da.isProfileComplete(account),
		CreatedAt:       da.anonymizeTimestamp(account.CreatedAt),
		Metadata:        make(map[string]interface{}),
	}
	
	// Generalize account metrics
	anonymized.AccountAgeTier = da.generalizeAccountAge(account.AccountAge)
	anonymized.FollowerTier = da.generalizeFollowerCount(account.FollowersCount)
	anonymized.ActivityLevel = da.generalizeActivityLevel(account.TweetsPerDay)
	anonymized.BotScoreTier = da.generalizeBotScore(account.BotScore)
	
	// Generalize location if present
	if account.Location != "" {
		anonymized.Location = da.generalizeLocation(account.Location)
	}
	
	// Add anonymization metadata
	anonymized.Metadata["anonymized_at"] = time.Now()
	anonymized.Metadata["method"] = da.config.DefaultMethod
	
	return anonymized, nil
}

// AnonymizeCluster anonymizes a spam cluster
func (da *DataAnonymizer) AnonymizeCluster(cluster *models.SpamCluster) (*models.SpamCluster, error) {
	if !da.config.Enabled {
		return cluster, nil
	}
	
	anonymized := &models.SpamCluster{
		ID:              da.anonymizeField("cluster_id", cluster.ID),
		CreatedAt:       da.anonymizeTimestamp(cluster.CreatedAt),
		UpdatedAt:       da.anonymizeTimestamp(cluster.UpdatedAt),
		Size:            cluster.Size,
		Confidence:      da.addNoise(cluster.Confidence, 0.05),
		Pattern:         da.anonymizePattern(cluster.Pattern),
		Template:        func() string { t, _ := da.anonymizeText(cluster.Template); return t }(),
		DetectionMethod: cluster.DetectionMethod,
		LSHThreshold:    cluster.LSHThreshold,
		SuspicionLevel:  cluster.SuspicionLevel,
		IsCoordinated:   cluster.IsCoordinated,
	}
	
	// Anonymize temporal information with reduced precision
	anonymized.FirstSeen = da.anonymizeTimestamp(cluster.FirstSeen)
	anonymized.LastSeen = da.anonymizeTimestamp(cluster.LastSeen)
	anonymized.Duration = cluster.Duration
	
	// Add noise to metrics
	anonymized.TweetsPerMinute = da.addNoise(cluster.TweetsPerMinute, 0.1)
	anonymized.UniqueAccounts = cluster.UniqueAccounts
	
	// Anonymize IDs
	anonymized.TweetIDs = da.anonymizeIDList(cluster.TweetIDs, "tweet_id")
	anonymized.AccountIDs = da.anonymizeIDList(cluster.AccountIDs, "account_id")
	
	// Note: Tweets and Accounts arrays are omitted for privacy
	
	return anonymized, nil
}

// anonymizeField anonymizes a field based on its configuration
func (da *DataAnonymizer) anonymizeField(fieldName, value string) string {
	config, exists := da.config.FieldConfigs[fieldName]
	if !exists {
		config = FieldConfig{Method: da.config.DefaultMethod, Enabled: true}
	}
	
	if !config.Enabled {
		return value
	}
	
	switch config.Method {
	case Hash:
		return da.hashValue(fieldName, value)
	case Pseudonymize:
		return da.pseudonymize(fieldName, value, config.PreservePattern)
	case Suppress:
		return "[REDACTED]"
	default:
		return da.hashValue(fieldName, value)
	}
}

// anonymizeText anonymizes text content while preserving analytical patterns
func (da *DataAnonymizer) anonymizeText(text string) (string, error) {
	if text == "" {
		return "", nil
	}
	
	// Remove or anonymize personal identifiers
	anonymized := da.removePersonalIdentifiers(text)
	
	// Preserve structure for analysis
	anonymized = da.patternPreserver.PreserveStructure(anonymized)
	
	return anonymized, nil
}

// removePersonalIdentifiers removes or anonymizes personal identifiers from text
func (da *DataAnonymizer) removePersonalIdentifiers(text string) string {
	// Remove email addresses
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	text = emailRegex.ReplaceAllString(text, "[EMAIL]")
	
	// Remove phone numbers
	phoneRegex := regexp.MustCompile(`(\+?1[-.\s]?)?(\()?[0-9]{3}(\))?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`)
	text = phoneRegex.ReplaceAllString(text, "[PHONE]")
	
	// Remove URLs but preserve the fact that a URL was present
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	text = urlRegex.ReplaceAllString(text, "[URL]")
	
	// Remove @mentions but preserve pattern
	mentionRegex := regexp.MustCompile(`@[a-zA-Z0-9_]+`)
	mentions := mentionRegex.FindAllString(text, -1)
	for i, mention := range mentions {
		replacement := fmt.Sprintf("@USER%d", i+1)
		text = strings.Replace(text, mention, replacement, 1)
	}
	
	// Remove #hashtags but preserve pattern
	hashtagRegex := regexp.MustCompile(`#[a-zA-Z0-9_]+`)
	hashtags := hashtagRegex.FindAllString(text, -1)
	for i, hashtag := range hashtags {
		replacement := fmt.Sprintf("#TAG%d", i+1)
		text = strings.Replace(text, hashtag, replacement, 1)
	}
	
	return text
}

// hashValue creates a consistent hash of a value with salt
func (da *DataAnonymizer) hashValue(fieldType, value string) string {
	salt := da.getSalt(fieldType)
	
	h := hmac.New(sha256.New, salt)
	h.Write([]byte(value))
	
	// Return first 16 characters of hex hash for readability
	hash := hex.EncodeToString(h.Sum(nil))
	return hash[:16]
}

// pseudonymize creates a pseudonym that preserves patterns
func (da *DataAnonymizer) pseudonymize(fieldType, value string, preservePattern bool) string {
	if !preservePattern {
		return da.hashValue(fieldType, value)
	}
	
	// Create a deterministic pseudonym based on hash
	hash := da.hashValue(fieldType, value)
	
	// Convert hash to a readable pseudonym
	h := fnv.New32a()
	h.Write([]byte(hash))
	pseudoID := h.Sum32()
	
	switch fieldType {
	case "author_id", "account_id":
		return fmt.Sprintf("USER_%08X", pseudoID)
	case "tweet_id":
		return fmt.Sprintf("TWEET_%08X", pseudoID)
	case "cluster_id":
		return fmt.Sprintf("CLUSTER_%08X", pseudoID)
	default:
		return fmt.Sprintf("ID_%08X", pseudoID)
	}
}

// anonymizeTimestamp reduces timestamp precision for privacy
func (da *DataAnonymizer) anonymizeTimestamp(t time.Time) time.Time {
	// Round to nearest hour for privacy
	return t.Truncate(time.Hour)
}

// Generalization functions

func (da *DataAnonymizer) generalizeEngagement(total int) string {
	switch {
	case total == 0:
		return "none"
	case total < 10:
		return "low"
	case total < 100:
		return "medium"
	case total < 1000:
		return "high"
	default:
		return "viral"
	}
}

func (da *DataAnonymizer) generalizeAccountAge(days int) string {
	switch {
	case days < 30:
		return "new"
	case days < 365:
		return "recent"
	case days < 365*3:
		return "established"
	default:
		return "veteran"
	}
}

func (da *DataAnonymizer) generalizeFollowerCount(count int) string {
	switch {
	case count == 0:
		return "none"
	case count < 100:
		return "small"
	case count < 1000:
		return "medium"
	case count < 10000:
		return "large"
	default:
		return "massive"
	}
}

func (da *DataAnonymizer) generalizeActivityLevel(tweetsPerDay float64) string {
	switch {
	case tweetsPerDay == 0:
		return "inactive"
	case tweetsPerDay < 1:
		return "low"
	case tweetsPerDay < 10:
		return "moderate"
	case tweetsPerDay < 50:
		return "high"
	default:
		return "extreme"
	}
}

func (da *DataAnonymizer) generalizeBotScore(score float64) string {
	switch {
	case score < 0.2:
		return "human"
	case score < 0.5:
		return "uncertain"
	case score < 0.8:
		return "likely_bot"
	default:
		return "definite_bot"
	}
}

func (da *DataAnonymizer) generalizeSpamScore(score float64) string {
	switch {
	case score < 0.3:
		return "clean"
	case score < 0.6:
		return "suspicious"
	case score < 0.8:
		return "likely_spam"
	default:
		return "definite_spam"
	}
}

func (da *DataAnonymizer) generalizeLocation(location string) string {
	// Simple location generalization
	location = strings.ToLower(location)
	
	// Map to broader regions
	if strings.Contains(location, "new york") || strings.Contains(location, "ny") {
		return "northeast_us"
	} else if strings.Contains(location, "california") || strings.Contains(location, "ca") {
		return "west_us"
	} else if strings.Contains(location, "texas") || strings.Contains(location, "tx") {
		return "south_us"
	} else if strings.Contains(location, "london") || strings.Contains(location, "uk") {
		return "uk"
	} else if strings.Contains(location, "europe") {
		return "europe"
	} else if strings.Contains(location, "asia") {
		return "asia"
	}
	
	return "other"
}

func (da *DataAnonymizer) categorizeContent(tweet *models.Tweet) string {
	text := strings.ToLower(tweet.Text)
	
	// Simple content categorization
	if len(tweet.URLs) > 0 {
		return "contains_links"
	} else if len(tweet.Mentions) > 3 {
		return "social"
	} else if len(tweet.Hashtags) > 2 {
		return "promotional"
	} else if strings.Contains(text, "buy") || strings.Contains(text, "sell") {
		return "commercial"
	} else if strings.Contains(text, "follow") || strings.Contains(text, "like") {
		return "engagement"
	}
	
	return "general"
}

func (da *DataAnonymizer) anonymizePattern(pattern string) string {
	// Remove specific details but preserve pattern type
	if strings.Contains(pattern, "URL") {
		return "contains_urls"
	} else if strings.Contains(pattern, "mention") {
		return "contains_mentions"
	} else if strings.Contains(pattern, "hashtag") {
		return "contains_hashtags"
	}
	
	return "text_similarity"
}

func (da *DataAnonymizer) anonymizeIDList(ids []string, fieldType string) []string {
	anonymized := make([]string, len(ids))
	for i, id := range ids {
		anonymized[i] = da.anonymizeField(fieldType, id)
	}
	return anonymized
}

// Utility functions

func (da *DataAnonymizer) addNoise(value, noiseLevel float64) float64 {
	// Add Gaussian noise for differential privacy
	return da.dpProcessor.AddNoise(value, noiseLevel)
}

func (da *DataAnonymizer) getSalt(fieldType string) []byte {
	salt, exists := da.salts[fieldType]
	if !exists {
		// Generate new salt
		salt = make([]byte, 32)
		cryptorand.Read(salt)
		da.salts[fieldType] = salt
	}
	return salt
}

func (da *DataAnonymizer) initializeSalts() error {
	fieldTypes := []string{
		"tweet_id", "author_id", "account_id", "cluster_id",
		"username", "email", "ip_address", "user_agent",
	}
	
	for _, fieldType := range fieldTypes {
		salt := make([]byte, 32)
		if _, err := cryptorand.Read(salt); err != nil {
			return fmt.Errorf("failed to generate salt for %s: %w", fieldType, err)
		}
		da.salts[fieldType] = salt
	}
	
	return nil
}

func (da *DataAnonymizer) isProfileComplete(account *models.Account) bool {
	completeness := 0
	if account.Bio != "" {
		completeness++
	}
	if account.Location != "" {
		completeness++
	}
	if account.Website != "" {
		completeness++
	}
	if account.ProfileImageURL != "" {
		completeness++
	}
	
	return completeness >= 2 // At least 50% complete
}

func (da *DataAnonymizer) convertTweetDirect(tweet *models.Tweet) *AnonymizedTweet {
	return &AnonymizedTweet{
		ID:              tweet.ID,
		Text:            tweet.Text,
		AuthorID:        tweet.AuthorID,
		CreatedAt:       tweet.CreatedAt,
		IsSpam:          tweet.IsSpam,
		SpamConfidence:  fmt.Sprintf("%.2f", tweet.SpamScore),
		ClusterID:       tweet.ClusterID,
		Metadata: map[string]interface{}{
			"anonymized_at": time.Now(),
			"method":        "none",
		},
	}
}

func (da *DataAnonymizer) convertAccountDirect(account *models.Account) *AnonymizedAccount {
	return &AnonymizedAccount{
		ID:              account.ID,
		IsVerified:      account.IsVerified,
		ProfileComplete: da.isProfileComplete(account),
		CreatedAt:       account.CreatedAt,
		Location:        account.Location,
		Metadata: map[string]interface{}{
			"anonymized_at": time.Now(),
			"method":        "none",
		},
	}
}

// PatternPreserver preserves text patterns for analysis
type PatternPreserver struct {
	patterns map[string]string
	mutex    sync.RWMutex
}

func NewPatternPreserver() *PatternPreserver {
	return &PatternPreserver{
		patterns: make(map[string]string),
	}
}

func (pp *PatternPreserver) PreserveStructure(text string) string {
	// Preserve word length patterns
	words := strings.Fields(text)
	preserved := make([]string, len(words))
	
	for i, word := range words {
		if strings.HasPrefix(word, "[") && strings.HasSuffix(word, "]") {
			// Keep anonymization markers
			preserved[i] = word
		} else {
			// Replace with pattern-preserving placeholder
			preserved[i] = pp.createPlaceholder(word)
		}
	}
	
	return strings.Join(preserved, " ")
}

func (pp *PatternPreserver) createPlaceholder(word string) string {
	// Create placeholder that preserves length and basic pattern
	length := len(word)
	if length == 0 {
		return ""
	}
	
	placeholder := make([]rune, length)
	for i := range placeholder {
		if i == 0 && length > 1 {
			placeholder[i] = 'W' // Word start
		} else {
			placeholder[i] = 'w' // Word continuation
		}
	}
	
	return string(placeholder)
}

// KAnonymityProcessor implements k-anonymity
type KAnonymityProcessor struct {
	k         int
	l         int // l-diversity
	groups    map[string][]interface{}
	mutex     sync.RWMutex
}

func NewKAnonymityProcessor(k, l int) *KAnonymityProcessor {
	return &KAnonymityProcessor{
		k:      k,
		l:      l,
		groups: make(map[string][]interface{}),
	}
}

func (kap *KAnonymityProcessor) EnsureKAnonymity(data []interface{}, quasiIdentifiers []string) error {
	// Implementation would ensure k-anonymity by grouping records
	// This is a placeholder for the complex k-anonymity algorithm
	return nil
}

// DifferentialPrivacyProcessor implements differential privacy
type DifferentialPrivacyProcessor struct {
	epsilon float64
	delta   float64
}

func NewDifferentialPrivacyProcessor(epsilon, delta float64) *DifferentialPrivacyProcessor {
	return &DifferentialPrivacyProcessor{
		epsilon: epsilon,
		delta:   delta,
	}
}

func (dpp *DifferentialPrivacyProcessor) AddNoise(value, sensitivity float64) float64 {
	// Add Laplacian noise for differential privacy
	// scale = sensitivity / epsilon
	scale := sensitivity / dpp.epsilon
	
	// Generate Laplacian noise
	// This is a simplified implementation
	noise := dpp.generateLaplacianNoise(scale)
	
	return value + noise
}

func (dpp *DifferentialPrivacyProcessor) generateLaplacianNoise(scale float64) float64 {
	// Simplified Laplacian noise generation
	// In production, use proper Laplacian distribution
	return (2.0*rand.Float64() - 1.0) * scale
}

// AnonymizationAudit tracks anonymization operations for compliance
type AnonymizationAudit struct {
	Timestamp    time.Time              `json:"timestamp"`
	Operation    string                 `json:"operation"`
	EntityType   string                 `json:"entity_type"`
	EntityID     string                 `json:"entity_id"`
	Method       AnonymizationMethod    `json:"method"`
	Fields       []string               `json:"fields"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// GetConfig returns the current anonymization configuration
func (da *DataAnonymizer) GetConfig() AnonymizationConfig {
	da.mutex.RLock()
	defer da.mutex.RUnlock()
	return da.config
}

// UpdateConfig updates the anonymization configuration
func (da *DataAnonymizer) UpdateConfig(config AnonymizationConfig) error {
	da.mutex.Lock()
	defer da.mutex.Unlock()
	
	da.config = config
	return nil
}

// GetStatistics returns anonymization statistics
func (da *DataAnonymizer) GetStatistics() AnonymizationStatistics {
	return AnonymizationStatistics{
		Enabled:           da.config.Enabled,
		TotalFields:       len(da.config.FieldConfigs),
		ActiveFields:      da.countActiveFields(),
		DefaultMethod:     da.config.DefaultMethod,
		KValue:           da.config.KValue,
		Epsilon:          da.config.Epsilon,
		LastSaltRotation: time.Now(), // Placeholder
	}
}

// AnonymizationStatistics represents anonymization statistics
type AnonymizationStatistics struct {
	Enabled           bool                `json:"enabled"`
	TotalFields       int                 `json:"total_fields"`
	ActiveFields      int                 `json:"active_fields"`
	DefaultMethod     AnonymizationMethod `json:"default_method"`
	KValue           int                 `json:"k_value"`
	Epsilon          float64             `json:"epsilon"`
	LastSaltRotation time.Time           `json:"last_salt_rotation"`
}

func (da *DataAnonymizer) countActiveFields() int {
	count := 0
	for _, config := range da.config.FieldConfigs {
		if config.Enabled {
			count++
		}
	}
	return count
}

// Default anonymization configuration
func DefaultAnonymizationConfig() AnonymizationConfig {
	return AnonymizationConfig{
		Enabled:         true,
		DefaultMethod:   Pseudonymize,
		SaltRotation:    24 * time.Hour,
		KValue:          5,
		LDiversity:      2,
		Epsilon:         1.0,
		Delta:           1e-5,
		RetentionPeriod: 30 * 24 * time.Hour,
		AuditEnabled:    true,
		FieldConfigs: map[string]FieldConfig{
			"tweet_id": {
				Method:          Pseudonymize,
				PreservePattern: true,
				Enabled:         true,
			},
			"author_id": {
				Method:          Pseudonymize,
				PreservePattern: true,
				Enabled:         true,
			},
			"account_id": {
				Method:          Pseudonymize,
				PreservePattern: true,
				Enabled:         true,
			},
			"cluster_id": {
				Method:          Pseudonymize,
				PreservePattern: true,
				Enabled:         true,
			},
			"text_content": {
				Method:          Generalize,
				PreservePattern: true,
				Enabled:         true,
			},
			"timestamp": {
				Method:   Generalize,
				Enabled:  true,
			},
		},
	}
}
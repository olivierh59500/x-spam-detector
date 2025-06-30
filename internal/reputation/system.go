package reputation

import (
	"fmt"
	"math"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// ReputationSystem manages reputation scoring for accounts and content
type ReputationSystem struct {
	scores      map[string]*ReputationScore
	decay       DecayFunction
	updateQueue chan ReputationUpdate
	mutex       sync.RWMutex
	
	// System configuration
	config ReputationConfig
	
	// Persistence and background processing
	persistence ReputationPersistence
	processor   *BackgroundProcessor
}

// ReputationScore represents a reputation score for an entity
type ReputationScore struct {
	EntityID    string                 `json:"entity_id"`
	EntityType  EntityType             `json:"entity_type"`
	Score       float64                `json:"score"`
	Confidence  float64                `json:"confidence"`
	LastUpdate  time.Time              `json:"last_update"`
	History     []ReputationEvent      `json:"history"`
	Factors     map[string]float64     `json:"factors"`
	Metadata    map[string]interface{} `json:"metadata"`
	
	// Derived metrics
	TrendDirection TrendDirection `json:"trend_direction"`
	Volatility     float64        `json:"volatility"`
	Reliability    float64        `json:"reliability"`
}

// ReputationEvent represents an event that affects reputation
type ReputationEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   EventType              `json:"event_type"`
	Impact      float64                `json:"impact"`
	Source      string                 `json:"source"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata"`
	DecayedBy   time.Time              `json:"decayed_by,omitempty"`
}

// EntityType defines types of entities that can have reputation
type EntityType int

const (
	Account EntityType = iota
	Content
	URL
	Hashtag
	IPAddress
	UserAgent
)

// EventType defines types of reputation events
type EventType int

const (
	SpamDetected EventType = iota
	LegitimateContent
	UserReported
	AutomaticFlag
	ManualReview
	AccountCreated
	FirstPost
	HighEngagement
	LowEngagement
	SuspiciousPattern
	CoordinatedBehavior
	VerificationPassed
	VerificationFailed
)

// TrendDirection indicates reputation trend
type TrendDirection int

const (
	Stable TrendDirection = iota
	Improving
	Deteriorating
	Volatile
)

// ReputationUpdate represents a reputation update request
type ReputationUpdate struct {
	EntityID   string
	EntityType EntityType
	Event      ReputationEvent
}

// ReputationConfig defines system configuration
type ReputationConfig struct {
	// Scoring parameters
	InitialScore        float64       `json:"initial_score"`
	MinScore           float64       `json:"min_score"`
	MaxScore           float64       `json:"max_score"`
	ConfidenceThreshold float64       `json:"confidence_threshold"`
	
	// Decay settings
	DecayRate          float64       `json:"decay_rate"`
	DecayInterval      time.Duration `json:"decay_interval"`
	MaxEventAge        time.Duration `json:"max_event_age"`
	
	// History settings
	MaxHistorySize     int           `json:"max_history_size"`
	
	// Update settings
	BatchSize          int           `json:"batch_size"`
	UpdateInterval     time.Duration `json:"update_interval"`
	
	// Thresholds
	SpamThreshold      float64       `json:"spam_threshold"`
	TrustedThreshold   float64       `json:"trusted_threshold"`
}

// DecayFunction defines how reputation scores decay over time
type DecayFunction interface {
	Apply(score float64, age time.Duration, config ReputationConfig) float64
}

// ReputationPersistence defines persistence interface
type ReputationPersistence interface {
	Save(score *ReputationScore) error
	Load(entityID string) (*ReputationScore, error)
	LoadBatch(entityIDs []string) (map[string]*ReputationScore, error)
	Delete(entityID string) error
}

// NewReputationSystem creates a new reputation system
func NewReputationSystem(config ReputationConfig, persistence ReputationPersistence) *ReputationSystem {
	rs := &ReputationSystem{
		scores:      make(map[string]*ReputationScore),
		decay:       &ExponentialDecay{},
		updateQueue: make(chan ReputationUpdate, 10000),
		config:      config,
		persistence: persistence,
	}
	
	rs.processor = NewBackgroundProcessor(rs)
	return rs
}

// Start starts the reputation system background processing
func (rs *ReputationSystem) Start() error {
	return rs.processor.Start()
}

// Stop stops the reputation system
func (rs *ReputationSystem) Stop() error {
	close(rs.updateQueue)
	return rs.processor.Stop()
}

// UpdateReputation updates reputation for an entity
func (rs *ReputationSystem) UpdateReputation(entityID string, entityType EntityType, event ReputationEvent) error {
	update := ReputationUpdate{
		EntityID:   entityID,
		EntityType: entityType,
		Event:      event,
	}
	
	select {
	case rs.updateQueue <- update:
		return nil
	default:
		return fmt.Errorf("update queue is full")
	}
}

// GetReputation gets reputation score for an entity
func (rs *ReputationSystem) GetReputation(entityID string) (*ReputationScore, error) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	score, exists := rs.scores[entityID]
	if !exists {
		// Try loading from persistence
		if rs.persistence != nil {
			if loaded, err := rs.persistence.Load(entityID); err == nil {
				rs.scores[entityID] = loaded
				return loaded, nil
			}
		}
		return nil, fmt.Errorf("reputation not found for entity %s", entityID)
	}
	
	// Apply decay before returning
	rs.applyDecay(score)
	return score, nil
}

// GetBatchReputation gets reputation scores for multiple entities
func (rs *ReputationSystem) GetBatchReputation(entityIDs []string) (map[string]*ReputationScore, error) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	results := make(map[string]*ReputationScore)
	missing := make([]string, 0)
	
	// Check in-memory cache first
	for _, entityID := range entityIDs {
		if score, exists := rs.scores[entityID]; exists {
			rs.applyDecay(score)
			results[entityID] = score
		} else {
			missing = append(missing, entityID)
		}
	}
	
	// Load missing from persistence
	if len(missing) > 0 && rs.persistence != nil {
		if loaded, err := rs.persistence.LoadBatch(missing); err == nil {
			for entityID, score := range loaded {
				rs.scores[entityID] = score
				rs.applyDecay(score)
				results[entityID] = score
			}
		}
	}
	
	return results, nil
}

// AnalyzeAccount analyzes an account and updates its reputation
func (rs *ReputationSystem) AnalyzeAccount(account *models.Account) error {
	// Calculate factors that influence reputation
	factors := rs.calculateAccountFactors(account)
	
	// Create reputation events based on factors
	events := rs.generateAccountEvents(account, factors)
	
	// Update reputation with events
	for _, event := range events {
		if err := rs.UpdateReputation(account.ID, Account, event); err != nil {
			return fmt.Errorf("failed to update account reputation: %w", err)
		}
	}
	
	return nil
}

// AnalyzeContent analyzes content and updates reputation
func (rs *ReputationSystem) AnalyzeContent(tweet *models.Tweet, cluster *models.SpamCluster) error {
	// Analyze tweet factors
	factors := rs.calculateContentFactors(tweet, cluster)
	
	// Generate events
	events := rs.generateContentEvents(tweet, cluster, factors)
	
	// Update content reputation
	for _, event := range events {
		if err := rs.UpdateReputation(tweet.ID, Content, event); err != nil {
			return fmt.Errorf("failed to update content reputation: %w", err)
		}
	}
	
	// Also update account reputation based on content
	if err := rs.updateAccountFromContent(tweet, cluster); err != nil {
		return fmt.Errorf("failed to update account reputation from content: %w", err)
	}
	
	return nil
}

// processUpdate processes a single reputation update
func (rs *ReputationSystem) processUpdate(update ReputationUpdate) error {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	score, exists := rs.scores[update.EntityID]
	if !exists {
		score = rs.createInitialScore(update.EntityID, update.EntityType)
		rs.scores[update.EntityID] = score
	}
	
	// Apply decay first
	rs.applyDecay(score)
	
	// Add event to history
	score.History = append(score.History, update.Event)
	
	// Trim history if too long
	if len(score.History) > rs.config.MaxHistorySize {
		score.History = score.History[len(score.History)-rs.config.MaxHistorySize:]
	}
	
	// Recalculate score
	rs.recalculateScore(score)
	
	// Update metadata
	score.LastUpdate = time.Now()
	
	// Persist if configured
	if rs.persistence != nil {
		if err := rs.persistence.Save(score); err != nil {
			return fmt.Errorf("failed to persist reputation score: %w", err)
		}
	}
	
	return nil
}

// createInitialScore creates an initial reputation score
func (rs *ReputationSystem) createInitialScore(entityID string, entityType EntityType) *ReputationScore {
	return &ReputationScore{
		EntityID:       entityID,
		EntityType:     entityType,
		Score:          rs.config.InitialScore,
		Confidence:     0.1, // Low initial confidence
		LastUpdate:     time.Now(),
		History:        make([]ReputationEvent, 0),
		Factors:        make(map[string]float64),
		Metadata:       make(map[string]interface{}),
		TrendDirection: Stable,
		Volatility:     0.0,
		Reliability:    0.5,
	}
}

// recalculateScore recalculates the reputation score based on history
func (rs *ReputationSystem) recalculateScore(score *ReputationScore) {
	if len(score.History) == 0 {
		return
	}
	
	// Calculate weighted average of recent events
	totalWeight := 0.0
	weightedSum := 0.0
	confidenceSum := 0.0
	
	now := time.Now()
	for _, event := range score.History {
		age := now.Sub(event.Timestamp)
		
		// Skip very old events
		if age > rs.config.MaxEventAge {
			continue
		}
		
		// Calculate time-based weight (newer events have more weight)
		timeWeight := math.Exp(-float64(age.Hours()) / 168) // Half-life of 1 week
		
		// Calculate event weight based on confidence
		eventWeight := timeWeight * event.Confidence
		
		weightedSum += event.Impact * eventWeight
		totalWeight += eventWeight
		confidenceSum += event.Confidence * timeWeight
	}
	
	if totalWeight > 0 {
		// Calculate new score
		impact := weightedSum / totalWeight
		newScore := score.Score + impact
		
		// Clamp to valid range
		newScore = math.Max(rs.config.MinScore, math.Min(rs.config.MaxScore, newScore))
		
		// Update score
		score.Score = newScore
		score.Confidence = math.Min(1.0, confidenceSum/totalWeight)
	}
	
	// Calculate trend and volatility
	rs.calculateTrendMetrics(score)
}

// calculateTrendMetrics calculates trend direction and volatility
func (rs *ReputationSystem) calculateTrendMetrics(score *ReputationScore) {
	if len(score.History) < 3 {
		score.TrendDirection = Stable
		score.Volatility = 0.0
		return
	}
	
	// Calculate recent trends
	recentEvents := score.History
	if len(recentEvents) > 10 {
		recentEvents = recentEvents[len(recentEvents)-10:] // Last 10 events
	}
	
	// Calculate slope of trend line
	n := len(recentEvents)
	if n < 2 {
		return
	}
	
	var sumX, sumY, sumXY, sumX2 float64
	for i, event := range recentEvents {
		x := float64(i)
		y := event.Impact
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Linear regression slope
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	
	// Determine trend direction
	if math.Abs(slope) < 0.01 {
		score.TrendDirection = Stable
	} else if slope > 0 {
		score.TrendDirection = Improving
	} else {
		score.TrendDirection = Deteriorating
	}
	
	// Calculate volatility (standard deviation of impacts)
	var variance float64
	mean := sumY / float64(n)
	for _, event := range recentEvents {
		diff := event.Impact - mean
		variance += diff * diff
	}
	variance /= float64(n)
	score.Volatility = math.Sqrt(variance)
	
	// High volatility indicates unstable behavior
	if score.Volatility > 0.5 {
		score.TrendDirection = Volatile
	}
	
	// Calculate reliability based on consistency
	score.Reliability = 1.0 - math.Min(1.0, score.Volatility)
}

// applyDecay applies time-based decay to a reputation score
func (rs *ReputationSystem) applyDecay(score *ReputationScore) {
	age := time.Since(score.LastUpdate)
	if age < rs.config.DecayInterval {
		return
	}
	
	// Apply decay to main score
	oldScore := score.Score
	score.Score = rs.decay.Apply(score.Score, age, rs.config)
	
	// Apply decay to event impacts in history
	cutoff := time.Now().Add(-rs.config.MaxEventAge)
	validEvents := make([]ReputationEvent, 0)
	
	for _, event := range score.History {
		if event.Timestamp.After(cutoff) {
			validEvents = append(validEvents, event)
		}
	}
	
	score.History = validEvents
	
	// Update timestamp if score changed
	if oldScore != score.Score {
		score.LastUpdate = time.Now()
	}
}

// calculateAccountFactors calculates reputation factors for an account
func (rs *ReputationSystem) calculateAccountFactors(account *models.Account) map[string]float64 {
	factors := make(map[string]float64)
	
	// Account age factor
	if account.AccountAge > 0 {
		// Older accounts are generally more trustworthy
		ageFactor := math.Min(1.0, float64(account.AccountAge)/365.0) // Max boost at 1 year
		factors["account_age"] = ageFactor * 0.2
	}
	
	// Follower ratio factor
	if account.FollowingCount > 0 {
		ratio := float64(account.FollowersCount) / float64(account.FollowingCount)
		if ratio > 10.0 {
			factors["follower_ratio"] = 0.1 // High ratio is good
		} else if ratio < 0.1 {
			factors["follower_ratio"] = -0.2 // Very low ratio is suspicious
		}
	}
	
	// Profile completeness factor
	completeness := 0.0
	if account.Bio != "" {
		completeness += 0.25
	}
	if account.Location != "" {
		completeness += 0.25
	}
	if account.Website != "" {
		completeness += 0.25
	}
	if account.ProfileImageURL != "" {
		completeness += 0.25
	}
	factors["profile_completeness"] = (completeness - 0.5) * 0.1 // Penalty for incomplete profiles
	
	// Verification factor
	if account.IsVerified {
		factors["verification"] = 0.3
	}
	
	// Bot score factor (negative impact)
	factors["bot_score"] = -account.BotScore * 0.4
	
	// Spam activity factor
	if account.SpamTweetCount > 0 {
		spamRatio := float64(account.SpamTweetCount) / float64(account.TweetCount)
		factors["spam_activity"] = -spamRatio * 0.5
	}
	
	return factors
}

// calculateContentFactors calculates reputation factors for content
func (rs *ReputationSystem) calculateContentFactors(tweet *models.Tweet, cluster *models.SpamCluster) map[string]float64 {
	factors := make(map[string]float64)
	
	// Spam detection factor
	if tweet.IsSpam {
		factors["spam_detected"] = -tweet.SpamScore
	} else {
		factors["legitimate_content"] = 0.1
	}
	
	// Engagement factor
	totalEngagement := tweet.RetweetCount + tweet.LikeCount + tweet.ReplyCount
	if totalEngagement > 0 {
		// High engagement generally indicates legitimate content
		engagementScore := math.Min(1.0, float64(totalEngagement)/100.0) * 0.2
		factors["engagement"] = engagementScore
	}
	
	// Cluster analysis factor
	if cluster != nil {
		if cluster.IsCoordinated {
			factors["coordinated_behavior"] = -0.4
		}
		
		// High confidence spam clusters are very negative
		if cluster.Confidence > 0.8 {
			factors["high_confidence_spam"] = -cluster.Confidence * 0.3
		}
	}
	
	// Content analysis factors
	if len(tweet.URLs) > 3 {
		factors["excessive_urls"] = -0.1
	}
	
	if len(tweet.Mentions) > 10 {
		factors["excessive_mentions"] = -0.1
	}
	
	return factors
}

// generateAccountEvents generates reputation events for an account
func (rs *ReputationSystem) generateAccountEvents(account *models.Account, factors map[string]float64) []ReputationEvent {
	var events []ReputationEvent
	now := time.Now()
	
	for factor, impact := range factors {
		if math.Abs(impact) > 0.01 { // Only generate events for significant impacts
			event := ReputationEvent{
				Timestamp:  now,
				EventType:  rs.factorToEventType(factor),
				Impact:     impact,
				Source:     "account_analysis",
				Confidence: 0.8,
				Metadata: map[string]interface{}{
					"factor":     factor,
					"account_id": account.ID,
				},
			}
			events = append(events, event)
		}
	}
	
	return events
}

// generateContentEvents generates reputation events for content
func (rs *ReputationSystem) generateContentEvents(tweet *models.Tweet, cluster *models.SpamCluster, factors map[string]float64) []ReputationEvent {
	var events []ReputationEvent
	now := time.Now()
	
	for factor, impact := range factors {
		if math.Abs(impact) > 0.01 {
			event := ReputationEvent{
				Timestamp:  now,
				EventType:  rs.factorToEventType(factor),
				Impact:     impact,
				Source:     "content_analysis",
				Confidence: 0.9,
				Metadata: map[string]interface{}{
					"factor":   factor,
					"tweet_id": tweet.ID,
				},
			}
			
			if cluster != nil {
				event.Metadata["cluster_id"] = cluster.ID
				event.Metadata["cluster_confidence"] = cluster.Confidence
			}
			
			events = append(events, event)
		}
	}
	
	return events
}

// updateAccountFromContent updates account reputation based on content analysis
func (rs *ReputationSystem) updateAccountFromContent(tweet *models.Tweet, cluster *models.SpamCluster) error {
	// Create indirect reputation event for account based on content
	impact := 0.0
	eventType := LegitimateContent
	
	if tweet.IsSpam {
		impact = -tweet.SpamScore * 0.3 // Reduce impact for account
		eventType = SpamDetected
	} else {
		impact = 0.05 // Small positive impact for legitimate content
	}
	
	event := ReputationEvent{
		Timestamp:  time.Now(),
		EventType:  eventType,
		Impact:     impact,
		Source:     "content_behavior",
		Confidence: 0.7,
		Metadata: map[string]interface{}{
			"tweet_id":    tweet.ID,
			"is_spam":     tweet.IsSpam,
			"spam_score":  tweet.SpamScore,
		},
	}
	
	if cluster != nil {
		event.Metadata["cluster_id"] = cluster.ID
		event.Metadata["cluster_confidence"] = cluster.Confidence
	}
	
	return rs.UpdateReputation(tweet.AuthorID, Account, event)
}

// factorToEventType maps factor names to event types
func (rs *ReputationSystem) factorToEventType(factor string) EventType {
	switch factor {
	case "spam_detected", "high_confidence_spam", "spam_activity":
		return SpamDetected
	case "legitimate_content":
		return LegitimateContent
	case "coordinated_behavior":
		return CoordinatedBehavior
	case "bot_score":
		return SuspiciousPattern
	case "verification":
		return VerificationPassed
	default:
		return AutomaticFlag
	}
}

// IsSpammy checks if an entity is considered spammy based on reputation
func (rs *ReputationSystem) IsSpammy(entityID string) (bool, float64, error) {
	score, err := rs.GetReputation(entityID)
	if err != nil {
		return false, 0.0, err
	}
	
	isSpammy := score.Score < rs.config.SpamThreshold && score.Confidence > rs.config.ConfidenceThreshold
	return isSpammy, score.Score, nil
}

// IsTrusted checks if an entity is considered trusted based on reputation
func (rs *ReputationSystem) IsTrusted(entityID string) (bool, float64, error) {
	score, err := rs.GetReputation(entityID)
	if err != nil {
		return false, 0.0, err
	}
	
	isTrusted := score.Score > rs.config.TrustedThreshold && score.Confidence > rs.config.ConfidenceThreshold
	return isTrusted, score.Score, nil
}

// GetStatistics returns reputation system statistics
func (rs *ReputationSystem) GetStatistics() ReputationStatistics {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	stats := ReputationStatistics{
		TotalEntities: len(rs.scores),
	}
	
	for _, score := range rs.scores {
		switch score.EntityType {
		case Account:
			stats.AccountCount++
		case Content:
			stats.ContentCount++
		}
		
		if score.Score < rs.config.SpamThreshold {
			stats.SpammyEntities++
		} else if score.Score > rs.config.TrustedThreshold {
			stats.TrustedEntities++
		}
		
		stats.AverageScore += score.Score
		stats.AverageConfidence += score.Confidence
	}
	
	if stats.TotalEntities > 0 {
		stats.AverageScore /= float64(stats.TotalEntities)
		stats.AverageConfidence /= float64(stats.TotalEntities)
	}
	
	return stats
}

// ReputationStatistics represents system statistics
type ReputationStatistics struct {
	TotalEntities     int     `json:"total_entities"`
	AccountCount      int     `json:"account_count"`
	ContentCount      int     `json:"content_count"`
	SpammyEntities    int     `json:"spammy_entities"`
	TrustedEntities   int     `json:"trusted_entities"`
	AverageScore      float64 `json:"average_score"`
	AverageConfidence float64 `json:"average_confidence"`
}

// ExponentialDecay implements exponential decay for reputation scores
type ExponentialDecay struct{}

// Apply applies exponential decay to a score
func (ed *ExponentialDecay) Apply(score float64, age time.Duration, config ReputationConfig) float64 {
	if age <= 0 {
		return score
	}
	
	// Exponential decay: score * e^(-rate * time)
	decayFactor := math.Exp(-config.DecayRate * age.Hours() / 24.0) // Decay per day
	
	// Move towards initial score
	decayedScore := score*decayFactor + config.InitialScore*(1-decayFactor)
	
	return math.Max(config.MinScore, math.Min(config.MaxScore, decayedScore))
}

// BackgroundProcessor handles background processing for the reputation system
type BackgroundProcessor struct {
	system   *ReputationSystem
	stopChan chan struct{}
}

// NewBackgroundProcessor creates a new background processor
func NewBackgroundProcessor(system *ReputationSystem) *BackgroundProcessor {
	return &BackgroundProcessor{
		system:   system,
		stopChan: make(chan struct{}),
	}
}

// Start starts background processing
func (bp *BackgroundProcessor) Start() error {
	go bp.processUpdates()
	go bp.periodicMaintenance()
	return nil
}

// Stop stops background processing
func (bp *BackgroundProcessor) Stop() error {
	close(bp.stopChan)
	return nil
}

// processUpdates processes reputation updates from the queue
func (bp *BackgroundProcessor) processUpdates() {
	for {
		select {
		case update := <-bp.system.updateQueue:
			if err := bp.system.processUpdate(update); err != nil {
				fmt.Printf("Error processing reputation update: %v\n", err)
			}
			
		case <-bp.stopChan:
			return
		}
	}
}

// periodicMaintenance performs periodic maintenance tasks
func (bp *BackgroundProcessor) periodicMaintenance() {
	ticker := time.NewTicker(bp.system.config.UpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			bp.performMaintenance()
			
		case <-bp.stopChan:
			return
		}
	}
}

// performMaintenance performs maintenance tasks
func (bp *BackgroundProcessor) performMaintenance() {
	bp.system.mutex.Lock()
	defer bp.system.mutex.Unlock()
	
	// Apply decay to all scores
	for _, score := range bp.system.scores {
		bp.system.applyDecay(score)
	}
	
	// Clean up very old scores with low confidence
	cutoff := time.Now().Add(-bp.system.config.MaxEventAge * 2)
	for entityID, score := range bp.system.scores {
		if score.LastUpdate.Before(cutoff) && score.Confidence < 0.1 {
			delete(bp.system.scores, entityID)
		}
	}
}

// Default configuration
func DefaultReputationConfig() ReputationConfig {
	return ReputationConfig{
		InitialScore:        0.5,
		MinScore:           0.0,
		MaxScore:           1.0,
		ConfidenceThreshold: 0.6,
		DecayRate:          0.1,
		DecayInterval:      24 * time.Hour,
		MaxEventAge:        30 * 24 * time.Hour, // 30 days
		MaxHistorySize:     100,
		BatchSize:          100,
		UpdateInterval:     time.Hour,
		SpamThreshold:      0.3,
		TrustedThreshold:   0.7,
	}
}
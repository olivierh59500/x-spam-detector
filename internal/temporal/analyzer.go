package temporal

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// TemporalAnalyzer analyzes temporal patterns in spam campaigns
type TemporalAnalyzer struct {
	windowSize     time.Duration
	minEvents      int
	burstThreshold float64
	periodicThreshold float64
	mutex          sync.RWMutex
	
	// Historical data for analysis
	eventHistory   []TemporalEvent
	patterns       map[string]*TemporalPattern
	baseline       *BaselineMetrics
}

// TemporalEvent represents a timestamped event
type TemporalEvent struct {
	Timestamp   time.Time
	EventType   string
	AccountID   string
	TweetID     string
	ClusterID   string
	Metadata    map[string]interface{}
}

// TemporalPattern represents a detected temporal pattern
type TemporalPattern struct {
	ID              string                `json:"id"`
	Type            PatternType           `json:"type"`
	StartTime       time.Time             `json:"start_time"`
	EndTime         time.Time             `json:"end_time"`
	Duration        time.Duration         `json:"duration"`
	EventCount      int                   `json:"event_count"`
	Frequency       float64               `json:"frequency"`
	Confidence      float64               `json:"confidence"`
	Accounts        []string              `json:"accounts"`
	Clusters        []string              `json:"clusters"`
	Characteristics PatternCharacteristics `json:"characteristics"`
	Severity        SeverityLevel         `json:"severity"`
}

// PatternType defines types of temporal patterns
type PatternType int

const (
	BurstPattern PatternType = iota
	PeriodicPattern
	CoordinatedPattern
	GradualRampPattern
	FlashMobPattern
)

// SeverityLevel defines pattern severity
type SeverityLevel int

const (
	Low SeverityLevel = iota
	Medium
	High
	Critical
)

// PatternCharacteristics describes pattern characteristics
type PatternCharacteristics struct {
	PeakIntensity      float64           `json:"peak_intensity"`
	AverageIntensity   float64           `json:"average_intensity"`
	Variability        float64           `json:"variability"`
	Periodicity        time.Duration     `json:"periodicity"`
	AccountDiversity   float64           `json:"account_diversity"`
	GeographicSpread   float64           `json:"geographic_spread"`
	ContentSimilarity  float64           `json:"content_similarity"`
	TemporalDistribution map[string]int  `json:"temporal_distribution"`
}

// BaselineMetrics stores baseline activity metrics
type BaselineMetrics struct {
	AverageRate     float64   `json:"average_rate"`
	StandardDev     float64   `json:"standard_deviation"`
	MaxRate         float64   `json:"max_rate"`
	MinRate         float64   `json:"min_rate"`
	LastUpdated     time.Time `json:"last_updated"`
	SampleSize      int       `json:"sample_size"`
	ConfidenceLevel float64   `json:"confidence_level"`
}

// BurstDetectionResult represents the result of burst detection
type BurstDetectionResult struct {
	IsBurst         bool          `json:"is_burst"`
	BurstStrength   float64       `json:"burst_strength"`
	ZScore          float64       `json:"z_score"`
	PValue          float64       `json:"p_value"`
	StartTime       time.Time     `json:"start_time"`
	PeakTime        time.Time     `json:"peak_time"`
	EndTime         time.Time     `json:"end_time"`
	Events          []TemporalEvent `json:"events"`
	Confidence      float64       `json:"confidence"`
}

// NewTemporalAnalyzer creates a new temporal analyzer
func NewTemporalAnalyzer(windowSize time.Duration, minEvents int, burstThreshold float64) *TemporalAnalyzer {
	return &TemporalAnalyzer{
		windowSize:        windowSize,
		minEvents:         minEvents,
		burstThreshold:    burstThreshold,
		periodicThreshold: 0.7,
		eventHistory:      make([]TemporalEvent, 0),
		patterns:          make(map[string]*TemporalPattern),
		baseline: &BaselineMetrics{
			ConfidenceLevel: 0.95,
		},
	}
}

// AddEvent adds a new temporal event for analysis
func (ta *TemporalAnalyzer) AddEvent(timestamp time.Time, eventType, accountID, tweetID, clusterID string, metadata map[string]interface{}) {
	ta.mutex.Lock()
	defer ta.mutex.Unlock()
	
	event := TemporalEvent{
		Timestamp: timestamp,
		EventType: eventType,
		AccountID: accountID,
		TweetID:   tweetID,
		ClusterID: clusterID,
		Metadata:  metadata,
	}
	
	ta.eventHistory = append(ta.eventHistory, event)
	
	// Keep only recent events within the analysis window
	cutoff := time.Now().Add(-ta.windowSize * 10) // Keep 10x window for baseline
	ta.eventHistory = ta.filterEventsByTime(ta.eventHistory, cutoff)
	
	// Update baseline metrics
	ta.updateBaselineMetrics()
}

// DetectBurst detects burst patterns in the event stream
func (ta *TemporalAnalyzer) DetectBurst(events []TemporalEvent, windowSize time.Duration) *BurstDetectionResult {
	if len(events) < ta.minEvents {
		return &BurstDetectionResult{IsBurst: false}
	}
	
	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	
	// Calculate event rates in sliding windows
	rates := ta.calculateSlidingWindowRates(events, windowSize)
	if len(rates) == 0 {
		return &BurstDetectionResult{IsBurst: false}
	}
	
	// Statistical analysis
	mean, stdDev := ta.calculateStatistics(rates)
	maxRate := ta.findMaxRate(rates)
	
	// Z-score calculation
	var zScore float64
	if stdDev > 0 {
		zScore = (maxRate - mean) / stdDev
	}
	
	// Determine if this is a burst
	isBurst := zScore > ta.burstThreshold
	confidence := ta.calculateBurstConfidence(zScore, len(events))
	
	// Find burst boundaries
	startTime, peakTime, endTime := ta.findBurstBoundaries(events, rates, maxRate)
	
	return &BurstDetectionResult{
		IsBurst:       isBurst,
		BurstStrength: maxRate,
		ZScore:        zScore,
		PValue:        ta.calculatePValue(zScore),
		StartTime:     startTime,
		PeakTime:      peakTime,
		EndTime:       endTime,
		Events:        events,
		Confidence:    confidence,
	}
}

// DetectPeriodic detects periodic patterns in the event stream
func (ta *TemporalAnalyzer) DetectPeriodic(events []TemporalEvent, minPeriod, maxPeriod time.Duration) []*TemporalPattern {
	if len(events) < ta.minEvents*2 {
		return nil
	}
	
	var patterns []*TemporalPattern
	
	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	
	// Test different periods
	for period := minPeriod; period <= maxPeriod; period += time.Minute {
		if correlation := ta.calculatePeriodicCorrelation(events, period); correlation > ta.periodicThreshold {
			pattern := &TemporalPattern{
				ID:        ta.generatePatternID(),
				Type:      PeriodicPattern,
				StartTime: events[0].Timestamp,
				EndTime:   events[len(events)-1].Timestamp,
				Duration:  events[len(events)-1].Timestamp.Sub(events[0].Timestamp),
				EventCount: len(events),
				Frequency: float64(len(events)) / events[len(events)-1].Timestamp.Sub(events[0].Timestamp).Hours(),
				Confidence: correlation,
				Characteristics: PatternCharacteristics{
					Periodicity: period,
				},
				Severity: ta.calculateSeverity(correlation, len(events)),
			}
			
			ta.enrichPatternCharacteristics(pattern, events)
			patterns = append(patterns, pattern)
		}
	}
	
	return patterns
}

// DetectCoordinated detects coordinated activity patterns
func (ta *TemporalAnalyzer) DetectCoordinated(cluster *models.SpamCluster) *TemporalPattern {
	if cluster.Size < ta.minEvents {
		return nil
	}
	
	// Extract temporal events from cluster
	events := ta.extractEventsFromCluster(cluster)
	
	// Calculate coordination metrics
	coordination := ta.calculateCoordinationScore(events)
	if coordination < 0.6 {
		return nil
	}
	
	pattern := &TemporalPattern{
		ID:          ta.generatePatternID(),
		Type:        CoordinatedPattern,
		StartTime:   cluster.FirstSeen,
		EndTime:     cluster.LastSeen,
		Duration:    cluster.Duration,
		EventCount:  cluster.Size,
		Frequency:   cluster.TweetsPerMinute,
		Confidence:  coordination,
		Accounts:    cluster.AccountIDs,
		Clusters:    []string{cluster.ID},
		Severity:    ta.calculateSeverity(coordination, cluster.Size),
	}
	
	ta.enrichPatternCharacteristics(pattern, events)
	return pattern
}

// calculateSlidingWindowRates calculates event rates in sliding windows
func (ta *TemporalAnalyzer) calculateSlidingWindowRates(events []TemporalEvent, windowSize time.Duration) []float64 {
	if len(events) < 2 {
		return nil
	}
	
	var rates []float64
	start := events[0].Timestamp
	end := events[len(events)-1].Timestamp
	
	for current := start; current.Before(end); current = current.Add(windowSize / 10) {
		windowEnd := current.Add(windowSize)
		count := 0
		
		for _, event := range events {
			if event.Timestamp.After(current) && event.Timestamp.Before(windowEnd) {
				count++
			}
		}
		
		rate := float64(count) / windowSize.Minutes()
		rates = append(rates, rate)
	}
	
	return rates
}

// calculateStatistics calculates mean and standard deviation
func (ta *TemporalAnalyzer) calculateStatistics(values []float64) (mean, stdDev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	
	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))
	
	// Calculate standard deviation
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values))
	stdDev = math.Sqrt(variance)
	
	return mean, stdDev
}

// findMaxRate finds the maximum rate in the slice
func (ta *TemporalAnalyzer) findMaxRate(rates []float64) float64 {
	if len(rates) == 0 {
		return 0
	}
	
	max := rates[0]
	for _, rate := range rates[1:] {
		if rate > max {
			max = rate
		}
	}
	return max
}

// calculateBurstConfidence calculates confidence for burst detection
func (ta *TemporalAnalyzer) calculateBurstConfidence(zScore float64, eventCount int) float64 {
	// Confidence based on Z-score and sample size
	confidence := math.Min(1.0, math.Abs(zScore)/5.0)
	
	// Adjust for sample size
	sampleFactor := math.Min(1.0, float64(eventCount)/float64(ta.minEvents*2))
	confidence *= sampleFactor
	
	return confidence
}

// calculatePValue calculates p-value for burst detection
func (ta *TemporalAnalyzer) calculatePValue(zScore float64) float64 {
	// Simplified p-value calculation (normally distributed)
	return 2.0 * (1.0 - ta.normalCDF(math.Abs(zScore)))
}

// normalCDF approximates the standard normal cumulative distribution function
func (ta *TemporalAnalyzer) normalCDF(x float64) float64 {
	// Abramowitz and Stegun approximation
	t := 1.0 / (1.0 + 0.2316419*math.Abs(x))
	d := 0.3989423 * math.Exp(-x*x/2.0)
	prob := d * t * (0.3193815+t*(-0.3565638+t*(1.781478+t*(-1.821256+t*1.330274))))
	
	if x > 0 {
		return 1.0 - prob
	}
	return prob
}

// findBurstBoundaries finds the start, peak, and end times of a burst
func (ta *TemporalAnalyzer) findBurstBoundaries(events []TemporalEvent, rates []float64, maxRate float64) (start, peak, end time.Time) {
	if len(events) == 0 || len(rates) == 0 {
		return
	}
	
	// Find peak index
	peakIndex := 0
	for i, rate := range rates {
		if rate == maxRate {
			peakIndex = i
			break
		}
	}
	
	// Calculate peak time
	if peakIndex < len(events) {
		peak = events[peakIndex].Timestamp
	}
	
	// Find burst boundaries (where rate drops below threshold)
	threshold := maxRate * 0.3
	
	// Find start
	for i := peakIndex; i >= 0; i-- {
		if rates[i] < threshold {
			if i+1 < len(events) {
				start = events[i+1].Timestamp
			}
			break
		}
	}
	
	// Find end
	for i := peakIndex; i < len(rates); i++ {
		if rates[i] < threshold {
			if i < len(events) {
				end = events[i].Timestamp
			}
			break
		}
	}
	
	if start.IsZero() {
		start = events[0].Timestamp
	}
	if end.IsZero() {
		end = events[len(events)-1].Timestamp
	}
	
	return start, peak, end
}

// calculatePeriodicCorrelation calculates correlation for periodic patterns
func (ta *TemporalAnalyzer) calculatePeriodicCorrelation(events []TemporalEvent, period time.Duration) float64 {
	if len(events) < ta.minEvents {
		return 0
	}
	
	// Create time series with period buckets
	start := events[0].Timestamp
	end := events[len(events)-1].Timestamp
	duration := end.Sub(start)
	
	buckets := int(duration / period)
	if buckets < 2 {
		return 0
	}
	
	bucketCounts := make([]float64, buckets)
	
	// Count events in each bucket
	for _, event := range events {
		bucketIndex := int(event.Timestamp.Sub(start) / period)
		if bucketIndex >= 0 && bucketIndex < buckets {
			bucketCounts[bucketIndex]++
		}
	}
	
	// Calculate autocorrelation
	return ta.calculateAutocorrelation(bucketCounts, 1)
}

// calculateAutocorrelation calculates autocorrelation with given lag
func (ta *TemporalAnalyzer) calculateAutocorrelation(data []float64, lag int) float64 {
	if len(data) <= lag {
		return 0
	}
	
	n := len(data) - lag
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))
	
	numerator := 0.0
	denominator := 0.0
	
	for i := 0; i < n; i++ {
		x := data[i] - mean
		y := data[i+lag] - mean
		numerator += x * y
		denominator += x * x
	}
	
	if denominator == 0 {
		return 0
	}
	
	return numerator / denominator
}

// calculateCoordinationScore calculates coordination score for events
func (ta *TemporalAnalyzer) calculateCoordinationScore(events []TemporalEvent) float64 {
	if len(events) < 2 {
		return 0
	}
	
	// Calculate temporal clustering
	intervals := make([]time.Duration, len(events)-1)
	for i := 1; i < len(events); i++ {
		intervals[i-1] = events[i].Timestamp.Sub(events[i-1].Timestamp)
	}
	
	// Calculate variance of intervals
	mean := time.Duration(0)
	for _, interval := range intervals {
		mean += interval
	}
	mean /= time.Duration(len(intervals))
	
	variance := float64(0)
	for _, interval := range intervals {
		diff := float64(interval - mean)
		variance += diff * diff
	}
	variance /= float64(len(intervals))
	
	// Lower variance = higher coordination
	coordination := 1.0 / (1.0 + variance/1e18) // Normalize variance
	
	return math.Min(1.0, coordination)
}

// extractEventsFromCluster extracts temporal events from a spam cluster
func (ta *TemporalAnalyzer) extractEventsFromCluster(cluster *models.SpamCluster) []TemporalEvent {
	events := make([]TemporalEvent, len(cluster.Tweets))
	
	for i, tweet := range cluster.Tweets {
		events[i] = TemporalEvent{
			Timestamp: tweet.CreatedAt,
			EventType: "spam_tweet",
			AccountID: tweet.AuthorID,
			TweetID:   tweet.ID,
			ClusterID: cluster.ID,
		}
	}
	
	return events
}

// enrichPatternCharacteristics adds detailed characteristics to a pattern
func (ta *TemporalAnalyzer) enrichPatternCharacteristics(pattern *TemporalPattern, events []TemporalEvent) {
	if len(events) == 0 {
		return
	}
	
	// Calculate account diversity
	accounts := make(map[string]bool)
	for _, event := range events {
		accounts[event.AccountID] = true
	}
	pattern.Characteristics.AccountDiversity = float64(len(accounts)) / float64(len(events))
	
	// Calculate temporal distribution
	pattern.Characteristics.TemporalDistribution = ta.calculateTemporalDistribution(events)
	
	// Calculate intensity metrics
	rates := ta.calculateSlidingWindowRates(events, time.Minute*5)
	if len(rates) > 0 {
		pattern.Characteristics.PeakIntensity = ta.findMaxRate(rates)
		mean, _ := ta.calculateStatistics(rates)
		pattern.Characteristics.AverageIntensity = mean
		
		// Calculate variability
		variance := 0.0
		for _, rate := range rates {
			variance += math.Pow(rate-mean, 2)
		}
		pattern.Characteristics.Variability = math.Sqrt(variance / float64(len(rates)))
	}
	
	// Update accounts and clusters
	accountSet := make(map[string]bool)
	clusterSet := make(map[string]bool)
	
	for _, event := range events {
		accountSet[event.AccountID] = true
		if event.ClusterID != "" {
			clusterSet[event.ClusterID] = true
		}
	}
	
	pattern.Accounts = make([]string, 0, len(accountSet))
	for account := range accountSet {
		pattern.Accounts = append(pattern.Accounts, account)
	}
	
	pattern.Clusters = make([]string, 0, len(clusterSet))
	for cluster := range clusterSet {
		pattern.Clusters = append(pattern.Clusters, cluster)
	}
}

// calculateTemporalDistribution calculates hourly distribution of events
func (ta *TemporalAnalyzer) calculateTemporalDistribution(events []TemporalEvent) map[string]int {
	distribution := make(map[string]int)
	
	for _, event := range events {
		hour := event.Timestamp.Format("15")
		distribution[hour]++
	}
	
	return distribution
}

// calculateSeverity calculates severity level based on confidence and event count
func (ta *TemporalAnalyzer) calculateSeverity(confidence float64, eventCount int) SeverityLevel {
	score := confidence * float64(eventCount) / 100.0
	
	if score > 0.8 {
		return Critical
	} else if score > 0.6 {
		return High
	} else if score > 0.4 {
		return Medium
	}
	return Low
}

// generatePatternID generates a unique pattern ID
func (ta *TemporalAnalyzer) generatePatternID() string {
	return fmt.Sprintf("pattern_%d", time.Now().UnixNano())
}

// updateBaselineMetrics updates baseline activity metrics
func (ta *TemporalAnalyzer) updateBaselineMetrics() {
	if len(ta.eventHistory) < 10 {
		return
	}
	
	// Calculate recent activity rate
	recent := time.Now().Add(-time.Hour)
	recentEvents := ta.filterEventsByTime(ta.eventHistory, recent)
	
	rate := float64(len(recentEvents)) / 1.0 // events per hour
	
	// Update baseline using exponential smoothing
	alpha := 0.1 // smoothing factor
	if ta.baseline.SampleSize == 0 {
		ta.baseline.AverageRate = rate
	} else {
		ta.baseline.AverageRate = alpha*rate + (1-alpha)*ta.baseline.AverageRate
	}
	
	ta.baseline.SampleSize++
	ta.baseline.LastUpdated = time.Now()
	
	// Update min/max
	if rate > ta.baseline.MaxRate {
		ta.baseline.MaxRate = rate
	}
	if ta.baseline.MinRate == 0 || rate < ta.baseline.MinRate {
		ta.baseline.MinRate = rate
	}
}

// filterEventsByTime filters events by minimum timestamp
func (ta *TemporalAnalyzer) filterEventsByTime(events []TemporalEvent, minTime time.Time) []TemporalEvent {
	var filtered []TemporalEvent
	for _, event := range events {
		if event.Timestamp.After(minTime) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// GetPatterns returns all detected patterns
func (ta *TemporalAnalyzer) GetPatterns() map[string]*TemporalPattern {
	ta.mutex.RLock()
	defer ta.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	patterns := make(map[string]*TemporalPattern)
	for id, pattern := range ta.patterns {
		patterns[id] = pattern
	}
	return patterns
}

// GetBaselineMetrics returns current baseline metrics
func (ta *TemporalAnalyzer) GetBaselineMetrics() *BaselineMetrics {
	ta.mutex.RLock()
	defer ta.mutex.RUnlock()
	return ta.baseline
}

// ClearOldPatterns removes patterns older than the specified duration
func (ta *TemporalAnalyzer) ClearOldPatterns(maxAge time.Duration) {
	ta.mutex.Lock()
	defer ta.mutex.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	for id, pattern := range ta.patterns {
		if pattern.EndTime.Before(cutoff) {
			delete(ta.patterns, id)
		}
	}
}
package lsh

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"x-spam-detector/internal/cache"
)

// HybridLSH combines multiple LSH algorithms with weighted scoring
type HybridLSH struct {
	minHashLSH      *CachedMinHashLSH
	simHashLSH      *OptimizedSimHashLSH
	weights         LSHWeights
	resultCache     *cache.LRUCache
	mutex           sync.RWMutex
	
	// Algorithm selection strategy
	strategy        HybridStrategy
	threshold       float64
	
	// Performance tracking
	algorithmStats  map[string]*AlgorithmStats
}

// LSHWeights defines weights for different LSH algorithms
type LSHWeights struct {
	MinHash      float64 `json:"minhash_weight"`
	SimHash      float64 `json:"simhash_weight"`
	TextLength   float64 `json:"text_length_factor"`
	ContentType  float64 `json:"content_type_factor"`
}

// HybridStrategy defines how algorithms are combined
type HybridStrategy int

const (
	WeightedAverage HybridStrategy = iota
	MaxScore
	MinScore
	AdaptiveWeighting
	ConsensusVoting
)

// AlgorithmStats tracks performance of individual algorithms
type AlgorithmStats struct {
	TotalQueries     int64         `json:"total_queries"`
	AvgResponseTime  time.Duration `json:"avg_response_time"`
	AccuracyScore    float64       `json:"accuracy_score"`
	LastUpdated      time.Time     `json:"last_updated"`
	TruePositives    int64         `json:"true_positives"`
	FalsePositives   int64         `json:"false_positives"`
	TrueNegatives    int64         `json:"true_negatives"`
	FalseNegatives   int64         `json:"false_negatives"`
}

// NewHybridLSH creates a new hybrid LSH instance
func NewHybridLSH(minHashConfig, simHashConfig LSHConfig, strategy HybridStrategy, threshold float64) (*HybridLSH, error) {
	weights := LSHWeights{
		MinHash:     0.6,
		SimHash:     0.4,
		TextLength:  0.1,
		ContentType: 0.1,
	}
	
	// Create MinHash LSH with error handling
	minHashLSH, err := NewCachedMinHashLSH(minHashConfig.NumHashes, minHashConfig.Bands, minHashConfig.Threshold, minHashConfig.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cached MinHash LSH: %w", err)
	}
	
	return &HybridLSH{
		minHashLSH:     minHashLSH,
		simHashLSH:     NewOptimizedSimHashLSH(simHashConfig.HammingThreshold, simHashConfig.NumTables, simHashConfig.CacheSize),
		weights:        weights,
		resultCache:    cache.NewLRUCache(1000, 10*time.Minute),
		strategy:       strategy,
		threshold:      threshold,
		algorithmStats: make(map[string]*AlgorithmStats),
	}, nil
}

// LSHConfig represents configuration for LSH algorithms
type LSHConfig struct {
	NumHashes         int     `json:"num_hashes"`
	Bands            int     `json:"bands"`
	Threshold        float64 `json:"threshold"`
	HammingThreshold int     `json:"hamming_threshold"`
	NumTables        int     `json:"num_tables"`
	CacheSize        int     `json:"cache_size"`
}

// Add adds a document to both LSH indices
func (h *HybridLSH) Add(docID, text string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	// Add to both algorithms concurrently
	var wg sync.WaitGroup
	
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		h.minHashLSH.Add(docID, text)
	}()
	
	go func() {
		defer wg.Done()
		h.simHashLSH.Add(docID, text)
	}()
	
	wg.Wait()
	
	return nil
}

// FindSimilar finds similar documents using hybrid approach
func (h *HybridLSH) FindSimilar(docID string) ([]SimilarDocument, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("hybrid_similar_%s", docID)
	if cached, exists := h.resultCache.Get(cacheKey); exists {
		return cached.([]SimilarDocument), nil
	}
	
	// Get results from both algorithms concurrently
	minHashChan := make(chan []SimilarDocument, 1)
	simHashChan := make(chan []SimilarDocument, 1)
	
	go func() {
		start := time.Now()
		results := h.minHashLSH.FindSimilar(docID)
		h.updateAlgorithmStats("minhash", time.Since(start), len(results))
		minHashChan <- results
	}()
	
	go func() {
		start := time.Now()
		results := h.simHashLSH.FindSimilar(docID)
		h.updateAlgorithmStats("simhash", time.Since(start), len(results))
		simHashChan <- results
	}()
	
	minHashResults := <-minHashChan
	simHashResults := <-simHashChan
	
	// Combine results using selected strategy
	combinedResults := h.combineResults(docID, minHashResults, simHashResults)
	
	// Cache the results
	h.resultCache.Set(cacheKey, combinedResults)
	
	return combinedResults, nil
}

// combineResults combines results from multiple algorithms
func (h *HybridLSH) combineResults(docID string, minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	switch h.strategy {
	case WeightedAverage:
		return h.weightedAverageStrategy(docID, minHashResults, simHashResults)
	case MaxScore:
		return h.maxScoreStrategy(minHashResults, simHashResults)
	case MinScore:
		return h.minScoreStrategy(minHashResults, simHashResults)
	case AdaptiveWeighting:
		return h.adaptiveWeightingStrategy(docID, minHashResults, simHashResults)
	case ConsensusVoting:
		return h.consensusVotingStrategy(minHashResults, simHashResults)
	default:
		return h.weightedAverageStrategy(docID, minHashResults, simHashResults)
	}
}

// weightedAverageStrategy combines results using weighted averages
func (h *HybridLSH) weightedAverageStrategy(docID string, minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	resultMap := make(map[string]*SimilarDocument)
	
	// Calculate dynamic weights based on text characteristics
	weights := h.calculateDynamicWeights(docID)
	
	// Add MinHash results
	for _, result := range minHashResults {
		resultMap[result.ID] = &SimilarDocument{
			ID:         result.ID,
			Text:       result.Text,
			Similarity: result.Similarity * weights.MinHash,
		}
	}
	
	// Add or update with SimHash results
	for _, result := range simHashResults {
		if existing, exists := resultMap[result.ID]; exists {
			existing.Similarity += result.Similarity * weights.SimHash
		} else {
			resultMap[result.ID] = &SimilarDocument{
				ID:         result.ID,
				Text:       result.Text,
				Similarity: result.Similarity * weights.SimHash,
			}
		}
	}
	
	// Convert to slice and filter by threshold
	var results []SimilarDocument
	for _, result := range resultMap {
		if result.Similarity >= h.threshold {
			results = append(results, *result)
		}
	}
	
	// Sort by similarity score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	
	return results
}

// maxScoreStrategy takes the maximum score from either algorithm
func (h *HybridLSH) maxScoreStrategy(minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	resultMap := make(map[string]*SimilarDocument)
	
	// Add MinHash results
	for _, result := range minHashResults {
		resultMap[result.ID] = &result
	}
	
	// Update with SimHash results if score is higher
	for _, result := range simHashResults {
		if existing, exists := resultMap[result.ID]; exists {
			if result.Similarity > existing.Similarity {
				resultMap[result.ID] = &result
			}
		} else {
			resultMap[result.ID] = &result
		}
	}
	
	return h.filterAndSort(resultMap)
}

// minScoreStrategy takes the minimum score from either algorithm
func (h *HybridLSH) minScoreStrategy(minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	resultMap := make(map[string]*SimilarDocument)
	
	// Only include documents found by both algorithms
	minHashMap := make(map[string]SimilarDocument)
	for _, result := range minHashResults {
		minHashMap[result.ID] = result
	}
	
	for _, simResult := range simHashResults {
		if minResult, exists := minHashMap[simResult.ID]; exists {
			similarity := math.Min(minResult.Similarity, simResult.Similarity)
			resultMap[simResult.ID] = &SimilarDocument{
				ID:         simResult.ID,
				Text:       simResult.Text,
				Similarity: similarity,
			}
		}
	}
	
	return h.filterAndSort(resultMap)
}

// adaptiveWeightingStrategy adjusts weights based on algorithm performance
func (h *HybridLSH) adaptiveWeightingStrategy(docID string, minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	// Get algorithm performance stats
	minHashStats := h.algorithmStats["minhash"]
	simHashStats := h.algorithmStats["simhash"]
	
	weights := h.weights
	
	// Adjust weights based on accuracy
	if minHashStats != nil && simHashStats != nil {
		totalAccuracy := minHashStats.AccuracyScore + simHashStats.AccuracyScore
		if totalAccuracy > 0 {
			weights.MinHash = minHashStats.AccuracyScore / totalAccuracy
			weights.SimHash = simHashStats.AccuracyScore / totalAccuracy
		}
	}
	
	// Apply text-specific adjustments
	textWeights := h.calculateDynamicWeights(docID)
	weights.MinHash *= textWeights.MinHash
	weights.SimHash *= textWeights.SimHash
	
	// Normalize weights
	totalWeight := weights.MinHash + weights.SimHash
	if totalWeight > 0 {
		weights.MinHash /= totalWeight
		weights.SimHash /= totalWeight
	}
	
	// Apply weighted combination
	resultMap := make(map[string]*SimilarDocument)
	
	for _, result := range minHashResults {
		resultMap[result.ID] = &SimilarDocument{
			ID:         result.ID,
			Text:       result.Text,
			Similarity: result.Similarity * weights.MinHash,
		}
	}
	
	for _, result := range simHashResults {
		if existing, exists := resultMap[result.ID]; exists {
			existing.Similarity += result.Similarity * weights.SimHash
		} else {
			resultMap[result.ID] = &SimilarDocument{
				ID:         result.ID,
				Text:       result.Text,
				Similarity: result.Similarity * weights.SimHash,
			}
		}
	}
	
	return h.filterAndSort(resultMap)
}

// consensusVotingStrategy requires agreement from multiple algorithms
func (h *HybridLSH) consensusVotingStrategy(minHashResults, simHashResults []SimilarDocument) []SimilarDocument {
	resultMap := make(map[string]*SimilarDocument)
	
	// Only include documents found by both algorithms
	minHashMap := make(map[string]SimilarDocument)
	for _, result := range minHashResults {
		minHashMap[result.ID] = result
	}
	
	for _, simResult := range simHashResults {
		if minResult, exists := minHashMap[simResult.ID]; exists {
			// Average the similarities
			avgSimilarity := (minResult.Similarity + simResult.Similarity) / 2.0
			resultMap[simResult.ID] = &SimilarDocument{
				ID:         simResult.ID,
				Text:       simResult.Text,
				Similarity: avgSimilarity,
			}
		}
	}
	
	return h.filterAndSort(resultMap)
}

// calculateDynamicWeights calculates weights based on text characteristics
func (h *HybridLSH) calculateDynamicWeights(_ string) LSHWeights {
	weights := h.weights
	
	// Get document text (assuming we can retrieve it)
	// This is a simplified implementation
	textLength := 100 // Default length
	
	// Adjust weights based on text length
	// MinHash works better for longer texts, SimHash for shorter
	if textLength > 200 {
		weights.MinHash *= 1.2
		weights.SimHash *= 0.8
	} else if textLength < 50 {
		weights.MinHash *= 0.8
		weights.SimHash *= 1.2
	}
	
	// Normalize weights
	totalWeight := weights.MinHash + weights.SimHash
	if totalWeight > 0 {
		weights.MinHash /= totalWeight
		weights.SimHash /= totalWeight
	}
	
	return weights
}

// filterAndSort filters results by threshold and sorts by similarity
func (h *HybridLSH) filterAndSort(resultMap map[string]*SimilarDocument) []SimilarDocument {
	var results []SimilarDocument
	for _, result := range resultMap {
		if result.Similarity >= h.threshold {
			results = append(results, *result)
		}
	}
	
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	
	return results
}

// updateAlgorithmStats updates performance statistics for an algorithm
func (h *HybridLSH) updateAlgorithmStats(algorithm string, responseTime time.Duration, _ int) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	stats, exists := h.algorithmStats[algorithm]
	if !exists {
		stats = &AlgorithmStats{
			LastUpdated: time.Now(),
		}
		h.algorithmStats[algorithm] = stats
	}
	
	stats.TotalQueries++
	if stats.AvgResponseTime == 0 {
		stats.AvgResponseTime = responseTime
	} else {
		stats.AvgResponseTime = (stats.AvgResponseTime + responseTime) / 2
	}
	stats.LastUpdated = time.Now()
}

// UpdateAccuracy updates accuracy statistics for an algorithm
func (h *HybridLSH) UpdateAccuracy(algorithm string, truePos, falsePos, trueNeg, falseNeg int64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	stats, exists := h.algorithmStats[algorithm]
	if !exists {
		stats = &AlgorithmStats{}
		h.algorithmStats[algorithm] = stats
	}
	
	stats.TruePositives += truePos
	stats.FalsePositives += falsePos
	stats.TrueNegatives += trueNeg
	stats.FalseNegatives += falseNeg
	
	// Calculate accuracy
	total := stats.TruePositives + stats.FalsePositives + stats.TrueNegatives + stats.FalseNegatives
	if total > 0 {
		stats.AccuracyScore = float64(stats.TruePositives+stats.TrueNegatives) / float64(total)
	}
	
	stats.LastUpdated = time.Now()
}

// GetStats returns comprehensive statistics for the hybrid LSH
func (h *HybridLSH) GetStats() HybridLSHStats {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	return HybridLSHStats{
		Strategy:          h.strategy,
		Threshold:         h.threshold,
		Weights:          h.weights,
		MinHashCacheStats: h.minHashLSH.GetCacheStats(),
		SimHashCacheStats: h.simHashLSH.GetCacheStats(),
		AlgorithmStats:    h.algorithmStats,
		ResultCacheStats:  h.resultCache.Stats(),
	}
}

// HybridLSHStats represents comprehensive statistics for hybrid LSH
type HybridLSHStats struct {
	Strategy          HybridStrategy                    `json:"strategy"`
	Threshold         float64                          `json:"threshold"`
	Weights           LSHWeights                       `json:"weights"`
	MinHashCacheStats CachePerformanceStats            `json:"minhash_cache_stats"`
	SimHashCacheStats CachePerformanceStats            `json:"simhash_cache_stats"`
	AlgorithmStats    map[string]*AlgorithmStats       `json:"algorithm_stats"`
	ResultCacheStats  cache.CacheStats                 `json:"result_cache_stats"`
}

// SetWeights updates the LSH weights
func (h *HybridLSH) SetWeights(weights LSHWeights) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.weights = weights
}

// SetStrategy updates the hybrid strategy
func (h *HybridLSH) SetStrategy(strategy HybridStrategy) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.strategy = strategy
}

// ClearCaches clears all caches
func (h *HybridLSH) ClearCaches() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	h.minHashLSH.ClearCache()
	h.simHashLSH.ClearCache()
	h.resultCache.Clear()
}
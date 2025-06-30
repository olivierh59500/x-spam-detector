package lsh

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"x-spam-detector/internal/cache"
)

// CachedMinHashLSH extends MinHashLSH with caching capabilities
type CachedMinHashLSH struct {
	*MinHashLSH
	signatureCache  *cache.LRUCache
	similarityCache *cache.SimilarityCache
	bucketCache     *cache.LRUCache
	mutex           sync.RWMutex
	
	// Performance metrics
	cacheHits   int64
	cacheMisses int64
}

// NewCachedMinHashLSH creates a new cached MinHash LSH instance
func NewCachedMinHashLSH(numHashes, bands int, threshold float64, cacheSize int) *CachedMinHashLSH {
	return &CachedMinHashLSH{
		MinHashLSH:      NewMinHashLSH(numHashes, bands, threshold),
		signatureCache:  cache.NewLRUCache(cacheSize, 30*time.Minute),
		similarityCache: cache.NewSimilarityCache(cacheSize*2, 15*time.Minute),
		bucketCache:     cache.NewLRUCache(cacheSize/2, 1*time.Hour),
	}
}

// Add adds a document with caching
func (c *CachedMinHashLSH) Add(docID, text string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if signature is already cached
	sigKey := c.getSignatureKey(text)
	if cachedSig, exists := c.signatureCache.Get(sigKey); exists {
		c.cacheHits++
		signatures := cachedSig.([]uint64)
		c.docHashes[docID] = signatures
		c.docTexts[docID] = text
		c.addToBucketsWithCache(docID, signatures)
		return
	}
	
	c.cacheMisses++
	
	// Compute signature and cache it
	tokens := c.tokenize(text)
	signatures := c.computeMinHash(tokens)
	
	c.signatureCache.Set(sigKey, signatures)
	c.docHashes[docID] = signatures
	c.docTexts[docID] = text
	c.addToBucketsWithCache(docID, signatures)
}

// FindSimilar finds similar documents with caching
func (c *CachedMinHashLSH) FindSimilar(docID string) []SimilarDocument {
	c.mutex.RLock()
	mh, exists := c.docHashes[docID]
	c.mutex.RUnlock()
	
	if !exists {
		return nil
	}
	
	// Check cache for precomputed results
	cacheKey := fmt.Sprintf("similar:%s", docID)
	if cached, exists := c.bucketCache.Get(cacheKey); exists {
		c.cacheHits++
		return cached.([]SimilarDocument)
	}
	
	c.cacheMisses++
	
	candidates := make(map[string]bool)
	
	// Get candidates from LSH buckets
	c.mutex.RLock()
	for band := 0; band < c.bands; band++ {
		sig := c.getBandSignature(mh, band)
		if docs, exists := c.buckets[sig]; exists {
			for _, candidateID := range docs {
				if candidateID != docID {
					candidates[candidateID] = true
				}
			}
		}
	}
	c.mutex.RUnlock()
	
	// Calculate similarities with caching
	var similar []SimilarDocument
	for candidateID := range candidates {
		// Check similarity cache first
		if cachedSim, exists := c.similarityCache.GetSimilarity(docID, candidateID); exists {
			c.cacheHits++
			if cachedSim >= c.threshold {
				similar = append(similar, SimilarDocument{
					ID:         candidateID,
					Text:       c.docTexts[candidateID],
					Similarity: cachedSim,
				})
			}
			continue
		}
		
		c.cacheMisses++
		candidateMH := c.docHashes[candidateID]
		similarity := c.jaccardSimilarity(mh, candidateMH)
		
		// Cache the similarity
		c.similarityCache.SetSimilarity(docID, candidateID, similarity)
		
		if similarity >= c.threshold {
			similar = append(similar, SimilarDocument{
				ID:         candidateID,
				Text:       c.docTexts[candidateID],
				Similarity: similarity,
			})
		}
	}
	
	// Cache the result
	c.bucketCache.Set(cacheKey, similar)
	
	return similar
}

// addToBucketsWithCache adds to buckets with caching
func (c *CachedMinHashLSH) addToBucketsWithCache(docID string, mh []uint64) {
	for band := 0; band < c.bands; band++ {
		sig := c.getBandSignature(mh, band)
		c.buckets[sig] = append(c.buckets[sig], docID)
	}
}

// getSignatureKey creates a cache key for text signatures
func (c *CachedMinHashLSH) getSignatureKey(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("sig:%x", hash)
}

// GetCacheStats returns cache performance statistics
func (c *CachedMinHashLSH) GetCacheStats() CachePerformanceStats {
	total := c.cacheHits + c.cacheMisses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.cacheHits) / float64(total)
	}
	
	return CachePerformanceStats{
		CacheHits:       c.cacheHits,
		CacheMisses:     c.cacheMisses,
		HitRate:         hitRate,
		SignatureCache:  c.signatureCache.Stats(),
		SimilarityCache: c.similarityCache.Stats(),
		BucketCache:     c.bucketCache.Stats(),
	}
}

// ClearCache clears all caches
func (c *CachedMinHashLSH) ClearCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.signatureCache.Clear()
	c.similarityCache.Clear()
	c.bucketCache.Clear()
	c.cacheHits = 0
	c.cacheMisses = 0
}

// CachePerformanceStats represents cache performance metrics
type CachePerformanceStats struct {
	CacheHits       int64             `json:"cache_hits"`
	CacheMisses     int64             `json:"cache_misses"`
	HitRate         float64           `json:"hit_rate"`
	SignatureCache  cache.CacheStats  `json:"signature_cache"`
	SimilarityCache cache.CacheStats  `json:"similarity_cache"`
	BucketCache     cache.CacheStats  `json:"bucket_cache"`
}

// OptimizedSimHashLSH extends SimHashLSH with optimizations
type OptimizedSimHashLSH struct {
	*SimHashLSH
	featureCache    *cache.LRUCache
	similarityCache *cache.SimilarityCache
	mutex           sync.RWMutex
	
	// Performance metrics
	cacheHits   int64
	cacheMisses int64
}

// NewOptimizedSimHashLSH creates a new optimized SimHash LSH instance
func NewOptimizedSimHashLSH(threshold, numTables int, cacheSize int) *OptimizedSimHashLSH {
	return &OptimizedSimHashLSH{
		SimHashLSH:      NewSimHashLSH(threshold, numTables),
		featureCache:    cache.NewLRUCache(cacheSize, 30*time.Minute),
		similarityCache: cache.NewSimilarityCache(cacheSize*2, 15*time.Minute),
	}
}

// Add adds a document with feature caching
func (o *OptimizedSimHashLSH) Add(docID, text string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	
	// Check feature cache
	featureKey := o.getFeatureKey(text)
	var features map[string]int
	
	if cachedFeatures, exists := o.featureCache.Get(featureKey); exists {
		o.cacheHits++
		features = cachedFeatures.(map[string]int)
	} else {
		o.cacheMisses++
		features = o.extractFeatures(text)
		o.featureCache.Set(featureKey, features)
	}
	
	hash := o.computeSimHash(features)
	
	doc := SimHashDoc{
		ID:       docID,
		Text:     text,
		Hash:     hash,
		Features: features,
	}
	
	o.docs[docID] = doc
	o.addToLSHBuckets(docID, hash)
}

// FindSimilar finds similar documents with caching
func (o *OptimizedSimHashLSH) FindSimilar(docID string) []SimilarDocument {
	o.mutex.RLock()
	doc, exists := o.docs[docID]
	o.mutex.RUnlock()
	
	if !exists {
		return nil
	}
	
	candidates := make(map[string]bool)
	
	// Get candidates from LSH buckets
	o.mutex.RLock()
	for table := 0; table < o.numTables; table++ {
		bucketKey := o.getBucketKey(doc.Hash, table, o.threshold)
		if docs, exists := o.buckets[bucketKey]; exists {
			for _, candidateID := range docs {
				if candidateID != docID {
					candidates[candidateID] = true
				}
			}
		}
	}
	o.mutex.RUnlock()
	
	// Calculate similarities with caching
	var similar []SimilarDocument
	for candidateID := range candidates {
		// Check similarity cache
		if cachedSim, exists := o.similarityCache.GetSimilarity(docID, candidateID); exists {
			o.cacheHits++
			distance := int((1.0 - cachedSim) * float64(o.hashLength))
			if distance <= o.threshold {
				similar = append(similar, SimilarDocument{
					ID:         candidateID,
					Text:       o.docs[candidateID].Text,
					Similarity: cachedSim,
				})
			}
			continue
		}
		
		o.cacheMisses++
		candidate := o.docs[candidateID]
		distance := o.hammingDistance(doc.Hash, candidate.Hash)
		
		if distance <= o.threshold {
			similarity := 1.0 - float64(distance)/float64(o.hashLength)
			o.similarityCache.SetSimilarity(docID, candidateID, similarity)
			
			similar = append(similar, SimilarDocument{
				ID:         candidateID,
				Text:       candidate.Text,
				Similarity: similarity,
			})
		}
	}
	
	return similar
}

// getFeatureKey creates a cache key for text features
func (o *OptimizedSimHashLSH) getFeatureKey(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("feat:%x", hash)
}

// GetCacheStats returns cache performance statistics
func (o *OptimizedSimHashLSH) GetCacheStats() CachePerformanceStats {
	total := o.cacheHits + o.cacheMisses
	var hitRate float64
	if total > 0 {
		hitRate = float64(o.cacheHits) / float64(total)
	}
	
	return CachePerformanceStats{
		CacheHits:       o.cacheHits,
		CacheMisses:     o.cacheMisses,
		HitRate:         hitRate,
		SignatureCache:  o.featureCache.Stats(),
		SimilarityCache: o.similarityCache.Stats(),
	}
}

// ClearCache clears all caches
func (o *OptimizedSimHashLSH) ClearCache() {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	
	o.featureCache.Clear()
	o.similarityCache.Clear()
	o.cacheHits = 0
	o.cacheMisses = 0
}
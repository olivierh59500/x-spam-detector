package cache

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntry represents an entry in the LRU cache
type CacheEntry struct {
	Key        string
	Value      interface{}
	Expiry     time.Time
	AccessTime time.Time
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return !e.Expiry.IsZero() && time.Now().After(e.Expiry)
}

// LRUCache implements a thread-safe Least Recently Used cache
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	order    *list.List
	mutex    sync.RWMutex
	ttl      time.Duration
	
	// Statistics
	hits   int64
	misses int64
}

// NewLRUCache creates a new LRU cache with the specified capacity and TTL
func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		order:    list.New(),
		ttl:      ttl,
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	elem, exists := c.cache[key]
	if !exists {
		c.misses++
		return nil, false
	}
	
	entry := elem.Value.(*CacheEntry)
	if entry.IsExpired() {
		c.removeElement(elem)
		c.misses++
		return nil, false
	}
	
	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	entry.AccessTime = time.Now()
	c.hits++
	
	return entry.Value, true
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	var expiry time.Time
	if c.ttl > 0 {
		expiry = time.Now().Add(c.ttl)
	}
	
	if elem, exists := c.cache[key]; exists {
		// Update existing entry
		entry := elem.Value.(*CacheEntry)
		entry.Value = value
		entry.Expiry = expiry
		entry.AccessTime = time.Now()
		c.order.MoveToFront(elem)
		return
	}
	
	// Add new entry
	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Expiry:     expiry,
		AccessTime: time.Now(),
	}
	
	elem := c.order.PushFront(entry)
	c.cache[key] = elem
	
	// Evict if over capacity
	if c.order.Len() > c.capacity {
		c.evictLRU()
	}
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if elem, exists := c.cache[key]; exists {
		c.removeElement(elem)
		return true
	}
	return false
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.cache = make(map[string]*list.Element)
	c.order.Init()
	c.hits = 0
	c.misses = 0
}

// Size returns the current number of entries in the cache
func (c *LRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.order.Len()
}

// Stats returns cache statistics
func (c *LRUCache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	
	return CacheStats{
		Hits:     c.hits,
		Misses:   c.misses,
		HitRate:  hitRate,
		Size:     c.order.Len(),
		Capacity: c.capacity,
	}
}

// CleanExpired removes expired entries from the cache
func (c *LRUCache) CleanExpired() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	cleaned := 0
	for elem := c.order.Back(); elem != nil; {
		entry := elem.Value.(*CacheEntry)
		if entry.IsExpired() {
			prev := elem.Prev()
			c.removeElement(elem)
			cleaned++
			elem = prev
		} else {
			elem = elem.Prev()
		}
	}
	
	return cleaned
}

// removeElement removes an element from both the list and map
func (c *LRUCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	delete(c.cache, entry.Key)
	c.order.Remove(elem)
}

// evictLRU removes the least recently used entry
func (c *LRUCache) evictLRU() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	Hits     int64   `json:"hits"`
	Misses   int64   `json:"misses"`
	HitRate  float64 `json:"hit_rate"`
	Size     int     `json:"size"`
	Capacity int     `json:"capacity"`
}

// SimilarityCache is a specialized cache for similarity calculations
type SimilarityCache struct {
	cache *LRUCache
}

// NewSimilarityCache creates a new similarity cache
func NewSimilarityCache(capacity int, ttl time.Duration) *SimilarityCache {
	return &SimilarityCache{
		cache: NewLRUCache(capacity, ttl),
	}
}

// GetSimilarity retrieves a cached similarity score
func (sc *SimilarityCache) GetSimilarity(docID1, docID2 string) (float64, bool) {
	key := sc.makeKey(docID1, docID2)
	if value, exists := sc.cache.Get(key); exists {
		return value.(float64), true
	}
	return 0.0, false
}

// SetSimilarity caches a similarity score
func (sc *SimilarityCache) SetSimilarity(docID1, docID2 string, similarity float64) {
	key := sc.makeKey(docID1, docID2)
	sc.cache.Set(key, similarity)
}

// makeKey creates a consistent cache key for two document IDs
func (sc *SimilarityCache) makeKey(docID1, docID2 string) string {
	if docID1 < docID2 {
		return docID1 + ":" + docID2
	}
	return docID2 + ":" + docID1
}

// Stats returns cache statistics
func (sc *SimilarityCache) Stats() CacheStats {
	return sc.cache.Stats()
}

// Clear clears the similarity cache
func (sc *SimilarityCache) Clear() {
	sc.cache.Clear()
}
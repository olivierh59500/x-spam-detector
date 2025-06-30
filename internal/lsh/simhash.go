package lsh

import (
	"crypto/md5"
	"fmt"
	"math/bits"
	"regexp"
	"strings"
	"unicode"
)

// SimHashLSH implements Locality Sensitive Hashing using SimHash for spam detection
type SimHashLSH struct {
	docs       map[string]SimHashDoc
	threshold  int // Hamming distance threshold
	buckets    map[string][]string // bucket -> document IDs
	numTables  int
	hashLength int
}

// SimHashDoc represents a document with its SimHash signature
type SimHashDoc struct {
	ID       string
	Text     string
	Hash     uint64
	Features map[string]int
}

// NewSimHashLSH creates a new SimHash LSH instance
func NewSimHashLSH(threshold, numTables int) *SimHashLSH {
	return &SimHashLSH{
		docs:       make(map[string]SimHashDoc),
		threshold:  threshold,
		buckets:    make(map[string][]string),
		numTables:  numTables,
		hashLength: 64,
	}
}

// Add adds a document to the SimHash LSH index
func (lsh *SimHashLSH) Add(docID, text string) {
	features := lsh.extractFeatures(text)
	hash := lsh.computeSimHash(features)
	
	doc := SimHashDoc{
		ID:       docID,
		Text:     text,
		Hash:     hash,
		Features: features,
	}
	
	lsh.docs[docID] = doc
	lsh.addToLSHBuckets(docID, hash)
}

// FindSimilar finds documents similar to the given document using LSH
func (lsh *SimHashLSH) FindSimilar(docID string) []SimilarDocument {
	doc, exists := lsh.docs[docID]
	if !exists {
		return nil
	}

	candidates := make(map[string]bool)
	
	// Get candidates from LSH buckets
	for table := 0; table < lsh.numTables; table++ {
		bucketKey := lsh.getBucketKey(doc.Hash, table, lsh.threshold)
		if docs, exists := lsh.buckets[bucketKey]; exists {
			for _, candidateID := range docs {
				if candidateID != docID {
					candidates[candidateID] = true
				}
			}
		}
	}

	// Calculate exact Hamming distances for candidates
	var similar []SimilarDocument
	for candidateID := range candidates {
		candidate := lsh.docs[candidateID]
		distance := lsh.hammingDistance(doc.Hash, candidate.Hash)
		
		if distance <= lsh.threshold {
			similarity := 1.0 - float64(distance)/float64(lsh.hashLength)
			similar = append(similar, SimilarDocument{
				ID:         candidateID,
				Text:       candidate.Text,
				Similarity: similarity,
			})
		}
	}

	return similar
}

// GetAllSimilarClusters returns all clusters of similar documents
func (lsh *SimHashLSH) GetAllSimilarClusters() [][]SimilarDocument {
	processed := make(map[string]bool)
	var clusters [][]SimilarDocument

	for docID, doc := range lsh.docs {
		if processed[docID] {
			continue
		}

		cluster := []SimilarDocument{{
			ID:         docID,
			Text:       doc.Text,
			Similarity: 1.0,
		}}

		similar := lsh.FindSimilar(docID)
		for _, sim := range similar {
			if !processed[sim.ID] {
				cluster = append(cluster, sim)
				processed[sim.ID] = true
			}
		}

		if len(cluster) > 1 {
			clusters = append(clusters, cluster)
		}
		processed[docID] = true
	}

	return clusters
}

// extractFeatures extracts weighted features from text
func (lsh *SimHashLSH) extractFeatures(text string) map[string]int {
	features := make(map[string]int)
	
	// Normalize text
	text = strings.ToLower(text)
	
	// Remove URLs but keep their presence as a feature
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	urls := urlRegex.FindAllString(text, -1)
	if len(urls) > 0 {
		features["__HAS_URL__"] = len(urls)
	}
	text = urlRegex.ReplaceAllString(text, " ")
	
	// Extract mentions and hashtags as features
	mentionRegex := regexp.MustCompile(`@([^\s]+)`)
	mentions := mentionRegex.FindAllStringSubmatch(text, -1)
	for _, mention := range mentions {
		features["@"+mention[1]] = 1
	}
	
	hashtagRegex := regexp.MustCompile(`#([^\s]+)`)
	hashtags := hashtagRegex.FindAllStringSubmatch(text, -1)
	for _, hashtag := range hashtags {
		features["#"+hashtag[1]] = 1
	}
	
	// Remove mentions and hashtags from text for word extraction
	text = mentionRegex.ReplaceAllString(text, " ")
	text = hashtagRegex.ReplaceAllString(text, " ")
	
	// Extract words
	words := lsh.tokenizeWords(text)
	
	// Add word features with frequency
	for _, word := range words {
		if len(word) >= 3 {
			features[word]++
		}
	}
	
	// Add n-gram features
	for i := 0; i < len(words)-1; i++ {
		if len(words[i]) >= 3 && len(words[i+1]) >= 3 {
			bigram := words[i] + " " + words[i+1]
			features[bigram]++
		}
	}
	
	// Add character-level features for better spam detection
	for i := 0; i < len(text)-2; i++ {
		if unicode.IsLetter(rune(text[i])) && unicode.IsLetter(rune(text[i+1])) && unicode.IsLetter(rune(text[i+2])) {
			trigram := text[i : i+3]
			features["__CHAR__"+trigram]++
		}
	}
	
	return features
}

// tokenizeWords splits text into words
func (lsh *SimHashLSH) tokenizeWords(text string) []string {
	var words []string
	word := strings.Builder{}
	
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else if word.Len() > 0 {
			words = append(words, word.String())
			word.Reset()
		}
	}
	
	if word.Len() > 0 {
		words = append(words, word.String())
	}
	
	return words
}

// computeSimHash computes the SimHash signature for given features
func (lsh *SimHashLSH) computeSimHash(features map[string]int) uint64 {
	v := make([]int, lsh.hashLength)
	
	for feature, weight := range features {
		// Hash the feature
		hash := lsh.hashFeature(feature)
		
		// Update the vector based on hash bits
		for i := 0; i < lsh.hashLength; i++ {
			bit := (hash >> uint(i)) & 1
			if bit == 1 {
				v[i] += weight
			} else {
				v[i] -= weight
			}
		}
	}
	
	// Convert vector to final hash
	var simhash uint64
	for i := 0; i < lsh.hashLength; i++ {
		if v[i] > 0 {
			simhash |= 1 << uint(i)
		}
	}
	
	return simhash
}

// hashFeature hashes a feature string to a 64-bit integer
func (lsh *SimHashLSH) hashFeature(feature string) uint64 {
	hash := md5.Sum([]byte(feature))
	
	// Convert first 8 bytes of MD5 hash to uint64
	var result uint64
	for i := 0; i < 8; i++ {
		result = (result << 8) | uint64(hash[i])
	}
	
	return result
}

// hammingDistance calculates the Hamming distance between two hashes
func (lsh *SimHashLSH) hammingDistance(hash1, hash2 uint64) int {
	return bits.OnesCount64(hash1 ^ hash2)
}

// addToLSHBuckets adds a document to LSH buckets for fast similarity search
func (lsh *SimHashLSH) addToLSHBuckets(docID string, hash uint64) {
	for table := 0; table < lsh.numTables; table++ {
		bucketKey := lsh.getBucketKey(hash, table, lsh.threshold)
		lsh.buckets[bucketKey] = append(lsh.buckets[bucketKey], docID)
	}
}

// getBucketKey generates a bucket key for LSH table
func (lsh *SimHashLSH) getBucketKey(hash uint64, table, threshold int) string {
	// For SimHash LSH, we create multiple projections by selecting different bit positions
	// This implements the "bit sampling" approach for SimHash LSH
	
	selectedBits := make([]byte, 0)
	bitsPerTable := lsh.hashLength / lsh.numTables
	
	start := table * bitsPerTable
	end := start + bitsPerTable
	if end > lsh.hashLength {
		end = lsh.hashLength
	}
	
	for i := start; i < end; i++ {
		bit := (hash >> uint(i)) & 1
		selectedBits = append(selectedBits, byte(bit))
	}
	
	return fmt.Sprintf("table_%d_%s", table, string(selectedBits))
}

// GetStatistics returns statistics about the SimHash LSH index
func (lsh *SimHashLSH) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	
	stats["total_documents"] = len(lsh.docs)
	stats["total_buckets"] = len(lsh.buckets)
	stats["threshold"] = lsh.threshold
	stats["num_tables"] = lsh.numTables
	
	// Calculate bucket distribution
	bucketSizes := make([]int, 0)
	for _, docs := range lsh.buckets {
		bucketSizes = append(bucketSizes, len(docs))
	}
	
	if len(bucketSizes) > 0 {
		sum := 0
		max := 0
		for _, size := range bucketSizes {
			sum += size
			if size > max {
				max = size
			}
		}
		stats["avg_bucket_size"] = float64(sum) / float64(len(bucketSizes))
		stats["max_bucket_size"] = max
	}
	
	return stats
}

// Clear removes all documents from the index
func (lsh *SimHashLSH) Clear() {
	lsh.docs = make(map[string]SimHashDoc)
	lsh.buckets = make(map[string][]string)
}

// GetDocumentCount returns the number of documents in the index
func (lsh *SimHashLSH) GetDocumentCount() int {
	return len(lsh.docs)
}

// GetDocument returns a document by ID
func (lsh *SimHashLSH) GetDocument(docID string) (SimHashDoc, bool) {
	doc, exists := lsh.docs[docID]
	return doc, exists
}

// CalculateExactSimilarity calculates exact similarity between two documents
func (lsh *SimHashLSH) CalculateExactSimilarity(docID1, docID2 string) float64 {
	doc1, exists1 := lsh.docs[docID1]
	doc2, exists2 := lsh.docs[docID2]
	
	if !exists1 || !exists2 {
		return 0.0
	}
	
	distance := lsh.hammingDistance(doc1.Hash, doc2.Hash)
	return 1.0 - float64(distance)/float64(lsh.hashLength)
}
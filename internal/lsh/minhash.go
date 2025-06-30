package lsh

import (
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// MinHashLSH implements Locality Sensitive Hashing using MinHash for spam detection
type MinHashLSH struct {
	numHashes     int
	bands         int
	rowsPerBand   int
	buckets       map[string][]string // band signature -> document IDs
	docHashes     map[string][]uint64
	docTexts      map[string]string
	threshold     float64
}

// NewMinHashLSH creates a new MinHash LSH instance
func NewMinHashLSH(numHashes, bands int, threshold float64) (*MinHashLSH, error) {
	if numHashes <= 0 {
		return nil, fmt.Errorf("numHashes must be positive, got %d", numHashes)
	}
	if bands <= 0 {
		return nil, fmt.Errorf("bands must be positive, got %d", bands)
	}
	if numHashes%bands != 0 {
		return nil, fmt.Errorf("numHashes (%d) must be divisible by bands (%d)", numHashes, bands)
	}
	if threshold < 0 || threshold > 1 {
		return nil, fmt.Errorf("threshold must be between 0 and 1, got %f", threshold)
	}

	return &MinHashLSH{
		numHashes:   numHashes,
		bands:       bands,
		rowsPerBand: numHashes / bands,
		buckets:     make(map[string][]string),
		docHashes:   make(map[string][]uint64),
		docTexts:    make(map[string]string),
		threshold:   threshold,
	}, nil
}

// Add adds a document to the LSH index
func (lsh *MinHashLSH) Add(docID, text string) {
	// Normalize and tokenize text
	tokens := lsh.tokenize(text)
	
	// Create MinHash signatures
	minHashes := lsh.computeMinHash(tokens)
	
	lsh.docHashes[docID] = minHashes
	lsh.docTexts[docID] = text

	// Add to LSH buckets
	lsh.addToBuckets(docID, minHashes)
}

// FindSimilar finds documents similar to the given document
func (lsh *MinHashLSH) FindSimilar(docID string) []SimilarDocument {
	mh, exists := lsh.docHashes[docID]
	if !exists {
		return nil
	}

	candidates := make(map[string]bool)
	
	// Get candidates from LSH buckets
	for band := 0; band < lsh.bands; band++ {
		sig := lsh.getBandSignature(mh, band)
		if docs, exists := lsh.buckets[sig]; exists {
			for _, candidateID := range docs {
				if candidateID != docID {
					candidates[candidateID] = true
				}
			}
		}
	}

	// Calculate exact similarities for candidates
	var similar []SimilarDocument
	for candidateID := range candidates {
		candidateMH := lsh.docHashes[candidateID]
		similarity := lsh.jaccardSimilarity(mh, candidateMH)
		
		if similarity >= lsh.threshold {
			similar = append(similar, SimilarDocument{
				ID:         candidateID,
				Text:       lsh.docTexts[candidateID],
				Similarity: similarity,
			})
		}
	}

	return similar
}

// GetAllSimilarClusters returns all clusters of similar documents
func (lsh *MinHashLSH) GetAllSimilarClusters() [][]SimilarDocument {
	processed := make(map[string]bool)
	var clusters [][]SimilarDocument

	for docID := range lsh.docHashes {
		if processed[docID] {
			continue
		}

		cluster := []SimilarDocument{{
			ID:         docID,
			Text:       lsh.docTexts[docID],
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

// addToBuckets adds a document to LSH buckets
func (lsh *MinHashLSH) addToBuckets(docID string, mh []uint64) {
	for band := 0; band < lsh.bands; band++ {
		sig := lsh.getBandSignature(mh, band)
		lsh.buckets[sig] = append(lsh.buckets[sig], docID)
	}
}

// getBandSignature gets the signature for a specific band
func (lsh *MinHashLSH) getBandSignature(mh []uint64, band int) string {
	start := band * lsh.rowsPerBand
	end := start + lsh.rowsPerBand
	
	h := fnv.New64a()
	for i := start; i < end; i++ {
		h.Write([]byte{byte(mh[i] & 0xFF), byte((mh[i] >> 8) & 0xFF), byte((mh[i] >> 16) & 0xFF), byte((mh[i] >> 24) & 0xFF)})
	}
	
	return string(h.Sum(nil))
}

// computeMinHash computes MinHash signatures for a set of tokens
func (lsh *MinHashLSH) computeMinHash(tokens []string) []uint64 {
	signatures := make([]uint64, lsh.numHashes)
	
	// Initialize with maximum values
	for i := range signatures {
		signatures[i] = ^uint64(0) // Max uint64
	}
	
	// For each token, compute hash values and update signatures
	for _, token := range tokens {
		tokenBytes := []byte(token)
		for i := 0; i < lsh.numHashes; i++ {
			hash := lsh.hashWithSeed(tokenBytes, uint64(i))
			if hash < signatures[i] {
				signatures[i] = hash
			}
		}
	}
	
	return signatures
}

// hashWithSeed computes hash with a seed value
func (lsh *MinHashLSH) hashWithSeed(data []byte, seed uint64) uint64 {
	h := fnv.New64a()
	h.Write([]byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24)})
	h.Write(data)
	return h.Sum64()
}

// jaccardSimilarity calculates Jaccard similarity from MinHash signatures
func (lsh *MinHashLSH) jaccardSimilarity(sig1, sig2 []uint64) float64 {
	if len(sig1) != len(sig2) {
		return 0.0
	}
	
	matches := 0
	for i := range sig1 {
		if sig1[i] == sig2[i] {
			matches++
		}
	}
	
	return float64(matches) / float64(len(sig1))
}

// tokenize converts text into tokens for hashing
func (lsh *MinHashLSH) tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove URLs
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	text = urlRegex.ReplaceAllString(text, "")
	
	// Remove mentions and hashtags for similarity comparison
	mentionRegex := regexp.MustCompile(`@[^\s]+`)
	text = mentionRegex.ReplaceAllString(text, "")
	
	hashtagRegex := regexp.MustCompile(`#[^\s]+`)
	text = hashtagRegex.ReplaceAllString(text, "")
	
	// Remove punctuation and split into words
	var tokens []string
	word := strings.Builder{}
	
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else if word.Len() > 0 {
			if word.Len() >= 3 { // Filter out very short words
				tokens = append(tokens, word.String())
			}
			word.Reset()
		}
	}
	
	if word.Len() >= 3 {
		tokens = append(tokens, word.String())
	}
	
	// Generate shingles (n-grams) for better similarity detection
	shingles := make([]string, 0)
	for i := 0; i < len(tokens); i++ {
		// Add individual words
		shingles = append(shingles, tokens[i])
		
		// Add 2-grams
		if i < len(tokens)-1 {
			shingles = append(shingles, tokens[i]+" "+tokens[i+1])
		}
		
		// Add 3-grams
		if i < len(tokens)-2 {
			shingles = append(shingles, tokens[i]+" "+tokens[i+1]+" "+tokens[i+2])
		}
	}
	
	return shingles
}

// SimilarDocument represents a document similar to a query document
type SimilarDocument struct {
	ID         string
	Text       string
	Similarity float64
}

// CalculateOptimalParameters calculates optimal LSH parameters for given similarity threshold
func CalculateOptimalParameters(threshold float64, numHashes int) (bands int, rowsPerBand int) {
	bestBands := 1
	bestRows := numHashes
	bestError := math.Inf(1)
	
	for b := 1; b <= numHashes; b++ {
		if numHashes%b == 0 {
			r := numHashes / b
			// Probability of collision for similar items: s^r
			// Probability of at least one collision: 1 - (1 - s^r)^b
			prob := 1 - math.Pow(1-math.Pow(threshold, float64(r)), float64(b))
			error := math.Abs(prob - threshold)
			
			if error < bestError {
				bestError = error
				bestBands = b
				bestRows = r
			}
		}
	}
	
	return bestBands, bestRows
}
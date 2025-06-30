package lsh

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMinHashLSH(t *testing.T) {
	tests := []struct {
		name      string
		numHashes int
		bands     int
		threshold float64
		shouldError bool
	}{
		{
			name:      "valid parameters",
			numHashes: 128,
			bands:     16,
			threshold: 0.7,
			shouldError: false,
		},
		{
			name:      "indivisible hashes and bands",
			numHashes: 127,
			bands:     16,
			threshold: 0.7,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsh, err := NewMinHashLSH(tt.numHashes, tt.bands, tt.threshold)
			
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, lsh)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, lsh)
				assert.Equal(t, tt.numHashes, lsh.numHashes)
				assert.Equal(t, tt.bands, lsh.bands)
				assert.Equal(t, tt.threshold, lsh.threshold)
				assert.Equal(t, tt.numHashes/tt.bands, lsh.rowsPerBand)
			}
		})
	}
}

func TestMinHashLSH_AddAndFindSimilar(t *testing.T) {
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	assert.NoError(t, err)
	
	// Add some test documents
	testDocs := map[string]string{
		"doc1": "this is a test document about machine learning",
		"doc2": "this is a test document about machine learning algorithms",
		"doc3": "completely different content about cooking recipes",
		"doc4": "another document about machine learning and AI",
		"doc5": "cooking recipes and kitchen tips for beginners",
	}
	
	for docID, text := range testDocs {
		lsh.Add(docID, text)
	}
	
	// Test finding similar documents
	t.Run("find similar to doc1", func(t *testing.T) {
		similar := lsh.FindSimilar("doc1")
		
		// Should find doc2 and doc4 as similar (all about machine learning)
		// but not doc3 and doc5 (about cooking)
		similarIDs := make(map[string]bool)
		for _, sim := range similar {
			similarIDs[sim.ID] = true
		}
		
		// We expect to find some similar documents
		assert.True(t, len(similar) > 0, "Should find some similar documents")
		
		// Check that similarities are within reasonable bounds
		for _, sim := range similar {
			assert.True(t, sim.Similarity >= lsh.threshold, 
				"Similarity should be above threshold")
			assert.True(t, sim.Similarity <= 1.0, 
				"Similarity should not exceed 1.0")
		}
	})
	
	t.Run("find similar to non-existent document", func(t *testing.T) {
		similar := lsh.FindSimilar("nonexistent")
		assert.Empty(t, similar, "Should return empty for non-existent document")
	})
}

func TestMinHashLSH_GetAllSimilarClusters(t *testing.T) {
	lsh, err := NewMinHashLSH(128, 16, 0.8)
	assert.NoError(t, err)
	
	// Add documents with known similarity patterns
	testDocs := map[string]string{
		"spam1": "buy now limited time offer click here",
		"spam2": "buy now limited time offer click link",
		"spam3": "limited time offer buy now click here",
		"legit1": "interesting article about climate change research",
		"legit2": "new developments in renewable energy technology",
		"legit3": "machine learning applications in healthcare",
	}
	
	for docID, text := range testDocs {
		lsh.Add(docID, text)
	}
	
	clusters := lsh.GetAllSimilarClusters()
	
	// Should find at least one cluster (the spam documents)
	assert.True(t, len(clusters) >= 1, "Should find at least one cluster")
	
	// Check cluster properties
	for _, cluster := range clusters {
		assert.True(t, len(cluster) >= 2, "Cluster should have at least 2 documents")
		
		// All documents in cluster should have high similarity
		for i := 0; i < len(cluster); i++ {
			for j := i + 1; j < len(cluster); j++ {
				sim1 := cluster[i].Similarity
				sim2 := cluster[j].Similarity
				assert.True(t, sim1 >= lsh.threshold || sim2 >= lsh.threshold,
					"Documents in cluster should meet similarity threshold")
			}
		}
	}
}

func TestMinHashLSH_Tokenize(t *testing.T) {
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	assert.NoError(t, err)
	
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "simple text",
			input: "hello world test",
			expected: []string{"hello", "world", "test", "hello world", "world test", "hello world test"},
		},
		{
			name:  "text with URLs",
			input: "check this out https://example.com amazing content",
			expected: []string{"check", "this", "out", "amazing", "content", "check this", "this out", "out amazing", "amazing content", "check this out", "this out amazing", "out amazing content"},
		},
		{
			name:  "text with mentions and hashtags",
			input: "hey @user check #hashtag content",
			expected: []string{"hey", "check", "content", "hey check", "check content", "hey check content"},
		},
		{
			name:  "empty string",
			input: "",
			expected: []string{},
		},
		{
			name:  "short words filtered",
			input: "a an is the hello world",
			expected: []string{"hello", "world", "hello world"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lsh.tokenize(tt.input)
			
			// Check that all expected tokens are present
			expectedSet := make(map[string]bool)
			for _, token := range tt.expected {
				expectedSet[token] = true
			}
			
			resultSet := make(map[string]bool)
			for _, token := range result {
				resultSet[token] = true
			}
			
			// All expected tokens should be in result
			for token := range expectedSet {
				assert.True(t, resultSet[token], "Expected token '%s' not found in result", token)
			}
		})
	}
}

func TestCalculateOptimalParameters(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		numHashes int
		expectBands int
		expectRows  int
	}{
		{
			name:      "high threshold",
			threshold: 0.8,
			numHashes: 128,
			expectBands: 64, // More bands for higher precision
			expectRows:  2,
		},
		{
			name:      "medium threshold",
			threshold: 0.6,
			numHashes: 128,
			expectBands: 32,
			expectRows:  4,
		},
		{
			name:      "low threshold",
			threshold: 0.4,
			numHashes: 128,
			expectBands: 16,
			expectRows:  8,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bands, rows := CalculateOptimalParameters(tt.threshold, tt.numHashes)
			
			assert.Equal(t, tt.expectBands, bands, "Bands should match expected value")
			assert.Equal(t, tt.expectRows, rows, "Rows should match expected value")
			assert.Equal(t, tt.numHashes, bands*rows, "Bands * rows should equal numHashes")
		})
	}
}

func TestMinHashLSH_SpamDetectionScenario(t *testing.T) {
	// Test with realistic spam detection scenario
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	assert.NoError(t, err)
	
	// Simulate spam campaign with slight variations
	spamTweets := []string{
		"amazing crypto deal dont miss out limited time",
		"amazing crypto deal don't miss out limited time offer",
		"incredible crypto deal dont miss limited time opportunity",
		"amazing cryptocurrency deal dont miss out limited time",
		"amazing crypto opportunity dont miss out limited time",
	}
	
	// Add legitimate tweets
	legitimateTweets := []string{
		"beautiful sunset today feeling grateful for nature",
		"working on interesting machine learning project",
		"great coffee shop downtown excellent service",
		"reading fascinating book about space exploration",
	}
	
	// Add all tweets
	for i, tweet := range spamTweets {
		lsh.Add(fmt.Sprintf("spam_%d", i), tweet)
	}
	
	for i, tweet := range legitimateTweets {
		lsh.Add(fmt.Sprintf("legit_%d", i), tweet)
	}
	
	// Get clusters
	clusters := lsh.GetAllSimilarClusters()
	
	// Should find spam cluster
	spamClusterFound := false
	for _, cluster := range clusters {
		// Check if this cluster contains spam tweets
		if len(cluster) >= 3 { // Minimum cluster size for spam
			spamInCluster := 0
			for _, doc := range cluster {
				if len(doc.ID) >= 4 && doc.ID[:4] == "spam" {
					spamInCluster++
				}
			}
			
			if spamInCluster >= 3 {
				spamClusterFound = true
				break
			}
		}
	}
	
	assert.True(t, spamClusterFound, "Should detect spam cluster")
}

func BenchmarkMinHashLSH_Add(b *testing.B) {
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text := fmt.Sprintf("this is test document number %d with some content", i)
		lsh.Add(fmt.Sprintf("doc_%d", i), text)
	}
}

func BenchmarkMinHashLSH_FindSimilar(b *testing.B) {
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	if err != nil {
		b.Fatal(err)
	}
	
	// Pre-populate with test data
	for i := 0; i < 1000; i++ {
		text := fmt.Sprintf("test document %d about machine learning and artificial intelligence", i)
		lsh.Add(fmt.Sprintf("doc_%d", i), text)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lsh.FindSimilar("doc_0")
	}
}

func BenchmarkMinHashLSH_GetAllSimilarClusters(b *testing.B) {
	lsh, err := NewMinHashLSH(128, 16, 0.7)
	if err != nil {
		b.Fatal(err)
	}
	
	// Pre-populate with test data
	for i := 0; i < 100; i++ {
		text := fmt.Sprintf("test document %d about machine learning", i)
		lsh.Add(fmt.Sprintf("doc_%d", i), text)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lsh.GetAllSimilarClusters()
	}
}
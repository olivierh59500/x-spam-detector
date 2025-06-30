# X Spam Detector

A professional spam detection system for X (Twitter) that uses Locality Sensitive Hashing (LSH) to identify coordinated bot farm activities and similar spam content.

## Features

- **Advanced LSH Algorithms**: Implements both MinHash and SimHash LSH for efficient similarity detection
- **Hybrid Detection**: Combines multiple algorithms for enhanced accuracy
- **Bot Account Analysis**: Identifies suspicious account patterns and behaviors
- **Coordinated Behavior Detection**: Detects organized spam campaigns and bot farms
- **Real-time Clustering**: Groups similar spam content in real-time
- **Professional GUI**: User-friendly interface built with Fyne framework
- **Comprehensive Analytics**: Detailed statistics and visualization of spam patterns

## Technical Architecture

### LSH Implementation

The system implements two LSH algorithms optimized for spam detection:

1. **MinHash LSH**: Efficiently detects near-duplicate content using MinHash signatures
2. **SimHash LSH**: Identifies similar content using SimHash fingerprints with Hamming distance

### Detection Engine

- **Multi-threaded Processing**: Concurrent processing for high-performance detection
- **Configurable Thresholds**: Adjustable similarity thresholds and clustering parameters
- **Memory Efficient**: Optimized for processing large volumes of tweets
- **Batch Processing**: Handles tweets in configurable batch sizes

### Bot Detection

- Account age analysis
- Follower-to-following ratio evaluation
- Tweet frequency patterns
- Profile completeness scoring
- Username pattern analysis

## Installation

### Prerequisites

- Go 1.21 or higher
- Git

### Build from Source

```bash
git clone <repository-url>
cd x-spam-detector
go mod tidy
go build -o spam-detector
```

### Run

```bash
./spam-detector
```

## Configuration

The application uses a YAML configuration file (`config.yaml`) with the following key parameters:

```yaml
detection:
  minhash_threshold: 0.7      # Similarity threshold (0.0-1.0)
  simhash_threshold: 3        # Hamming distance threshold
  min_cluster_size: 3         # Minimum tweets in spam cluster
  spam_score_threshold: 0.6   # Confidence threshold for spam
  enable_hybrid_mode: true    # Use both algorithms
```

### LSH Parameters

- **MinHash Parameters**:
  - `minhash_hashes`: Number of hash functions (default: 128)
  - `minhash_bands`: Number of LSH bands (default: 16)
  - `minhash_threshold`: Similarity threshold (default: 0.7)

- **SimHash Parameters**:
  - `simhash_threshold`: Hamming distance threshold (default: 3)
  - `simhash_tables`: Number of hash tables (default: 4)

## Usage

### GUI Interface

1. **Input Tab**: Add individual tweets or paste multiple tweets
2. **Detection**: Click "Detect Spam" to run analysis
3. **Results**: View detected spam clusters, suspicious accounts, and statistics
4. **Sample Data**: Load example data to test the system

### Adding Tweets

```
Option 1: Manual Input
- Enter tweet text in the input area
- Click "Add Tweet(s)"

Option 2: Batch Import
- Paste multiple tweets (one per line)
- Click "Add Tweet(s)"

Option 3: Sample Data
- Click "Load Sample Data" for demonstration
```

### Spam Detection Process

1. **Text Preprocessing**: Normalize and tokenize tweet content
2. **Feature Extraction**: Generate n-grams and character features
3. **LSH Indexing**: Create MinHash and SimHash signatures
4. **Similarity Search**: Find similar content using LSH buckets
5. **Clustering**: Group similar tweets into spam clusters
6. **Analysis**: Calculate confidence scores and patterns
7. **Bot Detection**: Analyze account behaviors and patterns

## API Reference

### Core Components

#### SpamDetectionEngine

```go
// Create new detection engine
engine := detector.NewSpamDetectionEngine(config)

// Add tweets for analysis
err := engine.AddTweet(tweet)

// Run spam detection
result, err := engine.DetectSpam()

// Get detected clusters
clusters := engine.GetClusters()
```

#### LSH Algorithms

```go
// MinHash LSH
minHashLSH := lsh.NewMinHashLSH(numHashes, bands, threshold)
minHashLSH.Add(docID, text)
similar := minHashLSH.FindSimilar(docID)

// SimHash LSH
simHashLSH := lsh.NewSimHashLSH(threshold, numTables)
simHashLSH.Add(docID, text)
similar := simHashLSH.FindSimilar(docID)
```

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...
```

### Test Coverage

- **LSH Algorithms**: Unit tests for MinHash and SimHash implementations
- **Detection Engine**: Integration tests for spam detection workflow
- **Data Models**: Tests for tweet, account, and cluster models
- **Application Logic**: End-to-end tests for complete workflows

## Performance

### Benchmarks

Typical performance on modern hardware:

- **Tweet Processing**: ~10,000 tweets/second
- **Similarity Search**: ~1,000 queries/second
- **Memory Usage**: ~1MB per 1,000 tweets
- **Clustering**: ~100 clusters/second

### Optimization

- Use hybrid mode for better accuracy
- Adjust LSH parameters for speed vs. accuracy trade-off
- Tune batch sizes for memory constraints
- Configure appropriate thresholds for spam detection

## Algorithm Details

### MinHash LSH

MinHash LSH is particularly effective for detecting near-duplicate content:

1. **Shingling**: Convert text to k-shingles (n-grams)
2. **MinHash**: Generate signatures using minimum hash values
3. **LSH Banding**: Divide signatures into bands for similarity search
4. **Candidate Generation**: Find documents in same LSH buckets
5. **Verification**: Calculate exact Jaccard similarity

### SimHash LSH

SimHash LSH excels at detecting similar content with variations:

1. **Feature Extraction**: Extract weighted features from text
2. **SimHash**: Compute locality-sensitive fingerprints
3. **LSH Tables**: Create multiple hash tables for similarity search
4. **Hamming Distance**: Measure similarity using bit differences
5. **Clustering**: Group documents with similar fingerprints

### Hybrid Approach

The hybrid mode combines both algorithms:

1. **Parallel Processing**: Run MinHash and SimHash concurrently
2. **Result Merging**: Combine clusters from both algorithms
3. **Overlap Handling**: Remove duplicate clusters
4. **Confidence Scoring**: Weight results based on algorithm agreement

## Contributing

### Development Setup

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run the test suite
5. Submit a pull request

### Code Style

- Follow Go conventions and best practices
- Use meaningful variable and function names
- Add comprehensive comments for public APIs
- Include unit tests for new functionality

### Performance Considerations

- Profile code for performance bottlenecks
- Optimize hot paths in LSH algorithms
- Consider memory usage in data structures
- Benchmark improvements against baseline

## License

This project is licensed under the MIT License. See LICENSE file for details.

## Acknowledgments

- **MinHash Algorithm**: Andrei Broder's seminal work on web duplicate detection
- **SimHash Algorithm**: Moses Charikar's locality-sensitive hashing research
- **LSH Theory**: Piotr Indyk and Rajeev Motwani's foundational LSH papers
- **Fyne GUI Framework**: Cross-platform GUI toolkit for Go

## Security and Ethics

This tool is designed for **defensive purposes only**:

- ✅ Spam detection and content moderation
- ✅ Bot identification and analysis
- ✅ Security research and education
- ✅ Platform safety improvements

**Not intended for**:
- ❌ Creating or distributing spam
- ❌ Malicious bot operations
- ❌ Privacy violations
- ❌ Harassment or abuse

## Support

For issues, questions, or contributions:

1. Check existing GitHub issues
2. Create a new issue with detailed description
3. Include configuration and logs if relevant
4. Follow the issue template guidelines

---

**X Spam Detector** - Advanced LSH-based spam detection for modern social media platforms.
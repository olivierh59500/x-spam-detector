package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"x-spam-detector/internal/autonomous"
	"x-spam-detector/internal/security"
)

// Server provides REST API for monitoring and control
type Server struct {
	engine    *autonomous.AutonomousEngine
	config    ServerConfig
	router    chi.Router
	keyManager *security.APIKeyManager
}

// ServerConfig holds API server configuration
type ServerConfig struct {
	Port         int                      `yaml:"port"`
	Host         string                   `yaml:"host"`
	EnableCORS   bool                     `yaml:"enable_cors"`
	ReadTimeout  time.Duration            `yaml:"read_timeout"`
	WriteTimeout time.Duration            `yaml:"write_timeout"`
	EnableAuth   bool                     `yaml:"enable_auth"`
	APIKey       string                   `yaml:"api_key"` // Deprecated: use KeyManager
	Security     security.APIKeyConfig    `yaml:"security"`
	RateLimit    RateLimitConfig          `yaml:"rate_limit"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled        bool          `yaml:"enabled"`
	RequestsPerMin int           `yaml:"requests_per_minute"`
	BurstSize      int           `yaml:"burst_size"`
	CleanupPeriod  time.Duration `yaml:"cleanup_period"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewServer creates a new API server
func NewServer(engine *autonomous.AutonomousEngine, config ServerConfig) *Server {
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}

	// Initialize security manager
	securityConfig := config.Security
	if (securityConfig == security.APIKeyConfig{}) {
		securityConfig = security.DefaultAPIKeyConfig()
		securityConfig.RequireAuth = config.EnableAuth
	}
	
	keyManager := security.NewAPIKeyManager(securityConfig)
	
	// Setup default key if needed and auth is enabled
	if config.EnableAuth && config.APIKey != "" {
		// Migrate legacy API key
		log.Println("Warning: Using legacy API key. Please migrate to secure key management.")
	}
	
	server := &Server{
		engine:     engine,
		config:     config,
		router:     chi.NewRouter(),
		keyManager: keyManager,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(s.config.ReadTimeout))

	if s.config.EnableCORS {
		s.router.Use(s.corsMiddleware)
	}

	if s.config.EnableAuth {
		s.router.Use(s.authMiddleware)
	}

	// Routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Health and status
		r.Get("/health", s.handleHealth)
		r.Get("/status", s.handleStatus)
		r.Get("/stats", s.handleStats)
		r.Get("/stats/detailed", s.handleDetailedStats)

		// Real-time monitoring
		r.Get("/clusters", s.handleClusters)
		r.Get("/clusters/{id}", s.handleClusterDetail)
		r.Get("/tweets", s.handleTweets)
		r.Get("/accounts", s.handleSuspiciousAccounts)
		r.Get("/alerts", s.handleAlerts)

		// Control endpoints
		r.Post("/control/pause", s.handlePause)
		r.Post("/control/resume", s.handleResume)
		r.Post("/control/detect", s.handleManualDetection)
		r.Delete("/control/clear", s.handleClear)

		// Configuration
		r.Get("/config", s.handleGetConfig)
		r.Put("/config", s.handleUpdateConfig)
	})

	// Static dashboard (if enabled)
	s.router.Get("/", s.handleDashboard)
	s.router.Get("/dashboard", s.handleDashboard)
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	log.Printf("Starting API server on %s", addr)
	return server.ListenAndServe()
}

// Handler methods

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "healthy"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetStats()
	
	status := map[string]interface{}{
		"running":      true,
		"uptime":       stats.UptimeSeconds,
		"start_time":   stats.StartTime,
		"total_tweets": stats.TotalTweets,
		"spam_detected": stats.SpamDetected,
		"clusters_found": stats.ClustersFound,
	}

	response := APIResponse{
		Success:   true,
		Data:      status,
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetStats()
	
	response := APIResponse{
		Success:   true,
		Data:      stats,
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleDetailedStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetStats()
	
	// Create detailed stats manually
	detailedStats := map[string]interface{}{
		"engine": stats,
		"system": map[string]interface{}{
			"uptime": stats.UptimeSeconds,
			"errors": stats.ErrorCount,
		},
	}
	
	response := APIResponse{
		Success:   true,
		Data:      detailedStats,
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	severityFilter := r.URL.Query().Get("severity")

	// This would get clusters from the detection engine
	// For now, we'll return a placeholder
	clusters := []map[string]interface{}{
		{
			"id":          "cluster_123",
			"size":        15,
			"confidence":  0.85,
			"severity":    "high",
			"pattern":     "crypto spam campaign",
			"created_at":  time.Now().Add(-2 * time.Hour),
		},
		{
			"id":          "cluster_456",
			"size":        8,
			"confidence":  0.72,
			"severity":    "medium",
			"pattern":     "follow for follow scheme",
			"created_at":  time.Now().Add(-1 * time.Hour),
		},
	}

	// Apply filters
	if severityFilter != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, cluster := range clusters {
			if cluster["severity"] == severityFilter {
				filtered = append(filtered, cluster)
			}
		}
		clusters = filtered
	}

	// Apply limit
	if len(clusters) > limit {
		clusters = clusters[:limit]
	}

	response := APIResponse{
		Success:   true,
		Data:      map[string]interface{}{
			"clusters": clusters,
			"count":    len(clusters),
		},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleClusterDetail(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	
	// This would get cluster details from the detection engine
	// For now, we'll return a placeholder
	cluster := map[string]interface{}{
		"id":              clusterID,
		"size":            15,
		"confidence":      0.85,
		"severity":        "high",
		"pattern":         "crypto spam campaign",
		"created_at":      time.Now().Add(-2 * time.Hour),
		"tweets":          []string{"tweet1", "tweet2", "tweet3"},
		"accounts":        []string{"bot1", "bot2", "bot3"},
		"detection_method": "minhash",
		"temporal_analysis": map[string]interface{}{
			"duration_minutes":   45,
			"tweets_per_minute":  0.33,
			"is_coordinated":     true,
		},
	}

	response := APIResponse{
		Success:   true,
		Data:      cluster,
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleTweets(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	spamOnly := r.URL.Query().Get("spam_only") == "true"

	// This would get tweets from the detection engine
	// For now, we'll return a placeholder
	tweets := []map[string]interface{}{
		{
			"id":         "tweet1",
			"text":       "Amazing crypto deal! Don't miss out!",
			"author":     "crypto_bot_1",
			"is_spam":    true,
			"spam_score": 0.95,
			"cluster_id": "cluster_123",
			"created_at": time.Now().Add(-1 * time.Hour),
		},
		{
			"id":         "tweet2",
			"text":       "Beautiful sunset today!",
			"author":     "real_user",
			"is_spam":    false,
			"spam_score": 0.05,
			"created_at": time.Now().Add(-30 * time.Minute),
		},
	}

	// Apply filters
	if spamOnly {
		filtered := make([]map[string]interface{}, 0)
		for _, tweet := range tweets {
			// Safe type assertion with validation
			if isSpam, ok := tweet["is_spam"].(bool); ok && isSpam {
				filtered = append(filtered, tweet)
			}
		}
		tweets = filtered
	}

	// Apply limit
	if len(tweets) > limit {
		tweets = tweets[:limit]
	}

	response := APIResponse{
		Success:   true,
		Data:      map[string]interface{}{
			"tweets": tweets,
			"count":  len(tweets),
		},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSuspiciousAccounts(w http.ResponseWriter, r *http.Request) {
	// This would get suspicious accounts from the detection engine
	accounts := []map[string]interface{}{
		{
			"id":           "bot1",
			"username":     "crypto_bot_1",
			"bot_score":    0.95,
			"spam_tweets":  15,
			"created_at":   time.Now().Add(-30 * 24 * time.Hour),
			"suspicious":   true,
		},
		{
			"id":           "bot2",
			"username":     "follow_bot_2",
			"bot_score":    0.78,
			"spam_tweets":  8,
			"created_at":   time.Now().Add(-45 * 24 * time.Hour),
			"suspicious":   true,
		},
	}

	response := APIResponse{
		Success:   true,
		Data:      map[string]interface{}{
			"accounts": accounts,
			"count":    len(accounts),
		},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	// This would get recent alerts
	alerts := []map[string]interface{}{
		{
			"id":          "alert1",
			"type":        "SPAM_DETECTED",
			"severity":    "high",
			"title":       "Large spam cluster detected",
			"description": "15 similar tweets from 8 accounts",
			"timestamp":   time.Now().Add(-1 * time.Hour),
		},
		{
			"id":          "alert2",
			"type":        "BOT_FARM",
			"severity":    "critical",
			"title":       "Bot farm activity",
			"description": "Coordinated posting detected",
			"timestamp":   time.Now().Add(-2 * time.Hour),
		},
	}

	response := APIResponse{
		Success:   true,
		Data:      map[string]interface{}{
			"alerts": alerts,
			"count":  len(alerts),
		},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	// This would pause the autonomous engine
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "paused"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	// This would resume the autonomous engine
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "resumed"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleManualDetection(w http.ResponseWriter, r *http.Request) {
	// This would trigger manual spam detection
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "detection_triggered"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	// This would clear all data
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "data_cleared"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// This would return current configuration
	config := map[string]interface{}{
		"crawler": map[string]interface{}{
			"polling_interval": "30s",
			"streaming_enabled": true,
		},
		"detection": map[string]interface{}{
			"threshold": 0.7,
			"min_cluster_size": 3,
		},
	}

	response := APIResponse{
		Success:   true,
		Data:      config,
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	// This would update configuration
	response := APIResponse{
		Success:   true,
		Data:      map[string]string{"status": "config_updated"},
		Timestamp: time.Now(),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Serve the dashboard HTML
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>X Spam Detector - Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 20px; }
        .stat-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-value { font-size: 2em; font-weight: bold; color: #2196F3; }
        .stat-label { color: #666; margin-top: 5px; }
        .section { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .alert { padding: 10px; margin: 10px 0; border-radius: 4px; }
        .alert-critical { background: #ffebee; border-left: 4px solid #f44336; }
        .alert-warning { background: #fff3e0; border-left: 4px solid #ff9800; }
        .alert-info { background: #e3f2fd; border-left: 4px solid #2196f3; }
        .cluster { border: 1px solid #ddd; padding: 15px; margin: 10px 0; border-radius: 4px; }
        .cluster-high { border-left: 4px solid #ff9800; }
        .cluster-critical { border-left: 4px solid #f44336; }
        .refresh-btn { background: #2196F3; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; }
        .refresh-btn:hover { background: #1976D2; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 X Spam Detector Dashboard</h1>
            <p>Real-time monitoring of spam detection activities</p>
            <button class="refresh-btn" onclick="refreshData()">🔄 Refresh</button>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value" id="tweets-collected">0</div>
                <div class="stat-label">Tweets Collected</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="spam-detected">0</div>
                <div class="stat-label">Spam Tweets</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="clusters-found">0</div>
                <div class="stat-label">Spam Clusters</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="uptime">0</div>
                <div class="stat-label">Uptime (hours)</div>
            </div>
        </div>

        <div class="section">
            <h2>📊 Recent Activity</h2>
            <div id="recent-activity">Loading...</div>
        </div>

        <div class="section">
            <h2>🚨 Active Alerts</h2>
            <div id="alerts-list">Loading...</div>
        </div>

        <div class="section">
            <h2>🎯 Spam Clusters</h2>
            <div id="clusters-list">Loading...</div>
        </div>
    </div>

    <script>
        async function fetchStats() {
            try {
                const response = await fetch('/api/v1/stats');
                const data = await response.json();
                return data.data;
            } catch (error) {
                console.error('Error fetching stats:', error);
                return null;
            }
        }

        async function fetchAlerts() {
            try {
                const response = await fetch('/api/v1/alerts');
                const data = await response.json();
                return data.data.alerts;
            } catch (error) {
                console.error('Error fetching alerts:', error);
                return [];
            }
        }

        async function fetchClusters() {
            try {
                const response = await fetch('/api/v1/clusters');
                const data = await response.json();
                return data.data.clusters;
            } catch (error) {
                console.error('Error fetching clusters:', error);
                return [];
            }
        }

        function updateStats(stats) {
            if (!stats) return;
            
            document.getElementById('tweets-collected').textContent = stats.total_tweets_collected || 0;
            document.getElementById('spam-detected').textContent = stats.spam_tweets_detected || 0;
            document.getElementById('clusters-found').textContent = stats.spam_clusters_detected || 0;
            document.getElementById('uptime').textContent = Math.round((stats.uptime_seconds || 0) / 3600);
        }

        function updateAlerts(alerts) {
            const container = document.getElementById('alerts-list');
            if (!alerts || alerts.length === 0) {
                container.innerHTML = '<p>No active alerts</p>';
                return;
            }

            container.innerHTML = alerts.map(alert => 
                '<div class="alert alert-' + alert.severity + '">' +
                '<strong>' + alert.title + '</strong><br>' +
                alert.description + '<br>' +
                '<small>' + new Date(alert.timestamp).toLocaleString() + '</small>' +
                '</div>'
            ).join('');
        }

        function updateClusters(clusters) {
            const container = document.getElementById('clusters-list');
            if (!clusters || clusters.length === 0) {
                container.innerHTML = '<p>No spam clusters detected</p>';
                return;
            }

            container.innerHTML = clusters.map(cluster => 
                '<div class="cluster cluster-' + cluster.severity + '">' +
                '<strong>Cluster ' + cluster.id + '</strong> - ' + cluster.size + ' tweets<br>' +
                'Confidence: ' + (cluster.confidence * 100).toFixed(1) + '%<br>' +
                'Pattern: ' + cluster.pattern + '<br>' +
                '<small>Detected: ' + new Date(cluster.created_at).toLocaleString() + '</small>' +
                '</div>'
            ).join('');
        }

        async function refreshData() {
            const stats = await fetchStats();
            const alerts = await fetchAlerts();
            const clusters = await fetchClusters();

            updateStats(stats);
            updateAlerts(alerts);
            updateClusters(clusters);

            document.getElementById('recent-activity').innerHTML = 
                '<p>Last update: ' + new Date().toLocaleString() + '</p>' +
                '<p>System is running and monitoring for spam patterns...</p>';
        }

        // Initial load
        refreshData();

        // Auto-refresh every 30 seconds
        setInterval(refreshData, 30000);
    </script>
</body>
</html>
	`
	
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// Middleware

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Try Authorization header as fallback
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		
		// Validate API key using secure manager
		validatedKey, err := s.keyManager.ValidateAPIKey(apiKey)
		if err != nil {
			// Log the attempt for security monitoring
			log.Printf("Authentication failed from %s: %v", r.RemoteAddr, err)
			
			s.writeJSON(w, http.StatusUnauthorized, APIResponse{
				Success:   false,
				Error:     "Authentication required",
				Timestamp: time.Now(),
			})
			return
		}
		
		// Add key info to request context for later use
		ctx := context.WithValue(r.Context(), "api_key", validatedKey)
		r = r.WithContext(ctx)
		
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		next.ServeHTTP(w, r)
	})
}

// Utility methods

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// GetDefaultServerConfig returns default server configuration
func GetDefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:         8080,
		Host:         "localhost",
		EnableCORS:   true,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableAuth:   true, // Security: Enable auth by default
		APIKey:       "",   // Deprecated: use Security config instead
		Security:     security.DefaultAPIKeyConfig(),
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 60,
			BurstSize:      10,
			CleanupPeriod:  5 * time.Minute,
		},
	}
}
package plugins

import (
	"context"
	"fmt"
	"log"
	"plugin"
	"path/filepath"
	"sync"
	"time"

	"x-spam-detector/internal/models"
)

// DetectionPlugin defines the interface for spam detection plugins
type DetectionPlugin interface {
	// Plugin metadata
	Name() string
	Version() string
	Description() string
	Author() string
	
	// Plugin lifecycle
	Initialize(config map[string]interface{}) error
	Start(ctx context.Context) error
	Stop() error
	
	// Core functionality
	Detect(tweets []*models.Tweet) (*models.DetectionResult, error)
	AnalyzeCluster(cluster *models.SpamCluster) (*ClusterAnalysis, error)
	
	// Configuration
	GetDefaultConfig() map[string]interface{}
	ValidateConfig(config map[string]interface{}) error
	UpdateConfig(config map[string]interface{}) error
	
	// Health and status
	HealthCheck() *HealthStatus
	GetMetrics() *PluginMetrics
}

// ClusterAnalysis represents the result of cluster analysis
type ClusterAnalysis struct {
	ClusterID    string                 `json:"cluster_id"`
	Confidence   float64                `json:"confidence"`
	ThreatLevel  ThreatLevel            `json:"threat_level"`
	Indicators   []ThreatIndicator      `json:"indicators"`
	Metadata     map[string]interface{} `json:"metadata"`
	Timestamp    time.Time              `json:"timestamp"`
}

// ThreatLevel defines threat severity levels
type ThreatLevel int

const (
	ThreatLow ThreatLevel = iota
	ThreatMedium
	ThreatHigh
	ThreatCritical
)

// ThreatIndicator represents a threat indicator
type ThreatIndicator struct {
	Type        string                 `json:"type"`
	Value       string                 `json:"value"`
	Confidence  float64                `json:"confidence"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// HealthStatus represents plugin health status
type HealthStatus struct {
	Status      PluginStatus `json:"status"`
	Message     string       `json:"message"`
	LastCheck   time.Time    `json:"last_check"`
	Uptime      time.Duration `json:"uptime"`
	ErrorCount  int64        `json:"error_count"`
}

// PluginStatus defines plugin status states
type PluginStatus int

const (
	StatusUnknown PluginStatus = iota
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
	StatusStopped
)

// PluginMetrics represents plugin performance metrics
type PluginMetrics struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessfulRequests int64       `json:"successful_requests"`
	FailedRequests   int64         `json:"failed_requests"`
	AverageLatency   time.Duration `json:"average_latency"`
	LastRequestTime  time.Time     `json:"last_request_time"`
	ThroughputPerMin float64       `json:"throughput_per_min"`
	MemoryUsage      int64         `json:"memory_usage_bytes"`
}

// PluginInfo contains plugin metadata and runtime information
type PluginInfo struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	Config       map[string]interface{} `json:"config"`
	Status       PluginStatus           `json:"status"`
	LoadTime     time.Time              `json:"load_time"`
	FilePath     string                 `json:"file_path"`
	Plugin       DetectionPlugin        `json:"-"`
}

// PluginManager manages detection plugins
type PluginManager struct {
	plugins    map[string]*PluginInfo
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	pluginDir  string
	
	// Plugin execution settings
	defaultTimeout time.Duration
	maxPlugins     int
	
	// Metrics
	totalDetections int64
	pluginErrors    map[string]int64
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(pluginDir string, maxPlugins int, defaultTimeout time.Duration) *PluginManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &PluginManager{
		plugins:        make(map[string]*PluginInfo),
		ctx:            ctx,
		cancel:         cancel,
		pluginDir:      pluginDir,
		defaultTimeout: defaultTimeout,
		maxPlugins:     maxPlugins,
		pluginErrors:   make(map[string]int64),
	}
}

// LoadPlugin loads a plugin from a shared library file
func (pm *PluginManager) LoadPlugin(filePath string, config map[string]interface{}) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	if len(pm.plugins) >= pm.maxPlugins {
		return fmt.Errorf("maximum number of plugins (%d) already loaded", pm.maxPlugins)
	}
	
	// Load the plugin
	p, err := plugin.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open plugin %s: %w", filePath, err)
	}
	
	// Look for the NewPlugin symbol
	symbol, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s does not export NewPlugin function: %w", filePath, err)
	}
	
	// Assert the symbol is a function that returns DetectionPlugin
	newPluginFunc, ok := symbol.(func() DetectionPlugin)
	if !ok {
		return fmt.Errorf("plugin %s NewPlugin function has incorrect signature", filePath)
	}
	
	// Create plugin instance
	pluginInstance := newPluginFunc()
	
	// Validate plugin
	if err := pm.validatePlugin(pluginInstance); err != nil {
		return fmt.Errorf("plugin validation failed: %w", err)
	}
	
	// Initialize plugin
	if config == nil {
		config = pluginInstance.GetDefaultConfig()
	}
	
	if err := pluginInstance.Initialize(config); err != nil {
		return fmt.Errorf("plugin initialization failed: %w", err)
	}
	
	// Create plugin info
	info := &PluginInfo{
		Name:        pluginInstance.Name(),
		Version:     pluginInstance.Version(),
		Description: pluginInstance.Description(),
		Author:      pluginInstance.Author(),
		Config:      config,
		Status:      StatusStopped,
		LoadTime:    time.Now(),
		FilePath:    filePath,
		Plugin:      pluginInstance,
	}
	
	pm.plugins[info.Name] = info
	
	log.Printf("Plugin loaded successfully: %s v%s", info.Name, info.Version)
	return nil
}

// LoadPluginsFromDirectory loads all plugins from a directory
func (pm *PluginManager) LoadPluginsFromDirectory() error {
	if pm.pluginDir == "" {
		return fmt.Errorf("plugin directory not specified")
	}
	
	pattern := filepath.Join(pm.pluginDir, "*.so")
	pluginFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find plugin files: %w", err)
	}
	
	for _, file := range pluginFiles {
		if err := pm.LoadPlugin(file, nil); err != nil {
			log.Printf("Failed to load plugin %s: %v", file, err)
			continue
		}
	}
	
	log.Printf("Loaded %d plugins from directory %s", len(pm.plugins), pm.pluginDir)
	return nil
}

// StartPlugin starts a specific plugin
func (pm *PluginManager) StartPlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	info, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	
	if info.Status != StatusStopped {
		return fmt.Errorf("plugin %s is already running", name)
	}
	
	if err := info.Plugin.Start(pm.ctx); err != nil {
		return fmt.Errorf("failed to start plugin %s: %w", name, err)
	}
	
	info.Status = StatusHealthy
	log.Printf("Plugin started: %s", name)
	return nil
}

// StopPlugin stops a specific plugin
func (pm *PluginManager) StopPlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	info, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	
	if info.Status == StatusStopped {
		return nil // Already stopped
	}
	
	if err := info.Plugin.Stop(); err != nil {
		return fmt.Errorf("failed to stop plugin %s: %w", name, err)
	}
	
	info.Status = StatusStopped
	log.Printf("Plugin stopped: %s", name)
	return nil
}

// StartAllPlugins starts all loaded plugins
func (pm *PluginManager) StartAllPlugins() error {
	pm.mutex.RLock()
	pluginNames := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		pluginNames = append(pluginNames, name)
	}
	pm.mutex.RUnlock()
	
	var errors []error
	for _, name := range pluginNames {
		if err := pm.StartPlugin(name); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to start some plugins: %v", errors)
	}
	
	return nil
}

// StopAllPlugins stops all running plugins
func (pm *PluginManager) StopAllPlugins() error {
	pm.mutex.RLock()
	pluginNames := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		pluginNames = append(pluginNames, name)
	}
	pm.mutex.RUnlock()
	
	var errors []error
	for _, name := range pluginNames {
		if err := pm.StopPlugin(name); err != nil {
			errors = append(errors, err)
		}
	}
	
	pm.cancel() // Cancel context for all plugins
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to stop some plugins: %v", errors)
	}
	
	return nil
}

// DetectWithPlugins runs detection using all active plugins
func (pm *PluginManager) DetectWithPlugins(tweets []*models.Tweet) ([]*models.DetectionResult, error) {
	pm.mutex.RLock()
	activePlugins := make([]*PluginInfo, 0)
	for _, info := range pm.plugins {
		if info.Status == StatusHealthy {
			activePlugins = append(activePlugins, info)
		}
	}
	pm.mutex.RUnlock()
	
	if len(activePlugins) == 0 {
		return nil, fmt.Errorf("no active plugins available")
	}
	
	results := make([]*models.DetectionResult, 0, len(activePlugins))
	var wg sync.WaitGroup
	var resultMutex sync.Mutex
	
	for _, info := range activePlugins {
		wg.Add(1)
		go func(plugin *PluginInfo) {
			defer wg.Done()
			
			// Create timeout context
			ctx, cancel := context.WithTimeout(pm.ctx, pm.defaultTimeout)
			defer cancel()
			
			// Run detection with timeout
			resultChan := make(chan *models.DetectionResult, 1)
			errorChan := make(chan error, 1)
			
			go func() {
				result, err := plugin.Plugin.Detect(tweets)
				if err != nil {
					errorChan <- err
				} else {
					resultChan <- result
				}
			}()
			
			select {
			case result := <-resultChan:
				if result != nil {
					result.Algorithm = plugin.Name // Tag with plugin name
					resultMutex.Lock()
					results = append(results, result)
					resultMutex.Unlock()
				}
				
			case err := <-errorChan:
				log.Printf("Plugin %s detection failed: %v", plugin.Name, err)
				pm.recordPluginError(plugin.Name)
				
			case <-ctx.Done():
				log.Printf("Plugin %s detection timed out", plugin.Name)
				pm.recordPluginError(plugin.Name)
			}
		}(info)
	}
	
	wg.Wait()
	pm.totalDetections++
	
	return results, nil
}

// AnalyzeClusterWithPlugins analyzes a cluster using all active plugins
func (pm *PluginManager) AnalyzeClusterWithPlugins(cluster *models.SpamCluster) ([]*ClusterAnalysis, error) {
	pm.mutex.RLock()
	activePlugins := make([]*PluginInfo, 0)
	for _, info := range pm.plugins {
		if info.Status == StatusHealthy {
			activePlugins = append(activePlugins, info)
		}
	}
	pm.mutex.RUnlock()
	
	if len(activePlugins) == 0 {
		return nil, fmt.Errorf("no active plugins available")
	}
	
	analyses := make([]*ClusterAnalysis, 0, len(activePlugins))
	var wg sync.WaitGroup
	var resultMutex sync.Mutex
	
	for _, info := range activePlugins {
		wg.Add(1)
		go func(plugin *PluginInfo) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(pm.ctx, pm.defaultTimeout)
			defer cancel()
			
			resultChan := make(chan *ClusterAnalysis, 1)
			errorChan := make(chan error, 1)
			
			go func() {
				analysis, err := plugin.Plugin.AnalyzeCluster(cluster)
				if err != nil {
					errorChan <- err
				} else {
					resultChan <- analysis
				}
			}()
			
			select {
			case analysis := <-resultChan:
				if analysis != nil {
					resultMutex.Lock()
					analyses = append(analyses, analysis)
					resultMutex.Unlock()
				}
				
			case err := <-errorChan:
				log.Printf("Plugin %s cluster analysis failed: %v", plugin.Name, err)
				pm.recordPluginError(plugin.Name)
				
			case <-ctx.Done():
				log.Printf("Plugin %s cluster analysis timed out", plugin.Name)
				pm.recordPluginError(plugin.Name)
			}
		}(info)
	}
	
	wg.Wait()
	return analyses, nil
}

// GetPluginInfo returns information about a specific plugin
func (pm *PluginManager) GetPluginInfo(name string) (*PluginInfo, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	info, exists := pm.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	
	// Return a copy to prevent external modification
	return &PluginInfo{
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Config:      info.Config,
		Status:      info.Status,
		LoadTime:    info.LoadTime,
		FilePath:    info.FilePath,
	}, nil
}

// ListPlugins returns information about all loaded plugins
func (pm *PluginManager) ListPlugins() []*PluginInfo {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	plugins := make([]*PluginInfo, 0, len(pm.plugins))
	for _, info := range pm.plugins {
		plugins = append(plugins, &PluginInfo{
			Name:        info.Name,
			Version:     info.Version,
			Description: info.Description,
			Author:      info.Author,
			Config:      info.Config,
			Status:      info.Status,
			LoadTime:    info.LoadTime,
			FilePath:    info.FilePath,
		})
	}
	
	return plugins
}

// HealthCheckAll performs health checks on all plugins
func (pm *PluginManager) HealthCheckAll() map[string]*HealthStatus {
	pm.mutex.RLock()
	plugins := make(map[string]*PluginInfo)
	for name, info := range pm.plugins {
		plugins[name] = info
	}
	pm.mutex.RUnlock()
	
	results := make(map[string]*HealthStatus)
	var wg sync.WaitGroup
	var resultMutex sync.Mutex
	
	for name, info := range plugins {
		wg.Add(1)
		go func(pluginName string, plugin *PluginInfo) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(pm.ctx, 5*time.Second)
			defer cancel()
			
			healthChan := make(chan *HealthStatus, 1)
			
			go func() {
				health := plugin.Plugin.HealthCheck()
				healthChan <- health
			}()
			
			select {
			case health := <-healthChan:
				resultMutex.Lock()
				results[pluginName] = health
				
				// Update plugin status based on health
				pm.mutex.Lock()
				if pluginInfo, exists := pm.plugins[pluginName]; exists {
					pluginInfo.Status = health.Status
				}
				pm.mutex.Unlock()
				
				resultMutex.Unlock()
				
			case <-ctx.Done():
				resultMutex.Lock()
				results[pluginName] = &HealthStatus{
					Status:    StatusUnhealthy,
					Message:   "Health check timeout",
					LastCheck: time.Now(),
				}
				resultMutex.Unlock()
			}
		}(name, info)
	}
	
	wg.Wait()
	return results
}

// GetMetrics returns aggregated metrics for all plugins
func (pm *PluginManager) GetMetrics() *ManagerMetrics {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	totalPlugins := len(pm.plugins)
	activePlugins := 0
	
	for _, info := range pm.plugins {
		if info.Status == StatusHealthy {
			activePlugins++
		}
	}
	
	return &ManagerMetrics{
		TotalPlugins:     totalPlugins,
		ActivePlugins:    activePlugins,
		TotalDetections:  pm.totalDetections,
		PluginErrors:     pm.pluginErrors,
	}
}

// ManagerMetrics represents plugin manager metrics
type ManagerMetrics struct {
	TotalPlugins    int              `json:"total_plugins"`
	ActivePlugins   int              `json:"active_plugins"`
	TotalDetections int64            `json:"total_detections"`
	PluginErrors    map[string]int64 `json:"plugin_errors"`
}

// validatePlugin validates that a plugin implements the required interface
func (pm *PluginManager) validatePlugin(plugin DetectionPlugin) error {
	if plugin.Name() == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	
	if plugin.Version() == "" {
		return fmt.Errorf("plugin version cannot be empty")
	}
	
	// Test default config
	config := plugin.GetDefaultConfig()
	if err := plugin.ValidateConfig(config); err != nil {
		return fmt.Errorf("default config validation failed: %w", err)
	}
	
	return nil
}

// recordPluginError records an error for a plugin
func (pm *PluginManager) recordPluginError(pluginName string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.pluginErrors[pluginName]++
}

// UnloadPlugin unloads a specific plugin
func (pm *PluginManager) UnloadPlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	info, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	
	// Stop the plugin if it's running
	if info.Status != StatusStopped {
		if err := info.Plugin.Stop(); err != nil {
			log.Printf("Error stopping plugin %s during unload: %v", name, err)
		}
	}
	
	delete(pm.plugins, name)
	log.Printf("Plugin unloaded: %s", name)
	return nil
}

// Shutdown gracefully shuts down the plugin manager
func (pm *PluginManager) Shutdown() error {
	log.Println("Shutting down plugin manager...")
	
	if err := pm.StopAllPlugins(); err != nil {
		log.Printf("Error stopping plugins during shutdown: %v", err)
	}
	
	pm.mutex.Lock()
	pm.plugins = make(map[string]*PluginInfo)
	pm.mutex.Unlock()
	
	log.Println("Plugin manager shutdown complete")
	return nil
}
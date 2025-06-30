package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MetricsRegistry manages application metrics
type MetricsRegistry struct {
	metrics map[string]Metric
	mutex   sync.RWMutex
	
	// System metrics
	systemCollector *SystemMetricsCollector
	
	// Collection settings
	collectInterval time.Duration
	retention      time.Duration
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Metric interface for all metric types
type Metric interface {
	Name() string
	Type() MetricType
	Value() interface{}
	Labels() map[string]string
	Timestamp() time.Time
	Reset()
}

// MetricType defines the type of metric
type MetricType int

const (
	Counter MetricType = iota
	Gauge
	Histogram
	Summary
)

// CounterMetric represents a monotonically increasing counter
type CounterMetric struct {
	name      string
	labels    map[string]string
	value     int64
	timestamp time.Time
	mutex     sync.RWMutex
}

// GaugeMetric represents a value that can go up and down
type GaugeMetric struct {
	name      string
	labels    map[string]string
	value     float64
	timestamp time.Time
	mutex     sync.RWMutex
}

// HistogramMetric collects observations and counts them in buckets
type HistogramMetric struct {
	name      string
	labels    map[string]string
	buckets   []float64
	counts    []int64
	sum       float64
	count     int64
	timestamp time.Time
	mutex     sync.RWMutex
}

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     int64     `json:"memory_usage_bytes"`
	MemoryUsagePct  float64   `json:"memory_usage_percent"`
	GoroutineCount  int       `json:"goroutine_count"`
	GCPauses        []float64 `json:"gc_pauses_ms"`
	GCCount         int64     `json:"gc_count"`
	HeapSize        int64     `json:"heap_size_bytes"`
	Uptime          time.Duration `json:"uptime"`
	Timestamp       time.Time `json:"timestamp"`
}

// ApplicationMetrics represents application-specific metrics
type ApplicationMetrics struct {
	TweetsProcessed     int64             `json:"tweets_processed"`
	SpamDetected        int64             `json:"spam_detected"`
	ClustersFound       int64             `json:"clusters_found"`
	DetectionLatency    time.Duration     `json:"detection_latency"`
	CacheHitRate        float64           `json:"cache_hit_rate"`
	ActiveConnections   int64             `json:"active_connections"`
	RequestRate         float64           `json:"request_rate_per_second"`
	ErrorRate           float64           `json:"error_rate_percent"`
	PluginMetrics       map[string]interface{} `json:"plugin_metrics"`
	Timestamp           time.Time         `json:"timestamp"`
}

// PerformanceMetrics represents performance metrics
type PerformanceMetrics struct {
	ProcessingRate      float64       `json:"processing_rate_tweets_per_second"`
	ThroughputMBps      float64       `json:"throughput_mbps"`
	P50Latency          time.Duration `json:"p50_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	MaxLatency          time.Duration `json:"max_latency"`
	MinLatency          time.Duration `json:"min_latency"`
	ConcurrentWorkers   int           `json:"concurrent_workers"`
	QueueDepth          int           `json:"queue_depth"`
	BackpressureEvents  int64         `json:"backpressure_events"`
	Timestamp           time.Time     `json:"timestamp"`
}

// SystemMetricsCollector collects system metrics
type SystemMetricsCollector struct {
	startTime    time.Time
	prevMemStats runtime.MemStats
	mutex        sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry
func NewMetricsRegistry(collectInterval, retention time.Duration) *MetricsRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsRegistry{
		metrics:         make(map[string]Metric),
		systemCollector: NewSystemMetricsCollector(),
		collectInterval: collectInterval,
		retention:      retention,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{
		startTime: time.Now(),
	}
}

// Start starts the metrics collection
func (mr *MetricsRegistry) Start() {
	mr.wg.Add(1)
	go mr.collectLoop()
}

// Stop stops the metrics collection
func (mr *MetricsRegistry) Stop() {
	mr.cancel()
	mr.wg.Wait()
}

// RegisterCounter registers a new counter metric
func (mr *MetricsRegistry) RegisterCounter(name string, labels map[string]string) *CounterMetric {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	
	counter := &CounterMetric{
		name:      name,
		labels:    labels,
		timestamp: time.Now(),
	}
	
	mr.metrics[name] = counter
	return counter
}

// RegisterGauge registers a new gauge metric
func (mr *MetricsRegistry) RegisterGauge(name string, labels map[string]string) *GaugeMetric {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	
	gauge := &GaugeMetric{
		name:      name,
		labels:    labels,
		timestamp: time.Now(),
	}
	
	mr.metrics[name] = gauge
	return gauge
}

// RegisterHistogram registers a new histogram metric
func (mr *MetricsRegistry) RegisterHistogram(name string, labels map[string]string, buckets []float64) *HistogramMetric {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()
	
	histogram := &HistogramMetric{
		name:      name,
		labels:    labels,
		buckets:   buckets,
		counts:    make([]int64, len(buckets)),
		timestamp: time.Now(),
	}
	
	mr.metrics[name] = histogram
	return histogram
}

// GetMetric retrieves a metric by name
func (mr *MetricsRegistry) GetMetric(name string) (Metric, bool) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	
	metric, exists := mr.metrics[name]
	return metric, exists
}

// GetAllMetrics returns all registered metrics
func (mr *MetricsRegistry) GetAllMetrics() map[string]Metric {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	metrics := make(map[string]Metric)
	for name, metric := range mr.metrics {
		metrics[name] = metric
	}
	
	return metrics
}

// GetSystemMetrics returns current system metrics
func (mr *MetricsRegistry) GetSystemMetrics() *SystemMetrics {
	return mr.systemCollector.Collect()
}

// collectLoop runs the metrics collection loop
func (mr *MetricsRegistry) collectLoop() {
	defer mr.wg.Done()
	
	ticker := time.NewTicker(mr.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mr.systemCollector.Collect()
			
		case <-mr.ctx.Done():
			return
		}
	}
}

// CounterMetric implementation

func (c *CounterMetric) Name() string {
	return c.name
}

func (c *CounterMetric) Type() MetricType {
	return Counter
}

func (c *CounterMetric) Value() interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

func (c *CounterMetric) Labels() map[string]string {
	return c.labels
}

func (c *CounterMetric) Timestamp() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.timestamp
}

func (c *CounterMetric) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value = 0
	c.timestamp = time.Now()
}

func (c *CounterMetric) Inc() {
	c.Add(1)
}

func (c *CounterMetric) Add(value int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value += value
	c.timestamp = time.Now()
}

func (c *CounterMetric) Get() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

// GaugeMetric implementation

func (g *GaugeMetric) Name() string {
	return g.name
}

func (g *GaugeMetric) Type() MetricType {
	return Gauge
}

func (g *GaugeMetric) Value() interface{} {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.value
}

func (g *GaugeMetric) Labels() map[string]string {
	return g.labels
}

func (g *GaugeMetric) Timestamp() time.Time {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.timestamp
}

func (g *GaugeMetric) Reset() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value = 0
	g.timestamp = time.Now()
}

func (g *GaugeMetric) Set(value float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value = value
	g.timestamp = time.Now()
}

func (g *GaugeMetric) Inc() {
	g.Add(1)
}

func (g *GaugeMetric) Dec() {
	g.Add(-1)
}

func (g *GaugeMetric) Add(value float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value += value
	g.timestamp = time.Now()
}

func (g *GaugeMetric) Get() float64 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.value
}

// HistogramMetric implementation

func (h *HistogramMetric) Name() string {
	return h.name
}

func (h *HistogramMetric) Type() MetricType {
	return Histogram
}

func (h *HistogramMetric) Value() interface{} {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	return map[string]interface{}{
		"buckets": h.buckets,
		"counts":  h.counts,
		"sum":     h.sum,
		"count":   h.count,
	}
}

func (h *HistogramMetric) Labels() map[string]string {
	return h.labels
}

func (h *HistogramMetric) Timestamp() time.Time {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.timestamp
}

func (h *HistogramMetric) Reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	for i := range h.counts {
		h.counts[i] = 0
	}
	h.sum = 0
	h.count = 0
	h.timestamp = time.Now()
}

func (h *HistogramMetric) Observe(value float64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	// Find appropriate bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
	
	h.sum += value
	h.count++
	h.timestamp = time.Now()
}

func (h *HistogramMetric) GetCount() int64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.count
}

func (h *HistogramMetric) GetSum() float64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.sum
}

func (h *HistogramMetric) GetBuckets() ([]float64, []int64) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	buckets := make([]float64, len(h.buckets))
	counts := make([]int64, len(h.counts))
	
	copy(buckets, h.buckets)
	copy(counts, h.counts)
	
	return buckets, counts
}

// SystemMetricsCollector implementation

func (smc *SystemMetricsCollector) Collect() *SystemMetrics {
	smc.mutex.Lock()
	defer smc.mutex.Unlock()
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate CPU usage (simplified)
	cpuUsage := float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 10.0
	if cpuUsage > 100.0 {
		cpuUsage = 100.0
	}
	
	// Calculate memory usage percentage
	var totalMem int64 = 8 * 1024 * 1024 * 1024 // Assume 8GB total, in real app get from system
	memUsagePct := float64(memStats.Alloc) / float64(totalMem) * 100.0
	
	// Calculate GC pause times (simplified)
	gcPauses := make([]float64, 0)
	// Note: In production, use debug.ReadGCStats for detailed GC information
	
	metrics := &SystemMetrics{
		CPUUsage:       cpuUsage,
		MemoryUsage:    int64(memStats.Alloc),
		MemoryUsagePct: memUsagePct,
		GoroutineCount: runtime.NumGoroutine(),
		GCPauses:       gcPauses,
		GCCount:        int64(memStats.NumGC),
		HeapSize:       int64(memStats.HeapAlloc),
		Uptime:         time.Since(smc.startTime),
		Timestamp:      time.Now(),
	}
	
	smc.prevMemStats = memStats
	
	return metrics
}

// MetricsExporter exports metrics in various formats
type MetricsExporter struct {
	registry *MetricsRegistry
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(registry *MetricsRegistry) *MetricsExporter {
	return &MetricsExporter{
		registry: registry,
	}
}

// ExportPrometheus exports metrics in Prometheus format
func (me *MetricsExporter) ExportPrometheus() string {
	metrics := me.registry.GetAllMetrics()
	var output string
	
	for _, metric := range metrics {
		labels := ""
		if len(metric.Labels()) > 0 {
			var labelPairs []string
			for k, v := range metric.Labels() {
				labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, k, v))
			}
			labels = "{" + fmt.Sprintf("%v", labelPairs) + "}"
		}
		
		switch metric.Type() {
		case Counter:
			output += fmt.Sprintf("# TYPE %s counter\n", metric.Name())
			output += fmt.Sprintf("%s%s %v\n", metric.Name(), labels, metric.Value())
			
		case Gauge:
			output += fmt.Sprintf("# TYPE %s gauge\n", metric.Name())
			output += fmt.Sprintf("%s%s %v\n", metric.Name(), labels, metric.Value())
			
		case Histogram:
			output += fmt.Sprintf("# TYPE %s histogram\n", metric.Name())
			if h, ok := metric.(*HistogramMetric); ok {
				buckets, counts := h.GetBuckets()
				for i, bucket := range buckets {
					bucketLabel := fmt.Sprintf(`{le="%.2f"}`, bucket)
					output += fmt.Sprintf("%s_bucket%s %d\n", metric.Name(), bucketLabel, counts[i])
				}
				output += fmt.Sprintf("%s_sum%s %.2f\n", metric.Name(), labels, h.GetSum())
				output += fmt.Sprintf("%s_count%s %d\n", metric.Name(), labels, h.GetCount())
			}
		}
		output += "\n"
	}
	
	return output
}

// ExportJSON exports metrics in JSON format
func (me *MetricsExporter) ExportJSON() map[string]interface{} {
	metrics := me.registry.GetAllMetrics()
	systemMetrics := me.registry.GetSystemMetrics()
	
	result := make(map[string]interface{})
	result["system"] = systemMetrics
	result["application"] = make(map[string]interface{})
	
	appMetrics := result["application"].(map[string]interface{})
	for name, metric := range metrics {
		appMetrics[name] = map[string]interface{}{
			"type":      metric.Type(),
			"value":     metric.Value(),
			"labels":    metric.Labels(),
			"timestamp": metric.Timestamp(),
		}
	}
	
	return result
}

// HealthChecker performs health checks and reports status
type HealthChecker struct {
	checks    map[string]HealthCheck
	timeout   time.Duration
	mutex     sync.RWMutex
}

// HealthCheck interface for health check implementations
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) *HealthResult
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// HealthStatus represents health check status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// NewHealthChecker creates a new health checker
func NewHealthChecker(timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		checks:  make(map[string]HealthCheck),
		timeout: timeout,
	}
}

// RegisterCheck registers a new health check
func (hc *HealthChecker) RegisterCheck(check HealthCheck) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.checks[check.Name()] = check
}

// RunChecks runs all registered health checks
func (hc *HealthChecker) RunChecks(ctx context.Context) map[string]*HealthResult {
	hc.mutex.RLock()
	checks := make(map[string]HealthCheck)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mutex.RUnlock()
	
	results := make(map[string]*HealthResult)
	var wg sync.WaitGroup
	var resultMutex sync.Mutex
	
	for name, check := range checks {
		wg.Add(1)
		go func(checkName string, healthCheck HealthCheck) {
			defer wg.Done()
			
			checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
			defer cancel()
			
			start := time.Now()
			result := healthCheck.Check(checkCtx)
			result.Duration = time.Since(start)
			result.Timestamp = time.Now()
			
			resultMutex.Lock()
			results[checkName] = result
			resultMutex.Unlock()
		}(name, check)
	}
	
	wg.Wait()
	return results
}

// GetOverallHealth returns overall system health
func (hc *HealthChecker) GetOverallHealth(ctx context.Context) *HealthResult {
	results := hc.RunChecks(ctx)
	
	overall := &HealthResult{
		Status:    HealthStatusHealthy,
		Message:   "All checks passed",
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0
	
	for name, result := range results {
		overall.Metadata[name] = result
		
		switch result.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}
	
	total := len(results)
	if total == 0 {
		overall.Status = HealthStatusUnknown
		overall.Message = "No health checks configured"
	} else if unhealthyCount > 0 {
		overall.Status = HealthStatusUnhealthy
		overall.Message = fmt.Sprintf("%d of %d checks unhealthy", unhealthyCount, total)
	} else if degradedCount > 0 {
		overall.Status = HealthStatusDegraded
		overall.Message = fmt.Sprintf("%d of %d checks degraded", degradedCount, total)
	}
	
	return overall
}
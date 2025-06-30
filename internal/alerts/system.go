package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"x-spam-detector/internal/models"
	"x-spam-detector/internal/temporal"
)

// AlertingSystem manages intelligent alerting for spam detection
type AlertingSystem struct {
	rules         map[string]*AlertRule
	channels      map[string]AlertChannel
	throttle      *ThrottleManager
	escalation    *EscalationManager
	history       *AlertHistory
	mutex         sync.RWMutex
	
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Condition   AlertCondition         `json:"condition"`
	Severity    AlertSeverity          `json:"severity"`
	Channels    []string               `json:"channels"`
	Enabled     bool                   `json:"enabled"`
	Throttle    ThrottleConfig         `json:"throttle"`
	Escalation  *EscalationConfig      `json:"escalation,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AlertCondition defines the condition for triggering an alert
type AlertCondition struct {
	Type       ConditionType          `json:"type"`
	Metric     string                 `json:"metric,omitempty"`
	Operator   ComparisonOperator     `json:"operator"`
	Threshold  float64                `json:"threshold"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Aggregation AggregationType       `json:"aggregation,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
}

// ConditionType defines types of alert conditions
type ConditionType int

const (
	MetricThreshold ConditionType = iota
	SpamClusterDetected
	TemporalPatternDetected
	HighVolumeEvent
	SystemHealth
	PluginFailure
	CustomCondition
)

// ComparisonOperator defines comparison operators for conditions
type ComparisonOperator int

const (
	GreaterThan ComparisonOperator = iota
	LessThan
	GreaterThanOrEqual
	LessThanOrEqual
	Equal
	NotEqual
)

// AggregationType defines how metrics are aggregated
type AggregationType int

const (
	Sum AggregationType = iota
	Average
	Maximum
	Minimum
	Count
	Rate
)

// AlertSeverity defines alert severity levels
type AlertSeverity int

const (
	Info AlertSeverity = iota
	Warning
	Error
	Critical
)

// Alert represents a triggered alert
type Alert struct {
	ID          string                 `json:"id"`
	RuleID      string                 `json:"rule_id"`
	RuleName    string                 `json:"rule_name"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Status      AlertStatus            `json:"status"`
	Metadata    map[string]interface{} `json:"metadata"`
	Context     *AlertContext          `json:"context,omitempty"`
	Escalations []EscalationEvent      `json:"escalations,omitempty"`
}

// AlertStatus defines alert status states
type AlertStatus int

const (
	Active AlertStatus = iota
	Acknowledged
	Resolved
	Suppressed
)

// AlertContext provides additional context for alerts
type AlertContext struct {
	SpamCluster    *models.SpamCluster           `json:"spam_cluster,omitempty"`
	TemporalPattern *temporal.TemporalPattern    `json:"temporal_pattern,omitempty"`
	Metrics        map[string]interface{}       `json:"metrics,omitempty"`
	Affected       []string                     `json:"affected,omitempty"`
	SystemInfo     map[string]interface{}       `json:"system_info,omitempty"`
}

// AlertChannel defines how alerts are delivered
type AlertChannel interface {
	Name() string
	Type() ChannelType
	Send(ctx context.Context, alert *Alert) error
	HealthCheck() error
	GetConfig() map[string]interface{}
}

// ChannelType defines types of alert channels
type ChannelType int

const (
	Email ChannelType = iota
	Slack
	Webhook
	SMS
	PagerDuty
	Discord
	Teams
)

// ThrottleConfig defines throttling settings for alerts
type ThrottleConfig struct {
	Enabled      bool          `json:"enabled"`
	Duration     time.Duration `json:"duration"`
	MaxAlerts    int           `json:"max_alerts"`
	PerTimeframe time.Duration `json:"per_timeframe"`
}

// EscalationConfig defines escalation settings
type EscalationConfig struct {
	Enabled   bool                    `json:"enabled"`
	Levels    []EscalationLevel       `json:"levels"`
	AutoResolve time.Duration         `json:"auto_resolve,omitempty"`
}

// EscalationLevel defines an escalation level
type EscalationLevel struct {
	Level     int           `json:"level"`
	After     time.Duration `json:"after"`
	Channels  []string      `json:"channels"`
	Severity  AlertSeverity `json:"severity"`
}

// EscalationEvent represents an escalation event
type EscalationEvent struct {
	Level     int       `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Channels  []string  `json:"channels"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// NewAlertingSystem creates a new alerting system
func NewAlertingSystem() *AlertingSystem {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AlertingSystem{
		rules:      make(map[string]*AlertRule),
		channels:   make(map[string]AlertChannel),
		throttle:   NewThrottleManager(),
		escalation: NewEscalationManager(),
		history:    NewAlertHistory(1000), // Keep last 1000 alerts
		ctx:        ctx,
		cancel:     cancel,
	}
}

// AddRule adds a new alert rule
func (as *AlertingSystem) AddRule(rule *AlertRule) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	if rule.ID == "" {
		rule.ID = as.generateRuleID()
	}
	
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	
	// Validate rule
	if err := as.validateRule(rule); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}
	
	as.rules[rule.ID] = rule
	log.Printf("Alert rule added: %s (%s)", rule.Name, rule.ID)
	return nil
}

// UpdateRule updates an existing alert rule
func (as *AlertingSystem) UpdateRule(ruleID string, rule *AlertRule) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	existing, exists := as.rules[ruleID]
	if !exists {
		return fmt.Errorf("rule %s not found", ruleID)
	}
	
	rule.ID = ruleID
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	
	if err := as.validateRule(rule); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}
	
	as.rules[ruleID] = rule
	log.Printf("Alert rule updated: %s (%s)", rule.Name, rule.ID)
	return nil
}

// RemoveRule removes an alert rule
func (as *AlertingSystem) RemoveRule(ruleID string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	if _, exists := as.rules[ruleID]; !exists {
		return fmt.Errorf("rule %s not found", ruleID)
	}
	
	delete(as.rules, ruleID)
	log.Printf("Alert rule removed: %s", ruleID)
	return nil
}

// AddChannel adds a new alert channel
func (as *AlertingSystem) AddChannel(channel AlertChannel) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	// Test channel connectivity
	if err := channel.HealthCheck(); err != nil {
		return fmt.Errorf("channel health check failed: %w", err)
	}
	
	as.channels[channel.Name()] = channel
	log.Printf("Alert channel added: %s (%s)", channel.Name(), as.channelTypeString(channel.Type()))
	return nil
}

// RemoveChannel removes an alert channel
func (as *AlertingSystem) RemoveChannel(name string) error {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	
	if _, exists := as.channels[name]; !exists {
		return fmt.Errorf("channel %s not found", name)
	}
	
	delete(as.channels, name)
	log.Printf("Alert channel removed: %s", name)
	return nil
}

// EvaluateSpamCluster evaluates a spam cluster against alert rules
func (as *AlertingSystem) EvaluateSpamCluster(cluster *models.SpamCluster) error {
	as.mutex.RLock()
	rules := make([]*AlertRule, 0)
	for _, rule := range as.rules {
		if rule.Enabled && rule.Condition.Type == SpamClusterDetected {
			rules = append(rules, rule)
		}
	}
	as.mutex.RUnlock()
	
	for _, rule := range rules {
		if as.evaluateSpamClusterCondition(rule, cluster) {
			alert := as.createSpamClusterAlert(rule, cluster)
			if err := as.triggerAlert(alert); err != nil {
				log.Printf("Failed to trigger alert for rule %s: %v", rule.ID, err)
			}
		}
	}
	
	return nil
}

// EvaluateTemporalPattern evaluates a temporal pattern against alert rules
func (as *AlertingSystem) EvaluateTemporalPattern(pattern *temporal.TemporalPattern) error {
	as.mutex.RLock()
	rules := make([]*AlertRule, 0)
	for _, rule := range as.rules {
		if rule.Enabled && rule.Condition.Type == TemporalPatternDetected {
			rules = append(rules, rule)
		}
	}
	as.mutex.RUnlock()
	
	for _, rule := range rules {
		if as.evaluateTemporalPatternCondition(rule, pattern) {
			alert := as.createTemporalPatternAlert(rule, pattern)
			if err := as.triggerAlert(alert); err != nil {
				log.Printf("Failed to trigger alert for rule %s: %v", rule.ID, err)
			}
		}
	}
	
	return nil
}

// EvaluateMetrics evaluates metrics against threshold-based alert rules
func (as *AlertingSystem) EvaluateMetrics(metrics map[string]interface{}) error {
	as.mutex.RLock()
	rules := make([]*AlertRule, 0)
	for _, rule := range as.rules {
		if rule.Enabled && rule.Condition.Type == MetricThreshold {
			rules = append(rules, rule)
		}
	}
	as.mutex.RUnlock()
	
	for _, rule := range rules {
		if as.evaluateMetricCondition(rule, metrics) {
			alert := as.createMetricAlert(rule, metrics)
			if err := as.triggerAlert(alert); err != nil {
				log.Printf("Failed to trigger alert for rule %s: %v", rule.ID, err)
			}
		}
	}
	
	return nil
}

// triggerAlert triggers an alert through the alerting pipeline
func (as *AlertingSystem) triggerAlert(alert *Alert) error {
	// Check throttling
	if as.throttle.ShouldThrottle(alert.RuleID) {
		log.Printf("Alert throttled: %s", alert.ID)
		return nil
	}
	
	// Record in history
	as.history.Add(alert)
	
	// Send through channels
	return as.sendAlert(alert)
}

// sendAlert sends an alert through configured channels
func (as *AlertingSystem) sendAlert(alert *Alert) error {
	as.mutex.RLock()
	rule := as.rules[alert.RuleID]
	channels := make([]AlertChannel, 0)
	for _, channelName := range rule.Channels {
		if channel, exists := as.channels[channelName]; exists {
			channels = append(channels, channel)
		}
	}
	as.mutex.RUnlock()
	
	if len(channels) == 0 {
		return fmt.Errorf("no valid channels configured for rule %s", alert.RuleID)
	}
	
	var wg sync.WaitGroup
	errors := make([]error, 0)
	var errorMutex sync.Mutex
	
	for _, channel := range channels {
		wg.Add(1)
		go func(ch AlertChannel) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(as.ctx, 30*time.Second)
			defer cancel()
			
			if err := ch.Send(ctx, alert); err != nil {
				log.Printf("Failed to send alert via %s: %v", ch.Name(), err)
				errorMutex.Lock()
				errors = append(errors, err)
				errorMutex.Unlock()
			} else {
				log.Printf("Alert sent via %s: %s", ch.Name(), alert.ID)
			}
		}(channel)
	}
	
	wg.Wait()
	
	// Handle escalation if enabled
	if rule.Escalation != nil && rule.Escalation.Enabled {
		as.escalation.ScheduleEscalation(alert, rule.Escalation)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to send alert via %d channels", len(errors))
	}
	
	return nil
}

// evaluateSpamClusterCondition evaluates a spam cluster condition
func (as *AlertingSystem) evaluateSpamClusterCondition(rule *AlertRule, cluster *models.SpamCluster) bool {
	switch rule.Condition.Metric {
	case "cluster_size":
		return as.compareValues(float64(cluster.Size), rule.Condition.Operator, rule.Condition.Threshold)
	case "confidence":
		return as.compareValues(cluster.Confidence, rule.Condition.Operator, rule.Condition.Threshold)
	case "unique_accounts":
		return as.compareValues(float64(cluster.UniqueAccounts), rule.Condition.Operator, rule.Condition.Threshold)
	case "tweets_per_minute":
		return as.compareValues(cluster.TweetsPerMinute, rule.Condition.Operator, rule.Condition.Threshold)
	default:
		return cluster.Size >= 3 // Default condition
	}
}

// evaluateTemporalPatternCondition evaluates a temporal pattern condition
func (as *AlertingSystem) evaluateTemporalPatternCondition(rule *AlertRule, pattern *temporal.TemporalPattern) bool {
	switch rule.Condition.Metric {
	case "confidence":
		return as.compareValues(pattern.Confidence, rule.Condition.Operator, rule.Condition.Threshold)
	case "event_count":
		return as.compareValues(float64(pattern.EventCount), rule.Condition.Operator, rule.Condition.Threshold)
	case "frequency":
		return as.compareValues(pattern.Frequency, rule.Condition.Operator, rule.Condition.Threshold)
	default:
		return pattern.Confidence >= 0.8 // Default condition
	}
}

// evaluateMetricCondition evaluates a metric threshold condition
func (as *AlertingSystem) evaluateMetricCondition(rule *AlertRule, metrics map[string]interface{}) bool {
	value, exists := metrics[rule.Condition.Metric]
	if !exists {
		return false
	}
	
	var numValue float64
	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float64:
		numValue = v
	case float32:
		numValue = float64(v)
	default:
		return false
	}
	
	return as.compareValues(numValue, rule.Condition.Operator, rule.Condition.Threshold)
}

// compareValues compares values using the specified operator
func (as *AlertingSystem) compareValues(value float64, operator ComparisonOperator, threshold float64) bool {
	switch operator {
	case GreaterThan:
		return value > threshold
	case LessThan:
		return value < threshold
	case GreaterThanOrEqual:
		return value >= threshold
	case LessThanOrEqual:
		return value <= threshold
	case Equal:
		return value == threshold
	case NotEqual:
		return value != threshold
	default:
		return false
	}
}

// createSpamClusterAlert creates an alert for a spam cluster
func (as *AlertingSystem) createSpamClusterAlert(rule *AlertRule, cluster *models.SpamCluster) *Alert {
	return &Alert{
		ID:        as.generateAlertID(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Title:     fmt.Sprintf("Spam Cluster Detected: %s", cluster.Pattern),
		Message:   as.formatSpamClusterMessage(cluster),
		Timestamp: time.Now(),
		Status:    Active,
		Metadata: map[string]interface{}{
			"cluster_id":      cluster.ID,
			"cluster_size":    cluster.Size,
			"confidence":      cluster.Confidence,
			"unique_accounts": cluster.UniqueAccounts,
			"pattern":         cluster.Pattern,
		},
		Context: &AlertContext{
			SpamCluster: cluster,
		},
	}
}

// createTemporalPatternAlert creates an alert for a temporal pattern
func (as *AlertingSystem) createTemporalPatternAlert(rule *AlertRule, pattern *temporal.TemporalPattern) *Alert {
	return &Alert{
		ID:        as.generateAlertID(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Title:     fmt.Sprintf("Temporal Pattern Detected: %s", as.patternTypeString(pattern.Type)),
		Message:   as.formatTemporalPatternMessage(pattern),
		Timestamp: time.Now(),
		Status:    Active,
		Metadata: map[string]interface{}{
			"pattern_id":    pattern.ID,
			"pattern_type":  pattern.Type,
			"confidence":    pattern.Confidence,
			"event_count":   pattern.EventCount,
			"frequency":     pattern.Frequency,
		},
		Context: &AlertContext{
			TemporalPattern: pattern,
		},
	}
}

// createMetricAlert creates an alert for a metric threshold
func (as *AlertingSystem) createMetricAlert(rule *AlertRule, metrics map[string]interface{}) *Alert {
	metricValue := metrics[rule.Condition.Metric]
	
	return &Alert{
		ID:        as.generateAlertID(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Title:     fmt.Sprintf("Metric Threshold Exceeded: %s", rule.Condition.Metric),
		Message:   as.formatMetricMessage(rule.Condition.Metric, metricValue, rule.Condition.Threshold),
		Timestamp: time.Now(),
		Status:    Active,
		Metadata: map[string]interface{}{
			"metric":    rule.Condition.Metric,
			"value":     metricValue,
			"threshold": rule.Condition.Threshold,
			"operator":  rule.Condition.Operator,
		},
		Context: &AlertContext{
			Metrics: metrics,
		},
	}
}

// formatSpamClusterMessage formats a message for spam cluster alerts
func (as *AlertingSystem) formatSpamClusterMessage(cluster *models.SpamCluster) string {
	return fmt.Sprintf(
		"A spam cluster with %d tweets from %d unique accounts has been detected. "+
			"Confidence: %.2f, Pattern: %s, Duration: %v",
		cluster.Size, cluster.UniqueAccounts, cluster.Confidence, cluster.Pattern, cluster.Duration,
	)
}

// formatTemporalPatternMessage formats a message for temporal pattern alerts
func (as *AlertingSystem) formatTemporalPatternMessage(pattern *temporal.TemporalPattern) string {
	return fmt.Sprintf(
		"A %s temporal pattern has been detected with %d events. "+
			"Confidence: %.2f, Frequency: %.2f events/hour",
		as.patternTypeString(pattern.Type), pattern.EventCount, pattern.Confidence, pattern.Frequency,
	)
}

// formatMetricMessage formats a message for metric alerts
func (as *AlertingSystem) formatMetricMessage(metric string, value interface{}, threshold float64) string {
	return fmt.Sprintf(
		"Metric '%s' has reached %v, which exceeds the threshold of %.2f",
		metric, value, threshold,
	)
}

// Helper functions

func (as *AlertingSystem) generateRuleID() string {
	return fmt.Sprintf("rule_%d", time.Now().UnixNano())
}

func (as *AlertingSystem) generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

func (as *AlertingSystem) validateRule(rule *AlertRule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}
	if len(rule.Channels) == 0 {
		return fmt.Errorf("rule must have at least one channel")
	}
	return nil
}

func (as *AlertingSystem) channelTypeString(channelType ChannelType) string {
	switch channelType {
	case Email:
		return "email"
	case Slack:
		return "slack"
	case Webhook:
		return "webhook"
	case SMS:
		return "sms"
	case PagerDuty:
		return "pagerduty"
	case Discord:
		return "discord"
	case Teams:
		return "teams"
	default:
		return "unknown"
	}
}

func (as *AlertingSystem) patternTypeString(patternType temporal.PatternType) string {
	switch patternType {
	case temporal.BurstPattern:
		return "burst"
	case temporal.PeriodicPattern:
		return "periodic"
	case temporal.CoordinatedPattern:
		return "coordinated"
	case temporal.GradualRampPattern:
		return "gradual_ramp"
	case temporal.FlashMobPattern:
		return "flash_mob"
	default:
		return "unknown"
	}
}

// GetRules returns all alert rules
func (as *AlertingSystem) GetRules() map[string]*AlertRule {
	as.mutex.RLock()
	defer as.mutex.RUnlock()
	
	rules := make(map[string]*AlertRule)
	for id, rule := range as.rules {
		rules[id] = rule
	}
	return rules
}

// GetChannels returns all alert channels
func (as *AlertingSystem) GetChannels() map[string]AlertChannel {
	as.mutex.RLock()
	defer as.mutex.RUnlock()
	
	channels := make(map[string]AlertChannel)
	for name, channel := range as.channels {
		channels[name] = channel
	}
	return channels
}

// GetHistory returns alert history
func (as *AlertingSystem) GetHistory() []*Alert {
	return as.history.GetAll()
}

// Shutdown gracefully shuts down the alerting system
func (as *AlertingSystem) Shutdown() error {
	log.Println("Shutting down alerting system...")
	as.cancel()
	as.wg.Wait()
	log.Println("Alerting system shutdown complete")
	return nil
}

// ThrottleManager manages alert throttling
type ThrottleManager struct {
	throttles map[string]*throttleState
	mutex     sync.RWMutex
}

type throttleState struct {
	count     int
	resetTime time.Time
}

// NewThrottleManager creates a new throttle manager
func NewThrottleManager() *ThrottleManager {
	return &ThrottleManager{
		throttles: make(map[string]*throttleState),
	}
}

// ShouldThrottle checks if an alert should be throttled
func (tm *ThrottleManager) ShouldThrottle(ruleID string) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	state, exists := tm.throttles[ruleID]
	now := time.Now()
	
	if !exists || now.After(state.resetTime) {
		tm.throttles[ruleID] = &throttleState{
			count:     1,
			resetTime: now.Add(5 * time.Minute), // Default 5-minute window
		}
		return false
	}
	
	state.count++
	// Throttle after 3 alerts in the time window
	return state.count > 3
}

// EscalationManager manages alert escalation
type EscalationManager struct {
	escalations map[string]*EscalationState
	mutex       sync.RWMutex
}

type EscalationState struct {
	alert      *Alert
	config     *EscalationConfig
	currentLevel int
	nextEscalation time.Time
}

// NewEscalationManager creates a new escalation manager
func NewEscalationManager() *EscalationManager {
	return &EscalationManager{
		escalations: make(map[string]*EscalationState),
	}
}

// ScheduleEscalation schedules an escalation for an alert
func (em *EscalationManager) ScheduleEscalation(alert *Alert, config *EscalationConfig) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	if len(config.Levels) == 0 {
		return
	}
	
	firstLevel := config.Levels[0]
	em.escalations[alert.ID] = &EscalationState{
		alert:          alert,
		config:         config,
		currentLevel:   0,
		nextEscalation: time.Now().Add(firstLevel.After),
	}
}

// AlertHistory manages alert history
type AlertHistory struct {
	alerts   []*Alert
	maxSize  int
	mutex    sync.RWMutex
}

// NewAlertHistory creates a new alert history
func NewAlertHistory(maxSize int) *AlertHistory {
	return &AlertHistory{
		alerts:  make([]*Alert, 0),
		maxSize: maxSize,
	}
}

// Add adds an alert to history
func (ah *AlertHistory) Add(alert *Alert) {
	ah.mutex.Lock()
	defer ah.mutex.Unlock()
	
	ah.alerts = append(ah.alerts, alert)
	
	// Trim if over capacity
	if len(ah.alerts) > ah.maxSize {
		ah.alerts = ah.alerts[len(ah.alerts)-ah.maxSize:]
	}
}

// GetAll returns all alerts in history
func (ah *AlertHistory) GetAll() []*Alert {
	ah.mutex.RLock()
	defer ah.mutex.RUnlock()
	
	result := make([]*Alert, len(ah.alerts))
	copy(result, ah.alerts)
	return result
}

// SlackChannel implements AlertChannel for Slack
type SlackChannel struct {
	name     string
	webhook  string
	username string
	channel  string
}

// NewSlackChannel creates a new Slack alert channel
func NewSlackChannel(name, webhook, username, channel string) *SlackChannel {
	return &SlackChannel{
		name:     name,
		webhook:  webhook,
		username: username,
		channel:  channel,
	}
}

func (sc *SlackChannel) Name() string {
	return sc.name
}

func (sc *SlackChannel) Type() ChannelType {
	return Slack
}

func (sc *SlackChannel) Send(ctx context.Context, alert *Alert) error {
	payload := map[string]interface{}{
		"username": sc.username,
		"channel":  sc.channel,
		"text":     fmt.Sprintf("*%s*\n%s", alert.Title, alert.Message),
		"attachments": []map[string]interface{}{
			{
				"color": sc.getSeverityColor(alert.Severity),
				"fields": []map[string]interface{}{
					{"title": "Severity", "value": sc.getSeverityString(alert.Severity), "short": true},
					{"title": "Rule", "value": alert.RuleName, "short": true},
					{"title": "Timestamp", "value": alert.Timestamp.Format(time.RFC3339), "short": true},
				},
			},
		},
	}
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", sc.webhook, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slack API error %d: %s", resp.StatusCode, string(body))
	}
	
	return nil
}

func (sc *SlackChannel) HealthCheck() error {
	// Simple connectivity test
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("HEAD", sc.webhook, nil)
	if err != nil {
		return err
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

func (sc *SlackChannel) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"webhook":  sc.webhook,
		"username": sc.username,
		"channel":  sc.channel,
	}
}

func (sc *SlackChannel) getSeverityColor(severity AlertSeverity) string {
	switch severity {
	case Info:
		return "good"
	case Warning:
		return "warning"
	case Error:
		return "danger"
	case Critical:
		return "danger"
	default:
		return "warning"
	}
}

func (sc *SlackChannel) getSeverityString(severity AlertSeverity) string {
	switch severity {
	case Info:
		return "Info"
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	case Critical:
		return "Critical"
	default:
		return "Unknown"
	}
}
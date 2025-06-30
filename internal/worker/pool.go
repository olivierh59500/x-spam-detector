package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"x-spam-detector/internal/models"
)

// Task represents a unit of work to be processed
type Task interface {
	Execute() error
	GetID() string
	GetPriority() Priority
}

// Priority defines task execution priority
type Priority int

const (
	Low Priority = iota
	Medium
	High
	Critical
)

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID    string
	Success   bool
	Error     error
	Duration  time.Duration
	Result    interface{}
	Timestamp time.Time
}

// WorkerPool manages a pool of workers for concurrent task processing
type WorkerPool struct {
	ctx        context.Context
	cancel     context.CancelFunc
	workers    int
	taskQueue  chan Task
	resultChan chan TaskResult
	wg         sync.WaitGroup
	
	// Statistics
	totalTasks    int64
	completedTasks int64
	failedTasks   int64
	avgDuration   time.Duration
	mutex         sync.RWMutex
	
	// Configuration
	queueSize    int
	workerTimeout time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers, queueSize int, workerTimeout time.Duration) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		ctx:           ctx,
		cancel:        cancel,
		workers:       workers,
		taskQueue:     make(chan Task, queueSize),
		resultChan:    make(chan TaskResult, queueSize),
		queueSize:     queueSize,
		workerTimeout: workerTimeout,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.taskQueue)
	wp.cancel()
	wp.wg.Wait()
	close(wp.resultChan)
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(task Task) error {
	select {
	case wp.taskQueue <- task:
		atomic.AddInt64(&wp.totalTasks, 1)
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("task queue is full")
	}
}

// GetResults returns a channel for receiving task results
func (wp *WorkerPool) GetResults() <-chan TaskResult {
	return wp.resultChan
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() WorkerStats {
	wp.mutex.RLock()
	defer wp.mutex.RUnlock()
	
	total := atomic.LoadInt64(&wp.totalTasks)
	completed := atomic.LoadInt64(&wp.completedTasks)
	failed := atomic.LoadInt64(&wp.failedTasks)
	
	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total)
	}
	
	return WorkerStats{
		Workers:       wp.workers,
		QueueSize:     wp.queueSize,
		QueueLength:   len(wp.taskQueue),
		TotalTasks:    total,
		CompletedTasks: completed,
		FailedTasks:   failed,
		SuccessRate:   successRate,
		AvgDuration:   wp.avgDuration,
	}
}

// worker is the worker goroutine that processes tasks
func (wp *WorkerPool) worker(_ int) {
	defer wp.wg.Done()
	
	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // Channel closed, worker should exit
			}
			wp.processTask(task)
			
		case <-wp.ctx.Done():
			return // Context cancelled, worker should exit
		}
	}
}

// processTask processes a single task
func (wp *WorkerPool) processTask(task Task) {
	start := time.Now()
	
	result := TaskResult{
		TaskID:    task.GetID(),
		Timestamp: start,
	}
	
	// Create timeout context for task execution
	taskCtx, cancel := context.WithTimeout(wp.ctx, wp.workerTimeout)
	defer cancel()
	
	// Execute task with timeout
	done := make(chan error, 1)
	go func() {
		done <- task.Execute()
	}()
	
	select {
	case err := <-done:
		result.Error = err
		result.Success = err == nil
		
	case <-taskCtx.Done():
		result.Error = fmt.Errorf("task timeout")
		result.Success = false
	}
	
	result.Duration = time.Since(start)
	
	// Update statistics
	if result.Success {
		atomic.AddInt64(&wp.completedTasks, 1)
	} else {
		atomic.AddInt64(&wp.failedTasks, 1)
	}
	
	wp.updateAvgDuration(result.Duration)
	
	// Send result
	select {
	case wp.resultChan <- result:
	default:
		// Result channel is full, drop the result
	}
}

// updateAvgDuration updates the average task duration
func (wp *WorkerPool) updateAvgDuration(duration time.Duration) {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()
	
	// Simple moving average
	if wp.avgDuration == 0 {
		wp.avgDuration = duration
	} else {
		wp.avgDuration = (wp.avgDuration + duration) / 2
	}
}

// WorkerStats represents worker pool statistics
type WorkerStats struct {
	Workers        int           `json:"workers"`
	QueueSize      int           `json:"queue_size"`
	QueueLength    int           `json:"queue_length"`
	TotalTasks     int64         `json:"total_tasks"`
	CompletedTasks int64         `json:"completed_tasks"`
	FailedTasks    int64         `json:"failed_tasks"`
	SuccessRate    float64       `json:"success_rate"`
	AvgDuration    time.Duration `json:"avg_duration"`
}

// SpamDetectionTask represents a spam detection task
type SpamDetectionTask struct {
	ID      string
	Tweets  []*models.Tweet
	Engine  SpamDetectionEngine
	Result  chan *models.DetectionResult
	priority Priority
}

// SpamDetectionEngine interface for dependency injection
type SpamDetectionEngine interface {
	AddTweets([]*models.Tweet) error
	DetectSpam() (*models.DetectionResult, error)
}

// NewSpamDetectionTask creates a new spam detection task
func NewSpamDetectionTask(id string, tweets []*models.Tweet, engine SpamDetectionEngine, priority Priority) *SpamDetectionTask {
	return &SpamDetectionTask{
		ID:       id,
		Tweets:   tweets,
		Engine:   engine,
		Result:   make(chan *models.DetectionResult, 1),
		priority: priority,
	}
}

// Execute executes the spam detection task
func (sdt *SpamDetectionTask) Execute() error {
	// Add tweets to engine
	if err := sdt.Engine.AddTweets(sdt.Tweets); err != nil {
		return fmt.Errorf("failed to add tweets: %w", err)
	}
	
	// Perform detection
	result, err := sdt.Engine.DetectSpam()
	if err != nil {
		return fmt.Errorf("failed to detect spam: %w", err)
	}
	
	// Send result
	select {
	case sdt.Result <- result:
	default:
		return fmt.Errorf("failed to send result")
	}
	
	return nil
}

// GetID returns the task ID
func (sdt *SpamDetectionTask) GetID() string {
	return sdt.ID
}

// GetPriority returns the task priority
func (sdt *SpamDetectionTask) GetPriority() Priority {
	return sdt.priority
}

// GetResult returns the detection result
func (sdt *SpamDetectionTask) GetResult() *models.DetectionResult {
	select {
	case result := <-sdt.Result:
		return result
	default:
		return nil
	}
}

// BatchProcessor processes tweets in batches using worker pools
type BatchProcessor struct {
	workerPool   *WorkerPool
	batchSize    int
	maxBatches   int
	engineFactory func() SpamDetectionEngine
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(workers, queueSize, batchSize, maxBatches int, engineFactory func() SpamDetectionEngine) *BatchProcessor {
	workerPool := NewWorkerPool(workers, queueSize, 30*time.Second)
	
	return &BatchProcessor{
		workerPool:    workerPool,
		batchSize:     batchSize,
		maxBatches:    maxBatches,
		engineFactory: engineFactory,
	}
}

// Start starts the batch processor
func (bp *BatchProcessor) Start() {
	bp.workerPool.Start()
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() {
	bp.workerPool.Stop()
}

// ProcessTweets processes tweets in parallel batches
func (bp *BatchProcessor) ProcessTweets(tweets []*models.Tweet) ([]*models.DetectionResult, error) {
	if len(tweets) == 0 {
		return nil, nil
	}
	
	// Create batches
	batches := bp.createBatches(tweets)
	if len(batches) > bp.maxBatches {
		return nil, fmt.Errorf("too many batches: %d > %d", len(batches), bp.maxBatches)
	}
	
	// Submit tasks
	tasks := make([]*SpamDetectionTask, len(batches))
	for i, batch := range batches {
		taskID := fmt.Sprintf("batch_%d_%d", time.Now().Unix(), i)
		engine := bp.engineFactory()
		task := NewSpamDetectionTask(taskID, batch, engine, Medium)
		tasks[i] = task
		
		if err := bp.workerPool.Submit(task); err != nil {
			return nil, fmt.Errorf("failed to submit task %s: %w", taskID, err)
		}
	}
	
	// Collect results
	results := make([]*models.DetectionResult, 0, len(tasks))
	timeout := time.After(5 * time.Minute)
	
	for _, task := range tasks {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for batch results")
		default:
			if result := task.GetResult(); result != nil {
				results = append(results, result)
			}
		}
	}
	
	return results, nil
}

// createBatches splits tweets into batches
func (bp *BatchProcessor) createBatches(tweets []*models.Tweet) [][]*models.Tweet {
	var batches [][]*models.Tweet
	
	for i := 0; i < len(tweets); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(tweets) {
			end = len(tweets)
		}
		batches = append(batches, tweets[i:end])
	}
	
	return batches
}

// GetStats returns batch processor statistics
func (bp *BatchProcessor) GetStats() WorkerStats {
	return bp.workerPool.GetStats()
}
// Package retry provides retry mechanisms with exponential backoff and circuit breaker functionality.
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/geekxflood/common/config"
)

// RetryConfig holds configuration for the retry mechanism
type RetryConfig struct {
	MaxAttempts       int           `json:"max_attempts"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	Jitter            bool          `json:"jitter"`
	JitterRange       float64       `json:"jitter_range"`
	EnableCircuitBreaker bool       `json:"enable_circuit_breaker"`
	CircuitBreakerConfig CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	HalfOpenMaxCalls int           `json:"half_open_max_calls"`
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		JitterRange:       0.1,
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			HalfOpenMaxCalls: 3,
		},
	}
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// String returns the string representation of a circuit state
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	halfOpenCalls    int
	mu               sync.RWMutex
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func(ctx context.Context, attempt int) error

// RetryResult represents the result of a retry operation
type RetryResult struct {
	Success      bool          `json:"success"`
	Attempts     int           `json:"attempts"`
	TotalTime    time.Duration `json:"total_time"`
	LastError    error         `json:"last_error,omitempty"`
	CircuitState string        `json:"circuit_state,omitempty"`
}

// RetryStats tracks retry statistics
type RetryStats struct {
	TotalRetries      int64         `json:"total_retries"`
	SuccessfulRetries int64         `json:"successful_retries"`
	FailedRetries     int64         `json:"failed_retries"`
	AverageAttempts   float64       `json:"average_attempts"`
	AverageRetryTime  time.Duration `json:"average_retry_time"`
	TotalRetryTime    time.Duration `json:"total_retry_time"`
	CircuitBreakerTrips int64       `json:"circuit_breaker_trips"`
}

// Retryer provides retry functionality with exponential backoff and circuit breaker
type Retryer struct {
	config         *RetryConfig
	circuitBreaker *CircuitBreaker
	stats          *RetryStats
	mu             sync.RWMutex
}

// NewRetryer creates a new retryer with the given configuration
func NewRetryer(cfg config.Provider) (*Retryer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration provider cannot be nil")
	}

	// Load configuration
	retryConfig := DefaultRetryConfig()
	
	if maxAttempts, err := cfg.GetInt("retry.max_attempts", retryConfig.MaxAttempts); err == nil {
		retryConfig.MaxAttempts = maxAttempts
	}
	
	if initialDelay, err := cfg.GetDuration("retry.initial_delay", retryConfig.InitialDelay); err == nil {
		retryConfig.InitialDelay = initialDelay
	}
	
	if maxDelay, err := cfg.GetDuration("retry.max_delay", retryConfig.MaxDelay); err == nil {
		retryConfig.MaxDelay = maxDelay
	}
	
	if backoffMultiplier, err := cfg.GetFloat("retry.backoff_multiplier", retryConfig.BackoffMultiplier); err == nil {
		retryConfig.BackoffMultiplier = backoffMultiplier
	}
	
	if jitter, err := cfg.GetBool("retry.jitter", retryConfig.Jitter); err == nil {
		retryConfig.Jitter = jitter
	}
	
	if enableCircuitBreaker, err := cfg.GetBool("retry.enable_circuit_breaker", retryConfig.EnableCircuitBreaker); err == nil {
		retryConfig.EnableCircuitBreaker = enableCircuitBreaker
	}

	var circuitBreaker *CircuitBreaker
	if retryConfig.EnableCircuitBreaker {
		circuitBreaker = &CircuitBreaker{
			config: retryConfig.CircuitBreakerConfig,
			state:  CircuitClosed,
		}
	}

	retryer := &Retryer{
		config:         retryConfig,
		circuitBreaker: circuitBreaker,
		stats:          &RetryStats{},
	}

	return retryer, nil
}

// Retry executes a function with retry logic
func (r *Retryer) Retry(ctx context.Context, fn RetryableFunc) *RetryResult {
	r.mu.Lock()
	r.stats.TotalRetries++
	r.mu.Unlock()

	startTime := time.Now()
	var lastError error
	
	// Check circuit breaker
	if r.circuitBreaker != nil && !r.circuitBreaker.CanExecute() {
		return &RetryResult{
			Success:      false,
			Attempts:     0,
			TotalTime:    time.Since(startTime),
			LastError:    fmt.Errorf("circuit breaker is open"),
			CircuitState: r.circuitBreaker.GetState().String(),
		}
	}

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return &RetryResult{
				Success:      false,
				Attempts:     attempt - 1,
				TotalTime:    time.Since(startTime),
				LastError:    ctx.Err(),
				CircuitState: r.getCircuitState(),
			}
		default:
		}

		// Execute function
		err := fn(ctx, attempt)
		if err == nil {
			// Success
			if r.circuitBreaker != nil {
				r.circuitBreaker.RecordSuccess()
			}
			
			totalTime := time.Since(startTime)
			r.recordSuccess(attempt, totalTime)
			
			return &RetryResult{
				Success:      true,
				Attempts:     attempt,
				TotalTime:    totalTime,
				CircuitState: r.getCircuitState(),
			}
		}

		lastError = err
		
		// Record failure
		if r.circuitBreaker != nil {
			r.circuitBreaker.RecordFailure()
		}

		// Don't wait after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay for next attempt
		delay := r.calculateDelay(attempt)
		
		// Wait before retry
		select {
		case <-ctx.Done():
			return &RetryResult{
				Success:      false,
				Attempts:     attempt,
				TotalTime:    time.Since(startTime),
				LastError:    ctx.Err(),
				CircuitState: r.getCircuitState(),
			}
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All attempts failed
	totalTime := time.Since(startTime)
	r.recordFailure(r.config.MaxAttempts, totalTime)
	
	return &RetryResult{
		Success:      false,
		Attempts:     r.config.MaxAttempts,
		TotalTime:    totalTime,
		LastError:    lastError,
		CircuitState: r.getCircuitState(),
	}
}

// calculateDelay calculates the delay for the next retry attempt
func (r *Retryer) calculateDelay(attempt int) time.Duration {
	// Exponential backoff
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffMultiplier, float64(attempt-1))
	
	// Apply maximum delay limit
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Apply jitter if enabled
	if r.config.Jitter {
		jitterRange := delay * r.config.JitterRange
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange
		delay += jitter
		
		// Ensure delay is not negative
		if delay < 0 {
			delay = float64(r.config.InitialDelay)
		}
	}

	return time.Duration(delay)
}

// recordSuccess records a successful retry operation
func (r *Retryer) recordSuccess(attempts int, totalTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.stats.SuccessfulRetries++
	r.stats.TotalRetryTime += totalTime
	
	// Update average attempts
	totalOps := r.stats.SuccessfulRetries + r.stats.FailedRetries
	r.stats.AverageAttempts = (r.stats.AverageAttempts*float64(totalOps-1) + float64(attempts)) / float64(totalOps)
	
	// Update average retry time
	r.stats.AverageRetryTime = r.stats.TotalRetryTime / time.Duration(totalOps)
}

// recordFailure records a failed retry operation
func (r *Retryer) recordFailure(attempts int, totalTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.stats.FailedRetries++
	r.stats.TotalRetryTime += totalTime
	
	// Update average attempts
	totalOps := r.stats.SuccessfulRetries + r.stats.FailedRetries
	r.stats.AverageAttempts = (r.stats.AverageAttempts*float64(totalOps-1) + float64(attempts)) / float64(totalOps)
	
	// Update average retry time
	r.stats.AverageRetryTime = r.stats.TotalRetryTime / time.Duration(totalOps)
}

// getCircuitState returns the current circuit breaker state as a string
func (r *Retryer) getCircuitState() string {
	if r.circuitBreaker == nil {
		return "disabled"
	}
	return r.circuitBreaker.GetState().String()
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == CircuitOpen && time.Since(cb.lastFailureTime) > cb.config.Timeout {
				cb.state = CircuitHalfOpen
				cb.halfOpenCalls = 0
			}
			cb.mu.Unlock()
			cb.mu.RLock()
		}
		return cb.state == CircuitHalfOpen
	case CircuitHalfOpen:
		return cb.halfOpenCalls < cb.config.HalfOpenMaxCalls
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount = 0
	case CircuitHalfOpen:
		cb.successCount++
		cb.halfOpenCalls++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.successCount = 0
		cb.halfOpenCalls = 0
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state.String(),
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"last_failure_time": cb.lastFailureTime,
		"half_open_calls":   cb.halfOpenCalls,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCalls = 0
}

// GetStats returns retry statistics
func (r *Retryer) GetStats() *RetryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *r.stats
	return &stats
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (r *Retryer) GetCircuitBreakerStats() map[string]interface{} {
	if r.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := r.circuitBreaker.GetStats()
	stats["enabled"] = true
	return stats
}

// GetConfig returns the retry configuration
func (r *Retryer) GetConfig() *RetryConfig {
	return r.config
}

// UpdateConfig updates the retry configuration
func (r *Retryer) UpdateConfig(config *RetryConfig) {
	r.config = config
	
	// Update circuit breaker config if enabled
	if r.circuitBreaker != nil {
		r.circuitBreaker.mu.Lock()
		r.circuitBreaker.config = config.CircuitBreakerConfig
		r.circuitBreaker.mu.Unlock()
	}
}

// ResetStats resets retry statistics
func (r *Retryer) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats = &RetryStats{}
}

// ResetCircuitBreaker resets the circuit breaker
func (r *Retryer) ResetCircuitBreaker() {
	if r.circuitBreaker != nil {
		r.circuitBreaker.Reset()
	}
}

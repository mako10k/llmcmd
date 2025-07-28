package openai

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryConfig holds configuration for retry mechanism
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts
	BaseDelay       time.Duration // Base delay between retries
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Exponential backoff factor
	RetriableErrors []string      // List of retriable error types
}

// DefaultRetryConfig returns sensible default retry settings
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		BaseDelay:     1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetriableErrors: []string{
			"rate_limit_exceeded",
			"server_error",
			"service_unavailable",
			"timeout",
		},
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
	Retryable  bool
}

func (r RetryableError) Error() string {
	return r.Err.Error()
}

// ChatCompletionWithRetry sends a chat completion request with retry mechanism
func (c *Client) ChatCompletionWithRetry(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	config := DefaultRetryConfig()
	
	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(config.BaseDelay) * math.Pow(config.BackoffFactor, float64(attempt-1)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			
			if c.stats.Verbose {
				fmt.Printf("[RETRY] Attempt %d/%d after %v\n", attempt, config.MaxRetries, delay)
			}
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		
		// Attempt request
		resp, err := c.ChatCompletion(ctx, req)
		if err == nil {
			if attempt > 0 && c.stats.Verbose {
				fmt.Printf("[RETRY] Success after %d attempts\n", attempt)
			}
			return resp, nil
		}
		
		// Check if error is retryable
		retryErr := classifyError(err)
		if !retryErr.Retryable || attempt >= config.MaxRetries {
			return nil, err
		}
		
		lastErr = err
		c.stats.RetryCount++
		
		// Handle rate limit with custom delay
		if retryErr.RetryAfter > 0 {
			if c.stats.Verbose {
				fmt.Printf("[RETRY] Rate limited, waiting %v\n", retryErr.RetryAfter)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryErr.RetryAfter):
			}
		}
	}
	
	return nil, fmt.Errorf("request failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// classifyError determines if an error is retryable and extracts retry information
func classifyError(err error) RetryableError {
	errStr := strings.ToLower(err.Error())
	
	// Rate limit errors
	if strings.Contains(errStr, "rate_limit_exceeded") || strings.Contains(errStr, "429") {
		return RetryableError{
			Err:        err,
			RetryAfter: 5 * time.Second, // Default rate limit backoff
			Retryable:  true,
		}
	}
	
	// Server errors (5xx)
	if strings.Contains(errStr, "server_error") || 
	   strings.Contains(errStr, "500") || 
	   strings.Contains(errStr, "502") || 
	   strings.Contains(errStr, "503") || 
	   strings.Contains(errStr, "504") {
		return RetryableError{
			Err:       err,
			Retryable: true,
		}
	}
	
	// Timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return RetryableError{
			Err:       err,
			Retryable: true,
		}
	}
	
	// Network errors
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return RetryableError{
			Err:       err,
			Retryable: true,
		}
	}
	
	// Non-retryable error
	return RetryableError{
		Err:       err,
		Retryable: false,
	}
}

// Enhanced error response with helpful messages
func enhanceErrorMessage(err error) error {
	errStr := strings.ToLower(err.Error())
	
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401") {
		return fmt.Errorf("%w\n\nTroubleshooting:\n- Check your OPENAI_API_KEY environment variable\n- Verify the API key is valid and active\n- Ensure you have sufficient OpenAI credits", err)
	}
	
	if strings.Contains(errStr, "rate_limit_exceeded") || strings.Contains(errStr, "429") {
		return fmt.Errorf("%w\n\nTroubleshooting:\n- You've exceeded OpenAI's rate limits\n- Wait a moment and try again\n- Consider upgrading your OpenAI plan for higher limits", err)
	}
	
	if strings.Contains(errStr, "insufficient_quota") {
		return fmt.Errorf("%w\n\nTroubleshooting:\n- Your OpenAI account has insufficient credits\n- Add payment method or purchase more credits\n- Check your usage at https://platform.openai.com/usage", err)
	}
	
	if strings.Contains(errStr, "model_not_found") {
		return fmt.Errorf("%w\n\nTroubleshooting:\n- The specified model is not available\n- Try using 'gpt-4o-mini' or 'gpt-3.5-turbo'\n- Set LLMCMD_MODEL environment variable", err)
	}
	
	return err
}

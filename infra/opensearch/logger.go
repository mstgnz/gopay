package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

// PaymentLog represents a structured payment log entry
type PaymentLog struct {
	Timestamp   time.Time   `json:"timestamp"`
	TenantID    string      `json:"tenant_id,omitempty"`
	Provider    string      `json:"provider"`
	Method      string      `json:"method"`
	Endpoint    string      `json:"endpoint"`
	RequestID   string      `json:"request_id"`
	UserAgent   string      `json:"user_agent,omitempty"`
	ClientIP    string      `json:"client_ip,omitempty"`
	Request     RequestLog  `json:"request"`
	Response    ResponseLog `json:"response"`
	PaymentInfo PaymentInfo `json:"payment_info,omitempty"`
	Error       ErrorInfo   `json:"error,omitempty"`
}

// RequestLog represents request details
type RequestLog struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// ResponseLog represents response details
type ResponseLog struct {
	StatusCode       int               `json:"status_code"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             string            `json:"body,omitempty"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
}

// PaymentInfo represents payment-specific information
type PaymentInfo struct {
	PaymentID     string  `json:"payment_id,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	CustomerEmail string  `json:"customer_email,omitempty"`
	Status        string  `json:"status,omitempty"`
	Use3D         bool    `json:"use_3d,omitempty"`
}

// ErrorInfo represents error details
type ErrorInfo struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Logger handles OpenSearch logging operations
type Logger struct {
	client *Client
}

// NewLogger creates a new OpenSearch logger
func NewLogger(client *Client) *Logger {
	return &Logger{
		client: client,
	}
}

// LogPaymentRequest logs a payment request to OpenSearch
func (l *Logger) LogPaymentRequest(ctx context.Context, log PaymentLog) error {
	if !l.client.IsEnabled() {
		return nil // Logging disabled
	}

	// Set timestamp if not provided
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// Generate request ID if not provided
	if log.RequestID == "" {
		log.RequestID = uuid.New().String()
	}

	// Determine the appropriate index based on tenant and provider
	indexName := l.client.GetLogIndexName(log.TenantID, log.Provider)

	// Convert log to JSON
	logJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	// Index the document
	req := opensearchapi.IndexRequest{
		Index: indexName,
		Body:  bytes.NewReader(logJSON),
	}

	res, err := req.Do(ctx, l.client.GetClient())
	if err != nil {
		return fmt.Errorf("failed to index log: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("opensearch error: %s", res.String())
	}

	return nil
}

// SearchLogs searches for payment logs based on criteria
func (l *Logger) SearchLogs(ctx context.Context, tenantID, provider string, query map[string]any) ([]PaymentLog, error) {
	if !l.client.IsEnabled() {
		return nil, fmt.Errorf("logging is disabled")
	}

	indexName := l.client.GetLogIndexName(tenantID, provider)

	// Build search query
	searchQuery := map[string]any{
		"query": query,
		"sort": []map[string]any{
			{"timestamp": map[string]string{"order": "desc"}},
		},
		"size": 100, // Limit results
	}

	queryJSON, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Perform search
	req := opensearchapi.SearchRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(queryJSON),
	}

	res, err := req.Do(ctx, l.client.GetClient())
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("opensearch search error: %s", res.String())
	}

	// Parse search results
	var searchResult struct {
		Hits struct {
			Hits []struct {
				Source PaymentLog `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	// Extract logs from search results
	logs := make([]PaymentLog, len(searchResult.Hits.Hits))
	for i, hit := range searchResult.Hits.Hits {
		logs[i] = hit.Source
	}

	return logs, nil
}

// GetPaymentLogs retrieves logs for a specific payment ID
func (l *Logger) GetPaymentLogs(ctx context.Context, tenantID, provider, paymentID string) ([]PaymentLog, error) {
	query := map[string]any{
		"match": map[string]any{
			"payment_info.payment_id": paymentID,
		},
	}

	return l.SearchLogs(ctx, tenantID, provider, query)
}

// GetRecentErrorLogs retrieves recent error logs for a provider
func (l *Logger) GetRecentErrorLogs(ctx context.Context, tenantID, provider string, hours int) ([]PaymentLog, error) {
	query := map[string]any{
		"bool": map[string]any{
			"must": []map[string]any{
				{
					"range": map[string]any{
						"timestamp": map[string]any{
							"gte": fmt.Sprintf("now-%dh", hours),
						},
					},
				},
				{
					"exists": map[string]any{
						"field": "error.code",
					},
				},
			},
		},
	}

	return l.SearchLogs(ctx, tenantID, provider, query)
}

// GetProviderStats retrieves statistics for a provider
func (l *Logger) GetProviderStats(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error) {
	if !l.client.IsEnabled() {
		return nil, fmt.Errorf("logging is disabled")
	}

	indexName := l.client.GetLogIndexName(tenantID, provider)

	// Build aggregation query
	aggQuery := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"timestamp": map[string]any{
					"gte": fmt.Sprintf("now-%dh", hours),
				},
			},
		},
		"aggs": map[string]any{
			"total_requests": map[string]any{
				"value_count": map[string]any{
					"field": "request_id",
				},
			},
			"success_count": map[string]any{
				"filter": map[string]any{
					"range": map[string]any{
						"response.status_code": map[string]any{
							"gte": 200,
							"lt":  300,
						},
					},
				},
			},
			"error_count": map[string]any{
				"filter": map[string]any{
					"exists": map[string]any{
						"field": "error.code",
					},
				},
			},
			"avg_processing_time": map[string]any{
				"avg": map[string]any{
					"field": "response.processing_time_ms",
				},
			},
			"status_codes": map[string]any{
				"terms": map[string]any{
					"field": "response.status_code",
					"size":  10,
				},
			},
		},
		"size": 0, // We only want aggregations
	}

	queryJSON, err := json.Marshal(aggQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregation query: %w", err)
	}

	// Perform search
	req := opensearchapi.SearchRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(queryJSON),
	}

	res, err := req.Do(ctx, l.client.GetClient())
	if err != nil {
		return nil, fmt.Errorf("aggregation search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("opensearch aggregation error: %s", res.String())
	}

	// Parse aggregation results
	var result map[string]any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode aggregation results: %w", err)
	}

	return result, nil
}

// SanitizeForLog removes sensitive information from data before logging
func SanitizeForLog(data string) string {
	// Replace common sensitive fields
	sensitiveFields := []string{
		"cardNumber", "card_number", "cvv", "cvc", "cardHolderName", "card_holder_name",
		"apiKey", "api_key", "secretKey", "secret_key", "password", "token",
		"authorization", "x-api-key", "x-secret-key",
	}

	result := data
	for _, field := range sensitiveFields {
		// Regex patterns for different formats
		patterns := []string{
			fmt.Sprintf(`"%s"\s*:\s*"[^"]*"`, field), // JSON format with double quotes
			fmt.Sprintf(`"%s"\s*:\s*'[^']*'`, field), // JSON format with single quotes
			fmt.Sprintf(`%s=\w+`, field),             // URL parameter format
		}

		for _, pattern := range patterns {
			// Use regex.MustCompile for pattern matching and replacement
			re := regexp.MustCompile(pattern)
			result = re.ReplaceAllString(result, fmt.Sprintf(`"%s":"***REDACTED***"`, field))
		}
	}

	return result
}

// LogSystemEvent logs a system event to OpenSearch
func (l *Logger) LogSystemEvent(ctx context.Context, log any) error {
	if !l.client.IsEnabled() {
		return nil // Logging disabled
	}

	// Use system logs index
	indexName := "gopay-system-logs"

	// Convert log to JSON
	logJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal system log: %w", err)
	}

	// Index the document
	req := opensearchapi.IndexRequest{
		Index: indexName,
		Body:  bytes.NewReader(logJSON),
	}

	res, err := req.Do(ctx, l.client.GetClient())
	if err != nil {
		return fmt.Errorf("failed to index system log: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("opensearch system log error: %s", res.String())
	}

	return nil
}

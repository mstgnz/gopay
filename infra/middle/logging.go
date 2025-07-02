package middle

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/opensearch"
)

// responseWriter wraps http.ResponseWriter to capture response data
type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	startTime  time.Time
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
		startTime:      time.Now(),
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// PaymentLoggingMiddleware creates a middleware for logging payment requests/responses
func PaymentLoggingMiddleware(logger *opensearch.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for non-payment endpoints
			if !isPaymentEndpoint(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Generate request ID
			requestID := uuid.New().String()
			r.Header.Set("X-Request-ID", requestID)

			// Extract provider from URL
			provider := extractProviderFromURL(r.URL.Path)
			if provider == "" {
				provider = "default"
			}

			// Capture request body
			var requestBody []byte
			if r.Body != nil {
				requestBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}

			// Create response writer wrapper
			rw := newResponseWriter(w)

			// Process request
			next.ServeHTTP(rw, r)

			// Create payment log
			paymentLog := opensearch.PaymentLog{
				Timestamp: rw.startTime,
				Provider:  provider,
				Method:    r.Method,
				Endpoint:  r.URL.Path,
				RequestID: requestID,
				UserAgent: r.UserAgent(),
				ClientIP:  getClientIP(r),
				Request: opensearch.RequestLog{
					Headers: extractHeaders(r.Header),
					Body:    opensearch.SanitizeForLog(string(requestBody)),
					Params:  extractURLParams(r.URL.Query()),
				},
				Response: opensearch.ResponseLog{
					StatusCode:       rw.statusCode,
					Headers:          extractHeaders(rw.Header()),
					Body:             opensearch.SanitizeForLog(rw.body.String()),
					ProcessingTimeMs: time.Since(rw.startTime).Milliseconds(),
				},
			}

			// Extract payment information from request/response
			if paymentInfo := extractPaymentInfo(string(requestBody), rw.body.String()); paymentInfo != nil {
				paymentLog.PaymentInfo = *paymentInfo
			}

			// Extract error information if response indicates error
			if rw.statusCode >= 400 {
				if errorInfo := extractErrorInfo(rw.body.String()); errorInfo != nil {
					paymentLog.Error = *errorInfo
				}
			}

			// Log to OpenSearch asynchronously to avoid blocking the response
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := logger.LogPaymentRequest(ctx, paymentLog); err != nil {
					// Log error to standard logger, but don't fail the request
					// log.Printf("Failed to log payment request to OpenSearch: %v", err)
				}
			}()
		})
	}
}

// isPaymentEndpoint checks if the URL path is a payment-related endpoint
func isPaymentEndpoint(path string) bool {
	paymentPaths := []string{
		"/v1/payments",
		"/v1/callback",
		"/v1/webhooks",
	}

	for _, paymentPath := range paymentPaths {
		if strings.HasPrefix(path, paymentPath) {
			return true
		}
	}

	return false
}

// extractProviderFromURL extracts the provider name from the URL path
func extractProviderFromURL(path string) string {
	// URL patterns:
	// /v1/payments/{provider} -> extract provider
	// /v1/callback/{provider} -> extract provider
	// /v1/webhooks/{provider} -> extract provider

	segments := strings.Split(strings.Trim(path, "/"), "/")

	if len(segments) >= 3 {
		switch segments[1] {
		case "payments", "callback", "webhooks":
			if len(segments) > 2 {
				return segments[2]
			}
		}
	}

	return ""
}

// extractHeaders converts http.Header to map[string]string for logging
func extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)

	for key, values := range headers {
		if len(values) > 0 {
			// Skip sensitive headers
			if isSensitiveHeader(key) {
				result[key] = "***REDACTED***"
			} else {
				result[key] = values[0]
			}
		}
	}

	return result
}

// isSensitiveHeader checks if a header contains sensitive information
func isSensitiveHeader(header string) bool {
	sensitiveHeaders := []string{
		"authorization", "x-api-key", "x-secret-key", "cookie",
		"x-auth-token", "x-access-token", "x-refresh-token",
	}

	header = strings.ToLower(header)
	for _, sensitive := range sensitiveHeaders {
		if header == sensitive {
			return true
		}
	}

	return false
}

// extractURLParams converts url.Values to map[string]string
func extractURLParams(params url.Values) map[string]string {
	result := make(map[string]string)

	for key, values := range params {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

// extractPaymentInfo extracts payment information from request/response bodies
func extractPaymentInfo(requestBody, responseBody string) *opensearch.PaymentInfo {
	paymentInfo := &opensearch.PaymentInfo{}

	// Try to extract from request body first
	if requestBody != "" {
		var requestData map[string]any
		if err := json.Unmarshal([]byte(requestBody), &requestData); err == nil {
			if amount, ok := requestData["amount"].(float64); ok {
				paymentInfo.Amount = amount
			}
			if currency, ok := requestData["currency"].(string); ok {
				paymentInfo.Currency = currency
			}
			if customer, ok := requestData["customer"].(map[string]any); ok {
				if email, ok := customer["email"].(string); ok {
					paymentInfo.CustomerEmail = email
				}
			}
			if use3d, ok := requestData["use3D"].(bool); ok {
				paymentInfo.Use3D = use3d
			}
		}
	}

	// Try to extract from response body
	if responseBody != "" {
		var responseData map[string]any
		if err := json.Unmarshal([]byte(responseBody), &responseData); err == nil {
			// Check for nested data structure
			if data, ok := responseData["data"].(map[string]any); ok {
				if paymentID, ok := data["paymentId"].(string); ok {
					paymentInfo.PaymentID = paymentID
				}
				if status, ok := data["status"].(string); ok {
					paymentInfo.Status = status
				}
			}
		}
	}

	// Return nil if no useful payment information was found
	if paymentInfo.PaymentID == "" && paymentInfo.Amount == 0 && paymentInfo.Currency == "" {
		return nil
	}

	return paymentInfo
}

// extractErrorInfo extracts error information from response body
func extractErrorInfo(responseBody string) *opensearch.ErrorInfo {
	if responseBody == "" {
		return nil
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(responseBody), &responseData); err != nil {
		return nil
	}

	errorInfo := &opensearch.ErrorInfo{}

	// Try different error formats
	if errorMsg, ok := responseData["error"].(string); ok {
		errorInfo.Message = errorMsg
	} else if errorMsg, ok := responseData["message"].(string); ok {
		errorInfo.Message = errorMsg
	}

	if errorCode, ok := responseData["errorCode"].(string); ok {
		errorInfo.Code = errorCode
	} else if code, ok := responseData["code"].(string); ok {
		errorInfo.Code = code
	}

	// Return nil if no error information found
	if errorInfo.Code == "" && errorInfo.Message == "" {
		return nil
	}

	return errorInfo
}

// LoggingStatsMiddleware creates middleware for collecting logging statistics
func LoggingStatsMiddleware(logger *opensearch.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a stats request
			if r.URL.Path == "/v1/stats" && r.Method == "GET" {
				handleStatsRequest(w, r, logger)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// handleStatsRequest handles requests for logging statistics
func handleStatsRequest(w http.ResponseWriter, r *http.Request, logger *opensearch.Logger) {
	provider := r.URL.Query().Get("provider")
	hoursStr := r.URL.Query().Get("hours")

	hours := 24 // Default to 24 hours
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	if provider == "" {
		http.Error(w, "provider parameter is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	stats, err := logger.GetProviderStats(ctx, provider, hours)
	if err != nil {
		http.Error(w, "Failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
		return
	}
}

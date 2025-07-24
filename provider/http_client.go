package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClientConfig represents configuration for HTTP client
type HTTPClientConfig struct {
	BaseURL            string
	Timeout            time.Duration
	InsecureSkipVerify bool
	DefaultHeaders     map[string]string
}

// HTTPRequest represents a standardized HTTP request
type HTTPRequest struct {
	Method      string
	Endpoint    string
	Headers     map[string]string
	Body        any
	FormData    map[string]string
	QueryParams map[string]string
}

// HTTPResponse represents a standardized HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	RawBody    string
}

// ProviderHTTPClient provides standardized HTTP operations for payment providers
type ProviderHTTPClient struct {
	config *HTTPClientConfig
	client *http.Client
}

// NewProviderHTTPClient creates a new provider HTTP client
func NewProviderHTTPClient(config *HTTPClientConfig) *ProviderHTTPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &ProviderHTTPClient{
		config: config,
		client: client,
	}
}

// SendJSON sends a JSON request and returns the response
func (c *ProviderHTTPClient) SendJSON(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
	fullURL := c.buildURL(req.Endpoint, req.QueryParams)
	var debugBody any
	if req.Body != nil {
		debugBody = req.Body
	} else if req.FormData != nil {
		debugBody = req.FormData
	}
	jsonBody, _ := json.Marshal(debugBody)
	fmt.Printf("[DEBUG] SendJSON URL: %s\n", fullURL)
	fmt.Printf("[DEBUG] SendJSON Body: %s\n", string(jsonBody))
	return c.sendRequest(ctx, req, "application/json")
}

// SendForm sends a form-encoded request and returns the response
func (c *ProviderHTTPClient) SendForm(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
	return c.sendRequest(ctx, req, "application/x-www-form-urlencoded")
}

// SendRaw sends a raw request and returns the response
func (c *ProviderHTTPClient) SendRaw(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
	return c.sendRequest(ctx, req, "")
}

// sendRequest is the internal method that handles all HTTP requests
func (c *ProviderHTTPClient) sendRequest(ctx context.Context, req *HTTPRequest, contentType string) (*HTTPResponse, error) {
	// Build full URL
	fullURL := c.buildURL(req.Endpoint, req.QueryParams)
	// Prepare request body
	var body io.Reader
	if contentType == "application/x-www-form-urlencoded" {
		if len(req.FormData) > 0 {
			formData := url.Values{}
			for key, value := range req.FormData {
				formData.Set(key, value)
			}
			body = strings.NewReader(formData.Encode())
		} else if req.Body != nil {
			// fallback: Body'den form-data üret
			if formMap, ok := req.Body.(map[string]string); ok {
				formData := url.Values{}
				for key, value := range formMap {
					formData.Set(key, value)
				}
				body = strings.NewReader(formData.Encode())
			} else {
				// Body başka bir tipteyse, string veya []byte olarak kullan
				if rawBody, ok := req.Body.(string); ok {
					body = strings.NewReader(rawBody)
				} else if rawBody, ok := req.Body.([]byte); ok {
					body = bytes.NewBuffer(rawBody)
				}
			}
		}
	} else if contentType == "application/json" && req.Body != nil {
		jsonData, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	} else if req.Body != nil {
		if rawBody, ok := req.Body.(string); ok {
			body = strings.NewReader(rawBody)
		} else if rawBody, ok := req.Body.([]byte); ok {
			body = bytes.NewBuffer(rawBody)
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	for key, value := range c.config.DefaultHeaders {
		httpReq.Header.Set(key, value)
	}

	// Set request-specific headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set content type if specified
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	// Send request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Create standardized response
	response := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		RawBody:    string(respBody),
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return response, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return response, nil
}

func joinURL(base, endpoint string) string {
	if strings.HasSuffix(base, "/") && strings.HasPrefix(endpoint, "/") {
		return base + endpoint[1:]
	}
	if !strings.HasSuffix(base, "/") && !strings.HasPrefix(endpoint, "/") {
		return base + "/" + endpoint
	}
	return base + endpoint
}

// buildURL constructs the full URL with query parameters
func (c *ProviderHTTPClient) buildURL(endpoint string, queryParams map[string]string) string {
	if strings.HasPrefix(endpoint, "http") {
		// Absolute URL
		u, err := url.Parse(endpoint)
		if err != nil {
			return endpoint
		}

		if len(queryParams) > 0 {
			q := u.Query()
			for key, value := range queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()
		}

		return u.String()
	}

	// Relative URL - prepend base URL
	fullURL := joinURL(c.config.BaseURL, endpoint)

	if len(queryParams) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return fullURL
		}

		q := u.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	return fullURL
}

// ParseJSONResponse parses the response body as JSON into the target interface
func (c *ProviderHTTPClient) ParseJSONResponse(response *HTTPResponse, target any) error {
	return json.Unmarshal(response.Body, target)
}

// CreateHTTPClientConfig creates a standard HTTP client configuration for providers
func CreateHTTPClientConfig(baseURL string, isProduction bool, timeout time.Duration) *HTTPClientConfig {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPClientConfig{
		BaseURL:            baseURL,
		Timeout:            timeout,
		InsecureSkipVerify: !isProduction, // Skip TLS verification in sandbox
		DefaultHeaders: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "GoPay/1.0",
		},
	}
}

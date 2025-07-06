package opensearch

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

// Client wraps the OpenSearch client
type Client struct {
	client *opensearch.Client
	config *config.AppConfig
	ctx    context.Context
}

// NewClient creates a new OpenSearch client
func NewClient(cfg *config.AppConfig) (*Client, error) {
	// OpenSearch client configuration
	opensearchConfig := opensearch.Config{
		Addresses: []string{cfg.OpenSearchURL},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For development/testing
			},
		},
		MaxRetries:    3,
		RetryOnStatus: []int{502, 503, 504, 429},
		RetryBackoff: func(i int) time.Duration {
			return time.Duration(i) * 100 * time.Millisecond
		},
	}

	// Add authentication if configured
	if cfg.OpenSearchUser != "" && cfg.OpenSearchPass != "" {
		opensearchConfig.Username = cfg.OpenSearchUser
		opensearchConfig.Password = cfg.OpenSearchPass
	}

	// Create OpenSearch client
	client, err := opensearch.NewClient(opensearchConfig)
	if err != nil {
		return nil, err
	}

	osClient := &Client{
		client: client,
		config: cfg,
		ctx:    context.Background(),
	}

	// Test connection and create default indices
	if err := osClient.setupIndices(); err != nil {
		log.Printf("Warning: Failed to setup OpenSearch indices: %v", err)
	}

	return osClient, nil
}

// GetClient returns the underlying OpenSearch client
func (c *Client) GetClient() *opensearch.Client {
	return c.client
}

// setupIndices creates the necessary indices if they don't exist
func (c *Client) setupIndices() error {
	indices := []string{
		"gopay-payment-logs",
		"gopay-system-logs",
		"gopay-analytics",
	}

	for _, indexName := range indices {
		if err := c.createIndexIfNotExists(indexName); err != nil {
			log.Printf("Warning: Failed to setup OpenSearch index %s: %v", indexName, err)
		}
	}

	return nil
}

// createIndexIfNotExists creates an index if it doesn't already exist
func (c *Client) createIndexIfNotExists(indexName string) error {
	// Check if index exists
	req := opensearchapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(c.ctx, c.client)
	if err != nil {
		log.Printf("Error checking index %s: %v", indexName, err)
		return err
	}
	defer res.Body.Close()

	// If index doesn't exist (404), create it
	if res.StatusCode == 404 {
		createReq := opensearchapi.IndicesCreateRequest{
			Index: indexName,
		}

		createRes, err := createReq.Do(c.ctx, c.client)
		if err != nil {
			log.Printf("Error creating index %s: %v", indexName, err)
			return err
		}
		defer createRes.Body.Close()

		log.Printf("Created OpenSearch index: %s", indexName)
	}

	return nil
}

// GetLogIndexName returns the index name for a tenant's provider logs
func (c *Client) GetLogIndexName(tenantID, provider string) string {
	if tenantID == "" {
		return "gopay-" + provider + "-logs"
	}
	return "gopay-" + tenantID + "-" + provider + "-logs"
}

// IsEnabled returns whether OpenSearch logging is enabled
func (c *Client) IsEnabled() bool {
	return c.config.EnableLogging
}

// Index indexes a document in OpenSearch
func (c *Client) Index(ctx context.Context, indexName string, body io.Reader) ([]byte, error) {
	req := opensearchapi.IndexRequest{
		Index: indexName,
		Body:  body,
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.IsError() {
		return nil, fmt.Errorf("index error: %s", string(responseBody))
	}

	return responseBody, nil
}

// Search performs a search query in OpenSearch
func (c *Client) Search(ctx context.Context, indexName string, body io.Reader) ([]byte, error) {
	req := opensearchapi.SearchRequest{
		Index: []string{indexName},
		Body:  body,
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", string(responseBody))
	}

	return responseBody, nil
}

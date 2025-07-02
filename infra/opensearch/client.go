package opensearch

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

// Client wraps the OpenSearch client
type Client struct {
	client *opensearch.Client
	config *config.AppConfig
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

// setupIndices creates the necessary indices for payment logging
func (c *Client) setupIndices() error {
	// List of payment providers to create indices for
	providers := []string{"iyzico", "ozanpay", "stripe", "paytr", "paycell", "papara", "nkolay", "shopier"}

	for _, provider := range providers {
		indexName := c.GetLogIndexName("", provider)

		// Check if index exists
		exists, err := c.indexExists(indexName)
		if err != nil {
			log.Printf("Error checking index %s: %v", indexName, err)
			continue
		}

		if !exists {
			if err := c.createLogIndex(indexName); err != nil {
				log.Printf("Error creating index %s: %v", indexName, err)
				continue
			}
			log.Printf("Created OpenSearch index: %s", indexName)
		}
	}

	return nil
}

// indexExists checks if an index exists
func (c *Client) indexExists(indexName string) (bool, error) {
	req := opensearchapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(nil, c.client)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// createLogIndex creates a new index for payment logs with proper mapping
func (c *Client) createLogIndex(indexName string) error {
	mapping := `{
		"mappings": {
			"properties": {
				"timestamp": {
					"type": "date",
					"format": "strict_date_optional_time||epoch_millis"
				},
				"tenant_id": {
					"type": "keyword"
				},
				"provider": {
					"type": "keyword"
				},
				"method": {
					"type": "keyword"
				},
				"endpoint": {
					"type": "keyword"
				},
				"request_id": {
					"type": "keyword"
				},
				"user_agent": {
					"type": "text"
				},
				"client_ip": {
					"type": "ip"
				},
				"request": {
					"type": "object",
					"properties": {
						"headers": {
							"type": "object"
						},
						"body": {
							"type": "text"
						},
						"params": {
							"type": "object"
						}
					}
				},
				"response": {
					"type": "object",
					"properties": {
						"status_code": {
							"type": "integer"
						},
						"headers": {
							"type": "object"
						},
						"body": {
							"type": "text"
						},
						"processing_time_ms": {
							"type": "integer"
						}
					}
				},
				"payment_info": {
					"type": "object",
					"properties": {
						"payment_id": {
							"type": "keyword"
						},
						"amount": {
							"type": "double"
						},
						"currency": {
							"type": "keyword"
						},
						"customer_email": {
							"type": "keyword"
						},
						"status": {
							"type": "keyword"
						},
						"use_3d": {
							"type": "boolean"
						}
					}
				},
				"error": {
					"type": "object",
					"properties": {
						"code": {
							"type": "keyword"
						},
						"message": {
							"type": "text"
						}
					}
				}
			}
		},
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 0,
			"index": {
				"lifecycle": {
					"name": "payment_logs_policy",
					"rollover_alias": "` + indexName + `"
				}
			}
		}
	}`

	req := opensearchapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(nil, c.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index creation error: %s", res.String())
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

package config

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStorage handles persistent storage of tenant configurations
type PostgresStorage struct {
	db          *sql.DB
	mu          sync.Mutex
	providerIDs map[string]int // Cache for provider name to ID mapping
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(dbURL string) (*PostgresStorage, error) {
	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &PostgresStorage{
		db:          db,
		providerIDs: make(map[string]int),
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Load provider IDs cache
	if err := storage.loadProviderIDs(); err != nil {
		log.Printf("Warning: failed to load provider IDs: %v", err)
	}

	log.Printf("PostgreSQL storage initialized")

	return storage, nil
}

// loadProviderIDs loads provider name to ID mapping into cache
func (s *PostgresStorage) loadProviderIDs() error {
	query := `SELECT id, name FROM providers`

	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query providers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return fmt.Errorf("failed to scan provider: %w", err)
		}
		s.providerIDs[name] = id
	}

	return rows.Err()
}

// getProviderID returns provider ID by name, creating if not exists
func (s *PostgresStorage) getProviderID(providerName string) (int, error) {
	// Check cache first
	if id, exists := s.providerIDs[providerName]; exists {
		return id, nil
	}

	// Query database
	var id int
	query := `SELECT id FROM providers WHERE name = $1`
	err := s.db.QueryRow(query, providerName).Scan(&id)

	if err == sql.ErrNoRows {
		// Create new provider
		insertQuery := `INSERT INTO providers (name) VALUES ($1) RETURNING id`
		err = s.db.QueryRow(insertQuery, providerName).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to create provider: %w", err)
		}
		log.Printf("Created new provider: %s with ID: %d", providerName, id)
	} else if err != nil {
		return 0, fmt.Errorf("failed to query provider: %w", err)
	}

	// Cache the result
	s.providerIDs[providerName] = id
	return id, nil
}

// SaveTenantConfig saves a tenant configuration to PostgreSQL using key-value structure
func (s *PostgresStorage) SaveTenantConfig(tenantID, providerName string, config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert tenantID to int
	tenantIDInt, err := strconv.Atoi(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Get provider ID
	providerID, err := s.getProviderID(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider ID: %w", err)
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing configs for this tenant-provider combination
	deleteQuery := `
		DELETE FROM tenant_configs 
		WHERE tenant_id = $1 AND provider_id = $2
	`
	_, err = tx.Exec(deleteQuery, tenantIDInt, providerID)
	if err != nil {
		return fmt.Errorf("failed to delete existing configs: %w", err)
	}

	// Insert new configs
	insertQuery := `
		INSERT INTO tenant_configs (tenant_id, provider_id, environment, key, value)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Determine environment from config or default to 'test'
	environment := "test"
	if env, exists := config["environment"]; exists {
		environment = env
	}

	// Insert each key-value pair
	for key, value := range config {
		if key == "environment" {
			continue // Skip environment key as it's handled separately
		}

		_, err = tx.Exec(insertQuery, tenantIDInt, providerID, environment, key, value)
		if err != nil {
			return fmt.Errorf("failed to insert config %s: %w", key, err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Saved %d config keys for tenant %s, provider %s", len(config), tenantID, providerName)
	return nil
}

// LoadTenantConfig loads a tenant configuration from PostgreSQL using key-value structure
func (s *PostgresStorage) LoadTenantConfig(tenantID, providerName string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert tenantID to int
	tenantIDInt, err := strconv.Atoi(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Get provider ID
	providerID, err := s.getProviderID(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider ID: %w", err)
	}

	query := `
		SELECT environment, key, value 
		FROM tenant_configs 
		WHERE tenant_id = $1 AND provider_id = $2
	`

	rows, err := s.db.Query(query, tenantIDInt, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant config: %w", err)
	}
	defer rows.Close()

	config := make(map[string]string)
	var environment string

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&environment, &key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}
		config[key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating config rows: %w", err)
	}

	// Add environment to config if we have any configs
	if len(config) > 0 {
		config["environment"] = environment
	}

	return config, nil
}

// LoadAllTenantConfigs loads all tenant configurations from PostgreSQL
func (s *PostgresStorage) LoadAllTenantConfigs() (map[string]map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT tc.tenant_id, p.name as provider_name, tc.environment, tc.key, tc.value 
		FROM tenant_configs tc
		JOIN providers p ON tc.provider_id = p.id
		ORDER BY tc.tenant_id, p.name, tc.key
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant configs: %w", err)
	}
	defer rows.Close()

	configs := make(map[string]map[string]string)

	for rows.Next() {
		var tenantID int
		var providerName, environment, key, value string
		if err := rows.Scan(&tenantID, &providerName, &environment, &key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create tenant-specific provider key (consistent with provider_config.go format)
		tenantProviderKey := fmt.Sprintf("%s_%s", strings.ToUpper(strconv.Itoa(tenantID)), strings.ToLower(providerName))

		// Initialize config map if not exists
		if configs[tenantProviderKey] == nil {
			configs[tenantProviderKey] = make(map[string]string)
			configs[tenantProviderKey]["environment"] = environment
		}

		configs[tenantProviderKey][key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Loaded configurations for %d tenant-provider combinations from PostgreSQL", len(configs))
	return configs, nil
}

// DeleteTenantConfig deletes a tenant configuration from PostgreSQL
func (s *PostgresStorage) DeleteTenantConfig(tenantID, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert tenantID to int
	tenantIDInt, err := strconv.Atoi(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Get provider ID
	providerID, err := s.getProviderID(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider ID: %w", err)
	}

	query := `
		DELETE FROM tenant_configs 
		WHERE tenant_id = $1 AND provider_id = $2
	`

	result, err := s.db.Exec(query, tenantIDInt, providerID)
	if err != nil {
		return fmt.Errorf("failed to delete tenant config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no configuration found for tenant: %s, provider: %s", tenantID, providerName)
	}

	log.Printf("Deleted %d config entries for tenant %s, provider %s", rowsAffected, tenantID, providerName)
	return nil
}

// GetTenantsByProvider returns all tenant IDs that have configuration for a specific provider
func (s *PostgresStorage) GetTenantsByProvider(providerName string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get provider ID
	providerID, err := s.getProviderID(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider ID: %w", err)
	}

	query := `
		SELECT DISTINCT tenant_id 
		FROM tenant_configs 
		WHERE provider_id = $1
		ORDER BY tenant_id
	`

	rows, err := s.db.Query(query, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenants by provider: %w", err)
	}
	defer rows.Close()

	var tenants []string
	for rows.Next() {
		var tenantID int
		if err := rows.Scan(&tenantID); err != nil {
			return nil, fmt.Errorf("failed to scan tenant ID: %w", err)
		}
		tenants = append(tenants, strconv.Itoa(tenantID))
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tenant rows: %w", err)
	}

	return tenants, nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStats returns database statistics
func (s *PostgresStorage) GetStats() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := make(map[string]any)

	// Count total configurations
	var totalConfigs int
	err := s.db.QueryRow("SELECT COUNT(*) FROM tenant_configs").Scan(&totalConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to count total configs: %w", err)
	}
	stats["total_configs"] = totalConfigs

	// Count unique tenants
	var uniqueTenants int
	err = s.db.QueryRow("SELECT COUNT(DISTINCT tenant_id) FROM tenant_configs").Scan(&uniqueTenants)
	if err != nil {
		return nil, fmt.Errorf("failed to count unique tenants: %w", err)
	}
	stats["unique_tenants"] = uniqueTenants

	// Count unique providers
	var uniqueProviders int
	err = s.db.QueryRow("SELECT COUNT(DISTINCT provider_id) FROM tenant_configs").Scan(&uniqueProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to count unique providers: %w", err)
	}
	stats["unique_providers"] = uniqueProviders

	stats["db_type"] = "postgresql"

	return stats, nil
}

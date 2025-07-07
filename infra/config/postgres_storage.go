package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStorage handles persistent storage of tenant configurations
type PostgresStorage struct {
	db *sql.DB
	mu sync.Mutex
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
		db: db,
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("PostgreSQL storage initialized")

	return storage, nil
}

// SaveTenantConfig saves a tenant configuration to PostgreSQL
func (s *PostgresStorage) SaveTenantConfig(tenantID, providerName string, config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert or update configuration
	query := `
		INSERT INTO tenant_configs (tenant_id, provider_name, config_data, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (tenant_id, provider_name) 
		DO UPDATE SET 
			config_data = EXCLUDED.config_data,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.Exec(query, tenantID, providerName, string(configJSON))
	if err != nil {
		return fmt.Errorf("failed to save tenant config: %w", err)
	}

	log.Printf("Saved config for tenant %s, provider %s", tenantID, providerName)
	return nil
}

// LoadTenantConfig loads a tenant configuration from PostgreSQL
func (s *PostgresStorage) LoadTenantConfig(tenantID, providerName string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT config_data 
		FROM tenant_configs 
		WHERE tenant_id = $1 AND provider_name = $2
	`

	var configJSON string
	err := s.db.QueryRow(query, tenantID, providerName).Scan(&configJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no configuration found for tenant: %s, provider: %s", tenantID, providerName)
		}
		return nil, fmt.Errorf("failed to load tenant config: %w", err)
	}

	// Deserialize JSON to config map
	var config map[string]string
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// LoadAllTenantConfigs loads all tenant configurations from PostgreSQL
func (s *PostgresStorage) LoadAllTenantConfigs() (map[string]map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT tenant_id, provider_name, config_data 
		FROM tenant_configs 
		ORDER BY tenant_id, provider_name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant configs: %w", err)
	}
	defer rows.Close()

	configs := make(map[string]map[string]string)

	for rows.Next() {
		var tenantID, providerName, configJSON string
		if err := rows.Scan(&tenantID, &providerName, &configJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Deserialize JSON to config map
		var config map[string]string
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			log.Printf("Warning: failed to unmarshal config for tenant %s, provider %s: %v", tenantID, providerName, err)
			continue
		}

		// Create tenant-specific provider key
		tenantProviderKey := fmt.Sprintf("%s_%s", tenantID, providerName)
		configs[tenantProviderKey] = config
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Loaded %d tenant configurations from PostgreSQL", len(configs))
	return configs, nil
}

// DeleteTenantConfig deletes a tenant configuration from PostgreSQL
func (s *PostgresStorage) DeleteTenantConfig(tenantID, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		DELETE FROM tenant_configs 
		WHERE tenant_id = $1 AND provider_name = $2
	`

	result, err := s.db.Exec(query, tenantID, providerName)
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

	log.Printf("Deleted config for tenant %s, provider %s", tenantID, providerName)
	return nil
}

// GetTenantsByProvider returns all tenant IDs that have configuration for a specific provider
func (s *PostgresStorage) GetTenantsByProvider(providerName string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT DISTINCT tenant_id 
		FROM tenant_configs 
		WHERE provider_name = $1
		ORDER BY tenant_id
	`

	rows, err := s.db.Query(query, providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenants by provider: %w", err)
	}
	defer rows.Close()

	var tenants []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, fmt.Errorf("failed to scan tenant ID: %w", err)
		}
		tenants = append(tenants, tenantID)
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
	err = s.db.QueryRow("SELECT COUNT(DISTINCT provider_name) FROM tenant_configs").Scan(&uniqueProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to count unique providers: %w", err)
	}
	stats["unique_providers"] = uniqueProviders

	stats["db_type"] = "postgresql"

	return stats, nil
}

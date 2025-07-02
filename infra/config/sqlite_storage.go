package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage handles persistent storage of tenant configurations
type SQLiteStorage struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// retryOperation executes a database operation with retry logic for SQLITE_BUSY errors
func (s *SQLiteStorage) retryOperation(operation func() error, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if it's a busy error
		if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
			lastErr = err
			if attempt < maxRetries {
				// Exponential backoff: 10ms, 20ms, 40ms, 80ms
				backoff := time.Duration(10*(1<<attempt)) * time.Millisecond
				log.Printf("SQLite busy, retrying in %v (attempt %d/%d)", backoff, attempt+1, maxRetries+1)
				time.Sleep(backoff)
				continue
			}
		} else {
			// Not a retry-able error
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries, last error: %w", maxRetries+1, lastErr)
}

// NewSQLiteStorage creates a new SQLite storage instance optimized for multiple processes
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// SQLite connection string with multi-process optimizations
	connStr := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_timeout=20000&_txlock=immediate", dbPath)

	// Open database connection
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for multi-replica environment
	db.SetMaxOpenConns(10)   // Max 10 concurrent connections
	db.SetMaxIdleConns(5)    // Keep 5 idle connections
	db.SetConnMaxLifetime(0) // No connection lifetime limit

	storage := &SQLiteStorage{
		db:   db,
		path: dbPath,
	}

	// Initialize database schema and optimizations
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Apply additional performance optimizations
	if err := storage.optimizeForMultiProcess(); err != nil {
		log.Printf("Warning: Failed to apply optimizations: %v", err)
	}

	log.Printf("SQLite storage initialized at: %s (multi-process optimized)", dbPath)
	return storage, nil
}

// initSchema creates the necessary tables
func (s *SQLiteStorage) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS tenant_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id TEXT NOT NULL,
		provider_name TEXT NOT NULL,
		config_data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, provider_name)
	);

	CREATE INDEX IF NOT EXISTS idx_tenant_provider ON tenant_configs(tenant_id, provider_name);
	
	-- Trigger to update updated_at column
	CREATE TRIGGER IF NOT EXISTS update_tenant_configs_updated_at 
		AFTER UPDATE ON tenant_configs
	BEGIN
		UPDATE tenant_configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
	END;
	`

	_, err := s.db.Exec(query)
	return err
}

// optimizeForMultiProcess applies SQLite optimizations for multi-process access
func (s *SQLiteStorage) optimizeForMultiProcess() error {
	optimizations := []string{
		"PRAGMA journal_mode = WAL;",    // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous = NORMAL;",  // Balance between safety and speed
		"PRAGMA cache_size = 1000;",     // Increase cache size
		"PRAGMA busy_timeout = 30000;",  // 30 second timeout for lock waits
		"PRAGMA temp_store = memory;",   // Store temp tables in memory
		"PRAGMA mmap_size = 268435456;", // 256MB memory mapping
		"PRAGMA optimize;",              // Optimize database
	}

	for _, pragma := range optimizations {
		if _, err := s.db.Exec(pragma); err != nil {
			log.Printf("Warning: Failed to execute %s: %v", pragma, err)
		}
	}

	// Test WAL mode is actually enabled
	var journalMode string
	err := s.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		return fmt.Errorf("failed to check journal mode: %w", err)
	}

	log.Printf("SQLite journal mode: %s", journalMode)
	return nil
}

// SaveTenantConfig saves a tenant configuration to SQLite
func (s *SQLiteStorage) SaveTenantConfig(tenantID, providerName string, config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert or update configuration with retry logic
	return s.retryOperation(func() error {
		query := `
		INSERT INTO tenant_configs (tenant_id, provider_name, config_data, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, provider_name) 
		DO UPDATE SET 
			config_data = excluded.config_data,
			updated_at = CURRENT_TIMESTAMP
		`

		_, err := s.db.Exec(query, tenantID, providerName, string(configJSON))
		if err != nil {
			return fmt.Errorf("failed to save tenant config: %w", err)
		}

		log.Printf("Saved config for tenant %s, provider %s", tenantID, providerName)
		return nil
	}, 3) // Max 3 retries
}

// LoadTenantConfig loads a tenant configuration from SQLite
func (s *SQLiteStorage) LoadTenantConfig(tenantID, providerName string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var config map[string]string
	err := s.retryOperation(func() error {
		query := `
		SELECT config_data 
		FROM tenant_configs 
		WHERE tenant_id = ? AND provider_name = ?
		`

		var configJSON string
		err := s.db.QueryRow(query, tenantID, providerName).Scan(&configJSON)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("no configuration found for tenant: %s, provider: %s", tenantID, providerName)
			}
			return fmt.Errorf("failed to load tenant config: %w", err)
		}

		// Deserialize JSON to config map
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w", err)
		}

		return nil
	}, 3) // Max 3 retries

	return config, err
}

// LoadAllTenantConfigs loads all tenant configurations from SQLite
func (s *SQLiteStorage) LoadAllTenantConfigs() (map[string]map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var configs map[string]map[string]string
	err := s.retryOperation(func() error {
		query := `
		SELECT tenant_id, provider_name, config_data 
		FROM tenant_configs 
		ORDER BY tenant_id, provider_name
		`

		rows, err := s.db.Query(query)
		if err != nil {
			return fmt.Errorf("failed to query tenant configs: %w", err)
		}
		defer rows.Close()

		configs = make(map[string]map[string]string)

		for rows.Next() {
			var tenantID, providerName, configJSON string
			if err := rows.Scan(&tenantID, &providerName, &configJSON); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
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
			return fmt.Errorf("error iterating rows: %w", err)
		}

		return nil
	}, 3) // Max 3 retries

	if err != nil {
		return nil, err
	}

	log.Printf("Loaded %d tenant configurations from SQLite", len(configs))
	return configs, nil
}

// DeleteTenantConfig deletes a tenant configuration from SQLite
func (s *SQLiteStorage) DeleteTenantConfig(tenantID, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.retryOperation(func() error {
		query := `
		DELETE FROM tenant_configs 
		WHERE tenant_id = ? AND provider_name = ?
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
	}, 3) // Max 3 retries
}

// GetTenantsByProvider returns all tenant IDs that have configuration for a specific provider
func (s *SQLiteStorage) GetTenantsByProvider(providerName string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	SELECT DISTINCT tenant_id 
	FROM tenant_configs 
	WHERE provider_name = ?
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
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStats returns database statistics
func (s *SQLiteStorage) GetStats() (map[string]any, error) {
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

	// Database file size
	if fileInfo, err := os.Stat(s.path); err == nil {
		stats["db_size_bytes"] = fileInfo.Size()
	}

	stats["db_path"] = s.path

	return stats, nil
}

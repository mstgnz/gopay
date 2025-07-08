package provider

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// ProviderCacheEntry represents a cached provider instance
type ProviderCacheEntry struct {
	Provider     PaymentProvider
	Key          string
	TenantID     int
	ProviderName string
	Environment  string
	CreatedAt    time.Time
	LastAccessed time.Time
	listElement  *list.Element // For LRU tracking
}

// ProviderCache interface defines cache operations
type ProviderCache interface {
	// Get retrieves a provider from cache, returns nil if not found
	Get(tenantID int, providerName, environment string) PaymentProvider

	// Set stores a provider in cache
	Set(tenantID int, providerName, environment string, provider PaymentProvider)

	// Delete removes a provider from cache
	Delete(tenantID int, providerName, environment string)

	// DeleteByTenantAndProvider removes all entries for a tenant-provider combination
	DeleteByTenantAndProvider(tenantID int, providerName string)

	// Clear removes all entries from cache
	Clear()

	// Size returns the current number of cached entries
	Size() int

	// Stats returns cache statistics
	Stats() CacheStats

	// Cleanup removes expired entries
	Cleanup()
}

// CacheStats represents cache performance metrics
type CacheStats struct {
	Size        int           `json:"size"`
	MaxSize     int           `json:"max_size"`
	Hits        int64         `json:"hits"`
	Misses      int64         `json:"misses"`
	Evictions   int64         `json:"evictions"`
	TTLExpiries int64         `json:"ttl_expiries"`
	HitRatio    float64       `json:"hit_ratio"`
	TTL         time.Duration `json:"ttl"`
}

// InMemoryProviderCache implements ProviderCache interface
type InMemoryProviderCache struct {
	entries     map[string]*ProviderCacheEntry
	accessOrder *list.List // For LRU tracking, most recent at front
	maxSize     int
	ttl         time.Duration
	mu          sync.RWMutex

	// Stats tracking
	hits        int64
	misses      int64
	evictions   int64
	ttlExpiries int64
}

// NewProviderCache creates a new in-memory provider cache
func NewProviderCache(maxSize int, ttl time.Duration) ProviderCache {
	return &InMemoryProviderCache{
		entries:     make(map[string]*ProviderCacheEntry),
		accessOrder: list.New(),
		maxSize:     maxSize,
		ttl:         ttl,
	}
}

// generateCacheKey creates a unique cache key for tenant-provider-environment combination
func generateCacheKey(tenantID int, providerName, environment string) string {
	return fmt.Sprintf("%d-%s-%s", tenantID, providerName, environment)
}

// Get retrieves a provider from cache
func (c *InMemoryProviderCache) Get(tenantID int, providerName, environment string) PaymentProvider {
	key := generateCacheKey(tenantID, providerName, environment)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.misses++
		return nil
	}

	// Check TTL expiry
	if c.ttl > 0 && time.Since(entry.CreatedAt) > c.ttl {
		c.deleteEntryUnsafe(key, entry)
		c.ttlExpiries++
		c.misses++
		return nil
	}

	// Update access time and move to front (most recently used)
	entry.LastAccessed = time.Now()
	c.accessOrder.MoveToFront(entry.listElement)

	c.hits++
	return entry.Provider
}

// Set stores a provider in cache
func (c *InMemoryProviderCache) Set(tenantID int, providerName, environment string, provider PaymentProvider) {
	key := generateCacheKey(tenantID, providerName, environment)
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// If entry already exists, update it
	if existingEntry, exists := c.entries[key]; exists {
		existingEntry.Provider = provider
		existingEntry.CreatedAt = now
		existingEntry.LastAccessed = now
		c.accessOrder.MoveToFront(existingEntry.listElement)
		return
	}

	// Check if we need to evict entries due to size limit
	if len(c.entries) >= c.maxSize {
		c.evictLRUUnsafe()
	}

	// Create new entry
	entry := &ProviderCacheEntry{
		Provider:     provider,
		Key:          key,
		TenantID:     tenantID,
		ProviderName: providerName,
		Environment:  environment,
		CreatedAt:    now,
		LastAccessed: now,
	}

	// Add to front of access order list (most recently used)
	entry.listElement = c.accessOrder.PushFront(entry)

	// Store in map
	c.entries[key] = entry
}

// Delete removes a provider from cache
func (c *InMemoryProviderCache) Delete(tenantID int, providerName, environment string) {
	key := generateCacheKey(tenantID, providerName, environment)

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.deleteEntryUnsafe(key, entry)
	}
}

// DeleteByTenantAndProvider removes all entries for a tenant-provider combination
func (c *InMemoryProviderCache) DeleteByTenantAndProvider(tenantID int, providerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect keys to delete (to avoid modifying map while iterating)
	var keysToDelete []string
	for key, entry := range c.entries {
		if entry.TenantID == tenantID && entry.ProviderName == providerName {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete collected entries
	for _, key := range keysToDelete {
		if entry, exists := c.entries[key]; exists {
			c.deleteEntryUnsafe(key, entry)
		}
	}
}

// Clear removes all entries from cache
func (c *InMemoryProviderCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*ProviderCacheEntry)
	c.accessOrder = list.New()
}

// Size returns the current number of cached entries
func (c *InMemoryProviderCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// Stats returns cache statistics
func (c *InMemoryProviderCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalRequests := c.hits + c.misses
	hitRatio := 0.0
	if totalRequests > 0 {
		hitRatio = float64(c.hits) / float64(totalRequests)
	}

	return CacheStats{
		Size:        len(c.entries),
		MaxSize:     c.maxSize,
		Hits:        c.hits,
		Misses:      c.misses,
		Evictions:   c.evictions,
		TTLExpiries: c.ttlExpiries,
		HitRatio:    hitRatio,
		TTL:         c.ttl,
	}
}

// Cleanup removes expired entries
func (c *InMemoryProviderCache) Cleanup() {
	if c.ttl <= 0 {
		return // No TTL configured
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var expiredKeys []string

	// Find expired entries
	for key, entry := range c.entries {
		if now.Sub(entry.CreatedAt) > c.ttl {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		if entry, exists := c.entries[key]; exists {
			c.deleteEntryUnsafe(key, entry)
			c.ttlExpiries++
		}
	}
}

// evictLRUUnsafe removes the least recently used entry (must be called with lock held)
func (c *InMemoryProviderCache) evictLRUUnsafe() {
	if c.accessOrder.Len() == 0 {
		return
	}

	// Get least recently used entry (back of the list)
	lruElement := c.accessOrder.Back()
	if lruElement == nil {
		return
	}

	lruEntry := lruElement.Value.(*ProviderCacheEntry)
	c.deleteEntryUnsafe(lruEntry.Key, lruEntry)
	c.evictions++
}

// deleteEntryUnsafe removes an entry from both map and list (must be called with lock held)
func (c *InMemoryProviderCache) deleteEntryUnsafe(key string, entry *ProviderCacheEntry) {
	delete(c.entries, key)
	if entry.listElement != nil {
		c.accessOrder.Remove(entry.listElement)
	}
}

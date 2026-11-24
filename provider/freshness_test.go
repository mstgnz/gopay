package provider

import (
	"net/http"
	"testing"
	"time"
)

// A config write invalidates the cache of one replica only, and an edit in the database none, so
// the TTL is how long the other replicas keep sending old credentials to the bank.
func TestProviderCacheServesAConfigForAtMostAMinute(t *testing.T) {
	if ttl := GetProviderCache().Stats().TTL; ttl <= 0 || ttl > time.Minute {
		t.Fatalf("provider cache TTL = %v, want (0, 1m]", ttl)
	}
}

// Every cache refresh builds new provider HTTP clients; their idle connections must close on their
// own, or each refresh leaves open sockets behind.
func TestProviderHTTPClientClosesIdleConnections(t *testing.T) {
	c := NewProviderHTTPClient(CreateHTTPClientConfig("https://bank.example", false))
	transport, ok := c.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", c.client.Transport)
	}
	if transport.IdleConnTimeout <= 0 {
		t.Fatalf("IdleConnTimeout = %v, idle connections would never close", transport.IdleConnTimeout)
	}
}

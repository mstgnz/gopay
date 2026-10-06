package provider

import (
	"context"
	"errors"
	"testing"
)

// The provider name in /v1/logs/{provider} is interpolated into SQL as a table name. Anything
// that is not a registered provider must be refused before a query is built; a nil db proves no
// query was attempted, because touching it would panic.
func TestProviderSpecificLogger_RejectsUnregisteredProvider(t *testing.T) {
	Register("logtabletest", func() PaymentProvider { return nil })
	t.Cleanup(func() {
		DefaultRegistry.mu.Lock()
		delete(DefaultRegistry.providers, "logtabletest")
		DefaultRegistry.mu.Unlock()
	})
	l := NewProviderSpecificLogger(nil)
	ctx := context.Background()

	for _, name := range []string{
		"paycell WHERE 1=1 --",
		"paycell;DROP TABLE tenants",
		"tenants",
		"tenant_configs",
		"",
	} {
		if _, err := l.SearchLogs(ctx, "2", name, map[string]any{}); !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("SearchLogs(%q): err = %v, want ErrUnknownProvider", name, err)
		}
		if _, err := l.GetPaymentLogs(ctx, "2", name, "p"); !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("GetPaymentLogs(%q): err = %v, want ErrUnknownProvider", name, err)
		}
		if _, err := l.GetRecentErrorLogs(ctx, "2", name, 24); !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("GetRecentErrorLogs(%q): err = %v, want ErrUnknownProvider", name, err)
		}
		if _, err := l.GetProviderStats(ctx, "2", name, 24); !errors.Is(err, ErrUnknownProvider) {
			t.Errorf("GetProviderStats(%q): err = %v, want ErrUnknownProvider", name, err)
		}
	}

	if got, err := logTable("logtabletest"); err != nil || got != "logtabletest" {
		t.Errorf("registered provider refused: %q, %v", got, err)
	}
	// Postgres folded unquoted table names, so mixed case worked before the allowlist.
	if got, err := logTable("LogTableTest"); err != nil || got != "logtabletest" {
		t.Errorf("mixed-case provider: got %q, %v; want the lowercase table", got, err)
	}
}

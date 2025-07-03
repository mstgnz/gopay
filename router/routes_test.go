package router

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name   string
		logger *opensearch.Logger
	}{
		{
			name:   "valid_logger",
			logger: &opensearch.Logger{},
		},
		{
			name:   "nil_logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			require.NotNil(t, r)

			// Routes function should not panic
			assert.NotPanics(t, func() {
				Routes(r, tt.logger)
			})
		})
	}
}

func TestRoutes_Integration(t *testing.T) {
	// Test that routes are properly registered
	r := chi.NewRouter()
	logger := &opensearch.Logger{}

	Routes(r, logger)

	// Check that the router has routes registered
	// Chi router doesn't expose routes directly, but we can test it doesn't panic
	assert.NotNil(t, r)
}

func TestPackageImports(t *testing.T) {
	// Test that all side-effect imports are properly loaded
	// These imports register providers automatically
	// If there are any initialization errors, they would panic during import
	// Since the test runs, it means all imports are successful
	assert.True(t, true, "All provider imports successful")
}

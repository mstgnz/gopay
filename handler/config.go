package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// ConfigHandler handles configuration related HTTP requests
type ConfigHandler struct {
	providerConfig *config.ProviderConfig
	paymentService *provider.PaymentService
	validate       *validator.Validate
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(providerConfig *config.ProviderConfig, paymentService *provider.PaymentService, validate *validator.Validate) *ConfigHandler {
	return &ConfigHandler{
		providerConfig: providerConfig,
		paymentService: paymentService,
		validate:       validate,
	}
}

// SetEnvRequest represents the request structure for setting environment variables
type SetEnvRequest struct {
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	Configs     []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"configs"`
}

// SetEnv handles setting environment variables for a tenant
func (h *ConfigHandler) PostTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Parse the request
	var req SetEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Environment = strings.ToLower(strings.TrimSpace(req.Environment))
	if req.Provider == "" || req.Environment == "" || len(req.Configs) == 0 {
		response.Error(w, http.StatusBadRequest, "provider, environment and configs are required", nil)
		return
	}
	if req.Environment != "sandbox" && req.Environment != "production" {
		response.Error(w, http.StatusBadRequest, "environment must be 'sandbox' or 'production'", nil)
		return
	}

	// Validate provider existence using DB (providers table)
	providerID, err := h.providerConfig.GetProviderIDByName(req.Provider)
	if err != nil || providerID <= 0 {
		response.Error(w, http.StatusBadRequest, "Provider not found", nil)
		return
	}

	// Prepare config map for DB
	configMap := make(map[string]string)
	configMap["environment"] = req.Environment
	for _, kv := range req.Configs {
		if kv.Key == "" {
			continue
		}
		configMap[kv.Key] = kv.Value
	}
	if len(configMap) <= 1 { // only environment
		response.Error(w, http.StatusBadRequest, "At least one config key/value required", nil)
		return
	}

	// Dynamic provider validation using provider's own validation method
	if err := h.validateConfigWithProvider(req.Provider, configMap); err != nil {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("Invalid configuration: %v", err), err)
		return
	}

	// Convert tenantID to int for cache operations
	tenantIDInt, err := strconv.Atoi(tenantID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	// Save to DB (tenant_configs)
	if err := h.providerConfig.SetTenantConfig(tenantID, req.Provider, configMap); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}

	// Invalidate provider cache for this tenant-provider-environment combination
	cache := provider.GetProviderCache()
	cache.Delete(tenantIDInt, req.Provider, req.Environment)

	responseData := map[string]any{
		"tenantId": tenantID,
		"message":  "Provider configuration set successfully",
	}
	response.Success(w, http.StatusOK, "Configuration created", responseData)
}

// GetTenantConfig returns the configuration for a specific tenant and provider
func (h *ConfigHandler) GetTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get provider from query parameter
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		response.Error(w, http.StatusBadRequest, "provider query parameter is required", nil)
		return
	}

	// Get configuration
	config, err := h.providerConfig.GetTenantConfig(tenantID, providerName)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Configuration not found", err)
		return
	}

	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": providerName,
		"config":   config,
	}

	response.Success(w, http.StatusOK, "Configuration retrieved", responseData)
}

// DeleteTenantConfig deletes a tenant configuration
func (h *ConfigHandler) DeleteTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get provider from query parameter
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		response.Error(w, http.StatusBadRequest, "provider query parameter is required", nil)
		return
	}

	// Convert tenantID to int for cache operations
	tenantIDInt, err := strconv.Atoi(tenantID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	// Delete configuration
	err = h.providerConfig.DeleteTenantConfig(tenantID, providerName)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Failed to delete configuration", err)
		return
	}

	// Invalidate provider cache for this tenant-provider combination (all environments)
	cache := provider.GetProviderCache()
	cache.DeleteByTenantAndProvider(tenantIDInt, providerName)

	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": providerName,
		"message":  "Configuration deleted successfully",
	}

	response.Success(w, http.StatusOK, "Configuration deleted", responseData)
}

// GetStats returns system statistics and configuration information
func (h *ConfigHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Get statistics from provider config
	stats, err := h.providerConfig.GetStats()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get statistics", err)
		return
	}

	response.Success(w, http.StatusOK, "Statistics retrieved", stats)
}

// validateConfigWithProvider validates configuration using provider's own validation method
func (h *ConfigHandler) validateConfigWithProvider(providerName string, config map[string]string) error {
	// Get provider factory from registry
	providerFactory, err := provider.Get(providerName)
	if err != nil {
		// If provider is not registered, use basic validation
		logger.Warn("Provider not found in registry, using basic validation", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"error": err.Error(),
			},
		})
		return errors.New("provider not found in registry")
	}

	// Create provider instance
	providerInstance := providerFactory()

	// Use provider's own validation
	if err := providerInstance.ValidateConfig(config); err != nil {
		return err
	}

	return nil
}

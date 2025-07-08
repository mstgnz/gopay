package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
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
	if req.Environment != "test" && req.Environment != "prod" {
		response.Error(w, http.StatusBadRequest, "environment must be 'test' or 'prod'", nil)
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

	// Save to DB (tenant_configs)
	if err := h.providerConfig.SetTenantConfig(tenantID, req.Provider, configMap); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}

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

	// Remove sensitive information from response
	publicConfig := make(map[string]string)
	for key, value := range config {
		if strings.Contains(strings.ToLower(key), "key") ||
			strings.Contains(strings.ToLower(key), "password") ||
			strings.Contains(strings.ToLower(key), "secret") {
			// Mask sensitive values
			if len(value) > 8 {
				publicConfig[key] = value[:4] + "****" + value[len(value)-4:]
			} else {
				publicConfig[key] = "****"
			}
		} else {
			publicConfig[key] = value
		}
	}

	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": providerName,
		"config":   publicConfig,
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

	// Delete configuration
	err := h.providerConfig.DeleteTenantConfig(tenantID, providerName)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Failed to delete configuration", err)
		return
	}

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

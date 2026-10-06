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

// adminTenantID is the tenant that administers the gateway.
const adminTenantID = "1"

func isAdminTenant(tenantID string) bool {
	return tenantID == adminTenantID
}

// configMask replaces every stored config value on read. Provider credentials go out only to
// the provider; a stolen tenant login must not be able to export them.
const configMask = "********"

// nonSecretConfigKeys are returned as stored because they carry no credential.
var nonSecretConfigKeys = map[string]bool{
	"environment":         true,
	"eulaId":              true,
	"compensationEnabled": true,
}

func maskTenantConfig(configs map[string]map[string]string) map[string]map[string]string {
	masked := make(map[string]map[string]string, len(configs))
	for environment, values := range configs {
		out := make(map[string]string, len(values))
		for key, value := range values {
			if value == "" || nonSecretConfigKeys[key] {
				out[key] = value
				continue
			}
			out[key] = configMask
		}
		masked[environment] = out
	}
	return masked
}

// configTargetTenant resolves which tenant an admin config write applies to: the explicit
// target when given, the admin itself otherwise.
func configTargetTenant(callerTenantID string, requested *int) (string, error) {
	if requested == nil {
		return callerTenantID, nil
	}
	if *requested <= 0 {
		return "", errors.New("tenantId must be a positive integer")
	}
	return strconv.Itoa(*requested), nil
}

// SetEnvRequest represents the request structure for setting environment variables
type SetEnvRequest struct {
	// TenantID lets the admin configure another tenant; omitted means the admin's own tenant.
	TenantID    *int   `json:"tenantId,omitempty"`
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	Configs     []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"configs"`
}

// PostTenantConfig stores provider credentials. Admin only: a tenant able to rewrite its own
// merchant credentials could route its payments to another merchant account.
func (h *ConfigHandler) PostTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	callerTenantID := middle.GetTenantIDFromContext(r.Context())
	if callerTenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}
	if !isAdminTenant(callerTenantID) {
		response.Error(w, http.StatusForbidden, "Only administrators can change provider configuration", nil)
		return
	}

	// Parse the request
	var req SetEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	tenantID, err := configTargetTenant(callerTenantID, req.TenantID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), nil)
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
		"config":   maskTenantConfig(config),
	}

	response.Success(w, http.StatusOK, "Configuration retrieved", responseData)
}

// DeleteTenantConfig deletes a tenant configuration. Admin only, like PostTenantConfig: deleting
// a tenant's provider config stops its payments.
func (h *ConfigHandler) DeleteTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	callerTenantID := middle.GetTenantIDFromContext(r.Context())
	if callerTenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}
	if !isAdminTenant(callerTenantID) {
		response.Error(w, http.StatusForbidden, "Only administrators can change provider configuration", nil)
		return
	}

	var requested *int
	if raw := r.URL.Query().Get("tenant_id"); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil {
			response.Error(w, http.StatusBadRequest, "tenant_id must be a positive integer", nil)
			return
		}
		requested = &id
	}
	tenantID, err := configTargetTenant(callerTenantID, requested)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), nil)
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

package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
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
	// Iyzico fields
	IyzicoApiKey    string `json:"IYZICO_API_KEY,omitempty"`
	IyzicoSecretKey string `json:"IYZICO_SECRET_KEY,omitempty"`
	IyzicoEnv       string `json:"IYZICO_ENVIRONMENT,omitempty"`

	// OzanPay fields
	OzanpayApiKey    string `json:"OZANPAY_API_KEY,omitempty"`
	OzanpaySecretKey string `json:"OZANPAY_SECRET_KEY,omitempty"`
	OzanpayMerchant  string `json:"OZANPAY_MERCHANT_ID,omitempty"`
	OzanpayEnv       string `json:"OZANPAY_ENVIRONMENT,omitempty"`

	// Paycell fields
	PaycellUsername   string `json:"PAYCELL_USERNAME,omitempty"`
	PaycellPassword   string `json:"PAYCELL_PASSWORD,omitempty"`
	PaycellMerchantId string `json:"PAYCELL_MERCHANT_ID,omitempty"`
	PaycellTerminalId string `json:"PAYCELL_TERMINAL_ID,omitempty"`
	PaycellEnv        string `json:"PAYCELL_ENVIRONMENT,omitempty"`
}

// SetEnv handles setting environment variables for a tenant
func (h *ConfigHandler) SetEnv(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from header
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
		return
	}

	// Parse the request
	var req SetEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Track which providers were configured
	var configuredProviders []string

	// Process Iyzico configuration
	if req.IyzicoApiKey != "" || req.IyzicoSecretKey != "" {
		iyzicoConfig := make(map[string]string)

		if req.IyzicoApiKey != "" {
			iyzicoConfig["apiKey"] = req.IyzicoApiKey
		}
		if req.IyzicoSecretKey != "" {
			iyzicoConfig["secretKey"] = req.IyzicoSecretKey
		}
		if req.IyzicoEnv != "" {
			iyzicoConfig["environment"] = req.IyzicoEnv
		} else {
			iyzicoConfig["environment"] = "sandbox" // Default
		}

		if err := h.providerConfig.SetTenantConfig(tenantID, "iyzico", iyzicoConfig); err != nil {
			response.Error(w, http.StatusBadRequest, "Failed to set Iyzico configuration", err)
			return
		}

		// Register provider in payment service
		if err := h.registerTenantProvider(tenantID, "iyzico", iyzicoConfig); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to register Iyzico provider", err)
			return
		}

		configuredProviders = append(configuredProviders, "iyzico")
	}

	// Process OzanPay configuration
	if req.OzanpayApiKey != "" || req.OzanpaySecretKey != "" || req.OzanpayMerchant != "" {
		ozanpayConfig := make(map[string]string)

		if req.OzanpayApiKey != "" {
			ozanpayConfig["apiKey"] = req.OzanpayApiKey
		}
		if req.OzanpaySecretKey != "" {
			ozanpayConfig["secretKey"] = req.OzanpaySecretKey
		}
		if req.OzanpayMerchant != "" {
			ozanpayConfig["merchantId"] = req.OzanpayMerchant
		}
		if req.OzanpayEnv != "" {
			ozanpayConfig["environment"] = req.OzanpayEnv
		} else {
			ozanpayConfig["environment"] = "sandbox" // Default
		}

		if err := h.providerConfig.SetTenantConfig(tenantID, "ozanpay", ozanpayConfig); err != nil {
			response.Error(w, http.StatusBadRequest, "Failed to set OzanPay configuration", err)
			return
		}

		// Register provider in payment service
		if err := h.registerTenantProvider(tenantID, "ozanpay", ozanpayConfig); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to register OzanPay provider", err)
			return
		}

		configuredProviders = append(configuredProviders, "ozanpay")
	}

	// Process Paycell configuration
	if req.PaycellUsername != "" || req.PaycellPassword != "" || req.PaycellMerchantId != "" || req.PaycellTerminalId != "" {
		paycellConfig := make(map[string]string)

		if req.PaycellUsername != "" {
			paycellConfig["username"] = req.PaycellUsername
		}
		if req.PaycellPassword != "" {
			paycellConfig["password"] = req.PaycellPassword
		}
		if req.PaycellMerchantId != "" {
			paycellConfig["merchantId"] = req.PaycellMerchantId
		}
		if req.PaycellTerminalId != "" {
			paycellConfig["terminalId"] = req.PaycellTerminalId
		}
		if req.PaycellEnv != "" {
			paycellConfig["environment"] = req.PaycellEnv
		} else {
			paycellConfig["environment"] = "sandbox" // Default
		}

		if err := h.providerConfig.SetTenantConfig(tenantID, "paycell", paycellConfig); err != nil {
			response.Error(w, http.StatusBadRequest, "Failed to set Paycell configuration", err)
			return
		}

		// Register provider in payment service
		if err := h.registerTenantProvider(tenantID, "paycell", paycellConfig); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to register Paycell provider", err)
			return
		}

		configuredProviders = append(configuredProviders, "paycell")
	}

	if len(configuredProviders) == 0 {
		response.Error(w, http.StatusBadRequest, "No valid provider configuration found in request", nil)
		return
	}

	// Return success response
	responseData := map[string]interface{}{
		"tenantId":            tenantID,
		"configuredProviders": configuredProviders,
		"message":             "Provider configurations set successfully",
	}

	response.Success(w, http.StatusOK, "Configuration updated", responseData)
}

// registerTenantProvider registers a tenant-specific provider in the payment service
func (h *ConfigHandler) registerTenantProvider(tenantID, providerName string, config map[string]string) error {
	// Create tenant-specific provider name
	tenantProviderName := strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)

	// Add GoPay base URL to config
	config["gopayBaseURL"] = h.providerConfig.GetBaseURL()

	// Add provider to payment service
	if err := h.paymentService.AddProvider(tenantProviderName, config); err != nil {
		return err
	}

	return nil
}

// GetTenantConfig returns the configuration for a specific tenant and provider
func (h *ConfigHandler) GetTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from header
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
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

	responseData := map[string]interface{}{
		"tenantId": tenantID,
		"provider": providerName,
		"config":   publicConfig,
	}

	response.Success(w, http.StatusOK, "Configuration retrieved", responseData)
}

// DeleteTenantConfig deletes a tenant configuration
func (h *ConfigHandler) DeleteTenantConfig(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from header
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
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

	responseData := map[string]interface{}{
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

package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
)

// PaymentService manages payment operations through various providers
type PaymentService struct {
	providers       map[string]PaymentProvider
	defaultProvider string
	logger          PaymentLogger
	mu              sync.RWMutex
}

// NewPaymentService creates a new payment service
func NewPaymentService(logger PaymentLogger) *PaymentService {
	return &PaymentService{
		providers: make(map[string]PaymentProvider),
		logger:    logger,
	}
}

// AddProvider adds a configured payment provider to the service
func (s *PaymentService) AddProvider(name string, config map[string]string) error {
	provider, err := CreateProvider(name)
	if err != nil {
		return fmt.Errorf("failed to create provider '%s': %w", name, err)
	}

	if err := provider.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize provider '%s': %w", name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers[name] = provider

	// Set as default if it's the first provider
	if len(s.providers) == 1 {
		s.defaultProvider = name
	}

	return nil
}

// SetDefaultProvider sets the default payment provider
func (s *PaymentService) SetDefaultProvider(name string) error {
	s.mu.RLock()
	_, exists := s.providers[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("provider '%s' is not registered", name)
	}

	s.mu.Lock()
	s.defaultProvider = name
	s.mu.Unlock()

	return nil
}

// GetProvider returns a registered provider by name
func (s *PaymentService) GetProvider(name string) (PaymentProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider, exists := s.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' is not registered", name)
	}

	return provider, nil
}

// GetDefaultProvider returns the default payment provider
func (s *PaymentService) GetDefaultProvider() (PaymentProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.defaultProvider == "" {
		return nil, errors.New("no default provider set")
	}

	provider, exists := s.providers[s.defaultProvider]
	if !exists {
		return nil, errors.New("default provider not found")
	}

	return provider, nil
}

// CreatePayment processes a payment using the specified provider
func (s *PaymentService) CreatePayment(ctx context.Context, providerName string, request PaymentRequest) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	// Extract tenant ID from request
	tenantID := 1 // Default tenant
	if request.TenantID != "" {
		if tid, err := strconv.Atoi(request.TenantID); err == nil {
			tenantID = tid
		}
	}

	// Determine the actual provider name for DB logging
	actualProviderName := s.getActualProviderName(providerName)

	// Determine method and endpoint
	method := "POST"
	endpoint := "/payment"
	if request.Use3D {
		endpoint = "/payment/3d"
	}

	// Log request to database
	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, method, endpoint, request, request.ClientUserAgent, request.ClientIP)
	if err != nil {
		// Log error but continue with payment
		logger.Warn("Failed to log payment request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"error": err.Error(),
			},
		})
	}

	// Process payment
	var response *PaymentResponse
	if request.Use3D {
		response, err = provider.Create3DPayment(ctx, request)
	} else {
		response, err = provider.CreatePayment(ctx, request)
	}

	// Calculate processing time
	processingMs := time.Since(startTime).Milliseconds()

	// Log response or error
	if logID > 0 {
		if err != nil {
			// Log error
			if logErr := s.logger.LogError(ctx, logID, "PROVIDER_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log payment error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		} else {
			// Log successful response
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log payment response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (s *PaymentService) Complete3DPayment(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	// For 3D completion, we can try to find the original request in logs if needed
	// For now, we'll create a minimal log entry
	tenantID := 1 // Default, could be extracted from context or data
	actualProviderName := s.getActualProviderName(providerName)

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, "POST", "/payment/3d/complete", data, "", "")
	if err != nil {
		logger.Warn("Failed to log 3D completion request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"payment_id": paymentID,
				"error":      err.Error(),
			},
		})
	}

	response, err := provider.Complete3DPayment(ctx, paymentID, conversationID, data)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "3D_COMPLETION_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log 3D completion error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log 3D completion response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// GetPaymentStatus retrieves the current status of a payment
func (s *PaymentService) GetPaymentStatus(ctx context.Context, providerName, paymentID string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	tenantID := 1 // Default
	actualProviderName := s.getActualProviderName(providerName)

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, "GET", "/payment/status", map[string]string{"paymentID": paymentID}, "", "")
	if err != nil {
		logger.Warn("Failed to log status request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"payment_id": paymentID,
				"error":      err.Error(),
			},
		})
	}

	response, err := provider.GetPaymentStatus(ctx, paymentID)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "STATUS_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log status error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log status response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// CancelPayment cancels a payment
func (s *PaymentService) CancelPayment(ctx context.Context, providerName, paymentID, reason string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	tenantID := 1 // Default
	actualProviderName := s.getActualProviderName(providerName)

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, "POST", "/payment/cancel", map[string]string{"paymentID": paymentID, "reason": reason}, "", "")
	if err != nil {
		logger.Warn("Failed to log cancel request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"payment_id": paymentID,
				"reason":     reason,
				"error":      err.Error(),
			},
		})
	}

	response, err := provider.CancelPayment(ctx, paymentID, reason)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "CANCEL_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log cancel error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log cancel response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": paymentID,
						"error":      logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// RefundPayment issues a refund for a payment
func (s *PaymentService) RefundPayment(ctx context.Context, providerName string, request RefundRequest) (*RefundResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	tenantID := 1 // Default
	actualProviderName := s.getActualProviderName(providerName)

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, "POST", "/payment/refund", request, "", "")
	if err != nil {
		logger.Warn("Failed to log refund request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"payment_id": request.PaymentID,
				"error":      err.Error(),
			},
		})
	}

	response, err := provider.RefundPayment(ctx, request)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "REFUND_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log refund error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": request.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log refund response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": request.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// ValidateWebhook validates an incoming webhook notification
func (s *PaymentService) ValidateWebhook(ctx context.Context, providerName string, data, headers map[string]string) (bool, map[string]string, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return false, nil, err
	}

	tenantID := 1 // Default
	actualProviderName := s.getActualProviderName(providerName)

	startTime := time.Now()
	webhookData := map[string]any{
		"data":    data,
		"headers": headers,
	}
	logID, err := s.logger.LogRequest(ctx, tenantID, actualProviderName, "POST", "/webhook", webhookData, "", "")
	if err != nil {
		logger.Warn("Failed to log webhook request", logger.LogContext{
			Provider: actualProviderName,
			Fields: map[string]any{
				"error": err.Error(),
			},
		})
	}

	valid, result, err := provider.ValidateWebhook(ctx, data, headers)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		webhookResult := map[string]any{
			"valid":  valid,
			"result": result,
		}

		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "WEBHOOK_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log webhook error", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, webhookResult, processingMs); logErr != nil {
				logger.Warn("Failed to log webhook response", logger.LogContext{
					Provider: actualProviderName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		}
	}

	return valid, result, err
}

// getActualProviderName extracts the actual provider name from tenant-specific provider names
func (s *PaymentService) getActualProviderName(providerName string) string {
	if providerName == "" {
		return s.defaultProvider
	}

	// Handle tenant-specific provider names like "TENANT1_paycell"
	parts := strings.Split(providerName, "_")
	if len(parts) > 1 {
		return parts[len(parts)-1] // Return the last part (actual provider name)
	}

	return providerName
}

// Helper method to get the right provider for an operation
func (s *PaymentService) getProviderForOperation(providerName string) (PaymentProvider, error) {
	if providerName != "" {
		return s.GetProvider(providerName)
	}

	return s.GetDefaultProvider()
}

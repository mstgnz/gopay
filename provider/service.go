package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/middle"
)

// getTenantIDFromContext extracts and validates tenant ID from context
func getTenantIDFromContext(ctx context.Context) (int, error) {
	tenantIDStr, ok := ctx.Value(middle.TenantIDKey).(string)
	if !ok || tenantIDStr == "" {
		return 0, fmt.Errorf("tenant ID not found in context")
	}

	tenantID, err := strconv.Atoi(tenantIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid tenant ID format '%s': %v", tenantIDStr, err)
	}

	return tenantID, nil
}

// PaymentService manages payment operations through various providers
type PaymentService struct {
	logger PaymentLogger
}

// NewPaymentService creates a new payment service
func NewPaymentService(logger PaymentLogger) *PaymentService {
	return &PaymentService{
		logger: logger,
	}
}

// CreatePayment processes a payment using the specified provider
func (s *PaymentService) CreatePayment(ctx context.Context, environment, providerName string, request PaymentRequest) (*PaymentResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	request.TenantID = tenantID
	request.Environment = environment
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	// Determine method and endpoint
	method := "POST"
	endpoint := "/payment"
	if request.Use3D {
		endpoint = "/payment/3d"
	}

	// Log request to database
	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, method, endpoint, request, request.ClientUserAgent, request.ClientIP)
	if err != nil {
		// Log error but continue with payment
		logger.Warn("Failed to log payment request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"error": err.Error(),
			},
		})
	}
	// required to add provider request to client request
	request.LogID = logID

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
					Provider: providerName,
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
					Provider: providerName,
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
func (s *PaymentService) Complete3DPayment(ctx context.Context, providerName, state string, data map[string]string) (*PaymentResponse, error) {
	callbackState, err := HandleCallbackState(ctx, state)
	if err != nil {
		return nil, err
	}

	provider, err := GetProvider(callbackState.TenantID, providerName, callbackState.Environment)
	if err != nil {
		return nil, err
	}

	data["currency"] = callbackState.Currency
	data["paymentId"] = callbackState.PaymentID
	data["amount"] = fmt.Sprintf("%.2f", callbackState.Amount)

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, callbackState.TenantID, providerName, "POST", "/payment/3d/complete", data, "", "")
	if err != nil {
		logger.Warn("Failed to log 3D completion request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"payment_id": callbackState.PaymentID,
				"error":      err.Error(),
			},
		})
	}
	callbackState.LogID = logID

	response, err := provider.Complete3DPayment(ctx, callbackState, data)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "3D_COMPLETION_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log 3D completion error", logger.LogContext{
					Provider: providerName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": callbackState.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log 3D completion response", logger.LogContext{
					Provider: providerName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": callbackState.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		}
	}

	return response, err
}

// GetPaymentStatus retrieves the current status of a payment
func (s *PaymentService) GetPaymentStatus(ctx context.Context, environment, providerName string, request GetPaymentStatusRequest) (*PaymentResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "GET", "/payment/status", request, "", "")
	if err != nil {
		logger.Warn("Failed to log status request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"payment_id": request.PaymentID,
				"error":      err.Error(),
			},
		})
	}

	request.LogID = logID
	response, err := provider.GetPaymentStatus(ctx, request)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "STATUS_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log status error", logger.LogContext{
					Provider: providerName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": request.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log status response", logger.LogContext{
					Provider: providerName,
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

// CancelPayment cancels a payment
func (s *PaymentService) CancelPayment(ctx context.Context, environment, providerName string, request CancelRequest) (*PaymentResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "POST", "/payment/cancel", request, "", "")
	if err != nil {
		logger.Warn("Failed to log cancel request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"payment_id": request.PaymentID,
				"reason":     request.Reason,
				"error":      err.Error(),
			},
		})
	}

	request.LogID = logID
	response, err := provider.CancelPayment(ctx, request)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "CANCEL_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log cancel error", logger.LogContext{
					Provider: providerName,
					Fields: map[string]any{
						"log_id":     logID,
						"payment_id": request.PaymentID,
						"error":      logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log cancel response", logger.LogContext{
					Provider: providerName,
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

// RefundPayment issues a refund for a payment
func (s *PaymentService) RefundPayment(ctx context.Context, environment, providerName string, request RefundRequest) (*RefundResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "POST", "/payment/refund", request, "", "")
	if err != nil {
		logger.Warn("Failed to log refund request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"payment_id": request.PaymentID,
				"error":      err.Error(),
			},
		})
	}

	request.LogID = logID
	response, err := provider.RefundPayment(ctx, request)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "REFUND_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log refund error", logger.LogContext{
					Provider: providerName,
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
					Provider: providerName,
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

func (s *PaymentService) GetInstallmentCount(ctx context.Context, environment, providerName string, request InstallmentInquireRequest) (InstallmentInquireResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return InstallmentInquireResponse{}, err
	}
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return InstallmentInquireResponse{}, err
	}

	startTime := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "POST", "/payment/installment", request, "", "")
	if err != nil {
		logger.Warn("Failed to log installment count request", logger.LogContext{
			Provider: providerName,
			Fields: map[string]any{
				"error": err.Error(),
			},
		})
	}

	request.LogID = logID
	response, err := provider.GetInstallmentCount(ctx, request)

	processingMs := time.Since(startTime).Milliseconds()

	if logID > 0 {
		if err != nil {
			if logErr := s.logger.LogError(ctx, logID, "INSTALLMENT_COUNT_ERROR", err.Error(), processingMs); logErr != nil {
				logger.Warn("Failed to log installment count error", logger.LogContext{
					Provider: providerName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, response, processingMs); logErr != nil {
				logger.Warn("Failed to log installment count response", logger.LogContext{
					Provider: providerName,
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

// ValidateWebhook validates an incoming webhook notification
func (s *PaymentService) ValidateWebhook(ctx context.Context, environment, providerName string, data, headers map[string]string) (bool, map[string]string, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return false, nil, err
	}
	provider, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return false, nil, err
	}

	startTime := time.Now()
	webhookData := map[string]any{
		"data":    data,
		"headers": headers,
	}
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "POST", "/webhook", webhookData, "", "")
	if err != nil {
		logger.Warn("Failed to log webhook request", logger.LogContext{
			Provider: providerName,
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
					Provider: providerName,
					Fields: map[string]any{
						"log_id": logID,
						"error":  logErr.Error(),
					},
				})
			}
		} else {
			if logErr := s.logger.LogResponse(ctx, logID, webhookResult, processingMs); logErr != nil {
				logger.Warn("Failed to log webhook response", logger.LogContext{
					Provider: providerName,
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

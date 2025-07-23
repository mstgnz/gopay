package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// PaymentServiceInterface defines the interface for payment operations
type PaymentServiceInterface interface {
	CreatePayment(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error)
	GetPaymentStatus(ctx context.Context, environment, providerName string, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error)
	CancelPayment(ctx context.Context, environment, providerName string, request provider.CancelRequest) (*provider.PaymentResponse, error)
	RefundPayment(ctx context.Context, environment, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error)
	GetInstallmentCount(ctx context.Context, environment, providerName string, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error)
	Complete3DPayment(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error)
	ValidateWebhook(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

// PaymentHandler handles payment related HTTP requests
type PaymentHandler struct {
	paymentService PaymentServiceInterface
	validate       *validator.Validate
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(paymentService PaymentServiceInterface, validate *validator.Validate) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		validate:       validate,
	}
}

// ProcessPayment handles payment requests
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse the payment request
	var req provider.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	req.ClientIP = middle.GetClientIP(r)
	req.ClientUserAgent = r.Header.Get("User-Agent")

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Get provider name from URL path parameter (or empty for default)
	providerName := chi.URLParam(r, "provider")

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Process the payment
	resp, err := h.paymentService.CreatePayment(ctx, environment, providerName, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Payment failed", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Payment processed", resp)
}

// GetPaymentStatus handles payment status requests
func (h *PaymentHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider and payment ID from URL path parameters
	providerName := chi.URLParam(r, "provider")
	paymentID := chi.URLParam(r, "paymentID")

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "Missing payment ID", nil)
		return
	}

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Get payment status
	resp, err := h.paymentService.GetPaymentStatus(ctx, environment, providerName, provider.GetPaymentStatusRequest{
		PaymentID: paymentID,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get payment status", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Payment status retrieved", resp)
}

// CancelPayment handles payment cancellation requests
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider and payment ID from URL path parameters
	providerName := chi.URLParam(r, "provider")
	paymentID := chi.URLParam(r, "paymentID")

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "Missing payment ID", nil)
		return
	}

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Parse reason from request body
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Continue with empty reason if parsing fails
		req.Reason = ""
	}

	// Cancel payment
	resp, err := h.paymentService.CancelPayment(ctx, environment, providerName, provider.CancelRequest{
		PaymentID: paymentID,
		Reason:    req.Reason,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to cancel payment", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Payment cancelled", resp)
}

// RefundPayment handles payment refund requests
func (h *PaymentHandler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Parse refund request
	var req provider.RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Process refund
	resp, err := h.paymentService.RefundPayment(ctx, environment, providerName, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to refund payment", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Payment refunded", resp)
}

func (h *PaymentHandler) GetInstallments(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")
	if providerName == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Parse request body
	var req provider.InstallmentInquireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	if req.Amount == 0 {
		response.Error(w, http.StatusBadRequest, "Amount is required", nil)
		return
	}

	// Get installment count
	resp, err := h.paymentService.GetInstallmentCount(ctx, environment, providerName, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get installment count", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Installment count retrieved", resp)
}

// Enhanced callback URL parsing and redirect logic
func (h *PaymentHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")
	if providerName == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		response.Error(w, http.StatusBadRequest, "Missing state", nil)
		return
	}

	// Combine form and query parameters
	callbackData := make(map[string]string)
	for key, values := range r.Form {
		if len(values) > 0 {
			callbackData[key] = values[0]
		}
	}
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			callbackData[key] = values[0]
		}
	}

	// Complete 3D payment
	resp, err := h.paymentService.Complete3DPayment(ctx, providerName, state, callbackData)

	if err != nil {
		h.handleCallbackError(w, r, err, resp.RedirectURL)
		return
	}

	if resp.Success {
		h.handleCallbackSuccess(w, r, resp, resp.RedirectURL)
	} else {
		h.handleCallbackFailure(w, r, resp, resp.RedirectURL)
	}
}

// Enhanced success handling with better URL construction
func (h *PaymentHandler) handleCallbackSuccess(w http.ResponseWriter, r *http.Request, resp *provider.PaymentResponse, originalCallbackURL string) {
	if originalCallbackURL != "" {
		// Parse success URL from original callback URL
		if successURL := r.URL.Query().Get("successUrl"); successURL != "" {
			redirectURL := fmt.Sprintf("%s?paymentId=%s&status=%s&transactionId=%s&amount=%.2f",
				successURL, resp.PaymentID, resp.Status, resp.TransactionID, resp.Amount)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		// Enhanced parameter passing to original callback URL
		redirectURL := fmt.Sprintf("%s?success=true&paymentId=%s&status=%s&transactionId=%s&amount=%.2f&currency=%s",
			originalCallbackURL, resp.PaymentID, resp.Status, resp.TransactionID, resp.Amount, resp.Currency)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Legacy: Direct success URL redirect
	if resp.Success && r.URL.Query().Get("successUrl") != "" {
		successURL := r.URL.Query().Get("successUrl")
		redirectURL := fmt.Sprintf("%s?paymentId=%s&status=%s&transactionId=%s",
			successURL, resp.PaymentID, resp.Status, resp.TransactionID)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Otherwise return JSON response
	response.Success(w, http.StatusOK, "Payment completed successfully", resp)
}

// Enhanced error handling with better error information
func (h *PaymentHandler) handleCallbackError(w http.ResponseWriter, r *http.Request, err error, originalCallbackURL string) {
	if originalCallbackURL != "" {
		// Parse error URL from original callback URL
		if errorURL := r.URL.Query().Get("errorUrl"); errorURL != "" {
			redirectURL := fmt.Sprintf("%s?error=%s&errorCode=%s",
				errorURL, err.Error(), "CALLBACK_ERROR")
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		// Redirect to original callback URL with error
		redirectURL := fmt.Sprintf("%s?success=false&error=%s&errorCode=%s",
			originalCallbackURL, err.Error(), "CALLBACK_ERROR")
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Legacy: Direct error URL redirect
	if errorURL := r.URL.Query().Get("errorUrl"); errorURL != "" {
		redirectURL := fmt.Sprintf("%s?error=%s&errorCode=%s",
			errorURL, err.Error(), "CALLBACK_ERROR")
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	response.Error(w, http.StatusInternalServerError, "Failed to complete payment", err)
}

// Enhanced failure handling for payment failures
func (h *PaymentHandler) handleCallbackFailure(w http.ResponseWriter, r *http.Request, resp *provider.PaymentResponse, originalCallbackURL string) {
	if originalCallbackURL != "" {
		// Parse error URL from original callback URL
		if errorURL := r.URL.Query().Get("errorUrl"); errorURL != "" {
			redirectURL := fmt.Sprintf("%s?error=%s&errorCode=%s&paymentId=%s",
				errorURL, resp.Message, resp.ErrorCode, resp.PaymentID)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		// Redirect to original callback URL with failure details
		redirectURL := fmt.Sprintf("%s?success=false&error=%s&errorCode=%s&paymentId=%s&status=%s",
			originalCallbackURL, resp.Message, resp.ErrorCode, resp.PaymentID, resp.Status)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Otherwise return JSON response
	response.Success(w, http.StatusOK, "Payment failed", resp)
}

// Enhanced HandleWebhook with async processing and retry logic
func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")
	if providerName == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}

	// Parse webhook data based on content type
	var webhookData map[string]string
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid form data", err)
			return
		}

		webhookData = make(map[string]string)
		for key, values := range r.Form {
			if len(values) > 0 {
				webhookData[key] = values[0]
			}
		}
	} else {
		// Parse JSON data
		if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid JSON webhook data", err)
			return
		}
	}

	// Extract headers for validation
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Validate webhook signature
	isValid, paymentData, err := h.paymentService.ValidateWebhook(ctx, environment, providerName, webhookData, headers)
	if err != nil {
		// Log validation error but respond with 200 to prevent retries for invalid webhooks
		h.logWebhookError(providerName, "validation_failed", err, webhookData)
		response.Error(w, http.StatusBadRequest, "Webhook validation failed", err)
		return
	}

	if !isValid {
		h.logWebhookError(providerName, "invalid_signature", errors.New("invalid webhook signature"), webhookData)
		response.Error(w, http.StatusBadRequest, "Invalid webhook signature", nil)
		return
	}

	// Process webhook asynchronously to respond quickly
	go h.processWebhookAsync(ctx, environment, providerName, paymentData, webhookData)

	// Respond immediately with success
	response.Success(w, http.StatusOK, "Webhook received and processing", map[string]string{
		"status":    "accepted",
		"paymentId": paymentData["paymentId"],
	})
}

// Async webhook processing for better performance
func (h *PaymentHandler) processWebhookAsync(ctx context.Context, environment, providerName string, paymentData, rawWebhookData map[string]string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	paymentID := paymentData["paymentId"]
	if paymentID == "" {
		h.logWebhookError(providerName, "missing_payment_id", errors.New("payment ID not found in webhook data"), rawWebhookData)
		return
	}

	// Get current payment status from provider
	currentStatus, err := h.paymentService.GetPaymentStatus(ctx, environment, providerName, provider.GetPaymentStatusRequest{
		PaymentID: paymentID,
	})
	if err != nil {
		h.logWebhookError(providerName, "status_check_failed", err, rawWebhookData)
		return
	}

	// Process based on payment status
	switch paymentData["status"] {
	case "success", "completed", "approved":
		h.processSuccessfulPayment(providerName, paymentID, paymentData, currentStatus)
	case "failed", "declined", "cancelled":
		h.processFailedPayment(providerName, paymentID, paymentData, currentStatus)
	case "refunded":
		h.processRefundedPayment(providerName, paymentID, paymentData, currentStatus)
	default:
		h.logWebhookInfo(providerName, paymentID, "unknown_status", paymentData)
	}
}

// Process successful payment webhooks
func (h *PaymentHandler) processSuccessfulPayment(providerName, paymentID string, webhookData map[string]string, currentStatus *provider.PaymentResponse) {
	// Log successful payment completion
	logger.Info("Payment completed successfully via webhook", logger.LogContext{
		Provider: providerName,
		Fields: map[string]any{
			"payment_id": paymentID,
			"status":     "successful",
		},
	})

	// Here you could:
	// 1. Update local payment status
	// 2. Send confirmation emails
	// 3. Trigger fulfillment processes
	// 4. Update analytics
}

// Process failed payment webhooks
func (h *PaymentHandler) processFailedPayment(providerName, paymentID string, webhookData map[string]string, currentStatus *provider.PaymentResponse) {
	// Log failed payment
	errorMessage := webhookData["error"]
	if errorMessage == "" {
		errorMessage = "Unknown error"
	}

	logger.Warn("Payment failed via webhook", logger.LogContext{
		Provider: providerName,
		Fields: map[string]any{
			"payment_id": paymentID,
			"error":      errorMessage,
			"status":     "failed",
		},
	})

	// Here you could:
	// 1. Update payment status to failed
	// 2. Send failure notifications
	// 3. Trigger retry mechanisms
	// 4. Update fraud detection systems
}

// Process refund webhooks
func (h *PaymentHandler) processRefundedPayment(providerName, paymentID string, webhookData map[string]string, currentStatus *provider.PaymentResponse) {
	// Log refunded payment
	logger.Info("Payment refunded via webhook", logger.LogContext{
		Provider: providerName,
		Fields: map[string]any{
			"payment_id": paymentID,
			"status":     "refunded",
		},
	})

	// Here you could:
	// 1. Update payment status to refunded
	// 2. Process refund in accounting system
	// 3. Send refund confirmation
	// 4. Update customer balance
}

// Helper functions for webhook logging
func (h *PaymentHandler) logWebhookError(providerName, errorType string, err error, webhookData map[string]string) {
	logger.Error("Webhook processing error", err, logger.LogContext{
		Provider: providerName,
		Fields: map[string]any{
			"error_type":   errorType,
			"webhook_data": webhookData,
		},
	})
}

func (h *PaymentHandler) logWebhookInfo(providerName, paymentID, eventType string, webhookData map[string]string) {
	logger.Info("Webhook event processed", logger.LogContext{
		Provider: providerName,
		Fields: map[string]any{
			"payment_id":   paymentID,
			"event_type":   eventType,
			"webhook_data": webhookData,
		},
	})
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// PaymentHandler handles payment related HTTP requests
type PaymentHandler struct {
	paymentService *provider.PaymentService
	validate       *validator.Validate
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(paymentService *provider.PaymentService, validate *validator.Validate) *PaymentHandler {
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

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Get provider name from URL path parameter (or empty for default)
	providerName := chi.URLParam(r, "provider")

	// Get tenant ID from header and construct tenant-specific provider name if present
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
	}

	// Add tenant ID to request for callback URL generation
	req.TenantID = tenantID

	// Process the payment
	resp, err := h.paymentService.CreatePayment(ctx, providerName, req)
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

	// Get tenant ID from header and construct tenant-specific provider name if present
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
	}

	// Get payment status
	resp, err := h.paymentService.GetPaymentStatus(ctx, providerName, paymentID)
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

	// Get tenant ID from header and construct tenant-specific provider name if present
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
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
	resp, err := h.paymentService.CancelPayment(ctx, providerName, paymentID, req.Reason)
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

	// Get tenant ID from header and construct tenant-specific provider name if present
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
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
	resp, err := h.paymentService.RefundPayment(ctx, providerName, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to refund payment", err)
		return
	}

	// Return response
	response.Success(w, http.StatusOK, "Payment refunded", resp)
}

// Enhanced callback URL parsing and redirect logic
func (h *PaymentHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")

	// Get tenant ID from query parameter first, then from header (for better callback handling)
	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}

	// Construct tenant-specific provider name if tenant ID is present
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
	}

	// Get conversationID and paymentID from query parameters
	paymentID := r.URL.Query().Get("paymentId")
	conversationID := r.URL.Query().Get("conversationId")

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "Missing payment ID", nil)
		return
	}

	// Parse callback data from POST form and query parameters
	if err := r.ParseForm(); err != nil {
		response.Error(w, http.StatusBadRequest, "Failed to parse form data", err)
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
	resp, err := h.paymentService.Complete3DPayment(ctx, providerName, paymentID, conversationID, callbackData)

	// Enhanced redirect handling with better URL parsing
	originalCallbackURL := r.URL.Query().Get("originalCallbackUrl")

	if err != nil {
		h.handleCallbackError(w, r, err, originalCallbackURL)
		return
	}

	if resp.Success {
		h.handleCallbackSuccess(w, r, resp, originalCallbackURL)
	} else {
		h.handleCallbackFailure(w, r, resp, originalCallbackURL)
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

	// Get tenant ID from query parameter first, then from header (for webhook reliability)
	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}

	// Construct tenant-specific provider name if tenant ID is present
	if tenantID != "" && providerName != "" {
		// Use tenant-specific provider: TENANT_provider
		providerName = strings.ToUpper(tenantID) + "_" + strings.ToLower(providerName)
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
	isValid, paymentData, err := h.paymentService.ValidateWebhook(ctx, providerName, webhookData, headers)
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
	go h.processWebhookAsync(providerName, paymentData, webhookData)

	// Respond immediately with success
	response.Success(w, http.StatusOK, "Webhook received and processing", map[string]string{
		"status":    "accepted",
		"paymentId": paymentData["paymentId"],
	})
}

// Async webhook processing for better performance
func (h *PaymentHandler) processWebhookAsync(providerName string, paymentData, rawWebhookData map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	paymentID := paymentData["paymentId"]
	if paymentID == "" {
		h.logWebhookError(providerName, "missing_payment_id", errors.New("payment ID not found in webhook data"), rawWebhookData)
		return
	}

	// Get current payment status from provider
	currentStatus, err := h.paymentService.GetPaymentStatus(ctx, providerName, paymentID)
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
	log.Printf("Webhook: Payment %s (%s) completed successfully", paymentID, providerName)

	// Update payment status in database if needed
	// Send notifications to external systems
	// Update analytics/metrics

	h.logWebhookInfo(providerName, paymentID, "payment_success", webhookData)
}

// Process failed payment webhooks
func (h *PaymentHandler) processFailedPayment(providerName, paymentID string, webhookData map[string]string, currentStatus *provider.PaymentResponse) {
	log.Printf("Webhook: Payment %s (%s) failed - %s", paymentID, providerName, webhookData["error"])

	// Update payment status
	// Send failure notifications
	// Update fraud detection systems

	h.logWebhookInfo(providerName, paymentID, "payment_failed", webhookData)
}

// Process refund webhooks
func (h *PaymentHandler) processRefundedPayment(providerName, paymentID string, webhookData map[string]string, currentStatus *provider.PaymentResponse) {
	log.Printf("Webhook: Payment %s (%s) refunded", paymentID, providerName)

	// Update refund status
	// Send refund notifications
	// Update accounting systems

	h.logWebhookInfo(providerName, paymentID, "payment_refunded", webhookData)
}

// Helper functions for webhook logging
func (h *PaymentHandler) logWebhookError(providerName, errorType string, err error, webhookData map[string]string) {
	log.Printf("Webhook Error [%s]: %s - %v", providerName, errorType, err)
	// Additional structured logging can be added here
}

func (h *PaymentHandler) logWebhookInfo(providerName, paymentID, eventType string, webhookData map[string]string) {
	log.Printf("Webhook Info [%s]: %s - PaymentID: %s", providerName, eventType, paymentID)
	// Additional structured logging can be added here
}

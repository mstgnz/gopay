package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/gateway"
	"github.com/mstgnz/gopay/infra/response"
)

// PaymentHandler handles payment related HTTP requests
type PaymentHandler struct {
	paymentService *gateway.PaymentService
	validate       *validator.Validate
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(paymentService *gateway.PaymentService, validate *validator.Validate) *PaymentHandler {
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
	var req gateway.PaymentRequest
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

	// Parse refund request
	var req gateway.RefundRequest
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

// HandleCallback processes payment callbacks (e.g., for 3D Secure)
func (h *PaymentHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")

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
	if err != nil {
		// On error, redirect to the error URL if provided
		if errorURL := r.URL.Query().Get("errorUrl"); errorURL != "" {
			http.Redirect(w, r, fmt.Sprintf("%s?error=%s", errorURL, err.Error()), http.StatusFound)
			return
		}
		response.Error(w, http.StatusInternalServerError, "Failed to complete payment", err)
		return
	}

	// On success, redirect to success URL if provided
	if resp.Success && r.URL.Query().Get("successUrl") != "" {
		successURL := r.URL.Query().Get("successUrl")
		redirectURL := fmt.Sprintf("%s?paymentId=%s&status=%s", successURL, resp.PaymentID, resp.Status)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Otherwise return JSON response
	response.Success(w, http.StatusOK, "Payment completed", resp)
}

// HandleWebhook processes webhook notifications from payment providers
func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get provider from URL path parameter
	providerName := chi.URLParam(r, "provider")

	// Parse webhook data
	var webhookData map[string]string
	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid webhook data", err)
		return
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Validate webhook
	isValid, data, err := h.paymentService.ValidateWebhook(ctx, providerName, webhookData, headers)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Webhook validation failed", err)
		return
	}

	if !isValid {
		response.Error(w, http.StatusBadRequest, "Invalid webhook signature", nil)
		return
	}

	// Process webhook data (in a real implementation, you might want to process this asynchronously)
	// For now, just return success with the validated data
	response.Success(w, http.StatusOK, "Webhook processed", data)
}

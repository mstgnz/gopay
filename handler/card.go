package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// CardServiceInterface defines the card-storage operations the handler depends on.
type CardServiceInterface interface {
	SendCardOTP(ctx context.Context, environment, providerName string, request provider.CardOTPSendRequest) (*provider.CardOTPSendResponse, error)
	ValidateCardOTP(ctx context.Context, environment, providerName string, request provider.CardOTPValidateRequest) (*provider.CardOTPValidateResponse, error)
	RegisterCard(ctx context.Context, environment, providerName string, request provider.RegisterCardRequest) (*provider.RegisterCardResponse, *provider.SavedCard, error)
	ListSavedCards(ctx context.Context, environment, providerName, msisdn string) ([]provider.SavedCard, error)
	DeleteSavedCard(ctx context.Context, environment, providerName string, cardRowID int) error
	PaySavedCard(ctx context.Context, environment, providerName string, cardRowID int, request provider.SavedCardPaymentRequest, use3D bool) (*provider.PaymentResponse, error)
}

// CardHandler handles card-storage related HTTP requests.
type CardHandler struct {
	cardService CardServiceInterface
	validate    *validator.Validate
}

// NewCardHandler creates a new card handler.
func NewCardHandler(cardService CardServiceInterface, validate *validator.Validate) *CardHandler {
	return &CardHandler{cardService: cardService, validate: validate}
}

// Request bodies are handler-local DTOs (mass-assignment guard): only client-facing fields are
// bound; internal fields (tenant id, log id, provider card id, environment) are set server-side.

type sendOTPBody struct {
	MSISDN string `json:"msisdn" validate:"required"`
}

type validateOTPBody struct {
	MSISDN          string `json:"msisdn" validate:"required"`
	ReferenceNumber string `json:"referenceNumber" validate:"required"`
	OTP             string `json:"otp" validate:"required"`
	Token           string `json:"token,omitempty"`
}

type registerCardBody struct {
	MSISDN          string            `json:"msisdn" validate:"required"`
	Card            provider.CardInfo `json:"card" validate:"required"`
	Alias           string            `json:"alias,omitempty"`
	ReferenceNumber string            `json:"referenceNumber,omitempty"`
}

type paySavedCardBody struct {
	MSISDN           string  `json:"msisdn,omitempty"`
	Amount           float64 `json:"amount" validate:"required,gt=0"`
	Currency         string  `json:"currency" validate:"required"`
	InstallmentCount int     `json:"installmentCount,omitempty"`
	Use3D            bool    `json:"use3D,omitempty"`
	CallbackURL      string  `json:"callbackUrl,omitempty"`
	ConversationID   string  `json:"conversationId,omitempty"`
	SessionID        string  `json:"sessionId,omitempty"`
}

func environmentFromRequest(r *http.Request) string {
	environment := r.URL.Query().Get("environment")
	if environment != "production" {
		environment = "sandbox"
	}
	return environment
}

// SendOTP handles POST /payments/{provider}/cards/otp/send
func (h *CardHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body sendOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	if err := h.validate.Struct(body); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	resp, err := h.cardService.SendCardOTP(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), provider.CardOTPSendRequest{
		MSISDN:   body.MSISDN,
		ClientIP: middle.GetClientIP(r),
	})
	if err != nil {
		h.writeServiceError(w, "Failed to send OTP", err)
		return
	}
	response.Return(w, http.StatusOK, resp.Success, resp.Message, resp)
}

// ValidateOTP handles POST /payments/{provider}/cards/otp/validate
func (h *CardHandler) ValidateOTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body validateOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	if err := h.validate.Struct(body); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	resp, err := h.cardService.ValidateCardOTP(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), provider.CardOTPValidateRequest{
		MSISDN:          body.MSISDN,
		ReferenceNumber: body.ReferenceNumber,
		OTP:             body.OTP,
		Token:           body.Token,
		ClientIP:        middle.GetClientIP(r),
	})
	if err != nil {
		h.writeServiceError(w, "Failed to validate OTP", err)
		return
	}
	response.Return(w, http.StatusOK, resp.Success, resp.Message, resp)
}

// RegisterCard handles POST /payments/{provider}/cards/register
func (h *CardHandler) RegisterCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body registerCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	if err := h.validate.Struct(body); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	resp, card, err := h.cardService.RegisterCard(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), provider.RegisterCardRequest{
		MSISDN:          body.MSISDN,
		Card:            body.Card,
		Alias:           body.Alias,
		ReferenceNumber: body.ReferenceNumber,
		ClientIP:        middle.GetClientIP(r),
	})
	if err != nil {
		h.writeServiceError(w, "Failed to register card", err)
		return
	}

	// Return only the masked saved-card record, never raw card data.
	response.Success(w, http.StatusOK, "Card registered", map[string]any{
		"card":      card,
		"message":   resp.Message,
		"errorCode": resp.ErrorCode,
	})
}

// ListCards handles GET /payments/{provider}/cards?msisdn=...
func (h *CardHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	msisdn := r.URL.Query().Get("msisdn")
	if msisdn == "" {
		response.Error(w, http.StatusBadRequest, "Missing msisdn", nil)
		return
	}

	cards, err := h.cardService.ListSavedCards(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), msisdn)
	if err != nil {
		h.writeServiceError(w, "Failed to list cards", err)
		return
	}
	response.Success(w, http.StatusOK, "Saved cards", cards)
}

// DeleteCard handles DELETE /payments/{provider}/cards/{cardId}
func (h *CardHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	cardID, err := strconv.Atoi(chi.URLParam(r, "cardId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid card id", err)
		return
	}

	if err := h.cardService.DeleteSavedCard(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), cardID); err != nil {
		h.writeServiceError(w, "Failed to delete card", err)
		return
	}
	response.Success(w, http.StatusOK, "Card deleted", nil)
}

// PayWithCard handles POST /payments/{provider}/cards/{cardId}/pay
func (h *CardHandler) PayWithCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	cardID, err := strconv.Atoi(chi.URLParam(r, "cardId"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid card id", err)
		return
	}

	var body paySavedCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}
	if err := h.validate.Struct(body); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	resp, err := h.cardService.PaySavedCard(ctx, environmentFromRequest(r), chi.URLParam(r, "provider"), cardID, provider.SavedCardPaymentRequest{
		MSISDN:           body.MSISDN,
		Amount:           body.Amount,
		Currency:         body.Currency,
		InstallmentCount: body.InstallmentCount,
		CallbackURL:      body.CallbackURL,
		ConversationID:   body.ConversationID,
		SessionID:        body.SessionID,
		ClientIP:         middle.GetClientIP(r),
		ClientUserAgent:  r.Header.Get("User-Agent"),
	}, body.Use3D)
	if err != nil {
		h.writeServiceError(w, "Payment failed", err)
		return
	}
	response.Return(w, http.StatusOK, resp.Success, resp.Message, resp)
}

// writeServiceError maps card-service errors to appropriate HTTP status codes.
func (h *CardHandler) writeServiceError(w http.ResponseWriter, message string, err error) {
	switch {
	case errors.Is(err, provider.ErrCardStorageUnsupported):
		response.Error(w, http.StatusBadRequest, "Provider does not support card storage", err)
	case errors.Is(err, provider.ErrSavedCardNotFound):
		response.Error(w, http.StatusNotFound, "Saved card not found", err)
	default:
		response.Error(w, http.StatusInternalServerError, message, err)
	}
}

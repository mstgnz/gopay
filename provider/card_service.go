package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/logger"
)

// CardService orchestrates card-storage operations: it resolves the tenant + provider, ensures the
// provider implements the optional CardStorageProvider capability, logs each request/response like
// PaymentService, and persists saved-card rows (scoped by tenant) for later charging.
type CardService struct {
	logger         PaymentLogger
	repo           *SavedCardRepository
	providerConfig *config.ProviderConfig
}

// NewCardService creates a new card service.
func NewCardService(logger PaymentLogger, repo *SavedCardRepository, providerConfig *config.ProviderConfig) *CardService {
	return &CardService{logger: logger, repo: repo, providerConfig: providerConfig}
}

// normalizeMSISDN strips the Turkish country code to the 10-digit form stored and matched everywhere.
func normalizeMSISDN(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+90")
	phone = strings.TrimPrefix(phone, "90")
	return phone
}

// getCardStorageProvider resolves the provider for a tenant and asserts the card-storage capability.
func getCardStorageProvider(tenantID int, providerName, environment string) (CardStorageProvider, error) {
	p, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}
	cs, ok := p.(CardStorageProvider)
	if !ok {
		return nil, ErrCardStorageUnsupported
	}
	return cs, nil
}

// startLog logs the request and returns the log id plus the start time for processing measurement.
func (s *CardService) startLog(ctx context.Context, tenantID int, providerName, endpoint string, request any, clientIP, userAgent string) (int64, time.Time) {
	start := time.Now()
	logID, err := s.logger.LogRequest(ctx, tenantID, providerName, "POST", endpoint, request, userAgent, clientIP)
	if err != nil {
		logger.Warn("Failed to log card request", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"error": err.Error()},
		})
	}
	return logID, start
}

// finishLog logs either the error or the response for a given log id.
func (s *CardService) finishLog(ctx context.Context, logID int64, start time.Time, response any, opErr error) {
	if logID <= 0 {
		return
	}
	ms := time.Since(start).Milliseconds()
	if opErr != nil {
		if err := s.logger.LogError(ctx, logID, "CARD_ERROR", opErr.Error(), ms); err != nil {
			logger.Warn("Failed to log card error", logger.LogContext{Fields: map[string]any{"log_id": logID, "error": err.Error()}})
		}
		return
	}
	if err := s.logger.LogResponse(ctx, logID, response, ms); err != nil {
		logger.Warn("Failed to log card response", logger.LogContext{Fields: map[string]any{"log_id": logID, "error": err.Error()}})
	}
}

// SendCardOTP sends an OTP SMS and returns the referenceNumber to reuse for validate/list.
func (s *CardService) SendCardOTP(ctx context.Context, environment, providerName string, request CardOTPSendRequest) (*CardOTPSendResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	cs, err := getCardStorageProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	logID, start := s.startLog(ctx, tenantID, providerName, "/cards/otp/send", request, request.ClientIP, "")
	request.LogID = logID
	resp, err := cs.SendCardOTP(ctx, request)
	s.finishLog(ctx, logID, start, resp, err)
	return resp, err
}

// ValidateCardOTP validates an OTP for a referenceNumber.
func (s *CardService) ValidateCardOTP(ctx context.Context, environment, providerName string, request CardOTPValidateRequest) (*CardOTPValidateResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	cs, err := getCardStorageProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	logID, start := s.startLog(ctx, tenantID, providerName, "/cards/otp/validate", request, request.ClientIP, "")
	request.LogID = logID
	resp, err := cs.ValidateCardOTP(ctx, request)
	s.finishLog(ctx, logID, start, resp, err)
	return resp, err
}

// RegisterCard registers a card with the provider and, on success, persists the saved-card row.
func (s *CardService) RegisterCard(ctx context.Context, environment, providerName string, request RegisterCardRequest) (*RegisterCardResponse, *SavedCard, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	cs, err := getCardStorageProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, nil, err
	}

	logID, start := s.startLog(ctx, tenantID, providerName, "/cards/register", request, request.ClientIP, "")
	request.LogID = logID
	resp, err := cs.RegisterCard(ctx, request)
	s.finishLog(ctx, logID, start, resp, err)
	if err != nil {
		return resp, nil, err
	}
	if resp == nil || !resp.Success {
		return resp, nil, errors.New("card registration failed")
	}

	providerID, err := s.providerConfig.GetProviderIDByName(providerName)
	if err != nil {
		return resp, nil, err
	}

	card := &SavedCard{
		TenantID:       tenantID,
		ProviderID:     providerID,
		Environment:    environment,
		MSISDN:         normalizeMSISDN(request.MSISDN),
		ProviderCardID: resp.ProviderCardID,
		MaskedCardNo:   resp.MaskedCardNo,
		CardBrand:      resp.CardBrand,
		CardType:       resp.CardType,
		Alias:          request.Alias,
	}
	id, err := s.repo.Create(ctx, card)
	if err != nil {
		return resp, nil, err
	}
	card.ID = id

	return resp, card, nil
}

// ListSavedCards returns the saved cards GoPay holds for a tenant + provider + environment + msisdn.
func (s *CardService) ListSavedCards(ctx context.Context, environment, providerName, msisdn string) ([]SavedCard, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	providerID, err := s.providerConfig.GetProviderIDByName(providerName)
	if err != nil {
		return nil, err
	}
	return s.repo.ListByMsisdn(ctx, tenantID, providerID, environment, normalizeMSISDN(msisdn))
}

// DeleteSavedCard deletes a saved card from the provider wallet and soft-deletes the GoPay row.
func (s *CardService) DeleteSavedCard(ctx context.Context, environment, providerName string, cardRowID int) error {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	// Tenant-scoped lookup is the IDOR/BOLA guard: a tenant can only delete its own card.
	card, err := s.repo.GetByID(ctx, tenantID, cardRowID)
	if err != nil {
		return err
	}
	cs, err := getCardStorageProvider(tenantID, providerName, environment)
	if err != nil {
		return err
	}

	logID, start := s.startLog(ctx, tenantID, providerName, "/cards/delete", map[string]any{"cardId": cardRowID, "providerCardId": card.ProviderCardID}, "", "")
	resp, err := cs.DeleteProviderCard(ctx, DeleteCardRequest{LogID: logID, MSISDN: card.MSISDN, ProviderCardID: card.ProviderCardID})
	s.finishLog(ctx, logID, start, resp, err)
	if err != nil {
		return err
	}

	return s.repo.SoftDeleteByID(ctx, tenantID, cardRowID)
}

// PaySavedCard charges a saved card by GoPay row id. use3D selects the 3D vs non-3D flow.
func (s *CardService) PaySavedCard(ctx context.Context, environment, providerName string, cardRowID int, request SavedCardPaymentRequest, use3D bool) (*PaymentResponse, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// Tenant-scoped lookup is the IDOR/BOLA guard: a tenant can only charge its own card.
	card, err := s.repo.GetByID(ctx, tenantID, cardRowID)
	if err != nil {
		return nil, err
	}
	if request.MSISDN != "" && normalizeMSISDN(request.MSISDN) != card.MSISDN {
		return nil, errors.New("msisdn does not match saved card")
	}
	cs, err := getCardStorageProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, err
	}

	request.TenantID = tenantID
	request.Environment = environment
	request.MSISDN = card.MSISDN
	request.ProviderCardID = card.ProviderCardID

	endpoint := "/cards/pay"
	if use3D {
		endpoint = "/cards/pay/3d"
	}
	logID, start := s.startLog(ctx, tenantID, providerName, endpoint, request, request.ClientIP, request.ClientUserAgent)
	request.LogID = logID

	var resp *PaymentResponse
	if use3D {
		resp, err = cs.Create3DPaymentWithSavedCard(ctx, request)
	} else {
		resp, err = cs.PayWithSavedCard(ctx, request)
	}
	if resp != nil {
		resp.SessionID = request.SessionID
	}
	s.finishLog(ctx, logID, start, resp, err)
	return resp, err
}

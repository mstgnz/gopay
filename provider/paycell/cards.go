package paycell

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// This file implements the optional provider.CardStorageProvider capability for Paycell:
// OTP send/validate, card registration, wallet card listing/deletion, and payment with a
// previously saved card id (3D and non-3D). It reuses paycell.go's request header, hash,
// transaction-id, HTTP client, logging and 3D infrastructure so the production payment flow
// in paycell.go stays untouched.
//
// IMPORTANT (verify against Paycell sandbox before go-live): the request/response field names
// of the card services below follow the Paycell "getCardToken" service group documented at
// apiportal.paycell.com.tr. The docs are screenshot-only; the integration test
// (paycell_integration_test.go) against the sandbox is the source of truth for these shapes.

// Ensure PaycellProvider satisfies the optional capability interface.
var _ provider.CardStorageProvider = (*PaycellProvider)(nil)

// PaycellExtraParameter is a single key/value entry of the extraParameters array used by the
// OTP / payment-method-list services (e.g. {key: "VALIDATION_TYPE", value: "PM_LIST"}).
type PaycellExtraParameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PaycellSendOTPRequest represents the sendOTP request.
type PaycellSendOTPRequest struct {
	RequestHeader   PaycellRequestHeader    `json:"requestHeader"`
	ExtraParameters []PaycellExtraParameter `json:"extraParameters,omitempty"`
	MerchantCode    string                  `json:"merchantCode"`
	Msisdn          string                  `json:"msisdn"`
	ReferenceNumber string                  `json:"referenceNumber"`
}

// PaycellSendOTPResponse represents the sendOTP response.
type PaycellSendOTPResponse struct {
	ResponseHeader      PaycellResponseHeader `json:"responseHeader"`
	Token               string                `json:"token"`
	ExpireDate          string                `json:"expireDate"`
	RemainingRetryCount string                `json:"remainingRetryCount"`
}

// PaycellValidateOTPRequest represents the validateOTP request.
type PaycellValidateOTPRequest struct {
	RequestHeader   PaycellRequestHeader    `json:"requestHeader"`
	ExtraParameters []PaycellExtraParameter `json:"extraParameters,omitempty"`
	MerchantCode    string                  `json:"merchantCode"`
	Msisdn          string                  `json:"msisdn"`
	ReferenceNumber string                  `json:"referenceNumber"`
	Otp             string                  `json:"otp"`
	Token           string                  `json:"token,omitempty"`
}

// PaycellValidateOTPResponse represents the validateOTP response.
type PaycellValidateOTPResponse struct {
	ResponseHeader PaycellResponseHeader `json:"responseHeader"`
}

// PaycellRegisterCardRequest represents the registerCard request.
type PaycellRegisterCardRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	MerchantCode    string               `json:"merchantCode"`
	Msisdn          string               `json:"msisdn"`
	CardToken       string               `json:"cardToken"`
	Alias           string               `json:"alias,omitempty"`
	EulaID          string               `json:"eulaId,omitempty"`
	ReferenceNumber string               `json:"referenceNumber,omitempty"`
}

// PaycellRegisterCardResponse represents the registerCard response.
type PaycellRegisterCardResponse struct {
	ResponseHeader PaycellResponseHeader `json:"responseHeader"`
	CardID         string                `json:"cardId"`
	MaskedCardNo   string                `json:"maskedCardNo"`
	CardBrand      string                `json:"cardBrand"`
	CardType       string                `json:"cardType"`
}

// PaycellGetPaymentMethodsRequest represents the getPaymentMethods (wallet card list) request.
type PaycellGetPaymentMethodsRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	MerchantCode    string               `json:"merchantCode"`
	Msisdn          string               `json:"msisdn"`
	ReferenceNumber string               `json:"referenceNumber,omitempty"`
}

// PaycellWalletCard represents a single saved card in the Paycell wallet.
type PaycellWalletCard struct {
	CardID       string `json:"cardId"`
	MaskedCardNo string `json:"maskedCardNo"`
	Alias        string `json:"alias"`
	CardBrand    string `json:"cardBrand"`
	CardType     string `json:"cardType"`
	IsDefault    bool   `json:"isDefault"`
}

// PaycellGetPaymentMethodsResponse represents the getPaymentMethods response.
type PaycellGetPaymentMethodsResponse struct {
	ResponseHeader PaycellResponseHeader `json:"responseHeader"`
	CardList       []PaycellWalletCard   `json:"cardList"`
}

// PaycellDeleteCardRequest represents the deleteCard request.
type PaycellDeleteCardRequest struct {
	RequestHeader PaycellRequestHeader `json:"requestHeader"`
	MerchantCode  string               `json:"merchantCode"`
	Msisdn        string               `json:"msisdn"`
	CardID        string               `json:"cardId"`
}

// PaycellDeleteCardResponse represents the deleteCard response.
type PaycellDeleteCardResponse struct {
	ResponseHeader PaycellResponseHeader `json:"responseHeader"`
}

// normalizeMsisdn strips the Turkish country code so the number is the 10-digit form Paycell expects.
func normalizeMsisdn(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+90")
	phone = strings.TrimPrefix(phone, "90")
	return phone
}

// cardRequestHeader builds the common request header for the tpay card services. Unlike
// getCardTokenSecure (payment management), these services authenticate via applicationName/
// applicationPwd in the header and do not carry a hashData field (same as provisionAll).
func (p *PaycellProvider) cardRequestHeader() PaycellRequestHeader {
	return PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: p.generateTransactionDateTime(),
		TransactionID:       p.generateTransactionID(),
	}
}

// SendCardOTP sends an OTP SMS to the customer's MSISDN and returns the referenceNumber that
// must be reused for ValidateCardOTP and ListProviderCards (PM_LIST flow).
func (p *PaycellProvider) SendCardOTP(ctx context.Context, request provider.CardOTPSendRequest) (*provider.CardOTPSendResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP

	msisdn := normalizeMsisdn(request.MSISDN)
	if len(msisdn) != 10 {
		return nil, errors.New("paycell: msisdn must be 10 digits")
	}

	referenceNumber := p.generateReferenceNumber()
	paycellReq := PaycellSendOTPRequest{
		RequestHeader:   p.cardRequestHeader(),
		ExtraParameters: []PaycellExtraParameter{{Key: "VALIDATION_TYPE", Value: "PM_LIST"}},
		MerchantCode:    p.merchantID,
		Msisdn:          msisdn,
		ReferenceNumber: referenceNumber,
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "sendOtpRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointSendOTP, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to send sendOTP request: %w", err)
	}

	var otpResp PaycellSendOTPResponse
	if err := p.httpClient.ParseJSONResponse(resp, &otpResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse sendOTP response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(otpResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "sendOtpResponse", respMap, p.logID)
	}

	success := otpResp.ResponseHeader.ResponseCode == responseCodeSuccess
	return &provider.CardOTPSendResponse{
		Success:          success,
		ReferenceNumber:  referenceNumber,
		Token:            otpResp.Token,
		Message:          otpResp.ResponseHeader.ResponseDescription,
		ErrorCode:        otpResp.ResponseHeader.ResponseCode,
		ProviderResponse: otpResp,
	}, nil
}

// ValidateCardOTP validates the OTP for the referenceNumber returned by SendCardOTP.
func (p *PaycellProvider) ValidateCardOTP(ctx context.Context, request provider.CardOTPValidateRequest) (*provider.CardOTPValidateResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP

	msisdn := normalizeMsisdn(request.MSISDN)
	if request.ReferenceNumber == "" {
		return nil, errors.New("paycell: referenceNumber is required")
	}
	if request.OTP == "" {
		return nil, errors.New("paycell: otp is required")
	}

	paycellReq := PaycellValidateOTPRequest{
		RequestHeader:   p.cardRequestHeader(),
		ExtraParameters: []PaycellExtraParameter{{Key: "VALIDATION_TYPE", Value: "PM_LIST"}},
		MerchantCode:    p.merchantID,
		Msisdn:          msisdn,
		ReferenceNumber: request.ReferenceNumber,
		Otp:             request.OTP,
		Token:           request.Token,
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "validateOtpRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointValidateOTP, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to send validateOTP request: %w", err)
	}

	var otpResp PaycellValidateOTPResponse
	if err := p.httpClient.ParseJSONResponse(resp, &otpResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse validateOTP response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(otpResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "validateOtpResponse", respMap, p.logID)
	}

	success := otpResp.ResponseHeader.ResponseCode == responseCodeSuccess
	return &provider.CardOTPValidateResponse{
		Success:          success,
		Message:          otpResp.ResponseHeader.ResponseDescription,
		ErrorCode:        otpResp.ResponseHeader.ResponseCode,
		ProviderResponse: otpResp,
	}, nil
}

// RegisterCard tokenizes the raw card via getCardTokenSecure, then registers the token in the
// Paycell wallet, returning the provider cardId plus masked metadata.
func (p *PaycellProvider) RegisterCard(ctx context.Context, request provider.RegisterCardRequest) (*provider.RegisterCardResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP

	msisdn := normalizeMsisdn(request.MSISDN)
	if len(msisdn) != 10 {
		return nil, errors.New("paycell: msisdn must be 10 digits")
	}
	if request.Card.CardNumber == "" || request.Card.ExpireMonth == "" || request.Card.ExpireYear == "" || request.Card.CVV == "" {
		return nil, errors.New("paycell: card number, expiry and cvv are required")
	}

	// Step 1: tokenize the card (raw card data goes to Paycell, never persisted by us).
	cardToken, err := p.getCardTokenSecure(ctx, provider.PaymentRequest{CardInfo: request.Card, LogID: request.LogID})
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to tokenize card: %w", err)
	}

	// Step 2: register the token in the wallet.
	paycellReq := PaycellRegisterCardRequest{
		RequestHeader:   p.cardRequestHeader(),
		MerchantCode:    p.merchantID,
		Msisdn:          msisdn,
		CardToken:       cardToken,
		Alias:           request.Alias,
		EulaID:          p.eulaID,
		ReferenceNumber: request.ReferenceNumber,
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "registerCardRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointRegisterCard, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to send registerCard request: %w", err)
	}

	var regResp PaycellRegisterCardResponse
	if err := p.httpClient.ParseJSONResponse(resp, &regResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse registerCard response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(regResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "registerCardResponse", respMap, p.logID)
	}

	if regResp.ResponseHeader.ResponseCode != responseCodeSuccess {
		return &provider.RegisterCardResponse{
			Success:          false,
			Message:          regResp.ResponseHeader.ResponseDescription,
			ErrorCode:        regResp.ResponseHeader.ResponseCode,
			ProviderResponse: regResp,
		}, fmt.Errorf("paycell: registerCard error: %s - %s", regResp.ResponseHeader.ResponseCode, regResp.ResponseHeader.ResponseDescription)
	}

	return &provider.RegisterCardResponse{
		Success:          true,
		ProviderCardID:   regResp.CardID,
		MaskedCardNo:     regResp.MaskedCardNo,
		CardBrand:        regResp.CardBrand,
		CardType:         regResp.CardType,
		Message:          regResp.ResponseHeader.ResponseDescription,
		ErrorCode:        regResp.ResponseHeader.ResponseCode,
		ProviderResponse: regResp,
	}, nil
}

// ListProviderCards lists the saved cards in the Paycell wallet for an MSISDN. The PM_LIST flow
// requires a referenceNumber that has been OTP-validated via SendCardOTP + ValidateCardOTP.
func (p *PaycellProvider) ListProviderCards(ctx context.Context, request provider.ListCardsRequest) (*provider.ListCardsResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP

	msisdn := normalizeMsisdn(request.MSISDN)
	if len(msisdn) != 10 {
		return nil, errors.New("paycell: msisdn must be 10 digits")
	}

	paycellReq := PaycellGetPaymentMethodsRequest{
		RequestHeader:   p.cardRequestHeader(),
		MerchantCode:    p.merchantID,
		Msisdn:          msisdn,
		ReferenceNumber: request.ReferenceNumber,
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "getPaymentMethodsRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointGetPaymentMethods, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to send getPaymentMethods request: %w", err)
	}

	var pmResp PaycellGetPaymentMethodsResponse
	if err := p.httpClient.ParseJSONResponse(resp, &pmResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse getPaymentMethods response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(pmResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "getPaymentMethodsResponse", respMap, p.logID)
	}

	cards := make([]provider.ProviderCard, 0, len(pmResp.CardList))
	for _, c := range pmResp.CardList {
		cards = append(cards, provider.ProviderCard{
			ProviderCardID: c.CardID,
			MaskedCardNo:   c.MaskedCardNo,
			CardBrand:      c.CardBrand,
			CardType:       c.CardType,
			Alias:          c.Alias,
		})
	}

	return &provider.ListCardsResponse{
		Success:          pmResp.ResponseHeader.ResponseCode == responseCodeSuccess,
		Cards:            cards,
		Message:          pmResp.ResponseHeader.ResponseDescription,
		ErrorCode:        pmResp.ResponseHeader.ResponseCode,
		ProviderResponse: pmResp,
	}, nil
}

// DeleteProviderCard removes a saved card from the Paycell wallet.
func (p *PaycellProvider) DeleteProviderCard(ctx context.Context, request provider.DeleteCardRequest) (*provider.DeleteCardResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP

	msisdn := normalizeMsisdn(request.MSISDN)
	if request.ProviderCardID == "" {
		return nil, errors.New("paycell: providerCardId is required")
	}

	paycellReq := PaycellDeleteCardRequest{
		RequestHeader: p.cardRequestHeader(),
		MerchantCode:  p.merchantID,
		Msisdn:        msisdn,
		CardID:        request.ProviderCardID,
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "deleteCardRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointDeleteCard, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to send deleteCard request: %w", err)
	}

	var delResp PaycellDeleteCardResponse
	if err := p.httpClient.ParseJSONResponse(resp, &delResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse deleteCard response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(delResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "deleteCardResponse", respMap, p.logID)
	}

	return &provider.DeleteCardResponse{
		Success:          delResp.ResponseHeader.ResponseCode == responseCodeSuccess,
		Message:          delResp.ResponseHeader.ResponseDescription,
		ErrorCode:        delResp.ResponseHeader.ResponseCode,
		ProviderResponse: delResp,
	}, nil
}

// PayWithSavedCard charges a saved card by provider cardId without 3D secure (the auto-top-up path).
func (p *PaycellProvider) PayWithSavedCard(ctx context.Context, request provider.SavedCardPaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP
	p.phoneNumber = request.MSISDN

	if request.ProviderCardID == "" {
		return nil, errors.New("paycell: providerCardId is required")
	}
	if request.Amount <= 0 {
		return nil, errors.New("paycell: amount must be greater than 0")
	}
	if request.Currency == "" {
		return nil, errors.New("paycell: currency is required")
	}

	return p.provisionAllWithCardId(ctx, request, "")
}

// Create3DPaymentWithSavedCard starts a 3D secure payment using a saved provider cardId. The
// completion is handled by the existing Complete3DPayment via the standard short-callback flow;
// the saved cardId is logged so that handler can finish with provisionAllWithCardId.
func (p *PaycellProvider) Create3DPaymentWithSavedCard(ctx context.Context, request provider.SavedCardPaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	p.clientIP = request.ClientIP
	p.phoneNumber = request.MSISDN

	if request.ProviderCardID == "" {
		return nil, errors.New("paycell: providerCardId is required")
	}
	if request.CallbackURL == "" {
		return nil, errors.New("paycell: callback URL is required for 3D payments")
	}

	threeDSession, err := p.get3dSessionWithCardId(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to get 3D session: %w", err)
	}

	// Persist the saved cardId on the same log row so Complete3DPayment can detect the
	// saved-card flow and finish with provisionAllWithCardId instead of provisionAll(cardToken).
	savedCardMap := map[string]any{"savedCardId": request.ProviderCardID, "msisdn": normalizeMsisdn(request.MSISDN)}
	_ = provider.AddProviderRequestToClientRequest("paycell", "savedCard", savedCardMap, p.logID)

	state := provider.CallbackState{
		TenantID:         request.TenantID,
		PaymentID:        threeDSession.ThreeDSessionId,
		OriginalCallback: request.CallbackURL,
		Amount:           request.Amount,
		Currency:         request.Currency,
		LogID:            p.logID,
		Provider:         "paycell",
		Environment:      request.Environment,
		Timestamp:        time.Now(),
		ClientIP:         request.ClientIP,
		Installment:      request.InstallmentCount,
		SessionID:        request.SessionID,
	}

	gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "paycell", state)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to create short callback URL: %w", err)
	}

	success := threeDSession.ResponseHeader.ResponseCode == responseCodeSuccess
	status := provider.StatusFailed
	if success {
		status = provider.StatusSuccessful
	}

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          success,
		Status:           status,
		PaymentID:        threeDSession.ThreeDSessionId,
		TransactionID:    threeDSession.ResponseHeader.TransactionID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		HTML:             p.generate3DSecureHTML(threeDSession.ThreeDSessionId, gopayCallbackURL),
		Message:          threeDSession.ResponseHeader.ResponseDescription,
		SystemTime:       &now,
		ProviderResponse: threeDSession,
	}, nil
}

// get3dSessionWithCardId requests a 3D session using a saved cardId. Unlike getThreeDSession it
// sends cardId and intentionally omits cardToken (Paycell requirement for saved-card payments).
func (p *PaycellProvider) get3dSessionWithCardId(ctx context.Context, request provider.SavedCardPaymentRequest) (*PaycellGetThreeDSessionResponse, error) {
	msisdn := normalizeMsisdn(request.MSISDN)
	installment := request.InstallmentCount
	if installment <= 0 {
		installment = 1
	}

	paycellReq := map[string]any{
		"requestHeader":    p.cardRequestHeader(),
		"amount":           fmt.Sprintf("%.0f", request.Amount*100), // kuruş
		"cardId":           request.ProviderCardID,
		"installmentCount": installment,
		"merchantCode":     p.merchantID,
		"msisdn":           msisdn,
		"target":           "MERCHANT",
		"transactionType":  "AUTH",
	}

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "getThreeDSessionRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointGetThreeDSession, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send getThreeDSession request: %w", err)
	}

	var threeDSessionResp PaycellGetThreeDSessionResponse
	if err := p.httpClient.ParseJSONResponse(resp, &threeDSessionResp); err != nil {
		return nil, fmt.Errorf("failed to parse getThreeDSession response: %w. Response body: %s", err, resp.RawBody)
	}

	if threeDSessionResp.ResponseHeader.ResponseCode != responseCodeSuccess {
		return nil, fmt.Errorf("getThreeDSession error: %s - %s", threeDSessionResp.ResponseHeader.ResponseCode, threeDSessionResp.ResponseHeader.ResponseDescription)
	}

	return &threeDSessionResp, nil
}

// buildCardIdProvisionRequest builds the provisionAll request body for a saved-card payment. It
// is a pure function (no I/O) so the critical invariant can be unit-tested: cardId is present and
// cardToken is intentionally absent (Paycell forbids cardToken when paying with a saved cardId).
func (p *PaycellProvider) buildCardIdProvisionRequest(request provider.SavedCardPaymentRequest, threeDSessionID string) map[string]any {
	installment := request.InstallmentCount
	if installment <= 0 {
		installment = 1
	}

	paycellReq := map[string]any{
		"requestHeader":     p.cardRequestHeader(),
		"amount":            strconv.FormatFloat(request.Amount*100, 'f', 0, 64), // kuruş
		"cardId":            request.ProviderCardID,
		"currency":          request.Currency,
		"installmentCount":  installment,
		"merchantCode":      p.merchantID,
		"msisdn":            normalizeMsisdn(request.MSISDN),
		"paymentType":       "SALE",
		"paymentMethodType": "CREDIT_CARD",
		"referenceNumber":   p.generateReferenceNumber(),
	}
	if threeDSessionID != "" {
		paycellReq["threeDSessionId"] = threeDSessionID
	}
	return paycellReq
}

// provisionAllWithCardId completes a provision using a saved cardId. It mirrors provisionAll but
// sends cardId and intentionally omits cardToken. threeDSessionID is empty for non-3D payments.
func (p *PaycellProvider) provisionAllWithCardId(ctx context.Context, request provider.SavedCardPaymentRequest, threeDSessionID string) (*provider.PaymentResponse, error) {
	paycellReq := p.buildCardIdProvisionRequest(request, threeDSessionID)

	if reqMap, err := provider.StructToMap(paycellReq); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "providerProvisionRequest", reqMap, p.logID)
	}

	httpReq := &provider.HTTPRequest{Method: "POST", Endpoint: endpointProvisionAll, Body: paycellReq}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send provisionAll request: %w", err)
	}

	var paycellResp PaycellProvisionResponse
	if err := p.httpClient.ParseJSONResponse(resp, &paycellResp); err != nil {
		return nil, fmt.Errorf("failed to parse provisionAll response: %w. Response body: %s", err, resp.RawBody)
	}

	if respMap, err := provider.StructToMap(paycellResp); err == nil {
		_ = provider.AddProviderRequestToClientRequest("paycell", "providerProvisionResponse", respMap, p.logID)
	}

	success := paycellResp.ResponseHeader.ResponseCode == responseCodeSuccess
	status := provider.StatusFailed
	if success {
		status = provider.StatusSuccessful
	}

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          success,
		Status:           status,
		PaymentID:        paycellResp.ResponseHeader.TransactionID,
		TransactionID:    paycellResp.ResponseHeader.TransactionID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		Message:          paycellResp.ResponseHeader.ResponseDescription,
		ErrorCode:        paycellResp.ResponseHeader.ResponseCode,
		SystemTime:       &now,
		ProviderResponse: paycellResp,
	}, nil
}

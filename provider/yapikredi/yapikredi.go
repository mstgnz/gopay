package yapikredi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/provider"
)

const (
	providerName = "yapikredi"

	sandboxXMLURL       = "https://setmpos.ykb.com/PosnetWebService/XML"
	sandboxThreeDURL    = "https://setmpos.ykb.com/3DSWebService/YKBPaymentService"
	productionXMLURL    = "https://posnet.yapikredi.com.tr/PosnetWebService/XML"
	productionThreeDURL = "https://posnet.yapikredi.com.tr/3DSWebService/YKBPaymentService"

	// The bank may take up to 45 s and recommends a 60 s client timeout.
	httpTimeout = 60 * time.Second

	posnetCurrencyTL = "TL"

	approvedYes     = "1"
	approvedAlready = "2" // the same order was approved before
	mdStatusOK      = "1"
	txnStatusOK     = "1"
	txnStatusFailed = "0" // assumed: the bank documents only 1

	// respCodeNoRecord is the agreement answer for an orderID the bank has never seen.
	respCodeNoRecord = "E219"

	// idPrefix + yyMMddHHmmss + random alphanumerics: 20 chars for a 3D XID, 24 for an XML orderID.
	idPrefix           = "GP"
	xidRandomLen       = 6
	orderIDRandomLen   = 10
	xidLength          = 20
	orderIDLength      = 24
	threeDOrderPrefix  = "TDS_"
	idAlphabet         = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxReturnURLLength = 255

	errCodeSessionExpired = "3D_SESSION_EXPIRED"
	errCodeUnverified     = "VERIFICATION_UNAVAILABLE"
	errCodeResolveFailed  = "3D_RESOLVE_UNAVAILABLE"
	errCodeAuthFailed     = "3D_AUTH_FAILED"
	errCodeMACMismatch    = "MAC_MISMATCH"
	errCodeDataMismatch   = "3D_DATA_MISMATCH"
	errCodeMissing3DData  = "MISSING_3D_DATA"
	errCodeDuplicateOrder = "DUPLICATE_ORDER"
	errCodeNotCancellable = "NOT_CANCELLABLE"
	errCodeNotRefundable  = "NOT_REFUNDABLE"
)

var paymentIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// YapiKrediProvider implements provider.PaymentProvider for Yapı Kredi Posnet (XML service + 3D).
type YapiKrediProvider struct {
	merchantID   string
	terminalID   string
	posnetID     string
	encKey       string
	xmlURL       string
	threeDURL    string
	gopayBaseURL string
	isProduction bool
	httpClient   *provider.ProviderHTTPClient
	logID        int64

	// Seams over the database, replaced in tests.
	storeCallback   func(ctx context.Context, gopayBaseURL string, state provider.CallbackState) (string, error)
	callbackPhase   func(ctx context.Context, paymentID string) (provider.CallbackPhase, error)
	startOutcome    func(ctx context.Context, paymentID string) (provider.LoggedOutcome, error)
	completeOutcome func(ctx context.Context, paymentID string) (provider.LoggedOutcome, error)
	recordLog       func(providerName, key string, data map[string]any, logID int64) error
	now             func() time.Time
}

// NewProvider creates a Yapı Kredi provider wired to the database helpers.
func NewProvider() provider.PaymentProvider {
	return &YapiKrediProvider{
		storeCallback: func(ctx context.Context, gopayBaseURL string, state provider.CallbackState) (string, error) {
			return provider.CreateShortCallbackURL(ctx, gopayBaseURL, providerName, state)
		},
		callbackPhase: func(ctx context.Context, paymentID string) (provider.CallbackPhase, error) {
			return provider.CallbackStateOf(ctx, providerName, paymentID)
		},
		startOutcome: func(ctx context.Context, paymentID string) (provider.LoggedOutcome, error) {
			return provider.GetThreeDStartOutcome(ctx, providerName, paymentID)
		},
		completeOutcome: func(ctx context.Context, paymentID string) (provider.LoggedOutcome, error) {
			return provider.GetThreeDCompleteOutcome(ctx, providerName, paymentID)
		},
		recordLog: provider.AddProviderRequestToClientRequest,
		now:       time.Now,
	}
}

// Clone returns a per-request copy. See provider.PaymentProvider.Clone.
func (p *YapiKrediProvider) Clone() provider.PaymentProvider {
	c := *p
	return &c
}

// GetRequiredConfig returns the configuration fields required for Yapı Kredi.
func (p *YapiKrediProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{Key: "merchantId", Required: true, Type: "string", Description: "Posnet merchant number (MID, 10 digits)", Example: "6700000001", Pattern: `^\d{10}$`},
		{Key: "terminalId", Required: true, Type: "string", Description: "Posnet terminal number (TID, 8 characters)", Example: "67000001", Pattern: `^[0-9A-Za-z]{8}$`},
		{Key: "posnetId", Required: true, Type: "string", Description: "Posnet ID used by the 3D Secure encryption (up to 16 digits)", Example: "9644", Pattern: `^\d{1,16}$`},
		{Key: "encKey", Required: true, Type: "string", Description: "ENCKEY for the 3D MAC (fixed in the test system; 15-35 characters live)", Example: "10,10,10,10,10,10,10,10", MinLength: 15, MaxLength: 35},
		{Key: "environment", Required: true, Type: "string", Description: "Environment setting (sandbox or production)", Example: "sandbox", Pattern: "^(sandbox|production)$"},
	}
}

// ValidateConfig validates the provided configuration against Yapı Kredi requirements.
func (p *YapiKrediProvider) ValidateConfig(conf map[string]string) error {
	return provider.ValidateConfigFields(providerName, conf, p.GetRequiredConfig(conf["environment"]))
}

// Initialize sets up the provider with the tenant's credentials.
func (p *YapiKrediProvider) Initialize(conf map[string]string) error {
	p.merchantID = strings.TrimSpace(conf["merchantId"])
	p.terminalID = strings.TrimSpace(conf["terminalId"])
	p.posnetID = strings.TrimSpace(conf["posnetId"])
	p.encKey = conf["encKey"]
	if p.merchantID == "" || p.terminalID == "" || p.posnetID == "" || p.encKey == "" {
		return errors.New("yapikredi: merchantId, terminalId, posnetId and encKey are required")
	}

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.xmlURL, p.threeDURL = productionXMLURL, productionThreeDURL
	} else {
		p.xmlURL, p.threeDURL = sandboxXMLURL, sandboxThreeDURL
	}
	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	// Certificate verification stays on: CreateHTTPClientConfig skips it for Nkolay's certificate.
	p.httpClient = provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
		BaseURL: p.xmlURL,
		Timeout: httpTimeout,
		DefaultHeaders: map[string]string{
			"Accept":     "text/xml, application/xml",
			"User-Agent": "GoPay/1.0",
		},
	})
	return nil
}

// GetInstallmentCount: Posnet has no installment inquiry; installments are sent with the payment.
func (p *YapiKrediProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission: Posnet has no commission inquiry.
func (p *YapiKrediProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// ValidateWebhook: Posnet sends no webhooks; the 3D result arrives on the callback.
func (p *YapiKrediProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	return false, nil, errors.New("yapikredi: Posnet sends no webhooks")
}

// CreatePayment makes a non-3D sale.
func (p *YapiKrediProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	in, err := p.paymentInput(request)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: invalid payment request: %w", err)
	}
	orderID, err := p.newID(orderIDRandomLen)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: %w", err)
	}
	p.record("saleRequest", map[string]any{"orderId": orderID, "amount": in.amount, "currency": in.currency, "installment": in.installment})

	resp, err := p.call(ctx, posnetRequest{
		TranDateRequired: "1",
		Sale: &saleData{
			Amount: in.amount, CCNo: in.ccno, CurrencyCode: in.currency, CVC: in.cvc,
			ExpDate: in.expDate, OrderID: orderID, Installment: in.installment,
		},
	})

	out := &provider.PaymentResponse{
		PaymentID:  orderID,
		OrderID:    orderID,
		Amount:     request.Amount,
		Currency:   request.Currency,
		SystemTime: timePtr(p.now()),
	}
	if err != nil {
		// The sale may have reached the bank; the status query settles it.
		logger.Warn("Yapı Kredi sale got no answer", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"order_id": orderID, "error": err.Error()},
		})
		out.Status, out.ErrorCode = provider.StatusPending, errCodeUnverified
		out.Message = "Sale result unknown, query the payment status"
		return out, nil
	}
	p.record("saleResponse", resultFields(resp))

	switch resp.Approved {
	case approvedYes:
		out.Success, out.Status, out.Message = true, provider.StatusSuccessful, "Payment successful"
		out.TransactionID = resp.HostLogKey
	case approvedAlready:
		// GoPay never resends an orderID, so this answer belongs to another sale.
		logger.Warn("Yapı Kredi reported a fresh orderID as already used", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"order_id": orderID, "resp_code": resp.RespCode},
		})
		out.Status, out.ErrorCode, out.Message = provider.StatusPending, errCodeDuplicateOrder, resp.RespText
	default:
		out.Status, out.ErrorCode, out.Message = provider.StatusFailed, resp.RespCode, failureText(resp.RespText, "Payment failed")
	}
	out.ProviderResponse = resultFields(resp)
	return out, nil
}

// GetPaymentStatus reads the payment from the bank's agreement service.
func (p *YapiKrediProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	view, err := p.agreement(ctx, request.PaymentID)
	if err != nil {
		return nil, err
	}

	out := &provider.PaymentResponse{
		PaymentID:  request.PaymentID,
		SystemTime: timePtr(p.now()),
	}
	if view.found {
		out.Status = view.status
		out.TransactionID = view.saleHostLogKey
		out.Success = view.status == provider.StatusSuccessful
		out.Message = "Payment status: " + string(view.status)
		out.ProviderResponse = map[string]any{"transactions": view.transactionsForResponse()}
		return out, nil
	}

	if !view.is3D {
		return nil, fmt.Errorf("yapikredi: the bank has no record of %s", request.PaymentID)
	}
	return p.statusWithoutRecord(ctx, request.PaymentID, out)
}

// statusWithoutRecord answers a 3D payment the bank has no record of, from GoPay's callback phase.
func (p *YapiKrediProvider) statusWithoutRecord(ctx context.Context, paymentID string, out *provider.PaymentResponse) (*provider.PaymentResponse, error) {
	phase, err := p.callbackPhase(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: callback phase of %s: %w", paymentID, err)
	}

	switch phase {
	case provider.CallbackOpen:
		out.Status, out.Message = provider.StatusPending, "3D authentication not finished"
		return out, nil

	case provider.CallbackAccepted:
		// The callback ran; only a logged failure is a verdict, anything else stays unknown.
		outcome, err := p.completeOutcome(ctx, paymentID)
		if err != nil && !errors.Is(err, provider.ErrLogValueNotFound) {
			return nil, fmt.Errorf("yapikredi: 3D callback outcome of %s: %w", paymentID, err)
		}
		if err == nil && outcome.Status == string(provider.StatusFailed) {
			out.Status, out.ErrorCode = provider.StatusFailed, outcome.ErrorCode
			out.Message = failureText(outcome.Message, "3D payment failed")
			return out, nil
		}
		return nil, fmt.Errorf("yapikredi: 3D callback of %s was accepted but the bank has no record, outcome unknown", paymentID)

	default: // provider.CallbackClosed
		// The callbacks rows are purged after retention, so a logged callback still outranks "closed".
		done, err := p.completeOutcome(ctx, paymentID)
		switch {
		case err == nil && done.Status == string(provider.StatusFailed):
			out.Status, out.ErrorCode = provider.StatusFailed, done.ErrorCode
			out.Message = failureText(done.Message, "3D payment failed")
			return out, nil
		case err == nil:
			return nil, fmt.Errorf("yapikredi: 3D callback of %s was logged but the bank has no record, outcome unknown", paymentID)
		case !errors.Is(err, provider.ErrLogValueNotFound):
			return nil, fmt.Errorf("yapikredi: 3D callback outcome of %s: %w", paymentID, err)
		}

		out.Status = provider.StatusFailed
		outcome, err := p.startOutcome(ctx, paymentID)
		switch {
		case err == nil && outcome.Status == string(provider.StatusFailed):
			out.ErrorCode, out.Message = outcome.ErrorCode, failureText(outcome.Message, "3D payment start was rejected")
		case err == nil || errors.Is(err, provider.ErrLogValueNotFound):
			out.ErrorCode, out.Message = errCodeSessionExpired, "3D session expired"
		default:
			return nil, fmt.Errorf("yapikredi: 3D start outcome of %s: %w", paymentID, err)
		}
		return out, nil
	}
}

// CancelPayment reverses a sale on the day it was made.
func (p *YapiKrediProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	view, err := p.agreement(ctx, request.PaymentID)
	if err != nil {
		return nil, err
	}
	if !view.found {
		return nil, fmt.Errorf("yapikredi: the bank has no record of %s", request.PaymentID)
	}

	out := &provider.PaymentResponse{PaymentID: request.PaymentID, Status: view.status, SystemTime: timePtr(p.now())}
	switch view.status {
	case provider.StatusCancelled:
		out.Success, out.Message = true, "Payment already cancelled"
		return out, nil
	case provider.StatusSuccessful:
	default:
		out.ErrorCode, out.Message = errCodeNotCancellable, "Payment is "+string(view.status)+", it cannot be cancelled"
		return out, nil
	}

	resp, err := p.call(ctx, posnetRequest{Reverse: &reverseData{Transaction: "sale", HostLogKey: view.saleHostLogKey}})
	if err != nil {
		return nil, fmt.Errorf("yapikredi: cancel of %s got no answer, query the payment status: %w", request.PaymentID, err)
	}
	p.record("cancelResponse", resultFields(resp))
	out.ProviderResponse = resultFields(resp)

	if resp.Approved == approvedYes {
		out.Success, out.Status, out.Message = true, provider.StatusCancelled, "Payment cancelled"
		out.TransactionID = resp.HostLogKey
		return out, nil
	}
	// 0211: the day is closed, a refund is needed. The sale still stands.
	out.ErrorCode, out.Message = resp.RespCode, failureText(resp.RespText, "Cancel failed")
	return out, nil
}

// RefundPayment returns all or part of a sale; partial and repeated refunds are allowed.
func (p *YapiKrediProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	p.logID = request.LogID
	if request.Currency != "" {
		if _, err := posnetCurrency(request.Currency); err != nil {
			return nil, fmt.Errorf("yapikredi: %w", err)
		}
	}
	amount, err := kurus(request.RefundAmount)
	if err != nil {
		return nil, fmt.Errorf("yapikredi: refund %w", err)
	}

	view, err := p.agreement(ctx, request.PaymentID)
	if err != nil {
		return nil, err
	}
	if !view.found {
		return nil, fmt.Errorf("yapikredi: the bank has no record of %s", request.PaymentID)
	}

	out := &provider.RefundResponse{PaymentID: request.PaymentID, RefundAmount: request.RefundAmount, SystemTime: timePtr(p.now())}
	if view.status != provider.StatusSuccessful && view.status != provider.StatusRefunded {
		out.Status, out.ErrorCode = "failed", errCodeNotRefundable
		out.Message = "Payment is " + string(view.status) + ", it cannot be refunded"
		return out, nil
	}

	resp, err := p.call(ctx, posnetRequest{
		TranDateRequired: "1",
		Return:           &returnData{Amount: amount, CurrencyCode: posnetCurrencyTL, HostLogKey: view.saleHostLogKey},
	})
	if err != nil {
		return nil, fmt.Errorf("yapikredi: refund of %s got no answer, query the payment status: %w", request.PaymentID, err)
	}
	p.record("refundResponse", resultFields(resp))
	out.RawResponse = resultFields(resp)

	if resp.Approved == approvedYes {
		out.Success, out.Status, out.Message, out.RefundID = true, "success", "Refund successful", resp.HostLogKey
		return out, nil
	}
	// 0411: the sale is not settled yet, a same-day cancel is needed.
	out.Status, out.ErrorCode, out.Message = "failed", resp.RespCode, failureText(resp.RespText, "Refund failed")
	return out, nil
}

// paymentFields are the request values every payment sends, already in Posnet format.
type paymentFields struct {
	amount, currency, installment  string
	cardHolder, ccno, expDate, cvc string
}

// paymentInput validates a payment request and converts it to Posnet's formats.
func (p *YapiKrediProvider) paymentInput(request provider.PaymentRequest) (paymentFields, error) {
	var in paymentFields
	if request.TenantID == 0 {
		return in, errors.New("tenantID is required")
	}
	var err error
	if in.amount, err = kurus(request.Amount); err != nil {
		return in, err
	}
	if in.currency, err = posnetCurrency(request.Currency); err != nil {
		return in, err
	}
	if in.installment, err = installmentCode(request.InstallmentCount); err != nil {
		return in, err
	}
	if in.ccno, in.expDate, in.cvc, err = cardFields(request.CardInfo); err != nil {
		return in, err
	}
	in.cardHolder = strings.TrimSpace(request.CardInfo.CardHolderName)
	return in, nil
}

// call sends one request to the XML service. The request text holds card data and is never logged.
func (p *YapiKrediProvider) call(ctx context.Context, req posnetRequest) (*posnetResponse, error) {
	req.MID, req.TID = p.merchantID, p.terminalID
	payload, err := encodeRequest(req)
	if err != nil {
		return nil, err
	}
	correlationID, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	// SendRaw, not SendForm: SendForm drops the charset the bank asks for in the Content-Type.
	httpResp, err := p.httpClient.SendRaw(ctx, &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: p.xmlURL,
		Body:     url.Values{"xmldata": {payload}}.Encode(),
		Headers: map[string]string{
			"Content-Type":     "application/x-www-form-urlencoded; charset=utf-8",
			"X-MERCHANT-ID":    p.merchantID,
			"X-TERMINAL-ID":    p.terminalID,
			"X-POSNET-ID":      p.posnetID,
			"X-CORRELATION-ID": correlationID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("posnet request failed: %w", err)
	}
	return decodeResponse(httpResp.Body)
}

// agreementView is one payment as the agreement service reports it.
type agreementView struct {
	orderID        string
	is3D           bool
	found          bool
	status         provider.PaymentStatus
	saleHostLogKey string
	txns           []agreementTxn
}

// agreement asks the bank for every transaction of a payment id.
func (p *YapiKrediProvider) agreement(ctx context.Context, paymentID string) (*agreementView, error) {
	orderID, is3D, err := agreementOrderID(paymentID)
	if err != nil {
		return nil, err
	}

	resp, err := p.call(ctx, posnetRequest{Agreement: &agreementData{OrderID: orderID}})
	if err != nil {
		return nil, fmt.Errorf("yapikredi: status query of %s failed: %w", paymentID, err)
	}

	view := &agreementView{orderID: orderID, is3D: is3D, txns: resp.Txns}
	if resp.Approved != approvedYes {
		if strings.EqualFold(resp.RespCode, respCodeNoRecord) {
			return view, nil
		}
		return nil, fmt.Errorf("yapikredi: status query of %s answered %s: %s", paymentID, resp.RespCode, resp.RespText)
	}
	if len(resp.Txns) == 0 {
		return view, nil
	}

	view.found = true
	view.status, view.saleHostLogKey = statusFromTransactions(paymentID, resp.Txns)
	return view, nil
}

// agreementOrderID maps a GoPay payment id to the agreement orderID: a 20-char 3D XID is known to
// the bank as TDS_ + XID, a 24-char orderID as itself.
func agreementOrderID(paymentID string) (string, bool, error) {
	if !paymentIDPattern.MatchString(paymentID) {
		return "", false, fmt.Errorf("yapikredi: invalid payment id %q", paymentID)
	}
	switch len(paymentID) {
	case xidLength:
		return threeDOrderPrefix + paymentID, true, nil
	case orderIDLength:
		return paymentID, false, nil
	default:
		return "", false, fmt.Errorf("yapikredi: invalid payment id %q", paymentID)
	}
}

// statusFromTransactions derives one status from the agreement rows. A row whose state GoPay does
// not know makes the answer pending: an unread reversal must never read as paid.
func statusFromTransactions(paymentID string, txns []agreementTxn) (provider.PaymentStatus, string) {
	var paid, saleFailed bool
	var saleKey string
	var reversals, returns, returnReversals int
	var unknown []string

	for _, t := range txns {
		// Only the two values GoPay can read are verdicts; an empty or other status is unknown.
		st := t.status()
		if st != txnStatusOK && st != txnStatusFailed {
			unknown = append(unknown, t.State+" txnStatus="+st)
			continue
		}
		ok := st == txnStatusOK
		switch strings.ToLower(t.State) {
		case "sale", "capture":
			if ok {
				paid = true
				if saleKey == "" {
					saleKey = t.HostLogKey
				}
			} else {
				saleFailed = true
			}
		case "sale_reverse", "capture_reverse":
			if ok {
				reversals++
			}
		case "return":
			if ok {
				returns++
			}
		case "return_reverse":
			if ok {
				returnReversals++
			}
		default:
			unknown = append(unknown, t.State)
		}
	}

	switch {
	case len(unknown) > 0:
		logger.Warn("Yapı Kredi agreement returned an unknown transaction state or status", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_id": paymentID, "states": unknown},
		})
		return provider.StatusPending, saleKey
	case paid && reversals > 0:
		return provider.StatusCancelled, saleKey
	case paid && returns > returnReversals:
		return provider.StatusRefunded, saleKey
	case paid:
		return provider.StatusSuccessful, saleKey
	case saleFailed:
		return provider.StatusFailed, saleKey
	default:
		return provider.StatusPending, saleKey
	}
}

// transactionsForResponse returns the agreement rows without card data.
func (v *agreementView) transactionsForResponse() []map[string]string {
	out := make([]map[string]string, 0, len(v.txns))
	for _, t := range v.txns {
		out = append(out, map[string]string{
			"orderId":    t.OrderID,
			"state":      t.State,
			"txnStatus":  t.status(),
			"hostLogKey": t.HostLogKey,
			"authCode":   t.AuthCode,
			"amount":     t.Amount,
			"tranDate":   t.TranDate,
		})
	}
	return out
}

// record adds non-card fields to the request column of the current log row.
func (p *YapiKrediProvider) record(key string, data map[string]any) {
	if p.logID <= 0 || p.recordLog == nil {
		return
	}
	if err := p.recordLog(providerName, key, data, p.logID); err != nil {
		logger.Warn("Failed to record Yapı Kredi exchange on the log row", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"log_id": p.logID, "key": key, "error": err.Error()},
		})
	}
}

// resultFields are the response values safe to log and return; packets and MACs are left out.
func resultFields(r *posnetResponse) map[string]any {
	return map[string]any{
		"approved":   r.Approved,
		"respCode":   r.RespCode,
		"respText":   r.RespText,
		"hostLogKey": r.HostLogKey,
		"authCode":   r.AuthCode,
	}
}

// newID returns idPrefix + yyMMddHHmmss + randomLen alphanumerics.
func (p *YapiKrediProvider) newID(randomLen int) (string, error) {
	b := make([]byte, randomLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate payment id: %w", err)
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return idPrefix + p.now().UTC().Format("060102150405") + string(b), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate correlation id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// kurus converts an amount to Posnet's integer kuruş string.
func kurus(amount float64) (string, error) {
	v := math.Round(amount * 100)
	if math.IsNaN(v) || v <= 0 || v > math.MaxInt64 {
		return "", errors.New("amount must be greater than 0")
	}
	return strconv.FormatInt(int64(v), 10), nil
}

// posnetCurrency maps GoPay's currency to Posnet's code. Only Turkish lira is enabled.
func posnetCurrency(currency string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "TRY", "TL":
		return posnetCurrencyTL, nil
	default:
		return "", fmt.Errorf("currency %q is not supported, only TRY", currency)
	}
}

// installmentCode is Posnet's two-digit installment; "00" is a single payment.
func installmentCode(count int) (string, error) {
	switch {
	case count < 0 || count > 99:
		return "", fmt.Errorf("invalid installment count %d", count)
	case count <= 1:
		return "00", nil
	default:
		return fmt.Sprintf("%02d", count), nil
	}
}

// cardFields validates the card and returns ccno, expDate (YYMM) and cvc.
func cardFields(card provider.CardInfo) (string, string, string, error) {
	ccno := strings.NewReplacer(" ", "", "-", "").Replace(card.CardNumber)
	if !allDigits(ccno) || len(ccno) < 12 || len(ccno) > 19 {
		return "", "", "", errors.New("card number is invalid")
	}
	cvc := strings.TrimSpace(card.CVV)
	if !allDigits(cvc) || len(cvc) < 3 || len(cvc) > 4 {
		return "", "", "", errors.New("CVV is invalid")
	}
	month, err := strconv.Atoi(strings.TrimSpace(card.ExpireMonth))
	if err != nil || month < 1 || month > 12 {
		return "", "", "", errors.New("expiry month is invalid")
	}
	year := strings.TrimSpace(card.ExpireYear)
	if !allDigits(year) || (len(year) != 2 && len(year) != 4) {
		return "", "", "", errors.New("expiry year is invalid")
	}
	return ccno, year[len(year)-2:] + fmt.Sprintf("%02d", month), cvc, nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func failureText(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func timePtr(t time.Time) *time.Time {
	return &t
}

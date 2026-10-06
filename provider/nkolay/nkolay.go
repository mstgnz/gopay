package nkolay

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	// Real Nkolay API URLs from postman collection
	apiSandboxURL    = "https://paynkolaytest.nkolayislem.com.tr"
	apiProductionURL = "https://paynkolay.nkolayislem.com.tr"

	// Real API Endpoints from postman collection
	endpointPayment             = "/Vpos/v1/Payment"
	endpointPaymentInstallments = "/Vpos/Payment/GetMerchandInformation"
	endpointCancelRefund        = "/Vpos/v1/CancelRefundPayment"
	endpointPaymentList         = "/Vpos/Payment/PaymentList"

	// Test credentials provided
	testSx        = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw=="
	testSxList    = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw==|3hJpHVF2cqvcCZ4q6F7rcA=="
	testSxCancel  = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw==|yDUZaCk6rsoHZJWI3d471A/+TJA7C81X"
	testSecretKey = "_YckdxUbv4vrnMUZ6VQsr"

	// Labels GoPay appends to its own successUrl/failUrl
	statusSuccess = "SUCCESS"
	statusFailed  = "FAILED"

	// Default Values
	defaultCurrency = "TRY"

	// responseCodeOK is Nkolay's RESPONSE_CODE for a call that worked; for PaymentList the
	// payment's own verdict is in LIST[].STATUS.
	responseCodeOK = "2"

	// PaymentList LIST[].STATUS values. NEW is a payment that was started but never completed.
	listStatusSuccess = "SUCCESS"
	listStatusError   = "ERROR"
	listStatusNew     = "NEW"

	// clientRefPrefix starts every clientRefCode processPayment generates; the rest is UnixNano.
	clientRefPrefix = "gopay_"

	// paymentListMaxDays is the widest PaymentList window Nkolay serves (one month).
	paymentListMaxDays = 30
)

// nkolayLocation is the zone of PaymentList's DD.MM.YYYY dates. Turkey has been UTC+3 with no DST
// since 2016; a fixed zone avoids depending on tzdata in the container.
var nkolayLocation = time.FixedZone("TRT", 3*60*60)

// callbackHashFields is the field order of the hashDataV2 Nkolay posts to successUrl/failUrl,
// per paynkolay.com.tr/entegrasyon/05-hash-response.php; the merchant secret key comes last.
var callbackHashFields = []string{
	"MERCHANT_NO", "REFERENCE_CODE", "AUTH_CODE", "RESPONSE_CODE", "USE_3D",
	"RND", "INSTALLMENT", "AUTHORIZATION_AMOUNT", "CURRENCY_CODE",
}

// errPaymentNotListed means PaymentList holds no sales row for the clientRefCode.
var errPaymentNotListed = errors.New("nkolay: payment not found in payment list")

// NkolayProvider implements the provider.PaymentProvider interface for Nkolay
type NkolayProvider struct {
	sx           string // Test token provided by Nkolay
	sxList       string // Token for listing operations
	sxCancel     string // Token for cancel/refund operations
	secretKey    string // Merchant secret key
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	httpClient   *provider.ProviderHTTPClient
	logID        int64

	// clientRefLookup maps a Nkolay REFERENCE_CODE to the clientRefCode GoPay sent with it.
	clientRefLookup func(referenceCode string) (string, error)
}

// NewProvider creates a new Nkolay payment provider
func NewProvider() provider.PaymentProvider {
	return &NkolayProvider{clientRefLookup: lookupClientRefCode}
}

// lookupClientRefCode reads the clientRefCode from the /payment/3d log row whose payment_id is the
// REFERENCE_CODE. Consumers only ever see the REFERENCE_CODE, but PaymentList filters by clientRefCode.
func lookupClientRefCode(referenceCode string) (string, error) {
	return provider.GetProviderNestedRequestValueFromLog("nkolay", referenceCode, "providerRequest", "clientRefCode")
}

// Clone returns a per-request copy. See provider.PaymentProvider.Clone.
func (p *NkolayProvider) Clone() provider.PaymentProvider {
	c := *p
	return &c
}

// GetRequiredConfig returns the configuration fields required for Nkolay
func (p *NkolayProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "sx",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for payment operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxList",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for listing operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|3hJpHVF2cqvcCZ4q6F7rcA==",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxCancel",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for cancel/refund operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|yDUZaCk6rsoHZJWI3d471A/+TJA7C81X",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Nkolay Secret Key (optional, uses test value if not provided)",
			Example:     "_YckdxUbv4vrnMUZ6VQsr",
			MinLength:   5,
			MaxLength:   100,
		},
		{
			Key:         "environment",
			Required:    true,
			Type:        "string",
			Description: "Environment setting (sandbox or production)",
			Example:     "sandbox",
			Pattern:     "^(sandbox|production)$",
		},
	}
}

// ValidateConfig validates the provided configuration against Nkolay requirements
func (p *NkolayProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("nkolay", config, requiredFields)
}

// Initialize sets up the Nkolay payment provider with authentication credentials
func (p *NkolayProvider) Initialize(conf map[string]string) error {
	// For real API, use provided credentials. For testing, use test values
	if sx := conf["sx"]; sx != "" {
		p.sx = sx
	} else {
		p.sx = testSx // Use test sx if not provided
	}

	if sxList := conf["sxList"]; sxList != "" {
		p.sxList = sxList
	} else {
		p.sxList = testSxList
	}

	if sxCancel := conf["sxCancel"]; sxCancel != "" {
		p.sxCancel = sxCancel
	} else {
		p.sxCancel = testSxCancel
	}

	if secretKey := conf["secretKey"]; secretKey != "" {
		p.secretKey = secretKey
	} else {
		p.secretKey = testSecretKey
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
	} else {
		p.baseURL = apiSandboxURL
	}

	p.httpClient = provider.NewProviderHTTPClient(provider.CreateHTTPClientConfig(p.baseURL, p.isProduction))

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *NkolayProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	formData := map[string]string{
		"sx":         p.sx,
		"amount":     fmt.Sprintf("%.2f", request.Amount),
		"hashDatav2": "",
	}

	// Generate hash: sx+date+secretkey - // Base64(SHA512(sx + "|" + date + "|" + merchantSecretKey))
	input := fmt.Sprintf("%s|%s|%s", p.sx, time.Now().Format("02.01.2006"), p.secretKey)
	hash := sha512.Sum512([]byte(input))
	formData["hashDatav2"] = base64.StdEncoding.EncodeToString(hash[:])

	responseBody, err := p.doNkolayFormRequest(ctx, endpointPaymentInstallments, formData)
	if err != nil {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: failed to get installment count: %w", err)
	}

	// Parse response as map first
	var rawResponse map[string]any
	if err := json.Unmarshal(responseBody, &rawResponse); err != nil {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: failed to unmarshal installment count response: %w", err)
	}

	// Initialize response structure
	response := provider.InstallmentInquireResponse{
		Amount:       request.Amount,
		Message:      "Installment options retrieved successfully",
		Installments: make(map[string][]provider.InstallmentInfo),
	}

	// Extract commission list from response
	commissionList, ok := rawResponse["COMMISSION_LIST"].([]any)
	if !ok || len(commissionList) == 0 {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: no commission list found in response")
	}

	// Process each commission entry (bank/card type)
	for _, entry := range commissionList {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		// Get card type code (PARAF, AXESS, etc.)
		cardType, ok := entryMap["CODE"].(string)
		if !ok {
			continue
		}

		// Get installment data array
		dataArray, ok := entryMap["DATA"].([]any)
		if !ok {
			continue
		}

		// Process installment options for this card type
		var installmentInfos []provider.InstallmentInfo
		for _, dataEntry := range dataArray {
			dataMap, ok := dataEntry.(map[string]any)
			if !ok {
				continue
			}

			// Extract installment number
			installmentNum, ok := dataMap["INSTALLMENT"].(float64)
			if !ok {
				continue
			}

			// Extract merchant installment surcharge rate. Nkolay renamed this
			// field MERCHANT_COMMISSION -> MERCHANT_COMMISSION_RATE in the
			// GetMerchandInformation response; read the new name first, fall back
			// to the legacy name for backward compatibility during their rollout.
			commission, ok := dataMap["MERCHANT_COMMISSION_RATE"].(float64)
			if !ok {
				commission, ok = dataMap["MERCHANT_COMMISSION"].(float64)
			}
			if !ok {
				continue
			}

			installmentInfos = append(installmentInfos, provider.InstallmentInfo{
				Installment: int(installmentNum),
				Commission:  commission,
			})
		}

		// Add to response if we have valid data
		if len(installmentInfos) > 0 {
			response.Installments[cardType] = installmentInfos
		}
	}

	return response, nil
}

// GetCommission returns the commission for a payment
func (p *NkolayProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *NkolayProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("nkolay: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *NkolayProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("nkolay: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *NkolayProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID

	response := &provider.PaymentResponse{
		PaymentID:        callbackState.PaymentID,
		TransactionID:    callbackState.PaymentID,
		SystemTime:       timePtr(time.Now()),
		ProviderResponse: data,
		Amount:           callbackState.Amount,
		Currency:         callbackState.Currency,
		RedirectURL:      callbackState.OriginalCallback,
	}

	// The "status" query parameter is ours (successUrl/failUrl) and anyone holding the callback URL
	// can set it, so the result comes from Nkolay's signed fields or, failing that, from PaymentList.
	if ref := data["REFERENCE_CODE"]; ref != "" && ref == callbackState.PaymentID && p.validCallbackHash(data) {
		if data["RESPONSE_CODE"] == responseCodeOK {
			response.Status = provider.StatusSuccessful
		} else {
			response.Status = provider.StatusFailed
			response.ErrorCode = data["ERROR_CODE"]
		}
	} else {
		logger.Warn("Nkolay 3D callback not signed for this payment, verifying with PaymentList", logger.LogContext{
			Provider: "nkolay",
			Fields: map[string]any{
				"payment_id":     callbackState.PaymentID,
				"reference_code": data["REFERENCE_CODE"],
				"has_hash":       data["hashDataV2"] != "",
			},
		})
		response.Status = p.verify3DResultWithList(ctx, callbackState.PaymentID)
		if response.Status == provider.StatusPending {
			response.ErrorCode = "VERIFICATION_UNAVAILABLE"
		}
	}

	response.Success = response.Status == provider.StatusSuccessful
	switch response.Status {
	case provider.StatusSuccessful:
		response.Message = "3D payment completed successfully"
	case provider.StatusFailed:
		response.Message = "3D payment failed"
	case provider.StatusPending:
		response.Message = "3D payment result could not be verified"
	default:
		response.Message = "3D payment " + string(response.Status)
	}

	// Parse amount if available
	if amountStr := data["amount"]; amountStr != "" {
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			response.Amount = amount
		}
	}

	return response, nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *NkolayProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	status, items, err := p.paymentStatusFromList(ctx, request.PaymentID, false)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to get payment status: %w", err)
	}

	var transactionID string
	for _, it := range items {
		if strings.EqualFold(it.transactionType, "sales") {
			transactionID = it.referenceCode
			break
		}
	}

	return &provider.PaymentResponse{
		PaymentID:     request.PaymentID,
		TransactionID: transactionID,
		Success:       status == provider.StatusSuccessful,
		Status:        status,
		Message:       "Payment status: " + string(status),
		SystemTime:    timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"transactions": listItemsForResponse(items),
		},
	}, nil
}

// verify3DResultWithList settles an unsigned 3D callback from PaymentList. By the time the bank
// redirects back the 3D flow is over, so NEW (never completed) is a failure here. When PaymentList
// cannot answer the result stays pending: an unverified callback is never reported as paid.
func (p *NkolayProvider) verify3DResultWithList(ctx context.Context, paymentID string) provider.PaymentStatus {
	status, _, err := p.paymentStatusFromList(ctx, paymentID, true)
	if err != nil {
		logger.Warn("Nkolay 3D result could not be verified with PaymentList", logger.LogContext{
			Provider: "nkolay",
			Fields:   map[string]any{"payment_id": paymentID, "error": err.Error()},
		})
		return provider.StatusPending
	}
	return status
}

// paymentStatusFromList asks PaymentList for paymentID, which is the Nkolay REFERENCE_CODE consumers
// hold or, when the 3D start returned none, GoPay's own clientRefCode. It returns the matching rows.
// flowOver says the 3D flow has ended, so a NEW sale is a failure rather than in progress; it also
// holds once the callback URL has expired, after which the payment can no longer complete.
func (p *NkolayProvider) paymentStatusFromList(ctx context.Context, paymentID string, flowOver bool) (provider.PaymentStatus, []paymentListItem, error) {
	clientRefCode, referenceCode := paymentID, ""
	if !strings.HasPrefix(paymentID, clientRefPrefix) {
		referenceCode = paymentID
		ref, err := p.clientRefLookup(paymentID)
		if err != nil {
			return "", nil, fmt.Errorf("no clientRefCode recorded for %s: %w", paymentID, err)
		}
		clientRefCode = ref
	}

	now := time.Now()
	if created, ok := clientRefCreatedAt(clientRefCode); ok && now.Sub(created) > provider.CallbackStateTTL {
		flowOver = true
	}

	items, err := p.queryPaymentList(ctx, clientRefCode, now)
	if err != nil {
		return "", nil, err
	}
	return statusFromListItems(items, clientRefCode, referenceCode, flowOver)
}

// clientRefCreatedAt reads the UnixNano that processPayment puts after clientRefPrefix.
func clientRefCreatedAt(clientRefCode string) (time.Time, bool) {
	if !strings.HasPrefix(clientRefCode, clientRefPrefix) {
		return time.Time{}, false
	}
	nanos, err := strconv.ParseInt(strings.TrimPrefix(clientRefCode, clientRefPrefix), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanos), true
}

// queryPaymentList calls PaymentList ("İşlem Doğrulama Servisi") for one clientRefCode.
func (p *NkolayProvider) queryPaymentList(ctx context.Context, clientRefCode string, now time.Time) ([]paymentListItem, error) {
	start, end := paymentListWindow(clientRefCode, now)
	formData := map[string]string{
		"sx":            p.sxList,
		"startDate":     start.Format("02.01.2006"),
		"endDate":       end.Format("02.01.2006"),
		"clientRefCode": clientRefCode,
	}
	formData["hashDatav2"] = hashV2(formData["sx"], formData["startDate"], formData["endDate"], formData["clientRefCode"], p.secretKey)

	body, err := p.doNkolayFormRequest(ctx, endpointPaymentList, formData)
	if err != nil {
		return nil, err
	}
	return parsePaymentList(body)
}

// paymentListWindow covers the day the payment started (encoded in the clientRefCode) up to today,
// capped at Nkolay's one-month limit. One day of slack absorbs clock skew around midnight. A code
// that carries no timestamp gets the last month.
func paymentListWindow(clientRefCode string, now time.Time) (start, end time.Time) {
	end = now.In(nkolayLocation)
	start = end.AddDate(0, 0, -paymentListMaxDays)

	created, ok := clientRefCreatedAt(clientRefCode)
	if !ok || created.After(end) {
		return start, end
	}
	start = created.In(nkolayLocation).AddDate(0, 0, -1)
	if limit := start.AddDate(0, 0, paymentListMaxDays); end.After(limit) {
		end = limit
	}
	return start, end
}

// paymentListItem holds the LIST fields GoPay reads. Items are decoded through a map so a field
// whose JSON type differs from the documented one does not fail the whole answer.
type paymentListItem struct {
	referenceCode   string
	clientRefCode   string
	transactionType string
	status          string
	trxDate         string
}

// parsePaymentList reads a PaymentList answer. Nkolay wraps it as {"id":"","result":{...}}, and on
// a hash error the result is a JSON string instead of an object; the documentation shows it bare.
// A RESPONSE_CODE other than "2" is an error, including "Listelenecek kayıt bulunamadı.", so a
// missing payment is never reported as a status.
func parsePaymentList(body []byte) ([]paymentListItem, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unreadable payment list response: %w", err)
	}
	switch result := raw["result"].(type) {
	case map[string]any:
		raw = result
	case string:
		var inner map[string]any
		if err := json.Unmarshal([]byte(result), &inner); err != nil {
			return nil, fmt.Errorf("unreadable payment list result: %w", err)
		}
		raw = inner
	}

	if code := listField(raw, "RESPONSE_CODE"); code != responseCodeOK {
		message := listField(raw, "ERROR_MESSAGE")
		if message == "" {
			message = listField(raw, "RESPONSE_DATA")
		}
		return nil, fmt.Errorf("payment list returned %s %s: %s", code, listField(raw, "ERROR_CODE"), message)
	}

	list, _ := raw["LIST"].([]any)
	items := make([]paymentListItem, 0, len(list))
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, paymentListItem{
			referenceCode:   listField(m, "REFERENCE_CODE"),
			clientRefCode:   listField(m, "CLIENT_REFERENCE_CODE"),
			transactionType: listField(m, "TRANSACTION_TYPE"),
			status:          listField(m, "STATUS"),
			trxDate:         listField(m, "TRX_DATE"),
		})
	}
	return items, nil
}

// statusFromListItems derives one status from the rows of a clientRefCode. When the REFERENCE_CODE
// is known only that sale counts. A successful cancel or refund on the same code outranks the sale.
// CANCELP and REFUNDP rows are ignored: their meaning is not documented.
func statusFromListItems(items []paymentListItem, clientRefCode, referenceCode string, flowOver bool) (provider.PaymentStatus, []paymentListItem, error) {
	var matched []paymentListItem
	var sale, cancelled, refunded bool
	status := provider.StatusFailed

	for _, it := range items {
		if it.clientRefCode != clientRefCode {
			continue
		}
		success := strings.EqualFold(it.status, listStatusSuccess)
		switch strings.ToLower(it.transactionType) {
		case "sales":
			if referenceCode != "" && it.referenceCode != referenceCode {
				continue
			}
			sale = true
			switch {
			case success:
				status = provider.StatusSuccessful
			case strings.EqualFold(it.status, listStatusNew):
				if !flowOver && status != provider.StatusSuccessful {
					status = provider.StatusPending
				}
			case !strings.EqualFold(it.status, listStatusError) && status == provider.StatusFailed:
				// An undocumented STATUS is not a verdict.
				status = provider.StatusPending
			}
		case "cancel":
			cancelled = cancelled || success
		case "refund":
			refunded = refunded || success
		default:
			continue
		}
		matched = append(matched, it)
	}

	if !sale {
		return "", nil, errPaymentNotListed
	}
	if status == provider.StatusSuccessful {
		if cancelled {
			status = provider.StatusCancelled
		} else if refunded {
			status = provider.StatusRefunded
		}
	}
	return status, matched, nil
}

// listItemsForResponse returns the matched rows without card data, which the full LIST carries.
func listItemsForResponse(items []paymentListItem) []map[string]string {
	out := make([]map[string]string, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]string{
			"referenceCode":   it.referenceCode,
			"clientRefCode":   it.clientRefCode,
			"transactionType": it.transactionType,
			"status":          it.status,
			"trxDate":         it.trxDate,
		})
	}
	return out
}

// listField renders a JSON value as Nkolay's string form; RESPONSE_CODE arrives as "2" or 2.
func listField(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// validCallbackHash checks the hashDataV2 Nkolay posts with a 3D result.
func (p *NkolayProvider) validCallbackHash(data map[string]string) bool {
	received := data["hashDataV2"]
	if received == "" || p.secretKey == "" {
		return false
	}
	parts := make([]string, 0, len(callbackHashFields)+1)
	for _, field := range callbackHashFields {
		parts = append(parts, data[field])
	}
	parts = append(parts, p.secretKey)
	return hmac.Equal([]byte(hashV2(parts...)), []byte(received))
}

// hashV2 is Nkolay's hashDataV2: Base64(SHA-512(parts joined with "|")).
func hashV2(parts ...string) string {
	sum := sha512.Sum512([]byte(strings.Join(parts, "|")))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// CancelPayment cancels a payment (same day cancellation)
func (p *NkolayProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	systemTime, err := provider.GetProviderRequestFromLogWithPaymentID("nkolay", request.PaymentID, "systemTime")
	if err != nil {
		return nil, fmt.Errorf("failed to get systemTime: %s %w", request.PaymentID, err)
	}

	// Convert systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
	trxDate, err := p.formatDateForNkolay(systemTime)
	if err != nil {
		return nil, fmt.Errorf("failed to format trxDate: %w", err)
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": request.PaymentID,
		"type":          "cancel",
		"trxDate":       trxDate,
		"resultUrl":     "json",
	}

	// Generate hash: sx+referenceCode+type+trxDate+secretkey
	input := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointCancelRefund, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to cancel payment: %w", err)
	}

	return &provider.PaymentResponse{
		PaymentID:  request.PaymentID,
		Success:    strings.Contains(string(responseBody), "SUCCESS"),
		Status:     provider.StatusCancelled,
		Message:    "Payment cancellation processed",
		SystemTime: timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}, nil
}

// RefundPayment issues a refund for a payment
func (p *NkolayProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	systemTime, err := provider.GetProviderRequestFromLogWithPaymentID("nkolay", request.PaymentID, "systemTime")
	if err != nil {
		return nil, fmt.Errorf("failed to get systemTime: %s %w", request.PaymentID, err)
	}

	// Convert systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
	trxDate, err := p.formatDateForNkolay(systemTime)
	if err != nil {
		return nil, fmt.Errorf("failed to format trxDate: %w", err)
	}

	refundAmount := request.RefundAmount
	if refundAmount <= 0 {
		return nil, errors.New("nkolay: refund amount must be greater than 0")
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": request.PaymentID,
		"type":          "refund",
		"trxDate":       trxDate,
		"amount":        fmt.Sprintf("%.2f", refundAmount),
		"resultUrl":     "json",
	}

	// Generate hash: sx+referenceCode+type+amount+trxDate+secretkey
	input := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["amount"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointCancelRefund, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to refund payment: %w", err)
	}

	return &provider.RefundResponse{
		Success:      strings.Contains(string(responseBody), "SUCCESS"),
		RefundID:     fmt.Sprintf("refund_%s_%d", request.PaymentID, time.Now().Unix()),
		PaymentID:    request.PaymentID,
		RefundAmount: refundAmount,
		Status:       "processed",
		Message:      "Refund processed",
		SystemTime:   timePtr(time.Now()),
		RawResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}, nil
}

// ValidateWebhook validates incoming webhook notifications
func (p *NkolayProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Nkolay sends POST callbacks to success/fail URLs
	// Basic validation - in real implementation would verify signature
	if referenceCode := data["referenceCode"]; referenceCode != "" {
		return true, data, nil
	}

	return false, nil, errors.New("nkolay: invalid webhook data")
}

// validatePaymentRequest validates the payment request
func (p *NkolayProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.TenantID == 0 {
		return errors.New("tenantID is required")
	}

	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Name == "" || request.Customer.Surname == "" {
		return errors.New("customer name and surname are required")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if request.CardInfo.CardNumber == "" {
		return errors.New("card number is required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("CVV is required")
	}

	if request.CardInfo.ExpireMonth == "" || request.CardInfo.ExpireYear == "" {
		return errors.New("expiry date is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment handles both regular and 3D payment processing
func (p *NkolayProvider) processPayment(ctx context.Context, request provider.PaymentRequest, use3D bool) (*provider.PaymentResponse, error) {
	// Generate unique reference code
	clientRefCode := fmt.Sprintf("gopay_%d", time.Now().UnixNano())

	formData := map[string]string{
		"sx":              p.sx,
		"clientRefCode":   clientRefCode,
		"amount":          fmt.Sprintf("%.2f", request.Amount),
		"transactionType": "SALES",
		"detail":          "true",
		"description":     request.Description,
		"rnd":             time.Now().Format("02-01-2006 15:04:05"),
		"instalments":     strconv.Itoa(request.InstallmentCount),
		"installmentNo":   strconv.Itoa(request.InstallmentCount),
		"ECOMM_PLATFORM":  request.Description,
	}

	if request.InstallmentCount > 0 {
		// get installment count from nkolay
		installmentCount, err := p.GetInstallmentCount(ctx, provider.InstallmentInquireRequest{
			Amount: request.Amount,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get installment count: %w", err)
		}
		// amount + amount * commission rate / 100, using the OTHERS bucket rate for this installment count
		for _, installment := range installmentCount.Installments["OTHERS"] {
			if installment.Installment == request.InstallmentCount {
				request.Amount = request.Amount + (request.Amount * installment.Commission / 100)
				break
			}
		}
	}

	// Add 3D settings
	stateId := ""
	if use3D {
		formData["use3D"] = "true"
		// Build callback URLs through GoPay

		state := provider.CallbackState{
			PaymentID:        request.ID,
			TenantID:         request.TenantID,
			Amount:           request.Amount,
			Currency:         request.Currency,
			LogID:            request.LogID,
			Provider:         "nkolay",
			Environment:      request.Environment,
			Timestamp:        time.Now(),
			OriginalCallback: request.CallbackURL,
			ClientIP:         request.ClientIP,
			Installment:      request.InstallmentCount,
			SessionID:        request.SessionID,
		}

		gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "nkolay", state)
		if err != nil {
			return nil, fmt.Errorf("failed to create short callback URL: %w", err)
		}

		// Extract state ID from the callback URL
		if parsedURL, err := url.Parse(gopayCallbackURL); err == nil {
			stateId = parsedURL.Query().Get("state")
		}

		// The status parameter only labels the log row; Complete3DPayment does not trust it.
		formData["successUrl"] = gopayCallbackURL + "&status=" + statusSuccess
		formData["failUrl"] = gopayCallbackURL + "&status=" + statusFailed

	}

	if request.CardInfo.CardHolderName != "" {
		formData["cardHolderName"] = request.CardInfo.CardHolderName
	}
	if request.CardInfo.ExpireMonth != "" {
		formData["month"] = request.CardInfo.ExpireMonth
	}
	if request.CardInfo.ExpireYear != "" {
		formData["year"] = request.CardInfo.ExpireYear
	}
	if request.CardInfo.CVV != "" {
		formData["cvv"] = request.CardInfo.CVV
	}
	if request.CardInfo.CardNumber != "" {
		formData["cardNumber"] = request.CardInfo.CardNumber
	}

	// Generate hash according to Nkolay documentation
	// Hash format varies by endpoint, for payment it's specific fields + secret key
	input := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["successUrl"] + formData["failUrl"] + formData["rnd"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointPayment, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: payment request failed: %w", err)
	}

	// add provider request to client request
	if reqMap, err := provider.StructToMap(formData); err == nil {
		_ = provider.AddProviderRequestToClientRequest("nkolay", "providerRequest", reqMap, p.logID)
	}

	return p.parsePaymentResponse(responseBody, clientRefCode, request.Amount, stateId)
}

// generateSHA1Hash generates SHA1 hash and encodes it in base64 (Nkolay official format)
func (p *NkolayProvider) generateSHA1Hash(input string) string {

	// PHP equivalent: base64_encode(pack('H*', sha1($hashstr)))
	// This means: SHA1 -> hex string -> binary -> base64
	h := sha1.New()
	h.Write([]byte(input))
	hexHash := fmt.Sprintf("%x", h.Sum(nil)) // Get hex string

	// Convert hex string to binary (like PHP's pack('H*', ...))
	binaryData := make([]byte, len(hexHash)/2)
	for i := 0; i < len(hexHash); i += 2 {
		val, _ := strconv.ParseUint(hexHash[i:i+2], 16, 8)
		binaryData[i/2] = byte(val)
	}

	return base64.StdEncoding.EncodeToString(binaryData)
}

// doNkolayFormRequest is a helper to send multipart/form-data requests to Nkolay API
func (p *NkolayProvider) doNkolayFormRequest(ctx context.Context, endpoint string, formData map[string]string) ([]byte, error) {
	httpReq := &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: endpoint,
		FormData: formData,
		Headers:  map[string]string{"Accept": "application/json, text/html"},
	}
	resp, err := p.httpClient.SendForm(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// parsePaymentResponse parses Nkolay payment response
func (p *NkolayProvider) parsePaymentResponse(responseBody []byte, paymentID string, amount float64, stateId string) (*provider.PaymentResponse, error) {
	response := &provider.PaymentResponse{
		PaymentID:  paymentID,
		Amount:     amount,
		Currency:   defaultCurrency,
		SystemTime: timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}

	responseStr := string(responseBody)

	// Try to parse as JSON first
	var jsonResponse map[string]any
	if err := json.Unmarshal(responseBody, &jsonResponse); err == nil {
		// JSON response - extract values
		responseCode := jsonResponse["RESPONSE_CODE"]
		responseData := jsonResponse["RESPONSE_DATA"]
		authCode := jsonResponse["AUTH_CODE"]
		referenceCode := jsonResponse["REFERENCE_CODE"]
		errorMessage := jsonResponse["ERROR_MESSAGE"]
		htmlString := jsonResponse["HTML_STRING"]

		// Update payment ID to use reference code if available
		if refCode, ok := referenceCode.(string); ok && refCode != "" {
			response.PaymentID = refCode
			// Nkolay issues the reference code only in this response, after the callback state was
			// stored; write it into the state so the 3D completion can match the payment.
			if stateId != "" {
				_ = provider.UpdateCallbackState(context.Background(), stateId, refCode)
			}
		}

		// Store additional data in provider response
		if providerResp, ok := response.ProviderResponse.(map[string]any); ok {
			if authCode != nil {
				providerResp["auth_code"] = authCode
			}
			if referenceCode != nil {
				providerResp["reference_code"] = referenceCode
			}
		}

		// Check for 3D Secure HTML form in BANK_REQUEST_MESSAGE
		if bankRequestMessage, ok := jsonResponse["BANK_REQUEST_MESSAGE"].(string); ok && bankRequestMessage != "" && strings.Contains(bankRequestMessage, "form") {
			response.Success = true
			response.Status = provider.StatusPending
			response.Message = bankRequestMessage

			// Clean HTML for client use
			cleanHTML := p.cleanHTMLForClient(bankRequestMessage)
			response.HTML = cleanHTML

			return response, nil
		}

		// Also check HTML_STRING as fallback
		if htmlStr, ok := htmlString.(string); ok && htmlStr != "" && strings.Contains(htmlStr, "form") {
			response.Success = true
			response.Status = provider.StatusPending
			response.HTML = htmlStr

			return response, nil
		}

		// Check response code for success
		if code, ok := responseCode.(float64); ok {
			switch int(code) {
			case 2: // Success response code for Nkolay
				response.Success = true
				response.Status = provider.StatusSuccessful
				if responseData != nil {
					response.Message = fmt.Sprintf("%v", responseData)
				} else {
					response.Message = "Payment successful"
				}
			case 0, 1, 3, 4, 5: // Various error codes
				response.Success = false
				response.Status = provider.StatusFailed
				if errorMessage != nil && errorMessage != "" {
					response.Message = fmt.Sprintf("%v", errorMessage)
				} else if responseData != nil {
					response.Message = fmt.Sprintf("%v", responseData)
				} else {
					response.Message = "Payment failed"
				}

				// Set error code based on response code
				switch int(code) {
				case 0:
					response.ErrorCode = "GENERAL_ERROR"
				case 1:
					response.ErrorCode = "INVALID_REQUEST"
				case 3:
					response.ErrorCode = "INSUFFICIENT_FUNDS"
				case 4:
					response.ErrorCode = "INVALID_CARD"
				case 5:
					response.ErrorCode = "DECLINED"
				default:
					response.ErrorCode = "PAYMENT_FAILED"
				}
			default:
				response.Success = false
				response.Status = provider.StatusFailed
				response.Message = "Unknown response code"
				response.ErrorCode = "UNKNOWN_RESPONSE"
			}
		} else {
			// No response code found, check for other indicators
			if responseData != nil && strings.Contains(strings.ToUpper(fmt.Sprintf("%v", responseData)), "BAŞARILI") {
				response.Success = true
				response.Status = provider.StatusSuccessful
				response.Message = fmt.Sprintf("%v", responseData)
			} else {
				response.Success = false
				response.Status = provider.StatusFailed
				response.Message = "Invalid response format"
				response.ErrorCode = "INVALID_RESPONSE"
			}
		}

		return response, nil
	}

	// Fallback to HTML/text parsing for non-JSON responses
	if strings.Contains(responseStr, "form") && strings.Contains(responseStr, "action") {
		// 3D Secure form returned
		response.Success = true
		response.Status = provider.StatusPending
		response.HTML = responseStr
	} else if strings.Contains(responseStr, "SUCCESS") || strings.Contains(responseStr, "APPROVED") {
		// Payment successful
		response.Success = true
		response.Status = provider.StatusSuccessful
		response.Message = "Payment successful"
	} else if strings.Contains(responseStr, "FAILED") || strings.Contains(responseStr, "ERROR") {
		// Payment failed
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = "Payment failed"

		// Extract error details
		if strings.Contains(responseStr, "INSUFFICIENT") {
			response.ErrorCode = "INSUFFICIENT_FUNDS"
		} else if strings.Contains(responseStr, "INVALID") {
			response.ErrorCode = "INVALID_CARD"
		} else {
			response.ErrorCode = "PAYMENT_FAILED"
		}
	} else {
		// Unknown response
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = "Unknown response from Nkolay"
		response.ErrorCode = "UNKNOWN_RESPONSE"
	}

	return response, nil
}

// formatDateForNkolay converts systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
func (p *NkolayProvider) formatDateForNkolay(systemTime string) (string, error) {
	// Parse the systemTime which is in format "2025-07-23T11:30:21.163704+03"
	// We want to extract just the date part and format as "2025.07.23"

	// Split on the first 'T' to keep just the date part
	datepart, _, _ := strings.Cut(systemTime, "T")

	// Parse the date part "2025-07-23"
	parsedTime, err := time.Parse("2006-01-02", datepart)
	if err != nil {
		return "", fmt.Errorf("failed to parse date %s: %w", datepart, err)
	}

	// Format as "2025.07.23"
	return parsedTime.Format("2006.01.02"), nil
}

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}

// cleanHTMLForClient cleans HTML by removing escape characters and formatting properly
func (p *NkolayProvider) cleanHTMLForClient(htmlStr string) string {
	// Remove common escape characters
	cleanHTML := strings.ReplaceAll(htmlStr, "\\r", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\n", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\t", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\r", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\n", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\t", "")

	// Remove JSON escape characters
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\\"", "\"")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\/", "/")

	// Fix JavaScript onload attribute quotation issue
	// Replace: onload=document.forms["form"].submit()
	// With: onload="document.forms['form'].submit()"
	cleanHTML = strings.ReplaceAll(cleanHTML, `onload=document.forms["form"].submit()`, `onload="document.forms['form'].submit()"`)

	// Remove extra spaces between tags and attributes
	onloadRegex := regexp.MustCompile(`>\s*<`)
	cleanHTML = onloadRegex.ReplaceAllString(cleanHTML, "><")

	// Clean script tag formatting
	cleanHTML = strings.ReplaceAll(cleanHTML, ">    var ", "> var ")
	cleanHTML = strings.ReplaceAll(cleanHTML, ";    ", "; ")

	return cleanHTML
}

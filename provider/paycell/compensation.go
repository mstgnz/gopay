package paycell

import (
	"context"
	"fmt"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/provider"
)

// Compensation for a provisionAll whose outcome gopay never learned.
//
// Paycell's documentation puts this on the merchant: when provisionAll times out, query
// inquireAll, and if the money was captured send reverse (or refundAll once the day is settled)
// so the books balance. Without it the customer is charged for a payment gopay reported as
// failed, and nothing surfaces until the daily reconciliation.
//
// Nothing here changes what the caller sees. The provision still returns its error; the
// compensation runs detached and only writes to the payment's log row.

const (
	responseCodeAlreadyReversed   = "2014" // reverse of a transaction that is already reversed
	responseCodeReverseAfterRecon = "2029" // too late to reverse, refundAll is the way
	responseCodeOriginalRefunded  = "2027" // the sale was already refunded
	responseCodeAlreadyRefunded   = "3380" // this refund was already performed
)

// provisionTimeoutCodes answer "no verdict", not "declined". A declined provision is a known
// outcome and needs no compensation; these do not tell us whether the money moved.
var provisionTimeoutCodes = map[string]bool{
	"3017": true, // Provision timeout
	"3364": true, // Internal Timeout
	"3383": true, // Single sale approve timeout
}

// compensationSlots bounds how many compensations run at once across the process. A full queue
// drops the task and says so: the alternative is an unbounded goroutine fan-out exactly when
// Paycell is already unwell.
var compensationSlots = make(chan struct{}, 32)

type compensationAction int

const (
	compensationRetry   compensationAction = iota // Paycell has not settled the transaction yet
	compensationNone                              // nothing was captured, or it was declined
	compensationReverse                           // money was captured, take it back
)

type compensationTiming struct {
	firstDelay time.Duration
	retryDelay time.Duration
	attempts   int
	budget     time.Duration
}

func defaultCompensationTiming() compensationTiming {
	return compensationTiming{
		firstDelay: 2 * time.Second,
		retryDelay: 5 * time.Second,
		attempts:   3,
		budget:     90 * time.Second,
	}
}

// unknownProvision is everything the compensation needs, captured by value at the call site.
// It is never read off the provider later: the fields it would need (logID, clientIP, msisdn)
// belong to one request, and this work outlives the request.
type unknownProvision struct {
	referenceNumber string
	amountKurus     string
	msisdn          string
	clientIP        string
	logID           int64
	reason          string
}

// newUnknownProvision reads back what was actually sent to Paycell, so the compensation targets
// the same transaction with the same amount rather than a re-derived one.
func newUnknownProvision(request map[string]any, logID int64, clientIP, reason string) unknownProvision {
	str := func(key string) string {
		value, _ := request[key].(string)
		return value
	}

	return unknownProvision{
		referenceNumber: str("referenceNumber"),
		amountKurus:     str("amount"),
		msisdn:          str("msisdn"),
		clientIP:        clientIP,
		logID:           logID,
		reason:          reason,
	}
}

// scheduleProvisionCompensation starts the compensation in the background and returns a channel
// that closes when it is finished. Production ignores the channel; tests wait on it.
func (p *PaycellProvider) scheduleProvisionCompensation(u unknownProvision) <-chan struct{} {
	done := make(chan struct{})

	if !p.compensationEnabled {
		close(done)
		return done
	}

	if u.referenceNumber == "" || u.amountKurus == "" || u.msisdn == "" {
		logger.Warn("paycell: provision compensation skipped, incomplete request", logger.LogContext{
			Provider: "paycell",
			Fields: map[string]any{
				"log_id":           u.logID,
				"reference_number": u.referenceNumber,
				"reason":           u.reason,
			},
		})
		close(done)
		return done
	}

	select {
	case compensationSlots <- struct{}{}:
	default:
		logger.Error("paycell: provision compensation dropped, queue full", nil, logger.LogContext{
			Provider: "paycell",
			Fields: map[string]any{
				"log_id":           u.logID,
				"reference_number": u.referenceNumber,
				"reason":           u.reason,
			},
		})
		close(done)
		return done
	}

	// Own copy of the provider: this outlives the request, and the request's own provider keeps
	// being written to until it returns.
	worker := *p
	if worker.timing.attempts <= 0 {
		worker.timing = defaultCompensationTiming()
	}

	// Counted before the goroutine starts, so a shutdown that begins right now already waits
	// for this one.
	provider.BackgroundTaskStarted()

	go func() {
		defer provider.BackgroundTaskDone()
		defer close(done)
		defer func() { <-compensationSlots }()
		defer func() {
			if r := recover(); r != nil {
				logger.Error("paycell: provision compensation panicked", fmt.Errorf("%v", r), logger.LogContext{
					Provider: "paycell",
					Fields: map[string]any{
						"log_id":           u.logID,
						"reference_number": u.referenceNumber,
					},
				})
			}
		}()

		// The request context is dead by now (its deadline is usually what produced the
		// timeout), so the work gets its own budget.
		ctx, cancel := context.WithTimeout(context.Background(), worker.timing.budget)
		defer cancel()

		worker.runProvisionCompensation(ctx, u)
	}()

	return done
}

func (p *PaycellProvider) runProvisionCompensation(ctx context.Context, u unknownProvision) {
	codes := make([]string, 0, p.timing.attempts)
	action := compensationRetry

	// answered: Paycell replied at least once. sawOrder: at least one of those replies was
	// something other than "order not found". Only "answered but never found" is quiet, because
	// it is the one case that proves the provision never reached them. No answer at all proves
	// nothing and has to be looked at.
	answered := false
	sawOrder := false

	for attempt := 1; attempt <= p.timing.attempts; attempt++ {
		delay := p.timing.retryDelay
		if attempt == 1 {
			delay = p.timing.firstDelay
		}
		if !sleepContext(ctx, delay) {
			p.recordCompensation(u, "undetermined", codes)
			return
		}

		inquireResp, err := p.compensationInquire(ctx, u)
		if err != nil {
			codes = append(codes, "transport_error")
			continue
		}

		codes = append(codes, inquireResp.ResponseHeader.ResponseCode)
		answered = true
		if inquireResp.ResponseHeader.ResponseCode != responseCodeOrderNotFound {
			sawOrder = true
		}

		if action = decideCompensation(inquireResp); action != compensationRetry {
			break
		}
	}

	switch action {
	case compensationNone:
		p.recordCompensation(u, "no_capture", codes)

	case compensationReverse:
		p.reverseCapturedProvision(ctx, u, codes)

	default:
		if answered && !sawOrder {
			// Paycell answered every time and never had the order, so the provision did not
			// reach them and no money moved.
			logger.Warn("paycell: provision never reached Paycell, nothing to compensate", logger.LogContext{
				Provider: "paycell",
				Fields: map[string]any{
					"log_id":           u.logID,
					"reference_number": u.referenceNumber,
					"reason":           u.reason,
				},
			})
			p.recordCompensation(u, "no_order", codes)
			return
		}

		// The order exists but will not settle. Nothing may be reversed on a guess, so this is
		// where a human takes over.
		logger.Error("paycell: provision outcome still unknown, manual reconciliation needed", nil, logger.LogContext{
			Provider: "paycell",
			Fields: map[string]any{
				"log_id":           u.logID,
				"reference_number": u.referenceNumber,
				"amount_kurus":     u.amountKurus,
				"inquire_codes":    codes,
			},
		})
		p.recordCompensation(u, "undetermined", codes)
	}
}

// decideCompensation reads an inquireAll response and answers the only question that matters:
// did Paycell capture money for a payment gopay reported as failed?
func decideCompensation(resp *PaycellInquireResponse) compensationAction {
	if resp == nil {
		return compensationRetry
	}

	if resp.ResponseHeader.ResponseCode != responseCodeSuccess {
		// 2013 means the order is not queryable yet and 3023 means it is still being processed;
		// both are transient. Anything else is a real answer: there is nothing to compensate.
		switch resp.ResponseHeader.ResponseCode {
		case responseCodeOrderNotFound, responseCodeProcessing:
			return compensationRetry
		default:
			return compensationNone
		}
	}

	if len(resp.ProvisionList) == 0 {
		// The inquiry worked but no provision exists yet.
		return compensationRetry
	}

	// A successful reverse or refund already in the list means someone got there first.
	for _, provision := range resp.ProvisionList {
		if provision.ResponseCode != responseCodeSuccess {
			continue
		}
		switch provision.ProvisionType {
		case "REVERSE", "REFUND":
			return compensationNone
		}
	}

	last := resp.ProvisionList[len(resp.ProvisionList)-1]
	switch last.ResponseCode {
	case responseCodeSuccess:
		return compensationReverse
	case responseCodeProcessing:
		return compensationRetry
	default:
		// A decline (4001 insufficient limit and friends). The customer was not charged.
		return compensationNone
	}
}

func (p *PaycellProvider) compensationInquire(ctx context.Context, u unknownProvision) (*PaycellInquireResponse, error) {
	request := map[string]any{
		"paymentMethodType":       "CREDIT_CARD",
		"merchantCode":            p.merchantID,
		"msisdn":                  u.msisdn,
		"originalReferenceNumber": u.referenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"currency":                "TRY",
		"paymentType":             "SALE",
		"orderId":                 u.referenceNumber,
		"requestHeader":           p.compensationHeader(u),
	}

	var response PaycellInquireResponse
	if err := p.compensationCall(ctx, endpointInquireAll, request, &response, "compensationInquire", u.logID); err != nil {
		return nil, err
	}

	return &response, nil
}

func (p *PaycellProvider) reverseCapturedProvision(ctx context.Context, u unknownProvision, codes []string) {
	request := map[string]any{
		"merchantCode":            p.merchantID,
		"msisdn":                  u.msisdn,
		"originalReferenceNumber": u.referenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"amount":                  u.amountKurus,
		"requestHeader":           p.compensationHeader(u),
	}

	var response PaycellReverseResponse
	if err := p.compensationCall(ctx, endpointReverse, request, &response, "compensationReverse", u.logID); err != nil {
		logger.Error("paycell: compensation reverse could not be sent", err, logger.LogContext{
			Provider: "paycell",
			Fields:   map[string]any{"log_id": u.logID, "reference_number": u.referenceNumber},
		})
		p.recordCompensation(u, "reverse_failed", codes)
		return
	}

	switch response.ResponseHeader.ResponseCode {
	case responseCodeSuccess:
		p.recordCompensation(u, "reversed", codes)

	case responseCodeAlreadyReversed:
		p.recordCompensation(u, "already_reversed", codes)

	case responseCodeReverseAfterRecon:
		// The day settled between the payment and this call, so the money can only come back
		// as a refund now.
		p.refundCapturedProvision(ctx, u, codes)

	default:
		logger.Error("paycell: compensation reverse rejected, money may still be captured", nil, logger.LogContext{
			Provider: "paycell",
			Fields: map[string]any{
				"log_id":           u.logID,
				"reference_number": u.referenceNumber,
				"amount_kurus":     u.amountKurus,
				"response_code":    response.ResponseHeader.ResponseCode,
			},
		})
		p.recordCompensation(u, "reverse_failed", codes)
	}
}

func (p *PaycellProvider) refundCapturedProvision(ctx context.Context, u unknownProvision, codes []string) {
	request := map[string]any{
		"msisdn":                  u.msisdn,
		"merchantCode":            p.merchantID,
		"originalReferenceNumber": u.referenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"amount":                  u.amountKurus,
		"pointAmount":             "",
		"requestHeader":           p.compensationHeader(u),
	}

	var response PaycellReverseResponse
	if err := p.compensationCall(ctx, endpointRefundAll, request, &response, "compensationRefund", u.logID); err != nil {
		logger.Error("paycell: compensation refund could not be sent", err, logger.LogContext{
			Provider: "paycell",
			Fields:   map[string]any{"log_id": u.logID, "reference_number": u.referenceNumber},
		})
		p.recordCompensation(u, "refund_failed", codes)
		return
	}

	switch response.ResponseHeader.ResponseCode {
	case responseCodeSuccess:
		p.recordCompensation(u, "refunded", codes)

	case responseCodeOriginalRefunded, responseCodeAlreadyRefunded:
		p.recordCompensation(u, "already_refunded", codes)

	default:
		logger.Error("paycell: compensation refund rejected, money may still be captured", nil, logger.LogContext{
			Provider: "paycell",
			Fields: map[string]any{
				"log_id":           u.logID,
				"reference_number": u.referenceNumber,
				"amount_kurus":     u.amountKurus,
				"response_code":    response.ResponseHeader.ResponseCode,
			},
		})
		p.recordCompensation(u, "refund_failed", codes)
	}
}

// compensationCall sends one compensation request and logs both sides of it under its own keys,
// so a compensation never overwrites anything the payment itself wrote.
func (p *PaycellProvider) compensationCall(ctx context.Context, endpoint string, request map[string]any, response any, logKey string, logID int64) error {
	if requestMap, err := provider.StructToMap(request); err == nil {
		p.logRequest(logKey+"Request", requestMap, logID)
	}

	httpResponse, err := p.httpClient.SendJSON(ctx, &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: endpoint,
		Body:     request,
	})
	if err != nil {
		return fmt.Errorf("failed to send %s request: %w", logKey, err)
	}

	if err := p.httpClient.ParseJSONResponse(httpResponse, response); err != nil {
		return fmt.Errorf("failed to unmarshal %s response: %w. Response body: %s", logKey, err, httpResponse.RawBody)
	}

	if responseMap, err := provider.StructToMap(response); err == nil {
		p.logRequest(logKey+"Response", responseMap, logID)
	}

	return nil
}

func (p *PaycellProvider) compensationHeader(u unknownProvision) PaycellRequestHeader {
	clientIP := u.clientIP
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}

	return PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     clientIP,
		TransactionDateTime: p.generateTransactionDateTime(),
		TransactionID:       p.generateTransactionID(),
	}
}

// recordCompensation writes the verdict into the payment's log row. It is the only place that
// says what happened, so every path ends here.
func (p *PaycellProvider) recordCompensation(u unknownProvision, outcome string, codes []string) {
	p.logRequest("compensationResult", map[string]any{
		"outcome":         outcome,
		"reason":          u.reason,
		"referenceNumber": u.referenceNumber,
		"amount":          u.amountKurus,
		"inquireCodes":    codes,
		"finishedAt":      time.Now().UTC().Format(time.RFC3339),
	}, u.logID)
}

// sleepContext waits for d and reports whether the wait completed rather than being cancelled.
func sleepContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

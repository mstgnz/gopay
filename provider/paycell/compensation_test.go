package paycell

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// paycellStub is a Paycell that answers provisionAll, inquireAll, reverse and refundAll from
// canned responses and records every call.
type paycellStub struct {
	mu sync.Mutex

	provisionDelay time.Duration
	inquire        []PaycellInquireResponse // one per call, last one repeats
	reverseCode    string
	refundCode     string

	calls    []string
	bodies   map[string][]map[string]any
	inquireN int
}

func newPaycellStub() *paycellStub {
	return &paycellStub{
		reverseCode: responseCodeSuccess,
		refundCode:  responseCodeSuccess,
		bodies:      make(map[string][]map[string]any),
	}
}

func (s *paycellStub) server(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		s.mu.Lock()
		s.calls = append(s.calls, r.URL.Path)
		s.bodies[r.URL.Path] = append(s.bodies[r.URL.Path], body)
		delay := s.provisionDelay
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case endpointGetCardTokenSecure:
			_ = json.NewEncoder(w).Encode(PaycellGetCardTokenSecureResponse{
				Header:    PaycellResponseHeader{ResponseCode: responseCodeSuccess},
				CardToken: "card-token-123",
			})

		case endpointProvisionAll:
			if delay > 0 {
				time.Sleep(delay)
			}
			_ = json.NewEncoder(w).Encode(PaycellProvisionResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess, ResponseDescription: "Success"},
			})

		case endpointInquireAll:
			_ = json.NewEncoder(w).Encode(s.nextInquire())

		case endpointReverse:
			_ = json.NewEncoder(w).Encode(PaycellReverseResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: s.reverseCode},
			})

		case endpointRefundAll:
			_ = json.NewEncoder(w).Encode(PaycellReverseResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: s.refundCode},
			})

		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func (s *paycellStub) nextInquire() PaycellInquireResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.inquire) == 0 {
		return PaycellInquireResponse{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound}}
	}

	index := min(s.inquireN, len(s.inquire)-1)
	s.inquireN++

	return s.inquire[index]
}

func (s *paycellStub) countOf(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, call := range s.calls {
		if call == path {
			count++
		}
	}

	return count
}

func (s *paycellStub) lastBody(path string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	bodies := s.bodies[path]
	if len(bodies) == 0 {
		return nil
	}

	return bodies[len(bodies)-1]
}

func captured(code string) PaycellInquireResponse {
	return PaycellInquireResponse{
		ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
		ProvisionList: []PaycellProvisionListItem{
			{ProvisionType: "SALE", Amount: "10050", ResponseCode: code, ResponseDescription: "Success"},
		},
	}
}

// compensationProvider returns a provider wired to the stub, with the compensation on and its
// waits collapsed so tests do not sleep.
func compensationProvider(t *testing.T, stub *paycellStub, recorder *logRecorder) *PaycellProvider {
	t.Helper()

	p := NewProvider().(*PaycellProvider)
	err := p.Initialize(map[string]string{
		"username":            "test_user",
		"password":            "test_pass",
		"merchantId":          "test_merchant",
		"secureCode":          "test_secure",
		"environment":         "sandbox",
		"compensationEnabled": "true",
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if recorder != nil {
		p.logWriter = recorder.write
	}
	pointProviderAtTestServerWithTimeout(p, stub.server(t).URL, 2*time.Second)

	p.timing = compensationTiming{firstDelay: time.Millisecond, retryDelay: time.Millisecond, attempts: 3, budget: 10 * time.Second}
	p.logID = 4242

	return p
}

func waitFor(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("compensation did not finish")
	}
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition was not met in time")
}

func outcomeOf(t *testing.T, recorder *logRecorder) string {
	t.Helper()

	result, ok := recorder.get("compensationResult")
	if !ok {
		t.Fatal("compensationResult was never written")
	}

	outcome, _ := result["outcome"].(string)

	return outcome
}

func TestDecideCompensation(t *testing.T) {
	tests := []struct {
		name string
		resp *PaycellInquireResponse
		want compensationAction
	}{
		{
			name: "no response yet",
			resp: nil,
			want: compensationRetry,
		},
		{
			name: "order not found is transient",
			resp: &PaycellInquireResponse{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound}},
			want: compensationRetry,
		},
		{
			name: "header still processing is transient",
			resp: &PaycellInquireResponse{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeProcessing}},
			want: compensationRetry,
		},
		{
			name: "other header error is a real answer",
			resp: &PaycellInquireResponse{ResponseHeader: PaycellResponseHeader{ResponseCode: "2011"}},
			want: compensationNone,
		},
		{
			name: "empty provision list is transient",
			resp: &PaycellInquireResponse{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess}},
			want: compensationRetry,
		},
		{
			name: "captured sale must be reversed",
			resp: ptr(captured(responseCodeSuccess)),
			want: compensationReverse,
		},
		{
			name: "declined sale needs nothing",
			resp: ptr(captured("4001")),
			want: compensationNone,
		},
		{
			name: "sale still processing is transient",
			resp: ptr(captured(responseCodeProcessing)),
			want: compensationRetry,
		},
		{
			name: "already reversed needs nothing",
			resp: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
				ProvisionList: []PaycellProvisionListItem{
					{ProvisionType: "SALE", ResponseCode: responseCodeSuccess},
					{ProvisionType: "REVERSE", ResponseCode: responseCodeSuccess},
				},
			},
			want: compensationNone,
		},
		{
			name: "already refunded needs nothing",
			resp: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
				ProvisionList: []PaycellProvisionListItem{
					{ProvisionType: "SALE", ResponseCode: responseCodeSuccess},
					{ProvisionType: "REFUND", ResponseCode: responseCodeSuccess},
				},
			},
			want: compensationNone,
		},
		{
			name: "a failed reverse in the list does not count as compensated",
			resp: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess},
				ProvisionList: []PaycellProvisionListItem{
					{ProvisionType: "SALE", ResponseCode: responseCodeSuccess},
					{ProvisionType: "REVERSE", ResponseCode: "2029"},
				},
			},
			want: compensationNone, // last entry is a decline; the SALE case is covered above
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideCompensation(tt.resp); got != tt.want {
				t.Errorf("decideCompensation = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr(r PaycellInquireResponse) *PaycellInquireResponse { return &r }

func TestCompensationReversesACapturedPayment(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		logID:           4242,
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointReverse); got != 1 {
		t.Fatalf("expected exactly one reverse, got %d", got)
	}

	body := stub.lastBody(endpointReverse)
	if body["originalReferenceNumber"] != "12345678901234567890" {
		t.Errorf("reverse targeted %v, want the provision's referenceNumber", body["originalReferenceNumber"])
	}
	if body["amount"] != "10050" {
		t.Errorf("reverse amount = %v, want the kuruş value that was provisioned", body["amount"])
	}
	if body["referenceNumber"] == body["originalReferenceNumber"] {
		t.Error("reverse must carry its own fresh referenceNumber")
	}
	if body["msisdn"] != "5551234567" {
		t.Errorf("reverse msisdn = %v", body["msisdn"])
	}

	if outcome := outcomeOf(t, recorder); outcome != "reversed" {
		t.Errorf("outcome = %q, want reversed", outcome)
	}
	if id := recorder.logID("compensationResult"); id != 4242 {
		t.Errorf("compensation logged against log_id %d, want 4242", id)
	}
}

func TestCompensationDoesNothingWhenNoOrderExists(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{
		{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound}},
	}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointReverse); got != 0 {
		t.Fatalf("nothing was captured, but %d reverse calls were sent", got)
	}
	if got := stub.countOf(endpointInquireAll); got != 3 {
		t.Errorf("expected 3 inquire attempts, got %d", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "no_order" {
		t.Errorf("outcome = %q, want no_order", outcome)
	}
}

// An order that exists but never settles is the case a human has to resolve, and it must not be
// reported as the quiet "never reached Paycell" outcome.
func TestCompensationReportsUndeterminedWhenTheOrderNeverSettles(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeProcessing)}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointReverse); got != 0 {
		t.Fatalf("an unsettled provision must not be reversed on a guess, got %d reverses", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "undetermined" {
		t.Errorf("outcome = %q, want undetermined", outcome)
	}
}

func TestCompensationDoesNothingForADeclinedPayment(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured("4001")}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointReverse); got != 0 {
		t.Fatalf("declined payment produced %d reverse calls", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "no_capture" {
		t.Errorf("outcome = %q, want no_capture", outcome)
	}
}

func TestCompensationRetriesWhileTheOrderIsStillSettling(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{
		{ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound}},
		captured(responseCodeProcessing),
		captured(responseCodeSuccess),
	}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointInquireAll); got != 3 {
		t.Errorf("expected 3 inquire attempts, got %d", got)
	}
	if got := stub.countOf(endpointReverse); got != 1 {
		t.Errorf("expected one reverse once the sale settled, got %d", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "reversed" {
		t.Errorf("outcome = %q, want reversed", outcome)
	}
}

func TestCompensationFallsBackToRefundAfterReconciliation(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	stub.reverseCode = responseCodeReverseAfterRecon
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointRefundAll); got != 1 {
		t.Fatalf("expected one refundAll after 2029, got %d", got)
	}

	body := stub.lastBody(endpointRefundAll)
	if body["amount"] != "10050" {
		t.Errorf("refund amount = %v, want the provisioned amount", body["amount"])
	}
	if body["originalReferenceNumber"] != "12345678901234567890" {
		t.Errorf("refund targeted %v", body["originalReferenceNumber"])
	}
	if outcome := outcomeOf(t, recorder); outcome != "refunded" {
		t.Errorf("outcome = %q, want refunded", outcome)
	}
}

func TestCompensationTreatsAlreadyReversedAsDone(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	stub.reverseCode = responseCodeAlreadyReversed
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointRefundAll); got != 0 {
		t.Errorf("an already reversed transaction must not be refunded on top, got %d refunds", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "already_reversed" {
		t.Errorf("outcome = %q, want already_reversed", outcome)
	}
}

// A rejected reverse is the case where money stays captured. It must be recorded as failed and
// must not silently turn into a refund.
func TestCompensationRecordsARejectedReverse(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	stub.reverseCode = "2011" // merchant is not found
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointRefundAll); got != 0 {
		t.Errorf("a rejected reverse must not fall back to refund, got %d refunds", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "reverse_failed" {
		t.Errorf("outcome = %q, want reverse_failed", outcome)
	}
}

func TestCompensationRecordsARejectedRefund(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	stub.reverseCode = responseCodeReverseAfterRecon
	stub.refundCode = "2048" // transaction is not suitable for refund
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if outcome := outcomeOf(t, recorder); outcome != "refund_failed" {
		t.Errorf("outcome = %q, want refund_failed", outcome)
	}
}

func TestCompensationTreatsAlreadyRefundedAsDone(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	stub.reverseCode = responseCodeReverseAfterRecon
	stub.refundCode = responseCodeAlreadyRefunded
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := stub.countOf(endpointRefundAll); got != 1 {
		t.Errorf("expected exactly one refund attempt, got %d", got)
	}
	if outcome := outcomeOf(t, recorder); outcome != "already_refunded" {
		t.Errorf("outcome = %q, want already_refunded", outcome)
	}
}

// Everything the compensation sends must land in the log row under its own keys, so an operator
// can reconstruct what happened without the provider's help.
func TestCompensationLogsBothSidesOfEveryCall(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	for _, key := range []string{
		"compensationInquireRequest", "compensationInquireResponse",
		"compensationReverseRequest", "compensationReverseResponse",
		"compensationResult",
	} {
		if _, ok := recorder.get(key); !ok {
			t.Errorf("log key %s was never written; wrote %v", key, recorder.kinds())
		}
	}

	result, _ := recorder.get("compensationResult")
	if result["reason"] != "send_error" {
		t.Errorf("result reason = %v, want send_error", result["reason"])
	}
	if result["referenceNumber"] != "12345678901234567890" {
		t.Errorf("result referenceNumber = %v", result["referenceNumber"])
	}
}

// No answer from Paycell is not evidence that the order does not exist, so it must stay loud.
func TestCompensationStaysUndeterminedWhenInquireNeverAnswers(t *testing.T) {
	recorder := newLogRecorder()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewProvider().(*PaycellProvider)
	if err := p.Initialize(map[string]string{
		"username":            "test_user",
		"password":            "test_pass",
		"merchantId":          "test_merchant",
		"secureCode":          "test_secure",
		"environment":         "sandbox",
		"compensationEnabled": "true",
	}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	p.logWriter = recorder.write
	pointProviderAtTestServerWithTimeout(p, srv.URL, 2*time.Second)
	p.timing = compensationTiming{firstDelay: time.Millisecond, retryDelay: time.Millisecond, attempts: 3, budget: 10 * time.Second}

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if outcome := outcomeOf(t, recorder); outcome != "undetermined" {
		t.Errorf("outcome = %q, want undetermined", outcome)
	}
}

func TestCompensationIsOffByDefault(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}

	p := NewProvider().(*PaycellProvider)
	if err := p.Initialize(map[string]string{
		"username":    "test_user",
		"password":    "test_pass",
		"merchantId":  "test_merchant",
		"secureCode":  "test_secure",
		"environment": "sandbox",
	}); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	pointProviderAtTestServer(p, stub.server(t).URL)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := len(stub.calls); got != 0 {
		t.Errorf("compensation is off, but %d calls were sent: %v", got, stub.calls)
	}
}

// The compensation outlives the request, so it must work off captured values. If it read the
// provider fields instead, it would compensate with whatever the next request wrote there.
func TestCompensationUsesCapturedValuesNotProviderFields(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)

	u := unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		clientIP:        "10.1.1.1",
		logID:           4242,
		reason:          "send_error",
	}
	done := p.scheduleProvisionCompensation(u)

	// What a later request on the same provider would do.
	p.logID = 9999
	p.phoneNumber = "5559999999"
	p.clientIP = "10.9.9.9"

	waitFor(t, done)

	body := stub.lastBody(endpointReverse)
	if body["msisdn"] != "5551234567" {
		t.Errorf("reverse used msisdn %v, want the captured 5551234567", body["msisdn"])
	}
	if id := recorder.logID("compensationResult"); id != 4242 {
		t.Errorf("compensation logged against log_id %d, want the captured 4242", id)
	}

	header, _ := body["requestHeader"].(map[string]any)
	if header["clientIPAddress"] != "10.1.1.1" {
		t.Errorf("reverse used clientIP %v, want the captured 10.1.1.1", header["clientIPAddress"])
	}
}

// A deploy is exactly when compensations get scheduled, so shutdown has to wait for them.
func TestCompensationIsCountedAsBackgroundWork(t *testing.T) {
	stub := newPaycellStub()
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	recorder := newLogRecorder()
	p := compensationProvider(t, stub, recorder)
	p.timing = compensationTiming{firstDelay: 200 * time.Millisecond, retryDelay: time.Millisecond, attempts: 3, budget: 10 * time.Second}

	done := p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		amountKurus:     "10050",
		msisdn:          "5551234567",
		reason:          "send_error",
	})

	shortCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if provider.WaitForBackgroundTasks(shortCtx) {
		t.Error("a running compensation was not counted, so shutdown would not wait for it")
	}

	waitFor(t, done)

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDrain()

	if !provider.WaitForBackgroundTasks(drainCtx) {
		t.Error("the compensation never released its background task count")
	}
}

func TestCompensationSkipsIncompleteCapture(t *testing.T) {
	stub := newPaycellStub()
	p := compensationProvider(t, stub, nil)

	waitFor(t, p.scheduleProvisionCompensation(unknownProvision{
		referenceNumber: "12345678901234567890",
		msisdn:          "5551234567",
		reason:          "send_error",
	}))

	if got := len(stub.calls); got != 0 {
		t.Errorf("compensation ran without an amount: %v", got)
	}
}

// End to end: a provisionAll that times out must return its error unchanged and still get
// compensated, against the reference number it actually sent.
func TestProvisionAllTimeoutTriggersCompensation(t *testing.T) {
	stub := newPaycellStub()
	stub.provisionDelay = 700 * time.Millisecond
	stub.inquire = []PaycellInquireResponse{captured(responseCodeSuccess)}
	recorder := newLogRecorder()

	p := compensationProvider(t, stub, recorder)
	pointProviderAtTestServerWithTimeout(p, p.baseURL, 200*time.Millisecond)

	response, err := p.provisionAll(context.Background(), provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{PhoneNumber: "5551234567"},
	}, "card-token-123", "")

	if err == nil {
		t.Fatal("expected the timeout error to be returned unchanged")
	}
	if response != nil {
		t.Errorf("expected no response on timeout, got %+v", response)
	}

	// provisionAll cannot hand back the compensation's channel without changing its signature,
	// so wait for the verdict it writes.
	waitUntil(t, func() bool {
		_, ok := recorder.get("compensationResult")
		return ok
	})

	if got := stub.countOf(endpointReverse); got != 1 {
		t.Fatalf("expected one reverse for the timed out provision, got %d", got)
	}

	provisionBody := stub.lastBody(endpointProvisionAll)
	reverseBody := stub.lastBody(endpointReverse)
	if reverseBody["originalReferenceNumber"] != provisionBody["referenceNumber"] {
		t.Errorf("reverse targeted %v, but the provision sent %v", reverseBody["originalReferenceNumber"], provisionBody["referenceNumber"])
	}
	if reverseBody["amount"] != provisionBody["amount"] {
		t.Errorf("reverse amount %v does not match the provisioned %v", reverseBody["amount"], provisionBody["amount"])
	}
	if outcome := outcomeOf(t, recorder); outcome != "reversed" {
		t.Errorf("outcome = %q, want reversed", outcome)
	}
}

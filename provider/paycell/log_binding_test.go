package paycell

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

// bindingProvider is a Paycell whose log row is still set from an earlier request, the way a shared
// instance used to carry it; every call below must write to its own request's row instead.
func bindingProvider(t *testing.T, serverURL string, rec *logRecorder) *PaycellProvider {
	t.Helper()
	p := NewProvider().(*PaycellProvider)
	if err := p.Initialize(map[string]string{
		"username":    "test_user",
		"password":    "test_pass",
		"merchantId":  "9998",
		"secureCode":  "test_secure",
		"environment": "sandbox",
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	p.logWriter = rec.write
	pointProviderAtTestServer(p, serverURL)
	p.logReader = &fakeLogReader{
		values: map[string]string{"msisdn": "5551234567", "cardToken": "card-token-123"},
		paths: map[string]string{
			"providerProvisionRequest.referenceNumber": "12345678901234567890",
			"providerProvisionRequest.amount":          "10050",
		},
	}
	p.logID = 999 // another request's row
	return p
}

func TestCancelAndRefundLogToTheirOwnRow(t *testing.T) {
	stub := newPaycellStub()
	srv := stub.server(t)

	rec := newLogRecorder()
	p := bindingProvider(t, srv.URL, rec)
	if _, err := p.CancelPayment(context.Background(), provider.CancelRequest{PaymentID: "pay-1", LogID: 55}); err != nil {
		t.Fatalf("CancelPayment: %v", err)
	}
	if got := rec.logID("reverseRequest"); got != 55 {
		t.Errorf("reverseRequest written to row %d, want 55", got)
	}

	p = bindingProvider(t, srv.URL, rec)
	if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "pay-1", RefundAmount: 10, LogID: 56}); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	if got := rec.logID("refundRequest"); got != 56 {
		t.Errorf("refundRequest written to row %d, want 56", got)
	}
}

// The refund guard records provider.MinorUnits(amount); Paycell must be sent the same kuruş, or a
// half-kuruş amount (10.125) would never match the ledger.
func TestRefundSendsTheMinorUnitsTheGuardRecords(t *testing.T) {
	stub := newPaycellStub()
	p := bindingProvider(t, stub.server(t).URL, newLogRecorder())
	for _, amount := range []float64{10.125, 0.025, 19.99} {
		if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "pay-1", RefundAmount: amount, LogID: 1}); err != nil {
			t.Fatal(err)
		}
		want := strconv.FormatInt(provider.MinorUnits(amount), 10)
		if got := stub.lastBody(endpointRefundAll)["amount"]; got != want {
			t.Errorf("refund of %v sent %v, want %s", amount, got, want)
		}
	}
}

// The status inquiry inside a 3D completion belongs to the completion's row; it used to be sent
// with no row at all, so the callback's inquiry was never logged.
func TestComplete3DPaymentLogsItsInquiryToTheCompletionRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case endpointGetThreeDSessionResult:
			ok := rejectedThreeDResult("")
			ok.MdStatus = "1"
			ok.ThreeDOperationResult.ThreeDResult = "0"
			_ = json.NewEncoder(w).Encode(ok)
		case endpointProvisionAll:
			_ = json.NewEncoder(w).Encode(PaycellProvisionResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeSuccess, ResponseDescription: "Success"},
			})
		case endpointInquireAll:
			_ = json.NewEncoder(w).Encode(captured(responseCodeSuccess))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	rec := newLogRecorder()
	p := bindingProvider(t, srv.URL, rec)
	state := &provider.CallbackState{
		PaymentID: "session-1", LogID: 77, Amount: 100.5, Currency: "TRY",
		ClientIP: "10.1.2.3", OriginalCallback: "https://consumer.example/cb",
	}
	if _, err := p.Complete3DPayment(context.Background(), state, map[string]string{}); err != nil {
		t.Fatalf("Complete3DPayment: %v", err)
	}

	for _, kind := range []string{"getThreeDSessionResultRequest", "providerProvisionRequest", "providerInquireRequest", "providerInquireResponse"} {
		if got := rec.logID(kind); got != 77 {
			t.Errorf("%s written to row %d, want 77 (logged: %v)", kind, got, rec.kinds())
		}
	}
}

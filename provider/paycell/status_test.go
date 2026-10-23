package paycell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

const statusTestPaymentID = "f521e32c-85d7-41fe-9f1a-edf94bb70b3f"

// fakeLogReader answers log lookups from memory. A missing key returns ErrLogValueNotFound, the
// way the database helpers do; failWith fails every lookup, failKeys only the keys it names.
type fakeLogReader struct {
	values   map[string]string // recursive lookup, by key
	paths    map[string]string // "parent.child"
	objects  map[string]string // top-level key, JSON text
	callback string            // newest 3D callback row's request JSON, "" = none; fails via failKeys["callback"]
	failWith error
	failKeys map[string]error
}

func (f *fakeLogReader) lookup(m map[string]string, key string) (string, error) {
	if f.failWith != nil {
		return "", f.failWith
	}
	if err, ok := f.failKeys[key]; ok {
		return "", err
	}
	if v, ok := m[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%s %w", key, provider.ErrLogValueNotFound)
}

func (f *fakeLogReader) value(_, key string) (string, error) { return f.lookup(f.values, key) }

func (f *fakeLogReader) nested(_, parentKey, childKey string) (string, error) {
	return f.lookup(f.paths, parentKey+"."+childKey)
}

func (f *fakeLogReader) object(_, key string) (string, error) { return f.lookup(f.objects, key) }

func (f *fakeLogReader) callbackRequest(string) (string, error) {
	m := map[string]string{}
	if f.callback != "" {
		m["callback"] = f.callback
	}
	return f.lookup(m, "callback")
}

// threeDStartLogs is what /payment/3d leaves behind: a 3D session request, no result, no provision.
func threeDStartLogs() *fakeLogReader {
	return &fakeLogReader{
		values: map[string]string{"msisdn": "5551234567"},
		paths:  map[string]string{"getThreeDSessionRequest.merchantCode": "9998"},
	}
}

func threeDResultJSON(t *testing.T, result PaycellGetThreeDSessionResultResponse) string {
	t.Helper()
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// rejectedThreeDResult mirrors production log row 83808: mdStatus 0, 3D password wrong, 10009.
func rejectedThreeDResult(mdErrorMessage string) PaycellGetThreeDSessionResultResponse {
	return PaycellGetThreeDSessionResultResponse{
		MdStatus:       "0",
		MdErrorMessage: mdErrorMessage,
		ThreeDOperationResult: PaycellThreeDOperationResult{
			ThreeDResult:            "10009",
			ThreeDResultDescription: "3D Doğrulama Başarısız",
			ResponseHeader:          PaycellResponseHeader{TransactionID: "3dtxn", ResponseCode: responseCodeSuccess},
		},
	}
}

// inquireServer serves inquireAll with resp and counts the calls.
type inquireServer struct {
	mu    sync.Mutex
	calls int
	body  map[string]any
}

func newStatusTestProvider(t *testing.T, logs paymentLogReader, inquire *PaycellInquireResponse) (*PaycellProvider, *inquireServer) {
	t.Helper()
	s := &inquireServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.calls++
		_ = json.NewDecoder(r.Body).Decode(&s.body)
		s.mu.Unlock()

		if r.URL.Path != endpointInquireAll || inquire == nil {
			t.Errorf("unexpected Paycell call %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(inquire)
	}))
	t.Cleanup(srv.Close)

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
	pointProviderAtTestServer(p, srv.URL)
	p.logReader = logs
	return p, s
}

func (s *inquireServer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func getStatus(t *testing.T, p *PaycellProvider) (*provider.PaymentResponse, error) {
	t.Helper()
	return p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: statusTestPaymentID})
}

func TestGetPaymentStatus_ThreeDRejectedIsFailed(t *testing.T) {
	logs := threeDStartLogs()
	logs.objects = map[string]string{"getThreeDSessionResultResponse": threeDResultJSON(t, rejectedThreeDResult("3D Şifresi Yanlış."))}
	p, srv := newStatusTestProvider(t, logs, nil)

	resp, err := getStatus(t, p)
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Success || resp.Status != provider.StatusFailed {
		t.Errorf("success=%v status=%q, want false/failed", resp.Success, resp.Status)
	}
	if resp.ErrorCode != "10009" {
		t.Errorf("errorCode = %q, want 10009", resp.ErrorCode)
	}
	if resp.Message != "3D Şifresi Yanlış." {
		t.Errorf("message = %q, want mdErrorMessage", resp.Message)
	}
	if resp.PaymentID != statusTestPaymentID {
		t.Errorf("paymentId = %q", resp.PaymentID)
	}
	if resp.Amount != 0 {
		t.Errorf("amount = %v, want 0: nothing was charged", resp.Amount)
	}
	if n := srv.callCount(); n != 0 {
		t.Errorf("Paycell called %d times; a logged rejection needs no inquiry", n)
	}
}

func TestGetPaymentStatus_ThreeDRejectedWithoutMdErrorMessage(t *testing.T) {
	logs := threeDStartLogs()
	logs.objects = map[string]string{"getThreeDSessionResultResponse": threeDResultJSON(t, rejectedThreeDResult(""))}
	p, _ := newStatusTestProvider(t, logs, nil)

	resp, err := getStatus(t, p)
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Status != provider.StatusFailed || resp.Message != "3D Doğrulama Başarısız" {
		t.Errorf("status=%q message=%q, want failed with threeDResultDescription", resp.Status, resp.Message)
	}
}

// Complete3DPayment fails every result other than "0", an empty one included, and never provisions.
func TestGetPaymentStatus_ThreeDResultEmptyIsFailed(t *testing.T) {
	logs := threeDStartLogs()
	logs.objects = map[string]string{"getThreeDSessionResultResponse": `{"mdStatus":"","threeDOperationResult":{"threeDResult":""}}`}
	p, _ := newStatusTestProvider(t, logs, nil)

	resp, err := getStatus(t, p)
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Status != provider.StatusFailed {
		t.Errorf("status = %q, want failed", resp.Status)
	}
}

func TestGetPaymentStatus_ThreeDOpenOrAbandonedIsPending(t *testing.T) {
	p, srv := newStatusTestProvider(t, threeDStartLogs(), nil)

	resp, err := getStatus(t, p)
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Success || resp.Status != provider.StatusPending {
		t.Errorf("success=%v status=%q, want false/pending", resp.Success, resp.Status)
	}
	if resp.ErrorCode != "" {
		t.Errorf("errorCode = %q, want empty: nothing was rejected", resp.ErrorCode)
	}
	if n := srv.callCount(); n != 0 {
		t.Errorf("Paycell called %d times", n)
	}
}

// A successful 3D result with no provision logged may still have been provisioned, because
// logRequest drops write failures. That is unknown, so it must not become pending or failed.
func TestGetPaymentStatus_ThreeDSuccessWithoutProvisionIsUnknown(t *testing.T) {
	logs := threeDStartLogs()
	ok := rejectedThreeDResult("")
	ok.MdStatus = "1"
	ok.ThreeDOperationResult.ThreeDResult = responseCodeSuccess
	logs.objects = map[string]string{"getThreeDSessionResultResponse": threeDResultJSON(t, ok)}
	p, _ := newStatusTestProvider(t, logs, nil)

	if resp, err := getStatus(t, p); err == nil {
		t.Fatalf("want an error, got status %q", resp.Status)
	}
}

func TestGetPaymentStatus_NothingLoggedIsError(t *testing.T) {
	p, _ := newStatusTestProvider(t, &fakeLogReader{}, nil)

	_, err := getStatus(t, p)
	if err == nil {
		t.Fatal("want an error for a payment with no log")
	}
	if !errors.Is(err, provider.ErrLogValueNotFound) {
		t.Errorf("err = %v, want it to wrap ErrLogValueNotFound", err)
	}
}

// A failed query says nothing about the payment; it must not fall through to pending or failed.
func TestGetPaymentStatus_LogQueryFailureIsError(t *testing.T) {
	p, srv := newStatusTestProvider(t, &fakeLogReader{failWith: errors.New("connection refused")}, nil)

	_, err := getStatus(t, p)
	if err == nil {
		t.Fatal("want an error when the log cannot be read")
	}
	if errors.Is(err, provider.ErrLogValueNotFound) {
		t.Errorf("err = %v must not read as not found", err)
	}
	if n := srv.callCount(); n != 0 {
		t.Errorf("Paycell called %d times", n)
	}
}

// After the provision lookup, a failing 3D result, callback or 3D session lookup is just as unknown.
func TestGetPaymentStatus_LaterLookupFailureIsError(t *testing.T) {
	for _, key := range []string{"getThreeDSessionResultResponse", "callback", "getThreeDSessionRequest.merchantCode"} {
		t.Run(key, func(t *testing.T) {
			logs := threeDStartLogs()
			logs.failKeys = map[string]error{key: errors.New("connection refused")}
			p, _ := newStatusTestProvider(t, logs, nil)

			if resp, err := getStatus(t, p); err == nil {
				t.Fatalf("want an error, got status %q", resp.Status)
			}
		})
	}
}

// LogError blanks payment_id on a failed callback, so its row is found only through its request.
// Once that callback got a successful 3D result or sent a provision, money may have moved.
func TestGetPaymentStatus_FailedCallbackRow(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		wantErr  bool
	}{
		{
			name:     "provision sent",
			callback: `{"getThreeDSessionResultResponse":{"threeDOperationResult":{"threeDResult":"0"}},"providerProvisionRequest":{"referenceNumber":"1"}}`,
			wantErr:  true,
		},
		{
			name:     "3D result successful",
			callback: `{"getThreeDSessionResultResponse":{"threeDOperationResult":{"threeDResult":"0"}}}`,
			wantErr:  true,
		},
		{
			name:     "unreadable row",
			callback: `{"providerProvisionRequest":`,
			wantErr:  true,
		},
		{
			// Production row 82878: getThreeDSessionResult failed with EOF, so nothing was provisioned.
			name:     "failed before the 3D result",
			callback: `{"getThreeDSessionResultRequest":{"merchantCode":"9998"}}`,
		},
		{
			name:     "provision logged as null",
			callback: `{"providerProvisionRequest":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := threeDStartLogs()
			logs.callback = tt.callback
			p, srv := newStatusTestProvider(t, logs, nil)

			resp, err := getStatus(t, p)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error, got status %q", resp.Status)
				}
			} else {
				if err != nil {
					t.Fatalf("GetPaymentStatus: %v", err)
				}
				if resp.Status != provider.StatusPending {
					t.Errorf("status = %q, want pending", resp.Status)
				}
			}
			if n := srv.callCount(); n != 0 {
				t.Errorf("Paycell called %d times", n)
			}
		})
	}
}

func TestGetPaymentStatus_UnreadableThreeDResultIsError(t *testing.T) {
	logs := threeDStartLogs()
	logs.objects = map[string]string{"getThreeDSessionResultResponse": `{"threeDOperationResult":`}
	p, _ := newStatusTestProvider(t, logs, nil)

	if resp, err := getStatus(t, p); err == nil {
		t.Fatalf("want an error, got status %q", resp.Status)
	}
}

// provisionedLogs is a payment that reached provisionAll.
func provisionedLogs() *fakeLogReader {
	logs := threeDStartLogs()
	logs.paths["providerProvisionRequest.referenceNumber"] = "00000000000000000042"
	// A rejected result next to a provision cannot come from one callback, but the provision must win.
	logs.objects = map[string]string{"getThreeDSessionResultResponse": `{"threeDOperationResult":{"threeDResult":"10009"}}`}
	return logs
}

func inquireWithProvision(code, description, amount string) *PaycellInquireResponse {
	return &PaycellInquireResponse{
		ResponseHeader: PaycellResponseHeader{TransactionID: "inq1", ResponseCode: responseCodeSuccess, ResponseDescription: "Success"},
		ProvisionList: []PaycellProvisionListItem{
			{ResponseCode: code, ResponseDescription: description, Amount: amount},
		},
	}
}

func TestGetPaymentStatus_ProvisionedPathUnchanged(t *testing.T) {
	tests := []struct {
		name      string
		inquire   *PaycellInquireResponse
		status    provider.PaymentStatus
		success   bool
		errorCode string
		amount    float64
	}{
		{
			name:    "provision successful",
			inquire: inquireWithProvision(responseCodeSuccess, "Islem basarili", "10050"),
			status:  provider.StatusSuccessful, success: true, amount: 100.50,
		},
		{
			name:    "provision declined",
			inquire: inquireWithProvision("4001", "Kart Limiti yetersiz", "10050"),
			status:  provider.StatusFailed, errorCode: "4001",
		},
		{
			name:    "provision still processing",
			inquire: inquireWithProvision(responseCodeProcessing, "Islem devam ediyor", ""),
			status:  provider.StatusPending,
		},
		{
			name: "order not queryable yet",
			inquire: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeOrderNotFound, ResponseDescription: "Order is not found"},
			},
			status: provider.StatusPending, errorCode: responseCodeOrderNotFound,
		},
		{
			name: "header processing",
			inquire: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: responseCodeProcessing},
			},
			status: provider.StatusPending, errorCode: responseCodeProcessing,
		},
		{
			name: "header error",
			inquire: &PaycellInquireResponse{
				ResponseHeader: PaycellResponseHeader{ResponseCode: "1", ResponseDescription: "System error"},
			},
			status: provider.StatusFailed, errorCode: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, srv := newStatusTestProvider(t, provisionedLogs(), tt.inquire)

			resp, err := getStatus(t, p)
			if err != nil {
				t.Fatalf("GetPaymentStatus: %v", err)
			}
			if resp.Status != tt.status || resp.Success != tt.success {
				t.Errorf("status=%q success=%v, want %q/%v", resp.Status, resp.Success, tt.status, tt.success)
			}
			if resp.ErrorCode != tt.errorCode {
				t.Errorf("errorCode = %q, want %q", resp.ErrorCode, tt.errorCode)
			}
			if resp.Amount != tt.amount {
				t.Errorf("amount = %v, want %v", resp.Amount, tt.amount)
			}
			if n := srv.callCount(); n != 1 {
				t.Fatalf("inquireAll called %d times, want 1", n)
			}
			srv.mu.Lock()
			ref := srv.body["originalReferenceNumber"]
			srv.mu.Unlock()
			if ref != "00000000000000000042" {
				t.Errorf("originalReferenceNumber = %v, want the logged provision reference", ref)
			}
		})
	}
}

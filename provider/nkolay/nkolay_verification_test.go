package nkolay

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Public test values from paynkolay.com.tr/entegrasyon (test merchant 273), not GoPay's constants.
const (
	docSxList    = "118591467|W8a1JLU8A5Cw+HfadVcO6HiR/GGGxr0NkWr2OGythr8fo0YWdw70cvnI6oKMqvzra3Qu+Wa5u0NRil9gRdJmjocVNd4XciDwfD9+pkVqDErw7/pVZfpcSO+GePg+ZvcqFbOO5A==|3hJpHVF2cqvcCZ4q6F7rcA=="
	docSecretKey = "_viH5wUS4HiBmmw9uGybN"
)

func sha512Base64(s string) string {
	sum := sha512.Sum512([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestHashV2_MatchesPaymentListDocExample(t *testing.T) {
	// String and digest copied from the PaymentList page of the Nkolay documentation.
	got := hashV2(docSxList, "05.10.2026", "06.10.2026", "", docSecretKey)
	want := "3a1yzE1TS1aUrPpcycyWVqUQhhzFb2YomZbw+W/83MD5/VglsBROO8G4cm2BlvEFTQsE/eYyLbV1viqU8sYkaw=="
	if got != want {
		t.Fatalf("hashV2 = %s, want %s", got, want)
	}
}

// signedCallback returns a 3D result as Nkolay posts it, signed with secretKey.
func signedCallback(secretKey, referenceCode, responseCode string) map[string]string {
	data := map[string]string{
		"MERCHANT_NO":           "400000002",
		"REFERENCE_CODE":        referenceCode,
		"AUTH_CODE":             "S32533320",
		"RESPONSE_CODE":         responseCode,
		"USE_3D":                "true",
		"RND":                   "1645700316156",
		"INSTALLMENT":           "1",
		"AUTHORIZATION_AMOUNT":  "1.00",
		"CURRENCY_CODE":         "TRY",
		"CLIENT_REFERENCE_CODE": "gopay_1",
		"ERROR_CODE":            "",
	}
	// Field order of the PHP example on the Nkolay "Hash Yanıtı" page, written out independently of
	// callbackHashFields so a reordering there fails this test.
	data["hashDataV2"] = sha512Base64(strings.Join([]string{
		data["MERCHANT_NO"], data["REFERENCE_CODE"], data["AUTH_CODE"], data["RESPONSE_CODE"],
		data["USE_3D"], data["RND"], data["INSTALLMENT"], data["AUTHORIZATION_AMOUNT"],
		data["CURRENCY_CODE"], secretKey,
	}, "|"))
	return data
}

func TestValidCallbackHash_SandboxCallback(t *testing.T) {
	// Posted by the Nkolay sandbox to successUrl on 2026-10-06 for a 3D sale of the public test merchant.
	data := map[string]string{
		"MERCHANT_NO":          "400000273",
		"REFERENCE_CODE":       "IKSIRPF1874018",
		"AUTH_CODE":            "S62888",
		"RESPONSE_CODE":        "2",
		"USE_3D":               "true",
		"RND":                  "1791312160942",
		"INSTALLMENT":          "1",
		"AUTHORIZATION_AMOUNT": "10.00",
		"CURRENCY_CODE":        "TRY",
		"hashDataV2":           "pZ+mAwfj5b/sfd4acbGjR7mhqbGuRYEnQbqbN4JxUGESaDn4JF3NdtshvGh9EhB2qpZIwcf1M9XF03ClAdMX+Q==",
	}
	if !(&NkolayProvider{secretKey: docSecretKey}).validCallbackHash(data) {
		t.Fatal("a callback signed by the Nkolay sandbox must validate")
	}
}

func TestValidCallbackHash(t *testing.T) {
	p := &NkolayProvider{secretKey: docSecretKey}

	if !p.validCallbackHash(signedCallback(docSecretKey, "IKSIRPF1", "2")) {
		t.Fatal("expected a correctly signed callback to validate")
	}

	tampered := signedCallback(docSecretKey, "IKSIRPF1", "0")
	tampered["RESPONSE_CODE"] = "2"
	if p.validCallbackHash(tampered) {
		t.Error("a callback whose RESPONSE_CODE was changed after signing must not validate")
	}

	if p.validCallbackHash(signedCallback("other-merchant-key", "IKSIRPF1", "2")) {
		t.Error("a callback signed with another merchant's key must not validate")
	}

	unsigned := signedCallback(docSecretKey, "IKSIRPF1", "2")
	delete(unsigned, "hashDataV2")
	if p.validCallbackHash(unsigned) {
		t.Error("a callback without hashDataV2 must not validate")
	}

	if (&NkolayProvider{}).validCallbackHash(signedCallback("", "IKSIRPF1", "2")) {
		t.Error("a provider without a secret key must not validate anything")
	}
}

// listServer serves PaymentList answers and records the forms it received.
type listServer struct {
	mu    sync.Mutex
	forms []url.Values
	reply func(form url.Values) (int, string)
}

func (s *listServer) calls() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.forms...)
}

func newListProvider(t *testing.T, reply func(form url.Values) (int, string), lookup func(string) (string, error)) (*NkolayProvider, *listServer) {
	t.Helper()
	ls := &listServer{reply: reply}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointPaymentList {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		ls.mu.Lock()
		ls.forms = append(ls.forms, r.PostForm)
		ls.mu.Unlock()
		code, body := ls.reply(r.PostForm)
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	p := &NkolayProvider{
		sxList:          docSxList,
		secretKey:       docSecretKey,
		baseURL:         server.URL,
		clientRefLookup: lookup,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
	}
	return p, ls
}

// listAnswer builds a PaymentList answer in the production shape ({"id":"","result":{...}}).
func listAnswer(rows ...string) string {
	return `{"id":"","result":{"RESPONSE_CODE":"2","RESPONSE_DATA":"Servis dönüşü başarılı","LIST":[` +
		strings.Join(rows, ",") + `],"ERROR_CODE":null,"ERROR_MESSAGE":null},"error":null}`
}

func listRow(referenceCode, clientRefCode, transactionType, status string) string {
	return fmt.Sprintf(`{"REFERENCE_CODE":%q,"CLIENT_REFERENCE_CODE":%q,"TRANSACTION_TYPE":%q,"STATUS":%q,`+
		`"CARD_NUMBER":"428220******8015","CARD_HOLDER_NAME":"Test","TRANSACTION_AMOUNT":"870.00","IS_3D":true,"INSTALLMENT_COUNT":1,"TRX_DATE":"28.07.2026 10:16:13"}`,
		referenceCode, clientRefCode, transactionType, status)
}

// Both answers as the Nkolay sandbox returned them on 2026-10-06.
const (
	noRecordAnswer  = `{"id":"","result":{"RESPONSE_CODE":"0","RESPONSE_DATA":"Listelenecek kayıt bulunamadı.","LIST":null,"ERROR_CODE":"CORE0305","ERROR_MESSAGE":"Listelenecek kayıt bulunamadı."},"error":null}`
	hashErrorAnswer = `{"result":"{\"RESPONSE_CODE\":9,\"ERROR_CODE\":\"\",\"RESPONSE_DATA\":\"API : hashData error :: \",\"sessionId\":null,\"CORE_TRX_ID_RESERVED\":null,\"ERROR_MESSAGE\":null,\"TimeStamp\":null}"}`
)

func lookupReturning(clientRefCode string) func(string) (string, error) {
	return func(string) (string, error) { return clientRefCode, nil }
}

func TestGetPaymentStatus_SendsDocumentedRequest(t *testing.T) {
	created := time.Now().Add(-2 * time.Hour)
	clientRef := fmt.Sprintf("gopay_%d", created.UnixNano())
	var lookedUp string
	lookup := func(ref string) (string, error) { lookedUp = ref; return clientRef, nil }

	p, ls := newListProvider(t, func(url.Values) (int, string) {
		return http.StatusOK, listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"))
	}, lookup)

	resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "IKSIRPF1"})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if lookedUp != "IKSIRPF1" {
		t.Errorf("clientRefCode looked up for %q, want IKSIRPF1", lookedUp)
	}

	calls := ls.calls()
	if len(calls) != 1 {
		t.Fatalf("PaymentList called %d times, want 1", len(calls))
	}
	form := calls[0]
	if form.Get("sx") != docSxList || form.Get("clientRefCode") != clientRef {
		t.Errorf("sx/clientRefCode not sent as expected: clientRefCode=%q", form.Get("clientRefCode"))
	}
	if form.Has("hashData") {
		t.Error("the old SHA-1 hashData must not be sent")
	}
	wantHash := sha512Base64(strings.Join([]string{docSxList, form.Get("startDate"), form.Get("endDate"), clientRef, docSecretKey}, "|"))
	if form.Get("hashDatav2") != wantHash {
		t.Errorf("hashDatav2 = %q, want %q", form.Get("hashDatav2"), wantHash)
	}
	wantStart := created.In(nkolayLocation).AddDate(0, 0, -1).Format("02.01.2006")
	wantEnd := time.Now().In(nkolayLocation).Format("02.01.2006")
	if form.Get("startDate") != wantStart || form.Get("endDate") != wantEnd {
		t.Errorf("window %s..%s, want %s..%s", form.Get("startDate"), form.Get("endDate"), wantStart, wantEnd)
	}

	if resp.Status != provider.StatusSuccessful || !resp.Success {
		t.Errorf("status %s success %v, want successful/true", resp.Status, resp.Success)
	}
	if resp.PaymentID != "IKSIRPF1" || resp.TransactionID != "IKSIRPF1" {
		t.Errorf("paymentId %q transactionId %q", resp.PaymentID, resp.TransactionID)
	}
	if strings.Contains(fmt.Sprint(resp.ProviderResponse), "428220") || strings.Contains(fmt.Sprint(resp.ProviderResponse), "CARD") {
		t.Errorf("card data leaked into the status response: %v", resp.ProviderResponse)
	}
}

func TestGetPaymentStatus_MapsListStatus(t *testing.T) {
	// Started five minutes ago, so the callback URL is still live.
	clientRef := fmt.Sprintf("gopay_%d", time.Now().Add(-5*time.Minute).UnixNano())
	tests := []struct {
		name        string
		answer      string
		wantStatus  provider.PaymentStatus
		wantSuccess bool
	}{
		{"sale succeeded", listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS")), provider.StatusSuccessful, true},
		{"sale failed", listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "ERROR")), provider.StatusFailed, false},
		{"sale started, 3D still possible", listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "NEW")), provider.StatusPending, false},
		{"undocumented status is not a verdict", listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "WAITING")), provider.StatusPending, false},
		{"partial cancel is not read as a cancel", listAnswer(
			listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"),
			listRow("IKSIRPF9", clientRef, "CANCELP", "SUCCESS"),
		), provider.StatusSuccessful, true},
		{"lowercase transaction type", listAnswer(listRow("IKSIRPF1", clientRef, "sales", "SUCCESS")), provider.StatusSuccessful, true},
		{"cancelled after the sale", listAnswer(
			listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"),
			listRow("IKSIRPF9", clientRef, "CANCEL", "SUCCESS"),
		), provider.StatusCancelled, false},
		{"refunded after the sale", listAnswer(
			listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"),
			listRow("IKSIRPF9", clientRef, "REFUND", "SUCCESS"),
		), provider.StatusRefunded, false},
		{"failed cancel leaves the sale successful", listAnswer(
			listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"),
			listRow("IKSIRPF9", clientRef, "CANCEL", "ERROR"),
		), provider.StatusSuccessful, true},
		{"bare documentation shape with numeric code", `{"RESPONSE_CODE":2,"LIST":[` + listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS") + `]}`, provider.StatusSuccessful, true},
		// The sandbox and production answer a clientRefCode query with the whole body as a JSON string.
		{"body JSON-encoded as a string", jsonString(listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"))), provider.StatusSuccessful, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newListProvider(t, func(url.Values) (int, string) { return http.StatusOK, tt.answer }, lookupReturning(clientRef))
			resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "IKSIRPF1"})
			if err != nil {
				t.Fatalf("GetPaymentStatus: %v", err)
			}
			if resp.Status != tt.wantStatus || resp.Success != tt.wantSuccess {
				t.Errorf("status %s success %v, want %s/%v", resp.Status, resp.Success, tt.wantStatus, tt.wantSuccess)
			}
		})
	}
}

func TestGetPaymentStatus_NewAfterCallbackExpiryIsFailed(t *testing.T) {
	old := fmt.Sprintf("gopay_%d", time.Now().Add(-provider.CallbackStateTTL-time.Minute).UnixNano())
	p, _ := newListProvider(t, func(url.Values) (int, string) {
		return http.StatusOK, listAnswer(listRow("IKSIRPF1", old, "SALES", "NEW"))
	}, lookupReturning(old))

	resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "IKSIRPF1"})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	// Left pending, sovtajyeri would keep the auction locked for a payment that can never finish.
	if resp.Status != provider.StatusFailed || resp.Success {
		t.Errorf("status %s success %v, want failed/false", resp.Status, resp.Success)
	}
}

func TestGetPaymentStatus_ErrorsInsteadOfGuessing(t *testing.T) {
	const clientRef = "gopay_1785744586740639757"
	tests := []struct {
		name    string
		code    int
		answer  string
		lookup  func(string) (string, error)
		wantErr string
	}{
		{"no record", http.StatusOK, noRecordAnswer, lookupReturning(clientRef), "Listelenecek kayıt bulunamadı"},
		{"hash rejected, result as a JSON string", http.StatusOK, hashErrorAnswer, lookupReturning(clientRef), "hashData error"},
		{"only another payment's sale", http.StatusOK, listAnswer(listRow("IKSIRPF2", clientRef, "SALES", "SUCCESS")), lookupReturning(clientRef), errPaymentNotListed.Error()},
		{"only a row for another clientRefCode", http.StatusOK, listAnswer(listRow("IKSIRPF1", "gopay_2", "SALES", "SUCCESS")), lookupReturning(clientRef), errPaymentNotListed.Error()},
		{"unknown reference code", http.StatusOK, listAnswer(), func(string) (string, error) { return "", errors.New("nested key not found") }, "no clientRefCode recorded"},
		{"HTTP failure", http.StatusBadGateway, "bad gateway", lookupReturning(clientRef), "HTTP error 502"},
		{"unreadable body", http.StatusOK, "<html>maintenance</html>", lookupReturning(clientRef), "unreadable payment list response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newListProvider(t, func(url.Values) (int, string) { return tt.code, tt.answer }, tt.lookup)
			resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "IKSIRPF1"})
			if err == nil {
				t.Fatalf("expected an error, got status %s", resp.Status)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestGetPaymentStatus_ClientRefCodeSkipsLookup(t *testing.T) {
	const clientRef = "gopay_1785744586740639757"
	lookup := func(string) (string, error) {
		t.Error("lookup must not run for an id that is already a clientRefCode")
		return "", nil
	}
	p, ls := newListProvider(t, func(url.Values) (int, string) {
		return http.StatusOK, listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS"))
	}, lookup)

	resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: clientRef})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Status != provider.StatusSuccessful {
		t.Errorf("status %s, want successful", resp.Status)
	}
	if got := ls.calls()[0].Get("clientRefCode"); got != clientRef {
		t.Errorf("clientRefCode sent %q, want %q", got, clientRef)
	}
}

func TestPaymentListWindow(t *testing.T) {
	now := time.Date(2026, 10, 6, 18, 0, 0, 0, time.UTC) // 06.10.2026 21:00 in Turkey
	format := func(start, end time.Time) string {
		return start.Format("02.01.2006") + ".." + end.Format("02.01.2006")
	}

	tests := []struct {
		name          string
		clientRefCode string
		want          string
	}{
		// 22:30 UTC on 4 October is already 5 October in Turkey; the day before that is the 4th.
		{"Turkish date, one day of slack", fmt.Sprintf("gopay_%d", time.Date(2026, 10, 4, 22, 30, 0, 0, time.UTC).UnixNano()), "04.10.2026..06.10.2026"},
		{"capped at one month", fmt.Sprintf("gopay_%d", time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC).UnixNano()), "31.07.2026..30.08.2026"},
		{"no timestamp falls back to the last month", "order-42", "06.09.2026..06.10.2026"},
		{"future timestamp falls back to the last month", fmt.Sprintf("gopay_%d", now.Add(48*time.Hour).UnixNano()), "06.09.2026..06.10.2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := format(paymentListWindow(tt.clientRefCode, now)); got != tt.want {
				t.Errorf("window %s, want %s", got, tt.want)
			}
		})
	}
}

func TestComplete3DPayment(t *testing.T) {
	const clientRef = "gopay_1785744586740639757"
	state := func() *provider.CallbackState {
		return &provider.CallbackState{PaymentID: "IKSIRPF1", Amount: 100, Currency: "TRY", OriginalCallback: "https://consumer.example/cb"}
	}

	tests := []struct {
		name        string
		data        map[string]string
		listAnswer  string
		listCode    int
		wantStatus  provider.PaymentStatus
		wantErrCode string
		wantListHit bool
	}{
		{
			name:       "signed success is trusted without a list call",
			data:       withStatus(signedCallback(docSecretKey, "IKSIRPF1", "2"), "FAILED"),
			wantStatus: provider.StatusSuccessful,
		},
		{
			name:        "signed failure",
			data:        withField(signedCallback(docSecretKey, "IKSIRPF1", "0"), "ERROR_CODE", "VPS-0001"),
			wantStatus:  provider.StatusFailed,
			wantErrCode: "VPS-0001",
		},
		{
			name:        "status=SUCCESS alone is not a payment",
			data:        map[string]string{"status": "SUCCESS", "REFERENCE_CODE": "IKSIRPF1", "RESPONSE_CODE": "2"},
			listAnswer:  listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "ERROR")),
			wantStatus:  provider.StatusFailed,
			wantListHit: true,
		},
		{
			name:        "RESPONSE_CODE changed after signing",
			data:        withField(signedCallback(docSecretKey, "IKSIRPF1", "0"), "RESPONSE_CODE", "2"),
			listAnswer:  listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "ERROR")),
			wantStatus:  provider.StatusFailed,
			wantListHit: true,
		},
		{
			name:        "another payment's signed success replayed onto this state",
			data:        signedCallback(docSecretKey, "IKSIRPF2", "2"),
			listAnswer:  listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "NEW")),
			wantStatus:  provider.StatusFailed,
			wantListHit: true,
		},
		{
			name:        "unsigned callback of a real payment is settled by the list",
			data:        map[string]string{"status": "SUCCESS"},
			listAnswer:  listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "SUCCESS")),
			wantStatus:  provider.StatusSuccessful,
			wantListHit: true,
		},
		{
			name:        "undocumented list status stays unverified",
			data:        map[string]string{"status": "SUCCESS"},
			listAnswer:  listAnswer(listRow("IKSIRPF1", clientRef, "SALES", "WAITING")),
			wantStatus:  provider.StatusPending,
			wantErrCode: "VERIFICATION_UNAVAILABLE",
			wantListHit: true,
		},
		{
			name:        "list cannot answer: pending, never paid",
			data:        map[string]string{"status": "SUCCESS"},
			listAnswer:  noRecordAnswer,
			wantStatus:  provider.StatusPending,
			wantErrCode: "VERIFICATION_UNAVAILABLE",
			wantListHit: true,
		},
		{
			name:        "list unreachable: pending, never paid",
			data:        map[string]string{"status": "SUCCESS"},
			listCode:    http.StatusServiceUnavailable,
			listAnswer:  "down",
			wantStatus:  provider.StatusPending,
			wantErrCode: "VERIFICATION_UNAVAILABLE",
			wantListHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ls := newListProvider(t, func(url.Values) (int, string) {
				code := tt.listCode
				if code == 0 {
					code = http.StatusOK
				}
				return code, tt.listAnswer
			}, lookupReturning(clientRef))

			resp, err := p.Complete3DPayment(context.Background(), state(), tt.data)
			if err != nil {
				t.Fatalf("Complete3DPayment: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Errorf("status %s, want %s", resp.Status, tt.wantStatus)
			}
			if resp.Success != (tt.wantStatus == provider.StatusSuccessful) {
				t.Errorf("success %v does not match status %s", resp.Success, resp.Status)
			}
			if resp.ErrorCode != tt.wantErrCode {
				t.Errorf("errorCode %q, want %q", resp.ErrorCode, tt.wantErrCode)
			}
			if hit := len(ls.calls()) > 0; hit != tt.wantListHit {
				t.Errorf("PaymentList called: %v, want %v", hit, tt.wantListHit)
			}
			if resp.RedirectURL != "https://consumer.example/cb" || resp.PaymentID != "IKSIRPF1" {
				t.Errorf("redirect %q paymentId %q", resp.RedirectURL, resp.PaymentID)
			}
		})
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestIntegration_PaymentListSandbox runs GetPaymentStatus against the Nkolay sandbox with the public
// test merchant. It needs a 3D sale completed there first:
//
//	NKOLAY_SANDBOX_CLIENT_REF=gopay_... NKOLAY_SANDBOX_REFERENCE_CODE=IKSIRPF... go test -run PaymentListSandbox ./provider/nkolay/
func TestIntegration_PaymentListSandbox(t *testing.T) {
	clientRef, referenceCode := os.Getenv("NKOLAY_SANDBOX_CLIENT_REF"), os.Getenv("NKOLAY_SANDBOX_REFERENCE_CODE")
	if clientRef == "" || referenceCode == "" {
		t.Skip("set NKOLAY_SANDBOX_CLIENT_REF and NKOLAY_SANDBOX_REFERENCE_CODE for a completed sandbox sale")
	}
	p := NewProvider().(*NkolayProvider)
	if err := p.Initialize(map[string]string{"sxList": docSxList, "secretKey": docSecretKey, "environment": "sandbox"}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	p.clientRefLookup = lookupReturning(clientRef)

	resp, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: referenceCode})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if resp.Status != provider.StatusSuccessful || resp.TransactionID != referenceCode {
		t.Errorf("status %s transactionId %q, want successful/%s", resp.Status, resp.TransactionID, referenceCode)
	}
}

func withStatus(data map[string]string, status string) map[string]string {
	return withField(data, "status", status)
}

func withField(data map[string]string, key, value string) map[string]string {
	data[key] = value
	return data
}

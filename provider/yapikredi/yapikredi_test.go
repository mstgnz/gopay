package yapikredi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/provider"
)

// Fixture values: the bank's documented sample merchant and its fixed test ENCKEY, and a sample
// card number from the bank's XML document. Nothing here is a live credential.
const (
	testMID      = "6706598320"
	testTID      = "67005551"
	testPosnetID = "9644"
	testEncKey   = "10,10,10,10,10,10,10,10"
	testPAN      = "4506340000000001"
	testCVV      = "000"
	testCallback = "https://gopay.example/v1/callback/yapikredi?state=42"
	test3DURL    = "https://bank.example/3DSWebService/YKBPaymentService"
)

var testNow = time.Date(2026, 11, 24, 12, 30, 45, 0, time.UTC)

// fakeBank answers the Posnet XML service. reply gets the decoded request and returns the status
// code and body; a nil reply answers 500.
type fakeBank struct {
	mu       sync.Mutex
	requests []posnetRequest
	raw      []string
	headers  []http.Header
	reply    func(req posnetRequest) (int, string)
}

func (b *fakeBank) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	raw := r.PostForm.Get("xmldata")
	var req posnetRequest
	if err := xml.Unmarshal([]byte(raw), &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.raw = append(b.raw, raw)
	b.headers = append(b.headers, r.Header.Clone())
	reply := b.reply
	b.mu.Unlock()

	status, body := http.StatusInternalServerError, "down"
	if reply != nil {
		status, body = reply(req)
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (b *fakeBank) count(pick func(posnetRequest) bool) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, r := range b.requests {
		if pick(r) {
			n++
		}
	}
	return n
}

type logEntry struct {
	key  string
	data map[string]any
}

type harness struct {
	p       *YapiKrediProvider
	bank    *fakeBank
	logs    []logEntry
	states  []provider.CallbackState
	phase   provider.CallbackPhase
	start   provider.LoggedOutcome
	startEr error
	done    provider.LoggedOutcome
	doneErr error
	phaseEr error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{bank: &fakeBank{}, startEr: provider.ErrLogValueNotFound, doneErr: provider.ErrLogValueNotFound}
	srv := httptest.NewServer(h.bank)
	t.Cleanup(srv.Close)

	h.p = &YapiKrediProvider{
		merchantID:   testMID,
		terminalID:   testTID,
		posnetID:     testPosnetID,
		encKey:       testEncKey,
		xmlURL:       srv.URL,
		threeDURL:    test3DURL,
		gopayBaseURL: "https://gopay.example",
		httpClient:   provider.NewProviderHTTPClient(&provider.HTTPClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second}),
		storeCallback: func(ctx context.Context, base string, state provider.CallbackState) (string, error) {
			h.states = append(h.states, state)
			return testCallback, nil
		},
		callbackPhase: func(ctx context.Context, id string) (provider.CallbackPhase, error) { return h.phase, h.phaseEr },
		startOutcome:  func(ctx context.Context, id string) (provider.LoggedOutcome, error) { return h.start, h.startEr },
		completeOutcome: func(ctx context.Context, id string) (provider.LoggedOutcome, error) {
			return h.done, h.doneErr
		},
		recordLog: func(name, key string, data map[string]any, logID int64) error {
			h.logs = append(h.logs, logEntry{key: key, data: data})
			return nil
		},
		now: func() time.Time { return testNow },
	}
	return h
}

func paymentRequest() provider.PaymentRequest {
	return provider.PaymentRequest{
		TenantID:         2,
		LogID:            7,
		Amount:           123.45,
		Currency:         "TRY",
		InstallmentCount: 3,
		CallbackURL:      "https://consumer.example/return",
		Environment:      "sandbox",
		ClientIP:         "203.0.113.5",
		SessionID:        "sess",
		CardInfo: provider.CardInfo{
			CardHolderName: "Ayşe Yılmaz",
			CardNumber:     "4506 3400 0000 0001",
			ExpireMonth:    "2",
			ExpireYear:     "2029",
			CVV:            testCVV,
		},
	}
}

func xmlReply(body string) (int, string) {
	return http.StatusOK, "<?xml version='1.0' encoding='iso-8859-9'?><posnetResponse>" + body + "</posnetResponse>"
}

// assertNoCardData fails when card data or the ENCKEY reached anything GoPay stores or returns.
func (h *harness) assertNoCardData(t *testing.T, extra ...any) {
	t.Helper()
	values := []any{h.logs}
	values = append(values, extra...)
	encoded, err := json.Marshal(fmt.Sprintf("%+v", values))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{testPAN, "4506 3400", testEncKey, "<ccno>", "<cvc>"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("card data or key %q reached the log or the response", secret)
		}
	}
	for _, e := range h.logs {
		for k := range e.data {
			if lk := strings.ToLower(k); strings.Contains(lk, "cvc") || strings.Contains(lk, "ccno") || strings.Contains(lk, "expdate") {
				t.Errorf("log entry %s carries card field %s", e.key, k)
			}
		}
	}
}

func TestNewProviderWiresSeamsAndRegisters(t *testing.T) {
	p, ok := NewProvider().(*YapiKrediProvider)
	if !ok {
		t.Fatal("NewProvider() should return *YapiKrediProvider")
	}
	if p.storeCallback == nil || p.callbackPhase == nil || p.startOutcome == nil || p.completeOutcome == nil || p.recordLog == nil || p.now == nil {
		t.Error("a database seam is not wired")
	}
	if _, err := provider.Get(providerName); err != nil {
		t.Errorf("provider not registered: %v", err)
	}
	if c := p.Clone().(*YapiKrediProvider); c == p {
		t.Error("Clone must return a copy")
	}
}

func TestInitialize(t *testing.T) {
	conf := map[string]string{"merchantId": testMID, "terminalId": "67C49494", "posnetId": testPosnetID, "encKey": testEncKey, "environment": "sandbox"}

	p := NewProvider().(*YapiKrediProvider)
	if err := p.Initialize(conf); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if p.xmlURL != sandboxXMLURL || p.threeDURL != sandboxThreeDURL || p.isProduction || p.httpClient == nil {
		t.Errorf("sandbox setup wrong: %+v", p)
	}

	conf["environment"] = "production"
	if err := p.Initialize(conf); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if p.xmlURL != productionXMLURL || p.threeDURL != productionThreeDURL || !p.isProduction {
		t.Errorf("production setup wrong: %+v", p)
	}

	delete(conf, "encKey")
	if err := p.Initialize(conf); err == nil {
		t.Error("missing encKey accepted")
	}
}

func TestValidateConfig(t *testing.T) {
	p := &YapiKrediProvider{}
	valid := map[string]string{"merchantId": testMID, "terminalId": "67C49494", "posnetId": "1010085994013002", "encKey": testEncKey, "environment": "sandbox"}
	if err := p.ValidateConfig(valid); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	for key, bad := range map[string]string{"merchantId": "67009726", "terminalId": "67C4949", "posnetId": "12345678901234567", "encKey": "short", "environment": "test"} {
		conf := maps.Clone(valid)
		conf[key] = bad
		if err := p.ValidateConfig(conf); err == nil {
			t.Errorf("%s=%q accepted", key, bad)
		}
	}
}

func TestUnsupportedOperations(t *testing.T) {
	p := &YapiKrediProvider{}
	if ok, data, err := p.ValidateWebhook(context.Background(), map[string]string{"a": "b"}, nil); ok || data != nil || err == nil {
		t.Error("webhooks must be refused")
	}
	if resp, err := p.GetInstallmentCount(context.Background(), provider.InstallmentInquireRequest{}); err != nil || len(resp.Installments) != 0 {
		t.Error("installment inquiry should be empty")
	}
	if resp, err := p.GetCommission(context.Background(), provider.CommissionRequest{}); err != nil || resp.Success {
		t.Error("commission inquiry should be empty")
	}
}

func TestPaymentInputValidation(t *testing.T) {
	cases := map[string]func(r *provider.PaymentRequest){
		"tenant":             func(r *provider.PaymentRequest) { r.TenantID = 0 },
		"zero amount":        func(r *provider.PaymentRequest) { r.Amount = 0 },
		"currency":           func(r *provider.PaymentRequest) { r.Currency = "USD" },
		"installments":       func(r *provider.PaymentRequest) { r.InstallmentCount = 100 },
		"card letters":       func(r *provider.PaymentRequest) { r.CardInfo.CardNumber = "4506ABCD00000001" },
		"card short":         func(r *provider.PaymentRequest) { r.CardInfo.CardNumber = "45063400" },
		"cvv":                func(r *provider.PaymentRequest) { r.CardInfo.CVV = "12" },
		"month":              func(r *provider.PaymentRequest) { r.CardInfo.ExpireMonth = "13" },
		"year":               func(r *provider.PaymentRequest) { r.CardInfo.ExpireYear = "202" },
		"year letters":       func(r *provider.PaymentRequest) { r.CardInfo.ExpireYear = "20x9" },
		"negative instalmnt": func(r *provider.PaymentRequest) { r.InstallmentCount = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			req := paymentRequest()
			mutate(&req)
			if _, err := h.p.CreatePayment(context.Background(), req); err == nil {
				t.Error("CreatePayment accepted it")
			}
			if _, err := h.p.Create3DPayment(context.Background(), req); err == nil {
				t.Error("Create3DPayment accepted it")
			}
			if len(h.bank.requests) != 0 {
				t.Error("an invalid request reached the bank")
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	for amount, want := range map[float64]string{123.45: "12345", 0.1: "10", 19.99: "1999", 1000: "100000"} {
		if got, err := kurus(amount); err != nil || got != want {
			t.Errorf("kurus(%v) = %q, %v; want %q", amount, got, err, want)
		}
	}
	for count, want := range map[int]string{0: "00", 1: "00", 2: "02", 12: "12"} {
		if got, err := installmentCode(count); err != nil || got != want {
			t.Errorf("installmentCode(%d) = %q, %v", count, got, err)
		}
	}
	if c, err := posnetCurrency(" try "); err != nil || c != "TL" {
		t.Errorf("posnetCurrency(try) = %q, %v", c, err)
	}
	if _, exp, _, err := cardFields(provider.CardInfo{CardNumber: testPAN, ExpireMonth: "07", ExpireYear: "31", CVV: "123"}); err != nil || exp != "3107" {
		t.Errorf("two-digit year: %q, %v", exp, err)
	}
}

func TestNewIDShapes(t *testing.T) {
	p := &YapiKrediProvider{now: func() time.Time { return testNow }}
	xid, err := p.newID(xidRandomLen)
	if err != nil || len(xid) != xidLength || !strings.HasPrefix(xid, "GP261124123045") || !paymentIDPattern.MatchString(xid) {
		t.Errorf("xid = %q, %v", xid, err)
	}
	orderID, err := p.newID(orderIDRandomLen)
	if err != nil || len(orderID) != orderIDLength {
		t.Errorf("orderID = %q, %v", orderID, err)
	}
	if other, _ := p.newID(xidRandomLen); other == xid {
		t.Error("two ids in the same second collided")
	}
}

func TestCreatePaymentApproved(t *testing.T) {
	h := newHarness(t)
	h.p.logID = 0
	h.bank.reply = func(req posnetRequest) (int, string) {
		return xmlReply(`<approved>1</approved><hostlogkey>019676067890000191</hostlogkey><authCode>760678</authCode>`)
	}

	resp, err := h.p.CreatePayment(context.Background(), paymentRequest())
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if !resp.Success || resp.Status != provider.StatusSuccessful || resp.TransactionID != "019676067890000191" || len(resp.PaymentID) != orderIDLength {
		t.Errorf("response %+v", resp)
	}

	req := h.bank.requests[0]
	if req.MID != testMID || req.TID != testTID || req.Sale == nil {
		t.Fatalf("request %+v", req)
	}
	s := req.Sale
	if s.Amount != "12345" || s.CurrencyCode != "TL" || s.Installment != "03" || s.ExpDate != "2902" || s.CCNo != testPAN || s.CVC != testCVV || s.OrderID != resp.PaymentID {
		t.Errorf("sale block %+v", s)
	}
	hd := h.bank.headers[0]
	if hd.Get("Content-Type") != "application/x-www-form-urlencoded; charset=utf-8" {
		t.Errorf("Content-Type %q", hd.Get("Content-Type"))
	}
	if hd.Get("X-MERCHANT-ID") != testMID || hd.Get("X-TERMINAL-ID") != testTID || hd.Get("X-POSNET-ID") != testPosnetID || len(hd.Get("X-CORRELATION-ID")) != 32 {
		t.Errorf("headers %v", hd)
	}
	h.assertNoCardData(t, resp)
}

func TestCreatePaymentOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   provider.PaymentStatus
		code   string
	}{
		{"declined", 200, `<approved>0</approved><respCode>0051</respCode><respText>RED-YETERSIZ BAKIYE</respText>`, provider.StatusFailed, "0051"},
		{"already used", 200, `<approved>2</approved><respCode>0127</respCode>`, provider.StatusPending, errCodeDuplicateOrder},
		{"no answer", 503, "", provider.StatusPending, errCodeUnverified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.p.logID = 7
			h.bank.reply = func(req posnetRequest) (int, string) {
				if tc.status != 200 {
					return tc.status, "unavailable"
				}
				return xmlReply(tc.body)
			}
			resp, err := h.p.CreatePayment(context.Background(), paymentRequest())
			if err != nil {
				t.Fatalf("CreatePayment: %v", err)
			}
			if resp.Success || resp.Status != tc.want || resp.ErrorCode != tc.code || resp.PaymentID == "" {
				t.Errorf("response %+v", resp)
			}
			h.assertNoCardData(t, resp)
		})
	}
}

func TestCreate3DPayment(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = func(req posnetRequest) (int, string) {
		return xmlReply(`<approved>1</approved><oosRequestDataResponse><data1>D1"&amp;x</data1><data2>CARDBLOCK</data2><sign>SIGN</sign></oosRequestDataResponse>`)
	}

	resp, err := h.p.Create3DPayment(context.Background(), paymentRequest())
	if err != nil {
		t.Fatalf("Create3DPayment: %v", err)
	}
	if !resp.Success || resp.Status != provider.StatusPending || len(resp.PaymentID) != xidLength {
		t.Fatalf("response %+v", resp)
	}

	o := h.bank.requests[0].OOSRequestData
	if o == nil || o.XID != resp.PaymentID || o.PosnetID != testPosnetID || o.Amount != "12345" || o.TranType != "Sale" ||
		o.CardHolderName != "Ayşe Yılmaz" || o.CCNo != testPAN || o.ExpDate != "2902" || o.Installment != "03" {
		t.Errorf("oosRequestData %+v", o)
	}

	if len(h.states) != 1 {
		t.Fatalf("callback states stored: %d", len(h.states))
	}
	st := h.states[0]
	if st.PaymentID != resp.PaymentID || st.TenantID != 2 || st.Amount != 123.45 || st.OriginalCallback != "https://consumer.example/return" ||
		st.Provider != providerName || st.SessionID != "sess" || st.LogID != 7 || st.Installment != 3 {
		t.Errorf("callback state %+v", st)
	}

	for _, want := range []string{
		`action="` + test3DURL + `"`,
		`name="mid" value="` + testMID + `"`,
		`name="posnetID" value="` + testPosnetID + `"`,
		`name="posnetData" value="D1&#34;&amp;x"`,
		`name="digest" value="SIGN"`,
		`name="merchantReturnURL" value="https://gopay.example/v1/callback/yapikredi?state=42"`,
		`name="openANewWindow" value="0"`,
	} {
		if !strings.Contains(resp.HTML, want) {
			t.Errorf("form lacks %s:\n%s", want, resp.HTML)
		}
	}
	h.assertNoCardData(t, resp)
}

func TestCreate3DPaymentRejectedStartStoresNoState(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = func(req posnetRequest) (int, string) {
		return xmlReply(`<approved>0</approved><respCode>0148</respCode><respText>MID,TID,IP HATALI</respText>`)
	}
	resp, err := h.p.Create3DPayment(context.Background(), paymentRequest())
	if err != nil {
		t.Fatalf("Create3DPayment: %v", err)
	}
	if resp.Success || resp.Status != provider.StatusFailed || resp.ErrorCode != "0148" || resp.HTML != "" {
		t.Errorf("response %+v", resp)
	}
	if len(h.states) != 0 {
		t.Error("a rejected start stored a callback state")
	}
}

func TestCreate3DPaymentErrors(t *testing.T) {
	ok := func(req posnetRequest) (int, string) {
		return xmlReply(`<approved>1</approved><oosRequestDataResponse><data1>D</data1><data2>C</data2><sign>S</sign></oosRequestDataResponse>`)
	}

	t.Run("no callback url", func(t *testing.T) {
		h := newHarness(t)
		req := paymentRequest()
		req.CallbackURL = " "
		if _, err := h.p.Create3DPayment(context.Background(), req); err == nil {
			t.Error("missing callback URL accepted")
		}
	})
	t.Run("bank down", func(t *testing.T) {
		h := newHarness(t)
		if _, err := h.p.Create3DPayment(context.Background(), paymentRequest()); err == nil {
			t.Error("transport error not returned")
		}
	})
	t.Run("state store fails", func(t *testing.T) {
		h := newHarness(t)
		h.bank.reply = ok
		h.p.storeCallback = func(context.Context, string, provider.CallbackState) (string, error) {
			return "", errors.New("db down")
		}
		if _, err := h.p.Create3DPayment(context.Background(), paymentRequest()); err == nil {
			t.Error("state store failure not returned")
		}
	})
	t.Run("callback url too long", func(t *testing.T) {
		h := newHarness(t)
		h.bank.reply = ok
		h.p.storeCallback = func(context.Context, string, provider.CallbackState) (string, error) {
			return "https://gopay.example/" + strings.Repeat("a", 300), nil
		}
		if _, err := h.p.Create3DPayment(context.Background(), paymentRequest()); err == nil {
			t.Error("over-long return URL accepted")
		}
	})
}

// bank3D answers resolve and tran like the real service, with MACs computed for the payment.
type bank3D struct {
	xid, amount                 string
	resolveApproved, mdStatus   string
	resolveXID, resolveAmount   string
	breakResolveMAC, resolveDie bool
	tranApproved, tranRespCode  string
	breakTranMAC, tranDie       bool
}

func (b bank3D) reply(req posnetRequest) (int, string) {
	first := firstHash(testEncKey, testTID)
	switch {
	case req.OOSResolve != nil:
		if b.resolveDie {
			return 502, "bad gateway"
		}
		xid, amount := b.xid, b.amount
		if b.resolveXID != "" {
			xid = b.resolveXID
		}
		if b.resolveAmount != "" {
			amount = b.resolveAmount
		}
		mac := resolveResponseMAC(b.mdStatus, xid, amount, "TL", testMID, first)
		if b.breakResolveMAC {
			mac = "forged"
		}
		return xmlReply(fmt.Sprintf(`<approved>%s</approved><respCode>0001</respCode><oosResolveMerchantDataResponse><xid>%s</xid><amount>%s</amount>`+
			`<currency>TL</currency><installment>00</installment><mdStatus>%s</mdStatus><mdErrorMessage>Not authenticated</mdErrorMessage><mac>%s</mac>`+
			`</oosResolveMerchantDataResponse>`, b.resolveApproved, xid, amount, b.mdStatus, mac))
	case req.OOSTran != nil:
		if b.tranDie {
			return 504, "timeout"
		}
		hostLogKey := "0000000002P0806031"
		mac := tranResponseMAC(hostLogKey, b.xid, b.amount, "TL", testMID, first)
		if b.breakTranMAC {
			mac = "forged"
		}
		return xmlReply(fmt.Sprintf(`<approved>%s</approved><respCode>%s</respCode><mac>%s</mac><hostlogkey>%s</hostlogkey><authCode>901477</authCode>`,
			b.tranApproved, b.tranRespCode, mac, hostLogKey))
	}
	return 400, "unexpected request"
}

func callbackState() *provider.CallbackState {
	return &provider.CallbackState{PaymentID: "GP2611241230450ABCDE", Amount: 123.45, Currency: "TRY", LogID: 9, OriginalCallback: "https://consumer.example/return"}
}

func callbackForm() map[string]string {
	return map[string]string{"MerchantPacket": "MP", "BankPacket": "BP", "Sign": "SG", "Xid": "forged", "Amount": "1"}
}

func TestComplete3DPayment(t *testing.T) {
	ok := bank3D{xid: "GP2611241230450ABCDE", amount: "12345", resolveApproved: "1", mdStatus: "1", tranApproved: "1"}

	cases := []struct {
		name     string
		bank     func(b bank3D) bank3D
		form     func(f map[string]string)
		want     provider.PaymentStatus
		code     string
		tranSent bool
	}{
		{name: "approved", want: provider.StatusSuccessful, tranSent: true},
		{name: "already approved", bank: func(b bank3D) bank3D { b.tranApproved = "2"; return b }, want: provider.StatusSuccessful, tranSent: true},
		{name: "field names in another case", form: func(f map[string]string) {
			delete(f, "MerchantPacket")
			delete(f, "BankPacket")
			delete(f, "Sign")
			f["merchantpacket"], f["BANKPACKET"], f["sign"] = "MP", "BP", "SG"
		}, want: provider.StatusSuccessful, tranSent: true},
		{name: "packets missing", form: func(f map[string]string) { delete(f, "BankPacket") }, want: provider.StatusFailed, code: errCodeMissing3DData},
		{name: "3D not authenticated", bank: func(b bank3D) bank3D { b.mdStatus = "0"; return b }, want: provider.StatusFailed, code: errCodeAuthFailed},
		{name: "merchant not 3D", bank: func(b bank3D) bank3D { b.mdStatus = "9"; return b }, want: provider.StatusFailed, code: errCodeAuthFailed},
		{name: "resolve refused", bank: func(b bank3D) bank3D { b.resolveApproved = "0"; return b }, want: provider.StatusFailed, code: "0001"},
		{name: "resolve no answer", bank: func(b bank3D) bank3D { b.resolveDie = true; return b }, want: provider.StatusFailed, code: errCodeResolveFailed},
		{name: "resolve MAC forged", bank: func(b bank3D) bank3D { b.breakResolveMAC = true; return b }, want: provider.StatusFailed, code: errCodeMACMismatch},
		{name: "resolve other xid", bank: func(b bank3D) bank3D { b.resolveXID = "GP2611241230459ZZZZZ"; return b }, want: provider.StatusFailed, code: errCodeDataMismatch},
		{name: "resolve other amount", bank: func(b bank3D) bank3D { b.resolveAmount = "100"; return b }, want: provider.StatusFailed, code: errCodeDataMismatch},
		{name: "finance declined", bank: func(b bank3D) bank3D { b.tranApproved, b.tranRespCode = "0", "0051"; return b }, want: provider.StatusFailed, code: "0051", tranSent: true},
		{name: "finance MAC forged", bank: func(b bank3D) bank3D { b.breakTranMAC = true; return b }, want: provider.StatusPending, code: errCodeUnverified, tranSent: true},
		{name: "finance no answer", bank: func(b bank3D) bank3D { b.tranDie = true; return b }, want: provider.StatusPending, code: errCodeUnverified, tranSent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			b := ok
			if tc.bank != nil {
				b = tc.bank(b)
			}
			h.bank.reply = b.reply
			form := callbackForm()
			if tc.form != nil {
				tc.form(form)
			}

			resp, err := h.p.Complete3DPayment(context.Background(), callbackState(), form)
			if err != nil {
				t.Fatalf("Complete3DPayment: %v", err)
			}
			if resp.Status != tc.want || resp.ErrorCode != tc.code || resp.Success != (tc.want == provider.StatusSuccessful) {
				t.Errorf("got status %s code %q success %v, want %s %q", resp.Status, resp.ErrorCode, resp.Success, tc.want, tc.code)
			}
			if resp.RedirectURL != "https://consumer.example/return" || resp.PaymentID != "GP2611241230450ABCDE" {
				t.Errorf("response %+v", resp)
			}
			if tc.want == provider.StatusSuccessful && resp.TransactionID != "0000000002P0806031" {
				t.Errorf("transaction id %q", resp.TransactionID)
			}
			sent := h.bank.count(func(r posnetRequest) bool { return r.OOSTran != nil }) > 0
			if sent != tc.tranSent {
				t.Errorf("finance call sent = %v, want %v", sent, tc.tranSent)
			}
		})
	}
}

func TestComplete3DPaymentSendsTheMACFromGoPayValues(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = bank3D{xid: "GP2611241230450ABCDE", amount: "12345", resolveApproved: "1", mdStatus: "1", tranApproved: "1"}.reply
	if _, err := h.p.Complete3DPayment(context.Background(), callbackState(), callbackForm()); err != nil {
		t.Fatalf("Complete3DPayment: %v", err)
	}
	want := requestMAC("GP2611241230450ABCDE", "12345", "TL", testMID, firstHash(testEncKey, testTID))
	r := h.bank.requests[0].OOSResolve
	tran := h.bank.requests[1].OOSTran
	if r.MAC != want || r.BankData != "BP" || r.MerchantData != "MP" || r.Sign != "SG" {
		t.Errorf("resolve %+v", r)
	}
	if tran.MAC != want || tran.BankData != "BP" || tran.WPAmount != "0" {
		t.Errorf("tran %+v", tran)
	}
}

func agreementReply(rows ...string) func(posnetRequest) (int, string) {
	return func(req posnetRequest) (int, string) {
		if req.Agreement == nil {
			return 400, "unexpected"
		}
		var b strings.Builder
		for _, r := range rows {
			b.WriteString("<transaction>")
			b.WriteString(r)
			b.WriteString("</transaction>")
		}
		return xmlReply(`<approved>1</approved><transactions>` + b.String() + `</transactions>`)
	}
}

func row(state, txnStatus, hostLogKey string) string {
	return fmt.Sprintf(`<orderID>TDS_GP2611241230450ABCDE</orderID><ccno>4506 34** **** 0001</ccno><amount>123,45</amount>`+
		`<state>%s</state><txnStatus>%s</txnStatus><hostLogKey>%s</hostLogKey>`, state, txnStatus, hostLogKey)
}

func TestGetPaymentStatusFromAgreement(t *testing.T) {
	cases := []struct {
		name string
		rows []string
		want provider.PaymentStatus
	}{
		{"paid", []string{row("Sale", "1", "K1")}, provider.StatusSuccessful},
		{"cancelled", []string{row("Sale", "1", "K1"), row("Sale_Reverse", "1", "K2")}, provider.StatusCancelled},
		{"refunded", []string{row("Sale", "1", "K1"), row("Return", "1", "K2")}, provider.StatusRefunded},
		{"refund reversed", []string{row("Sale", "1", "K1"), row("Return", "1", "K2"), row("Return_Reverse", "1", "K3")}, provider.StatusSuccessful},
		{"failed refund ignored", []string{row("Sale", "1", "K1"), row("Return", "0", "K2")}, provider.StatusSuccessful},
		{"declined sale", []string{row("Sale", "0", "K1")}, provider.StatusFailed},
		{"unknown state", []string{row("Sale", "1", "K1"), row("Mystery", "1", "K2")}, provider.StatusPending},
		// A paid sale must never read failed because the status element was not read.
		{"status element missing", []string{`<state>Sale</state><hostLogKey>K1</hostLogKey>`}, provider.StatusPending},
		{"status value unknown", []string{row("Sale", "9", "K1")}, provider.StatusPending},
		{"status spelled txstatus", []string{`<state>Sale</state><txstatus>1</txstatus><hostLogKey>K1</hostLogKey>`}, provider.StatusSuccessful},
		{"status spelled txStatus", []string{`<state>Sale</state><txStatus>1</txStatus><hostLogKey>K1</hostLogKey>`}, provider.StatusSuccessful},
		{"reversal without status", []string{row("Sale", "1", "K1"), `<state>Sale_Reverse</state><hostLogKey>K2</hostLogKey>`}, provider.StatusPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.bank.reply = agreementReply(tc.rows...)
			resp, err := h.p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "GP2611241230450ABCDE"})
			if err != nil {
				t.Fatalf("GetPaymentStatus: %v", err)
			}
			if resp.Status != tc.want || resp.Success != (tc.want == provider.StatusSuccessful) {
				t.Errorf("status %s, want %s", resp.Status, tc.want)
			}
			if got := h.bank.requests[0].Agreement.OrderID; got != "TDS_GP2611241230450ABCDE" {
				t.Errorf("agreement orderID %q", got)
			}
			encoded, _ := json.Marshal(resp)
			if strings.Contains(string(encoded), "4506 34") {
				t.Error("masked card number reached the response")
			}
		})
	}
}

func TestGetPaymentStatusNon3DUsesOrderIDAsIs(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = agreementReply(row("Sale", "1", "K1"))
	id := "GP261124123045ABCDEFGHIJ"
	if _, err := h.p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: id}); err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if got := h.bank.requests[0].Agreement.OrderID; got != id {
		t.Errorf("agreement orderID %q", got)
	}
}

func noRecord(req posnetRequest) (int, string) {
	return xmlReply(`<approved>0</approved><respCode>E219</respCode><respText>KAYIT BULUNAMADI</respText>`)
}

func TestGetPaymentStatusWithoutBankRecord(t *testing.T) {
	failedLog := provider.LoggedOutcome{Status: "failed", ErrorCode: errCodeAuthFailed, Message: "Not authenticated"}
	cases := []struct {
		name    string
		setup   func(h *harness)
		want    provider.PaymentStatus
		code    string
		wantErr bool
	}{
		{name: "page still open", setup: func(h *harness) { h.phase = provider.CallbackOpen }, want: provider.StatusPending},
		// After retention the callbacks rows are gone; the logged callback still decides.
		{name: "closed, callback logged failed", setup: func(h *harness) {
			h.phase, h.done, h.doneErr = provider.CallbackClosed, failedLog, nil
		}, want: provider.StatusFailed, code: errCodeAuthFailed},
		{name: "closed, callback logged successful", setup: func(h *harness) {
			h.phase, h.done, h.doneErr = provider.CallbackClosed, provider.LoggedOutcome{Status: "successful"}, nil
		}, wantErr: true},
		{name: "closed, callback lookup fails", setup: func(h *harness) {
			h.phase, h.doneErr = provider.CallbackClosed, errors.New("db down")
		}, wantErr: true},
		{name: "session expired", setup: func(h *harness) { h.phase = provider.CallbackClosed }, want: provider.StatusFailed, code: errCodeSessionExpired},
		{name: "start rejected", setup: func(h *harness) {
			h.phase, h.start, h.startEr = provider.CallbackClosed, provider.LoggedOutcome{Status: "failed", ErrorCode: "0148"}, nil
		}, want: provider.StatusFailed, code: "0148"},
		{name: "start lookup fails", setup: func(h *harness) { h.phase, h.startEr = provider.CallbackClosed, errors.New("db down") }, wantErr: true},
		{name: "callback logged failed", setup: func(h *harness) {
			h.phase, h.done, h.doneErr = provider.CallbackAccepted, failedLog, nil
		}, want: provider.StatusFailed, code: errCodeAuthFailed},
		{name: "callback logged pending", setup: func(h *harness) {
			h.phase, h.done, h.doneErr = provider.CallbackAccepted, provider.LoggedOutcome{Status: "pending"}, nil
		}, wantErr: true},
		{name: "callback not logged", setup: func(h *harness) { h.phase = provider.CallbackAccepted }, wantErr: true},
		{name: "callback lookup fails", setup: func(h *harness) { h.phase, h.doneErr = provider.CallbackAccepted, errors.New("db down") }, wantErr: true},
		{name: "phase lookup fails", setup: func(h *harness) { h.phaseEr = errors.New("db down") }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.bank.reply = noRecord
			tc.setup(h)
			resp, err := h.p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "GP2611241230450ABCDE"})
			if tc.wantErr {
				if err == nil {
					t.Errorf("want an error, got %+v", resp)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPaymentStatus: %v", err)
			}
			if resp.Status != tc.want || resp.ErrorCode != tc.code || resp.Success {
				t.Errorf("got %s %q, want %s %q", resp.Status, resp.ErrorCode, tc.want, tc.code)
			}
		})
	}
}

func TestGetPaymentStatusErrors(t *testing.T) {
	cases := map[string]struct {
		id    string
		reply func(posnetRequest) (int, string)
	}{
		"non-3D without record": {"GP261124123045ABCDEFGHIJ", noRecord},
		"bad id length":         {"GP123", agreementReply()},
		"bad id characters":     {"GP2611241230450ABC<E", agreementReply()},
		"bank down":             {"GP2611241230450ABCDE", nil},
		"other refusal": {"GP2611241230450ABCDE", func(posnetRequest) (int, string) {
			return xmlReply(`<approved>0</approved><respCode>0148</respCode>`)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.bank.reply = tc.reply
			if resp, err := h.p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: tc.id}); err == nil {
				t.Errorf("want an error, got %+v", resp)
			}
		})
	}
}

func TestGetPaymentStatusEmptyTransactionListIsNoRecord(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = agreementReply()
	h.phase = provider.CallbackOpen
	resp, err := h.p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "GP2611241230450ABCDE"})
	if err != nil || resp.Status != provider.StatusPending {
		t.Errorf("got %+v, %v", resp, err)
	}
}

// operationBank answers agreement with rows and reverse/return with the given body.
func operationBank(rows []string, opStatus int, opBody string) func(posnetRequest) (int, string) {
	agreement := agreementReply(rows...)
	return func(req posnetRequest) (int, string) {
		if req.Agreement != nil {
			return agreement(req)
		}
		if opStatus != 200 {
			return opStatus, "unavailable"
		}
		return xmlReply(opBody)
	}
}

func TestCancelPayment(t *testing.T) {
	paid := []string{row("Sale", "1", "SALEKEY")}
	cases := []struct {
		name     string
		rows     []string
		opStatus int
		opBody   string
		success  bool
		status   provider.PaymentStatus
		code     string
		reversed bool
		wantErr  bool
	}{
		{name: "cancelled", rows: paid, opStatus: 200, opBody: `<approved>1</approved><hostlogkey>REVKEY</hostlogkey>`, success: true, status: provider.StatusCancelled, reversed: true},
		{name: "day closed", rows: paid, opStatus: 200, opBody: `<approved>0</approved><respCode>0211</respCode><respText>GRUP KAPAMA YAPILMIS</respText>`, status: provider.StatusSuccessful, code: "0211", reversed: true},
		{name: "already cancelled", rows: []string{row("Sale", "1", "SALEKEY"), row("Sale_Reverse", "1", "R")}, success: true, status: provider.StatusCancelled},
		{name: "refunded", rows: []string{row("Sale", "1", "SALEKEY"), row("Return", "1", "R")}, status: provider.StatusRefunded, code: errCodeNotCancellable},
		{name: "no answer", rows: paid, opStatus: 504, reversed: true, wantErr: true},
		{name: "no record", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.p.logID = 0
			if tc.rows == nil {
				h.bank.reply = noRecord
			} else {
				h.bank.reply = operationBank(tc.rows, tc.opStatus, tc.opBody)
			}
			resp, err := h.p.CancelPayment(context.Background(), provider.CancelRequest{PaymentID: "GP2611241230450ABCDE", LogID: 5})
			reverses := h.bank.count(func(r posnetRequest) bool { return r.Reverse != nil })
			if (reverses > 0) != tc.reversed {
				t.Errorf("reverse sent = %d, want %v", reverses, tc.reversed)
			}
			if tc.wantErr {
				if err == nil {
					t.Errorf("want an error, got %+v", resp)
				}
				return
			}
			if err != nil {
				t.Fatalf("CancelPayment: %v", err)
			}
			if resp.Success != tc.success || resp.Status != tc.status || resp.ErrorCode != tc.code {
				t.Errorf("got success %v status %s code %q", resp.Success, resp.Status, resp.ErrorCode)
			}
			if tc.reversed {
				rev := h.bank.requests[1].Reverse
				if rev.Transaction != "sale" || rev.HostLogKey != "SALEKEY" {
					t.Errorf("reverse %+v", rev)
				}
			}
		})
	}
}

func TestRefundPayment(t *testing.T) {
	paid := []string{row("Sale", "1", "SALEKEY")}
	cases := []struct {
		name     string
		req      provider.RefundRequest
		rows     []string
		opStatus int
		opBody   string
		success  bool
		code     string
		returned bool
		wantErr  bool
	}{
		{name: "refunded", req: provider.RefundRequest{RefundAmount: 50.5}, rows: paid, opStatus: 200, opBody: `<approved>1</approved><hostlogkey>RETKEY</hostlogkey>`, success: true, returned: true},
		{name: "second partial refund", req: provider.RefundRequest{RefundAmount: 10}, rows: []string{row("Sale", "1", "SALEKEY"), row("Return", "1", "R")}, opStatus: 200, opBody: `<approved>1</approved>`, success: true, returned: true},
		{name: "not settled yet", req: provider.RefundRequest{RefundAmount: 50.5}, rows: paid, opStatus: 200, opBody: `<approved>0</approved><respCode>0411</respCode>`, code: "0411", returned: true},
		{name: "cancelled payment", req: provider.RefundRequest{RefundAmount: 1}, rows: []string{row("Sale", "1", "SALEKEY"), row("Sale_Reverse", "1", "R")}, code: errCodeNotRefundable},
		{name: "zero amount", req: provider.RefundRequest{}, rows: paid, wantErr: true},
		{name: "foreign currency", req: provider.RefundRequest{RefundAmount: 1, Currency: "EUR"}, rows: paid, wantErr: true},
		{name: "no answer", req: provider.RefundRequest{RefundAmount: 1}, rows: paid, opStatus: 504, returned: true, wantErr: true},
		{name: "no record", req: provider.RefundRequest{RefundAmount: 1}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.rows == nil {
				h.bank.reply = noRecord
			} else {
				h.bank.reply = operationBank(tc.rows, tc.opStatus, tc.opBody)
			}
			tc.req.PaymentID = "GP2611241230450ABCDE"
			resp, err := h.p.RefundPayment(context.Background(), tc.req)
			returns := h.bank.count(func(r posnetRequest) bool { return r.Return != nil })
			if (returns > 0) != tc.returned {
				t.Errorf("return sent = %d, want %v", returns, tc.returned)
			}
			if tc.wantErr {
				if err == nil {
					t.Errorf("want an error, got %+v", resp)
				}
				return
			}
			if err != nil {
				t.Fatalf("RefundPayment: %v", err)
			}
			if resp.Success != tc.success || resp.ErrorCode != tc.code {
				t.Errorf("got success %v code %q", resp.Success, resp.ErrorCode)
			}
			if tc.returned {
				ret := h.bank.requests[1].Return
				if ret.HostLogKey != "SALEKEY" || ret.CurrencyCode != "TL" {
					t.Errorf("return %+v", ret)
				}
			}
		})
	}
}

// The logged copy of the real 3D form must lose posnetData2 and keep the rest.
func TestThreeDFormIsSanitizedForTheLog(t *testing.T) {
	h := newHarness(t)
	form := h.p.threeDForm(&oosRequestResult{Data1: "ORDERBLOCK", Data2: "CARDBLOCK", Sign: "SIGN"}, testCallback)

	logged, err := json.Marshal(postgres.SanitizeForLog(map[string]any{"html": form}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(logged), "CARDBLOCK") {
		t.Errorf("posnetData2 survived in the log: %s", logged)
	}
	for _, kept := range []string{"ORDERBLOCK", "SIGN", "state=42"} {
		if !strings.Contains(string(logged), kept) {
			t.Errorf("%s was removed from the logged form", kept)
		}
	}
}

// A schema rejection echoes the rejected value, which can be a card field.
func TestBankTextNeverCarriesCardData(t *testing.T) {
	h := newHarness(t)
	h.bank.reply = func(req posnetRequest) (int, string) {
		return xmlReply(`<approved>0</approved><respCode>0002</respCode><respText>XNIException: :::::1:261:cvc-datatype-valid.1.2.1: '1234' değeri 'integer' için geçerli değil.</respText>`)
	}
	resp, err := h.p.CreatePayment(context.Background(), paymentRequest())
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if strings.Contains(resp.Message, "1234") || resp.ErrorCode != "0002" {
		t.Errorf("message %q code %q", resp.Message, resp.ErrorCode)
	}

	for in, want := range map[string]string{
		"KART 4506349043174632 RED":  "KART **** RED",
		"MID,TID,IP HATALI: 1.2.3.4": "MID,TID,IP HATALI: 1.2.3.4",
		"RED-YETERSIZ BAKIYE 0051":   "RED-YETERSIZ BAKIYE 0051",
	} {
		if got := safeBankText(in); got != want {
			t.Errorf("safeBankText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRecordSkipsWithoutLogRowAndSurvivesWriteErrors(t *testing.T) {
	h := newHarness(t)
	h.p.logID = 0
	h.p.record("k", map[string]any{"a": 1})
	if len(h.logs) != 0 {
		t.Error("recorded without a log row")
	}
	h.p.logID = 3
	h.p.recordLog = func(string, string, map[string]any, int64) error { return errors.New("db down") }
	h.p.record("k", map[string]any{"a": 1}) // must not panic
}

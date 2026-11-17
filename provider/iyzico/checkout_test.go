package iyzico

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func checkoutRequest() provider.HostedCheckoutRequest {
	return provider.HostedCheckoutRequest{
		ConversationID: "conv-1", BasketID: "B-1", Price: "450.50", Currency: "TRY",
		CallbackURL: "https://tenant.example/odeme/sonuc",
		Buyer: provider.CheckoutBuyer{ID: "u-7", Name: "Ada", Surname: "Kaya", IdentityNumber: "12345678901",
			Email: "ada@example.com", GSMNumber: "+905551112233", Address: "Moda Cd. 1", City: "Istanbul",
			Country: "Turkey", IP: "85.34.78.112"},
		BillingAddress: provider.CheckoutAddress{ContactName: "Ada Kaya", Address: "Moda Cd. 1", City: "Istanbul", Country: "Turkey"},
		Items: []provider.CheckoutItem{{ID: "BK-9", Name: "Saha 1 20:00", Category: "Saha kiralama", ItemType: "VIRTUAL",
			Price: "450.50", SubMerchantKey: "smk-1", SubMerchantPrice: "405.45"}},
	}
}

func TestStartHostedCheckoutSendsTheBasketExactly(t *testing.T) {
	c := newClient(testAPIKey, testSecretKey, "http://unused")
	answer := mustJSON(t, map[string]any{"status": "success", "conversationId": "conv-1", "token": "tok-1",
		"paymentPageUrl": "https://sandbox-cpp.iyzipay.com?token=tok-1", "tokenExpireTime": 1800,
		"signature": c.responseSignature("conv-1", "tok-1")})
	p, fake := newTestProvider(t, http.StatusOK, answer)

	got, err := p.StartHostedCheckout(context.Background(), checkoutRequest())
	if err != nil {
		t.Fatalf("StartHostedCheckout: %v", err)
	}
	want := &provider.HostedCheckoutSession{Token: "tok-1", PaymentPageURL: "https://sandbox-cpp.iyzipay.com?token=tok-1",
		TokenExpireSeconds: 1800, ConversationID: "conv-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("session = %+v, want %+v", got, want)
	}

	sent := fake.last()
	if sent.path != pathCFInitialize {
		t.Errorf("path = %s", sent.path)
	}
	for field, want := range map[string]any{"price": "450.50", "paidPrice": "450.50", "currency": "TRY",
		"basketId": "B-1", "paymentGroup": "PRODUCT", "locale": "tr", "conversationId": "conv-1",
		"callbackUrl": "https://tenant.example/odeme/sonuc"} {
		if sent.body[field] != want {
			t.Errorf("%s = %v, want %v", field, sent.body[field], want)
		}
	}
	if !reflect.DeepEqual(sent.body["enabledInstallments"], []any{float64(1)}) {
		t.Errorf("enabledInstallments = %v", sent.body["enabledInstallments"])
	}
	if _, ok := sent.body["shippingAddress"]; ok {
		t.Error("a VIRTUAL basket must not carry a shipping address")
	}
	buyer := sent.body["buyer"].(map[string]any)
	if buyer["identityNumber"] != "12345678901" || buyer["ip"] != "85.34.78.112" || buyer["registrationAddress"] != "Moda Cd. 1" {
		t.Errorf("buyer = %v", buyer)
	}
	item := sent.body["basketItems"].([]any)[0].(map[string]any)
	for field, want := range map[string]any{"price": "450.50", "subMerchantKey": "smk-1", "subMerchantPrice": "405.45",
		"itemType": "VIRTUAL", "category1": "Saha kiralama"} {
		if item[field] != want {
			t.Errorf("item %s = %v, want %v", field, item[field], want)
		}
	}
}

func TestStartHostedCheckoutDefaultsAndShipping(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","token":"t","paymentPageUrl":"https://p"}`)
	r := checkoutRequest()
	r.ConversationID, r.Buyer.Country, r.BillingAddress.Country = "", "", ""
	r.ShippingAddress = &provider.CheckoutAddress{ContactName: "Ada", Address: "A", City: "Istanbul"}

	got, err := p.StartHostedCheckout(context.Background(), r)
	if err != nil {
		t.Fatalf("StartHostedCheckout: %v", err)
	}
	sent := fake.last().body
	if sent["conversationId"] == "" || sent["conversationId"] != got.ConversationID {
		t.Errorf("a generated conversationId must be sent and returned: %v vs %q", sent["conversationId"], got.ConversationID)
	}
	if sent["buyer"].(map[string]any)["country"] != "Turkey" || sent["billingAddress"].(map[string]any)["country"] != "Turkey" {
		t.Error("country must default to Turkey")
	}
	if sent["shippingAddress"].(map[string]any)["city"] != "Istanbul" {
		t.Error("a given shipping address must be sent")
	}
}

func TestStartHostedCheckoutRefusesUntrustedAnswers(t *testing.T) {
	cases := map[string]string{
		"no token":      `{"status":"success","paymentPageUrl":"https://p"}`,
		"no page":       `{"status":"success","token":"t"}`,
		"bad signature": `{"status":"success","conversationId":"conv-1","token":"t","paymentPageUrl":"https://evil","signature":"00"}`,
	}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := newTestProvider(t, http.StatusOK, answer)
			if _, err := p.StartHostedCheckout(context.Background(), checkoutRequest()); !errors.Is(err, provider.ErrProviderUnavailable) {
				t.Fatalf("want unavailable, got %v", err)
			}
		})
	}
}

func TestStartHostedCheckoutPassesRefusalThrough(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"5062","errorMessage":"Total item price must be equal to price"}`)
	_, err := p.StartHostedCheckout(context.Background(), checkoutRequest())
	var refusal *provider.ProviderRefusal
	if !errors.As(err, &refusal) || refusal.Code != "5062" {
		t.Fatalf("want refusal 5062, got %v", err)
	}
}

// retrieve builds a CF retrieve answer; signed adds iyzico's signature over it.
func retrieve(t *testing.T, fields map[string]any, signed bool) string {
	t.Helper()
	a := map[string]any{"status": "success", "paymentStatus": "SUCCESS", "paymentId": "12345", "currency": "TRY",
		"basketId": "B-1", "conversationId": "tok-1", "paidPrice": 450.5, "price": 450.50000000, "token": "tok-1",
		"fraudStatus": 1, "itemTransactions": []any{map[string]any{"itemId": "BK-9", "paymentTransactionId": "777",
			"transactionStatus": 1, "price": 450.5, "paidPrice": 450.5, "subMerchantKey": "smk-1", "subMerchantPrice": 405.45}}}
	for k, v := range fields {
		if v == nil {
			delete(a, k)
		} else {
			a[k] = v
		}
	}
	if signed {
		c := newClient(testAPIKey, testSecretKey, "http://unused")
		str := func(k string) string {
			v, ok := a[k]
			if !ok {
				return ""
			}
			n, err := normaliseAmount(mustJSON(t, v))
			if k == "paidPrice" || k == "price" {
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
			s, _ := v.(string)
			return s
		}
		a["signature"] = c.responseSignature(str("paymentStatus"), str("paymentId"), str("currency"), str("basketId"),
			str("conversationId"), str("paidPrice"), str("price"), str("token"))
	}
	return mustJSON(t, a)
}

func TestHostedCheckoutResultOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		fields   map[string]any
		signed   bool
		outcome  provider.PaymentOutcome
		readable bool
		code     string
	}{
		{"paid", nil, true, provider.OutcomePaid, true, ""},
		{"in fraud review", map[string]any{"fraudStatus": 0}, true, provider.OutcomeInReview, true, ""},
		{"fraud rejected", map[string]any{"fraudStatus": -1}, true, provider.OutcomeDeclined, true, ""},
		{"declined", map[string]any{"paymentStatus": "FAILURE"}, true, provider.OutcomeDeclined, true, ""},
		{"3DS still open", map[string]any{"paymentStatus": "INIT_THREEDS", "fraudStatus": nil}, false, provider.OutcomeNoVerdict, true, ""},
		{"callback 3DS", map[string]any{"paymentStatus": "CALLBACK_THREEDS"}, false, provider.OutcomeNoVerdict, true, ""},
		{"success without fraud status", map[string]any{"fraudStatus": nil}, true, provider.OutcomeNoVerdict, false, codeUnreadableResult},
		{"success without items", map[string]any{"itemTransactions": []any{}}, true, provider.OutcomeNoVerdict, false, codeUnreadableResult},
		{"item without transaction id", map[string]any{"itemTransactions": []any{map[string]any{"itemId": "BK-9", "paidPrice": 1}}}, true, provider.OutcomeNoVerdict, false, codeUnreadableResult},
		{"success without payment id", map[string]any{"paymentId": nil}, true, provider.OutcomeNoVerdict, false, codeUnreadableResult},
		{"paid but unsigned", nil, false, provider.OutcomeNoVerdict, false, codeSignatureMismatch},
		{"paid with a wrong signature", map[string]any{"signature": "deadbeef"}, false, provider.OutcomeNoVerdict, false, codeSignatureMismatch},
		{"declined but unsigned", map[string]any{"paymentStatus": "FAILURE"}, false, provider.OutcomeNoVerdict, false, codeSignatureMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, fake := newTestProvider(t, http.StatusOK, retrieve(t, tc.fields, tc.signed))
			got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"})
			if err != nil {
				t.Fatalf("HostedCheckoutResult: %v", err)
			}
			if got.Outcome != tc.outcome || got.Readable != tc.readable || got.ErrorCode != tc.code {
				t.Errorf("outcome=%s readable=%v code=%q, want %s %v %q", got.Outcome, got.Readable, got.ErrorCode, tc.outcome, tc.readable, tc.code)
			}
			sent := fake.last()
			if sent.path != pathCFRetrieve || sent.body["token"] != "tok-1" || sent.body["conversationId"] != "tok-1" {
				t.Errorf("sent %s %v", sent.path, sent.body)
			}
		})
	}
}

// Most bookings are whole lira; iyzico signs 450.00000000 as "450" (docs: "Trailing Zero").
func TestHostedCheckoutResultWholeLiraIsSignedWithoutFraction(t *testing.T) {
	whole := map[string]any{"price": 450.00000000, "paidPrice": 450.0, "itemTransactions": []any{map[string]any{
		"itemId": "BK-9", "paymentTransactionId": "777", "transactionStatus": 1, "price": 450.0, "paidPrice": 450.0}}}
	p, _ := newTestProvider(t, http.StatusOK, retrieve(t, whole, true))
	got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"})
	if err != nil || got.Outcome != provider.OutcomePaid || got.PaidPrice != "450" {
		t.Fatalf("got %+v %v", got, err)
	}
	c := newClient(testAPIKey, testSecretKey, "http://unused")
	answer := mustJSON(t, map[string]any{"status": "success", "paymentStatus": "SUCCESS", "paymentId": "12345", "currency": "TRY",
		"basketId": "B-1", "conversationId": "tok-1", "paidPrice": 450.0, "price": 450.0, "token": "tok-1", "fraudStatus": 1,
		"itemTransactions": []any{map[string]any{"paymentTransactionId": "777", "paidPrice": 450.0}},
		"signature":        c.responseSignature("SUCCESS", "12345", "TRY", "B-1", "tok-1", "450.0", "450.0", "tok-1")})
	p, _ = newTestProvider(t, http.StatusOK, answer)
	if got, _ := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"}); got.Outcome != provider.OutcomeNoVerdict {
		t.Errorf("a signature over un-normalised amounts must not match, got %s", got.Outcome)
	}
}

func TestHostedCheckoutResultCarriesExactAmountsAndItems(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, retrieve(t, nil, true))
	got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.PaidPrice != "450.5" || got.Price != "450.5" || got.PaymentID != "12345" || got.BasketID != "B-1" || got.Currency != "TRY" {
		t.Errorf("result = %+v", got)
	}
	if got.FraudStatus == nil || *got.FraudStatus != 1 {
		t.Errorf("fraudStatus = %v", got.FraudStatus)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	it := got.Items[0]
	if it.PaymentTransactionID != "777" || it.PaidPrice != "450.5" || it.SubMerchantPrice != "405.45" ||
		it.SubMerchantKey != "smk-1" || it.TransactionStatus == nil || *it.TransactionStatus != 1 {
		t.Errorf("item = %+v", it)
	}
}

func TestHostedCheckoutResultErrorAnswers(t *testing.T) {
	t.Run("error answer is no verdict", func(t *testing.T) {
		p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"5129","errorMessage":"token not found"}`)
		got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "x", ConversationID: "c"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != provider.OutcomeNoVerdict || got.Readable || got.ErrorCode != "5129" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("declined card on an error answer", func(t *testing.T) {
		p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","paymentStatus":"FAILURE","errorCode":"10051","errorMessage":"Kart limiti yetersiz"}`)
		got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != provider.OutcomeDeclined || !got.Readable || got.ErrorCode != "10051" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("a signature on an error answer must match", func(t *testing.T) {
		declined := map[string]any{"status": "failure", "paymentStatus": "FAILURE", "errorCode": "10051", "fraudStatus": nil}
		p, _ := newTestProvider(t, http.StatusOK, retrieve(t, declined, true))
		got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"})
		if err != nil || got.Outcome != provider.OutcomeDeclined {
			t.Fatalf("signed decline: got %+v %v", got, err)
		}
		declined["signature"] = "deadbeef"
		p, _ = newTestProvider(t, http.StatusOK, retrieve(t, declined, false))
		got, err = p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "tok-1"})
		if err != nil || got.Outcome != provider.OutcomeNoVerdict || got.Readable || got.ErrorCode != codeSignatureMismatch {
			t.Fatalf("wrongly signed decline: got %+v %v", got, err)
		}
	})
	t.Run("transport failure is an error, never a decline", func(t *testing.T) {
		p, _ := newTestProvider(t, http.StatusBadGateway, `bad gateway`)
		got, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: "x"})
		if !errors.Is(err, provider.ErrProviderUnavailable) || got != nil {
			t.Fatalf("want unavailable and no result, got %+v %v", got, err)
		}
	})
}

func TestCancelHostedPayment(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","paymentId":"12345","price":450.50000000,"currency":"TRY","conversationId":"c-1"}`)
	got, err := p.CancelHostedPayment(context.Background(), provider.HostedCancelRequest{PaymentID: "12345", IP: "85.34.78.112",
		ConversationID: "c-1", Reason: "BUYER_REQUEST", Description: "customer cancelled"})
	if err != nil {
		t.Fatal(err)
	}
	if *got != (provider.HostedCancelResult{PaymentID: "12345", Price: "450.5", Currency: "TRY", ConversationID: "c-1"}) {
		t.Errorf("result = %+v", got)
	}
	sent := fake.last()
	want := map[string]any{"locale": "tr", "conversationId": "c-1", "paymentId": "12345", "ip": "85.34.78.112",
		"reason": "BUYER_REQUEST", "description": "customer cancelled"}
	if sent.path != pathCancel || !reflect.DeepEqual(sent.body, want) {
		t.Errorf("sent %s %v", sent.path, sent.body)
	}
}

func TestCancelHostedPaymentRefusal(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"5086","errorMessage":"Payment is not for this merchant"}`)
	_, err := p.CancelHostedPayment(context.Background(), provider.HostedCancelRequest{PaymentID: "1", IP: "1.1.1.1"})
	var refusal *provider.ProviderRefusal
	if !errors.As(err, &refusal) || refusal.Code != "5086" {
		t.Fatalf("want refusal, got %v", err)
	}
}

func refundAnswerJSON(t *testing.T, signed bool) string {
	a := map[string]any{"status": "success", "paymentId": "12345", "paymentTransactionId": "777", "price": 100.0,
		"currency": "TRY", "conversationId": "c-2"}
	if signed {
		a["signature"] = newClient(testAPIKey, testSecretKey, "http://unused").responseSignature("12345", "100", "TRY", "c-2")
	} else {
		a["signature"] = "0bad"
	}
	return mustJSON(t, a)
}

func TestRefundHostedItem(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, refundAnswerJSON(t, true))
	got, err := p.RefundHostedItem(context.Background(), provider.HostedRefundRequest{PaymentTransactionID: "777",
		Price: "100.00", Currency: "TRY", IP: "85.34.78.112", ConversationID: "c-2"})
	if err != nil {
		t.Fatal(err)
	}
	if *got != (provider.HostedRefundResult{PaymentID: "12345", PaymentTransactionID: "777", Price: "100", Currency: "TRY", ConversationID: "c-2"}) {
		t.Errorf("result = %+v", got)
	}
	want := map[string]any{"locale": "tr", "conversationId": "c-2", "paymentTransactionId": "777", "price": "100.00",
		"currency": "TRY", "ip": "85.34.78.112"}
	if sent := fake.last(); sent.path != pathRefund || !reflect.DeepEqual(sent.body, want) {
		t.Errorf("sent %s %v", sent.path, sent.body)
	}
}

func TestRefundHostedItemFailures(t *testing.T) {
	t.Run("same-day refusal 10223 is a refusal", func(t *testing.T) {
		p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"10223","errorMessage":"End of day required for the POS.","retryable":false}`)
		_, err := p.RefundHostedItem(context.Background(), provider.HostedRefundRequest{PaymentTransactionID: "777", Price: "1", Currency: "TRY", IP: "1.1.1.1"})
		var refusal *provider.ProviderRefusal
		if !errors.As(err, &refusal) || refusal.Code != "10223" {
			t.Fatalf("want refusal 10223, got %v", err)
		}
	})
	t.Run("bad signature is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, http.StatusOK, refundAnswerJSON(t, false))
		_, err := p.RefundHostedItem(context.Background(), provider.HostedRefundRequest{PaymentTransactionID: "777", Price: "100", Currency: "TRY", IP: "1.1.1.1"})
		if !errors.Is(err, provider.ErrProviderUnavailable) {
			t.Fatalf("want unavailable, got %v", err)
		}
	})
}

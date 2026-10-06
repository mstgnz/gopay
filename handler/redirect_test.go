package handler

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/provider"
)

var (
	formActionRe  = regexp.MustCompile(`<form [^>]*action="([^"]*)"`)
	hiddenInputRe = regexp.MustCompile(`<input type="hidden" name="([^"]*)" value="([^"]*)">`)
)

// formFields reads the auto-submit page and returns the form action and the hidden field values
// after entity decoding, which is what the browser submits to the consumer. Escaped values
// cannot contain a raw quote, so splitting on quotes is exact here.
func formFields(t *testing.T, body string) (string, map[string]string) {
	t.Helper()
	action := ""
	if m := formActionRe.FindStringSubmatch(body); m != nil {
		action = html.UnescapeString(m[1])
	}
	fields := map[string]string{}
	for _, m := range hiddenInputRe.FindAllStringSubmatch(body, -1) {
		fields[html.UnescapeString(m[1])] = html.UnescapeString(m[2])
	}
	if strings.Count(body, "<script>") != 1 || strings.Count(body, "<input") != len(fields) {
		t.Errorf("redirect page contains markup that did not come from the template:\n%s", body)
	}
	return action, fields
}

func callbackWithRedirect(t *testing.T, redirectURL string) *httptest.ResponseRecorder {
	t.Helper()
	mock := &MockPaymentService{
		Complete3DPaymentFunc: func(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error) {
			return &provider.PaymentResponse{Success: true, Status: provider.StatusSuccessful, PaymentID: "pay-1", RedirectURL: redirectURL}, nil
		},
	}
	h := NewPaymentHandler(mock, validator.New())

	req := httptest.NewRequest(http.MethodPost, "/v1/callback/paycell?state=7", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "paycell")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.HandleCallback(rr, req)
	return rr
}

// A tenant chooses the callbackUrl. A script scheme must never reach the form action, however
// the browser would normalize it; every URL that works today must still be redirected.
func TestHandleCallback_RedirectSchemeAllowlist(t *testing.T) {
	refused := []string{
		"javascript:alert(document.cookie)",
		"JavaScript:alert(1)",
		"java\tscript:alert(1)",
		"java\nscript:alert(1)",
		"  javascript:alert(1)",
		"\x01javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
	}
	for _, target := range refused {
		rr := callbackWithRedirect(t, target)
		if strings.Contains(rr.Body.String(), "<form") || rr.Code == http.StatusOK {
			t.Errorf("%q: status %d, a redirect page was served", target, rr.Code)
		}
	}

	allowed := []string{
		"https://consumer.example/payment/callback?type=sales",
		"HTTPS://consumer.example/sonuc",
		"http://consumer.example/sonuc",
		"//consumer.example/sonuc",
		"/payment/result",
	}
	for _, target := range allowed {
		rr := callbackWithRedirect(t, target)
		action, _ := formFields(t, rr.Body.String())
		if rr.Code != http.StatusOK || action != target {
			t.Errorf("%q: status %d, form action %q; want a redirect to it", target, rr.Code, action)
		}
	}
}

// The redirect page carries provider messages and error text. It must neither break out of the
// attribute nor change the values the consumer reads.
func TestHandleCallback_RedirectPageEscapesValues(t *testing.T) {
	hostile := `declined"><script>alert(1)</script><input name="success" value="true`
	callbackURL := "https://consumer.example/odeme_sonuc.php?a=1&b=2"

	mock := &MockPaymentService{
		Complete3DPaymentFunc: func(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error) {
			return &provider.PaymentResponse{
				Success:     false,
				Status:      provider.StatusFailed,
				PaymentID:   "pay-1",
				Message:     hostile,
				Currency:    "TRY",
				RedirectURL: callbackURL,
			}, nil
		},
	}
	h := NewPaymentHandler(mock, validator.New())

	req := httptest.NewRequest(http.MethodPost, "/v1/callback/paycell?state=7", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "paycell")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.HandleCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	action, fields := formFields(t, rr.Body.String())
	if action != callbackURL {
		t.Errorf("form action = %q, want %q", action, callbackURL)
	}
	if fields["message"] != hostile {
		t.Errorf("message value changed for the consumer: %q", fields["message"])
	}
	if fields["success"] != "false" {
		t.Errorf("success = %q, the message injected a field", fields["success"])
	}
	if fields["paymentId"] != "pay-1" || fields["status"] != "failed" {
		t.Errorf("unexpected fields: %+v", fields)
	}
}

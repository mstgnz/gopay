package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestCardRoutesNoConflict ensures the static "cards" routes coexist with the {paymentID}
// wildcard without chi panicking and that requests dispatch to the intended handlers.
func TestCardRoutesNoConflict(t *testing.T) {
	r := chi.NewRouter()
	hit := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(name)) }
	}
	r.Route("/payments", func(r chi.Router) {
		r.Post("/{provider}", hit("process"))
		r.Post("/{provider}/cards/otp/send", hit("send"))
		r.Post("/{provider}/cards/otp/validate", hit("validate"))
		r.Post("/{provider}/cards/register", hit("register"))
		r.Get("/{provider}/cards", hit("list"))
		r.Delete("/{provider}/cards/{cardId}", hit("delete"))
		r.Post("/{provider}/cards/{cardId}/pay", hit("pay"))
		r.Post("/{provider}/checkout", hit("checkout"))
		r.Get("/{provider}/checkout/{token}", hit("checkout-result"))
		r.Post("/{provider}/checkout/cancel", hit("checkout-cancel"))
		r.Post("/{provider}/checkout/refund", hit("checkout-refund"))
		r.Post("/{provider}/submerchants", hit("submerchant-create"))
		r.Put("/{provider}/submerchants/{key}", hit("submerchant-update"))
		r.Get("/{provider}/submerchants/{externalId}", hit("submerchant-detail"))
		r.Post("/{provider}/items/{paymentTransactionId}/approve", hit("approve"))
		r.Post("/{provider}/items/{paymentTransactionId}/disapprove", hit("disapprove"))
		r.Get("/{provider}/{paymentID}", hit("status"))
		r.Delete("/{provider}/{paymentID}", hit("cancel"))
		r.Post("/{provider}/refund", hit("refund"))
		r.Post("/{provider}/installments", hit("installments"))
		r.Post("/{provider}/commission", hit("commission"))
	})

	cases := []struct{ method, path, want string }{
		{"GET", "/payments/paycell/cards", "list"},
		{"GET", "/payments/paycell/pay_123", "status"},
		{"DELETE", "/payments/paycell/cards/5", "delete"},
		{"DELETE", "/payments/paycell/pay_123", "cancel"},
		{"POST", "/payments/paycell/cards/5/pay", "pay"},
		{"POST", "/payments/paycell/cards/register", "register"},
		{"POST", "/payments/paycell", "process"},
		{"POST", "/payments/nkolay", "process"},
		{"GET", "/payments/nkolay/gopay_123", "status"},
		{"DELETE", "/payments/nkolay/gopay_123", "cancel"},
		{"POST", "/payments/nkolay/refund", "refund"},
		{"POST", "/payments/paycell/refund", "refund"},
		{"POST", "/payments/paycell/installments", "installments"},
		{"POST", "/payments/nkolay/commission", "commission"},
		{"POST", "/payments/paycell/cards/otp/send", "send"},
		{"POST", "/payments/paycell/cards/otp/validate", "validate"},
		{"POST", "/payments/iyzico/checkout", "checkout"},
		{"GET", "/payments/iyzico/checkout/tok-1", "checkout-result"},
		{"POST", "/payments/iyzico/checkout/cancel", "checkout-cancel"},
		{"POST", "/payments/iyzico/checkout/refund", "checkout-refund"},
		{"GET", "/payments/iyzico/checkout", "status"},
		{"POST", "/payments/iyzico/refund", "refund"},
		{"POST", "/payments/iyzico/submerchants", "submerchant-create"},
		{"PUT", "/payments/iyzico/submerchants/smk-1", "submerchant-update"},
		{"GET", "/payments/iyzico/submerchants/fac-1", "submerchant-detail"},
		{"POST", "/payments/iyzico/items/777/approve", "approve"},
		{"POST", "/payments/iyzico/items/777/disapprove", "disapprove"},
	}
	for _, c := range cases {
		ctx := chi.NewRouteContext()
		if !r.Match(ctx, c.method, c.path) {
			t.Fatalf("no route matched for %s %s", c.method, c.path)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if got := rec.Body.String(); got != c.want {
			t.Errorf("%s %s dispatched to %q, want %q", c.method, c.path, got, c.want)
		}
	}
}

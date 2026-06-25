package v1

import (
	"net/http"
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
		r.Get("/{provider}/{paymentID}", hit("status"))
		r.Delete("/{provider}/{paymentID}", hit("cancel"))
	})

	cases := []struct{ method, path, want string }{
		{"GET", "/payments/paycell/cards", "list"},
		{"GET", "/payments/paycell/pay_123", "status"},
		{"DELETE", "/payments/paycell/cards/5", "delete"},
		{"DELETE", "/payments/paycell/pay_123", "cancel"},
		{"POST", "/payments/paycell/cards/5/pay", "pay"},
		{"POST", "/payments/paycell/cards/register", "register"},
	}
	for _, c := range cases {
		ctx := chi.NewRouteContext()
		if !r.Match(ctx, c.method, c.path) {
			t.Fatalf("no route matched for %s %s", c.method, c.path)
		}
	}
}

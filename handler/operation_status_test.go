package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/provider"
)

// A refusal by the duplicate guard is a conflict the consumer waits out; it must not read as a
// provider failure.
func TestRefundAndCancelGuardRefusalsAreConflicts(t *testing.T) {
	for guardErr, want := range map[error]int{
		provider.ErrOperationInFlight:    http.StatusConflict,
		provider.ErrOperationUnresolved:  http.StatusConflict,
		provider.ErrOperationNotRecorded: http.StatusServiceUnavailable,
		errors.New("paycell: timeout"):   http.StatusInternalServerError,
	} {
		mock := &MockPaymentService{
			RefundPaymentFunc: func(context.Context, string, string, provider.RefundRequest) (*provider.RefundResponse, error) {
				return nil, guardErr
			},
			CancelPaymentFunc: func(context.Context, string, string, provider.CancelRequest) (*provider.PaymentResponse, error) {
				return nil, guardErr
			},
		}
		h := NewPaymentHandler(mock, validator.New())

		refund := httptest.NewRequest(http.MethodPost, "/v1/payments/paycell/refund?environment=sandbox",
			strings.NewReader(`{"paymentId":"P1","refundAmount":10}`))
		cancel := httptest.NewRequest(http.MethodDelete, "/v1/payments/paycell/P1?environment=sandbox", nil)
		for name, req := range map[string]*http.Request{"refund": refund, "cancel": cancel} {
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "paycell")
			rctx.URLParams.Add("paymentID", "P1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			rr := httptest.NewRecorder()
			if name == "refund" {
				h.RefundPayment(rr, req)
			} else {
				h.CancelPayment(rr, req)
			}
			if rr.Code != want {
				t.Errorf("%s with %v: status %d, want %d", name, guardErr, rr.Code, want)
			}
		}
	}
}

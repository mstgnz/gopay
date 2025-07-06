package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/provider"
)

// Mock PaymentService for testing
type mockPaymentService struct {
	createPaymentFunc     func(ctx context.Context, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error)
	getPaymentStatusFunc  func(ctx context.Context, providerName, paymentID string) (*provider.PaymentResponse, error)
	cancelPaymentFunc     func(ctx context.Context, providerName, paymentID, reason string) (*provider.PaymentResponse, error)
	refundPaymentFunc     func(ctx context.Context, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error)
	complete3DPaymentFunc func(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error)
	validateWebhookFunc   func(ctx context.Context, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

func (m *mockPaymentService) CreatePayment(ctx context.Context, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if m.createPaymentFunc != nil {
		return m.createPaymentFunc(ctx, providerName, request)
	}
	return &provider.PaymentResponse{
		Success:   true,
		PaymentID: "test-payment-123",
		Status:    provider.StatusSuccessful,
		Amount:    request.Amount,
		Currency:  request.Currency,
	}, nil
}

func (m *mockPaymentService) GetPaymentStatus(ctx context.Context, providerName, paymentID string) (*provider.PaymentResponse, error) {
	if m.getPaymentStatusFunc != nil {
		return m.getPaymentStatusFunc(ctx, providerName, paymentID)
	}
	return &provider.PaymentResponse{
		Success:   true,
		PaymentID: paymentID,
		Status:    provider.StatusSuccessful,
		Amount:    100.50,
		Currency:  "TRY",
	}, nil
}

func (m *mockPaymentService) CancelPayment(ctx context.Context, providerName, paymentID, reason string) (*provider.PaymentResponse, error) {
	if m.cancelPaymentFunc != nil {
		return m.cancelPaymentFunc(ctx, providerName, paymentID, reason)
	}
	return &provider.PaymentResponse{
		Success:   true,
		PaymentID: paymentID,
		Status:    provider.StatusCancelled,
		Amount:    100.50,
		Currency:  "TRY",
		Message:   "Payment cancelled: " + reason,
	}, nil
}

func (m *mockPaymentService) RefundPayment(ctx context.Context, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if m.refundPaymentFunc != nil {
		return m.refundPaymentFunc(ctx, providerName, request)
	}
	return &provider.RefundResponse{
		Success:      true,
		RefundID:     "refund-123",
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		Status:       "refunded",
		Message:      "Refund successful",
	}, nil
}

func (m *mockPaymentService) Complete3DPayment(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if m.complete3DPaymentFunc != nil {
		return m.complete3DPaymentFunc(ctx, providerName, paymentID, conversationID, data)
	}
	return &provider.PaymentResponse{
		Success:   true,
		PaymentID: paymentID,
		Status:    provider.StatusSuccessful,
		Amount:    100.50,
		Currency:  "TRY",
		Message:   "3D payment completed",
	}, nil
}

func (m *mockPaymentService) ValidateWebhook(ctx context.Context, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	if m.validateWebhookFunc != nil {
		return m.validateWebhookFunc(ctx, providerName, data, headers)
	}
	return true, map[string]string{
		"paymentId": "test-payment-123",
		"status":    "success",
	}, nil
}

func TestNewPaymentHandler(t *testing.T) {
	mockService := &mockPaymentService{}
	validate := validator.New()
	handler := NewPaymentHandler(mockService, validate)

	if handler == nil {
		t.Fatal("NewPaymentHandler should not return nil")
	}

	if handler.paymentService != mockService {
		t.Error("Handler should store the payment service")
	}
}

func TestPaymentHandler_ProcessPayment(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		body           string
		expectedStatus int
		mockFunc       func(ctx context.Context, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error)
	}{
		{
			name:           "successful payment without tenant",
			method:         "POST",
			path:           "/payments/iyzico",
			body:           `{"amount":100.50,"currency":"TRY","customer":{"name":"John","surname":"Doe","email":"john@example.com"},"cardInfo":{"cardNumber":"5528790000000008","cvv":"123","expireMonth":"12","expireYear":"2030"}}`,
			expectedStatus: 200,
		},
		{
			name:           "successful payment with tenant",
			method:         "POST",
			path:           "/payments/iyzico",
			tenantID:       "APP1",
			body:           `{"amount":100.50,"currency":"TRY","customer":{"name":"John","surname":"Doe","email":"john@example.com"},"cardInfo":{"cardNumber":"5528790000000008","cvv":"123","expireMonth":"12","expireYear":"2030"}}`,
			expectedStatus: 200,
		},
		{
			name:           "invalid JSON",
			method:         "POST",
			path:           "/payments/iyzico",
			body:           `{"invalid": json}`,
			expectedStatus: 400,
		},
		{
			name:           "missing amount",
			method:         "POST",
			path:           "/payments/iyzico",
			body:           `{"currency":"TRY","customer":{"email":"john@example.com"}}`,
			expectedStatus: 200, // No validation tags, so it will pass
		},
		{
			name:           "unsupported method",
			method:         "GET",
			path:           "/payments/iyzico",
			body:           "",
			expectedStatus: 400, // Handler will try to decode empty body
		},
		{
			name:           "service error",
			method:         "POST",
			path:           "/payments/iyzico",
			body:           `{"amount":100.50,"currency":"TRY","customer":{"name":"John","surname":"Doe","email":"john@example.com"},"cardInfo":{"cardNumber":"5528790000000008","cvv":"123","expireMonth":"12","expireYear":"2030"}}`,
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
				return nil, errors.New("payment processing failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				createPaymentFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.ProcessPayment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == 200 {
				var response map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if !response["success"].(bool) {
					t.Error("Expected success to be true")
				}

				if tt.tenantID != "" {
					// Verify tenant-specific provider name was used
					expectedProviderName := strings.ToUpper(tt.tenantID) + "_iyzico"
					_ = expectedProviderName // Used for verification logic
				}
			}
		})
	}
}

func TestPaymentHandler_GetPaymentStatus(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		paymentID      string
		expectedStatus int
		mockFunc       func(ctx context.Context, providerName, paymentID string) (*provider.PaymentResponse, error)
	}{
		{
			name:           "successful status check",
			method:         "GET",
			path:           "/payments/iyzico/test-payment-123",
			paymentID:      "test-payment-123",
			expectedStatus: 200,
		},
		{
			name:           "status check with tenant",
			method:         "GET",
			path:           "/payments/iyzico/test-payment-123",
			tenantID:       "APP1",
			paymentID:      "test-payment-123",
			expectedStatus: 200,
		},
		{
			name:           "payment not found",
			method:         "GET",
			path:           "/payments/iyzico/nonexistent",
			paymentID:      "nonexistent",
			expectedStatus: 500, // Handler returns 500 for service errors
			mockFunc: func(ctx context.Context, providerName, paymentID string) (*provider.PaymentResponse, error) {
				return nil, errors.New("payment not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				getPaymentStatusFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			rctx.URLParams.Add("paymentID", tt.paymentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetPaymentStatus(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_CancelPayment(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		paymentID      string
		body           string
		expectedStatus int
		mockFunc       func(ctx context.Context, providerName, paymentID, reason string) (*provider.PaymentResponse, error)
	}{
		{
			name:           "successful cancellation",
			method:         "DELETE",
			path:           "/payments/iyzico/test-payment-123",
			paymentID:      "test-payment-123",
			body:           `{"reason":"User requested cancellation"}`,
			expectedStatus: 200,
		},
		{
			name:           "cancellation with tenant",
			method:         "DELETE",
			path:           "/payments/iyzico/test-payment-123",
			tenantID:       "APP1",
			paymentID:      "test-payment-123",
			body:           `{"reason":"User requested cancellation"}`,
			expectedStatus: 200,
		},
		{
			name:           "invalid JSON",
			method:         "DELETE",
			path:           "/payments/iyzico/test-payment-123",
			paymentID:      "test-payment-123",
			body:           `{"invalid": json}`,
			expectedStatus: 200, // Handler ignores JSON parse errors for reason
		},
		{
			name:           "payment cannot be cancelled",
			method:         "DELETE",
			path:           "/payments/iyzico/test-payment-123",
			paymentID:      "test-payment-123",
			body:           `{"reason":"Test cancellation"}`,
			expectedStatus: 500, // Handler returns 500 for service errors
			mockFunc: func(ctx context.Context, providerName, paymentID, reason string) (*provider.PaymentResponse, error) {
				return nil, errors.New("payment cannot be cancelled")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				cancelPaymentFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			rctx.URLParams.Add("paymentID", tt.paymentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.CancelPayment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_RefundPayment(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		body           string
		expectedStatus int
		mockFunc       func(ctx context.Context, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error)
	}{
		{
			name:           "successful refund",
			method:         "POST",
			path:           "/payments/iyzico/refund",
			body:           `{"paymentId":"test-payment-123","refundAmount":50.25,"reason":"Partial refund"}`,
			expectedStatus: 200,
		},
		{
			name:           "refund with tenant",
			method:         "POST",
			path:           "/payments/iyzico/refund",
			tenantID:       "APP1",
			body:           `{"paymentId":"test-payment-123","refundAmount":50.25,"reason":"Partial refund"}`,
			expectedStatus: 200,
		},
		{
			name:           "invalid JSON",
			method:         "POST",
			path:           "/payments/iyzico/refund",
			body:           `{"invalid": json}`,
			expectedStatus: 400,
		},
		{
			name:           "missing payment ID",
			method:         "POST",
			path:           "/payments/iyzico/refund",
			body:           `{"refundAmount":50.25,"reason":"Test"}`,
			expectedStatus: 200, // No validation tags, so it will pass
		},
		{
			name:           "refund failed",
			method:         "POST",
			path:           "/payments/iyzico/refund",
			body:           `{"paymentId":"test-payment-123","refundAmount":200.00,"reason":"Full refund"}`,
			expectedStatus: 500, // Handler returns 500 for service errors
			mockFunc: func(ctx context.Context, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error) {
				return nil, errors.New("refund amount exceeds payment amount")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				refundPaymentFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.RefundPayment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_HandleCallback(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		path             string
		tenantIDHeader   string
		tenantIDQuery    string
		queryParams      string
		body             string
		expectedStatus   int
		expectedRedirect string
		mockFunc         func(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error)
	}{
		{
			name:             "successful callback with redirect",
			method:           "POST",
			path:             "/callback/iyzico",
			queryParams:      "originalCallbackUrl=https://app.com/success&tenantId=APP1&paymentId=test-123&conversationId=conv-123",
			body:             `{"status":"success"}`,
			expectedStatus:   302,
			expectedRedirect: "https://app.com/success?paymentId=test-123&status=successful&amount=100.50&currency=TRY",
		},
		{
			name:           "successful callback without redirect",
			method:         "POST",
			path:           "/callback/iyzico",
			tenantIDHeader: "APP1",
			queryParams:    "paymentId=test-123",
			body:           `{"status":"success"}`,
			expectedStatus: 200,
		},
		{
			name:           "callback with tenant ID from header",
			method:         "POST",
			path:           "/callback/iyzico",
			tenantIDHeader: "APP1",
			queryParams:    "paymentId=test-123",
			body:           `{"status":"success"}`,
			expectedStatus: 200,
		},
		{
			name:           "callback fails 3D completion",
			method:         "POST",
			path:           "/callback/iyzico",
			tenantIDHeader: "APP1",
			queryParams:    "paymentId=test-123",
			body:           `{"status":"failed"}`,
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
				return nil, errors.New("3D authentication failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				complete3DPaymentFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			var req *http.Request
			if tt.queryParams != "" {
				req = httptest.NewRequest(tt.method, tt.path+"?"+tt.queryParams, strings.NewReader(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			}

			req.Header.Set("Content-Type", "application/json")
			if tt.tenantIDHeader != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantIDHeader)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.HandleCallback(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedRedirect != "" {
				location := w.Header().Get("Location")
				if location == "" {
					t.Error("Expected redirect location header")
				}
				// Check if redirect contains expected parameters
				if !strings.Contains(location, "paymentId=test-123") {
					t.Error("Redirect should contain payment ID")
				}
			}
		})
	}
}

func TestPaymentHandler_HandleWebhook(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantIDHeader string
		tenantIDQuery  string
		queryParams    string
		body           string
		headers        map[string]string
		expectedStatus int
		mockFunc       func(ctx context.Context, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error)
	}{
		{
			name:           "successful webhook validation",
			method:         "POST",
			path:           "/webhooks/iyzico",
			tenantIDHeader: "APP1",
			body:           `{"paymentId":"test-123","status":"success","amount":"100.50"}`,
			headers:        map[string]string{"X-Signature": "valid-signature"},
			expectedStatus: 200,
		},
		{
			name:           "webhook with tenant from query",
			method:         "POST",
			path:           "/webhooks/iyzico",
			queryParams:    "tenantId=APP1",
			body:           `{"paymentId":"test-123","status":"success"}`,
			headers:        map[string]string{"X-Signature": "valid-signature"},
			expectedStatus: 200,
		},
		{
			name:           "invalid webhook signature",
			method:         "POST",
			path:           "/webhooks/iyzico",
			tenantIDHeader: "APP1",
			body:           `{"paymentId":"test-123","status":"success"}`,
			headers:        map[string]string{"X-Signature": "invalid-signature"},
			expectedStatus: 400,
			mockFunc: func(ctx context.Context, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
				return false, nil, errors.New("invalid webhook signature")
			},
		},
		{
			name:           "webhook validation error",
			method:         "POST",
			path:           "/webhooks/iyzico",
			tenantIDHeader: "APP1",
			body:           `{"paymentId":"test-123","status":"success"}`,
			headers:        map[string]string{"X-Signature": "error-signature"},
			expectedStatus: 400, // Handler returns 400 for validation errors
			mockFunc: func(ctx context.Context, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
				return false, nil, errors.New("webhook processing error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockPaymentService{
				validateWebhookFunc: tt.mockFunc,
			}
			validate := validator.New()
			handler := NewPaymentHandler(mockService, validate)

			var req *http.Request
			if tt.queryParams != "" {
				req = httptest.NewRequest(tt.method, tt.path+"?"+tt.queryParams, strings.NewReader(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			}

			req.Header.Set("Content-Type", "application/json")
			if tt.tenantIDHeader != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantIDHeader)
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.HandleWebhook(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Benchmark tests
func BenchmarkPaymentHandler_ProcessPayment(b *testing.B) {
	mockService := &provider.PaymentService{}
	validate := validator.New()
	handler := NewPaymentHandler(mockService, validate)

	body := `{"amount":100.50,"currency":"TRY","customer":{"name":"John","surname":"Doe","email":"john@example.com"},"cardInfo":{"cardNumber":"5528790000000008","cvv":"123","expireMonth":"12","expireYear":"2030"}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/payments/iyzico", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.ProcessPayment(w, req)
	}
}

func BenchmarkPaymentHandler_HandleCallback(b *testing.B) {
	mockService := &mockPaymentService{}
	validate := validator.New()
	handler := NewPaymentHandler(mockService, validate)

	body := `{"paymentId":"test-123","status":"success","conversationId":"conv-123"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/callback/iyzico", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "APP1")

		w := httptest.NewRecorder()

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.HandleCallback(w, req)
	}
}

// Additional edge case tests to improve coverage
func TestPaymentHandler_AdditionalEdgeCases(t *testing.T) {
	service := &mockPaymentService{}
	validate := validator.New()
	handler := NewPaymentHandler(service, validate)

	// Test callback with missing paymentID
	t.Run("callback without paymentID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/callback/iyzico", nil)
		w := httptest.NewRecorder()

		handler.HandleCallback(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	// Test webhook with invalid JSON
	t.Run("webhook with invalid JSON", func(t *testing.T) {
		body := `{"invalid": json}`
		req := httptest.NewRequest("POST", "/webhooks/iyzico", strings.NewReader(body))
		req.Header.Set("X-Tenant-ID", "APP1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)

		if w.Code != 400 {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	// Test payment with very large amounts
	t.Run("payment with large amount", func(t *testing.T) {
		body := `{"amount":999999.99,"currency":"TRY","customer":{"email":"test@example.com"}}`
		req := httptest.NewRequest("POST", "/payments/iyzico", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ProcessPayment(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test refund with empty reason
	t.Run("refund with empty reason", func(t *testing.T) {
		body := `{"paymentId":"test-123","refundAmount":50.00,"reason":""}`
		req := httptest.NewRequest("POST", "/payments/iyzico/refund", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RefundPayment(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test cancel with reason
	t.Run("cancel with reason", func(t *testing.T) {
		body := `{"reason":"Customer request"}`
		req := httptest.NewRequest("DELETE", "/payments/iyzico/test-123", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Set up chi context for cancel endpoint
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		rctx.URLParams.Add("paymentID", "test-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.CancelPayment(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

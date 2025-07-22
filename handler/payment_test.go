package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/provider"
)

// MockPaymentService implements PaymentServiceInterface for testing
type MockPaymentService struct {
	CreatePaymentFunc     func(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error)
	GetPaymentStatusFunc  func(ctx context.Context, environment, providerName string, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error)
	CancelPaymentFunc     func(ctx context.Context, environment, providerName string, request provider.CancelRequest) (*provider.PaymentResponse, error)
	RefundPaymentFunc     func(ctx context.Context, environment, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error)
	Complete3DPaymentFunc func(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error)
	ValidateWebhookFunc   func(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

func (m *MockPaymentService) CreatePayment(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if m.CreatePaymentFunc != nil {
		return m.CreatePaymentFunc(ctx, environment, providerName, request)
	}
	return &provider.PaymentResponse{
		Success:       true,
		PaymentID:     "test-payment-123",
		TransactionID: "test-tx-123",
		Status:        "success",
		Amount:        request.Amount,
		Currency:      request.Currency,
		Message:       "Payment successful",
	}, nil
}

func (m *MockPaymentService) GetPaymentStatus(ctx context.Context, environment, providerName string, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if m.GetPaymentStatusFunc != nil {
		return m.GetPaymentStatusFunc(ctx, environment, providerName, request)
	}
	return &provider.PaymentResponse{
		Success:       true,
		PaymentID:     request.PaymentID,
		TransactionID: "test-tx-123",
		Status:        "success",
		Amount:        100.0,
		Currency:      "TRY",
		Message:       "Payment successful",
	}, nil
}

func (m *MockPaymentService) CancelPayment(ctx context.Context, environment, providerName string, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if m.CancelPaymentFunc != nil {
		return m.CancelPaymentFunc(ctx, environment, providerName, request)
	}
	return &provider.PaymentResponse{
		Success:   true,
		PaymentID: request.PaymentID,
		Status:    "cancelled",
		Message:   "Payment cancelled",
	}, nil
}

func (m *MockPaymentService) RefundPayment(ctx context.Context, environment, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if m.RefundPaymentFunc != nil {
		return m.RefundPaymentFunc(ctx, environment, providerName, request)
	}
	return &provider.RefundResponse{
		Success:   true,
		RefundID:  "refund-123",
		PaymentID: request.PaymentID,
		Status:    "refunded",
		Message:   "Refund successful",
	}, nil
}

func (m *MockPaymentService) Complete3DPayment(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error) {
	if m.Complete3DPaymentFunc != nil {
		return m.Complete3DPaymentFunc(ctx, providerName, state, data)
	}
	return &provider.PaymentResponse{
		Success:       true,
		PaymentID:     "test-payment-123",
		TransactionID: "test-tx-123",
		Status:        "success",
		Amount:        100.0,
		Currency:      "TRY",
		Message:       "3D Payment completed",
	}, nil
}

func (m *MockPaymentService) ValidateWebhook(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	if m.ValidateWebhookFunc != nil {
		return m.ValidateWebhookFunc(ctx, environment, providerName, data, headers)
	}
	return true, map[string]string{
		"paymentId": "test-payment-123",
		"status":    "success",
	}, nil
}

func TestNewPaymentHandler(t *testing.T) {
	mockService := &MockPaymentService{}
	validator := validator.New()

	handler := NewPaymentHandler(mockService, validator)

	if handler == nil {
		t.Fatal("NewPaymentHandler should not return nil")
	}

	if handler.paymentService != mockService {
		t.Error("Handler should store the payment service")
	}

	if handler.validate != validator {
		t.Error("Handler should store the validator")
	}
}

func TestPaymentHandler_ProcessPayment(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    any
		environment    string
		provider       string
		expectedStatus int
		mockFunc       func(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error)
	}{
		{
			name: "successful payment",
			requestBody: provider.PaymentRequest{
				Amount:         100.50,
				Currency:       "TRY",
				ConversationID: "test-conv-123",
				CallbackURL:    "https://example.com/callback",
			},
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid-json",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name: "missing environment",
			requestBody: provider.PaymentRequest{
				Amount:   100.50,
				Currency: "TRY",
			},
			environment:    "",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name: "payment service error",
			requestBody: provider.PaymentRequest{
				Amount:   100.50,
				Currency: "TRY",
			},
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
				return nil, errors.New("payment service error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				CreatePaymentFunc: tt.mockFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest("POST", "/payments/"+tt.provider+"?environment="+tt.environment, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.ProcessPayment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_GetPaymentStatus(t *testing.T) {
	tests := []struct {
		name           string
		paymentID      string
		environment    string
		provider       string
		expectedStatus int
		mockFunc       func(ctx context.Context, environment, providerName string, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error)
	}{
		{
			name:           "successful status check",
			paymentID:      "test-payment-123",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "missing payment ID",
			paymentID:      "",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "missing environment",
			paymentID:      "test-payment-123",
			environment:    "",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "service error",
			paymentID:      "test-payment-123",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, environment, providerName string, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
				return nil, errors.New("service error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				GetPaymentStatusFunc: tt.mockFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			req := httptest.NewRequest("GET", "/payments/"+tt.provider+"/"+tt.paymentID+"?environment="+tt.environment, nil)

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			rctx.URLParams.Add("paymentID", tt.paymentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
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
		paymentID      string
		requestBody    string
		environment    string
		provider       string
		expectedStatus int
		mockFunc       func(ctx context.Context, environment, providerName string, request provider.CancelRequest) (*provider.PaymentResponse, error)
	}{
		{
			name:           "successful cancellation",
			paymentID:      "test-payment-123",
			requestBody:    `{"reason": "customer request"}`,
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "cancellation without reason",
			paymentID:      "test-payment-123",
			requestBody:    "",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "missing payment ID",
			paymentID:      "",
			requestBody:    `{"reason": "test"}`,
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "service error",
			paymentID:      "test-payment-123",
			requestBody:    `{"reason": "test"}`,
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, environment, providerName string, request provider.CancelRequest) (*provider.PaymentResponse, error) {
				return nil, errors.New("cancellation failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				CancelPaymentFunc: tt.mockFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			req := httptest.NewRequest("DELETE", "/payments/"+tt.provider+"/"+tt.paymentID+"?environment="+tt.environment, strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			rctx.URLParams.Add("paymentID", tt.paymentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
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
		requestBody    any
		environment    string
		provider       string
		expectedStatus int
		mockFunc       func(ctx context.Context, environment, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error)
	}{
		{
			name: "successful refund",
			requestBody: provider.RefundRequest{
				PaymentID: "test-payment-123",
				Currency:  "TRY",
				Reason:    "customer request",
			},
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid-json",
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "missing environment",
			requestBody:    provider.RefundRequest{PaymentID: "test"},
			environment:    "",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "service error",
			requestBody:    provider.RefundRequest{PaymentID: "test"},
			environment:    "sandbox",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, environment, providerName string, request provider.RefundRequest) (*provider.RefundResponse, error) {
				return nil, errors.New("refund failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				RefundPaymentFunc: tt.mockFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest("POST", "/payments/"+tt.provider+"/refund?environment="+tt.environment, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.RefundPayment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_HandleCallback(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		formData       map[string]string
		expectedStatus int
		expectRedirect bool
		mockFunc       func(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error)
	}{
		{
			name: "successful callback with redirect",
			queryParams: map[string]string{
				"state":               "test-encrypted-state",
				"originalCallbackUrl": "https://example.com/callback",
			},
			expectedStatus: 302,
			expectRedirect: true,
		},
		{
			name: "successful callback without redirect",
			queryParams: map[string]string{
				"state": "test-encrypted-state",
			},
			expectedStatus: 200,
			expectRedirect: false,
		},
		{
			name: "missing state",
			queryParams: map[string]string{
				"originalCallbackUrl": "https://example.com/callback",
			},
			expectedStatus: 400,
			expectRedirect: false,
		},
		{
			name: "3D payment completion error",
			queryParams: map[string]string{
				"state": "test-encrypted-state",
			},
			expectedStatus: 500,
			expectRedirect: false,
			mockFunc: func(ctx context.Context, providerName, state string, data map[string]string) (*provider.PaymentResponse, error) {
				return nil, errors.New("3D completion failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				Complete3DPaymentFunc: tt.mockFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			// Build URL with query parameters
			u := &url.URL{Path: "/callback/iyzico"}
			q := u.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest("POST", u.String(), nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Add form data
			if tt.formData != nil {
				form := url.Values{}
				for key, value := range tt.formData {
					form.Set(key, value)
				}
				req.PostForm = form
			}

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.HandleCallback(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectRedirect && w.Header().Get("Location") == "" {
				t.Error("Expected redirect but Location header is empty")
			}
		})
	}
}

func TestPaymentHandler_HandleWebhook(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		requestBody      string
		queryParams      map[string]string
		headers          map[string]string
		expectedStatus   int
		mockValidateFunc func(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error)
	}{
		{
			name:        "successful JSON webhook",
			contentType: "application/json",
			requestBody: `{"paymentId":"test-123","status":"success"}`,
			queryParams: map[string]string{
				"environment": "sandbox",
			},
			headers: map[string]string{
				"X-Signature": "valid-signature",
			},
			expectedStatus: 200,
		},
		{
			name:        "successful form webhook",
			contentType: "application/x-www-form-urlencoded",
			requestBody: "paymentId=test-123&status=success",
			queryParams: map[string]string{
				"environment": "sandbox",
			},
			expectedStatus: 200,
		},
		{
			name:        "invalid JSON",
			contentType: "application/json",
			requestBody: "invalid-json",
			queryParams: map[string]string{
				"environment": "sandbox",
			},
			expectedStatus: 400,
		},
		{
			name:           "missing environment",
			contentType:    "application/json",
			requestBody:    `{"paymentId":"test-123"}`,
			expectedStatus: 400,
		},
		{
			name:        "webhook validation error",
			contentType: "application/json",
			requestBody: `{"paymentId":"test-123"}`,
			queryParams: map[string]string{
				"environment": "sandbox",
			},
			expectedStatus: 400,
			mockValidateFunc: func(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
				return false, nil, errors.New("validation failed")
			},
		},
		{
			name:        "invalid webhook signature",
			contentType: "application/json",
			requestBody: `{"paymentId":"test-123"}`,
			queryParams: map[string]string{
				"environment": "sandbox",
			},
			expectedStatus: 400,
			mockValidateFunc: func(ctx context.Context, environment, providerName string, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
				return false, nil, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{
				ValidateWebhookFunc: tt.mockValidateFunc,
			}
			handler := NewPaymentHandler(mockService, validator.New())

			// Build URL with query parameters
			u := &url.URL{Path: "/webhooks/iyzico"}
			q := u.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest("POST", u.String(), strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", tt.contentType)

			// Add custom headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", "iyzico")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.HandleWebhook(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestPaymentHandler_TenantSpecificProvider(t *testing.T) {
	mockService := &MockPaymentService{
		CreatePaymentFunc: func(ctx context.Context, environment, providerName string, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
			// Check if tenant-specific provider name is constructed correctly
			if providerName != "TENANT123_iyzico" {
				t.Errorf("Expected provider name 'TENANT123_iyzico', got '%s'", providerName)
			}
			return &provider.PaymentResponse{Success: true}, nil
		},
	}

	handler := NewPaymentHandler(mockService, validator.New())

	requestBody := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "TRY",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/payments/iyzico?environment=sandbox", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Simulate tenant ID from JWT context
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "tenant123")
	req = req.WithContext(ctx)

	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "iyzico")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ProcessPayment(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func BenchmarkPaymentHandler_ProcessPayment(b *testing.B) {
	mockService := &MockPaymentService{}
	handler := NewPaymentHandler(mockService, validator.New())

	requestBody := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "TRY",
	}
	body, _ := json.Marshal(requestBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/payments/iyzico?environment=sandbox", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.ProcessPayment(w, req)
	}
}

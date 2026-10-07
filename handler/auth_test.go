package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/middle"
)

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	// Create minimal auth services for testing
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_CreateTenant(t *testing.T) {
	handler := NewAuthHandler(&auth.TenantService{}, &auth.JWTService{}, validator.New())

	tests := []struct {
		name     string
		tenantID string
		username string
		want     int
	}{
		// The caller is checked before the body is read.
		{name: "no authenticated tenant", want: http.StatusUnauthorized},
		{name: "non-admin tenant", tenantID: "2", username: "tenant2", want: http.StatusForbidden},
		{name: "admin with invalid JSON", tenantID: "1", username: "admin", want: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth/tenants", bytes.NewBufferString("invalid-json"))
			req.Header.Set("Content-Type", "application/json")
			if tt.tenantID != "" {
				ctx := context.WithValue(req.Context(), middle.TenantIDKey, tt.tenantID)
				ctx = context.WithValue(ctx, middle.TenantUserKey, tt.username)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.CreateTenant(w, req)

			if w.Code != tt.want {
				t.Errorf("Expected status %d, got %d", tt.want, w.Code)
			}
		})
	}
}

func TestAuthHandler_RefreshToken_InvalidJSON(t *testing.T) {
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RefreshToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_ValidateToken_MissingHeader(t *testing.T) {
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	req := httptest.NewRequest("GET", "/auth/validate", nil)
	w := httptest.NewRecorder()

	handler.ValidateToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_Logout_MissingContext(t *testing.T) {
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func BenchmarkAuthHandler_Login(b *testing.B) {
	tenantService := &auth.TenantService{}
	jwtService := &auth.JWTService{}
	handler := NewAuthHandler(tenantService, jwtService, validator.New())

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.Login(w, req)
	}
}

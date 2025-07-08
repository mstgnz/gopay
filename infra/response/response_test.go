package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()

	Success(w, http.StatusOK, "Test successful", map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, http.StatusBadRequest, "Test error", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func BenchmarkSuccessResponse(b *testing.B) {
	data := map[string]string{"test": "data"}

	for b.Loop() {
		w := httptest.NewRecorder()
		Success(w, http.StatusOK, "Benchmark test", data)
	}
}

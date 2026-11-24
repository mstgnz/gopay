package middle

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A chunked body declares no length; the limit must still stop it while it is read.
func TestRequestValidationMiddlewareBoundsAChunkedBody(t *testing.T) {
	var readErr error
	var read int64
	handler := RequestValidationMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, readErr = io.Copy(io.Discard, r.Body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/payments/paycell", io.NopCloser(bytes.NewReader(make([]byte, maxRequestBody+1))))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1 // chunked
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) || read > maxRequestBody {
		t.Fatalf("read %d bytes, err %v; want the read stopped at %d", read, readErr, maxRequestBody)
	}

	// A body within the limit reads whole.
	req = httptest.NewRequest(http.MethodPost, "/v1/payments/paycell", bytes.NewReader([]byte(`{"amount":10}`)))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if readErr != nil || read != 13 {
		t.Fatalf("small body: read %d, err %v", read, readErr)
	}
}

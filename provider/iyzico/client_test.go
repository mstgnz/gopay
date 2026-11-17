package iyzico

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

const (
	testAPIKey    = "sandbox-api"
	testSecretKey = "sandbox-secret"
	testRandomKey = "123456789"
)

// recorded is one request the fake iyzico received.
type recorded struct {
	method, path, auth, rnd string
	body                    map[string]any
	raw                     []byte
}

// fakeIyzico answers every request with status and body, and records what it got.
type fakeIyzico struct {
	t        *testing.T
	status   int
	answer   string
	requests []recorded
}

func (f *fakeIyzico) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.requests = append(f.requests, recorded{method: r.Method, path: r.URL.Path,
		auth: r.Header.Get("Authorization"), rnd: r.Header.Get("x-iyzi-rnd"), body: body, raw: raw})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	_, _ = io.WriteString(w, f.answer)
}

func (f *fakeIyzico) last() recorded {
	f.t.Helper()
	if len(f.requests) == 0 {
		f.t.Fatal("iyzico received no request")
	}
	return f.requests[len(f.requests)-1]
}

// newTestProvider points a provider at a fake iyzico answering status and answer.
func newTestProvider(t *testing.T, status int, answer string) (*IyzicoProvider, *fakeIyzico) {
	t.Helper()
	fake := &fakeIyzico{t: t, status: status, answer: answer}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	c := newClient(testAPIKey, testSecretKey, srv.URL)
	c.http = srv.Client()
	c.randomKey = func() string { return testRandomKey }
	return &IyzicoProvider{client: c}, fake
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Vector computed with openssl, independent of this code:
// printf '%s' '123456789/payment/bin/check{"binNumber":"589004"}' | openssl dgst -sha256 -hmac sandbox-secret
func TestAuthorizationMatchesIndependentVector(t *testing.T) {
	got := authorization(testAPIKey, testSecretKey, testRandomKey, "/payment/bin/check", []byte(`{"binNumber":"589004"}`))
	want := "IYZWSv2 YXBpS2V5OnNhbmRib3gtYXBpJnJhbmRvbUtleToxMjM0NTY3ODkmc2lnbmF0dXJlOjU1NGMyYmMzMzRiODIyYWRlMjU5NmFlNjJhZmYyNzMzOWY4YWQ3MDVlMGEzMzNlZWRlNmIwMzJkZGIxMGIwYWY="
	if got != want {
		t.Fatalf("authorization header\n got %s\nwant %s", got, want)
	}
}

// Vector computed with openssl: HMAC-SHA256(secret, "SUCCESS:12345:TRY:B-1:conv-1:450.5:450.5:tok-1").
func TestResponseSignatureMatchesIndependentVector(t *testing.T) {
	c := newClient(testAPIKey, testSecretKey, "http://unused")
	want := "3595c1ca55c107c163d494f8bbefd0e322f1c718664785850e3fd0b65df3dd10"
	if got := c.responseSignature("SUCCESS", "12345", "TRY", "B-1", "conv-1", "450.5", "450.5", "tok-1"); got != want {
		t.Fatalf("response signature\n got %s\nwant %s", got, want)
	}
	if !c.signatureMatches(strings.ToUpper(want), "SUCCESS", "12345", "TRY", "B-1", "conv-1", "450.5", "450.5", "tok-1") {
		t.Error("an upper-case hex signature must match")
	}
	if c.signatureMatches("", "SUCCESS") {
		t.Error("an empty signature must never match")
	}
}

func TestDoSignsTheExactBodySentAndSetsHeaders(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success"}`)
	var out struct{}
	if err := p.client.do(context.Background(), http.MethodPost, "/payment/x", map[string]string{"a": "1"}, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	got := fake.last()
	if got.method != http.MethodPost || got.path != "/payment/x" {
		t.Errorf("sent %s %s", got.method, got.path)
	}
	if got.rnd != testRandomKey {
		t.Errorf("x-iyzi-rnd = %q", got.rnd)
	}
	if want := authorization(testAPIKey, testSecretKey, testRandomKey, "/payment/x", got.raw); got.auth != want {
		t.Errorf("Authorization does not sign the bytes sent:\n got %s\nwant %s", got.auth, want)
	}
}

func TestDoClassifiesAnswers(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		answer      string
		unavailable bool
		refusalCode string
		retryable   bool
	}{
		{"success", 200, `{"status":"success"}`, false, "", false},
		{"refusal with 200", 200, `{"status":"failure","errorCode":"10051","errorMessage":"limit"}`, false, "10051", false},
		{"refusal with 400", 400, `{"status":"failure","errorCode":"5086","errorMessage":"not yours"}`, false, "5086", false},
		{"numeric error code", 200, `{"status":"failure","errorCode":12,"errorMessage":"x"}`, false, "12", false},
		{"retryable refusal", 200, `{"status":"failure","errorCode":"10223","retryable":true}`, false, "10223", true},
		{"rate limit code", 200, `{"status":"failure","errorCode":50000,"errorMessage":"Request Limit Exceeded"}`, true, "", false},
		{"http 429", 429, `{"status":"failure","errorCode":"x"}`, true, "", false},
		{"http 500", 500, `{"status":"failure","errorCode":"x"}`, true, "", false},
		{"http 502 html", 502, `<html>bad gateway</html>`, true, "", false},
		{"unreadable 200", 200, `not json`, true, "", false},
		{"no status", 200, `{"errorCode":"1"}`, true, "", false},
		{"success with 4xx", 404, `{"status":"success"}`, true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := newTestProvider(t, tc.status, tc.answer)
			var out struct{}
			err := p.client.do(context.Background(), http.MethodPost, "/x", map[string]string{}, &out)

			var refusal *provider.ProviderRefusal
			switch {
			case tc.unavailable:
				if !errors.Is(err, provider.ErrProviderUnavailable) {
					t.Fatalf("want unavailable, got %v", err)
				}
				if errors.As(err, &refusal) {
					t.Fatal("an unavailable answer must not read as a refusal")
				}
			case tc.refusalCode != "":
				if !errors.As(err, &refusal) {
					t.Fatalf("want refusal, got %v", err)
				}
				if refusal.Code != tc.refusalCode || refusal.Retryable != tc.retryable {
					t.Errorf("refusal = %+v", refusal)
				}
				if errors.Is(err, provider.ErrProviderUnavailable) {
					t.Error("a refusal must not read as unavailable")
				}
			default:
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
			}
		})
	}
}

func TestDoTransportFailureIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	c := newClient(testAPIKey, testSecretKey, srv.URL)
	var out struct{}
	if err := c.do(context.Background(), http.MethodPost, "/x", map[string]string{}, &out); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestDoTimeoutIsUnavailable(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	// Cleanups run last-in first-out: the handler is released before Close waits for it.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	c := newClient(testAPIKey, testSecretKey, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var out struct{}
	if err := c.do(ctx, http.MethodPost, "/x", map[string]string{}, &out); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestDoRefusesAnOversizedAnswer(t *testing.T) {
	big := `{"status":"success","pad":"` + strings.Repeat("x", maxAnswerBytes) + `"}`
	p, _ := newTestProvider(t, http.StatusOK, big)
	var out struct{}
	if err := p.client.do(context.Background(), http.MethodPost, "/x", map[string]string{}, &out); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

// The shared provider HTTP client skips TLS verification for Nkolay; iyzico's must not.
func TestHTTPClientVerifiesTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status":"success"}`)
	}))
	t.Cleanup(srv.Close)
	c := newClient(testAPIKey, testSecretKey, srv.URL) // the production client, self-signed server
	var out struct{}
	err := c.do(context.Background(), http.MethodPost, "/x", map[string]string{}, &out)
	if !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("a self-signed certificate must be refused, got %v", err)
	}
	if tr, ok := httpClient.Transport.(*http.Transport); !ok || tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("iyzico transport must verify TLS")
	}
}

func TestRandomKeyIsFreshAndNumeric(t *testing.T) {
	a, b := newRandomKey(), newRandomKey()
	if a == b {
		t.Error("two random keys are equal")
	}
	for _, r := range a {
		if r < '0' || r > '9' {
			t.Fatalf("random key %q is not numeric", a)
		}
	}
}

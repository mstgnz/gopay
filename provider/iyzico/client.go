package iyzico

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mstgnz/gopay/provider"
)

const (
	apiSandboxURL    = "https://sandbox-api.iyzipay.com"
	apiProductionURL = "https://api.iyzipay.com"

	// maxAnswerBytes bounds an iyzico answer; the largest (CF retrieve) is a few kilobytes.
	maxAnswerBytes = 1 << 20

	// errorCodeRateLimit is iyzico's "Request Limit Exceeded"; it is no verdict on the request.
	errorCodeRateLimit = "50000"
)

// httpClient is shared by every tenant's provider. Unlike provider.ProviderHTTPClient it verifies
// TLS (the skip there exists for Nkolay's certificate) and is bounded at every phase. 25 s stays
// under the 30 s the handlers give a request.
var httpClient = &http.Client{
	Timeout: 25 * time.Second,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   10,
	},
}

type client struct {
	apiKey, secretKey, baseURL string
	http                       *http.Client
	randomKey                  func() string
}

func newClient(apiKey, secretKey, baseURL string) *client {
	return &client{
		apiKey: apiKey, secretKey: secretKey, baseURL: strings.TrimRight(baseURL, "/"),
		http: httpClient, randomKey: newRandomKey,
	}
}

// authorization builds the IYZWSv2 header: lowercase hex HMAC-SHA256 under the secret key of
// randomKey + path + body, then Base64 of "apiKey:..&randomKey:..&signature:..". path carries no
// host and no query; body is signed byte for byte as sent.
func authorization(apiKey, secretKey, randomKey, path string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secretKey))
	m.Write([]byte(randomKey + path))
	m.Write(body)
	signature := hex.EncodeToString(m.Sum(nil))
	params := "apiKey:" + apiKey + "&randomKey:" + randomKey + "&signature:" + signature
	return "IYZWSv2 " + base64.StdEncoding.EncodeToString([]byte(params))
}

func newRandomKey() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000_000))
	if err != nil {
		// crypto/rand does not fail on supported platforms; a fixed key would weaken nothing
		// secret but must not pass silently.
		panic("iyzico: crypto/rand failed: " + err.Error())
	}
	return fmt.Sprintf("%d%09d", time.Now().UnixMilli(), n.Int64())
}

// envelope is the part every iyzico answer shares.
type envelope struct {
	Status       string     `json:"status"`
	ErrorCode    flexString `json:"errorCode"`
	ErrorMessage string     `json:"errorMessage"`
	Retryable    bool       `json:"retryable"`
}

// do sends a signed request and decodes the answer into out. It returns:
//   - nil for status "success";
//   - *provider.ProviderRefusal for status "failure" (out is still filled from the body, because
//     a declined payment's retrieve carries its paymentStatus there);
//   - an error wrapping provider.ErrProviderUnavailable for everything that is no verdict.
//
// iyzico sends refusals with HTTP 200 or 400, so the body decides, not the status code.
func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("iyzico: encoding %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("iyzico: building %s: %w", path, err)
	}
	rnd := c.randomKey()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-iyzi-rnd", rnd)
	req.Header.Set("Authorization", authorization(c.apiKey, c.secretKey, rnd, path, body))

	res, err := c.http.Do(req)
	if err != nil {
		return unavailable("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, maxAnswerBytes+1))
	if err != nil {
		return unavailable("%s %s: reading the answer: %v", method, path, err)
	}
	if len(raw) > maxAnswerBytes {
		return unavailable("%s %s: answer exceeds %d bytes", method, path, maxAnswerBytes)
	}
	if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
		return unavailable("%s %s answered HTTP %d", method, path, res.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return unavailable("%s %s answered HTTP %d with an unreadable body", method, path, res.StatusCode)
	}
	switch env.Status {
	case "success":
		if res.StatusCode < 200 || res.StatusCode > 299 {
			return unavailable("%s %s answered success with HTTP %d", method, path, res.StatusCode)
		}
	case "failure":
		if string(env.ErrorCode) == errorCodeRateLimit {
			return unavailable("%s %s: rate limited (%s)", method, path, env.ErrorMessage)
		}
		_ = json.Unmarshal(raw, out)
		return &provider.ProviderRefusal{Code: string(env.ErrorCode), Message: env.ErrorMessage, Retryable: env.Retryable}
	default:
		return unavailable("%s %s answered HTTP %d without a status", method, path, res.StatusCode)
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return unavailable("%s %s: decoding the answer: %v", method, path, err)
	}
	return nil
}

func unavailable(format string, args ...any) error {
	return fmt.Errorf("iyzico: %w: %s", provider.ErrProviderUnavailable, fmt.Sprintf(format, args...))
}

// responseSignature is iyzico's signature over the given answer fields: lowercase hex
// HMAC-SHA256 under the secret key of the fields joined with ":". Amounts must already be
// normalised (trailing zeros stripped), as iyzico computes them.
func (c *client) responseSignature(fields ...string) string {
	m := hmac.New(sha256.New, []byte(c.secretKey))
	m.Write([]byte(strings.Join(fields, ":")))
	return hex.EncodeToString(m.Sum(nil))
}

// signatureMatches compares in constant time. An empty signature never matches.
func (c *client) signatureMatches(got string, fields ...string) bool {
	if got == "" {
		return false
	}
	return hmac.Equal([]byte(strings.ToLower(got)), []byte(c.responseSignature(fields...)))
}

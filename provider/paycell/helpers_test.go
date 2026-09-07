package paycell

import (
	"sync"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// pointProviderAtTestServer sends every Paycell call to url and keeps the request log in memory.
// Initialize builds the HTTP clients from the real hosts, and assigning p.baseURL afterwards does
// not move them, so the clients themselves have to be replaced.
func pointProviderAtTestServer(p *PaycellProvider, url string) {
	pointProviderAtTestServerWithTimeout(p, url, 5*time.Second)
}

func pointProviderAtTestServerWithTimeout(p *PaycellProvider, url string, timeout time.Duration) {
	p.baseURL = url
	p.paymentManagementURL = url

	provisionCfg := provider.CreateHTTPClientConfig(url, false)
	provisionCfg.Timeout = timeout
	p.httpClient = provider.NewProviderHTTPClient(provisionCfg)

	managementCfg := provider.CreateHTTPClientConfig(url, false)
	managementCfg.Timeout = timeout
	p.paymentManagementClient = provider.NewProviderHTTPClient(managementCfg)

	if p.logWriter == nil {
		p.logWriter = func(string, map[string]any, int64) {}
	}
}

// logRecorder captures what would have been written to the payment log row.
type logRecorder struct {
	mu      sync.Mutex
	entries map[string]map[string]any
	logIDs  map[string]int64
	order   []string
}

func newLogRecorder() *logRecorder {
	return &logRecorder{
		entries: make(map[string]map[string]any),
		logIDs:  make(map[string]int64),
	}
}

func (r *logRecorder) write(kind string, payload map[string]any, logID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[kind] = payload
	r.logIDs[kind] = logID
	r.order = append(r.order, kind)
}

func (r *logRecorder) get(kind string) (map[string]any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	payload, ok := r.entries[kind]
	return payload, ok
}

func (r *logRecorder) logID(kind string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.logIDs[kind]
}

func (r *logRecorder) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.order...)
}

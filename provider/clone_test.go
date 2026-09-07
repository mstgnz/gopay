package provider

import (
	"context"
	"sync"
	"testing"
)

// stubProvider is the smallest PaymentProvider that exercises GetProvider's cache path. Only
// the mutable per-request fields matter here; every call returns a zero value.
type stubProvider struct {
	logID    int64
	clientIP string
}

func (s *stubProvider) Initialize(map[string]string) error     { return nil }
func (s *stubProvider) GetRequiredConfig(string) []ConfigField { return nil }
func (s *stubProvider) ValidateConfig(map[string]string) error { return nil }
func (s *stubProvider) Clone() PaymentProvider                 { c := *s; return &c }

func (s *stubProvider) GetInstallmentCount(context.Context, InstallmentInquireRequest) (InstallmentInquireResponse, error) {
	return InstallmentInquireResponse{}, nil
}

func (s *stubProvider) CreatePayment(context.Context, PaymentRequest) (*PaymentResponse, error) {
	return nil, nil
}

func (s *stubProvider) Create3DPayment(context.Context, PaymentRequest) (*PaymentResponse, error) {
	return nil, nil
}

func (s *stubProvider) Complete3DPayment(context.Context, *CallbackState, map[string]string) (*PaymentResponse, error) {
	return nil, nil
}

func (s *stubProvider) GetPaymentStatus(context.Context, GetPaymentStatusRequest) (*PaymentResponse, error) {
	return nil, nil
}

func (s *stubProvider) CancelPayment(context.Context, CancelRequest) (*PaymentResponse, error) {
	return nil, nil
}

func (s *stubProvider) RefundPayment(context.Context, RefundRequest) (*RefundResponse, error) {
	return nil, nil
}

func (s *stubProvider) GetCommission(context.Context, CommissionRequest) (CommissionResponse, error) {
	return CommissionResponse{}, nil
}

func (s *stubProvider) ValidateWebhook(context.Context, map[string]string, map[string]string) (bool, map[string]string, error) {
	return false, nil, nil
}

const (
	cloneTestTenant   = 990001
	cloneTestProvider = "stub-clone"
	cloneTestEnv      = "sandbox"
)

func cacheStub(t *testing.T) *stubProvider {
	t.Helper()
	cached := &stubProvider{}
	GetProviderCache().Set(cloneTestTenant, cloneTestProvider, cloneTestEnv, cached)
	t.Cleanup(func() {
		GetProviderCache().Delete(cloneTestTenant, cloneTestProvider, cloneTestEnv)
	})
	return cached
}

func TestGetProviderHandsOutACopyPerCall(t *testing.T) {
	cached := cacheStub(t)

	first, err := GetProvider(cloneTestTenant, cloneTestProvider, cloneTestEnv)
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}
	second, err := GetProvider(cloneTestTenant, cloneTestProvider, cloneTestEnv)
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}

	if first == PaymentProvider(cached) || second == PaymentProvider(cached) {
		t.Fatal("GetProvider returned the cached instance; per-request state would be shared")
	}
	if first == second {
		t.Fatal("two calls returned the same instance; per-request state would be shared")
	}

	// The per-request fields of one caller must not reach the cache or another caller.
	first.(*stubProvider).logID = 42
	first.(*stubProvider).clientIP = "10.0.0.1"

	if cached.logID != 0 || cached.clientIP != "" {
		t.Errorf("cached instance was mutated: logID=%d clientIP=%q", cached.logID, cached.clientIP)
	}
	if got := second.(*stubProvider); got.logID != 0 || got.clientIP != "" {
		t.Errorf("second caller saw the first caller's state: logID=%d clientIP=%q", got.logID, got.clientIP)
	}
}

// Run with -race: before GetProvider cloned, this was a write/write race on one shared struct.
func TestGetProviderIsolatesConcurrentCallers(t *testing.T) {
	cached := cacheStub(t)

	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()

			p, err := GetProvider(cloneTestTenant, cloneTestProvider, cloneTestEnv)
			if err != nil {
				t.Errorf("GetProvider failed: %v", err)
				return
			}

			sp := p.(*stubProvider)
			sp.logID = n
			if sp.logID != n {
				t.Errorf("logID was overwritten by another caller: want %d, got %d", n, sp.logID)
			}
		}(int64(i))
	}
	wg.Wait()

	if cached.logID != 0 {
		t.Errorf("cached instance was mutated by concurrent callers: logID=%d", cached.logID)
	}
}

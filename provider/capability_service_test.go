package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/middle"
)

const (
	capTenant   = 990002
	capProvider = "stub-capable"
	capEnv      = "sandbox"
)

// capableProvider has both optional capabilities; err is returned by every capability call.
type capableProvider struct {
	stubProvider
	err   error
	calls []string
	got   any
}

func (c *capableProvider) Clone() PaymentProvider { return c } // shared on purpose: the test reads calls

func (c *capableProvider) record(name string, req any) { c.calls = append(c.calls, name); c.got = req }

func (c *capableProvider) StartHostedCheckout(_ context.Context, r HostedCheckoutRequest) (*HostedCheckoutSession, error) {
	c.record("start", r)
	if c.err != nil {
		return nil, c.err
	}
	return &HostedCheckoutSession{Token: "tok", PaymentPageURL: "https://p", ConversationID: "c"}, nil
}

func (c *capableProvider) HostedCheckoutResult(_ context.Context, r HostedCheckoutResultRequest) (*HostedCheckoutResult, error) {
	c.record("result", r)
	if c.err != nil {
		return nil, c.err
	}
	return &HostedCheckoutResult{Outcome: OutcomePaid}, nil
}

func (c *capableProvider) CancelHostedPayment(_ context.Context, r HostedCancelRequest) (*HostedCancelResult, error) {
	c.record("cancel", r)
	if c.err != nil {
		return nil, c.err
	}
	return &HostedCancelResult{PaymentID: r.PaymentID}, nil
}

func (c *capableProvider) RefundHostedItem(_ context.Context, r HostedRefundRequest) (*HostedRefundResult, error) {
	c.record("refund", r)
	if c.err != nil {
		return nil, c.err
	}
	return &HostedRefundResult{PaymentTransactionID: r.PaymentTransactionID, Price: r.Price}, nil
}

func (c *capableProvider) CreateSubMerchant(_ context.Context, r SubMerchantRequest) (*SubMerchantResult, error) {
	c.record("create", r)
	if c.err != nil {
		return nil, c.err
	}
	return &SubMerchantResult{SubMerchantKey: "smk"}, nil
}

func (c *capableProvider) UpdateSubMerchant(_ context.Context, r SubMerchantRequest) (*SubMerchantResult, error) {
	c.record("update", r)
	if c.err != nil {
		return nil, c.err
	}
	return &SubMerchantResult{SubMerchantKey: r.Key}, nil
}

func (c *capableProvider) SubMerchantDetail(_ context.Context, r SubMerchantDetailRequest) (*SubMerchantDetail, error) {
	c.record("detail", r)
	if c.err != nil {
		return nil, c.err
	}
	return &SubMerchantDetail{SubMerchantKey: "smk", ExternalID: r.ExternalID}, nil
}

func (c *capableProvider) ApproveItem(_ context.Context, r ItemApprovalRequest) (*ItemApprovalResult, error) {
	c.record("approve", r)
	if c.err != nil {
		return nil, c.err
	}
	return &ItemApprovalResult{PaymentTransactionID: r.PaymentTransactionID}, nil
}

func (c *capableProvider) DisapproveItem(_ context.Context, r ItemApprovalRequest) (*ItemApprovalResult, error) {
	c.record("disapprove", r)
	if c.err != nil {
		return nil, c.err
	}
	return &ItemApprovalResult{PaymentTransactionID: r.PaymentTransactionID}, nil
}

// fakeLogger records what the services log.
type fakeLogger struct {
	requests   []any
	responses  []any
	errorCodes []string
	failLog    bool
}

func (f *fakeLogger) LogRequest(_ context.Context, tenantID int, providerName, method, endpoint string, request any, _, _ string) (int64, error) {
	if f.failLog {
		return 0, errors.New("log table down")
	}
	if tenantID != capTenant || providerName != capProvider {
		return 0, fmt.Errorf("logged for tenant %d provider %s", tenantID, providerName)
	}
	f.requests = append(f.requests, request)
	return int64(len(f.requests)), nil
}

func (f *fakeLogger) LogResponse(_ context.Context, _ int64, response any, _ int64) error {
	f.responses = append(f.responses, response)
	return nil
}

func (f *fakeLogger) LogError(_ context.Context, _ int64, code, _ string, _ int64) error {
	f.errorCodes = append(f.errorCodes, code)
	return nil
}

func capCtx() context.Context {
	return context.WithValue(context.Background(), middle.TenantIDKey, fmt.Sprint(capTenant))
}

func cacheCapable(t *testing.T, err error) *capableProvider {
	t.Helper()
	p := &capableProvider{err: err}
	GetProviderCache().Set(capTenant, capProvider, capEnv, p)
	t.Cleanup(func() { GetProviderCache().Delete(capTenant, capProvider, capEnv) })
	return p
}

func TestCheckoutServiceRunsValidatesAndLogs(t *testing.T) {
	p := cacheCapable(t, nil)
	log := &fakeLogger{}
	s := NewCheckoutService(log)
	ctx := capCtx()

	session, err := s.StartCheckout(ctx, capEnv, capProvider, validCheckout())
	if err != nil || session.Token != "tok" {
		t.Fatalf("start: %+v %v", session, err)
	}
	if got := p.got.(HostedCheckoutRequest); got.LogID != 1 {
		t.Errorf("the provider must receive the log id, got %d", got.LogID)
	}
	if _, err := s.CheckoutResult(ctx, capEnv, capProvider, HostedCheckoutResultRequest{Token: "tok"}); err != nil {
		t.Fatalf("result: %v", err)
	}
	if _, err := s.CancelPayment(ctx, capEnv, capProvider, HostedCancelRequest{PaymentID: "1", IP: "1.2.3.4"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := s.RefundItem(ctx, capEnv, capProvider, HostedRefundRequest{PaymentTransactionID: "7", Price: "1", Currency: "TRY", IP: "1.2.3.4"}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if strings.Join(p.calls, ",") != "start,result,cancel,refund" {
		t.Errorf("calls = %v", p.calls)
	}
	if len(log.requests) != 4 || len(log.responses) != 4 || len(log.errorCodes) != 0 {
		t.Errorf("logged %d requests, %d responses, errors %v", len(log.requests), len(log.responses), log.errorCodes)
	}
}

func TestCheckoutServiceRefusesInvalidRequestsBeforeTheProvider(t *testing.T) {
	p := cacheCapable(t, nil)
	log := &fakeLogger{}
	s := NewCheckoutService(log)
	ctx := capCtx()

	bad := validCheckout()
	bad.Price = "1.00"
	if _, err := s.StartCheckout(ctx, capEnv, capProvider, bad); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("start: %v", err)
	}
	if _, err := s.CheckoutResult(ctx, capEnv, capProvider, HostedCheckoutResultRequest{}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("result: %v", err)
	}
	if _, err := s.CancelPayment(ctx, capEnv, capProvider, HostedCancelRequest{PaymentID: "1"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("cancel: %v", err)
	}
	if _, err := s.RefundItem(ctx, capEnv, capProvider, HostedRefundRequest{PaymentTransactionID: "7"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("refund: %v", err)
	}
	if len(p.calls) != 0 || len(log.requests) != 0 {
		t.Errorf("invalid requests reached the provider %v or the log %d", p.calls, len(log.requests))
	}
}

func TestCapabilityServicesLogTheFailureClass(t *testing.T) {
	cases := map[string]struct {
		err  error
		code string
	}{
		"refusal":     {&ProviderRefusal{Code: "10223"}, "REFUSED_10223"},
		"unavailable": {fmt.Errorf("x: %w", ErrProviderUnavailable), "PROVIDER_UNAVAILABLE"},
		"invalid":     {fmt.Errorf("x: %w", ErrInvalidRequest), "INVALID_REQUEST"},
		"other":       {errors.New("boom"), "PROVIDER_ERROR"},
		"long code":   {&ProviderRefusal{Code: strings.Repeat("9", 60)}, "REFUSED_" + strings.Repeat("9", 42)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cacheCapable(t, tc.err)
			log := &fakeLogger{}
			_, err := NewCheckoutService(log).RefundItem(capCtx(), capEnv, capProvider,
				HostedRefundRequest{PaymentTransactionID: "7", Price: "1", Currency: "TRY", IP: "1.2.3.4"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("the provider error must pass through unchanged, got %v", err)
			}
			if len(log.errorCodes) != 1 || log.errorCodes[0] != tc.code {
				t.Errorf("error codes = %v, want %s", log.errorCodes, tc.code)
			}
		})
	}
}

func TestCapabilityServicesWithoutCapability(t *testing.T) {
	cacheStubAs(t, capProvider)
	ctx := capCtx()
	if _, err := NewCheckoutService(&fakeLogger{}).CheckoutResult(ctx, capEnv, capProvider, HostedCheckoutResultRequest{Token: "t"}); !errors.Is(err, ErrCheckoutUnsupported) {
		t.Errorf("checkout: %v", err)
	}
	if _, err := NewMarketplaceService(&fakeLogger{}).ApproveItem(ctx, capEnv, capProvider, ItemApprovalRequest{PaymentTransactionID: "7"}); !errors.Is(err, ErrMarketplaceUnsupported) {
		t.Errorf("marketplace: %v", err)
	}
}

func cacheStubAs(t *testing.T, name string) {
	t.Helper()
	GetProviderCache().Set(capTenant, name, capEnv, &stubProvider{})
	t.Cleanup(func() { GetProviderCache().Delete(capTenant, name, capEnv) })
}

func TestCapabilityServicesNeedATenant(t *testing.T) {
	if _, err := NewCheckoutService(nil).CheckoutResult(context.Background(), capEnv, capProvider, HostedCheckoutResultRequest{Token: "t"}); err == nil {
		t.Error("checkout without a tenant must fail")
	}
	if _, err := NewMarketplaceService(nil).SubMerchantDetail(context.Background(), capEnv, capProvider, SubMerchantDetailRequest{ExternalID: "x"}); err == nil {
		t.Error("marketplace without a tenant must fail")
	}
}

func TestMarketplaceServiceRunsValidatesAndLogs(t *testing.T) {
	p := cacheCapable(t, nil)
	log := &fakeLogger{}
	s := NewMarketplaceService(log)
	ctx := capCtx()

	if r, err := s.CreateSubMerchant(ctx, capEnv, capProvider, SubMerchantRequest{SubMerchant: SubMerchant{ExternalID: "fac-1"}}); err != nil || r.SubMerchantKey != "smk" {
		t.Fatalf("create: %+v %v", r, err)
	}
	if _, err := s.UpdateSubMerchant(ctx, capEnv, capProvider, SubMerchantRequest{Key: "smk"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := s.SubMerchantDetail(ctx, capEnv, capProvider, SubMerchantDetailRequest{ExternalID: "fac-1"}); err != nil {
		t.Fatalf("detail: %v", err)
	}
	if _, err := s.ApproveItem(ctx, capEnv, capProvider, ItemApprovalRequest{PaymentTransactionID: "7"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := s.DisapproveItem(ctx, capEnv, capProvider, ItemApprovalRequest{PaymentTransactionID: "7"}); err != nil {
		t.Fatalf("disapprove: %v", err)
	}
	if strings.Join(p.calls, ",") != "create,update,detail,approve,disapprove" {
		t.Errorf("calls = %v", p.calls)
	}
	if len(log.requests) != 5 || len(log.responses) != 5 {
		t.Errorf("logged %d requests, %d responses", len(log.requests), len(log.responses))
	}

	p.calls = nil
	for name, call := range map[string]func() error{
		"create with a key": func() error {
			_, err := s.CreateSubMerchant(ctx, capEnv, capProvider, SubMerchantRequest{Key: "k", SubMerchant: SubMerchant{ExternalID: "x"}})
			return err
		},
		"create without id": func() error {
			_, err := s.CreateSubMerchant(ctx, capEnv, capProvider, SubMerchantRequest{})
			return err
		},
		"update without key": func() error {
			_, err := s.UpdateSubMerchant(ctx, capEnv, capProvider, SubMerchantRequest{})
			return err
		},
		"detail without id": func() error {
			_, err := s.SubMerchantDetail(ctx, capEnv, capProvider, SubMerchantDetailRequest{})
			return err
		},
		"approve without tx": func() error { _, err := s.ApproveItem(ctx, capEnv, capProvider, ItemApprovalRequest{}); return err },
		"disapprove blank tx": func() error {
			_, err := s.DisapproveItem(ctx, capEnv, capProvider, ItemApprovalRequest{PaymentTransactionID: " "})
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: want invalid, got %v", name, err)
		}
	}
	if len(p.calls) != 0 {
		t.Errorf("invalid requests reached the provider: %v", p.calls)
	}
}

func TestOperationLogSurvivesALoggingFailure(t *testing.T) {
	cacheCapable(t, nil)
	s := NewCheckoutService(&fakeLogger{failLog: true})
	if _, err := s.CheckoutResult(capCtx(), capEnv, capProvider, HostedCheckoutResultRequest{Token: "t"}); err != nil {
		t.Fatalf("a log failure must not fail the payment call: %v", err)
	}
	o := operationLog{}
	id, started := o.start(capCtx(), capTenant, capProvider, "GET", "/x", nil, "")
	if id != 0 || time.Since(started) > time.Minute {
		t.Errorf("a nil logger must log nothing, got id %d", id)
	}
	o.finish(capCtx(), capProvider, 1, started, nil, nil)
}

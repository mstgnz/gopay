package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/gopay/infra/middle"
)

// A nil TenantID reads every tenant's rows, so only the admin may end up with one.
func TestParseAnalyticsFiltersScopesNonAdminToOwnTenant(t *testing.T) {
	h := &AnalyticsHandler{}
	cases := []struct {
		caller string // tenant id in the JWT context
		param  string // ?tenant_id=
		want   *int   // nil = every tenant
	}{
		{"2", "", intPtr(2)},
		{"2", "all", intPtr(2)},
		{"2", "5", intPtr(2)},
		{"2", "x", intPtr(2)}, // the leak: a non-numeric value used to give every tenant
		{"2", "1", intPtr(2)},
		{"", "5", intPtr(noTenant)},
		{"bogus", "", intPtr(noTenant)},
		{"1", "5", intPtr(5)},
		{"1", "x", nil},
		{"1", "", nil},
		{"1", "all", nil},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/v1/analytics/activity?tenant_id="+tc.param, nil)
		req = req.WithContext(context.WithValue(req.Context(), middle.TenantIDKey, tc.caller))

		got := h.parseAnalyticsFilters(req).TenantID
		switch {
		case tc.want == nil && got != nil:
			t.Errorf("caller %q, tenant_id %q: got tenant %d, want every tenant", tc.caller, tc.param, *got)
		case tc.want != nil && (got == nil || *got != *tc.want):
			t.Errorf("caller %q, tenant_id %q: got %v, want %d", tc.caller, tc.param, got, *tc.want)
		}
	}
}

func intPtr(v int) *int { return &v }

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/infra/middle"
)

func withTenant(r *http.Request, tenantID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middle.TenantIDKey, tenantID))
}

// A stolen tenant login must not be able to rewrite or delete its merchant credentials. The
// refusal has to happen before the body is read or storage is touched, so a nil ProviderConfig
// is enough here: reaching it would panic.
func TestConfigWrites_NonAdminForbidden(t *testing.T) {
	h := NewConfigHandler(nil, nil, nil)
	body := `{"provider":"paycell","environment":"production","configs":[{"key":"merchantId","value":"x"}]}`

	post := withTenant(httptest.NewRequest(http.MethodPost, "/v1/config/tenant", strings.NewReader(body)), "2")
	rr := httptest.NewRecorder()
	h.PostTenantConfig(rr, post)
	if rr.Code != http.StatusForbidden {
		t.Errorf("POST by tenant 2: status = %d, want 403", rr.Code)
	}

	del := withTenant(httptest.NewRequest(http.MethodDelete, "/v1/config/tenant?provider=paycell", nil), "2")
	rr = httptest.NewRecorder()
	h.DeleteTenantConfig(rr, del)
	if rr.Code != http.StatusForbidden {
		t.Errorf("DELETE by tenant 2: status = %d, want 403", rr.Code)
	}
}

func TestConfigWrites_AdminRejectsBadTarget(t *testing.T) {
	h := NewConfigHandler(nil, nil, nil)

	post := withTenant(httptest.NewRequest(http.MethodPost, "/v1/config/tenant", strings.NewReader(`{"tenantId":0}`)), adminTenantID)
	rr := httptest.NewRecorder()
	h.PostTenantConfig(rr, post)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST tenantId=0: status = %d, want 400", rr.Code)
	}

	del := withTenant(httptest.NewRequest(http.MethodDelete, "/v1/config/tenant?provider=paycell&tenant_id=abc", nil), adminTenantID)
	rr = httptest.NewRecorder()
	h.DeleteTenantConfig(rr, del)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DELETE tenant_id=abc: status = %d, want 400", rr.Code)
	}
}

func TestConfigTargetTenant(t *testing.T) {
	if got, err := configTargetTenant("1", nil); err != nil || got != "1" {
		t.Errorf("no target: got %q, %v; want the caller", got, err)
	}
	four := 4
	if got, err := configTargetTenant("1", &four); err != nil || got != "4" {
		t.Errorf("target 4: got %q, %v", got, err)
	}
	negative := -1
	if _, err := configTargetTenant("1", &negative); err == nil {
		t.Error("negative target accepted")
	}
}

func TestMaskTenantConfig(t *testing.T) {
	masked := maskTenantConfig(map[string]map[string]string{
		"production": {
			"username":            "SOMEUSER",
			"password":            "s3cret",
			"secureCode":          "CODE",
			"sx":                  "123|token",
			"eulaId":              "17",
			"compensationEnabled": "true",
			"merchantId":          "",
		},
	})

	prod := masked["production"]
	for _, key := range []string{"username", "password", "secureCode", "sx"} {
		if prod[key] != configMask {
			t.Errorf("%s = %q, want masked", key, prod[key])
		}
	}
	if prod["eulaId"] != "17" || prod["compensationEnabled"] != "true" {
		t.Errorf("non-secret keys altered: %+v", prod)
	}
	if prod["merchantId"] != "" {
		t.Errorf("empty value should stay empty so a missing key is visible, got %q", prod["merchantId"])
	}
}

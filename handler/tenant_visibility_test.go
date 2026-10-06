package handler

import "testing"

func TestVisibleTenants(t *testing.T) {
	all := []map[string]any{
		{"id": 1, "name": "admin"},
		{"id": 2, "name": "tenant-a"},
		{"id": 4, "name": "tenant-b"},
	}

	if got := visibleTenants(all, adminTenantID, true); len(got) != 3 {
		t.Errorf("admin sees %d tenants, want 3", len(got))
	}

	got := visibleTenants(all, "2", false)
	if len(got) != 1 || got[0]["name"] != "tenant-a" {
		t.Errorf("tenant 2 sees %+v, want only itself", got)
	}

	if got := visibleTenants(all, "9", false); len(got) != 0 {
		t.Errorf("unknown tenant sees %+v, want nothing", got)
	}
}

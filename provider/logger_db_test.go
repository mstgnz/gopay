package provider

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/config"
)

// requireTestDB skips unless a throwaway test database is configured. config.App() log.Fatal's
// without Postgres, and these tests write rows, so a database not named *_test is refused.
func requireTestDB(t *testing.T) {
	t.Helper()
	if os.Getenv("DB_HOST") == "" || !strings.HasSuffix(os.Getenv("DB_NAME"), "_test") {
		t.Skip("needs DB_HOST and a DB_NAME ending in _test")
	}
}

func insertPaycellLogRow(t *testing.T, paymentID, requestJSON string) {
	t.Helper()
	var id int64
	err := config.App().DB.QueryRow(
		`INSERT INTO paycell (tenant_id, request, payment_id, endpoint) VALUES (1, $1::jsonb, $2, '/test') RETURNING id`,
		requestJSON, paymentID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert log row: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.App().DB.Exec(`DELETE FROM paycell WHERE id = $1`, id); err != nil {
			t.Errorf("delete log row %d: %v", id, err)
		}
	})
}

func TestGetProviderRequestObjectFromLog(t *testing.T) {
	requireTestDB(t)
	paymentID := fmt.Sprintf("logobj-%d", time.Now().UnixNano())

	insertPaycellLogRow(t, paymentID, `{"result":{"code":"10009","message":"older"}}`)
	insertPaycellLogRow(t, paymentID, `{"result":{"code":"0","message":"newer"}}`)
	insertPaycellLogRow(t, paymentID, `{"other":{"code":"x"}}`)

	got, err := GetProviderRequestObjectFromLog("paycell", paymentID, "result")
	if err != nil {
		t.Fatalf("GetProviderRequestObjectFromLog: %v", err)
	}
	// The newest row holding the key wins, and the object comes back whole from that one row.
	if !strings.Contains(got, `"newer"`) || !strings.Contains(got, `"code": "0"`) {
		t.Errorf("got %s, want the newer result object", got)
	}

	if _, err := GetProviderRequestObjectFromLog("paycell", paymentID, "missing"); !errors.Is(err, ErrLogValueNotFound) {
		t.Errorf("missing key: err = %v, want ErrLogValueNotFound", err)
	}
	if _, err := GetProviderRequestObjectFromLog("paycell", paymentID+"-none", "result"); !errors.Is(err, ErrLogValueNotFound) {
		t.Errorf("unknown payment: err = %v, want ErrLogValueNotFound", err)
	}
}

func TestGetProviderNestedRequestValueFromLog_NotFoundSentinel(t *testing.T) {
	requireTestDB(t)
	paymentID := fmt.Sprintf("lognested-%d", time.Now().UnixNano())
	insertPaycellLogRow(t, paymentID, `{"getThreeDSessionRequest":{"merchantCode":"9998"}}`)

	got, err := GetProviderNestedRequestValueFromLog("paycell", paymentID, "getThreeDSessionRequest", "merchantCode")
	if err != nil || got != "9998" {
		t.Fatalf("got %q, %v; want 9998", got, err)
	}

	_, err = GetProviderNestedRequestValueFromLog("paycell", paymentID, "providerProvisionRequest", "referenceNumber")
	if !errors.Is(err, ErrLogValueNotFound) {
		t.Fatalf("err = %v, want ErrLogValueNotFound", err)
	}
	// Callers and logs read this text; wrapping the sentinel must not change it.
	if err.Error() != "nested key providerProvisionRequest.referenceNumber not found" {
		t.Errorf("message = %q", err.Error())
	}
}

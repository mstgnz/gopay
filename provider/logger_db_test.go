package provider

import (
	"context"
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
	insertPaycellLogRowAt(t, "/test", paymentID, requestJSON)
}

func insertPaycellLogRowAt(t *testing.T, endpoint, paymentID, requestJSON string) {
	t.Helper()
	var id int64
	err := config.App().DB.QueryRow(
		`INSERT INTO paycell (tenant_id, request, payment_id, endpoint) VALUES (1, $1::jsonb, $2, $3) RETURNING id`,
		requestJSON, paymentID, endpoint,
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
	// A JSON null is not a value; it must not shadow the older object.
	insertPaycellLogRow(t, paymentID, `{"result":null}`)

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

func TestGetThreeDCallbackRequest(t *testing.T) {
	requireTestDB(t)
	session := fmt.Sprintf("logcb-%d", time.Now().UnixNano())
	body := func(paymentID, marker string) string {
		return fmt.Sprintf(`{"paymentId":%q,"marker":%q}`, paymentID, marker)
	}

	// LogError leaves callback rows with payment_id = ''; they must still be found.
	insertPaycellLogRowAt(t, Complete3DEndpoint, "", body(session, "older"))
	insertPaycellLogRowAt(t, Complete3DEndpoint, "", body(session, "newer"))
	insertPaycellLogRowAt(t, "/payment/status", session, body(session, "status"))
	insertPaycellLogRowAt(t, Complete3DEndpoint, "", body(session+"-other", "other"))

	got, err := GetThreeDCallbackRequest("paycell", session)
	if err != nil {
		t.Fatalf("GetThreeDCallbackRequest: %v", err)
	}
	if !strings.Contains(got, `"newer"`) {
		t.Errorf("got %s, want the newest callback row of this session", got)
	}

	if _, err := GetThreeDCallbackRequest("paycell", session+"-none"); !errors.Is(err, ErrLogValueNotFound) {
		t.Errorf("no callback: err = %v, want ErrLogValueNotFound", err)
	}
}

func TestGetThreeDSessionAge(t *testing.T) {
	requireTestDB(t)
	session := fmt.Sprintf("logage-%d", time.Now().UnixNano())

	insertPaycellLogRow(t, session, `{"getThreeDSessionRequest":{"merchantCode":"9998"}}`)
	setResponseAt(t, session, "40 minutes")
	insertPaycellLogRow(t, session, `{"paymentId":"status query"}`)

	age, err := GetThreeDSessionAge("paycell", session)
	if err != nil {
		t.Fatalf("GetThreeDSessionAge: %v", err)
	}
	if age < 40*time.Minute || age > 41*time.Minute {
		t.Errorf("age = %v, want about 40m", age)
	}

	// A row without response_at never got payment_id from LogResponse in production; it is not a start.
	pending := session + "-noresponse"
	insertPaycellLogRow(t, pending, `{"getThreeDSessionRequest":{"merchantCode":"9998"}}`)
	if _, err := GetThreeDSessionAge("paycell", pending); !errors.Is(err, ErrLogValueNotFound) {
		t.Errorf("no response_at: err = %v, want ErrLogValueNotFound", err)
	}
	if _, err := GetThreeDSessionAge("paycell", session+"-none"); !errors.Is(err, ErrLogValueNotFound) {
		t.Errorf("unknown session: err = %v, want ErrLogValueNotFound", err)
	}
}

// setResponseAt backdates the response_at of the session's 3D start row, using the database clock
// the way LogResponse does.
func setResponseAt(t *testing.T, paymentID, ago string) {
	t.Helper()
	_, err := config.App().DB.Exec(
		`UPDATE paycell SET response_at = now() - $2::interval WHERE payment_id = $1 AND request -> 'getThreeDSessionRequest' IS NOT NULL`,
		paymentID, ago,
	)
	if err != nil {
		t.Fatalf("set response_at: %v", err)
	}
}

func TestCallbackStateExists(t *testing.T) {
	requireTestDB(t)
	session := fmt.Sprintf("cbstate-%d", time.Now().UnixNano())

	var id int64
	err := config.App().DB.QueryRow(
		`INSERT INTO callbacks (tenant_id, provider, payment_id, state_data, expires_at) VALUES (1, 'paycell', $1, '{}'::jsonb, now()) RETURNING id`,
		session,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert callbacks row: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.App().DB.Exec(`DELETE FROM callbacks WHERE id = $1`, id); err != nil {
			t.Errorf("delete callbacks row %d: %v", id, err)
		}
	})

	ctx := context.Background()
	for _, tt := range []struct {
		provider, paymentID string
		want                bool
	}{
		{"paycell", session, true},
		{"nkolay", session, false},
		{"paycell", session + "-none", false},
	} {
		got, err := CallbackStateExists(ctx, tt.provider, tt.paymentID)
		if err != nil {
			t.Fatalf("CallbackStateExists(%s, %s): %v", tt.provider, tt.paymentID, err)
		}
		if got != tt.want {
			t.Errorf("CallbackStateExists(%s, %s) = %v, want %v", tt.provider, tt.paymentID, got, tt.want)
		}
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

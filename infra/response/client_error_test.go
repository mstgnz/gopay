package response

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lib/pq"
)

func errorField(t *testing.T, err error) string {
	t.Helper()
	w := httptest.NewRecorder()
	Error(w, http.StatusInternalServerError, "Failed to get payment status", err)
	var body Response
	if jerr := json.Unmarshal(w.Body.Bytes(), &body); jerr != nil {
		t.Fatal(jerr)
	}
	return body.Error
}

// A database driver error names tables and constraints; a wrapped one must not reach the client.
func TestErrorKeepsDatabaseErrorsOutOfTheResponse(t *testing.T) {
	dbErr := fmt.Errorf("failed to retrieve callback state: %w",
		&pq.Error{Code: "42P01", Message: `relation "callbacks" does not exist`})
	if got := errorField(t, dbErr); got != internalErrorText {
		t.Errorf("error = %q, want %q", got, internalErrorText)
	}
	for _, connErr := range []error{driver.ErrBadConn, sql.ErrConnDone, sql.ErrTxDone} {
		if got := errorField(t, fmt.Errorf("failed to store callback state: %w", connErr)); got != internalErrorText {
			t.Errorf("%v: error = %q, want %q", connErr, got, internalErrorText)
		}
	}
}

// Consumers read other error text: sovtajyeri branches on "not found", assistants on a log-key
// message, and every consumer shows provider decline reasons.
func TestErrorKeepsOtherErrorText(t *testing.T) {
	for _, text := range []string{
		"paycell: Order is not found",
		"failed to get reference number: pay-1 providerProvisionRequest.referenceNumber not found",
	} {
		if got := errorField(t, errors.New(text)); got != text {
			t.Errorf("error = %q, want it unchanged", got)
		}
	}
}

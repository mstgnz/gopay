package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The legacy encrypted state was self-describing (tenant, amount, redirect target) and could be
// minted by anyone holding the fallback key. Anything that is not a stored callback id must now
// be refused before the database or a provider is touched.
func TestHandleCallbackState_RejectsNonNumericState(t *testing.T) {
	for _, state := range []string{
		"",
		"test-encrypted-state",
		"eyJ0ZW5hbnRJZCI6Mn0=",
		"12abc",
		"1 OR 1=1",
	} {
		if _, err := HandleCallbackState(context.Background(), "paycell", state); !errors.Is(err, ErrInvalidCallbackState) {
			t.Errorf("state %q: err = %v, want ErrInvalidCallbackState", state, err)
		}
	}
}

// The route a callback URL names and the provider stored with the state must agree; otherwise the
// payer's callback would be refused. Checked before anything is stored.
func TestCreateShortCallbackURL_RefusesProviderMismatch(t *testing.T) {
	_, err := CreateShortCallbackURL(context.Background(), "https://gopay.example", "paycell", CallbackState{TenantID: 1, Provider: "ziraat"})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("err = %v, want a provider mismatch", err)
	}
}

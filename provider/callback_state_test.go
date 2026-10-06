package provider

import (
	"context"
	"errors"
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
		if _, err := HandleCallbackState(context.Background(), state); !errors.Is(err, ErrInvalidCallbackState) {
			t.Errorf("state %q: err = %v, want ErrInvalidCallbackState", state, err)
		}
	}
}

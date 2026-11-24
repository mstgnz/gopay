package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The legacy encrypted state was self-describing (tenant, amount, redirect target) and could be
// minted by anyone holding the fallback key, and the numeric ids after it could be walked. Anything
// that is not a token must be refused before the database or a provider is touched.
func TestHandleCallbackState_RejectsMalformedState(t *testing.T) {
	for _, state := range []string{
		"",
		"test-encrypted-state",
		"eyJ0ZW5hbnRJZCI6Mn0=",
		"12abc",
		"1 OR 1=1",
		"17359", // a numeric id, the pre-token state
		"0",
		"-5",
		"abcdefghijklmnopqrstu",   // 21 characters
		"abcdefghijklmnopqrstuvw", // 23 characters
		"abcdefghijklmnopqrst+/",  // not URL-safe
	} {
		if _, err := HandleCallbackState(context.Background(), "paycell", state); !errors.Is(err, ErrInvalidCallbackState) {
			t.Errorf("state %q: err = %v, want ErrInvalidCallbackState", state, err)
		}
	}
}

func TestCallbackTokensAreRandomURLSafeAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		token, hash, err := newCallbackToken()
		if err != nil {
			t.Fatalf("newCallbackToken: %v", err)
		}
		if !callbackTokenPattern.MatchString(token) || seen[token] {
			t.Fatalf("token %q malformed or repeated", token)
		}
		seen[token] = true
		if hash != callbackTokenHash(token) || len(hash) != 64 || hash == token {
			t.Fatalf("hash %q for %q", hash, token)
		}
		got, err := parseCallbackState(token)
		if err != nil || got != hash {
			t.Fatalf("parse %q = %q, %v", token, got, err)
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

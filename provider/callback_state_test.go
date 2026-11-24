package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The legacy encrypted state was self-describing (tenant, amount, redirect target) and could be
// minted by anyone holding the fallback key. Anything that is neither a token nor a canonical
// numeric id must be refused before the database or a provider is touched.
func TestHandleCallbackState_RejectsMalformedState(t *testing.T) {
	for _, state := range []string{
		"",
		"test-encrypted-state",
		"eyJ0ZW5hbnRJZCI6Mn0=",
		"12abc",
		"1 OR 1=1",
		"0",
		"-5",
		"+5",
		"007",
		"abcdefghijklmnopqrstu",   // 21 characters
		"abcdefghijklmnopqrstuvw", // 23 characters
		"abcdefghijklmnopqrst+/",  // not URL-safe
		"3000000000",              // beyond int4: callbacks.id
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
		key, err := parseCallbackState(token)
		if err != nil || key.tokenHash != hash || key.legacyID != nil {
			t.Fatalf("parse %q = %+v, %v", token, key, err)
		}
	}
	if key, err := parseCallbackState("123456"); err != nil || key.legacyID != 123456 || key.tokenHash != nil {
		t.Errorf("legacy id parse = %+v, %v", key, err)
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

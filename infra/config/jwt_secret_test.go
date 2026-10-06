package config

import (
	"strings"
	"testing"
)

// cmd/main.go refuses to start on an error here; a production secret is 64 characters.
func TestCheckJWTSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		wantErr  bool
		wantWeak bool
	}{
		{"unset", "", true, false},
		{"public fallback", fallbackJWTSecret, true, false},
		{"short", "only-twenty-chars-xx", false, true},
		{"production length", strings.Repeat("k", 64), false, false},
		{"exactly the minimum", strings.Repeat("k", minJWTSecretLen), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weak, err := CheckJWTSecret(tt.secret)
			if (err != nil) != tt.wantErr || weak != tt.wantWeak {
				t.Fatalf("CheckJWTSecret = weak %v, err %v; want weak %v, err %v", weak, err, tt.wantWeak, tt.wantErr)
			}
		})
	}
}

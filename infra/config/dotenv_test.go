package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The image no longer ships a .env, so a missing file must not stop the process.
func TestLoadDotEnv_MissingFileIsNotAnError(t *testing.T) {
	loaded, err := LoadDotEnv(filepath.Join(t.TempDir(), ".env"))
	if err != nil || loaded {
		t.Fatalf("LoadDotEnv(missing) = %v, %v; want false, nil", loaded, err)
	}
}

// compose env_file sets the real values; a stale .env baked into an old image must not win.
func TestLoadDotEnv_ProcessEnvironmentWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("GOPAY_DOTENV_TEST_A=from-file\nGOPAY_DOTENV_TEST_B=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPAY_DOTENV_TEST_A", "from-process")
	t.Setenv("GOPAY_DOTENV_TEST_B", "")
	os.Unsetenv("GOPAY_DOTENV_TEST_B")

	loaded, err := LoadDotEnv(path)
	if err != nil || !loaded {
		t.Fatalf("LoadDotEnv = %v, %v; want true, nil", loaded, err)
	}
	if got := os.Getenv("GOPAY_DOTENV_TEST_A"); got != "from-process" {
		t.Errorf("A = %q, the file overrode the process environment", got)
	}
	if got := os.Getenv("GOPAY_DOTENV_TEST_B"); got != "from-file" {
		t.Errorf("B = %q, want the file value for an unset variable", got)
	}
}

func TestLoadDotEnv_UnreadablePathIsAnError(t *testing.T) {
	// A directory exists but cannot be read as a file: that is a real misconfiguration.
	if _, err := LoadDotEnv(t.TempDir()); err == nil {
		t.Fatal("expected an error for a path that exists but is not a readable file")
	}
}

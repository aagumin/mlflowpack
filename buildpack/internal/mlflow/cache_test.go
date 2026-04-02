package mlflow

import (
	"testing"
)

func TestPrevDepsHashFromEnv(t *testing.T) {
	// Test empty env
	t.Run("empty env returns empty string", func(t *testing.T) {
		got := PrevDepsHashFromEnv()
		if got != "" {
			t.Errorf("PrevDepsHashFromEnv() = %q, want empty string", got)
		}
	})

	// Test with env set
	t.Run("env set returns value", func(t *testing.T) {
		t.Setenv(EnvPrevDepsHash, "sha256:abc123")
		got := PrevDepsHashFromEnv()
		if got != "sha256:abc123" {
			t.Errorf("PrevDepsHashFromEnv() = %q, want %q", got, "sha256:abc123")
		}
	})
}

func TestCachedDepsHashEmpty(t *testing.T) {
	// Test with non-existent layer
	got := CachedDepsHash("/nonexistent/path")
	if got != "" {
		t.Errorf("CachedDepsHash() = %q, want empty string", got)
	}
}

func TestCachedPythonVersionEmpty(t *testing.T) {
	// Test with non-existent layer
	got := CachedPythonVersion("/nonexistent/path")
	if got != "" {
		t.Errorf("CachedPythonVersion() = %q, want empty string", got)
	}
}

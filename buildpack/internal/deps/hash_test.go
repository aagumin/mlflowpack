// Package deps provides dependency hash computation for layer caching.
package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeHash(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	condaYaml := `
name: test
dependencies:
  - python=3.11
  - pip:
    - pandas==2.0.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte(condaYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	requirements := `pandas==2.0.0
numpy==1.24.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(requirements), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() error = %v", err)
	}

	if hash1 == "" {
		t.Error("ComputeHash() returned empty string")
	}

	// Same files should produce same hash
	hash2, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() second call error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("ComputeHash() not deterministic: %s != %s", hash1, hash2)
	}
}

func TestComputeHashEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	hash, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() error = %v", err)
	}

	// Empty dir should still produce a hash (default dependencies)
	if hash == "" {
		t.Error("ComputeHash() returned empty string for empty dir")
	}
}

func TestComputeHashChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial conda.yaml
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte("name: test"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Modify conda.yaml
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte("name: test2"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash2, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("ComputeHash() should change when files change")
	}
}

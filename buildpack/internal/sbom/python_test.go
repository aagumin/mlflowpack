// buildpack/internal/sbom/python_test.go
package sbom_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/sbom"
)

func TestParseMetadata(t *testing.T) {
	metadataPath := filepath.Join("testdata", "numpy-1.26.0.dist-info", "METADATA")
	pkg, err := sbom.ParseMetadata(metadataPath)
	if err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}

	if pkg.Name != "numpy" {
		t.Errorf("expected name numpy, got %s", pkg.Name)
	}
	if pkg.Version != "1.26.0" {
		t.Errorf("expected version 1.26.0, got %s", pkg.Version)
	}
	if pkg.License != "BSD-3-Clause" {
		t.Errorf("expected license BSD-3-Clause, got %s", pkg.License)
	}
}

func TestParseMetadataNormalizedPURL(t *testing.T) {
	pkg := sbom.Package{
		Name:    "scikit_learn",
		Version: "1.3.0",
	}
	purl := pkg.PURL()

	if purl != "pkg:pypi/scikit-learn@1.3.0" {
		t.Errorf("expected normalized purl, got %s", purl)
	}
}

func TestParseVenv(t *testing.T) {
	venvPath := filepath.Join("testdata", "sample-venv")
	packages, err := sbom.ParseVenv(venvPath)
	if err != nil {
		t.Fatalf("failed to parse venv: %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(packages))
	}

	foundNumpy := false
	for _, pkg := range packages {
		if pkg.Name == "numpy" {
			foundNumpy = true
			if pkg.Version != "1.26.0" {
				t.Errorf("numpy version mismatch: %s", pkg.Version)
			}
		}
	}
	if !foundNumpy {
		t.Error("numpy package not found in venv")
	}
}

func TestMain(m *testing.M) {
	// Create test fixtures
	if err := os.MkdirAll(filepath.Join("testdata", "numpy-1.26.0.dist-info"), 0o755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "numpy-1.26.0.dist-info"), 0o755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "mlserver-1.3.0.dist-info"), 0o755); err != nil {
		panic(err)
	}

	numpyMeta := `Metadata-Version: 2.1
Name: numpy
Version: 1.26.0
License: BSD-3-Clause
Home-page: https://numpy.org
`
	if err := os.WriteFile(filepath.Join("testdata", "numpy-1.26.0.dist-info", "METADATA"), []byte(numpyMeta), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "numpy-1.26.0.dist-info", "METADATA"), []byte(numpyMeta), 0o644); err != nil {
		panic(err)
	}

	mlserverMeta := `Metadata-Version: 2.1
Name: mlserver
Version: 1.3.0
License: Apache-2.0
`
	if err := os.WriteFile(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "mlserver-1.3.0.dist-info", "METADATA"), []byte(mlserverMeta), 0o644); err != nil {
		panic(err)
	}

	code := m.Run()

	if err := os.RemoveAll("testdata"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup testdata: %v\n", err)
	}
	os.Exit(code)
}

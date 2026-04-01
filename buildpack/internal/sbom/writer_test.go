// buildpack/internal/sbom/writer_test.go
package sbom_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/sbom"
)

func TestWriteLayerSBOM(t *testing.T) {
	tmpDir := t.TempDir()

	packages := []sbom.Package{
		{Name: "numpy", Version: "1.26.0", License: "BSD-3-Clause"},
		{Name: "mlserver", Version: "1.3.0", License: "Apache-2.0"},
	}

	err := sbom.WriteLayerSBOM(tmpDir, "venv", packages)
	if err != nil {
		t.Fatalf("failed to write SBOM: %v", err)
	}

	sbomPath := filepath.Join(tmpDir, "venv.sbom.cdx.json")
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Fatalf("SBOM file not created at %s", sbomPath)
	}

	data, err := os.ReadFile(sbomPath)
	if err != nil {
		t.Fatalf("failed to read SBOM: %v", err)
	}

	content := string(data)
	if !containsString(content, `"bomFormat": "CycloneDX"`) {
		t.Error("SBOM missing bomFormat")
	}
	if !containsString(content, "numpy") {
		t.Error("SBOM missing numpy component")
	}
	if !containsString(content, "pkg:pypi/numpy@1.26.0") {
		t.Error("SBOM missing numpy purl")
	}
}

func TestWritePythonSBOM(t *testing.T) {
	tmpDir := t.TempDir()

	err := sbom.WritePythonSBOM(tmpDir, "3.10.13")
	if err != nil {
		t.Fatalf("failed to write Python SBOM: %v", err)
	}

	sbomPath := filepath.Join(tmpDir, "python.sbom.cdx.json")
	data, err := os.ReadFile(sbomPath)
	if err != nil {
		t.Fatalf("failed to read SBOM: %v", err)
	}

	content := string(data)
	if !containsString(content, "cpython") {
		t.Error("Python SBOM missing cpython component")
	}
	if !containsString(content, "3.10.13") {
		t.Error("Python SBOM missing version")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

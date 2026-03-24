// buildpack/internal/sbom/cyclonedx_test.go
package sbom_test

import (
	"encoding/json"
	"testing"

	"github.com/aagumin/mlflowpack/internal/sbom"
)

func TestBOMJSONMarshaling(t *testing.T) {
	bom := sbom.BOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Components: []sbom.Component{
			{
				Type:    "library",
				Name:    "numpy",
				Version: "1.26.0",
				PURL:    "pkg:pypi/numpy@1.26.0",
				Licenses: []sbom.License{
					{Expression: "BSD-3-Clause"},
				},
			},
		},
	}

	data, err := json.Marshal(bom)
	if err != nil {
		t.Fatalf("failed to marshal BOM: %v", err)
	}

	expected := `"bomFormat":"CycloneDX"`
	if !contains(string(data), expected) {
		t.Errorf("expected JSON to contain %s, got %s", expected, string(data))
	}
}

func TestComponentPURLGeneration(t *testing.T) {
	comp := sbom.Component{
		Type:    "library",
		Name:    "scikit-learn",
		Version: "1.3.0",
	}
	comp.SetPURL()

	if comp.PURL != "pkg:pypi/scikit-learn@1.3.0" {
		t.Errorf("expected purl pkg:pypi/scikit-learn@1.3.0, got %s", comp.PURL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

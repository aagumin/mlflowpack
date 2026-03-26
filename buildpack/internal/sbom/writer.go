// buildpack/internal/sbom/writer.go
package sbom

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteLayerSBOM creates a CycloneDX SBOM file for a layer containing Python packages.
func WriteLayerSBOM(layersDir, layerName string, packages []Package) error {
	bom := NewBOM()

	for _, pkg := range packages {
		comp := Component{
			Type:    "library",
			Name:    pkg.Name,
			Version: pkg.Version,
			PURL:    pkg.PURL(),
		}

		if pkg.License != "" {
			comp.Licenses = []License{{Expression: pkg.License}}
		}

		bom.Components = append(bom.Components, comp)
	}

	outputPath := filepath.Join(layersDir, layerName+".sbom.cdx.json")
	return writeJSON(outputPath, bom)
}

// WritePythonSBOM creates a CycloneDX SBOM file for the Python layer.
func WritePythonSBOM(layersDir, pythonVersion string) error {
	bom := NewBOM()

	bom.Components = []Component{
		{
			Type:    "application",
			Name:    "cpython",
			Version: pythonVersion,
			PURL:    "pkg:generic/cpython@" + pythonVersion,
			ExternalRefs: []ExternalReference{
				{
					Type: "distribution",
					URL:  "https://www.python.org/downloads/release/python-" + versionToURL(pythonVersion),
				},
			},
		},
	}

	outputPath := filepath.Join(layersDir, "python.sbom.cdx.json")
	return writeJSON(outputPath, bom)
}

// ExternalReference represents an external reference in CycloneDX.
type ExternalReference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func versionToURL(version string) string {
	v := ""
	for _, c := range version {
		if c >= '0' && c <= '9' {
			v += string(c)
		}
	}
	return v
}

func writeJSON(path string, data interface{}) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating SBOM file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing %q: %w", path, closeErr))
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding SBOM JSON: %w", err)
	}

	return nil
}

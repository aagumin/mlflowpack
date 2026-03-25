# SBOM Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add CycloneDX SBOM generation for Python and venv layers in the MLflow buildpack.

**Architecture:** Create a separate `sbom` package with CycloneDX types, Python METADATA parser, and SBOM writer. Integrate into builder.go after Python and venv layer creation.

**Tech Stack:** Go 1.25, CycloneDX JSON 1.5, CNB Buildpack API 0.12

---

## Task 1: Create CycloneDX Types

**Files:**
- Create: `buildpack/internal/sbom/cyclonedx.go`
- Create: `buildpack/internal/sbom/cyclonedx_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: FAIL with "package sbom is not in GOROOT"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/sbom/cyclonedx.go
package sbom

import "strings"

// BOM represents a CycloneDX Software Bill of Materials.
type BOM struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Components  []Component `json:"components,omitempty"`
}

// Component represents a single component in the BOM.
type Component struct {
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Version  string    `json:"version,omitempty"`
	PURL     string    `json:"purl,omitempty"`
	Licenses []License `json:"licenses,omitempty"`
}

// License represents a software license.
type License struct {
	Expression string `json:"expression,omitempty"`
}

// SetPURL generates and sets the Package URL for a PyPI package.
func (c *Component) SetPURL() {
	name := strings.ToLower(c.Name)
	name = strings.ReplaceAll(name, "_", "-")
	c.PURL = "pkg:pypi/" + name + "@" + c.Version
}

// NewBOM creates a new CycloneDX BOM with default values.
func NewBOM() *BOM {
	return &BOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Components:  []Component{},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/sbom/cyclonedx.go buildpack/internal/sbom/cyclonedx_test.go
git commit -m "feat(sbom): add CycloneDX types and BOM structure"
```

---

## Task 2: Create Python METADATA Parser

**Files:**
- Create: `buildpack/internal/sbom/python.go`
- Create: `buildpack/internal/sbom/python_test.go`
- Create test fixture: `buildpack/internal/sbom/testdata/numpy-1.26.0.dist-info/METADATA`

**Step 1: Write the failing test**

```go
// buildpack/internal/sbom/python_test.go
package sbom_test

import (
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
	// Test package with underscores in name
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

	// Check that numpy was found
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
	os.MkdirAll(filepath.Join("testdata", "numpy-1.26.0.dist-info"), 0755)
	os.MkdirAll(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "numpy-1.26.0.dist-info"), 0755)
	os.MkdirAll(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "mlserver-1.3.0.dist-info"), 0755)

	// Write numpy METADATA
	numpyMeta := `Metadata-Version: 2.1
Name: numpy
Version: 1.26.0
License: BSD-3-Clause
Home-page: https://numpy.org
`
	os.WriteFile(filepath.Join("testdata", "numpy-1.26.0.dist-info", "METADATA"), []byte(numpyMeta), 0644)
	os.WriteFile(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "numpy-1.26.0.dist-info", "METADATA"), []byte(numpyMeta), 0644)

	// Write mlserver METADATA
	mlserverMeta := `Metadata-Version: 2.1
Name: mlserver
Version: 1.3.0
License: Apache-2.0
`
	os.WriteFile(filepath.Join("testdata", "sample-venv", "lib", "python3.10", "site-packages", "mlserver-1.3.0.dist-info", "METADATA"), []byte(mlserverMeta), 0644)

	code := m.Run()

	// Cleanup
	os.RemoveAll("testdata")
	os.Exit(code)
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: FAIL with "undefined: sbom.ParseMetadata"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/sbom/python.go
package sbom

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Package represents a Python package with its metadata.
type Package struct {
	Name    string
	Version string
	License string
}

// PURL returns the Package URL for this Python package.
func (p Package) PURL() string {
	name := strings.ToLower(p.Name)
	name = strings.ReplaceAll(name, "_", "-")
	return "pkg:pypi/" + name + "@" + p.Version
}

// ParseMetadata parses a Python METADATA file and returns package information.
func ParseMetadata(path string) (Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return Package{}, fmt.Errorf("opening metadata file: %w", err)
	}
	defer file.Close()

	pkg := Package{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Name:") {
			pkg.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		} else if strings.HasPrefix(line, "Version:") {
			pkg.Version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		} else if strings.HasPrefix(line, "License:") {
			pkg.License = strings.TrimSpace(strings.TrimPrefix(line, "License:"))
		} else if strings.HasPrefix(line, "License-Expression:") {
			// Prefer License-Expression if both exist
			pkg.License = strings.TrimSpace(strings.TrimPrefix(line, "License-Expression:"))
		}
	}

	if err := scanner.Err(); err != nil {
		return Package{}, fmt.Errorf("reading metadata file: %w", err)
	}

	return pkg, nil
}

// ParseVenv scans a virtual environment for installed packages.
func ParseVenv(venvPath string) ([]Package, error) {
	// Find site-packages directory
	sitePackagesPattern := filepath.Join(venvPath, "lib", "python*", "site-packages")
	sitePackagesDirs, err := filepath.Glob(sitePackagesPattern)
	if err != nil {
		return nil, fmt.Errorf("finding site-packages: %w", err)
	}
	if len(sitePackagesDirs) == 0 {
		return nil, fmt.Errorf("no site-packages directory found in %s", venvPath)
	}

	sitePackages := sitePackagesDirs[0]
	var packages []Package

	// Find all .dist-info directories
	distInfoPattern := filepath.Join(sitePackages, "*.dist-info")
	distInfoDirs, err := filepath.Glob(distInfoPattern)
	if err != nil {
		return nil, fmt.Errorf("finding dist-info directories: %w", err)
	}

	for _, distInfoDir := range distInfoDirs {
		metadataPath := filepath.Join(distInfoDir, "METADATA")
		if _, err := os.Stat(metadataPath); err != nil {
			continue
		}

		pkg, err := ParseMetadata(metadataPath)
		if err != nil {
			// Log warning but continue
			continue
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/sbom/python.go buildpack/internal/sbom/python_test.go
git commit -m "feat(sbom): add Python METADATA parser"
```

---

## Task 3: Create SBOM Writer

**Files:**
- Create: `buildpack/internal/sbom/writer.go`
- Create: `buildpack/internal/sbom/writer_test.go`

**Step 1: Write the failing test**

```go
// buildpack/internal/sbom/writer_test.go
package sbom_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/sbom"
)

func TestWriteLayerSBOM(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sbom-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packages := []sbom.Package{
		{Name: "numpy", Version: "1.26.0", License: "BSD-3-Clause"},
		{Name: "mlserver", Version: "1.3.0", License: "Apache-2.0"},
	}

	err = sbom.WriteLayerSBOM(tmpDir, "venv", packages)
	if err != nil {
		t.Fatalf("failed to write SBOM: %v", err)
	}

	// Verify file was created
	sbomPath := filepath.Join(tmpDir, "venv.sbom.cdx.json")
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Fatalf("SBOM file not created at %s", sbomPath)
	}

	// Verify content
	data, err := os.ReadFile(sbomPath)
	if err != nil {
		t.Fatalf("failed to read SBOM: %v", err)
	}

	content := string(data)
	if !containsString(content, `"bomFormat":"CycloneDX"`) {
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
	tmpDir, err := os.MkdirTemp("", "sbom-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = sbom.WritePythonSBOM(tmpDir, "3.10.13")
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
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: FAIL with "undefined: sbom.WriteLayerSBOM"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/sbom/writer.go
package sbom

import (
	"encoding/json"
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

// Add ExternalRefs to Component
type ComponentExt struct {
	Type         string              `json:"type"`
	Name         string              `json:"name"`
	Version      string              `json:"version,omitempty"`
	PURL         string              `json:"purl,omitempty"`
	Licenses     []License           `json:"licenses,omitempty"`
	ExternalRefs []ExternalReference `json:"externalReferences,omitempty"`
}

func versionToURL(version string) string {
	// Convert 3.10.13 to 31013
	v := ""
	for _, c := range version {
		if c >= '0' && c <= '9' {
			v += string(c)
		}
	}
	return v
}

func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating SBOM file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding SBOM JSON: %w", err)
	}

	return nil
}
```

**Step 4: Update cyclonedx.go to include ExternalRefs**

```go
// Add to Component struct in cyclonedx.go:
type Component struct {
	Type         string              `json:"type"`
	Name         string              `json:"name"`
	Version      string              `json:"version,omitempty"`
	PURL         string              `json:"purl,omitempty"`
	Licenses     []License           `json:"licenses,omitempty"`
	ExternalRefs []ExternalReference `json:"externalReferences,omitempty"`
}

// Add ExternalReference type:
type ExternalReference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
```

**Step 5: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/sbom/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add buildpack/internal/sbom/writer.go buildpack/internal/sbom/writer_test.go buildpack/internal/sbom/cyclonedx.go
git commit -m "feat(sbom): add SBOM writer for layers"
```

---

## Task 4: Update buildpack.toml

**Files:**
- Modify: `buildpack/buildpack.toml`

**Step 1: Add sbom-formats field**

```toml
api = "0.12"

[buildpack]
id = "io.github.aagumin.mlflow-model"
version = "0.1.0"
name = "MLflow Model Buildpack"
homepage = "https://github.com/aagumin/mlflowpack"
description = "Buildpack for MLflow models with MLServer runtime"
sbom-formats = ["application/vnd.cyclonedx+json"]

# Modern targets approach (replaces deprecated stacks)
[[targets]]
os = "linux"
arch = "amd64"

[[targets]]
os = "linux"
arch = "arm64"

[metadata]
include-files = ["bin/detect", "bin/build", "bin/linux-amd64", "bin/linux-arm64"]
```

**Step 2: Verify buildpack.toml is valid**

Run: `cat buildpack/buildpack.toml`
Expected: Valid TOML with sbom-formats field

**Step 3: Commit**

```bash
git add buildpack/buildpack.toml
git commit -m "feat(buildpack): declare CycloneDX SBOM format support"
```

---

## Task 5: Integrate SBOM into Builder

**Files:**
- Modify: `buildpack/internal/build/builder.go`
- Modify: `buildpack/internal/layer/layers.go` (add constants if needed)

**Step 1: Add import for sbom package**

In `buildpack/internal/build/builder.go`, add to imports:

```go
import (
	// ... existing imports ...
	"github.com/aagumin/mlflowpack/internal/sbom"
)
```

**Step 2: Add SBOM writing after Python installation**

After line ~107 (after `installer.SetupFromConda`), add:

```go
// Write Python SBOM
pythonVersion := dependencies.conda.PythonVersion()
if pythonVersion == "" {
	pythonVersion = python.DefaultPythonVersion
}
if err := sbom.WritePythonSBOM(ctx.LayersDir, pythonVersion); err != nil {
	return result, fmt.Errorf("writing python SBOM: %w", err)
}
```

**Step 3: Add SBOM writing after venv installation**

After line ~113 (after `installer.InstallDepsFromFile`), add:

```go
// Write venv SBOM
packages, err := sbom.ParseVenv(venvPath)
if err != nil {
	return result, fmt.Errorf("parsing venv for SBOM: %w", err)
}
if err := sbom.WriteLayerSBOM(ctx.LayersDir, layer.VenvLayerName, packages); err != nil {
	return result, fmt.Errorf("writing venv SBOM: %w", err)
}
```

**Step 4: Run tests to verify integration**

Run: `cd buildpack && go test ./internal/build/... -v`
Expected: PASS

**Step 5: Run full build test**

Run: `cd buildpack && go build ./...`
Expected: No compilation errors

**Step 6: Commit**

```bash
git add buildpack/internal/build/builder.go
git commit -m "feat(build): integrate SBOM generation for python and venv layers"
```

---

## Task 6: Run Full Test Suite

**Files:**
- All modified files

**Step 1: Run all tests**

Run: `cd buildpack && go test ./... -v`
Expected: All tests PASS

**Step 2: Run linter**

Run: `cd buildpack && golangci-lint run`
Expected: No errors (or only pre-existing warnings)

**Step 3: Build binaries**

Run: `cd buildpack && go build -o bin/build ./cmd/build && go build -o bin/detect ./cmd/detect`
Expected: Binaries created successfully

**Step 4: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "test(sbom): verify full test suite passes"
```

---

## Task 7: Manual Integration Test

**Step 1: Build the buildpack**

Run: `make package`
Expected: Buildpack packaged successfully

**Step 2: Build a test app with SBOM**

Run: `pack build test-mlflow-app --builder <your-builder> --sbom-output-dir ./sbom-output`
Expected: Build succeeds, SBOM files in ./sbom-output

**Step 3: Verify SBOM content**

Run: `cat ./sbom-output/*/sbom.cdx.json | jq .`
Expected: Valid CycloneDX JSON with Python and pip packages

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | CycloneDX types | `sbom/cyclonedx.go`, `sbom/cyclonedx_test.go` |
| 2 | Python METADATA parser | `sbom/python.go`, `sbom/python_test.go` |
| 3 | SBOM writer | `sbom/writer.go`, `sbom/writer_test.go` |
| 4 | buildpack.toml update | `buildpack.toml` |
| 5 | Builder integration | `build/builder.go` |
| 6 | Full test suite | All files |
| 7 | Manual verification | N/A |

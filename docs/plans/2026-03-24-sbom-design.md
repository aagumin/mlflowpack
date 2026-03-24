# SBOM Support for MLflow Buildpack

## Overview

Add Software Bill of Materials (SBOM) support to the MLflow buildpack using CycloneDX format. SBOM files will be generated for Python and venv layers during the build phase.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Layers to include | python + venv | Covers Python runtime and all pip packages |
| Metadata source | METADATA files only | No external API calls, faster builds |
| Format | CycloneDX only | Most widely supported, sufficient for security tools |
| Architecture | Separate `sbom` package | Clean separation, testable, reusable |

## File Structure

```
buildpack/internal/
├── sbom/
│   ├── cyclonedx.go      # CycloneDX types and BOM generation
│   ├── python.go         # Parse METADATA from .dist-info/
│   └── writer.go         # Write <layer>.sbom.cdx.json files
├── build/
│   └── builder.go        # Integration points for SBOM
└── buildpack.toml        # Add sbom-formats field
```

## Components

### 1. CycloneDX Types (cyclonedx.go)

```go
type BOM struct {
    BOMFormat    string      `json:"bomFormat"`
    SpecVersion  string      `json:"specVersion"`
    Version      int         `json:"version"`
    Components   []Component `json:"components,omitempty"`
}

type Component struct {
    Type         string              `json:"type"`
    Name         string              `json:"name"`
    Version      string              `json:"version,omitempty"`
    PURL         string              `json:"purl,omitempty"`
    Licenses     []License           `json:"licenses,omitempty"`
    ExternalRefs []ExternalReference `json:"externalReferences,omitempty"`
}

type License struct {
    Expression string `json:"expression,omitempty"`
}

type ExternalReference struct {
    Type string `json:"type"`
    URL  string `json:"url"`
}
```

### 2. Python Package Parser (python.go)

```go
type PythonPackage struct {
    Name    string
    Version string
    License string
    PURL    string  // pkg:pypi/name@version
}

func ParseVenv(venvPath string) ([]PythonPackage, error)
func parseDistInfo(distInfoPath string) (PythonPackage, error)
```

**ParseVenv Algorithm:**
1. Find all `*.dist-info` directories in `lib/python*/site-packages/`
2. Read `METADATA` file in each directory
3. Extract: `Name`, `Version`, `License` (from headers)
4. Generate `purl`: `pkg:pypi/<normalized-name>@<version>`

### 3. SBOM Writer (writer.go)

```go
func WriteLayerSBOM(layersDir, layerName string, packages []PythonPackage) error
func WritePythonSBOM(layersDir string, pythonVersion string) error
```

**Output files:**
- `<layersDir>/python.sbom.cdx.json` — Python version
- `<layersDir>/venv.sbom.cdx.json` — All pip packages

### 4. Integration Points (builder.go)

After Python installation:
```go
if err := sbom.WritePythonSBOM(ctx.LayersDir, pythonVersion); err != nil {
    return result, fmt.Errorf("writing python SBOM: %w", err)
}
```

After venv creation and package installation:
```go
packages, err := sbom.ParseVenv(venvPath)
if err != nil {
    return result, fmt.Errorf("parsing venv for SBOM: %w", err)
}
if err := sbom.WriteLayerSBOM(ctx.LayersDir, layer.VenvLayerName, packages); err != nil {
    return result, fmt.Errorf("writing venv SBOM: %w", err)
}
```

### 5. buildpack.toml Update

```toml
[buildpack]
sbom-formats = ["application/vnd.cyclonedx+json"]
```

## METADATA File Format

Python packages store metadata in `<package>-<version>.dist-info/METADATA`:

```
Metadata-Version: 2.1
Name: numpy
Version: 1.26.0
License: BSD-3-Clause
Home-page: https://numpy.org
```

Fields to extract:
- `Name:` → Component.Name
- `Version:` → Component.Version
- `License:` or `License-Expression:` → Component.Licenses

## Package URL (purl) Format

```
pkg:pypi/<normalized-name>@<version>
```

Examples:
- `pkg:pypi/numpy@1.26.0`
- `pkg:pypi/scikit-learn@1.3.0`

Name normalization: lowercase, replace `_` with `-`

## Verification

After build:
```bash
pack inspect-image <image> --bom
pack sbom download <image> -o ./sbom-output
cat ./sbom-output/**/sbom.cdx.json | jq .
```

## Future Enhancements (Not in Scope)

- SPDX format support
- PyPI API fallback for missing licenses
- Model layer SBOM (MLflow model metadata)
- CPE (Common Platform Enumeration) identifiers

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

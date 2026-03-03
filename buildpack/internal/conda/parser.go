// Package conda provides utilities for parsing MLflow conda.yaml files.
package conda

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// File represents the structure of a conda.yaml file.
type File struct {
	Channels     []string     `yaml:"channels,omitempty"`
	Dependencies []Dependency `yaml:"dependencies,omitempty"`
}

// Dependency represents a single dependency entry.
// It can be a simple string (e.g., "python=3.10.13") or a pip block.
type Dependency struct {
	Name    string   // Parsed name (e.g., "python")
	Version string   // Parsed version (e.g., "3.10.13")
	Pip     []string // Pip dependencies if this is a pip block
	Raw     string   // Raw string value for simple dependencies
}

// UnmarshalYAML implements custom YAML unmarshaling for Dependency.
func (d *Dependency) UnmarshalYAML(node *yaml.Node) error {
	// Try to unmarshal as a string first
	var str string
	if err := node.Decode(&str); err == nil {
		d.Raw = str
		d.parseRaw()
		return nil
	}

	// Try to unmarshal as a map (for pip blocks)
	var pipBlock map[string][]string
	if err := node.Decode(&pipBlock); err == nil {
		if pip, ok := pipBlock["pip"]; ok {
			d.Pip = pip
			return nil
		}
	}

	return fmt.Errorf("cannot unmarshal dependency: unexpected type")
}

func (d *Dependency) parseRaw() {
	if d.Raw == "" {
		return
	}

	// Parse "name=version" or "name>=version" format
	parts := strings.SplitN(d.Raw, "=", 2)
	if len(parts) == 2 {
		d.Name = strings.TrimSpace(parts[0])
		d.Version = strings.TrimSpace(parts[1])
		return
	}

	parts = strings.SplitN(d.Raw, ">=", 2)
	if len(parts) == 2 {
		d.Name = strings.TrimSpace(parts[0])
		d.Version = ">=" + strings.TrimSpace(parts[1])
		return
	}

	parts = strings.SplitN(d.Raw, ">", 2)
	if len(parts) == 2 {
		d.Name = strings.TrimSpace(parts[0])
		d.Version = ">" + strings.TrimSpace(parts[1])
		return
	}

	parts = strings.SplitN(d.Raw, "<=", 2)
	if len(parts) == 2 {
		d.Name = strings.TrimSpace(parts[0])
		d.Version = "<=" + strings.TrimSpace(parts[1])
		return
	}

	parts = strings.SplitN(d.Raw, "<", 2)
	if len(parts) == 2 {
		d.Name = strings.TrimSpace(parts[0])
		d.Version = "<" + strings.TrimSpace(parts[1])
		return
	}

	// Just a name without version
	d.Name = strings.TrimSpace(d.Raw)
}

// IsPython checks if this dependency is Python.
func (d *Dependency) IsPython() bool {
	return d.Name == "python" || d.Name == "python3"
}

// IsPipBlock checks if this dependency is a pip block.
func (d *Dependency) IsPipBlock() bool {
	return len(d.Pip) > 0
}

// ParseFile parses a conda.yaml file from the given path.
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading conda file: %w", err)
	}

	return Parse(data)
}

// Parse parses conda.yaml content from a byte slice.
func Parse(data []byte) (*File, error) {
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing conda yaml: %w", err)
	}

	return &file, nil
}

// PythonVersion extracts the Python version from the conda file.
// Returns an empty string if Python is not specified.
func (f *File) PythonVersion() string {
	for _, dep := range f.Dependencies {
		if dep.IsPython() && dep.Version != "" {
			// Strip any version operators
			version := strings.TrimPrefix(dep.Version, ">=")
			version = strings.TrimPrefix(version, ">")
			version = strings.TrimPrefix(version, "<=")
			version = strings.TrimPrefix(version, "<")
			version = strings.TrimPrefix(version, "==")
			return version
		}
	}
	return ""
}

// PipDependencies extracts all pip dependencies from the conda file.
func (f *File) PipDependencies() []string {
	var deps []string
	for _, dep := range f.Dependencies {
		if dep.IsPipBlock() {
			deps = append(deps, dep.Pip...)
		}
	}
	return deps
}

// HasPipDependencies returns true if there are pip dependencies.
func (f *File) HasPipDependencies() bool {
	return len(f.PipDependencies()) > 0
}

// HasPython returns true if Python is specified in dependencies.
func (f *File) HasPython() bool {
	for _, dep := range f.Dependencies {
		if dep.IsPython() {
			return true
		}
	}
	return false
}

// String returns a string representation of the conda file.
func (f *File) String() string {
	return fmt.Sprintf("File{python: %s, pip_deps: %d}",
		f.PythonVersion(), len(f.PipDependencies()))
}

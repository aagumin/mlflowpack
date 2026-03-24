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
	defer func() { _ = file.Close() }()

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
			continue
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

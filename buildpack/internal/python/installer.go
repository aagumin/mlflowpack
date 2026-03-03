// Package python provides utilities for Python installation using uv.
package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// DefaultPythonVersion is used when no version is specified.
	DefaultPythonVersion = "3.10"
)

// Installer handles Python installation using uv.
type Installer struct {
	uvPath string
}

// NewInstaller creates a new Python installer.
func NewInstaller() *Installer {
	return &Installer{
		uvPath: "uv",
	}
}

// NewInstallerWithPath creates a new Python installer with a custom uv path.
func NewInstallerWithPath(uvPath string) *Installer {
	return &Installer{
		uvPath: uvPath,
	}
}

// InstallPython installs Python of the specified version.
// Uses: uv python install <version>
func (i *Installer) InstallPython(ctx context.Context, version string) error {
	if version == "" {
		version = DefaultPythonVersion
	}

	fmt.Printf("Installing Python %s\n", version)

	cmd := exec.CommandContext(ctx, i.uvPath,
		"python", "install",
		version,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing python %s: %w", version, err)
	}

	return nil
}

// CreateVenv creates a virtual environment using uv with specified Python version.
// If venv already exists, it skips creation.
// Uses: uv venv --python <version> <venvDir>
func (i *Installer) CreateVenv(ctx context.Context, version, venvDir string) error {
	// Check if venv already exists
	pythonBin := filepath.Join(venvDir, "bin", "python")
	if _, err := os.Stat(pythonBin); err == nil {
		fmt.Printf("Virtual environment already exists at %s, skipping creation\n", venvDir)
		return nil
	}

	fmt.Printf("Creating venv with Python %s at %s\n", version, venvDir)

	cmd := exec.CommandContext(ctx, i.uvPath,
		"venv",
		"--python", version,
		venvDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating venv: %w", err)
	}

	return nil
}

// InstallDeps installs pip dependencies using uv.
func (i *Installer) InstallDeps(ctx context.Context, venvDir string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	pythonBin := filepath.Join(venvDir, "bin", "python")

	args := []string{
		"pip", "install",
		"--python", pythonBin,
	}
	args = append(args, deps...)

	cmd := exec.CommandContext(ctx, i.uvPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing dependencies: %w", err)
	}

	return nil
}

// InstallDepsFromFile installs dependencies from a requirements.txt file.
func (i *Installer) InstallDepsFromFile(ctx context.Context, venvDir, requirementsFile string) error {
	pythonBin := filepath.Join(venvDir, "bin", "python")

	cmd := exec.CommandContext(ctx, i.uvPath,
		"pip", "install",
		"--python", pythonBin,
		"-r", requirementsFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing dependencies from file: %w", err)
	}

	return nil
}

// CondaFile represents a parsed conda.yaml file.
type CondaFile interface {
	PythonVersion() string
	HasPipDependencies() bool
	PipDependencies() []string
}

// SetupFromConda sets up Python and dependencies from a conda.yaml file.
func (i *Installer) SetupFromConda(ctx context.Context, condaFile CondaFile, pythonDir, venvDir string) error {
	// Get Python version
	pythonVersion := condaFile.PythonVersion()
	if pythonVersion == "" {
		pythonVersion = DefaultPythonVersion
	}

	// Install Python (uv manages it in ~/.local/share/uv/python)
	if err := i.InstallPython(ctx, pythonVersion); err != nil {
		return err
	}

	// Create virtual environment with the installed Python version
	if err := i.CreateVenv(ctx, pythonVersion, venvDir); err != nil {
		return err
	}

	// Install pip dependencies
	if condaFile.HasPipDependencies() {
		deps := condaFile.PipDependencies()
		if err := i.InstallDeps(ctx, venvDir, deps); err != nil {
			return err
		}
	}

	return nil
}

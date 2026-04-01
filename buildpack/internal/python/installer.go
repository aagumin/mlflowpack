// Package python provides utilities for Python installation using uv.
package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// DefaultPythonVersion is used when no version is specified.
	DefaultPythonVersion = "3.10"
)

// Installer handles Python installation using uv.
type Installer struct {
	uvPath string
	env    []string
}

// NewInstaller creates a new Python installer.
func NewInstaller() *Installer {
	return &Installer{uvPath: "uv"}
}

// NewInstallerWithPath creates a new Python installer with a custom uv path.
func NewInstallerWithPath(uvPath string) *Installer {
	return &Installer{uvPath: uvPath}
}

// NewInstallerWithEnv creates a new Python installer with custom command env overrides.
func NewInstallerWithEnv(env []string) *Installer {
	return &Installer{uvPath: "uv", env: env}
}

// NewInstallerWithPathAndEnv creates a new Python installer with a custom uv path and env overrides.
func NewInstallerWithPathAndEnv(uvPath string, env []string) *Installer {
	return &Installer{uvPath: uvPath, env: env}
}

func (i *Installer) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, i.uvPath, args...)
	cmd.Env = mergeEnv(os.Environ(), i.env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func mergeEnv(base, overrides []string) []string {
	if len(overrides) == 0 {
		return append([]string(nil), base...)
	}

	result := append([]string(nil), base...)
	index := make(map[string]int, len(base)+len(overrides))
	for i, item := range result {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = i
		}
	}

	for _, item := range overrides {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}

		if i, exists := index[key]; exists {
			result[i] = key + "=" + value
			continue
		}

		index[key] = len(result)
		result = append(result, key+"="+value)
	}

	return result
}

// InstallPython installs Python of the specified version to the given directory.
// Uses: uv python install --install-dir <dir> <version>
func (i *Installer) InstallPython(ctx context.Context, version, installDir string) error {
	if version == "" {
		version = DefaultPythonVersion
	}

	// Check if Python is already installed using uv python find
	// First update shell shims, then find the Python binary
	findCmd := i.command(ctx, "python", "find", version)
	findCmd.Stdout = nil // Capture output instead of printing
	findCmd.Stderr = nil
	output, err := findCmd.Output()
	if err == nil && len(output) > 0 {
		// Python found, verify it exists
		pythonPath := strings.TrimSpace(string(output))
		if _, statErr := os.Stat(pythonPath); statErr == nil {
			fmt.Printf("Python %s already installed at %s, skipping installation\n", version, pythonPath)
			return nil
		}
	}

	fmt.Printf("Installing Python %s\n", version)

	cmd := i.command(ctx,
		"python", "install",
		"--install-dir", installDir,
		version,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing python %s: %w", version, err)
	}

	return nil
}

// extractPythonVersion extracts the Python version from the binary path.
// The path format is: <dir>/cpython-<version>-<platform>/bin/python3.x
// It resolves symlinks to get the actual version (e.g., cpython-3.10 -> cpython-3.10.18).
func extractPythonVersion(pythonBin string) string {
	// Walk up to find the cpython directory
	dir := filepath.Dir(filepath.Dir(pythonBin))

	// Resolve symlink if it's one (cpython-3.10 -> cpython-3.10.18)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolvedDir
	}

	dirName := filepath.Base(dir)

	// Parse cpython-3.10.18-linux-aarch64-gnu format
	if !strings.HasPrefix(dirName, "cpython-") {
		return ""
	}

	// Extract version: cpython-3.10.18-... -> 3.10.18
	parts := strings.Split(strings.TrimPrefix(dirName, "cpython-"), "-")
	if len(parts) < 2 {
		return ""
	}

	return parts[0]
}

// CreateVenv creates a virtual environment using uv with Python from installDir.
// If venv already exists, it skips creation.
// Uses: uv venv --python <pythonPath> <venvDir>
func (i *Installer) CreateVenv(ctx context.Context, pythonPath, venvDir string) error {
	// Check if venv already exists
	venvPythonBin := filepath.Join(venvDir, "bin", "python")
	if _, err := os.Stat(venvPythonBin); err == nil {
		fmt.Printf("Virtual environment already exists at %s, skipping creation\n", venvDir)
		return nil
	}

	fmt.Printf("Creating venv with Python at %s\n", venvDir)

	cmd := i.command(ctx,
		"venv",
		"--python", pythonPath,
		venvDir,
	)

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

	cmd := i.command(ctx, args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing dependencies: %w", err)
	}

	return nil
}

// InstallDepsFromFile installs dependencies from a requirements.txt file.
func (i *Installer) InstallDepsFromFile(ctx context.Context, venvDir, requirementsFile string) error {
	pythonBin := filepath.Join(venvDir, "bin", "python")

	cmd := i.command(ctx,
		"pip", "install",
		"--python", pythonBin,
		"-r", requirementsFile,
	)

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

	// Install Python to the python layer directory
	if err := i.InstallPython(ctx, pythonVersion, pythonDir); err != nil {
		return err
	}

	// Find the installed Python binary
	// uv installs to <pythonDir>/cpython-<version>-<platform>/bin/python3
	pythonBin, err := findPythonBinary(pythonDir)
	if err != nil {
		return fmt.Errorf("finding python binary: %w", err)
	}

	// Create virtual environment with the installed Python
	if err := i.CreateVenv(ctx, pythonBin, venvDir); err != nil {
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

// findPythonBinary finds the Python binary in a uv-managed installation directory.
func findPythonBinary(pythonDir string) (string, error) {
	// Look for cpython-*/bin/python3 pattern
	entries, err := os.ReadDir(pythonDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		name := entry.Name()
		// Check if it's a cpython directory or symlink to one
		if len(name) >= 7 && name[:7] == "cpython" {
			fullPath := filepath.Join(pythonDir, name)

			// Check if it's a symlink and resolve it
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			if !info.IsDir() {
				continue
			}

			binPath := filepath.Join(fullPath, "bin")

			// Look for python3 or python3.x
			if files, err := os.ReadDir(binPath); err == nil {
				for _, f := range files {
					if f.Name() == "python3" || (len(f.Name()) > 8 && f.Name()[:8] == "python3.") {
						return filepath.Join(binPath, f.Name()), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("python binary not found in %s", pythonDir)
}

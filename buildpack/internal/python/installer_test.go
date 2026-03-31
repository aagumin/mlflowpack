package python

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstaller_UsesParentEnvWhenNoOverrides(t *testing.T) {
	uvPath, envDump := writeFakeUv(t)

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "parent-tmp"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "parent-home"))
	t.Setenv("UV_CACHE_DIR", filepath.Join(t.TempDir(), "parent-uv-cache"))
	t.Setenv("PIP_CACHE_DIR", filepath.Join(t.TempDir(), "parent-pip-cache"))
	t.Setenv("UV_PYTHON_INSTALL_DIR", filepath.Join(t.TempDir(), "parent-python"))
	t.Setenv("UV_ENV_DUMP", envDump)

	installer := NewInstallerWithPath(uvPath)
	if err := installer.InstallPython(context.Background(), "3.11", t.TempDir()); err != nil {
		t.Fatalf("InstallPython() error = %v", err)
	}

	got := readEnvDump(t, envDump)
	wantKeys := []string{"TMPDIR", "HOME", "UV_CACHE_DIR", "PIP_CACHE_DIR", "UV_PYTHON_INSTALL_DIR"}
	for _, key := range wantKeys {
		if got[key] == "" {
			t.Fatalf("env %s missing from fake uv environment: %#v", key, got)
		}
	}

	if got["TMPDIR"] != os.Getenv("TMPDIR") {
		t.Fatalf("TMPDIR = %q, want %q", got["TMPDIR"], os.Getenv("TMPDIR"))
	}
	if got["HOME"] != os.Getenv("HOME") {
		t.Fatalf("HOME = %q, want %q", got["HOME"], os.Getenv("HOME"))
	}
	if got["UV_CACHE_DIR"] != os.Getenv("UV_CACHE_DIR") {
		t.Fatalf("UV_CACHE_DIR = %q, want %q", got["UV_CACHE_DIR"], os.Getenv("UV_CACHE_DIR"))
	}
	if got["PIP_CACHE_DIR"] != os.Getenv("PIP_CACHE_DIR") {
		t.Fatalf("PIP_CACHE_DIR = %q, want %q", got["PIP_CACHE_DIR"], os.Getenv("PIP_CACHE_DIR"))
	}
	if got["UV_PYTHON_INSTALL_DIR"] != os.Getenv("UV_PYTHON_INSTALL_DIR") {
		t.Fatalf("UV_PYTHON_INSTALL_DIR = %q, want %q", got["UV_PYTHON_INSTALL_DIR"], os.Getenv("UV_PYTHON_INSTALL_DIR"))
	}
}

func TestInstaller_AppliesExplicitEnvOverrides(t *testing.T) {
	uvPath, envDump := writeFakeUv(t)

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "parent-tmp"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "parent-home"))
	t.Setenv("UV_CACHE_DIR", filepath.Join(t.TempDir(), "parent-uv-cache"))
	t.Setenv("PIP_CACHE_DIR", filepath.Join(t.TempDir(), "parent-pip-cache"))
	t.Setenv("UV_PYTHON_INSTALL_DIR", filepath.Join(t.TempDir(), "parent-python"))
	t.Setenv("UV_ENV_DUMP", envDump)

	workRoot := filepath.Join(t.TempDir(), "work-root")
	installer := NewInstallerWithPathAndEnv(uvPath, []string{
		"TMPDIR=" + filepath.Join(workRoot, "tmp"),
		"TMP=" + filepath.Join(workRoot, "tmp"),
		"TEMP=" + filepath.Join(workRoot, "tmp"),
		"HOME=" + filepath.Join(workRoot, "home"),
		"XDG_CACHE_HOME=" + filepath.Join(workRoot, "home", ".cache"),
		"UV_CACHE_DIR=" + filepath.Join(workRoot, "cache", "uv"),
		"PIP_CACHE_DIR=" + filepath.Join(workRoot, "cache", "pip"),
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(workRoot, "python"),
	})

	if err := installer.InstallPython(context.Background(), "3.11", t.TempDir()); err != nil {
		t.Fatalf("InstallPython() error = %v", err)
	}

	got := readEnvDump(t, envDump)
	want := map[string]string{
		"TMPDIR":                filepath.Join(workRoot, "tmp"),
		"TMP":                   filepath.Join(workRoot, "tmp"),
		"TEMP":                  filepath.Join(workRoot, "tmp"),
		"HOME":                  filepath.Join(workRoot, "home"),
		"XDG_CACHE_HOME":        filepath.Join(workRoot, "home", ".cache"),
		"UV_CACHE_DIR":          filepath.Join(workRoot, "cache", "uv"),
		"PIP_CACHE_DIR":         filepath.Join(workRoot, "cache", "pip"),
		"UV_PYTHON_INSTALL_DIR": filepath.Join(workRoot, "python"),
	}

	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Fatalf("%s = %q, want %q", key, got[key], wantValue)
		}
	}
}

func writeFakeUv(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	envDump := filepath.Join(dir, "uv-env.txt")
	script := filepath.Join(dir, "uv")

	content := "#!/bin/sh\n" +
		"env > \"$UV_ENV_DUMP\"\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake uv: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(script, 0o755); err != nil {
			t.Fatalf("chmod fake uv: %v", err)
		}
	}

	return script, envDump
}

func readEnvDump(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env dump: %v", err)
	}

	env := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid env line %q", line)
		}
		env[key] = value
	}

	return env
}

func TestExtractPythonVersion(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "standard format",
			path:     "/layers/python/cpython-3.10.18-linux-aarch64-gnu/bin/python3.10",
			expected: "3.10.18",
		},
		{
			name:     "x86_64 format",
			path:     "/layers/python/cpython-3.11.4-linux-x86_64-gnu/bin/python3",
			expected: "3.11.4",
		},
		{
			name:     "macOS format",
			path:     "/layers/python/cpython-3.12.0-macos-aarch64/bin/python3.12",
			expected: "3.12.0",
		},
		{
			name:     "short version",
			path:     "/layers/python/cpython-3.10-linux-aarch64-gnu/bin/python3",
			expected: "3.10",
		},
		{
			name:     "invalid path - no cpython prefix",
			path:     "/layers/python/python-3.10/bin/python3",
			expected: "",
		},
		{
			name:     "invalid path - too few parts",
			path:     "/layers/python/cpython/bin/python3",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPythonVersion(tt.path)
			if got != tt.expected {
				t.Errorf("extractPythonVersion(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestInstallPython_SkipsExistingInstallation(t *testing.T) {
	// Create a fake Python installation directory
	pythonDir := t.TempDir()
	cpythonDir := filepath.Join(pythonDir, "cpython-3.10.18-linux-aarch64-gnu")
	binDir := filepath.Join(cpythonDir, "bin")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create fake python binary
	pythonBin := filepath.Join(binDir, "python3.10")
	if err := os.WriteFile(pythonBin, []byte("#!/bin/sh\necho 'Python 3.10.18'"), 0o755); err != nil {
		t.Fatalf("write python bin: %v", err)
	}

	// Create a fake uv that would fail if called
	uvPath := filepath.Join(t.TempDir(), "uv-fail")
	uvScript := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(uvPath, []byte(uvScript), 0o755); err != nil {
		t.Fatalf("write fake uv: %v", err)
	}

	installer := NewInstallerWithPath(uvPath)

	// This should NOT call uv because Python already exists
	err := installer.InstallPython(context.Background(), "3.10.18", pythonDir)
	if err != nil {
		t.Errorf("InstallPython() error = %v, want nil (should skip installation)", err)
	}
}

func TestInstallPython_InstallsWhenVersionMismatch(t *testing.T) {
	// Create a fake Python installation with different version
	pythonDir := t.TempDir()
	cpythonDir := filepath.Join(pythonDir, "cpython-3.10.18-linux-aarch64-gnu")
	binDir := filepath.Join(cpythonDir, "bin")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create fake python binary
	pythonBin := filepath.Join(binDir, "python3.10")
	if err := os.WriteFile(pythonBin, []byte("#!/bin/sh\necho 'Python 3.10.18'"), 0o755); err != nil {
		t.Fatalf("write python bin: %v", err)
	}

	// Create a fake uv that succeeds
	uvPath := filepath.Join(t.TempDir(), "uv-ok")
	uvScript := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(uvPath, []byte(uvScript), 0o755); err != nil {
		t.Fatalf("write fake uv: %v", err)
	}

	installer := NewInstallerWithPath(uvPath)

	// This SHOULD call uv because version mismatch (3.11 vs 3.10.18)
	err := installer.InstallPython(context.Background(), "3.11", pythonDir)
	if err != nil {
		t.Errorf("InstallPython() error = %v, want nil", err)
	}
}

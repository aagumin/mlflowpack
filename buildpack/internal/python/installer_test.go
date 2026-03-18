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
	t.Setenv("UV_ENV_DUMP", envDump)

	installer := NewInstallerWithPath(uvPath)
	if err := installer.InstallPython(context.Background(), "3.11", t.TempDir()); err != nil {
		t.Fatalf("InstallPython() error = %v", err)
	}

	got := readEnvDump(t, envDump)
	wantKeys := []string{"TMPDIR", "HOME", "UV_CACHE_DIR", "PIP_CACHE_DIR"}
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
}

func TestInstaller_AppliesExplicitEnvOverrides(t *testing.T) {
	uvPath, envDump := writeFakeUv(t)

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "parent-tmp"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "parent-home"))
	t.Setenv("UV_CACHE_DIR", filepath.Join(t.TempDir(), "parent-uv-cache"))
	t.Setenv("PIP_CACHE_DIR", filepath.Join(t.TempDir(), "parent-pip-cache"))
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
	})

	if err := installer.InstallPython(context.Background(), "3.11", t.TempDir()); err != nil {
		t.Fatalf("InstallPython() error = %v", err)
	}

	got := readEnvDump(t, envDump)
	want := map[string]string{
		"TMPDIR":         filepath.Join(workRoot, "tmp"),
		"TMP":            filepath.Join(workRoot, "tmp"),
		"TEMP":           filepath.Join(workRoot, "tmp"),
		"HOME":           filepath.Join(workRoot, "home"),
		"XDG_CACHE_HOME": filepath.Join(workRoot, "home", ".cache"),
		"UV_CACHE_DIR":   filepath.Join(workRoot, "cache", "uv"),
		"PIP_CACHE_DIR":  filepath.Join(workRoot, "cache", "pip"),
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

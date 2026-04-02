package mlflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestInstallerEnv_UsesWritableRootDefaults(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	pythonDir := filepath.Join(t.TempDir(), "python")
	setEnv(t, "BP_MLFLOW_WORK_DIR", workRoot)
	unsetEnv(t, "TMPDIR")
	unsetEnv(t, "TMP")
	unsetEnv(t, "TEMP")
	unsetEnv(t, "HOME")
	unsetEnv(t, "XDG_CACHE_HOME")
	unsetEnv(t, "UV_CACHE_DIR")
	unsetEnv(t, "PIP_CACHE_DIR")
	unsetEnv(t, "UV_PYTHON_INSTALL_DIR")

	got, err := installerEnv(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")}, pythonDir)
	if err != nil {
		t.Fatalf("installerEnv() error = %v", err)
	}

	want := map[string]string{
		"TMPDIR":                filepath.Join(workRoot, "tmp"),
		"TMP":                   filepath.Join(workRoot, "tmp"),
		"TEMP":                  filepath.Join(workRoot, "tmp"),
		"HOME":                  filepath.Join(workRoot, "home"),
		"XDG_CACHE_HOME":        filepath.Join(workRoot, "home", ".cache"),
		"UV_CACHE_DIR":          filepath.Join(workRoot, "cache", "uv"),
		"PIP_CACHE_DIR":         filepath.Join(workRoot, "cache", "pip"),
		"UV_PYTHON_INSTALL_DIR": pythonDir,
	}

	for key, wantValue := range want {
		if gotValue := lookupEnv(got, key); gotValue != wantValue {
			t.Fatalf("%s = %q, want %q", key, gotValue, wantValue)
		}
	}
}

func TestInstallerEnv_PreservesExplicitParentEnv(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	pythonDir := filepath.Join(t.TempDir(), "python")
	setEnv(t, "BP_MLFLOW_WORK_DIR", workRoot)

	values := map[string]string{
		"TMPDIR":                filepath.Join(t.TempDir(), "parent-tmp"),
		"TMP":                   filepath.Join(t.TempDir(), "parent-tmp"),
		"TEMP":                  filepath.Join(t.TempDir(), "parent-tmp"),
		"HOME":                  filepath.Join(t.TempDir(), "parent-home"),
		"XDG_CACHE_HOME":        filepath.Join(t.TempDir(), "parent-home", ".cache"),
		"UV_CACHE_DIR":          filepath.Join(t.TempDir(), "parent-uv-cache"),
		"PIP_CACHE_DIR":         filepath.Join(t.TempDir(), "parent-pip-cache"),
		"UV_PYTHON_INSTALL_DIR": filepath.Join(t.TempDir(), "parent-python"),
	}
	for key, value := range values {
		setEnv(t, key, value)
	}

	got, err := installerEnv(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")}, pythonDir)
	if err != nil {
		t.Fatalf("installerEnv() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("installerEnv() = %v, want no overrides when parent env is explicit", got)
	}

	for key, wantValue := range values {
		if gotValue := os.Getenv(key); gotValue != wantValue {
			t.Fatalf("%s parent env = %q, want %q", key, gotValue, wantValue)
		}
	}
}

func TestInstallerEnv_UsesWritableRootForEmptyParentEnv(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	pythonDir := filepath.Join(t.TempDir(), "python")
	setEnv(t, "BP_MLFLOW_WORK_DIR", workRoot)

	values := []string{"TMPDIR", "TMP", "TEMP", "HOME", "XDG_CACHE_HOME", "UV_CACHE_DIR", "PIP_CACHE_DIR", "UV_PYTHON_INSTALL_DIR"}
	for _, key := range values {
		setEnv(t, key, "")
	}

	got, err := installerEnv(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")}, pythonDir)
	if err != nil {
		t.Fatalf("installerEnv() error = %v", err)
	}

	want := map[string]string{
		"TMPDIR":                filepath.Join(workRoot, "tmp"),
		"TMP":                   filepath.Join(workRoot, "tmp"),
		"TEMP":                  filepath.Join(workRoot, "tmp"),
		"HOME":                  filepath.Join(workRoot, "home"),
		"XDG_CACHE_HOME":        filepath.Join(workRoot, "home", ".cache"),
		"UV_CACHE_DIR":          filepath.Join(workRoot, "cache", "uv"),
		"PIP_CACHE_DIR":         filepath.Join(workRoot, "cache", "pip"),
		"UV_PYTHON_INSTALL_DIR": pythonDir,
	}

	for key, wantValue := range want {
		if gotValue := lookupEnv(got, key); gotValue != wantValue {
			t.Fatalf("%s = %q, want %q", key, gotValue, wantValue)
		}
	}
}

func lookupEnv(items []string, key string) string {
	prefix := key + "="
	for _, item := range items {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			return item[len(prefix):]
		}
	}
	return ""
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	if value, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() {
			t.Setenv(key, value)
		})
	}

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

func setEnv(t *testing.T, key, value string) {
	t.Helper()

	if oldValue, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() {
			if err := os.Setenv(key, oldValue); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
		})
	} else {
		t.Cleanup(func() {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
		})
	}

	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

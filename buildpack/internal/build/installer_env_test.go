package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestInstallerEnv_UsesWritableRootDefaults(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	setEnv(t, "BP_MLFLOW_WORK_DIR", workRoot)
	unsetEnv(t, "TMPDIR")
	unsetEnv(t, "TMP")
	unsetEnv(t, "TEMP")
	unsetEnv(t, "HOME")
	unsetEnv(t, "XDG_CACHE_HOME")
	unsetEnv(t, "UV_CACHE_DIR")
	unsetEnv(t, "PIP_CACHE_DIR")

	got, err := installerEnv(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")})
	if err != nil {
		t.Fatalf("installerEnv() error = %v", err)
	}

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
		if gotValue := lookupEnv(got, key); gotValue != wantValue {
			t.Fatalf("%s = %q, want %q", key, gotValue, wantValue)
		}
	}
}

func TestInstallerEnv_PreservesExplicitParentEnv(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	setEnv(t, "BP_MLFLOW_WORK_DIR", workRoot)

	values := map[string]string{
		"TMPDIR":         filepath.Join(t.TempDir(), "parent-tmp"),
		"TMP":            filepath.Join(t.TempDir(), "parent-tmp"),
		"TEMP":           filepath.Join(t.TempDir(), "parent-tmp"),
		"HOME":           filepath.Join(t.TempDir(), "parent-home"),
		"UV_CACHE_DIR":   filepath.Join(t.TempDir(), "parent-uv-cache"),
		"PIP_CACHE_DIR":  filepath.Join(t.TempDir(), "parent-pip-cache"),
	}
	for key, value := range values {
		setEnv(t, key, value)
	}
	unsetEnv(t, "XDG_CACHE_HOME")

	got, err := installerEnv(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")})
	if err != nil {
		t.Fatalf("installerEnv() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("installerEnv() = %v, want exactly one override", got)
	}

	wantXDG := filepath.Join(values["HOME"], ".cache")
	if gotValue := lookupEnv(got, "XDG_CACHE_HOME"); gotValue != wantXDG {
		t.Fatalf("XDG_CACHE_HOME = %q, want %q", gotValue, wantXDG)
	}

	for key, wantValue := range values {
		if gotValue := lookupEnv(got, key); gotValue != "" {
			t.Fatalf("%s = %q, want empty override (parent env should win with %q)", key, gotValue, wantValue)
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

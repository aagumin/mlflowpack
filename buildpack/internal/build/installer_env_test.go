package build

import (
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestInstallerEnv_UsesWritableRootDefaults(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	t.Setenv("BP_MLFLOW_WORK_DIR", workRoot)

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

func lookupEnv(items []string, key string) string {
	prefix := key + "="
	for _, item := range items {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			return item[len(prefix):]
		}
	}
	return ""
}

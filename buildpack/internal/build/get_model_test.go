package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

type fakeModelDownloader struct {
	destDir string
}

func (f *fakeModelDownloader) DownloadModel(ctx context.Context, name, version, destDir string) (string, error) {
	f.destDir = destDir
	return filepath.Join(destDir, "downloaded", "MLmodel"), nil
}

func TestGetModel_UsesWorkDirForRegistryDownloads(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	t.Setenv("BP_MLFLOW_WORK_DIR", workRoot)

	originalFactory := newModelDownloader
	t.Cleanup(func() {
		newModelDownloader = originalFactory
	})

	fake := &fakeModelDownloader{}
	newModelDownloader = func() (modelDownloader, error) {
		return fake, nil
	}

	model, err := getModel(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")}, &modelSource{
		Type:    "registry",
		Name:    "iris-model",
		Version: "42",
	})
	if err != nil {
		t.Fatalf("getModel() error = %v", err)
	}

	wantPrefix := filepath.Join(workRoot, "tmp") + string(os.PathSeparator)
	if !strings.HasPrefix(fake.destDir+string(os.PathSeparator), wantPrefix) {
		t.Fatalf("download destDir = %q, want prefix %q", fake.destDir, wantPrefix)
	}

	wantModelPath := filepath.Join(fake.destDir, "downloaded", "MLmodel")
	if model.Path != wantModelPath {
		t.Fatalf("model.Path = %q, want %q", model.Path, wantModelPath)
	}

	if got := model.ModelName; got != "iris-model" {
		t.Fatalf("model.ModelName = %q, want %q", got, "iris-model")
	}

	if got := model.ModelVersion; got != "42" {
		t.Fatalf("model.ModelVersion = %q, want %q", got, "42")
	}
}

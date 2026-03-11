package detect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrLocalModelNotFound means no local MLmodel file exists in app dir.
	ErrLocalModelNotFound = errors.New("local MLmodel file not found")
)

// FindLocalModelDir resolves a local MLflow model directory.
// Resolution order:
// 1. BP_MLFLOW_MODEL_PATH (absolute or relative to app dir)
// 2. MLmodel at app dir root
// 3. Recursive search under app dir for a single MLmodel file
func FindLocalModelDir(appDir string) (string, error) {
	if appDir == "" {
		return "", fmt.Errorf("app directory is empty")
	}
	if _, _, ok, err := DetectFromModelPathEnv(); err != nil {
		return "", err
	} else if ok {
		// models:/... explicitly switches to registry mode, so local source is absent.
		return "", ErrLocalModelNotFound
	}

	if dir, ok, err := modelDirFromEnv(appDir); ok || err != nil {
		return dir, err
	}

	rootMLmodel := filepath.Join(appDir, MLmodelFile)
	if info, err := os.Stat(rootMLmodel); err == nil && !info.IsDir() {
		return appDir, nil
	}

	matches, err := findMLmodelFiles(appDir)
	if err != nil {
		return "", fmt.Errorf("searching for %s: %w", MLmodelFile, err)
	}

	switch len(matches) {
	case 0:
		return "", ErrLocalModelNotFound
	case 1:
		return filepath.Dir(matches[0]), nil
	default:
		return "", fmt.Errorf(
			"multiple %s files found (%d): %s; set %s to select model directory",
			MLmodelFile,
			len(matches),
			formatPaths(appDir, matches),
			EnvModelPath,
		)
	}
}

func modelDirFromEnv(appDir string) (string, bool, error) {
	raw := strings.TrimSpace(os.Getenv(EnvModelPath))
	if raw == "" {
		return "", false, nil
	}
	if strings.HasPrefix(raw, ModelRegistryPrefix) {
		return "", false, nil
	}

	modelDir := raw
	if !filepath.IsAbs(modelDir) {
		modelDir = filepath.Join(appDir, modelDir)
	}
	modelDir = filepath.Clean(modelDir)

	mlmodelPath := filepath.Join(modelDir, MLmodelFile)
	info, err := os.Stat(mlmodelPath)
	if err == nil && !info.IsDir() {
		return modelDir, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", true, fmt.Errorf("%s=%q points to %q but %s was not found", EnvModelPath, raw, modelDir, MLmodelFile)
	}
	if err != nil {
		return "", true, fmt.Errorf("reading %s from %q: %w", MLmodelFile, modelDir, err)
	}

	return "", true, fmt.Errorf("%s=%q points to %q but %s is a directory", EnvModelPath, raw, modelDir, MLmodelFile)
}

func findMLmodelFiles(appDir string) ([]string, error) {
	matches := make([]string, 0, 1)

	err := filepath.WalkDir(appDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == MLmodelFile {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return matches, nil
}

func formatPaths(appDir string, paths []string) string {
	formatted := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(appDir, path)
		if err != nil {
			formatted = append(formatted, path)
			continue
		}
		formatted = append(formatted, rel)
	}

	return strings.Join(formatted, ", ")
}

// Package build implements the CNB build phase for MLflow models.
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/conda"
	"github.com/aagumin/mlflowpack/internal/detect"
	"github.com/aagumin/mlflowpack/internal/layer"
	"github.com/aagumin/mlflowpack/internal/mlflow"
	"github.com/aagumin/mlflowpack/internal/python"
	"github.com/aagumin/mlflowpack/internal/sbom"
)

const (
	// EnvModelName is the environment variable for model name.
	EnvModelName = "BP_MLFLOW_MODEL_NAME"

	// EnvModelVersion is the environment variable for model version.
	EnvModelVersion = "BP_MLFLOW_MODEL_VERSION"

	// EnvModelStage is the environment variable for model stage.
	EnvModelStage = "BP_MLFLOW_MODEL_STAGE"
)

// cachedModelUUID returns the model UUID from cached model layer metadata.
func cachedModelUUID(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.ModelLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	metadata, ok := meta.Metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	uuid, _ := metadata["model_uuid"].(string)
	return uuid
}

// Build executes the build phase.
func Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	result := cnb.BuildResult{
		Layers: make(map[string]cnb.LayerMetadata),
	}

	// Determine model source
	modelSource, err := determineModelSource(ctx)
	if err != nil {
		return result, err
	}

	// Get model
	model, err := getModel(ctx, modelSource)
	if err != nil {
		return result, err
	}
	if modelSource.Type == "registry" && model.Path != "" {
		defer func() {
			if cleanupErr := os.RemoveAll(model.Path); cleanupErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove temporary model directory %q: %v\n", model.Path, cleanupErr)
			}
		}()
	}

	// Parse MLmodel file to detect flavor
	if err := model.ParseMLmodel(); err != nil {
		return result, fmt.Errorf("parsing MLmodel: %w", err)
	}

	// Check for cached layers with same model UUID
	cachedUUID := cachedModelUUID(ctx.LayersDir)
	currentUUID := model.UUID()

	if cachedUUID != "" && cachedUUID == currentUUID {
		fmt.Printf("Model unchanged (UUID: %s), reusing cached layers\n", currentUUID)

		// Return cached layer metadata
		result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}
		result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{Types: layer.DefaultVenvLayerTypes()}
		result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{Types: layer.DefaultModelLayerTypes()}

		// Add process for cached venv
		result.Launch.Processes = append(result.Launch.Processes, cnb.ProcessEntry{
			Type:    "web",
			Command: []string{filepath.Join(ctx.LayersDir, layer.VenvLayerName, "bin", "mlserver"), "start", filepath.Join(ctx.LayersDir, layer.ModelLayerName)},
			Default: true,
		})

		return result, nil
	}

	// Get MLServer extension based on model flavor
	mlserverExt, err := model.GetMLServerExtension()
	if err != nil {
		return result, fmt.Errorf("detecting MLServer extension: %w", err)
	}

	flavor := model.GetPrimaryFlavor()
	fmt.Printf("Detected model flavor: %s\n", flavor)
	fmt.Printf("Required MLServer extension: %s\n", mlserverExt.PipPackage)

	dependencies, err := resolveDependencies(model, mlserverExt.PipPackage)
	if err != nil {
		return result, err
	}

	// Create Python layer
	pythonPath, err := layer.SetupLayer(ctx.LayersDir, layer.PythonLayerName, layer.DefaultPythonLayerTypes())
	if err != nil {
		return result, fmt.Errorf("creating python layer: %w", err)
	}
	result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}

	// Create venv layer
	venvPath, err := layer.SetupLayer(ctx.LayersDir, layer.VenvLayerName, layer.DefaultVenvLayerTypes())
	if err != nil {
		return result, fmt.Errorf("creating venv layer: %w", err)
	}
	result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{Types: layer.DefaultVenvLayerTypes()}

	// Create model layer
	modelPath, err := layer.SetupLayer(ctx.LayersDir, layer.ModelLayerName, layer.DefaultModelLayerTypes())
	if err != nil {
		return result, fmt.Errorf("creating model layer: %w", err)
	}

	// Install Python and dependencies using uv
	uvEnv, err := installerEnv(ctx, pythonPath)
	if err != nil {
		return result, fmt.Errorf("configuring uv environment: %w", err)
	}
	installer := python.NewInstallerWithPathAndEnv("uv", uvEnv)
	if err := installer.SetupFromConda(context.Background(), dependencies.conda, pythonPath, venvPath); err != nil {
		return result, fmt.Errorf("setting up Python: %w", err)
	}

	// Write Python SBOM
	pythonVersion := dependencies.conda.PythonVersion()
	if pythonVersion == "" {
		pythonVersion = python.DefaultPythonVersion
	}

	// Set model layer metadata with UUID
	result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{
		Types: layer.DefaultModelLayerTypes(),
		Metadata: map[string]interface{}{
			"model_uuid":     model.UUID(),
			"flavor":         flavor,
			"python_version": pythonVersion,
		},
	}
	if err := sbom.WritePythonSBOM(ctx.LayersDir, pythonVersion); err != nil {
		return result, fmt.Errorf("writing python SBOM: %w", err)
	}

	if dependencies.requirementsPath != "" {
		fmt.Printf("conda.yaml not found; installing dependencies from %s\n", filepath.Base(dependencies.requirementsPath))
		if err := installer.InstallDepsFromFile(context.Background(), venvPath, dependencies.requirementsPath); err != nil {
			return result, fmt.Errorf("installing dependencies from requirements.txt: %w", err)
		}
	}

	// Write venv SBOM
	packages, err := sbom.ParseVenv(venvPath)
	if err != nil {
		return result, fmt.Errorf("parsing venv for SBOM: %w", err)
	}
	if err := sbom.WriteLayerSBOM(ctx.LayersDir, layer.VenvLayerName, packages); err != nil {
		return result, fmt.Errorf("writing venv SBOM: %w", err)
	}

	// Configure PATH for build and launch
	if err := cnb.PrependToPath(pythonPath, filepath.Join(pythonPath, "bin")); err != nil {
		return result, fmt.Errorf("configuring PATH: %w", err)
	}
	if err := cnb.PrependToPath(venvPath, filepath.Join(venvPath, "bin")); err != nil {
		return result, fmt.Errorf("configuring PATH: %w", err)
	}

	// Copy model to layer
	if err := copyModel(model, modelPath); err != nil {
		return result, fmt.Errorf("copying model: %w", err)
	}

	// Create model-settings.json for mlserver
	// This is the recommended way to configure mlserver
	modelSettings := map[string]interface{}{
		"name":           modelSource.Name,
		"implementation": mlserverExt.Runtime,
		"parameters": map[string]interface{}{
			"uri": modelPath,
		},
	}
	if err := writeModelSettings(modelPath, modelSettings); err != nil {
		return result, fmt.Errorf("writing model-settings.json: %w", err)
	}

	// Add default process to start MLServer from venv
	// Use the venv's mlserver to ensure Python version compatibility
	result.Launch.Processes = append(result.Launch.Processes, cnb.ProcessEntry{
		Type:    "web",
		Command: []string{filepath.Join(venvPath, "bin", "mlserver"), "start", modelPath},
		Default: true,
	})

	return result, nil
}

func installerEnv(ctx cnb.BuildContext, pythonDir string) ([]string, error) {
	workRoot, err := WorkDir(ctx)
	if err != nil {
		return nil, err
	}

	tmpDir := filepath.Join(workRoot, "tmp")
	homeDir := filepath.Join(workRoot, "home")
	uvCacheDir := filepath.Join(workRoot, "cache", "uv")
	pipCacheDir := filepath.Join(workRoot, "cache", "pip")

	for _, dir := range []string{
		tmpDir,
		homeDir,
		filepath.Join(homeDir, ".cache"),
		uvCacheDir,
		pipCacheDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating installer env dir %q: %w", dir, err)
		}
	}

	env := make([]string, 0, 7)

	if _, ok := envValue("TMPDIR"); !ok {
		env = append(env, "TMPDIR="+tmpDir)
	}
	if _, ok := envValue("TMP"); !ok {
		env = append(env, "TMP="+tmpDir)
	}
	if _, ok := envValue("TEMP"); !ok {
		env = append(env, "TEMP="+tmpDir)
	}

	homeValue, ok := envValue("HOME")
	if !ok {
		homeValue = homeDir
		env = append(env, "HOME="+homeDir)
	}
	if _, ok := envValue("XDG_CACHE_HOME"); !ok {
		env = append(env, "XDG_CACHE_HOME="+filepath.Join(homeValue, ".cache"))
	}
	if _, ok := envValue("UV_CACHE_DIR"); !ok {
		env = append(env, "UV_CACHE_DIR="+uvCacheDir)
	}
	if _, ok := envValue("PIP_CACHE_DIR"); !ok {
		env = append(env, "PIP_CACHE_DIR="+pipCacheDir)
	}
	if _, ok := envValue("UV_PYTHON_INSTALL_DIR"); !ok {
		env = append(env, "UV_PYTHON_INSTALL_DIR="+pythonDir)
	}

	return env, nil
}

func envValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

// modelSource represents the source of the model.
type modelSource struct {
	Type    string // "local" or "registry"
	Path    string // Local path (for local models)
	Name    string // Model name (for registry models)
	Version string // Model version or stage (for registry models)
}

type dependencySource struct {
	conda            *conda.File
	requirementsPath string
}

type modelDownloader interface {
	DownloadModel(ctx context.Context, name, version, destDir string) (string, error)
}

var newModelDownloader = func() (modelDownloader, error) {
	return mlflow.NewDownloader()
}

func determineModelSource(ctx cnb.BuildContext) (*modelSource, error) {
	// models:/... in BP_MLFLOW_MODEL_PATH explicitly selects registry source.
	if name, version, ok, err := detect.DetectFromModelPathEnv(); err != nil {
		return nil, err
	} else if ok {
		return &modelSource{
			Type:    "registry",
			Name:    name,
			Version: version,
		}, nil
	}

	// Check for local model directory (root, recursive, or BP_MLFLOW_MODEL_PATH).
	localModelDir, err := detect.FindLocalModelDir(ctx.AppDir)
	if err == nil {
		return &modelSource{
			Type: "local",
			Path: localModelDir,
			Name: "model",
		}, nil
	}
	if !errors.Is(err, detect.ErrLocalModelNotFound) {
		return nil, err
	}

	// Check for registry model via environment variables
	name, version, ok, err := detect.DetectFromEnv()
	if err != nil {
		return nil, err
	}
	if ok {
		return &modelSource{
			Type:    "registry",
			Name:    name,
			Version: version,
		}, nil
	}

	return nil, fmt.Errorf(
		"no model source found: provide %s (root or nested, or set %s) or set BP_MLFLOW_MODEL_NAME",
		detect.MLmodelFile,
		detect.EnvModelPath,
	)
}

func getModel(ctx cnb.BuildContext, source *modelSource) (*mlflow.Model, error) {
	if source.Type == "local" {
		return mlflow.NewLocalModel(source.Path), nil
	}

	// modctl reads MLflow credentials directly from environment:
	// - MLFLOW_TRACKING_URI, MLFLOW_TRACKING_USERNAME, MLFLOW_TRACKING_PASSWORD
	// - DATABRICKS_HOST, DATABRICKS_TOKEN
	// S3 credentials are read from AWS_* env vars or ~/.aws/* files

	// Create temp directory for download
	tempDir, err := TempDir(ctx, "mlflow-model-")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}

	// Use modctl-based downloader
	downloader, err := newModelDownloader()
	if err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			return nil, errors.Join(fmt.Errorf("creating downloader: %w", err), fmt.Errorf("cleanup temp dir: %w", removeErr))
		}
		return nil, fmt.Errorf("creating downloader: %w", err)
	}

	downloadPath, err := downloader.DownloadModel(context.Background(), source.Name, source.Version, tempDir)
	if err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			return nil, errors.Join(fmt.Errorf("downloading model: %w", err), fmt.Errorf("cleanup temp dir: %w", removeErr))
		}
		return nil, fmt.Errorf("downloading model: %w", err)
	}

	return mlflow.NewRegistryModel(downloadPath, source.Name, source.Version), nil
}

func parseConda(model *mlflow.Model) (*conda.File, error) {
	if !model.HasConda() {
		// Return default conda file
		return &conda.File{
			Dependencies: []conda.Dependency{
				{Name: "python", Version: python.DefaultPythonVersion},
			},
		}, nil
	}

	return conda.ParseFile(model.CondaPath())
}

func resolveDependencies(model *mlflow.Model, extensionPackage string) (dependencySource, error) {
	condaFile, err := parseConda(model)
	if err != nil {
		return dependencySource{}, err
	}

	addMLServerDependencies(condaFile, extensionPackage)

	deps := dependencySource{
		conda: condaFile,
	}
	if !model.HasConda() && model.HasRequirements() {
		deps.requirementsPath = model.RequirementsPath()
	}

	return deps, nil
}

// addMLServerDependencies adds MLServer core and extension to conda dependencies if not present.
func addMLServerDependencies(condaFile *conda.File, extensionPackage string) {
	pipDeps := condaFile.PipDependencies()

	// Packages to add
	packagesToAdd := []string{"mlserver", "mlserver-mlflow", extensionPackage}

	// Filter out already present packages
	var newPackages []string
	for _, pkg := range packagesToAdd {
		found := false
		for _, dep := range pipDeps {
			if dep == pkg || strings.HasPrefix(dep, pkg+"==") {
				found = true
				break
			}
		}
		if !found {
			newPackages = append(newPackages, pkg)
		}
	}

	if len(newPackages) == 0 {
		return
	}

	// Find pip block and add the packages
	for i := range condaFile.Dependencies {
		if condaFile.Dependencies[i].IsPipBlock() {
			condaFile.Dependencies[i].Pip = append(condaFile.Dependencies[i].Pip, newPackages...)
			return
		}
	}

	// No pip block found, add one
	condaFile.Dependencies = append(condaFile.Dependencies, conda.Dependency{
		Pip: newPackages,
	})
}

func copyModel(model *mlflow.Model, destPath string) error {
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return err
	}

	// If model is already local, copy files
	if model.Path != "" && model.Path != destPath {
		return filepath.Walk(model.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(model.Path, path)
			if err != nil {
				return err
			}

			dest := filepath.Join(destPath, relPath)

			if info.IsDir() {
				return os.MkdirAll(dest, info.Mode())
			}

			return copyFile(path, dest, info.Mode())
		})
	}

	return nil
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing source file %q: %w", src, closeErr))
		}
	}()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing destination file %q: %w", dst, closeErr))
		}
	}()

	if _, err = io.Copy(output, input); err != nil {
		return err
	}

	return nil
}

// writeModelSettings writes the model-settings.json file for mlserver.
func writeModelSettings(modelPath string, settings map[string]interface{}) error {
	settingsPath := filepath.Join(modelPath, "model-settings.json")

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0o644)
}

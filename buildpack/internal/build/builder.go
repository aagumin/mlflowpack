// Package build implements the CNB build phase for MLflow models.
package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amazme/aipack/buildpack/internal/bindings"
	"github.com/amazme/aipack/buildpack/internal/cnb"
	"github.com/amazme/aipack/buildpack/internal/conda"
	"github.com/amazme/aipack/buildpack/internal/detect"
	"github.com/amazme/aipack/buildpack/internal/layer"
	"github.com/amazme/aipack/buildpack/internal/mlflow"
	"github.com/amazme/aipack/buildpack/internal/mlflow/storage"
	"github.com/amazme/aipack/buildpack/internal/python"
)

const (
	// EnvModelName is the environment variable for model name.
	EnvModelName = "BP_MLFLOW_MODEL_NAME"

	// EnvModelVersion is the environment variable for model version.
	EnvModelVersion = "BP_MLFLOW_MODEL_VERSION"

	// EnvModelStage is the environment variable for model stage.
	EnvModelStage = "BP_MLFLOW_MODEL_STAGE"
)

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

	// Parse MLmodel file to detect flavor
	if err := model.ParseMLmodel(); err != nil {
		return result, fmt.Errorf("parsing MLmodel: %w", err)
	}

	// Get MLServer extension based on model flavor
	mlserverExt, err := model.GetMLServerExtension()
	if err != nil {
		return result, fmt.Errorf("detecting MLServer extension: %w", err)
	}

	flavor := model.GetPrimaryFlavor()
	fmt.Printf("Detected model flavor: %s\n", flavor)
	fmt.Printf("Required MLServer extension: %s\n", mlserverExt.PipPackage)

	// Parse conda.yaml for dependencies
	condaFile, err := parseConda(model)
	if err != nil {
		return result, err
	}

	// Add MLServer core and extension to dependencies if not already present
	addMLServerDependencies(condaFile, mlserverExt.PipPackage)

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
	result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{Types: layer.DefaultModelLayerTypes()}

	// Install Python and dependencies using uv
	installer := python.NewInstaller()
	if err := installer.SetupFromConda(context.Background(), condaFile, pythonPath, venvPath); err != nil {
		return result, fmt.Errorf("setting up Python: %w", err)
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

// modelSource represents the source of the model.
type modelSource struct {
	Type    string // "local" or "registry"
	Path    string // Local path (for local models)
	Name    string // Model name (for registry models)
	Version string // Model version or stage (for registry models)
}

func determineModelSource(ctx cnb.BuildContext) (*modelSource, error) {
	// Check for local MLmodel file
	mlmodelPath := filepath.Join(ctx.AppDir, detect.MLmodelFile)
	if _, err := os.Stat(mlmodelPath); err == nil {
		return &modelSource{
			Type: "local",
			Path: ctx.AppDir,
			Name: "model",
		}, nil
	}

	// Check for registry model via environment variables
	name, version, ok := detect.DetectFromEnv()
	if ok {
		return &modelSource{
			Type:    "registry",
			Name:    name,
			Version: version,
		}, nil
	}

	return nil, fmt.Errorf("no model source found: provide MLmodel file or set BP_MLFLOW_MODEL_NAME")
}

func getModel(ctx cnb.BuildContext, source *modelSource) (*mlflow.Model, error) {
	if source.Type == "local" {
		return mlflow.NewLocalModel(source.Path), nil
	}

	// Get bindings for registry model
	bindingsDir := bindings.GetBindingsDir()
	reader := bindings.NewReader(bindingsDir)

	mlflowBinding, err := reader.ReadMLflowBinding()
	if err != nil {
		return nil, fmt.Errorf("reading MLflow binding: %w", err)
	}
	if mlflowBinding == nil {
		return nil, fmt.Errorf("MLflow binding not found at %s", bindingsDir)
	}

	// Create MLflow client
	client := mlflow.NewClient(mlflowBinding.TrackingURI,
		mlflow.WithCredentials(mlflowBinding.Username, mlflowBinding.Password))

	// Resolve model version
	modelVersion, err := client.ResolveModelVersion(context.Background(), source.Name, source.Version)
	if err != nil {
		return nil, fmt.Errorf("resolving model version: %w", err)
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "mlflow-model-")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}

	// Get S3 config from bindings
	s3Binding, err := reader.ReadS3Binding()
	if err != nil {
		return nil, fmt.Errorf("reading S3 binding: %w", err)
	}

	var s3Config storage.S3Config
	if s3Binding != nil {
		s3Config = storage.S3Config{
			Endpoint:  s3Binding.Endpoint,
			AccessKey: s3Binding.AccessKey,
			SecretKey: s3Binding.SecretKey,
			Region:    s3Binding.Region,
		}
	}

	// Download model artifacts
	backend, err := storage.DefaultBackends(context.Background(), s3Config)
	if err != nil {
		return nil, fmt.Errorf("creating storage backend: %w", err)
	}

	if err := backend.Download(context.Background(), modelVersion.ArtifactURI, tempDir); err != nil {
		return nil, fmt.Errorf("downloading model: %w", err)
	}

	return mlflow.NewRegistryModel(tempDir, source.Name, modelVersion.Version), nil
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
	if err := os.MkdirAll(destPath, 0755); err != nil {
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

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			return os.WriteFile(dest, data, info.Mode())
		})
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

	return os.WriteFile(settingsPath, data, 0644)
}

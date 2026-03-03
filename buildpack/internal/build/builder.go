// Package build implements the CNB build phase for MLflow models.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amazme/aipack/buildpack/internal/bindings"
	"github.com/amazme/aipack/buildpack/internal/conda"
	"github.com/amazme/aipack/buildpack/internal/detect"
	"github.com/amazme/aipack/buildpack/internal/layer"
	"github.com/amazme/aipack/buildpack/internal/mlflow"
	"github.com/amazme/aipack/buildpack/internal/mlflow/storage"
	"github.com/amazme/aipack/buildpack/internal/python"
	"github.com/buildpacks/libcnb/v2"
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
func Build(ctx libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

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

	// Add MLServer extension to dependencies if not already present
	addMLServerExtension(condaFile, mlserverExt.PipPackage)

	// Create Python layer
	pythonLayer, err := ctx.Layers.Layer(layer.PythonLayerName)
	if err != nil {
		return result, fmt.Errorf("creating python layer: %w", err)
	}
	pythonLayer.LayerTypes = layer.DefaultPythonLayerTypes()

	// Create venv layer
	venvLayer, err := ctx.Layers.Layer(layer.VenvLayerName)
	if err != nil {
		return result, fmt.Errorf("creating venv layer: %w", err)
	}
	venvLayer.LayerTypes = layer.DefaultVenvLayerTypes()

	// Create model layer
	modelLayer, err := ctx.Layers.Layer(layer.ModelLayerName)
	if err != nil {
		return result, fmt.Errorf("creating model layer: %w", err)
	}
	modelLayer.LayerTypes = layer.DefaultModelLayerTypes()

	// Install Python and dependencies using uv
	installer := python.NewInstaller()
	if err := installer.SetupFromConda(context.Background(), condaFile, pythonLayer.Path, venvLayer.Path); err != nil {
		return result, fmt.Errorf("setting up Python: %w", err)
	}

	// Configure PATH
	layer.PrependToPath(&pythonLayer, filepath.Join(pythonLayer.Path, "bin"))
	layer.PrependToPath(&venvLayer, filepath.Join(venvLayer.Path, "bin"))

	// Copy model to layer
	if err := copyModel(model, modelLayer.Path); err != nil {
		return result, fmt.Errorf("copying model: %w", err)
	}

	// Set MLServer environment variables
	layer.SetPythonPath(&modelLayer, modelLayer.Path)
	layer.SetLayerEnv(&modelLayer, "MLSERVER_MODEL_NAME", modelSource.Name)
	layer.SetLayerEnv(&modelLayer, "MLSERVER_MODEL_URI", modelLayer.Path)
	layer.SetLayerEnv(&modelLayer, "MLSERVER_MODEL_IMPLEMENTATION", mlserverExt.Runtime)

	// Add layers to result
	result.Layers = append(result.Layers, pythonLayer)
	result.Layers = append(result.Layers, venvLayer)
	result.Layers = append(result.Layers, modelLayer)

	// Add default process to start MLServer
	result.Processes = append(result.Processes, libcnb.Process{
		Type:    "web",
		Command: []string{"mlserver", "start", modelLayer.Path},
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

func determineModelSource(ctx libcnb.BuildContext) (*modelSource, error) {
	// Check for local MLmodel file
	mlmodelPath := filepath.Join(ctx.ApplicationPath, detect.MLmodelFile)
	if _, err := os.Stat(mlmodelPath); err == nil {
		return &modelSource{
			Type: "local",
			Path: ctx.ApplicationPath,
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

func getModel(ctx libcnb.BuildContext, source *modelSource) (*mlflow.Model, error) {
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

// addMLServerExtension adds the MLServer extension to conda dependencies if not present.
func addMLServerExtension(condaFile *conda.File, pipPackage string) {
	// Check if already present in pip dependencies
	for _, dep := range condaFile.PipDependencies() {
		if dep == pipPackage || strings.HasPrefix(dep, pipPackage+"==") {
			return
		}
	}

	// Find pip block and add the extension
	for i := range condaFile.Dependencies {
		if condaFile.Dependencies[i].IsPipBlock() {
			condaFile.Dependencies[i].Pip = append(condaFile.Dependencies[i].Pip, pipPackage)
			return
		}
	}

	// No pip block found, add one
	condaFile.Dependencies = append(condaFile.Dependencies, conda.Dependency{
		Pip: []string{pipPackage},
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

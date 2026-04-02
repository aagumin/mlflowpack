package mlflow

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
	"github.com/aagumin/mlflowpack/internal/deps"
	"github.com/aagumin/mlflowpack/internal/layer"
	"github.com/aagumin/mlflowpack/internal/python"
	"github.com/aagumin/mlflowpack/internal/sbom"
	"github.com/aagumin/mlflowpack/internal/storage"
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
	uuid, ok := metadata["model_uuid"].(string)
	if !ok {
		return ""
	}
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

	var model *Model
	var store storage.Storage
	var tempMetaDir string

	// Handle storage type (s3:// or local path) with two-phase download
	if modelSource.Type == "storage" {
		// Phase 1: Download metadata only for dependency hash computation
		tempMetaDir, err = TempDir(ctx, "mlflow-metadata-")
		if err != nil {
			return result, fmt.Errorf("creating temp metadata dir: %w", err)
		}
		defer func() {
			if err := os.RemoveAll(tempMetaDir); err != nil {
				fmt.Printf("Warning: failed to cleanup temp directory: %v\n", err)
			}
		}()

		// Create storage backend
		var storagePath string
		if modelSource.StorageType == "s3" {
			storagePath = "s3://" + modelSource.Path
		} else {
			storagePath = modelSource.Path
		}

		store, err = storage.NewStorage(storagePath)
		if err != nil {
			return result, fmt.Errorf("creating storage: %w", err)
		}

		fmt.Printf("Downloading metadata from %s...\n", store.String())
		if err := store.DownloadMetadata(context.Background(), tempMetaDir); err != nil {
			return result, fmt.Errorf("downloading model metadata: %w", err)
		}

		// Compute dependency hash from metadata
		currentDepsHash, err := deps.ComputeHash(tempMetaDir)
		if err != nil {
			return result, fmt.Errorf("computing dependency hash: %w", err)
		}

		// Get previous hash (from env or cache)
		prevDepsHash := PrevDepsHashFromEnv()
		if prevDepsHash == "" {
			prevDepsHash = CachedDepsHash(ctx.LayersDir)
		}

		fmt.Printf("Dependency hash: %s\n", currentDepsHash)
		if prevDepsHash != "" {
			fmt.Printf("Previous hash: %s\n", prevDepsHash)
		}

		// Parse MLmodel from metadata
		model = NewLocalModel(tempMetaDir)
		if err := model.ParseMLmodel(); err != nil {
			return result, fmt.Errorf("parsing MLmodel: %w", err)
		}

		// Check if dependencies changed
		depsChanged := currentDepsHash != prevDepsHash || prevDepsHash == ""

		if !depsChanged {
			fmt.Println("Dependencies unchanged, reusing cached Python and venv layers")
			// Reuse Python and venv layers (they exist from previous build)
			result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}
			result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{Types: layer.DefaultVenvLayerTypes()}
		} else {
			fmt.Println("Dependencies changed, rebuilding Python and venv layers")
			// Will rebuild Python and venv layers below
		}

		// Store the current deps hash for later use
		result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{
			Types: layer.DefaultVenvLayerTypes(),
			Metadata: map[string]interface{}{
				"deps_hash": currentDepsHash,
			},
		}
	} else {
		// Get model for local type
		model, err = getModel(ctx, modelSource)
		if err != nil {
			return result, err
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

	// Check if we need to rebuild Python and venv layers
	rebuildDeps := modelSource.Type == "storage" ||
		!result.Layers[layer.PythonLayerName].Types.Launch

	var pythonPath, venvPath string

	if rebuildDeps {
		// Create Python layer
		pythonPath, err = layer.SetupLayer(ctx.LayersDir, layer.PythonLayerName, layer.DefaultPythonLayerTypes())
		if err != nil {
			return result, fmt.Errorf("creating python layer: %w", err)
		}
		result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}

		// Create venv layer
		venvPath, err = layer.SetupLayer(ctx.LayersDir, layer.VenvLayerName, layer.DefaultVenvLayerTypes())
		if err != nil {
			return result, fmt.Errorf("creating venv layer: %w", err)
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
	} else {
		// Use existing layer paths
		venvPath = filepath.Join(ctx.LayersDir, layer.VenvLayerName)
	}

	// Create model layer
	modelPath, err := layer.SetupLayer(ctx.LayersDir, layer.ModelLayerName, layer.DefaultModelLayerTypes())
	if err != nil {
		return result, fmt.Errorf("creating model layer: %w", err)
	}

	// Phase 2: Download full model for storage type
	if modelSource.Type == "storage" && store != nil {
		fmt.Printf("Downloading full model from %s...\n", store.String())
		if err := store.Download(context.Background(), modelPath); err != nil {
			return result, fmt.Errorf("downloading model: %w", err)
		}
		// Update model path for copyModel
		model = NewLocalModel(modelPath)
	} else {
		// Copy model to layer (for local type)
		if err := copyModel(model, modelPath); err != nil {
			return result, fmt.Errorf("copying model: %w", err)
		}
	}

	// Get Python version for metadata
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

	// Add image labels
	result.Launch.Labels = []cnb.Label{
		{Key: "org.opencontainers.image.title", Value: modelSource.Name},
		{Key: "org.opencontainers.image.version", Value: modelSource.Version},
		{Key: "org.opencontainers.image.description", Value: "MLflow model served by MLServer"},
		{Key: "io.github.aagumin.model-flavor", Value: flavor},
		{Key: "io.github.aagumin.model-name", Value: modelSource.Name},
		{Key: "io.github.aagumin.mlserver-runtime", Value: mlserverExt.Runtime},
	}

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
	Type        string // "local" or "storage" (s3/local path)
	Path        string // Local path (for local models) or storage path
	Name        string // Model name (for image labels)
	Version     string // Model version (for image labels)
	StorageType string // "s3" or "local" for storage type models
}

type dependencySource struct {
	conda            *conda.File
	requirementsPath string
}

func determineModelSource(ctx cnb.BuildContext) (*modelSource, error) {
	// Check for storage path (s3:// or local path via BP_MLFLOW_MODEL_PATH)
	if storageType, path, ok := DetectStoragePath(); ok {
		return &modelSource{
			Type:        "storage",
			Path:        path,
			Name:        "model",
			Version:     "latest",
			StorageType: storageType,
		}, nil
	}

	// Check for local model directory (root, recursive, or BP_MLFLOW_MODEL_PATH).
	localModelDir, err := FindLocalModelDir(ctx.AppDir)
	if err == nil {
		return &modelSource{
			Type: "local",
			Path: localModelDir,
			Name: "model",
		}, nil
	}
	if !errors.Is(err, ErrLocalModelNotFound) {
		return nil, err
	}

	return nil, fmt.Errorf(
		"no model source found: provide %s (root or nested, or set %s to s3://bucket/path or /absolute/path)",
		MLmodelFile,
		EnvModelPath,
	)
}

func getModel(ctx cnb.BuildContext, source *modelSource) (*Model, error) {
	if source.Type == "local" {
		return NewLocalModel(source.Path), nil
	}

	if source.Type == "storage" {
		// For storage type, model will be downloaded in Build function
		// Return a model with path set to empty (will be set after download)
		return &Model{}, nil
	}

	return nil, fmt.Errorf("unsupported model source type: %s", source.Type)
}

func parseConda(model *Model) (*conda.File, error) {
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

func resolveDependencies(model *Model, extensionPackage string) (dependencySource, error) {
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

func copyModel(model *Model, destPath string) error {
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

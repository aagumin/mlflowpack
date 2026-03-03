// Package mlflow provides MLflow model utilities.
package mlflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MLmodel represents the MLmodel file structure.
type MLmodel struct {
	ArtifactPath  string                 `yaml:"artifact_path,omitempty"`
	Flavors       map[string]Flavor      `yaml:"flavors"`
	MLflowVersion string                 `yaml:"mlflow_version,omitempty"`
	ModelUUID     string                 `yaml:"model_uuid,omitempty"`
	RunID         string                 `yaml:"run_id,omitempty"`
	TimeCreated   string                 `yaml:"time_created,omitempty"`
	ModelSize     int64                  `yaml:"model_size_bytes,omitempty"`
	Signature     map[string]interface{} `yaml:"signature,omitempty"`
}

// Flavor represents a model flavor configuration.
type Flavor struct {
	LoaderModule        string `yaml:"loader_module,omitempty"`
	ModelPath           string `yaml:"model_path,omitempty"`
	PickledModel        string `yaml:"pickled_model,omitempty"`
	Code                string `yaml:"code,omitempty"`
	SklearnVersion      string `yaml:"sklearn_version,omitempty"`
	XGBoostVersion      string `yaml:"xgboost_version,omitempty"`
	LightGBMVersion     string `yaml:"lightgbm_version,omitempty"`
	TensorflowVersion   string `yaml:"tensorflow_version,omitempty"`
	PyTorchVersion      string `yaml:"pytorch_version,omitempty"`
	TransformersVersion string `yaml:"transformers_version,omitempty"`
}

// MLServerExtension maps MLflow flavors to MLServer extensions.
type MLServerExtension struct {
	PipPackage string
	Runtime    string
}

// MLServerExtensions maps flavor names to their MLServer extensions.
var MLServerExtensions = map[string]MLServerExtension{
	"sklearn": {
		PipPackage: "mlserver-sklearn",
		Runtime:    "mlserver_sklearn.SKLearnRuntime",
	},
	"xgboost": {
		PipPackage: "mlserver-xgboost",
		Runtime:    "mlserver_xgboost.XGBoostRuntime",
	},
	"lightgbm": {
		PipPackage: "mlserver-lightgbm",
		Runtime:    "mlserver_lightgbm.LightGBMRuntime",
	},
	"tensorflow": {
		PipPackage: "mlserver-tensorflow",
		Runtime:    "mlserver_tensorflow.TensorFlowRuntime",
	},
	"pytorch": {
		PipPackage: "mlserver-torchserve",
		Runtime:    "mlserver_torchserve.TorchServeRuntime",
	},
	"transformers": {
		PipPackage: "mlserver-huggingface",
		Runtime:    "mlserver_huggingface.HuggingFaceRuntime",
	},
	"mlflow": {
		PipPackage: "mlserver-mlflow",
		Runtime:    "mlserver_mlflow.MLflowRuntime",
	},
	"python_function": {
		PipPackage: "mlserver-mlflow",
		Runtime:    "mlserver_mlflow.MLflowRuntime",
	},
}

// ParseMLmodel parses an MLmodel file.
func ParseMLmodel(path string) (*MLmodel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading MLmodel file: %w", err)
	}

	var mlmodel MLmodel
	if err := yaml.Unmarshal(data, &mlmodel); err != nil {
		return nil, fmt.Errorf("parsing MLmodel yaml: %w", err)
	}

	return &mlmodel, nil
}

// GetPrimaryFlavor returns the primary flavor for the model.
// Priority: sklearn > xgboost > lightgbm > tensorflow > pytorch > transformers > mlflow > python_function
func (m *MLmodel) GetPrimaryFlavor() string {
	priority := []string{
		"sklearn", "xgboost", "lightgbm", "tensorflow",
		"pytorch", "transformers", "mlflow", "python_function",
	}

	for _, flavor := range priority {
		if _, ok := m.Flavors[flavor]; ok {
			return flavor
		}
	}

	// Return first flavor if no priority match
	for flavor := range m.Flavors {
		return flavor
	}

	return ""
}

// GetMLServerExtension returns the MLServer extension for the model's primary flavor.
func (m *MLmodel) GetMLServerExtension() (*MLServerExtension, error) {
	flavor := m.GetPrimaryFlavor()
	if flavor == "" {
		return nil, fmt.Errorf("no flavor found in model")
	}

	ext, ok := MLServerExtensions[flavor]
	if !ok {
		return nil, fmt.Errorf("unsupported flavor: %s", flavor)
	}

	return &ext, nil
}

// GetRequiredPipPackages returns the pip packages needed for the model.
func (m *MLmodel) GetRequiredPipPackages() ([]string, error) {
	ext, err := m.GetMLServerExtension()
	if err != nil {
		return nil, err
	}

	return []string{ext.PipPackage}, nil
}

// GetRuntimeImplementation returns the MLServer runtime implementation class.
func (m *MLmodel) GetRuntimeImplementation() (string, error) {
	ext, err := m.GetMLServerExtension()
	if err != nil {
		return "", err
	}

	return ext.Runtime, nil
}

package mlflow

import "testing"

func TestGetMLServerExtensionMLflowRuntime(t *testing.T) {
	tests := []struct {
		name   string
		flavor string
	}{
		{name: "mlflow flavor", flavor: "mlflow"},
		{name: "python function flavor", flavor: "python_function"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &MLmodel{Flavors: map[string]Flavor{tt.flavor: {}}}

			ext, err := model.GetMLServerExtension()
			if err != nil {
				t.Fatalf("GetMLServerExtension() error = %v", err)
			}

			if ext.PipPackage != "mlserver-mlflow" {
				t.Fatalf("PipPackage = %q, want %q", ext.PipPackage, "mlserver-mlflow")
			}

			if ext.Runtime != "mlserver_mlflow.MLflowRuntime" {
				t.Fatalf("Runtime = %q, want %q", ext.Runtime, "mlserver_mlflow.MLflowRuntime")
			}
		})
	}
}

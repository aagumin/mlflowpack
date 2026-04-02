package provider

import (
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

type mockProvider struct {
	name      string
	pass      bool
	detectErr error
	buildErr  error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	return cnb.DetectResult{Pass: m.pass}, m.detectErr
}

func (m *mockProvider) Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	return cnb.BuildResult{}, m.buildErr
}

func TestRegisterAndAll(t *testing.T) {
	// Reset registry
	registry = nil

	p1 := &mockProvider{name: "alpha"}
	p2 := &mockProvider{name: "beta"}

	Register(p1)
	Register(p2)

	all := All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d providers, want 2", len(all))
	}
	if all[0].Name() != "alpha" {
		t.Fatalf("All()[0] = %q, want %q", all[0].Name(), "alpha")
	}
	if all[1].Name() != "beta" {
		t.Fatalf("All()[1] = %q, want %q", all[1].Name(), "beta")
	}
}

func TestByName(t *testing.T) {
	registry = nil

	p := &mockProvider{name: "mlflow"}
	Register(p)

	got := ByName("mlflow")
	if got == nil {
		t.Fatal("ByName(mlflow) = nil, want provider")
	}
	if got.Name() != "mlflow" {
		t.Fatalf("ByName(mlflow).Name() = %q, want %q", got.Name(), "mlflow")
	}

	missing := ByName("nonexistent")
	if missing != nil {
		t.Fatal("ByName(nonexistent) should return nil")
	}
}

func TestDetectFirst(t *testing.T) {
	registry = nil

	p1 := &mockProvider{name: "fail-first", pass: false}
	p2 := &mockProvider{name: "pass-second", pass: true}

	Register(p1)
	Register(p2)

	provider, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if !result.Pass {
		t.Fatal("DetectFirst() result.Pass = false, want true")
	}
	if provider.Name() != "pass-second" {
		t.Fatalf("DetectFirst() provider = %q, want %q", provider.Name(), "pass-second")
	}
}

func TestDetectFirst_NonePass(t *testing.T) {
	registry = nil

	Register(&mockProvider{name: "fail1", pass: false})
	Register(&mockProvider{name: "fail2", pass: false})

	_, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if result.Pass {
		t.Fatal("DetectFirst() should not pass when all providers fail")
	}
}

func TestDetectFirst_EmptyRegistry(t *testing.T) {
	registry = nil

	_, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if result.Pass {
		t.Fatal("DetectFirst() should not pass with empty registry")
	}
}

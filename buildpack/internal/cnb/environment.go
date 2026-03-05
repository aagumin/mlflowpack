package cnb

import (
	"os"
	"path/filepath"
)

// EnvironmentAction determines how to modify an environment variable.
// File suffixes: .override, .prepend, .append, .default
type EnvironmentAction int

const (
	ActionOverride EnvironmentAction = iota // VAR.override - replace value
	ActionPrepend                            // VAR.prepend - prepend to value
	ActionAppend                             // VAR.append - append to value
	ActionDefault                            // VAR.default - set if not already set
)

// WriteEnvFile writes an environment variable file to a layer's env directory.
// Used for build-time environment modifications (affects subsequent buildpacks).
func WriteEnvFile(layerPath, name, value string, action EnvironmentAction) error {
	return writeEnvFile(layerPath, "env", name, value, action)
}

// WriteEnvLaunchFile writes an environment variable file to a layer's env.launch directory.
// Used for launch-time environment modifications (affects runtime).
func WriteEnvLaunchFile(layerPath, name, value string, action EnvironmentAction) error {
	return writeEnvFile(layerPath, "env.launch", name, value, action)
}

// WriteEnvBuildFile writes an environment variable file to a layer's env.build directory.
// Used for build-time environment modifications (affects subsequent buildpacks only).
func WriteEnvBuildFile(layerPath, name, value string, action EnvironmentAction) error {
	return writeEnvFile(layerPath, "env.build", name, value, action)
}

func writeEnvFile(layerPath, envDir, name, value string, action EnvironmentAction) error {
	dir := filepath.Join(layerPath, envDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var filename string
	switch action {
	case ActionOverride:
		filename = name + ".override"
	case ActionPrepend:
		filename = name + ".prepend"
	case ActionAppend:
		filename = name + ".append"
	case ActionDefault:
		filename = name + ".default"
	}

	return os.WriteFile(filepath.Join(dir, filename), []byte(value), 0644)
}

// PrependToPath prepends a path to PATH environment variable for build and launch.
func PrependToPath(layerPath, path string) error {
	if err := WriteEnvFile(layerPath, "PATH", path, ActionPrepend); err != nil {
		return err
	}
	return WriteEnvLaunchFile(layerPath, "PATH", path, ActionPrepend)
}

// SetEnvDefault sets an environment variable only if not already set (build and launch).
func SetEnvDefault(layerPath, name, value string) error {
	if err := WriteEnvFile(layerPath, name, value, ActionDefault); err != nil {
		return err
	}
	return WriteEnvLaunchFile(layerPath, name, value, ActionDefault)
}

// SetEnvLaunchDefault sets an environment variable at launch time only if not already set.
func SetEnvLaunchDefault(layerPath, name, value string) error {
	return WriteEnvLaunchFile(layerPath, name, value, ActionDefault)
}

// SetEnvOverride sets an environment variable, overriding any existing value (build and launch).
func SetEnvOverride(layerPath, name, value string) error {
	if err := WriteEnvFile(layerPath, name, value, ActionOverride); err != nil {
		return err
	}
	return WriteEnvLaunchFile(layerPath, name, value, ActionOverride)
}

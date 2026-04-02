// Package cnb provides minimal types for Cloud Native Buildpacks API 0.12.
// Based on buildpacks/lifecycle reference implementation.
package cnb

// BuildContext contains inputs for build phase (from CNB_* env vars).
type BuildContext struct {
	LayersDir    string // CNB_LAYERS_DIR
	PlatformDir  string // CNB_PLATFORM_DIR
	BpPlanPath   string // CNB_BP_PLAN_PATH
	BuildpackDir string // CNB_BUILDPACK_DIR
	AppDir       string // Working directory (current dir)
	ExecEnv      string // CNB_EXEC_ENV (API 0.12+, optional)
}

// DetectContext contains inputs for detect phase (from CNB_* env vars).
type DetectContext struct {
	PlatformDir   string // CNB_PLATFORM_DIR
	BuildPlanPath string // CNB_BUILD_PLAN_PATH
	BuildpackDir  string // CNB_BUILDPACK_DIR
	AppDir        string // Working directory (current dir)
	ExecEnv       string // CNB_EXEC_ENV (API 0.12+, optional)
}

// LayerTypes describes how a layer is used (maps to [types] table in layer.toml).
type LayerTypes struct {
	Build  bool `toml:"build"`
	Launch bool `toml:"launch"`
	Cache  bool `toml:"cache"`
}

// LayerMetadata represents a layer's metadata file (layer.toml).
// Matches lifecycle's LayerMetadataFile structure.
type LayerMetadata struct {
	Types    LayerTypes `toml:"types,omitempty"`
	Metadata any        `toml:"metadata,omitempty"`
}

// ProcessEntry represents a launch process entry in launch.toml.
// Buildpacks write ProcessEntry; the launcher converts to launch.Process at runtime.
type ProcessEntry struct {
	Type             string   `toml:"type"`
	Command          []string `toml:"command"` // API 0.9+: array of strings
	Args             []string `toml:"args,omitempty"`
	Default          bool     `toml:"default,omitempty"`
	WorkingDirectory string   `toml:"working-dir,omitempty"`
	ExecEnv          []string `toml:"exec-env,omitempty"` // API 0.12+
}

// Label represents an image label in launch.toml.
type Label struct {
	Key   string `toml:"key"`
	Value string `toml:"value"`
}

// LaunchTOML represents the launch.toml file structure.
type LaunchTOML struct {
	Processes []ProcessEntry `toml:"processes,omitempty"`
	Labels    []Label        `toml:"labels,omitempty"`
}

// BuildResult contains outputs from build phase.
type BuildResult struct {
	Layers map[string]LayerMetadata
	Launch LaunchTOML
}

// DetectResult contains outputs from detect phase.
type DetectResult struct {
	Pass bool
}

// BuildPlan represents the build plan written during detect.
type BuildPlan struct {
	Provides []BuildPlanEntry `toml:"provides"`
	Requires []BuildPlanEntry `toml:"requires"`
}

// BuildPlanEntry represents a single provides/requires entry.
type BuildPlanEntry struct {
	Name     string                 `toml:"name"`
	Metadata map[string]interface{} `toml:"metadata,omitempty"`
}

// Exit codes for detect phase.
const (
	ExitCodePass = 0   // Detection passed
	ExitCodeFail = 100 // Detection failed (not an error)
	ExitCodeErr  = 1   // Error occurred
)

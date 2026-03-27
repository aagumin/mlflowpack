# Contributing to MLflow Model Buildpack

Thanks for your interest in the project! This document describes the development process and how to contribute.

## Conventions

### Commits

The project follows [Conventional Commits v1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).

Commit message format:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat` — new functionality
- `fix` — bug fix
- `docs` — documentation changes
- `style` — formatting, missing semicolons
- `refactor` — refactoring without behavior change
- `test` — adding or fixing tests
- `chore` — build, tool changes

**Examples:**

```bash
feat(mlflow): add support for LightGBM flavor
fix(python): correct Python binary lookup in uv installation
docs(readme): update installation instructions
refactor(build): simplify layer creation logic
```

### Go Code Style

The project follows [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

Key principles:
- Use `gofmt` and `goimports`
- Avoid uninformative names (data, info, thing)
- Return interfaces, accept concrete types
- Prefer channels over mutexes for communication
- Use context for cancellation

```bash
# Install linters
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Check
make lint
```

## Development

### Prerequisites

- **Go** >= 1.24
- **Docker** or **Podman**
- **pack** CLI >= 0.38.0
- **Make** (optional, but recommended)

### Installing Tools

```bash
# macOS
brew install go pack

# Linux
# Go: https://go.dev/doc/install
# pack: https://buildpacks.io/docs/tools/pack/
```

### Cloning and Building

```bash
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack

# Install Go dependencies
cd buildpack && go mod download

# Build binaries
make build
```

### Project Structure

```
mlflowpack/
├── buildpack/                   # CNB Buildpack
│   ├── cmd/
│   │   ├── detect/main.go       # Detect phase entrypoint
│   │   └── build/main.go        # Build phase entrypoint
│   ├── internal/
│   │   ├── cnb/                 # CNB API types
│   │   ├── detect/detector.go   # Detection logic
│   │   ├── build/builder.go     # Main build logic
│   │   ├── mlflow/              # MLflow client and parsers
│   │   ├── conda/parser.go      # conda.yaml parser
│   │   ├── python/installer.go  # Python installation via uv
│   │   ├── sbom/                # SBOM generation
│   │   └── layer/layers.go      # Layer management
│   ├── buildpack.toml.template  # Template (versioned from git)
│   └── go.mod
├── stack/
│   ├── build/Dockerfile         # Build image
│   └── run/Dockerfile           # Run image
├── e2e/                         # E2E tests and models
│   ├── models/
│   │   ├── pyfunc/              # python_function test model
│   │   └── sklearn/             # sklearn test model
│   └── scripts/
├── docs/
│   └── plans/                   # Design documents
├── Makefile
└── builder.toml.template
```

### Building and Testing

```bash
# Build buildpack
make build

# Run unit tests
make test

# Linting
make lint

# Full cycle: stack + package + builder
make builder
```

### Versioning

The buildpack version is derived from git tags:

```bash
# Check current version
git describe --tags --always --dirty

# During packaging, buildpack.toml is generated from template
# with the version injected
make package
```

### Local Testing with Model

```bash
# Build test image
pack build test-mlflow-model \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --path e2e/models/sklearn \
  --pull-policy never \
  --trust-builder

# Run container
docker run --rm -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 test-mlflow-model:latest

# Test prediction
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

### Adding a New MLflow Flavor

1. Add mapping in `buildpack/internal/mlflow/flavor.go`:

   ```go
   var MLServerExtensions = map[string]MLServerExtension{
       // ...
       "newflavor": {
           PipPackage: "mlserver-newflavor",
           Runtime:    "mlserver_newflavor.NewFlavorModel",
       },
   }
   ```

2. Update priority in `GetPrimaryFlavor()`

3. Update documentation (README.md, docs/USAGE.md)

### Debugging

Enable debug logging:

```bash
pack build my-model \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --path e2e/models/sklearn \
  --pull-policy never \
  --env CNB_LOG_LEVEL=debug
```

Inspect layer contents:

```bash
# Create container without running
docker create --name debug my-model

# Copy layers
docker cp debug:/layers ./layers-debug

# View structure
find layers-debug -type f | head -50
```

## CI/CD

The project uses GitHub Actions for:
- Running tests on PR
- Building and publishing images on release
- Code linting

## Questions and Issues

- **Bugs**: Create an issue with problem description and reproduction steps
- **Feature requests**: Create an issue describing desired functionality
- **Questions**: Create a discussion or issue with `question` label

## License

By contributing to the project, you agree that your code will be distributed under the MIT license.

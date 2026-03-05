# Repository Guidelines

CNCF Buildpack for building container images with ML models from MLflow Model Registry. Uses MLServer as runtime and installs Python dependencies via uv. Works with pack and custom Kubernetes operators in unprivileged mode.



## Project Structure & Module Organization
- `buildpack/` contains the main Go module (`go.mod`) and CNB implementation.
- `buildpack/cmd/{build,detect}/` are entrypoints for buildpack phases.
- `buildpack/internal/` holds core packages: `build`, `detect`, `mlflow`, `python`, `conda`, `bindings`, `layer`, and `cnb`.
- `stack/{build,run}/` contains Dockerfiles for build/run base images.
- `docs/` includes user docs and design plans (`docs/plans/`).
- `context/` stores Buildpack and Platform API specs used for implementation reference.

## Build, Test, and Development Commands
- `make build` compiles `buildpack/bin/{build,detect}` for Linux.
- `make test` runs Go tests with race detector (`go test -v -race ./...`).
- `make lint` runs `golangci-lint` for the Go module.
- `make stack` builds stack images (`amazme/fedora-mlserver-build:43`, `...-run:43`).
- `make package` packages the buildpack for `pack`.
- `make builder` runs full local builder assembly (`stack + package + builder create`).
- Direct module test: `cd buildpack && go test ./...`.

## Coding Style & Naming Conventions
- Follow Uber Go Style Guide and standard Go conventions.
- Use `gofmt` (and `goimports` when needed) before committing.
- Keep package names lowercase; exported identifiers use `CamelCase`.
- Prefer descriptive names (`modelSource`, `ResolveModelVersion`) over generic names.
- Keep CNB-facing behavior explicit: env vars (`CNB_*`, `BP_MLFLOW_*`) and layer names (`python`, `venv`, `model`) should remain stable.

## Testing Guidelines
- Primary framework: Go `testing` package.
- Place tests next to code as `*_test.go` files (table-driven tests preferred).
- Cover detect/build flow, MLflow flavor resolution, conda parsing, and storage backends.
- Run `make test` locally before opening a PR; add regression tests for bug fixes.

## Commit & Pull Request Guidelines
- Use Conventional Commits: `feat(scope): ...`, `fix(scope): ...`, `docs(scope): ...`, `refactor(scope): ...`, `test(scope): ...`, `chore(scope): ...`.
- Keep commits focused and logically grouped.
- PRs should include:
  - what changed and why,
  - linked issue/design doc (if available),
  - verification output (commands run, e.g. `make test`, `make lint`),
  - any behavior/config changes (env vars, bindings, builder/stack tags).

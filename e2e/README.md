# E2E Assets and Scripts

`e2e/` contains deterministic local integration test data for the buildpack:

- `models/pyfunc` - MLflow `python_function` test model
- `models/sklearn` - MLflow `sklearn` test model
- `scripts/` - build/runtime verification scripts

## Prerequisites

- Builder image created locally (`make builder`)
- `pack` (or `lima pack` on macOS)
- Docker (or alternative container runtime via `CONTAINER_TOOL`)
- `curl`
- `python3`

## Run checks

```bash
# Full e2e verification for all models
./e2e/scripts/run-all.sh

# Build-only checks
./e2e/scripts/verify-build.sh pyfunc
./e2e/scripts/verify-build.sh sklearn

# Build + runtime inference checks
./e2e/scripts/verify-runtime.sh pyfunc
./e2e/scripts/verify-runtime.sh sklearn
```

## Regenerate model artifacts

Artifacts are committed to git to keep e2e checks reproducible.

```bash
./.venv/bin/python e2e/scripts/generate_models.py
```

## Environment overrides

- `BUILDER_IMAGE` - builder to use (default: `aagumin/mlserver-builder:<git-describe>`)
- `PACK_CMD` - override pack command (example: `PACK_CMD='lima pack'`)
- `CONTAINER_TOOL` - container runtime command (default: `docker`)
- `IMAGE_PREFIX` / `IMAGE_SUFFIX` - output image tag customization
- `E2E_PORT` - force runtime check port
- `READINESS_ATTEMPTS` / `READINESS_SLEEP_SECONDS` - readiness polling tuning

# Simplify Registry Credentials Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove duplicate credential handling by deleting the bindings package and letting modctl handle all credential reading directly.

**Architecture:** Delete `bindings/` package, simplify `builder.go` to not validate credentials upfront, update documentation to show minimal env var setup.

**Tech Stack:** Go 1.24, modctl library (handles MLflow/AWS credentials via databricks-sdk-go and aws-sdk-go-v2)

---

## Task 1: Delete bindings package

**Files:**
- Delete: `buildpack/internal/bindings/bindings.go`
- Delete: `buildpack/internal/bindings/bindings_test.go`

**Step 1: Delete bindings.go**

```bash
rm buildpack/internal/bindings/bindings.go
```

**Step 2: Delete bindings_test.go**

```bash
rm buildpack/internal/bindings/bindings_test.go
```

**Step 3: Remove bindings directory if empty**

```bash
rmdir buildpack/internal/bindings
```

**Step 4: Commit**

```bash
git add -A buildpack/internal/bindings
git commit -m "refactor(bindings): remove unused bindings package

modctl handles credential reading directly via databricks-sdk-go
and aws-sdk-go-v2, making this package redundant."
```

---

## Task 2: Update builder.go to remove bindings dependency

**Files:**
- Modify: `buildpack/internal/build/builder.go`

**Step 1: Remove bindings import**

Find and remove this import:
```go
"github.com/aagumin/mlflowpack/internal/bindings"
```

**Step 2: Simplify getModel function**

Replace the credential validation block in `getModel()` function (lines 212-223).

Before:
```go
// Get bindings with env vars fallback
bindingsDir := bindings.GetBindingsDir()
reader := bindings.NewReader(bindingsDir)

// Check for MLflow binding (optional - env vars work without it)
mlflowBinding, err := reader.ReadMLflowBindingWithFallback()
if err != nil {
	return nil, fmt.Errorf("reading MLflow binding: %w", err)
}
if mlflowBinding == nil {
	return nil, fmt.Errorf("MLflow credentials not found: provide bindings at %s or set MLFLOW_TRACKING_URI environment variable", bindingsDir)
}
```

After:
```go
// modctl reads MLflow credentials directly from environment:
// - MLFLOW_TRACKING_URI, MLFLOW_TRACKING_USERNAME, MLFLOW_TRACKING_PASSWORD
// - DATABRICKS_HOST, DATABRICKS_TOKEN
// S3 credentials are read from AWS_* env vars or ~/.aws/* files
```

**Step 3: Verify build**

```bash
cd buildpack && go build ./...
```

Expected: No errors

**Step 4: Commit**

```bash
git add buildpack/internal/build/builder.go
git commit -m "refactor(build): remove bindings dependency from builder

modctl handles all credential reading directly."
```

---

## Task 3: Run tests and fix any failures

**Files:**
- Test: `buildpack/internal/build/` tests

**Step 1: Run all tests**

```bash
cd buildpack && go test ./...
```

Expected: All tests pass

**Step 2: If tests fail, investigate and fix**

Tests that may reference bindings need to be updated. Check test files for any bindings imports.

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "test: update tests after bindings removal"
```

---

## Task 4: Clean up go.mod

**Files:**
- Modify: `buildpack/go.mod`

**Step 1: Run go mod tidy**

```bash
cd buildpack && go mod tidy
```

**Step 2: Verify build still works**

```bash
cd buildpack && go build ./...
```

**Step 3: Commit**

```bash
git add buildpack/go.mod buildpack/go.sum
git commit -m "chore: tidy go.mod after bindings removal"
```

---

## Task 5: Update documentation

**Files:**
- Modify: `docs/USAGE.md`

**Step 1: Simplify "Сборка с MLflow Registry" section**

Replace the entire "Сборка с MLflow Registry" section (lines 167-256) with:

```markdown
## Сборка с MLflow Registry

### Минимальная настройка

Укажите путь к модели и креды MLflow через переменные окружения:

```bash
# MLflow credentials
export MLFLOW_TRACKING_URI="https://mlflow.your-company.com"
export MLFLOW_TRACKING_USERNAME="your-username"
export MLFLOW_TRACKING_PASSWORD="your-password"

# ИЛИ Databricks credentials
export DATABRICKS_HOST="https://your-workspace.cloud.databricks.com"
export DATABRICKS_TOKEN="your-token"

# Сборка
pack build my-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1"
```

### Переменные окружения

**MLflow:**

| Переменная | Описание |
|------------|----------|
| `MLFLOW_TRACKING_URI` | URL MLflow сервера |
| `MLFLOW_TRACKING_USERNAME` | Basic auth username |
| `MLFLOW_TRACKING_PASSWORD` | Basic auth password |

**Databricks:**

| Переменная | Описание |
|------------|----------|
| `DATABRICKS_HOST` | Databricks workspace URL |
| `DATABRICKS_TOKEN` | Personal access token |

**S3 (для артефактов):**

| Переменная | Описание |
|------------|----------|
| `AWS_ACCESS_KEY_ID` | Access key |
| `AWS_SECRET_ACCESS_KEY` | Secret key |
| `AWS_REGION` | Region (default: us-east-1) |
| `AWS_ENDPOINT_URL` | Custom S3 endpoint (MinIO, etc.) |

### Полная команда сборки

```bash
pack build my-registry-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.company.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass"
```
```

**Step 2: Remove "Service Bindings структура" section**

Find and remove the section (around lines 280-293):
```markdown
### Service Bindings структура

/bindings/mlflow/
├── type              # "mlflow" (обязательно)
...
```

**Step 3: Update "Переменные окружения buildpack" table**

Update the table to clarify that only `BP_MLFLOW_MODEL_PATH` is needed for registry:

```markdown
### Переменные окружения buildpack

| Переменная | Обязательно | Описание |
|------------|-------------|----------|
| `BP_MLFLOW_MODEL_PATH` | Да* | Путь к модели: локальный путь ИЛИ `models:/<name>[/<version-or-stage>]` |

*\* Обязательно только при сборке из registry. При локальной модели определяется автоматически.*
```

**Step 4: Commit**

```bash
git add docs/USAGE.md
git commit -m "docs: simplify registry credentials documentation

Remove Service Bindings complexity, show minimal env var setup."
```

---

## Task 6: Run lint and final verification

**Step 1: Run linter**

```bash
cd buildpack && make lint
```

Expected: No errors

**Step 2: Run all tests**

```bash
cd buildpack && make test
```

Expected: All tests pass

**Step 3: Build the buildpack**

```bash
cd buildpack && make build
```

Expected: Build succeeds

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: resolve lint issues after refactoring"
```

---

## Summary

After completing this plan:
- `bindings/` package removed
- `builder.go` simplified
- Documentation updated
- User only needs to set `BP_MLFLOW_MODEL_PATH` and MLflow/AWS credentials

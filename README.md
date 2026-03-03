# MLflow Model Buildpack

CNCF Buildpack для сборки container-образов с ML моделями из MLflow Model Registry.

## Особенности

- **Unprivileged сборка** — работает в rootless режиме без специальных привилегий
- **MLServer runtime** — использует Seldon MLServer для инференса
- **Автоопределение flavor** — автоматически выбирает нужный MLServer extension (sklearn, xgboost, lightgbm, tensorflow, pytorch, transformers)
- **Быстрая установка зависимостей** — использует uv вместо pip
- **Совместимость** — работает с pack, kpack, и кастомными Kubernetes операторами

## Поддерживаемые модели

| Flavor | MLServer Extension | Runtime |
|--------|-------------------|---------|
| sklearn | mlserver-sklearn | mlserver_sklearn.SKLearnRuntime |
| xgboost | mlserver-xgboost | mlserver_xgboost.XGBoostRuntime |
| lightgbm | mlserver-lightgbm | mlserver_lightgbm.LightGBMRuntime |
| tensorflow | mlserver-tensorflow | mlserver_tensorflow.TensorFlowRuntime |
| pytorch | mlserver-torchserve | mlserver_torchserve.TorchServeRuntime |
| transformers | mlserver-huggingface | mlserver_huggingface.HuggingFaceRuntime |

## Быстрый старт

### Предварительные требования

- [pack](https://buildpacks.io/docs/tools/pack/) >= 0.30.0
- Docker или Podman
- Go >= 1.21 (для разработки)

### Локальная сборка с локальной моделью

```bash
# Клонировать репозиторий
git clone https://github.com/amazme/aipack.git
cd aipack

# Собрать stack images и builder
make stack builder

# Собрать образ с моделью
pack build my-mlflow-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --path /path/to/model/with/MLmodel

# Запустить
docker run -p 8080:8080 my-mlflow-model
```

### Сборка с моделью из MLflow Registry

```bash
# Создать service bindings
mkdir -p bindings/mlflow
echo "mlflow" > bindings/mlflow/type
echo "https://mlflow.example.com" > bindings/mlflow/tracking_uri
echo "username" > bindings/mlflow/username
echo "password" > bindings/mlflow/password

# S3 credentials (если артефакты в S3)
mkdir -p bindings/mlflow/s3
echo "https://s3.example.com" > bindings/mlflow/s3/endpoint
echo "access_key" > bindings/mlflow/s3/access_key
echo "secret_key" > bindings/mlflow/s3/secret_key

# Собрать с переменными окружения
pack build my-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_NAME=my-classifier \
  --env BP_MLFLOW_MODEL_VERSION=latest \
  --volume ./bindings:/bindings/mlflow
```

### Запуск инференса

```bash
# HTTP API
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'

# gRPC API (порт 9080)
grpcurl -plaintext localhost:9080 list
```

## Структура проекта

```
aipack/
├── buildpack/                 # CNB Buildpack
│   ├── cmd/
│   │   ├── detect/main.go     # Detect phase
│   │   └── build/main.go      # Build phase
│   ├── internal/
│   │   ├── detect/            # Detection logic
│   │   ├── build/             # Build logic
│   │   ├── mlflow/            # MLflow client & model parsing
│   │   ├── conda/             # conda.yaml parser
│   │   ├── python/            # Python installer (uv)
│   │   └── layer/             # Layer utilities
│   ├── buildpack.toml
│   └── package.toml
├── stack/
│   ├── build/Dockerfile       # Build image
│   └── run/Dockerfile         # Run image
├── builder.toml               # Builder configuration
├── Makefile
└── docs/
    └── USAGE.md               # Подробная документация
```

## Команды Makefile

```bash
make build          # Собрать buildpack binaries
make test           # Запустить unit тесты
make lint           # Запустить linter
make stack          # Собрать stack images (build + run)
make package        # Упаковать buildpack
make builder        # Создать builder
make test-build     # Тестовая сборка модели
```

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `BP_MLFLOW_MODEL_NAME` | Имя модели в Registry | — |
| `BP_MLFLOW_MODEL_VERSION` | Версия модели | `latest` |
| `BP_MLFLOW_MODEL_STAGE` | Stage модели (Production, Staging) | — |

### Service Bindings

```
/bindings/mlflow/
├── type           # "mlflow"
├── tracking_uri   # https://mlflow.example.com
├── username       # (optional)
├── password       # (optional)
└── s3/            # S3 credentials (optional)
    ├── endpoint
    ├── access_key
    ├── secret_key
    └── region
```

## Kubernetes / kpack

Пример использования с kpack:

```yaml
apiVersion: kpack.io/v1alpha2
kind: Image
metadata:
  name: mlflow-model-image
spec:
  tag: registry.example.com/my-model:latest
  builder:
    name: mlserver-builder
    kind: ClusterBuilder
  serviceAccountName: mlflow-service-account
  source:
    blob:
      url: file:///empty  # Пустой source, модель из registry
  env:
    - name: BP_MLFLOW_MODEL_NAME
      value: "my-classifier"
    - name: BP_MLFLOW_MODEL_VERSION
      value: "latest"
```

## Разработка

См. [CONTRIBUTING.md](CONTRIBUTING.md) для руководства по разработке.

## Лицензия

MIT

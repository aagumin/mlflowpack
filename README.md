# MLflow Model Buildpack

CNCF Buildpack для сборки container-образов с ML моделями из MLflow Model Registry.

## Особенности

- **Unprivileged сборка** — работает в rootless режиме без специальных привилегий
- **MLServer runtime** — использует Seldon MLServer для инференса
- **Автоопределение flavor** — автоматически выбирает нужный MLServer extension
- **Быстрая установка зависимостей** — использует uv вместо pip
- **Multi-architecture** — поддерживает linux/amd64 и linux/arm64

## Поддерживаемые модели

| Flavor | Pip Package | Runtime |
|--------|-------------|---------|
| sklearn | mlserver-sklearn | mlserver_sklearn.SKLearnModel |
| xgboost | mlserver-xgboost | mlserver_xgboost.XGBoostModel |
| lightgbm | mlserver-lightgbm | mlserver_lightgbm.LightGBMModel |
| tensorflow | mlserver-tensorflow | mlserver_tensorflow.TensorFlowModel |
| pytorch | mlserver-torchserve | mlserver_torchserve.TorchServeModel |
| transformers | mlserver-huggingface | mlserver_huggingface.HuggingFaceModel |

## Быстрый старт

### Предварительные требования

| Инструмент | Версия | Назначение |
|------------|--------|------------|
| [pack](https://buildpacks.io/docs/tools/pack/) | >= 0.38.0 | CLI для работы с buildpacks |
| Docker или Podman | любой | Container runtime |
| Go | >= 1.24 | Для разработки buildpack |

### Установка

```bash
# macOS
brew install pack

# Linux
# pack: https://buildpacks.io/docs/tools/pack/
# Go: https://go.dev/doc/install
```

## Локальная сборка buildpack

```bash
# Клонировать репозиторий
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack

# Собрать builder (stack images + buildpack package + builder)
make builder

# Или пошагово:
make build     # Скомпилировать buildpack binaries (amd64 + arm64)
make test      # Запустить unit тесты
make stack     # Собрать stack images (multi-arch)
make package   # Упаковать buildpack
make builder   # Создать builder image
```

## Сборка образа с моделью

### Вариант 1: Локальная модель (e2e пример)

```bash
# Использовать тестовую модель из e2e
pack build my-sklearn-model:latest \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --path e2e/models/sklearn \
  --pull-policy never \
  --trust-builder

# Запустить
docker run --rm -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 my-sklearn-model:latest

# Тест инференса
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json
```

### Вариант 2: Модель из MLflow Registry

```bash
pack build my-model-image \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.example.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass" \
  --pull-policy never \
  --trust-builder
```

### Запуск и тест

```bash
# Проверить readiness
curl http://localhost:8080/v2/health/ready

# Произвольный инференс запрос
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

### E2E тестирование

```bash
# Полный e2e цикл (pyfunc + sklearn модели)
make e2e

# Или вручную
./e2e/scripts/verify-build.sh sklearn
./e2e/scripts/verify-runtime.sh sklearn
```

## Команды Makefile

```bash
make build     # Собрать buildpack binaries (amd64 + arm64)
make test      # Запустить unit тесты
make lint      # Запустить linter
make stack     # Собрать stack images (multi-arch)
make package   # Упаковать buildpack
make builder   # Создать builder (stack + package)
make e2e       # Build+runtime проверки e2e моделей
```

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `BP_MLFLOW_MODEL_NAME` | Имя модели в Registry | — |
| `BP_MLFLOW_MODEL_VERSION` | Версия модели | `latest` |
| `BP_MLFLOW_MODEL_STAGE` | Stage модели (Production, Staging) | — |
| `BP_MLFLOW_MODEL_PATH` | Локальный путь к модели ИЛИ `models:/<name>[/<version-or-stage>]` для Registry | auto-detect |

Если `BP_MLFLOW_MODEL_PATH` начинается с `models:/`, buildpack скачивает модель из Registry.
Иначе локальная модель определяется автоматически по `MLmodel` (корень или рекурсивный поиск).
Если найдено несколько `MLmodel`, укажите `BP_MLFLOW_MODEL_PATH`.

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
    └── secret_key
```

### Environment Variables (альтернатива bindings)

```bash
# MLflow Registry
export MLFLOW_TRACKING_URI="https://mlflow.example.com"
export MLFLOW_TRACKING_USERNAME="your-username"
export MLFLOW_TRACKING_PASSWORD="your-password"

# S3 (для артефактов)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# Сборка
pack build my-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/Production"
```

## Документация

- [USAGE.md](docs/USAGE.md) — подробное руководство пользователя
- [CONTRIBUTING.md](CONTRIBUTING.md) — руководство по разработке

## Лицензия

MIT

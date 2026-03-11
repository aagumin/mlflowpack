# MLflow Model Buildpack

CNCF Buildpack для сборки container-образов с ML моделями из MLflow Model Registry.

## Особенности

- **Unprivileged сборка** — работает в rootless режиме без специальных привилегий
- **MLServer runtime** — использует Seldon MLServer для инференса
- **Автоопределение flavor** — автоматически выбирает нужный MLServer extension
- **Быстрая установка зависимостей** — использует uv вместо pip
- **Поддержка macOS** — работает с Lima + Docker

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

- [pack](https://buildpacks.io/docs/tools/pack/) >= 0.38.0
- Docker или Podman
- Go >= 1.24 (для разработки)
- Lima (на macOS)

### Установка

```bash
# macOS
brew install pack lima

# Linux
# pack: https://buildpacks.io/docs/tools/pack/
```

### Локальная сборка

```bash
# Клонировать репозиторий
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack

# Собрать builder
make builder

# Прогнать локальные e2e проверки (pyfunc + sklearn модели)
make e2e

# Запустить
docker run -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 aipack-e2e-sklearn:local

# Тест инференса
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json
```

### Сборка с моделью из MLflow Registry

```bash
# Создать service bindings
mkdir -p bindings/mlflow
echo "mlflow" > bindings/mlflow/type
echo "https://mlflow.example.com" > bindings/mlflow/tracking_uri

# Собрать
lima pack build my-model \
  --builder aagumin/mlserver-builder:<version> \
  --env BP_MLFLOW_MODEL_NAME=my-classifier \
  --env BP_MLFLOW_MODEL_VERSION=latest \
  --volume ./bindings:/bindings/mlflow \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder
```

## Команды Makefile

```bash
make build     # Собрать buildpack binaries
make test      # Запустить unit тесты
make lint      # Запустить linter
make stack     # Собрать stack images
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

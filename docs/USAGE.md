# MLflow Model Buildpack: Руководство пользователя

Полное руководство по использованию buildpack для упаковки ML моделей в контейнерные образы.

## Содержание

1. [Установка](#установка)
2. [Базовое использование](#базовое-использование)
3. [Сборка с MLflow Registry](#сборка-с-mlflow-registry)
4. [Конфигурация](#конфигурация)
5. [Инференс](#инференс)
6. [Troubleshooting](#troubleshooting)

---

## Установка

### Предварительные требования

| Инструмент | Версия | Назначение |
|------------|--------|------------|
| pack | >= 0.30.0 | CLI для работы с buildpacks |
| Docker или Podman | любой | Container runtime |
| Lima | любой | Docker на macOS (опционально) |

### Установка pack

**macOS:**
```bash
brew install pack
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y pack

# Или скачать бинарник
(curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.37.0/pack-v0.37.0-linux-$(uname -m | sed 's/x86_64/amd64/').tgz" | sudo tar -C /usr/local/bin/ --no-same-owner -xz pack)
```

### Lima на macOS (Опционально)

```bash
# Установка Lima
brew install lima

# Создать VM с Docker
limactl start template://docker

# Использовать lima prefix для docker и pack
lima docker ps
lima pack --version
```

### Получение builder

**Вариант 1: Собрать локально**
```bash
git clone https://github.com/amazme/aipack.git
cd aipack
make builder
```

**Вариант 2: Использовать готовый (когда опубликован)**
```bash
pack builder pull amazme/mlserver-builder:0.1.0
```

---

## Базовое использование

### Сценарий 1: Локальная модель

Если у вас есть папка с MLflow моделью (с файлом `MLmodel`):

```
my-model/
├── MLmodel
├── model.pkl
├── conda.yaml
└── requirements.txt
```

**macOS с Lima:**
```bash
lima pack build my-model-image \
  --builder amazme/mlserver-builder:0.1.0 \
  --path my-model \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder
```

**Linux:**
```bash
pack build my-model-image \
  --builder amazme/mlserver-builder:0.1.0 \
  --path my-model
```

### Сценарий 2: Использование тестовой модели

В репозитории есть готовая тестовая модель в папке `test-model/`:

```bash
# Сборка (macOS с Lima)
lima pack build test-mlflow-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --path test-model \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder

# Запуск
docker run --rm -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 test-mlflow-model:latest

# Тест инференса
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

---

## Сборка с MLflow Registry

### Настройка Service Bindings

Service Bindings — механизм передачи credentials в buildpack.

```bash
# Создать структуру директорий
mkdir -p bindings/mlflow/s3

# MLflow credentials
echo "mlflow" > bindings/mlflow/type
echo "https://mlflow.your-company.com" > bindings/mlflow/tracking_uri
echo "your-username" > bindings/mlflow/username
echo "your-password" > bindings/mlflow/password

# S3 credentials (если артефакты хранятся в S3)
echo "https://s3.your-company.com" > bindings/mlflow/s3/endpoint
echo "your-access-key" > bindings/mlflow/s3/access_key
echo "your-secret-key" > bindings/mlflow/s3/secret_key
echo "us-east-1" > bindings/mlflow/s3/region
```

### Переменные окружения

```bash
export BP_MLFLOW_MODEL_NAME="my-classifier"
export BP_MLFLOW_MODEL_VERSION="3"  # или "latest"
# ИЛИ по stage
export BP_MLFLOW_MODEL_STAGE="Production"
```

### Полная команда сборки

```bash
# macOS с Lima
lima pack build my-registry-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_NAME=my-classifier \
  --env BP_MLFLOW_MODEL_VERSION=latest \
  --volume $(pwd)/bindings:/bindings/mlflow \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder

# Linux
pack build my-registry-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_NAME=my-classifier \
  --env BP_MLFLOW_MODEL_VERSION=latest \
  --volume $(pwd)/bindings:/bindings/mlflow
```

---

## Конфигурация

### Переменные окружения buildpack

| Переменная | Обязательно | Описание |
|------------|-------------|----------|
| `BP_MLFLOW_MODEL_NAME` | Да* | Имя зарегистрированной модели |
| `BP_MLFLOW_MODEL_VERSION` | Нет | Версия модели (по умолчанию `latest`) |
| `BP_MLFLOW_MODEL_STAGE` | Нет | Stage модели: `Production`, `Staging`, `Archived` |

*\* Обязательно только при сборке из registry. При локальной модели определяется автоматически.*

### Service Bindings структура

```
/bindings/mlflow/
├── type              # "mlflow" (обязательно)
├── tracking_uri      # URL MLflow сервера (обязательно)
├── username          # Username для Basic Auth (опционально)
├── password          # Password для Basic Auth (опционально)
└── s3/               # S3-compatible storage (опционально)
    ├── endpoint      # S3 endpoint URL
    ├── access_key    # AWS_ACCESS_KEY_ID
    ├── secret_key    # AWS_SECRET_ACCESS_KEY
    └── region        # AWS region (default: us-east-1)
```

### Конфигурация модели (conda.yaml)

Buildpack читает `conda.yaml` из модели для определения:

- Версии Python
- Зависимостей pip

Пример:
```yaml
channels:
  - defaults
  - conda-forge
dependencies:
  - python=3.11
  - pip:
    - scikit-learn==1.3.0
    - pandas>=2.0.0
    - numpy>=1.24.0
```

Если `conda.yaml` отсутствует, используется Python 3.10 по умолчанию.

---

## Инференс

### HTTP API (V2 Protocol)

MLServer реализует [V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md).

**Проверка readiness:**
```bash
curl http://localhost:8080/v2/health/ready
```

**Информация о модели:**
```bash
curl http://localhost:8080/v2/models/model
```

**Инференс:**
```bash
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-1",
    "inputs": [{
      "name": "input-0",
      "shape": [1, 4],
      "datatype": "FP32",
      "data": [[5.1, 3.5, 1.4, 0.2]]
    }]
  }'
```

**Ответ:**
```json
{
  "id": "test-1",
  "model_name": "model",
  "outputs": [{
    "name": "predict",
    "shape": [1, 1],
    "datatype": "INT64",
    "data": [0]
  }]
}
```

### gRPC API

```bash
# Установить grpcurl
brew install grpcurl  # macOS

# Список сервисов
grpcurl -plaintext localhost:9080 list

# Инференс
grpcurl -plaintext \
  -d '{"model_name": "model", "id": "test-1", "inputs": [{"name": "input-0", "shape": [1, 4], "datatype": "FP32", "contents": {"fp32_contents": [5.1, 3.5, 1.4, 0.2]}}]}' \
  localhost:9080 \
  inference.GRPCInferenceService/ModelInfer
```

### Запуск в Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mlflow-model
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mlflow-model
  template:
    metadata:
      labels:
        app: mlflow-model
    spec:
      containers:
        - name: model
          image: registry.example.com/models/sklearn:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 9080
              name: grpc
          env:
            - name: MLSERVER_PARALLEL_WORKERS
              value: "0"
          resources:
            requests:
              memory: "512Mi"
              cpu: "250m"
            limits:
              memory: "2Gi"
              cpu: "1"
---
apiVersion: v1
kind: Service
metadata:
  name: mlflow-model
spec:
  selector:
    app: mlflow-model
  ports:
    - port: 8080
      name: http
    - port: 9080
      name: grpc
```

---

## Troubleshooting

### Buildpack не обнаруживает модель

**Симптом:** `Detect: fail`

**Решение:**
- Убедитесь, что файл `MLmodel` существует в корне `--path`
- Или установите переменную `BP_MLFLOW_MODEL_NAME`

### Ошибка подключения к MLflow

**Симптом:** `reading MLflow binding: MLflow binding not found`

**Решение:**
- Проверьте путь к bindings: `--volume $(pwd)/bindings:/bindings/mlflow`
- Убедитесь, что файл `type` содержит `mlflow`
- Проверьте содержимое: `cat bindings/mlflow/type`

### Ошибка скачивания артефактов

**Симптом:** `downloading model: ...`

**Решение:**
- Проверьте S3 credentials в `bindings/mlflow/s3/`
- Убедитесь, что endpoint правильный (с `https://`)
- Проверьте доступ к bucket

### Python версия не найдена

**Симптом:** `installing Python: version X.Y not found`

**Решение:**
- Используйте поддерживаемую версию Python (3.9-3.12)
- Обновите `conda.yaml` в модели

### Включение debug логов

```bash
lima pack build my-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --path my-model \
  --env CNB_LOG_LEVEL=debug \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder \
  -v
```

### Проверка содержимого образа

```bash
# Запустить shell в образе
docker run --rm -it --entrypoint /bin/bash my-model

# Посмотреть слои
ls -la /layers/

# Проверить переменные окружения
env | grep MLSERVER
env | grep PYTHON
```

---

## Ссылки

- [Cloud Native Buildpacks](https://buildpacks.io/)
- [MLServer Documentation](https://mlserver.readthedocs.io/)
- [MLflow Documentation](https://mlflow.org/docs/latest/index.html)
- [V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)

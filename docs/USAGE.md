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
| pack | >= 0.38.0 | CLI для работы с buildpacks |
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
(curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.38.2/pack-v0.38.2-linux-$(uname -m | sed 's/x86_64/amd64/').tgz" | sudo tar -C /usr/local/bin/ --no-same-owner -xz pack)
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
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack
make builder
```

**Вариант 2: Использовать готовый (когда опубликован)**
```bash
pack builder pull aagumin/mlserver-builder:0.1.0
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
  --builder aagumin/mlserver-builder:0.1.0 \
  --path my-model \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder
```

**Linux:**
```bash
pack build my-model-image \
  --builder aagumin/mlserver-builder:0.1.0 \
  --path my-model
```

### Сценарий 2: Полный e2e цикл на тестовых моделях (pyfunc + sklearn)

В репозитории есть две готовые MLflow модели:
- `e2e/models/pyfunc` (`python_function`)
- `e2e/models/sklearn` (`sklearn`)

Их можно прогонять полностью через e2e-скрипты.

#### Вариант A: полный цикл одной командой

```bash
# 1) Собрать локальный builder
make builder

# 2) Для обеих моделей выполнить:
#    - упаковку образа через buildpack
#    - запуск контейнера
#    - проверку readiness (/v2/health/ready)
#    - inference запрос
#    - сравнение ответа с expected-response.json
make e2e
```

#### Вариант B: полный цикл вручную (пример sklearn)

```bash
# 1) Упаковать образ из e2e модели
./e2e/scripts/verify-build.sh sklearn

# 2) Запустить контейнер
docker run --rm --name e2e-sklearn -p 8080:8080 \
  -e MLSERVER_PARALLEL_WORKERS=0 \
  aipack-e2e-sklearn:local
```

Во втором терминале:

```bash
# 3) Проверить readiness
curl -fsS http://localhost:8080/v2/health/ready

# 4) Отправить inference request из тестового файла
curl -fsS -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json

# 5) Быстро посмотреть предсказания (ожидается [0, 1])
curl -fsS -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json | \
python3 -c 'import json,sys; print(json.load(sys.stdin)["outputs"][0]["data"])'
```

#### Вариант C: полный цикл вручную (пример pyfunc)

```bash
# Build + run + readiness + infer + проверка expected-response.json
./e2e/scripts/verify-runtime.sh pyfunc

# Ожидаемый inference output_data: [4.0, 5.0]
```

---

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
# macOS с Lima
lima pack build my-registry-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.company.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass" \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder

# Linux
pack build my-registry-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.company.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass"
```

---

## Конфигурация

### Переменные окружения buildpack

| Переменная | Обязательно | Описание |
|------------|-------------|----------|
| `BP_MLFLOW_MODEL_PATH` | Да* | Путь к модели: локальный путь ИЛИ `models:/<name>[/<version-or-stage>]` |

*\* Обязательно только при сборке из registry. При локальной модели определяется автоматически.*

Локальная модель теперь определяется так:
- если `BP_MLFLOW_MODEL_PATH` начинается с `models:/`, buildpack использует Model Registry (локальная файловая система не сканируется),
- иначе сначала `BP_MLFLOW_MODEL_PATH` (если задан),
- затем `MLmodel` в корне `--path`,
- затем рекурсивный поиск единственного `MLmodel` под `--path`.

Если найдено несколько `MLmodel`, buildpack завершится ошибкой неоднозначности и попросит указать `BP_MLFLOW_MODEL_PATH`.

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
Если рядом есть `requirements.txt`, buildpack дополнительно установит зависимости из него (fallback режим).

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
  --builder aagumin/mlserver-builder:0.1.0 \
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

lima pack build my-registry-model \
  --env BP_MLFLOW_MODEL_PATH="models:/bert-base-cased-squad/1" \
  --env DATABRICKS_HOST="asdasd" \
  --env DATABRICKS_USERNAME="mladmin" \
  --env DATABRICKS_PASSWORD="asdasd" \
  --builder aagumin/mlserver-builder:d3c57c8 \
  --env AWS_ACCESS_KEY_ID="asdas" \
  --env AWS_SECRET_ACCESS_KEY="sadasd" \
  --env AWS_ENDPOINT_URL="" \
  --env AWS_SIGNATURE_VERSION="s3" \
  --env AWS_ADDRESSING_STYLE="virtual" \
  --env AWS_REGION="ru-moscow-1" \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder

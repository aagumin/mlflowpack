# GitLab Auto DevOps Integration

Инструкция по интеграции MLflow Buildpack с GitLab Auto DevOps.

## Обзор

GitLab Auto DevOps автоматически собирает, тестирует и деплоит приложения. Для ML моделей можно использовать Cloud Native Buildpacks с кастомным builder'ом.

## Предварительные требования

1. GitLab instance (GitLab.com или self-managed)
2. Опубликованный builder image в container registry
3. Доступ к MLflow Model Registry
4. Kubernetes кластер (для Auto Deploy)

---

## Шаг 1: Опубликовать Builder

### Вариант A: GitLab Container Registry

```bash
# Собрать builder
make builder

# Тегировать для GitLab Registry
docker tag aagumin/mlserver-builder:0.1.0 registry.gitlab.com/YOUR_GROUP/YOUR_PROJECT/mlserver-builder:0.1.0

# Запушить
docker push registry.gitlab.com/YOUR_GROUP/YOUR_PROJECT/mlserver-builder:0.1.0
```

### Вариант B: Docker Hub

```bash
# Тегировать
docker tag aagumin/mlserver-builder:0.1.0 YOUR_DOCKERHUB_USER/mlserver-builder:0.1.0

# Запушить
docker push YOUR_DOCKERHUB_USER/mlserver-builder:0.1.0
```

---

## Шаг 2: Настройка GitLab Project

### 2.1 Включить Auto DevOps

1. Откройте **Settings > CI/CD > Auto DevOps**
2. Включите **Default to Auto DevOps pipeline**
3. Выберите стратегию деплоя (Kubernetes или ECS)

### 2.2 Настроить CI/CD Variables

Откройте **Settings > CI/CD > Variables** и добавьте:

#### Builder Configuration

| Variable | Value | Protected | Masked |
|----------|-------|-----------|--------|
| `AUTO_DEVOPS_BUILD_IMAGE_CNB_BUILDER` | `YOUR_DOCKERHUB_USER/mlserver-builder:0.1.0` | No | No |

#### MLflow Registry (вариант с env vars)

| Variable | Value | Protected | Masked |
|----------|-------|-----------|--------|
| `MLFLOW_TRACKING_URI` | `https://mlflow.your-company.com` | No | No |
| `MLFLOW_TRACKING_USERNAME` | `your-username` | No | No |
| `MLFLOW_TRACKING_PASSWORD` | `your-password` | Yes | Yes |
| `AWS_ACCESS_KEY_ID` | `your-access-key` | No | No |
| `AWS_SECRET_ACCESS_KEY` | `your-secret-key` | Yes | Yes |
| `AWS_REGION` | `us-east-1` | No | No |
| `AWS_ENDPOINT_URL` | `https://s3.your-company.com` | No | No |

#### Model Selection

| Variable | Value | Protected | Masked |
|----------|-------|-----------|--------|
| `BP_MLFLOW_MODEL_PATH` | `models:/my-model/production` | No | No |

---

## Шаг 3: Минимальный .gitlab-ci.yml

Если Auto DevOps включён, можно использовать минимальную конфигурацию:

```yaml
# .gitlab-ci.yml
include:
  - template: Auto-DevOps.gitlab-ci.yml

# Переопределение переменных
variables:
  AUTO_DEVOPS_BUILD_IMAGE_CNB_BUILDER: "your-registry/mlserver-builder:0.1.0"
  BP_MLFLOW_MODEL_PATH: "models:/my-model/production"
  # MLflow credentials передаются через CI/CD Variables в UI
```

---

## Шаг 4: Полный .gitlab-ci.yml для MLflow

Для большего контроля используйте кастомный пайплайн:

```yaml
# .gitlab-ci.yml
stages:
  - build
  - test
  - deploy

variables:
  # Builder
  BUILDER_IMAGE: "your-registry/mlserver-builder:0.1.0"

  # Model
  BP_MLFLOW_MODEL_PATH: "models:/my-model/production"

  # MLflow credentials (переопределяются в Settings > CI/CD > Variables)
  # MLFLOW_TRACKING_URI: ""
  # MLFLOW_TRACKING_USERNAME: ""
  # MLFLOW_TRACKING_PASSWORD: ""
  # AWS_ACCESS_KEY_ID: ""
  # AWS_SECRET_ACCESS_KEY: ""

# ============================================================================
# BUILD STAGE
# ============================================================================

build_model_image:
  stage: build
  image: docker:24
  services:
    - docker:24-dind
  variables:
    DOCKER_TLS_CERTDIR: ""
    # Передаём переменные в buildpack
    PACK_VARS: >
      --env BP_MLFLOW_MODEL_PATH=${BP_MLFLOW_MODEL_PATH}
      --env MLFLOW_TRACKING_URI=${MLFLOW_TRACKING_URI}
      --env MLFLOW_TRACKING_USERNAME=${MLFLOW_TRACKING_USERNAME}
      --env MLFLOW_TRACKING_PASSWORD=${MLFLOW_TRACKING_PASSWORD}
      --env AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      --env AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      --env AWS_REGION=${AWS_REGION}
  before_script:
    - apk add --no-cache pack
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
  script:
    # Создаём пустую директорию для pack (модель скачается из registry)
    - mkdir -p empty-context

    # Собираем образ с pack
    - |
      pack build ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} \
        --builder ${BUILDER_IMAGE} \
        --path empty-context \
        --trust-builder \
        ${PACK_VARS}

    # Пушим в registry
    - docker push ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA}

    # Тегируем latest для default branch
    - |
      if [ "$CI_COMMIT_BRANCH" == "$CI_DEFAULT_BRANCH" ]; then
        docker tag ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA} ${CI_REGISTRY_IMAGE}:latest
        docker push ${CI_REGISTRY_IMAGE}:latest
      fi
  only:
    - main
    - production
    - merge_requests

# ============================================================================
# TEST STAGE
# ============================================================================

test_model_endpoint:
  stage: test
  image: curlimages/curl:latest
  services:
    - name: ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA}
      alias: model-server
  variables:
    # Передаём credentials для pull image
    CI_SERVICE_PULL_POLICY: always
  script:
    # Ждём запуска сервера
    - sleep 30

    # Проверяем health
    - curl -f http://model-server:8080/v2/health/ready

    # Тестируем inference
    - |
      curl -X POST http://model-server:8080/v2/models/model/infer \
        -H "Content-Type: application/json" \
        -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
  only:
    - merge_requests

# ============================================================================
# DEPLOY STAGE
# ============================================================================

deploy_staging:
  stage: deploy
  image: bitnami/kubectl:latest
  variables:
    KUBE_NAMESPACE: "ml-models-staging"
  script:
    - kubectl config set-cluster k8s --server="${KUBE_URL}" --insecure-skip-tls-verify=true
    - kubectl config set-credentials gitlab --token="${KUBE_TOKEN}"
    - kubectl config set-context default --cluster=k8s --user=gitlab --namespace=${KUBE_NAMESPACE}
    - kubectl config use-context default

    # Деплой
    - |
      cat <<EOF | kubectl apply -f -
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${CI_PROJECT_NAME}
        namespace: ${KUBE_NAMESPACE}
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: ${CI_PROJECT_NAME}
        template:
          metadata:
            labels:
              app: ${CI_PROJECT_NAME}
          spec:
            containers:
              - name: model
                image: ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA}
                ports:
                  - containerPort: 8080
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
      EOF
  environment:
    name: staging
    url: https://staging.your-domain.com/${CI_PROJECT_NAME}
  only:
    - main

deploy_production:
  stage: deploy
  image: bitnami/kubectl:latest
  variables:
    KUBE_NAMESPACE: "ml-models-production"
  script:
    - kubectl config set-cluster k8s --server="${KUBE_URL}" --insecure-skip-tls-verify=true
    - kubectl config set-credentials gitlab --token="${KUBE_TOKEN}"
    - kubectl config set-context default --cluster=k8s --user=gitlab --namespace=${KUBE_NAMESPACE}
    - kubectl config use-context default

    # Деплой с большим количеством реплик
    - |
      cat <<EOF | kubectl apply -f -
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${CI_PROJECT_NAME}
        namespace: ${KUBE_NAMESPACE}
      spec:
        replicas: 3
        selector:
          matchLabels:
            app: ${CI_PROJECT_NAME}
        template:
          metadata:
            labels:
              app: ${CI_PROJECT_NAME}
          spec:
            containers:
              - name: model
                image: ${CI_REGISTRY_IMAGE}:${CI_COMMIT_SHA}
                ports:
                  - containerPort: 8080
                env:
                  - name: MLSERVER_PARALLEL_WORKERS
                    value: "0"
                resources:
                  requests:
                    memory: "1Gi"
                    cpu: "500m"
                  limits:
                    memory: "4Gi"
                    cpu: "2"
                livenessProbe:
                  httpGet:
                    path: /v2/health/live
                    port: 8080
                  initialDelaySeconds: 30
                  periodSeconds: 10
                readinessProbe:
                  httpGet:
                    path: /v2/health/ready
                    port: 8080
                  initialDelaySeconds: 30
                  periodSeconds: 10
      EOF
  environment:
    name: production
    url: https://api.your-domain.com/${CI_PROJECT_NAME}
  only:
    - production
  when: manual
```

---

## Шаг 5: Использование Service Bindings (Kubernetes)

Для production рекомендуется использовать Kubernetes Service Bindings вместо env vars:

### 5.1 Создать Secret

```yaml
# mlflow-binding.yaml
apiVersion: v1
kind: Secret
metadata:
  name: mlflow-credentials
  namespace: ml-models-production
type: Opaque
stringData:
  type: mlflow
  tracking_uri: "https://mlflow.your-company.com"
  username: "your-username"
  password: "your-password"
---
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
  namespace: ml-models-production
type: Opaque
stringData:
  type: s3
  endpoint: "https://s3.your-company.com"
  access_key: "your-access-key"
  secret_key: "your-secret-key"
  region: "us-east-1"
```

### 5.2 Смонтировать в Deployment

```yaml
spec:
  volumes:
    - name: bindings
      projected:
        sources:
          - secret:
              name: mlflow-credentials
              items:
                - key: type
                  path: mlflow/type
                - key: tracking_uri
                  path: mlflow/tracking_uri
                - key: username
                  path: mlflow/username
                - key: password
                  path: mlflow/password
          - secret:
              name: s3-credentials
              items:
                - key: type
                  path: mlflow/s3/type
                - key: endpoint
                  path: mlflow/s3/endpoint
                - key: access_key
                  path: mlflow/s3/access_key
                - key: secret_key
                  path: mlflow/s3/secret_key
                - key: region
                  path: mlflow/s3/region
  containers:
    - name: model
      volumeMounts:
        - name: bindings
          mountPath: /bindings
          readOnly: true
```

---

## Переменные GitLab CI/CD

### Обязательные

| Variable | Description |
|----------|-------------|
| `AUTO_DEVOPS_BUILD_IMAGE_CNB_BUILDER` | URL вашего builder image |
| `BP_MLFLOW_MODEL_PATH` | `models:/model-name/version` |
| `MLFLOW_TRACKING_URI` | URL MLflow сервера |
| `AWS_ACCESS_KEY_ID` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key |

### Опциональные

| Variable | Description | Default |
|----------|-------------|---------|
| `MLFLOW_TRACKING_USERNAME` | Basic auth username | - |
| `MLFLOW_TRACKING_PASSWORD` | Basic auth password | - |
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_ENDPOINT_URL` | Custom S3 endpoint | - |

---

## Troubleshooting

### Build failed: builder not found

```bash
# Проверьте доступность builder
docker pull your-registry/mlserver-builder:0.1.0
```

### Build failed: MLflow credentials not found

```bash
# Проверьте переменные в GitLab
# Settings > CI/CD > Variables
```

### Model download failed: S3 access denied

```bash
# Проверьте S3 credentials
aws s3 ls --endpoint-url https://s3.your-company.com
```

### Container fails to start

```bash
# Проверьте логи
kubectl logs -n ml-models-staging deployment/your-model

# Проверьте health endpoint
kubectl port-forward -n ml-models-staging deployment/your-model 8080:8080
curl http://localhost:8080/v2/health/ready
```

---

## Ссылки

- [GitLab Auto DevOps Documentation](https://docs.gitlab.com/ee/topics/autodevops/)
- [Cloud Native Buildpacks](https://buildpacks.io/)
- [MLServer Documentation](https://mlserver.readthedocs.io/)

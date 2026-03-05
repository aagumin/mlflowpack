# Contributing to MLflow Model Buildpack

Спасибо за интерес к проекту! Этот документ описывает процесс разработки и внесения изменений.

## Соглашения

### Коммиты

Проект следует [Conventional Commits v1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).

Формат сообщения коммита:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Типы:**
- `feat` — новая функциональность
- `fix` — исправление бага
- `docs` — изменения в документации
- `style` — форматирование, пропущенные точки с запятой
- `refactor` — рефакторинг без изменения поведения
- `test` — добавление или исправление тестов
- `chore` — изменения в сборке, инструментах

**Примеры:**

```bash
feat(mlflow): add support for LightGBM flavor
fix(python): correct Python binary lookup in uv installation
docs(readme): update installation instructions
refactor(build): simplify layer creation logic
```

### Go Code Style

Проект следует [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

Ключевые принципы:
- Используйте `gofmt` и `goimports`
- Избегайте неинформативных имён (data, info, thing)
- Возвращайте интерфейсы, принимайте конкретные типы
- Предпочитайте каналы вместо мьютексов для коммуникации
- Используйте контекст для отмены операций

```bash
# Установка линтеров
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Проверка
make lint
```

## Разработка

### Предварительные требования

- **Go** >= 1.21
- **Docker** или **Podman**
- **pack** CLI >= 0.30.0
- **Make** (опционально, но рекомендуется)

### Установка инструментов

```bash
# macOS
brew install go pack

# Linux
# Go: https://go.dev/doc/install
# pack: https://buildpacks.io/docs/tools/pack/
```

### Клонирование и сборка

```bash
git clone https://github.com/amazme/aipack.git
cd aipack

# Установить Go зависимости
cd buildpack && go mod download

# Собрать binaries
make build
```

### Структура проекта

```
aipack/
├── buildpack/                   # CNB Buildpack
│   ├── cmd/
│   │   ├── detect/main.go       # Точка входа detect phase
│   │   └── build/main.go        # Точка входа build phase
│   ├── internal/
│   │   ├── cnb/                 # CNB API типы
│   │   ├── detect/detector.go   # Логика детекции
│   │   ├── build/builder.go     # Основная логика сборки
│   │   ├── mlflow/              # MLflow client и парсеры
│   │   ├── conda/parser.go      # Парсер conda.yaml
│   │   ├── python/installer.go  # Установка Python через uv
│   │   └── layer/layers.go      # Управление слоями
│   ├── buildpack.toml
│   └── go.mod
├── stack/
│   ├── build/Dockerfile         # Build image
│   └── run/Dockerfile           # Run image
├── test-model/                  # Тестовая модель для локальной проверки
├── docs/
│   └── plans/                   # Дизайн документы
├── Makefile
└── builder.toml
```

### Сборка и тестирование

```bash
# Сборка buildpack
make build

# Запуск unit тестов
make test

# Линтинг
make lint

# Полный цикл: stack + package + builder
make builder
```

### Локальное тестирование с моделью

```bash
# Сборка тестового образа
lima pack build test-mlflow-model \
  --builder amazme/mlserver-builder:<version> \
  --path test-model \
  --pull-policy never \
  --docker-host=inherit \
  --trust-builder

# Запуск контейнера
docker run --rm -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 test-mlflow-model:latest

# Тестирование предсказания
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

### Добавление нового MLflow flavor

1. Добавьте маппинг в `buildpack/internal/mlflow/flavor.go`:

   ```go
   var MLServerExtensions = map[string]MLServerExtension{
       // ...
       "newflavor": {
           PipPackage: "mlserver-newflavor",
           Runtime:    "mlserver_newflavor.NewFlavorModel",
       },
   }
   ```

2. Обновите приоритет в `GetPrimaryFlavor()`

3. Обновите документацию (README.md, docs/USAGE.md)

### Отладка

Включить debug логирование:

```bash
pack build my-model \
  --builder amazme/mlserver-builder:0.1.0 \
  --path test-model \
  --env CNB_LOG_LEVEL=debug
```

Проверить содержимое слоёв:

```bash
# Создать контейнер без запуска
docker create --name debug my-model

# Скопировать слои
docker cp debug:/layers ./layers-debug

# Посмотреть структуру
find layers-debug -type f | head -50
```

## CI/CD

Проект использует GitHub Actions для:
- Запуска тестов при PR
- Сборки и публикации образов при релизе
- Линтинга кода

## Вопросы и проблемы

- **Баги**: Создайте issue с описанием проблемы и шагами воспроизведения
- **Feature requests**: Создайте issue с описанием желаемой функциональности
- **Вопросы**: Создайте discussion или issue с меткой `question`

## Лицензия

Внося изменения в проект, вы соглашаетесь, что ваш код будет распространяться под лицензией MIT.

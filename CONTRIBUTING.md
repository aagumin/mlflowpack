# Contributing to MLflow Model Buildpack

Спасибо за интерес к проекту! Этот документ описывает процесс разработки и внесения изменений.

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
buildpack/
├── cmd/
│   ├── detect/main.go      # Точка входа detect phase
│   └── build/main.go       # Точка входа build phase
├── internal/
│   ├── detect/
│   │   └── detector.go     # Логика детекции
│   ├── build/
│   │   └── builder.go      # Основная логика сборки
│   ├── mlflow/
│   │   ├── client.go       # MLflow API клиент
│   │   ├── model.go        # Работа с моделью
│   │   ├── flavor.go       # Определение flavor
│   │   └── storage/
│   │       └── s3.go       # S3 backend
│   ├── conda/
│   │   └── parser.go       # Парсер conda.yaml
│   ├── python/
│   │   └── installer.go    # Установка Python через uv
│   ├── bindings/
│   │   └── bindings.go     # Service bindings
│   └── layer/
│       └── layers.go       # Утилиты для слоёв
├── buildpack.toml          # Метаданные buildpack
├── package.toml            # Конфигурация упаковки
└── go.mod
```

### Запуск тестов

```bash
# Unit тесты
make test
# или
cd buildpack && go test -v -race ./...

# Тесты конкретного пакета
cd buildpack && go test -v ./internal/mlflow/...

# Lint
make lint
# или
cd buildpack && golangci-lint run ./...
```

### Стиль кода

- Используйте `gofmt` для форматирования
- Следуйте [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Документируйте экспортируемые функции и типы
- Пишите тесты для новой функциональности

### Создание изменений

1. Создайте ветку для feature/fix:
   ```bash
   git checkout -b feature/my-feature
   ```

2. Внесите изменения и добавьте тесты

3. Убедитесь что тесты проходят:
   ```bash
   make test lint
   ```

4. Сделайте коммит:
   ```bash
   git commit -m "feat: add support for new model flavor"
   ```

   Используйте [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` — новая функциональность
   - `fix:` — исправление бага
   - `docs:` — документация
   - `test:` — тесты
   - `refactor:` — рефакторинг
   - `chore:` — обслуживание

### Тестирование изменений

Для полноценного тестирования нужно пересобрать builder:

```bash
# Пересобрать всё
make clean build stack package builder

# Тестовая сборка модели
make test-build

# Запустить образ
docker run -p 8080:8080 test-mlflow-model

# Проверить инференс
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

### Добавление поддержки нового flavor

1. Добавьте маппинг в `internal/mlflow/flavor.go`:
   ```go
   var MLServerExtensions = map[string]MLServerExtension{
       // ... существующие
       "new_flavor": {
           PipPackage: "mlserver-newflavor",
           Runtime:    "mlserver_newflavor.NewFlavorRuntime",
       },
   }
   ```

2. Обновите приоритет в `GetPrimaryFlavor()`

3. Добавьте тесты

4. Обновите документацию (README.md, docs/USAGE.md)

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

### CI/CD

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

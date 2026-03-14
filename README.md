# API Gateway Control Plane

Система управления API Gateway на основе Envoy с поддержкой мультитенантности и множественных окружений.

## Архитектура

Проект построен на принципах Clean Architecture с четким разделением слоев:

``` text
├── cmd/                    # Точки входа приложения
├── internal/
│   ├── config/            # Конфигурация
│   ├── container/         # DI контейнер
│   ├── database/          # Работа с БД
│   ├── delivery/          # Слой доставки (HTTP/gRPC)
│   │   ├── grpc/         # gRPC handlers
│   │   └── http/         # REST API handlers
│   ├── domain/           # Доменная логика
│   │   ├── interfaces/   # Интерфейсы
│   │   └── models/       # Доменные модели
│   ├── repository/       # Слой данных (генерируется sqlc)
│   └── usecase/          # Бизнес-логика
├── api/proto/            # Protobuf определения
└── pkg/                  # Сгенерированный код
```

## Основные сущности

### Tenant (Тенант)

Представляет кластер (dev, prod) - высший уровень изоляции.

### Environment (Окружение)

Представляет окружение внутри тенанта (dev, preprod, stage, test).

### Listener (Слушатель)

Представляет конфигурацию Envoy listener для конкретного окружения.

## API

### REST API

Сервис предоставляет REST API на порту `:8080`:

#### Тенанты

- `POST /api/v1/tenants` - Создать тенант
- `GET /api/v1/tenants` - Получить список тенантов
- `GET /api/v1/tenants/{id}` - Получить тенант по ID
- `GET /api/v1/tenants/by-name?name={name}` - Получить тенант по имени
- `PUT /api/v1/tenants/{id}` - Обновить тенант
- `DELETE /api/v1/tenants/{id}` - Удалить тенант

#### Окружения

- `POST /api/v1/environments` - Создать окружение
- `GET /api/v1/environments` - Получить список окружений
- `GET /api/v1/environments/{id}` - Получить окружение по ID
- `GET /api/v1/environments/by-name?name={name}` - Получить окружение по имени
- `PUT /api/v1/environments/{id}` - Обновить окружение
- `DELETE /api/v1/environments/{id}` - Удалить окружение
- `GET /api/v1/tenants/{tenant_id}/environments` - Получить окружения тенанта

#### Слушатели

- `POST /api/v1/listeners` - Создать слушатель
- `GET /api/v1/listeners` - Получить список слушателей
- `GET /api/v1/listeners/{id}` - Получить слушатель по ID
- `GET /api/v1/listeners/by-name?name={name}` - Получить слушатель по имени
- `PUT /api/v1/listeners/{id}` - Обновить слушатель
- `DELETE /api/v1/listeners/{id}` - Удалить слушатель
- `GET /api/v1/environments/{environment_id}/listeners` - Получить слушатели окружения

### gRPC API

Сервис также предоставляет gRPC API на порту `:9090` с аналогичным функционалом.

## Установка и запуск

### Требования

- Go 1.21+
- PostgreSQL 14+
- Docker и Docker Compose (опционально)

### Локальная разработка

1. Клонируйте репозиторий
2. Установите зависимости:

   ```bash
   make deps
   ```

3. Запустите PostgreSQL (через Docker):

   ```bash
   make docker-up
   ```

4. Запустите приложение:

   ```bash
   make run
   ```

### Команды разработки

```bash
# Запуск с hot reload
make dev

# Генерация кода из protobuf
make proto-generate

# Генерация кода из SQL (sqlc)
make sqlc-generate

# Форматирование кода
make fmt

# Линтинг
make lint

# Тесты
make test

# Тесты с покрытием
make test-coverage
```

## Конфигурация

Конфигурация загружается из файла, путь к которому указывается в переменной окружения `CONFIG_PATH`.

Пример конфигурации:

```yaml
database:
  postgres:
    host: localhost
    port: 5432
    user: postgres
    password: postgres
    dbname: postgres
    sslmode: disable

server:
  http_port: 8080
  grpc_port: 9090
```

## База данных

Проект использует PostgreSQL с автоматическими миграциями. Схема БД включает:

- `tenants` - тенанты
- `environments` - окружения  
- `listeners` - слушатели
- `tenants_environments` - связи тенантов и окружений
- `listeners_environments` - связи слушателей и окружений

## Примеры использования

### Создание тенанта

```bash
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{"name": "production"}'
```

### Создание окружения

```bash
curl -X POST http://localhost:8080/api/v1/environments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod",
    "tenant_id": "tenant-uuid-here",
    "config": {"port": 8080, "ssl": true}
  }'
```

### Создание слушателя

```bash
curl -X POST http://localhost:8080/api/v1/listeners \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-listener",
    "environment_id": "environment-uuid-here",
    "config": {
      "address": "0.0.0.0:8080",
      "filter_chains": []
    }
  }'
```

## Планы развития

- [ ] Интеграция с Envoy xDS API
- [ ] Web UI для управления
- [ ] Система авторизации и Service Keys
- [ ] Метрики и мониторинг
- [ ] Валидация конфигураций Envoy
- [ ] Поддержка Kubernetes
- [ ] CI/CD пайплайны

## Лицензия

MIT License

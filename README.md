# Classroom18. Инфраструктура знакомого Task API

Исходная точка — финальный classroom15: PostgreSQL, HTTP/JSON, users/tasks, access JWT,
opaque refresh cookie, ownership, DTO, UseCase и Repository interfaces. Занятия 16–17
пока пропущены. Их содержание для запуска не требуется.

Код продолжается в одном корне. Папок `before/`, вложенного classroom и домашней работы здесь нет.
Эталон для сравнения — отдельный classroom15 из материалов курса.

## Что сохраняется

`GET /health`, `POST /register`, `POST /login`, `POST /auth/refresh`, `POST /auth/logout`;
защищённые `GET /tasks`, `GET /tasks/{id}`, `POST /tasks`, `PATCH /tasks/{id}`,
`DELETE /tasks/{id}`. Task HTTP DTO по-прежнему содержит `id`, `title`, `done`.
AuthUseCase, bcrypt/JWT, HTTP handlers, cookie contract и middleware взяты из финала 15.
Ownership остаётся в TaskUseCase, не в middleware и не в SQL.

## Что добавляется

- Redis реализует прежний `user.RefreshSessionRepository`. AuthUseCase не меняется.
- `processing_status` отражает фоновую обработку отдельно от пользовательского `Done`.
- TaskUseCase публикует `task.created` через EventPublisher после сохранения в PostgreSQL.
- Отдельный worker считает слова в title и публикует `task.processed`, не обращаясь к БД.
- API consumer передаёт результат application handler, который идемпотентно меняет статус.
- gRPC CreateTask/GetTask использует тот же TaskUseCase. Automation — отдельный typed client.
- Docker Compose описывает шесть долгоживущих процессов и два подготовительных job.

## Быстрый запуск

Нужны Go 1.22, C compiler для race, Docker Engine + Compose v2, Python3 и make.
Go toolchain не повышается. Generated protobuf уже включён, для обычной сборки protoc не нужен.

```bash
make help
make ci
make env
make compose-config
make up
make ps
curl -i http://localhost:8080/health
```

`make env` создаёт локальный `.env` с независимыми случайными секретами и правами 600,
существующий файл сохраняется. Не коммитьте его. Запускать команды нужно из корня classroom18.
Сначала пройдите [ручную demo](docs/demo.md), затем `make demo`.
В отдельных терминалах откройте `make logs-worker` и `make kafka-messages`: это блокирующие
наблюдатели. Первая команда показывает логи процесса, вторая — сами Kafka records с JSON,
key, timestamp, partition и offset. `make kafka-messages-new` пропускает историю.
На macOS consumer запускается внутри Kafka-контейнера: отдельные Kafka CLI/Java на хосте
для этих Make-команд не нужны. Подробнее — [как смотреть сообщения](docs/setup.md#сообщения-kafka).

## Команды миграций изменили имена явно

| Команда | Занятие 15 | Занятие 18 |
| --- | --- | --- |
| make up / down | goose Up / Down | Compose stack / остановка с сохранением volumes |
| make migrate-up / migrate-down | — | goose Up / Down последней версии |
| make migrate-status | — | goose Status |
| make status | goose Status | совместимый alias migrate-status; состояние контейнеров — make ps |
| make structure | pg_dump | pg_dump из Compose PostgreSQL |

`001_init.sql` и `002_refresh_sessions.sql` сохранены **побайтово**. `003_processing_status.sql`
добавляет только колонку tasks. История — `goose_db_version`, без ручного baseline.
PostgreSQL `refresh_sessions` остаётся вместе с данными и старой реализацией repository.
Новый активный adapter использует Redis. SQL cookies автоматически не переносятся:
после переключения нужен Login. Старый access JWT продолжает работать до expiration.
Down 003 удаляет processing_status и сохраняет users/tasks/SQL sessions.
Повторные Down откатят 002 и 001, это уже удаление данных, а не restart.

## Три Go-процесса

| Процесс | Зависимости | Роль |
| --- | --- | --- |
| api-service | PostgreSQL, Redis, Kafka | HTTP + gRPC, task.created producer, task.processed consumer |
| task-worker | Kafka | task.created consumer, task.processed producer |
| automation-service | gRPC API | локальный HTTP фасад typed gRPC client, без repositories |

RabbitMQ изучается теоретически, его контейнера и клиента нет.

## Адреса

| Ресурс | С хоста | Из Compose network |
| --- | --- | --- |
| PostgreSQL | localhost:5432 | postgres:5432 |
| Redis | localhost:6379 | redis:6379 |
| Kafka | localhost:19092 | kafka:9092 |
| API HTTP | localhost:8080 | api-service:8080 |
| API gRPC | localhost:50051 | api-service:50051 |
| Automation HTTP | localhost:8090 | automation-service:8090 |

Публикация ports ограничена loopback. `localhost` внутри контейнера указывает на него самого.
Redis намеренно без persistence: его restart может удалить refresh-сессии, но не Tasks.
PostgreSQL и Kafka используют named volumes. `make down` их сохраняет.

## Ограничения демонстрации

gRPC принимает явный user_id в доверенной локальной сети и работает plaintext.
Это не production authentication. В production нужны service identity/credentials, mTLS
и authorization. HTTP Bearer authentication продолжает работать.

SQL commit и Kafka publish не атомарны. При ошибке publish Create возвращает 500,
но pending Task уже может существовать. Не повторяйте Create вслепую. Полноценный
Transactional Outbox здесь не реализован, он разобран отдельно в документации.
Consumer подтверждает offset после обработки, retry может повторить результат.
Невалидный JSON/неисправимое application event в этой demo логируется и пропускается;
production требует явной политики DLQ/retry budget.

## Структура

```text
cmd/api/                  HTTP + gRPC + result consumer
cmd/task-worker/          отдельный worker
cmd/automation-service/   отдельный typed client
cmd/migrate/              goose CLI
internal/task/            TaskUseCase, Repository, EventPublisher, ProcessedHandler
internal/user/            неизменный AuthUseCase и session port
internal/auth/            bcrypt, access JWT, opaque refresh generation
internal/httpapi/         прежние DTO, Mapper, Handler, Middleware
internal/repository/      postgres, memory, redis adapters
internal/messaging/kafka/ producer, consumer, JSON mapping
internal/grpcapi/         protobuf/application mapping
internal/worker/          независимый от Kafka processor
internal/automation/      gRPC client facade
api/proto/task/v1/        исходный контракт
pkg/proto/task/v1/        generated messages/client/server
migrations/               goose 001/002/003 Up + Down
structure.sql             полный schema-only snapshot
sql/ docs/ scripts/ deploy/
```

## Полная команда генерации

Compiler и plugins устанавливаются по docs/setup.md. Команда make proto раскрывается так:

```bash
export PATH="$PWD/.tools:$PATH"
mkdir -p pkg/proto
cd api/proto
protoc -I . \
  --go_out=../../pkg/proto --go_opt=paths=source_relative \
  --go-grpc_out=../../pkg/proto --go-grpc_opt=paths=source_relative \
  task/v1/task.proto
```

Вернуться в корень classroom18 перед другими make commands. Source — api/proto,
output — pkg/proto/task/v1. Повторная генерация проверена с pinned compiler/plugins.

## Проверки и материалы

`make ci`: fmt-check, test, vet, race, build без инфраструктуры.
`make test-db`, `make test-db-race`: TEST_DATABASE_URL.
`make test-redis`: TEST_REDIS_URL. `make test-kafka`: TEST_KAFKA_BROKERS.
`make integration`: все отдельные infrastructure suites. `make qa-compose`:
изолированный холодный stack на Docker-хосте, свободные порты обязательны.

[Настройка](docs/setup.md), [преемственность](docs/continuity.md),
[архитектура](docs/architecture.md), [сессии](docs/session-flow.md),
[события и отказы](docs/events-and-failures.md), [protobuf](docs/protobuf.md),
[сценарий 90/120 минут](docs/teacher-guide.md), [live coding](docs/live-coding.md),
[QA](docs/testing.md). Домашняя работа — отдельный homework_go_18 с Document API.
Дополнительное чтение лежит рядом с презентацией: additional_reading.md.

# Настройка окружения

Go 1.22.0 в go.mod, рекомендуемый patch toolchain 1.22.12, make, Python3.
Для race нужен C compiler. Docker Engine и Compose v2 нужны для контейнерного запуска.

## Compose

```bash
make env
make compose-config
make docker-build
make up
make ps
make migrate-status
make structure
```

Шесть долгоживущих сервисов: postgres, redis, kafka, api-service, task-worker,
automation-service. migrate и kafka-init — одноразовые jobs.
У worker нет HTTP health endpoint: Compose ждёт запуска процесса, а готовность consumer
подтверждается логами и появлением processed. У остальных сервисов есть healthchecks.
Успешный healthcheck не гарантирует готовность произвольной бизнес-операции.

make env не заменяет существующий .env, генерирует случайные JWT_SECRET и POSTGRES_PASSWORD.
Секреты не печатаются. Dockerfile копирует SQL в runtime /app/migrations и задаёт WORKDIR /app,
поэтому cmd/migrate использует тот же os.DirFS("migrations"), что и на занятии 15.

## Нативный запуск

Поднимите отдельные PostgreSQL16, Redis7.4 и Kafka3.9.2. Создайте пустую task_api или
используйте учебную БД занятия 15 с goose history. Экспортируйте DATABASE_URL, JWT_SECRET,
REDIS_URL и KAFKA_BROKERS в своём терминале, не коммитьте реальные credentials.

```bash
make migrate-local-status
make migrate-local-up
make build
bin/api
```

Другие терминалы: `bin/task-worker`, `bin/automation-service`.
По умолчанию хостовые адреса: Redis localhost:6379, Kafka localhost:19092,
API HTTP :8080, gRPC 127.0.0.1:50051, automation 127.0.0.1:8090.
Задайте COOKIE_SECURE=false только для локального HTTP.
В Compose используются DNS postgres:5432, redis:6379, kafka:9092, api-service:50051.
Не запускайте второй экземпляр на занятых портах.

Для SQL snapshot: `make structure-local` с pg_dump в PATH, для Compose — `make structure`.
Для тестов экспортируйте TEST_DATABASE_URL, TEST_REDIS_URL, TEST_KAFKA_BROKERS.
Fixtures создают собственные schema/prefix/topic, рабочие таблицы не очищают.

## Миграции после занятия 15

001/002 и goose_db_version уже существуют: выполняется только новая 003.
`make migrate-up` в Compose, `make migrate-local-up` для нативного DSN.
Rollback: остановить API18, один migrate-down удаляет processing_status.
Повторные Down удаляют SQL sessions и затем users/tasks. Выполнять только на тестовой БД.
Старые SQL refresh cookies после Redis cutover требуют Login. Access JWT жив до exp.
Users, Tasks и SQL sessions остаются в PostgreSQL. Никакого автоматического переноса нет.

## Protobuf generation

Generated code включён. Для воспроизведения compiler — libprotoc3.21.12 (release v21.12).
Пример установки Linux x86_64 из официального release:

```bash
curl -fL https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip -o protoc.zip
unzip protoc.zip -d "$HOME/.local/protoc-21.12"
export PATH="$HOME/.local/protoc-21.12/bin:$PATH"
protoc --version
make tools
make proto
git diff --exit-code -- pkg/proto
```

Для macOS/другой architecture выберите архив той же версии на официальной странице release.
Plugins закреплены: protoc-gen-go v1.36.5, protoc-gen-go-grpc v1.5.1, grpcurl v1.9.3.
make proto использует source_relative, source api/proto/task/v1, output pkg/proto/task/v1.

## Логи

LOG_LEVEL=DEBUG|INFO|WARN|ERROR|FATAL, LOG_FORMAT=text|json.
INFO — startup/lifecycle, DEBUG — repository operations без credentials,
ERROR — неожиданная ошибка на границе обработки, FATAL — только main после cleanup.
Redis hash, password и access/refresh credentials в логи не записываются.
`make logs-*` — блокирующие команды наблюдения, запускайте в отдельных терминалах.

## Состояние данных

make down сохраняет PostgreSQL/Kafka named volumes. Redis настроен без persistence:
после его пересоздания refresh может стать недействительным, users/tasks не исчезают.
Не используйте down -v для обычного завершения занятия.

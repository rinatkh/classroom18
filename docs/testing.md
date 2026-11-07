# Тестирование

## Быстрый цикл

```bash
make ci
```

fmt-check, go test ./..., go vet ./..., go test -race ./..., build всех cmd.
JWT/handler/UseCase/middleware tests используют fakes/memory, PostgreSQL не нужен.
Все прежние именованные тесты из 15 сохранены; migration fixture расширен для третьего Down.

## Реальная инфраструктура

Экспортируйте отдельные TEST_DATABASE_URL, TEST_REDIS_URL, TEST_KAFKA_BROKERS.
`make test-db`, `make test-db-race`, `make test-redis`, `make test-kafka`, `make integration`.
Fixtures изолируют schema/key prefix/topic. Отсутствие адреса при explicit integration run — ошибка.

Проверяются goose up/down/up/history/data, SQL user/task/session repositories,
Redis metadata/hash/TTL/revoke/concurrent single-winner/collision/intermediate failure,
Kafka publish → worker → result. Unit consumer tests контролируют Handle перед Commit,
retry того же record, retry Commit, cancellation и заявленную poison skip policy.

`make tools && make proto`, затем `git diff --exit-code -- pkg/proto`.
`make compose-config` проверяет декларацию, не доказывает запуск контейнеров.
`make qa-compose` запускает disposable stack на Docker-хосте. Скрипт удаляет только
свои project volumes и требует свободные ports; сначала остановите обычный учебный stack.

Ручные проверки: docs/demo.md. Старые HTTP: make curl-demo.
Реальный JWT expiration: API с ACCESS_TOKEN_TTL=20s и make session-demo, затем вернуть 15m.
Различайте невозможность выполнить проверку и успешно выполненную проверку.

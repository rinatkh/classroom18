# Continuity: final15 → classroom18

Начало занятия — отдельный финальный classroom15. Он не копируется в before и не изменяется.
Занятия 16–17 пока пропущены, новых предварительных требований нет.

## Без изменения поведения

- internal/user: AuthUseCase, ports и модель пользователя/refresh session.
- internal/auth: bcrypt, opaque refresh/hash, JWT и проверки expiration/algorithm.
- internal/httpapi: routes, DTO, Mapper, Handler, AuthMiddleware и cookie configuration.
- Ownership, PATCH semantics, title validation, limit/repeated completion.
- PostgreSQL users/tasks и прежний SQL session adapter для сравнения/тестов.
- Все прежние именованные tests; fixtures используют тот же goose.
- migrations/001_init.sql и 002_refresh_sessions.sql побайтово из 15.

## Изменённые production файлы прежнего приложения

| Файл | Причина |
| --- | --- |
| cmd/api/main.go | Redis session wiring, Kafka publisher/results, gRPC listener, lifecycle |
| internal/task/model.go | дополнительное processing state, Done сохраняет смысл |
| internal/task/usecase.go | EventPublisher port после прежнего Create |
| internal/repository/memory/task.go | initial processing status, сохранены DEBUG logs |
| internal/repository/postgres/task.go | Scan/RETURNING нового поля, сохранены DEBUG logs |

Новые packages отвечают за Redis, Kafka, worker, gRPC/automation и обработку результата.
Domain/application не импортируют их библиотеки. Generated protobuf находится в pkg/proto.

## Database continuity

Пустая БД: goose применяет 001 → 002 → 003.
БД после занятия 15: goose видит applied 001/002 и применяет только 003.
Нет schema_migrations или ручного baseline. SQL refresh_sessions и история остаются.
Новый Login записывает HASH с TTL в Redis. SQL credential не импортируется автоматически.
Старый access сохраняет силу до exp; users/tasks остаются на месте.
Down 003 удаляет только processing_status. Старые SQL sessions не восстанавливаются из Redis.

## Main как место переключения

Было: postgres.NewRefreshSessionRepository(pool).
Стало: redisrepo.NewRefreshSessionRepository(redisClient).
Далее тот же user.NewUseCase(...sessions...), JWT, cookie handlers и middleware.
TaskUseCase дополнен port для новой async-потребности; HTTP CRUD contract не заменён.

Проверка: сравнить перечисленные неизменные файлы после замены module prefix 15 → 18;
запустить прежние tests и scripts/curl-demo.sh, scripts/session-demo.sh, затем scripts/demo.sh.

# Ручная системная demo

Сначала `make up`, `make ps`. В другом терминале `make kafka-consume`, ещё в одном
`make logs-worker`. Никогда не печатайте реальные access/refresh в общий log.
Следующие значения относятся только к одноразовому localhost classroom.

```bash
curl -i http://localhost:8080/health
curl -sS -X POST http://localhost:8080/register -H 'Content-Type: application/json' \
  -d '{"email":"alice18@example.test","password":"demo-password"}'
```

Запомните id из register. Login/refresh cookie лучше исследовать через временный файл
с правами 600 (`umask 077`) и `curl -c/-b`. Полный безопасный lifecycle уже реализован
в `scripts/demo.sh`: его source показывает каждый request и удаляет credentials после run.
Не вставляйте access token в документацию/историю публичного терминала.

`make redis-cli`: `SCAN 0 MATCH auth:* COUNT 100`, затем HGETALL session-key и TTL.
В key только SHA256(raw), raw refresh нет. После rotation старый HASH имеет revoked_at,
новый — новый hash и TTL. После logout revoked. После expiry Redis удаляет HASH.

```bash
make tools
make grpc-list
.tools/grpcurl -plaintext -d '{"user_id":1,"title":"Learn real infrastructure"}' \
  localhost:50051 task.v1.TaskService/CreateTask
.tools/grpcurl -plaintext -d '{"user_id":1,"task_id":1}' \
  localhost:50051 task.v1.TaskService/GetTask
```

Замените user_id/task_id фактическими значениями. Эти commands используют trusted demo
identity, они не проверяют JWT. Повторите Get после worker — processingStatus processed,
Done остаётся false. `grpcurl` показывает JSON mapping protobuf, raw transport binary.

Теперь отдельный client process:

```bash
curl -sS -X POST http://localhost:8090/automation/tasks -H 'Content-Type: application/json' \
  -d '{"user_id":1,"title":"Create from automation"}'
curl -sS 'http://localhost:8090/automation/tasks/2?user_id=1'
```

Фасад automation использует protojson с use_proto_names: int64 ID в JSON строки,
processing_status snake_case. Это отдельный demo transport; Task API HTTP DTO не меняется.
Проверьте SQL через make psql:

```sql
SELECT id,owner_id,title,done,processing_status FROM tasks ORDER BY id;
SELECT to_regclass('refresh_sessions');
```

Вторая query возвращает refresh_sessions: SQL таблица и её данные сохранены. В Kafka console видны оба JSON event
с одним task_id; API и worker имеют свои stdout. Automation не имеет DB connection.

Создайте Bob через register/login. Bearer Bob + PATCH Task Alice →403. GetTask с Bob
user_id тоже PermissionDenied, потому что ownership находится в общем TaskUseCase.
Обычный Task HTTP GET Alice всё ещё id/title/done, без processing field.

Для автоматического повторения после ручного разбора: `make demo`.
Для привычных HTTP ошибок/lifecycle: `bash scripts/curl-demo.sh`.
Для реального expiration: запустите API с ACCESS_TOKEN_TTL=20s, затем
`bash scripts/session-demo.sh`; после опыта верните 15m и пересоздайте API environment.

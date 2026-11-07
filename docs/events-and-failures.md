# Events, delivery и отказы

Один task.events topic, три partitions. Record key=task_id, поэтому связанные записи
одной Task попадают в одну partition при неизменном partition count/partitioner.
Offset — положение внутри partition, глобального offset всего topic нет.

Envelope version1: event_id, event_type, event_version, occurred_at, task_id, user_id.
Created добавляет title, processed — result=success и word_count. JSON — инфраструктурный
формат, application Event остаётся обычным Go type. Kafka events и protobuf contracts
развиваются отдельно. Worker не принимает password/access/refresh credentials.

Группы task-workers и api-task-results читают независимо. Внутри group partition
назначается одному участнику одновременно по протоколу группы; при rebalance возможен
повтор бизнес-обработки. Число занятых consumers ограничено числом partitions.
Console consumer — независимый наблюдатель, не делите с ним application group ID.

Создание ждёт Kafka ack, но не ждёт worker. HTTP/gRPC response означает Task сохранён
и publish подтверждён при успешном ответе, а не то, что Task уже processed.
Pending в response может успеть стать processed до следующего Get.

Consumer: Fetch → Decode → type filter → Handle → Commit. Выход worker публикуется
до commit входного offset. Publish failure повторяет этот record. Crash после publish
и до commit повторит обработку и породит duplicate result. API MarkProcessed идемпотентен
в SQL, повтор не меняет Done/updated_at. Result owner mismatch считается invalid event.
Удалённая Task игнорируется: async результат больше не нужен.

Ошибки JSON/version и invalid application events в demo пропускаются с безопасным log,
без raw payload. Production добавит DLQ/retry budget/metrics, иначе poison record или
бесконечный transient failure может задержать stream. Один последовательный handler
умышленно задерживает остальные partitions этого reader при ошибке.

| Отказ | Наблюдаемое поведение | Следующий шаг |
| --- | --- | --- |
| PostgreSQL down | CRUD/register/login не могут читать/писать business data | Восстановить PG, проверить neutral 500 |
| Redis down | Login/Refresh/Logout могут вернуть 500 | Обычный Bearer access до exp не читает Redis |
| Redis restart без persistence | Refresh keys потеряны, старые refresh → 401 | Login снова, users/tasks сохранены |
| Kafka down при Create | Task уже мог commit, publish ошибся, API 500 | Проверить pending в PG, не повторять Create слепо |
| Worker down | Created остаётся в log, processing задерживается | Вернуть worker до истечения retention |
| API down | Automation получает RPC Unavailable/timeout | Deadline ограничивает ожидание, результат Create мог быть неоднозначен |
| API result consumer down | Processed log ждёт consumer | Возврат к committed offset, idempotent update |

## Dual write и outbox

SQL transaction не включает Kafka. Ошибка publish не превращается в SQL rollback.
В коде это ErrEventPublish и нейтральный HTTP 500 / gRPC Internal. Клиент не знает,
успела ли операция сохраниться. Outbox пишет Task и строку outbox в одной PG transaction,
затем отдельный dispatcher доставляет в Kafka с повторами. Он тоже требует идемпотентного
consumer. Здесь outbox только концепция, никакого обещания exactly once end-to-end.
Kafka producer idempotence и broker transactions сами по себе не делают SQL side effect
частью единой transaction.

## Локальная fault lab

Поднимите make up и создайте Task. В отдельных терминалах смотрите logs-worker/api и
kafka-consume. `docker compose --env-file .env -f deploy/compose.yaml stop task-worker`,
создайте ещё Task: pending. `start task-worker`: processed. Не используйте down -v.
Аналогично stop redis: прежний Bearer GET /tasks работает, Refresh возвращает 500.
`start redis` с demo config теряет refresh state, Login нужен заново.
Сначала выполните эти шаги руками, затем сравните с QA report.

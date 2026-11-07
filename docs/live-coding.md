# Live coding: подготовленный проект, ключевые изменения руками

0. **Финальный classroom15.** Запустить отдельный classroom15 с отдельной PG schema15. Login, Refresh,
Task CRUD. Посмотреть SQL sessions. Спросить, какие слои изменятся при смене session storage.
Ожидаем RefreshSessionRepository implementation + composition root. Остановить classroom15.
1. **Redis.** PING, SET/GET/EXPIRE/TTL/DEL на lab keys. Открыть прежний session interface,
реализовать Create/Get, объяснить подготовленный Lua Rotate и atomic recheck. Подменить
ровно один session repository в main, проверить Login/Refresh/replay401/Logout.
2. **Docker.** Собрать API image, показать multi-stage и runtime без compiler. Image/container,
ports/volumes/network. Поднять PG/Redis/API через Compose, localhost vs service DNS.
3. **Kafka port.** Новая потребность: title analysis вне основного request. TaskUseCase
после SQL вызывает EventPublisher. Добавить JSON envelope в adapter, показать raw console
message. Честно остановиться на dual write и 500 при publish failure.
4. **Worker.** Отдельный cmd: consumer → processor → publisher. Processor считает words,
не ходит в чужую DB. Показать output TaskProcessed и manual input offset commit.
5. **API consumer.** ProcessedHandler + MarkProcessed в PG, повтор event не меняет Done.
Две independent groups читают один topic и фильтруют разные types.
6. **Protobuf.** Открыть api/proto/task/v1/task.proto, объяснить field numbers/go_package.
Выполнить make proto. Открыть оба generated files в pkg/proto, показать Go import.
7. **gRPC server.** Create/Get adapters вызывают прежний TaskUseCase, error status mapping,
caller ctx до PG/Kafka. grpcurl с deadlines по умолчанию ограничьте `-max-time 6` при lab.
8. **Automation.** Отдельный executable и typed TaskServiceClient, deadlines каждого RPC.
DB repository ему не передаётся. Разобрать доверенный user_id как local упрощение.
9. **Full stack.** make up/ps/demo, Kafka console, SQL status, Redis TTL, process logs.
Выключить worker, создать pending Task, вернуть worker, увидеть processed.
10. **Вернуться к знакомому коду.** AuthUseCase/HTTP handlers/JWT middleware не менялись.
TaskUseCase расширился через port. Outbox/service identity/HA остаются следующим шагом.

Не набирать SQL fixtures, generated code и Compose boilerplate на уроке с нуля.
Они подготовлены, ученица изменяет принципиальные seams. Teacher guide задаёт тайминг.

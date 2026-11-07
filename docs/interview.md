# Interview

## 1. Почему при замене PostgreSQL sessions на Redis не меняется AuthUseCase?

**Ответ:** Он зависит от RefreshSessionRepository, concrete adapter выбирает main.

**Key point:** Порт принадлежит потребителю.

**Частая ошибка:** Импортировать Redis client в user package.

## 2. Что хранится в Redis и почему TTL?

**Ответ:** Hash-key и session metadata, session истекает естественно.

**Key point:** Raw token остаётся только у клиента.

**Частая ошибка:** Redis заменяет все persistent таблицы.

## 3. Почему rotation не делаем двумя DEL/SET calls?

**Ответ:** Между calls вклинится другой refresh. Lua проверяет и меняет состояние атомарно.

**Key point:** Конкуренция существует и между разными API processes.

**Частая ошибка:** Думать, что Go mutex защитит distributed state.

## 4. Что означает localhost внутри container?

**Ответ:** Текущий container. PG доступен по postgres:5432 в общей сети.

**Key point:** Host и container адресуются по-разному.

**Частая ошибка:** Копировать host DSN внутрь compose.

## 5. Чем image отличается от container?

**Ответ:** Image является собранным шаблоном, container — его runtime instance.

**Key point:** Один image можно запустить несколько раз.

**Частая ошибка:** Считать image работающим процессом.

## 6. Почему automation не читает tasks таблицу?

**Ответ:** Business data принадлежит API service, automation использует его gRPC contract.

**Key point:** Граница приложения важнее физической доступности БД.

**Частая ошибка:** Shared сеть трактовать как shared ownership.

## 7. Чем Kafka отличается от gRPC?

**Ответ:** Kafka доставляет события асинхронно, gRPC возвращает результат вызова.

**Key point:** Consumer может прийти позже, RPC имеет deadline.

**Частая ошибка:** Использовать request/response и event как взаимозаменяемые понятия.

## 8. Почему worker публикует TaskProcessed вместо UPDATE?

**Ответ:** API владеет Task и применяет результат через application handler.

**Key point:** Worker одновременно consumer и producer.

**Частая ошибка:** Связать оба процесса чужой SQL schema.

## 9. Зачем field numbers в protobuf?

**Ответ:** Это идентификаторы полей wire format, их сохраняют для compatibility.

**Key point:** Не переиспользовать удалённые номера.

**Частая ошибка:** Считать title=2 значением поля.

## 10. Redis упал: пропали Task?

**Ответ:** Нет, Task source of truth PostgreSQL. Session operations недоступны, access JWT живёт до exp.

**Key point:** Разные stores имеют разные роли.

**Частая ошибка:** Проверять Redis на каждом JWT request без необходимости.

## 11. Worker выключен 5 минут: события обязательно потерялись?

**Ответ:** Нет, при сохранённом broker log и retention consumer продолжит с offset.

**Key point:** Commit относится к group/partition.

**Частая ошибка:** Путать commit с удалением всего log.

## 12. В чём dual write?

**Ответ:** SQL commit и Kafka publish не единая transaction, между ними возможен сбой.

**Key point:** Outbox связывает business write и запись будущего события в PG.

**Частая ошибка:** Обещать rollback PostgreSQL при ошибке Kafka.

## 13. Как сравнить Kafka и RabbitMQ?

**Ответ:** Kafka ориентирован на event log/replay/groups, classic Rabbit queues — routing/ack/work distribution.

**Key point:** Возможности пересекаются, Rabbit также имеет streams.

**Частая ошибка:** Оценивать инструменты как хороший/плохой.

## 14. Почему generated package в pkg/proto?

**Ответ:** Он нужен другим modules как transport contract, internal ограничивает внешний import.

**Key point:** Domain model остаётся internal.

**Частая ошибка:** Считать pkg специальным Go keyword.

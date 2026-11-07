# Mental models и альтернативы

## Redis и PostgreSQL

PostgreSQL хранит relational business data: ограничения, связи, transactions, durable
users/tasks. Redis — быстрый in-memory store с key-value структурами: STRING, HASH,
LIST, SET, SORTED SET. Помимо cache его используют для sessions/counters/temporary
state, иногда coordination с отдельно продуманными гарантиями.

В этом проекте session имеет естественный expires_at. Redis TTL удаляет key без
отдельного SQL cleanup query. Это удобство модели, а не аргумент «вся БД должна быть Redis».
Redis умеет persistence/replication, но они требуют конфигурации и понимания компромиссов.
Demo намеренно их не включает. Redis restart требует нового Login, Tasks не исчезают.

CLI lab, отдельные disposable keys:

```text
PING
SET lab:session alice EX 30
GET lab:session
TTL lab:session
EXPIRE lab:session 10
DEL lab:session
SCAN 0 MATCH auth:* COUNT 100
```

Для реального session HASH: HGETALL key, TTL key. `sequence` не session, TTL=-1 нормально.
TTL=-2 означает key отсутствует, -1 означает expiration не задана.
KEYS допустим только на маленьком локальном keyspace, в production предпочитайте SCAN.

## Docker и VM

VM включает guest OS/kernel. Container изолирует процесс и использует kernel runtime host.
На macOS/Windows Docker Desktop обычно запускает Linux containers внутри Linux VM.
Image — собранный шаблон layers, container — запущенный instance. Registry хранит images.
Dockerfile описывает build, build stage содержит compiler, runtime stage — binaries/CA.
Ports связывают host listener и container port. Volume хранит данные независимо от
container writable layer. Network даёт service DNS, env vars задают local config.

Compose service описывает, как запускать container; `up` материализует декларацию.
Compose подходит для development/integration/small deployments. Kubernetes — другая
модель orchestrator, Deployment на этом уроке не создаётся. Process running не означает
ready: healthchecks и depends_on conditions обеспечивают initial startup readiness.
После startup приложение всё равно должно обрабатывать недоступность зависимостей.

## Kafka и gRPC

| | Kafka | gRPC |
| --- | --- | --- |
| Модель | Событие уже произошло | Выполни операцию и верни результат |
| Связь во времени | Consumer может прийти позже | Client ждёт bounded response |
| Контракт урока | JSON TaskCreated/Processed envelope | Protobuf TaskService |
| Пример | Worker анализирует Task позже | Automation Create/Get прямо сейчас |

Kafka — event log с retention, partition offsets и независимыми consumer groups.
Коммит offset не равен удалению message. Replay зависит от retention и доступного log.
RPC — remote procedure call, сетевой вызов со сбоями/deadline/неоднозначным результатом.
Даже typed client не делает network локальной функцией. gRPC обычно использует HTTP/2
и protobuf, поддерживает streaming, но на уроке только unary request/response.
HTTP/JSON проще исследовать curl и удобно браузерам; gRPC даёт generated typed contracts
и client/server stubs для service-to-service. Оба подходят разным границам.

## RabbitMQ: только теория

AMQP producer публикует в exchange. Exchange применяет bindings/routing key и
маршрутизирует сообщение в queue. Consumer читает queue и подтверждает обработку ack.
Consumer ack и publisher confirm — разные подтверждения, ни одно не отменяет нужду
в повторной безопасной обработке бизнес-операции.

| Exchange type | Правило |
| --- | --- |
| direct | Точное совпадение routing key с binding key |
| topic | Pattern segments, `*` один segment, `#` ноль или больше |
| fanout | Всем привязанным queues, routing key не определяет маршрут |

Kafka типично используют как durable stream/replay/event pipeline с независимыми groups.
RabbitMQ классические queues удобны для routing/work distribution/ack-based delivery.
В обычной queue ack освобождает сообщение, replay не организован как Kafka offset log.
Границы пересекаются: RabbitMQ также имеет streams и другие возможности. Нельзя свести
сравнение к «один быстрый/хороший, другой плохой» или объявить, что один всегда хранит,
а другой никогда не хранит данные. Для нашего TaskCreated/Processed stream выбираем Kafka.
RabbitMQ dependency/container в classroom отсутствует.

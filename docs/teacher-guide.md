# Сценарий занятия 18: 90 и 120 минут

Исходная точка — отдельный финальный classroom15. Код занятия 18 подготовлен:
за 90 минут не нужно набирать все Docker/Lua/Kafka/gRPC implementations с нуля.
Папки before и дубли проекта не нужны. Полная речь для каждого слайда находится
в Speaker Notes и рядом с PPTX в lesson18_teacher_speech.md.

## Подготовка

На отдельном Docker-хосте: make ci, make env, make compose-config, make up, make demo.
Заранее скачать images/modules, установить pinned compiler/plugins по docs/setup.md,
проверить make proto с пустым diff. Проверить свободные ports, открыть GoLand и терминалы
для API/worker/Kafka/Redis. Для сравнения держать classroom15 рядом, без изменения его файлов.
RabbitMQ и Kubernetes не устанавливать. Down -v для обычного занятия не используется.

## Основной маршрут на 90 минут

| Минуты | Слайды CORE | Режим | Что сделать / контроль |
| --- | --- | --- | --- |
| 0–7 | 1, 2, 4 | знакомый проект | HTTP/auth, repositories из 15. Где source of truth Task? |
| 7–22 | 9, 10, 12, 13, 15, 16 | CLI + live coding seam | TTL lab key, session port, wiring Redis; Lua разбирается готовой. Почему один winner? |
| 22–37 | 19, 20, 23, 24, 27, 29, 32 | готовый Dockerfile/Compose | Image/container, COPY runtime SQL, service DNS, make up/ps. Что означает localhost? |
| 37–57 | 35, 36, 38, 39, 41, 43, 45, 48, 49, 52 | live coding EventPublisher + demo | Один Event, worker processor, owner result handler, dual write. Когда commit? |
| 57–62 | 55, 58 | теория на схеме | Exchange/queue и сравнение Kafka. Ack не равно completed Task. |
| 62–78 | 60, 64, 69, 72, 74, 78, 81 | generation + mapping | .proto, make proto, оба generated файла, общий UseCase, deadlines и trusted user_id. |
| 78–88 | 88 | ручной E2E, затем автоматизация | Redis TTL, Kafka console, automation Create/Get, SQL processed, Bob403, make demo. |
| 88–90 | 90 | самостоятельная работа | Открыть отдельный homework_go_18. CI green, acceptance red до TODO. |

Всего 37 CORE slides. 53 DEEP DIVE доступны для повторения и вопросов, не требуют
дополнительных 53 live explanations за те же 90 минут. При отставании переносите
подробный consumer loop и failure lab, сохраняйте смену adapter и полный system flow.
В logs-* и kafka-consume переходить через отдельные терминалы: команды блокирующие.

## Расширенный маршрут на 120 минут

Сохраните основной маршрут и добавьте 30 минут:

| Дополнительное время | Слайды | Работа |
| --- | --- | --- |
| +7 минут Redis | 14, 17, 18 | checks перед mutations, Lua atomicity без обещания rollback, Redis restart |
| +6 минут Kafka | 50, 51, 53 | same-record retry, Commit retry, duplicate window, outbox только теория |
| +3 минуты RabbitMQ | 56, 57 | direct/topic/fanout, consumer ack и publisher confirm |
| +7 минут gRPC | 65, 68, 75, 80 | numbers, installation path, status mapping, ownership без fake authentication |
| +7 минут финал | 85, 86, 87, 89 | goose history15→18, failure matrix, tests и короткие interview answers |

Эти добавления не означают чтение вслух всех DEEP DIVE slides.

## Практика и возврат

В Notes каждого практического слайда указан реальный file, command, ожидаемый результат
и номер возврата. Команды выполняются из classroom18, кроме явно указанной homework.
Live coding: поменять session wiring, расширить Create через port, написать несколько строк
processor и mapping. Boilerplate Compose, generated Go и SQL fixtures открывать готовыми.

Для ручной demo сначала docs/demo.md. Сопоставить один TaskID в automation response,
created/processed JSON, worker logs и SQL. Убедиться, что Done остался false.
Для sessions expiration нужен отдельный перезапуск API с ACCESS_TOKEN_TTL=20s и
make session-demo; после опыта вернуть 15m. Это отдельная демонстрация, не unit test.

Финальные ответы: AuthUseCase и HTTP contract прежние; TaskUseCase расширен новой async
потребностью. Worker не владеет Task DB. gRPC user_id без credentials — доверенное demo,
не production auth. Dual write остаётся явным ограничением, outbox не реализован.

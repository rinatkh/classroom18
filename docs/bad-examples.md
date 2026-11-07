# Ошибки, которые нужно распознать в этом проекте

| Ошибка | Почему ломает сценарий | В нашем коде |
| --- | --- | --- |
| redis.Client в AuthUseCase | Новый storage переписывает business | Прежний session interface, новый adapter |
| Raw refresh как Redis value | Утечка store раскрывает credentials | SHA256 raw в key, metadata в HASH |
| GET old, DEL old, SET new отдельно | Два refresh могут пройти одновременно | Lua atomic recheck/rotate |
| Compiler в runtime image | Лишние build dependencies | Multi-stage, scratch с binaries и CA |
| localhost:5432 внутри API container | Ищет PG в самом API container | postgres:5432 через Compose DNS |
| Worker UPDATE tasks | Чужой процесс обходит application owner | TaskProcessed event, API handler writes PG |
| Publish Kafka прямо в HTTP handler | Второй transport повторяет orchestration | EventPublisher в общем TaskUseCase |
| Commit до publish result | Crash теряет обработку | Output ack, потом input commit |
| «Exactly once, дубликатов нет» | Повторы/dual write остаются | Idempotent result + честное ограничение |
| .pb.go вручную | Следующая генерация уничтожит правку | Изменить .proto, make proto |
| Proto request в TaskUseCase | Application зависит от transport | grpcapi mapping в task.CreateInput |
| gRPC user_id считают authentication | Любой trusted caller может подменить ID | Явное local-only demo, production identity позже |
| One API process mutex считают global | Несколько instances не делят mutex | Один API в compose, SQL coordination отдельно |

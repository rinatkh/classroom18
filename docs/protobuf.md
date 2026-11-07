# Protobuf и gRPC: source, generation, imports

`api/proto/task/v1/task.proto` — исходник. `pkg/proto/task/v1/*.pb.go` — generated output.
`syntax="proto3"` выбирает язык, `package task.v1` — protobuf namespace,
`option go_package="github.com/rinatkh/classroom18/pkg/proto/task/v1;taskv1"` — Go import и alias.
Message содержит поля с wire identifiers, service содержит rpc signatures.

`title=2` не присваивает title число 2. Номер 2 записывается в binary wire format.
После публикации нельзя произвольно заменить его на 7 или переиспользовать 2 для нового
смысла. При удалении используйте `reserved 2; reserved "title";`, планируйте compatibility.

`make tools` устанавливает protoc-gen-go v1.36.5, protoc-gen-go-grpc v1.5.1 и grpcurl.
`protoc` находит plugins через PATH `.tools`. `make proto` раскрыт полностью в README.
Source-relative paths оставляют `task/v1` под output directory; иначе легко получить
лишние module path folders. Проверка воспроизводимости: сохраните generated files,
выполните make proto и убедитесь, что git diff pkg/proto пуст.

В основном contract timestamps не нужны. Отдельный минимальный пример import:

```proto
syntax = "proto3";
package example.v1;
import "google/protobuf/timestamp.proto";
message Audit { google.protobuf.Timestamp occurred_at = 1; }
```

Compiler должен иметь include directory с Google well-known types (обычно входит в
пакет protoc). Go generator использует `timestamppb` из protobuf runtime.
Этот дополнительный пример не добавляет поле в основной Task API.

Server делает RegisterTaskServiceServer(grpcServer,handler), implementation embeds
UnimplementedTaskServiceServer. Client делает grpc.NewClient(target, credentials),
затем NewTaskServiceClient(conn). NewClient создаёт connection object, сам по себе не
гарантирует, что server уже доступен. Каждый RPC имеет context deadline.

| Application result | gRPC status | HTTP аналог |
| --- | --- | --- |
| Empty title / invalid IDs | InvalidArgument | 400 |
| Missing Task | NotFound | 404 |
| Foreign Task | PermissionDenied | 403 |
| Business conflict | FailedPrecondition | 409 |
| Context deadline | DeadlineExceeded | Обычно 504 на proxy |
| Internal repository/publish error | Internal | 500 |

Unauthenticated (аналог 401) относится к identity. В нашем trusted local gRPC demo нет
identity interceptor: user_id передаётся явно. Не выдавайте это за защищённый public API.
HTTP AuthMiddleware из 13/15 продолжает проверять JWT. Ownership находится в TaskUseCase,
поэтому gRPC GetTask для Bob тоже возвращает PermissionDenied.

# Пять отдельных проходов QA

1. Continuity: final15 как эталон, неизменные auth/HTTP/interfaces, старые tests,
   byte-identical 001/002, только 003 поверх goose history.
2. Architecture/code: Domain без Redis/Kafka/pgx/protobuf, Handler через UseCase,
   worker без DB, contexts/deadlines/cleanup, comments, fmt/test/vet/race/build.
3. Infrastructure/E2E: холодный isolated stack, migrations, Redis rotation/TTL,
   Kafka created/processed, automation gRPC, restart/failure paths, 401/403/404/409.
4. Presentation/speech: все слайды отдельно и montage, Notes и речь совпадают,
   CORE/DEEP route и практика с реальными файлами, повторный render после исправления.
5. Homework/package: самостоятельный Document TODO starter, baseline CI green,
   acceptance red до реализации, ссылки/команды/generated output, fresh git apply и tests.

make ci, make compose-config, make proto, make integration, make qa-compose.
Compose config validation не заменяет Docker build/run. Native services не проверяют Docker DNS.

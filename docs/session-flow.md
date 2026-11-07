# Sessions: тот же port, другой adapter

AuthUseCase и RefreshSessionRepository взяты из финала 15 без изменений.
Create/GetByTokenHash/Rotate/Revoke сохраняют контракты. Сравнивайте с отдельным classroom15,
а две реализации доступны в internal/repository/postgres/session.go и redis/session.go.

Login генерирует opaque raw credential, хранит только SHA-256 hash и metadata.
Ключ auth:{refresh}:<hash> — HASH, фигурные скобки задают общий Redis hash slot для Lua keys.
Sequence — отдельный технический ключ, он не является сессией и не имеет её TTL.
PEXPIREAT использует абсолютный ExpiresAt. TTL записи не продлевает срок credentials.
После expiration Redis удаляет HASH, приложение также проверяет ExpiresAt.

Rotate повторно проверяет revoked/expiry/owner/collision и создаёт новый HASH,
сохраняя ссылку на замену и revocation старого. Lua исключает interleaving клиентов.
Это атомарность выполнения, а не SQL rollback произвольной runtime ошибки.
Проверки выполняются до изменений, INCR — до создания нового HASH; недопустимый timestamp
отсекается заранее. Интеграционные тесты проверяют collision, wrong owner, ошибку sequence,
TTL и ровно одного победителя из 20 конкурентных refresh.

Logout отзывает refresh и очищает cookie. Уже выданный stateless access JWT остаётся жив до exp.
Redis outage мешает Login/Refresh/Logout, Bearer access к Tasks не читает Redis.
В Compose Redis неперсистентный: restart может потерять sessions. PostgreSQL Tasks остаются.
SQL refresh_sessions не удалена, но новый adapter её не читает. Cutover требует Login.

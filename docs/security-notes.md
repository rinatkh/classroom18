# Security continuity

Сохраняются bcrypt/JWT signing algorithm/expiry/HttpOnly cookie/host-only/Path/auth/
SameSiteLax/rotation/logout из 15. AuthMiddleware по-прежнему читает только access Bearer,
не обращается к session repository. HttpOnly не устраняет CSRF сам по себе, local HTTP
COOKIE_SECURE=false относится только к lab, production требует HTTPS/Secure/CSRF strategy.

Raw refresh не хранится в Redis/PG/logs/events. Metadata хранится по SHA256 hash-key.
Rotated/revoked session retained до expiry, replay401, один concurrent winner.
Сроки sliding как в 15, family reuse detection, абсолютный lifetime/logout-all отсутствуют.
Logout не отменяет уже выданный access JWT. Redis restart без persistence теряет sessions,
значит Login снова. Пользовательские Task/пароли от этого не исчезают.

В PostgreSQL был FK session→user с CASCADE. После Redis swap межсистемной FK нет.
Удаление account пока не является HTTP feature. При добавлении удаления/logout-all
придётся явно отзывать user sessions и определить access invalidation policy.
Обычный Refresh не делает новый UserRepository lookup: это неизменное поведение 15.

Local gRPC и automation доверяют user_id, plaintext/reflection допускаются только в
доверенной localhost/Compose среде. Это учебный service-to-service seam, не production
access control. HTTP endpoints всё ещё требуют JWT и ownership. Production gRPC identity
нужно связать с разрешённым actor, применять mTLS/credentials/network restrictions.

Kafka JSON не содержит credentials, но title может содержать пользовательские данные:
production retention/ACL/encryption/observability проектируются отдельно.
В local stack plaintext broker/Redis/PG ports публикуются только на loopback.

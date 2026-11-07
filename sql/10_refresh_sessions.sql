-- Состояние после login/refresh/logout без credentials.
SELECT id, user_id, expires_at, created_at, revoked_at, replaced_by_session_id
FROM refresh_sessions
ORDER BY id;

-- token_hash исключён: для анализа состояния сессии достаточно id и статусов.
SELECT user_id,
       count(*) AS total,
       count(*) FILTER (WHERE revoked_at IS NULL AND expires_at > now()) AS active
FROM refresh_sessions
GROUP BY user_id;

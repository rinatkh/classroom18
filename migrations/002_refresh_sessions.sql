-- +goose Up
-- +goose StatementBegin

CREATE TABLE refresh_sessions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    replaced_by_session_id bigint REFERENCES refresh_sessions(id)
);

-- UNIQUE уже создаёт индекс для поиска по hash. user_id нужен для FK cascade.
CREATE INDEX refresh_sessions_user_id_idx ON refresh_sessions(user_id);

COMMENT ON TABLE refresh_sessions IS 'Серверные refresh-сессии существующего Task API';
COMMENT ON COLUMN refresh_sessions.id IS 'Идентификатор сессии, создаваемый PostgreSQL';
COMMENT ON COLUMN refresh_sessions.user_id IS 'Пользователь сессии; удаление пользователя удаляет его сессии';
COMMENT ON COLUMN refresh_sessions.token_hash IS 'SHA-256 hash случайного refresh credential; исходный токен не хранится';
COMMENT ON COLUMN refresh_sessions.expires_at IS 'Срок действия refresh-сессии, проверяемый независимо от access JWT';
COMMENT ON COLUMN refresh_sessions.created_at IS 'Время создания сессии';
COMMENT ON COLUMN refresh_sessions.revoked_at IS 'Время отзыва; NULL означает, что отзыв не выполнялся';
COMMENT ON COLUMN refresh_sessions.replaced_by_session_id IS 'Следующая сессия после успешной rotation; NULL до замены';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS refresh_sessions;

-- +goose StatementEnd

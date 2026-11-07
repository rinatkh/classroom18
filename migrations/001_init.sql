-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text NOT NULL CONSTRAINT users_email_key UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id   bigint NOT NULL,
    title      text NOT NULL,
    done       boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tasks_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT tasks_title_not_blank CHECK (btrim(title) <> '')
);

-- Тот же query pattern, что в MemoryRepository урока 13:
-- WHERE owner_id = $1 ORDER BY id ASC. Не меняем порядок API ради нового индекса.
CREATE INDEX tasks_owner_id_id_idx ON tasks (owner_id, id);

COMMENT ON TABLE users IS 'Пользователи существующего Task API';
COMMENT ON COLUMN users.email IS 'Нормализованный lower-case email, уникальный в приложении';
COMMENT ON COLUMN users.password_hash IS 'Только hash от PasswordHasher, не plaintext password';
COMMENT ON TABLE tasks IS 'Задачи существующего Task API, принадлежащие пользователю';
COMMENT ON COLUMN tasks.owner_id IS 'Владелец Task; authorization проверяется в UseCase';
COMMENT ON COLUMN tasks.updated_at IS 'Обновляется SQL-операцией Update Repository; trigger — отдельный optional lab';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Сначала дочерняя таблица, затем родительская. Индекс удалится вместе с tasks.
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS users;

-- +goose StatementEnd

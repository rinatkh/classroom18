-- +goose Up
-- +goose StatementBegin
ALTER TABLE tasks ADD COLUMN processing_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (processing_status IN ('pending', 'processed'));
COMMENT ON COLUMN tasks.processing_status IS 'Фоновая обработка, независимая от пользовательского done';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tasks DROP COLUMN IF EXISTS processing_status;
-- +goose StatementEnd

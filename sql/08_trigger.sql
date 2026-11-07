-- Дополнительный материал: production migration не устанавливает trigger.
BEGIN;
\ir _seed.sql
CREATE FUNCTION lab_touch_task_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at=clock_timestamp();
    RETURN NEW;
END;
$$;
CREATE TRIGGER lab_task_updated_at BEFORE UPDATE ON tasks
FOR EACH ROW EXECUTE FUNCTION lab_touch_task_updated_at();
-- Здесь updated_at меняется database-side, хотя UPDATE явно его не указывает.
UPDATE tasks SET title=title
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-alice@example.test')
RETURNING id,updated_at;
-- Убираются и function/trigger, и данные опыта.
ROLLBACK;

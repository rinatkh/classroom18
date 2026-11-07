BEGIN;
\ir _seed.sql
-- INSERT ... RETURNING получает ID и DEFAULT, назначенные базой.
INSERT INTO tasks(owner_id,title)
SELECT id,'INSERT from psql' FROM users WHERE email='sql-lab-alice@example.test'
RETURNING id,owner_id,title,done,created_at,updated_at;
SELECT id,email FROM users WHERE email LIKE 'sql-lab-%' ORDER BY id;
-- Для постоянного сохранения транзакции нужен COMMIT; этот опыт убирает свои данные.
ROLLBACK;

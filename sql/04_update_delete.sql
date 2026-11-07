BEGIN;
\ir _seed.sql
UPDATE tasks SET done=true,updated_at=now()
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-alice@example.test')
  AND title='SQL lab: first'
RETURNING id,title,done,updated_at;
DELETE FROM tasks
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-bob@example.test')
RETURNING id,title;
-- На API ownership всё ещё проверяет UseCase. Здесь работаем администратором через SQL.
ROLLBACK;

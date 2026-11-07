BEGIN;
\ir _seed.sql
SAVEPOINT before_change;
UPDATE tasks SET title='transaction draft'
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-alice@example.test')
  AND title='SQL lab: first';
SELECT id,title FROM tasks WHERE title='transaction draft';
ROLLBACK TO SAVEPOINT before_change;
SELECT id,title FROM tasks
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-alice@example.test')
ORDER BY id;
-- COMMIT закрепил бы успешную транзакцию. В лаборатории показываем полный откат.
ROLLBACK;

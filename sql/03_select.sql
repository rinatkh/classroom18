BEGIN;
\ir _seed.sql
\pset null '(NULL)'
SELECT NULL = NULL AS unknown_boolean, NULL IS NULL AS is_null;
SELECT pg_typeof(id),pg_typeof(owner_id),pg_typeof(title),pg_typeof(done),pg_typeof(created_at)
FROM tasks LIMIT 1;
-- Именно такой query pattern уже есть в Task Repository.
SELECT id,title,done
FROM tasks
WHERE owner_id=(SELECT id FROM users WHERE email='sql-lab-alice@example.test')
ORDER BY id ASC;
SELECT id,title FROM tasks WHERE done=false ORDER BY id;
ROLLBACK;

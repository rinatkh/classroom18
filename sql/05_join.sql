BEGIN;
\ir _seed.sql
-- Естественная связь из урока 13: один User владеет многими Task.
SELECT t.id,t.title,u.email
FROM tasks t
JOIN users u ON u.id=t.owner_id
WHERE u.email IN ('sql-lab-alice@example.test','sql-lab-bob@example.test')
ORDER BY t.id;
ROLLBACK;

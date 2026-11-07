BEGIN;
-- Создаём историю нескольких пользователей; всё откатится после EXPLAIN.
INSERT INTO users(email,password_hash)
SELECT 'sql-index-user-'||n||'@example.test','$2a$04$QF1evtslwnuNLCts9sksXul0sitJbs4PMmvnA7v01tLZEofBdSd5e'
FROM generate_series(1,200) n
ON CONFLICT (email) DO NOTHING;
INSERT INTO tasks(owner_id,title,done)
SELECT u.id,'history '||n,true
FROM users u CROSS JOIN generate_series(1,50) n
WHERE u.email LIKE 'sql-index-user-%@example.test';
ANALYZE users;
ANALYZE tasks;

-- (owner_id,id) возник из WHERE owner_id + прежнего ORDER BY id ASC.
EXPLAIN (ANALYZE,BUFFERS)
SELECT id,owner_id,title,done,created_at,updated_at
FROM tasks
WHERE owner_id=(SELECT id FROM users WHERE email='sql-index-user-42@example.test')
ORDER BY id ASC;
-- На маленьких таблицах Seq Scan может быть дешевле: индекс не обещание конкретного plan.
ROLLBACK;

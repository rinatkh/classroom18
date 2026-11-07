-- Общие данные только внутри BEGIN/ROLLBACK вызывающего SQL lab.
-- Это настоящий заранее вычисленный bcrypt hash учебного sql-lab-password.
-- В HTTP-потоке hash по-прежнему создаёт неизменённый Register из занятия 13.
INSERT INTO users(email,password_hash)
VALUES ('sql-lab-alice@example.test','$2a$04$QF1evtslwnuNLCts9sksXul0sitJbs4PMmvnA7v01tLZEofBdSd5e'),
       ('sql-lab-bob@example.test','$2a$04$QF1evtslwnuNLCts9sksXul0sitJbs4PMmvnA7v01tLZEofBdSd5e')
ON CONFLICT (email) DO NOTHING;

INSERT INTO tasks(owner_id,title,done)
SELECT id,'SQL lab: first',false FROM users WHERE email='sql-lab-alice@example.test';
INSERT INTO tasks(owner_id,title,done)
SELECT id,'SQL lab: completed',true FROM users WHERE email='sql-lab-alice@example.test';
INSERT INTO tasks(owner_id,title,done)
SELECT id,'SQL lab: Bob',false FROM users WHERE email='sql-lab-bob@example.test';

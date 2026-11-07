-- Выполнять после migrations003 и полного demo.
SELECT id,owner_id,title,done,processing_status FROM tasks ORDER BY id;
SELECT to_regclass('refresh_sessions'); -- SQL table сохранена; активный session adapter теперь Redis.

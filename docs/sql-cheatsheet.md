# SQL reference занятия 14

Сохранён как справочник. Сегодня добавляем только migration002 и SQL10.

# SQL basics на users/tasks

| Понятие | Пример в нашем приложении |
|---|---|
| database | go_course_tasks |
| schema | public, namespace таблиц внутри database |
| table / row / column | users / одна запись User / email |
| bigint / text / boolean / timestamptz | ID / title / done / created_at |
| NULL | отсутствие значения; сравниваем IS NULL, не = NULL |
| PRIMARY KEY | уникальный непустой users.id или tasks.id |
| UNIQUE | users.email, уже нормализованный приложением |
| FOREIGN KEY | tasks.owner_id → users.id |
| NOT NULL | не разрешает отсутствие обязательного значения |
| CHECK | btrim(title) <> '' — защита таблицы даже от прямого INSERT |
| DEFAULT | done=false, timestamps=now() |

PK/UNIQUE создают свои indexes. FK — это целостность ссылки, а не готовый
index на любую комбинацию колонок. Выбранный tasks(owner_id,id) обслуживает
реальный список владельца в прежнем id ASC порядке.

## SQL операции

```sql
INSERT INTO users(email,password_hash) VALUES($1,$2) RETURNING id;
SELECT id,email,password_hash FROM users WHERE email=$1;
SELECT id,title,done FROM tasks WHERE owner_id=$1 ORDER BY id ASC;
UPDATE tasks SET title=COALESCE($2,title),done=COALESCE($3,done),updated_at=now()
WHERE id=$1 RETURNING id,title,done;
DELETE FROM tasks WHERE id=$1;
SELECT t.id,t.title,u.email FROM tasks t JOIN users u ON u.id=t.owner_id;
```

$1 в Go query — параметр, аргументы передаются отдельно. В интерактивном psql
без PREPARE используйте конкретные значения либо готовые sql labs: нельзя
скопировать query с $1 и ожидать, что psql сам угадает аргумент.

## pgx

| API | Когда | Важная проверка |
|---|---|---|
| QueryRow(ctx,sql,args...).Scan(...) | одна строка / RETURNING / COUNT | ошибка приходит из Scan; ErrNoRows для отсутствующей строки |
| Query(ctx,sql,args...) | несколько строк | Next/Scan, Close, затем Rows.Err |
| Exec(ctx,sql,args...) | строки ответа не нужны | ошибка + CommandTag.RowsAffected |

0 строк списка — корректный пустой результат []. DELETE 0 rows превращаем в
ErrNotFound, чтобы сохранить 404. SQL text не строим из email/title через fmt.

## Transactions / EXPLAIN / trigger

BEGIN объединяет шаги; COMMIT сохраняет; ROLLBACK отменяет. SAVEPOINT позволяет
откатить часть. Identity/sequence numbers не обязаны быть непрерывными после
rollback или ошибок. Отдельный pgx.Tx реализует инфраструктурный DBTX для lab.

EXPLAIN показывает plan, EXPLAIN ANALYZE выполняет запрос и показывает реальные
измерения. На маленькой таблице planner может выбрать Seq Scan: это не поломка.
Смотрите WHERE/order/row estimates, а не добавляйте indexes каждому полю.

Trigger — код, исполняемый БД, не UseCase. В основной схеме его нет; optional
08_trigger.sql показывает и откатывает его внутри одного опыта.

Источники: [pgx 5.7.2](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.7.2),
[constraints](https://www.postgresql.org/docs/16/ddl-constraints.html),
[multicolumn indexes](https://www.postgresql.org/docs/16/indexes-multicolumn.html),
[transactions](https://www.postgresql.org/docs/16/tutorial-transactions.html).

\echo 'Database/schema/types/constraints'
\ir 01_inspect_schema.sql
\echo 'INSERT / DEFAULT / RETURNING'
\ir 02_insert.sql
\echo 'SELECT / WHERE / NULL / types'
\ir 03_select.sql
\echo 'UPDATE / DELETE'
\ir 04_update_delete.sql
\echo 'User 1:N Task / JOIN'
\ir 05_join.sql
\echo 'Query pattern / index / EXPLAIN'
\ir 06_index_explain.sql
\echo 'BEGIN / SAVEPOINT / ROLLBACK'
\ir 07_transaction.sql
\echo 'Core SQL labs PASS; optional trigger: sql/08_trigger.sql'

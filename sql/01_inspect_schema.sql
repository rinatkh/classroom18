-- database → schema → table → row/column: смотрим существующий домен.
SELECT current_database() AS database_name, current_schema() AS schema_name, version();
\d users
\d tasks
SELECT table_name,column_name,data_type,is_nullable,column_default
FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name IN ('users','tasks')
ORDER BY table_name,ordinal_position;
SELECT table_name,constraint_name,constraint_type
FROM information_schema.table_constraints
WHERE table_schema=current_schema() AND table_name IN ('users','tasks')
ORDER BY table_name,constraint_type;

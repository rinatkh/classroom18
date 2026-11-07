#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?Задайте DATABASE_URL для учебной БД}"
pg_dump_bin="${PG_DUMP:-pg_dump}"

# Сначала пишем во временный файл: ошибка подключения не должна стереть
# уже сохранённый structure.sql. Данные пользователей и пароль подключения
# в schema-only dump не включаются; owners/ACL не привязывают файл к роли машины.
tmp_file="$(mktemp structure.sql.tmp.XXXXXX)"
trap 'rm -f "$tmp_file"' EXIT
"$pg_dump_bin" --schema-only --no-owner --no-privileges \
  --schema=public --dbname="$DATABASE_URL" >"$tmp_file"
# Убираем только пустые строки в конце, не меняя SQL или защитные команды pg_dump.
python3 -c 'import pathlib,sys; p=pathlib.Path(sys.argv[1]); p.write_bytes(p.read_bytes().rstrip(b"\r\n")+b"\n")' "$tmp_file"
mv "$tmp_file" structure.sql
echo 'structure.sql обновлён'

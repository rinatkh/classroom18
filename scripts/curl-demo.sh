#!/usr/bin/env bash
set -euo pipefail
umask 077

BASE_URL="${BASE_URL:-http://localhost:8080}"
DEMO_TMP="$(mktemp -d)"
trap 'rm -rf "$DEMO_TMP"' EXIT

LAST_BODY="$DEMO_TMP/body"
LAST_HEADERS="$DEMO_TMP/headers"

request() {
  local title="$1"
  local expected="$2"
  shift 2

  local status
  status="$(curl -sS -D "$LAST_HEADERS" -o "$LAST_BODY" -w '%{http_code}' "$@")"
  if [[ "$status" != "$expected" ]]; then
    echo "FAIL: $title: status=$status, want=$expected" >&2
    echo "Response hidden to protect credentials" >&2
    exit 1
  fi
  echo "OK: $title -> $status" >&2
}

register_and_login() {
  local email="$1"

  request "register $email" 201 -X POST "$BASE_URL/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"secret123\"}"
  if grep -Eqi 'password|hash' "$LAST_BODY"; then
    echo 'FAIL: register response exposes password data' >&2
    exit 1
  fi

  request "login $email" 200 -c "$DEMO_TMP/$email.cookies" -X POST "$BASE_URL/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"secret123\"}"
  python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])' <"$LAST_BODY"
}

suffix="${RANDOM}-$$"
alice_email="alice-${suffix}@example.com"
bob_email="bob-${suffix}@example.com"

request 'health' 200 "$BASE_URL/health"
request 'protected endpoint without token' 401 "$BASE_URL/tasks"
if ! tr -d '\r' <"$LAST_HEADERS" | grep -qi '^WWW-Authenticate: Bearer$'; then
  echo 'FAIL: 401 response has no WWW-Authenticate: Bearer' >&2
  exit 1
fi

alice_token="$(register_and_login "$alice_email")"
bob_token="$(register_and_login "$bob_email")"

request 'Alice creates Task' 201 -X POST "$BASE_URL/tasks" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $alice_token" \
  -d '{"title":"Private task"}'
task_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$LAST_BODY")"

request 'Bob cannot read Alice Task' 403 "$BASE_URL/tasks/$task_id" \
  -H "Authorization: Bearer $bob_token"

request 'Bob cannot change Alice Task' 403 -X PATCH "$BASE_URL/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $bob_token" \
  -d '{"done":true}'

request 'Bob cannot delete Alice Task' 403 -X DELETE "$BASE_URL/tasks/$task_id" \
  -H "Authorization: Bearer $bob_token"

request 'Bob list contains only Bob Tasks' 200 "$BASE_URL/tasks" \
  -H "Authorization: Bearer $bob_token"
python3 -c 'import json,sys; assert json.load(sys.stdin) == []' <"$LAST_BODY"

request 'Alice reads her Task' 200 "$BASE_URL/tasks/$task_id" \
  -H "Authorization: Bearer $alice_token"

request 'Alice updates her Task' 200 -X PATCH "$BASE_URL/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $alice_token" \
  -d '{"title":"Updated"}'

python3 -c 'import json,sys; v=json.load(sys.stdin); assert v["title"] == "Updated" and v["done"] is False and set(v) == {"id","title","done"}' <"$LAST_BODY"

request 'Alice completes her Task' 200 -X PATCH "$BASE_URL/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $alice_token" \
  -d '{"done":true}'
python3 -c 'import json,sys; v=json.load(sys.stdin); assert v["title"] == "Updated" and v["done"] is True' <"$LAST_BODY"

request 'Repeated completion remains a business conflict' 409 -X PATCH "$BASE_URL/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $alice_token" \
  -d '{"done":true}'

request 'Alice reopens her Task with explicit false' 200 -X PATCH "$BASE_URL/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $alice_token" \
  -d '{"done":false}'

request 'Alice deletes her Task' 204 -X DELETE "$BASE_URL/tasks/$task_id" \
  -H "Authorization: Bearer $alice_token"
if [[ -s "$LAST_BODY" ]]; then
  echo 'FAIL: 204 response must have an empty body' >&2
  exit 1
fi

request 'deleted Task is missing' 404 "$BASE_URL/tasks/$task_id" \
  -H "Authorization: Bearer $alice_token"

request 'Alice logout' 204 -X POST "$BASE_URL/auth/logout" -b "$DEMO_TMP/$alice_email.cookies"
request 'Bob logout' 204 -X POST "$BASE_URL/auth/logout" -b "$DEMO_TMP/$bob_email.cookies"
echo 'curl demo: PASS'

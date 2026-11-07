#!/usr/bin/env bash
set -euo pipefail
umask 077
BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTOMATION_URL="${AUTOMATION_URL:-http://localhost:8090}"
DEMO_TMP="$(mktemp -d)"
trap 'rm -rf "$DEMO_TMP"' EXIT
body="$DEMO_TMP/body"
jar="$DEMO_TMP/cookies"
request() {
  local expected="$1"; shift
  local code
  code="$(curl --max-time 10 -sS -o "$body" -w '%{http_code}' "$@")"
  if [[ "$code" != "$expected" ]]; then echo "expected $expected, got $code" >&2; exit 1; fi
}
field() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$body" "$1"; }
email="infra-demo-$(date +%s)-$RANDOM@example.test"
credentials="$(python3 -c 'import json,sys;print(json.dumps({"email":sys.argv[1],"password":"demo-password"}))' "$email")"
request 201 -X POST "$BASE_URL/register" -H 'Content-Type: application/json' -d "$credentials"
user_id="$(field id)"
request 200 -c "$jar" -X POST "$BASE_URL/login" -H 'Content-Type: application/json' -d "$credentials"
access="$(field access_token)"
cp "$jar" "$DEMO_TMP/old-cookies"
request 200 -b "$jar" -c "$jar" -X POST "$BASE_URL/auth/refresh"
access="$(field access_token)"
request 401 -b "$DEMO_TMP/old-cookies" -X POST "$BASE_URL/auth/refresh"
echo 'OK: HTTP Login / Redis rotation / old refresh 401'
request 201 -X POST "$AUTOMATION_URL/automation/tasks" -H 'Content-Type: application/json' -d "{\"user_id\":$user_id,\"title\":\"Learn distributed Task processing\"}"
task_id="$(field id)"
echo "OK: automation-service gRPC CreateTask -> Task $task_id"
for ((attempt=0; attempt<150; attempt++)); do
  request 200 "$AUTOMATION_URL/automation/tasks/$task_id?user_id=$user_id"
  if [[ "$(field processing_status)" == 'processed' ]]; then break; fi
  sleep 0.2
done
[[ "$(field processing_status)" == 'processed' ]] || { echo 'worker result timeout' >&2; exit 1; }
echo 'OK: task.created -> worker analysis -> task.processed -> API -> PostgreSQL'
request 200 "$BASE_URL/tasks/$task_id" -H "Authorization: Bearer $access"
python3 -c 'import json,sys;v=json.load(open(sys.argv[1]));assert set(v)=={"id","title","done"} and v["done"] is False' "$body"
request 204 -b "$jar" -X POST "$BASE_URL/auth/logout"
request 401 -b "$jar" -X POST "$BASE_URL/auth/refresh"
request 200 "$BASE_URL/tasks/$task_id" -H "Authorization: Bearer $access"
request 401 "$BASE_URL/tasks"
echo 'OK: unchanged HTTP DTO / logout / stateless access / missing access 401'
echo "Inspect tasks.id=$task_id in PostgreSQL, Kafka console and each process log."
echo 'infrastructure demo: PASS'

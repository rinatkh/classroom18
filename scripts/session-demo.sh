#!/usr/bin/env bash
set -euo pipefail
umask 077
BASE_URL="${BASE_URL:-http://localhost:8080}"
DEMO_DIR="$(mktemp -d)"
trap 'rm -rf "$DEMO_DIR"' EXIT
BODY="$DEMO_DIR/body.json"
HEADERS="$DEMO_DIR/headers"
JAR="$DEMO_DIR/current.jar"
OLD_JAR="$DEMO_DIR/old.jar"

request() {
  local title="$1" expected="$2" status
  shift 2
  status="$(curl -sS -D "$HEADERS" -o "$BODY" -w '%{http_code}' "$@")"
  if [[ "$status" != "$expected" ]]; then
    echo "FAIL: $title -> $status (want $expected); response hidden to protect credentials" >&2
    exit 1
  fi
  echo "OK: $title -> $status"
}
access() { python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])' <"$BODY"; }

email="sessions-${RANDOM}-$$@example.test"
request 'Register' 201 -X POST "$BASE_URL/register" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"secret123\"}"
request 'Login and save HttpOnly cookie jar' 200 -c "$JAR" -X POST "$BASE_URL/login" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$email\",\"password\":\"secret123\"}"
ACCESS="$(access)"
EXPIRY="$(python3 -c 'import base64,json,sys; p=sys.argv[1].split(".")[1]; print(json.loads(base64.urlsafe_b64decode(p+"="*(-len(p)%4)))["exp"])' "$ACCESS")"
TTL="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["expires_in"])' <"$BODY")"
if (( TTL > 30 )); then
  echo 'Для полного expiration demo перезапустите API с ACCESS_TOKEN_TTL=20s (не более 30s).' >&2
  curl -sS -o /dev/null -b "$JAR" -X POST "$BASE_URL/auth/logout"
  exit 1
fi
python3 - "$HEADERS" "$BODY" <<'PY'
import json,sys
headers=open(sys.argv[1]).read().lower()
body=json.load(open(sys.argv[2]))
assert set(body)=={'access_token','token_type','expires_in'} and body['token_type']=='Bearer'
assert 'httponly' in headers and 'path=/auth' in headers and 'samesite=lax' in headers and 'cache-control: no-store' in headers
PY
cp "$JAR" "$OLD_JAR"
request 'Create familiar Task' 201 -X POST "$BASE_URL/tasks" -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' -d '{"title":"Refresh lesson"}'
TASK_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <"$BODY")"
request 'Protected request with access' 200 "$BASE_URL/tasks/$TASK_ID" -H "Authorization: Bearer $ACCESS"
request 'Refresh cookie alone cannot open Tasks' 401 "$BASE_URL/tasks" -b "$JAR"
request 'Refresh without cookie' 401 -X POST "$BASE_URL/auth/refresh"
echo 'Waiting for the short demo access token to expire...'
python3 - "$EXPIRY" <<'PY'
import sys,time
# Deliberate runtime demonstration only. Unit expiration tests never sleep.
time.sleep(max(0,int(sys.argv[1])-time.time()+0.3))
PY
request 'Expired access' 401 "$BASE_URL/tasks" -H "Authorization: Bearer $ACCESS"
request 'Refresh and rotate cookie automatically' 200 -X POST "$BASE_URL/auth/refresh" -b "$JAR" -c "$JAR"
ACCESS="$(access)"
python3 - "$OLD_JAR" "$JAR" <<'PY'
import http.cookiejar,sys
jars=[]
for p in sys.argv[1:]:
 jar=http.cookiejar.MozillaCookieJar(p);jar.load(ignore_discard=True,ignore_expires=True);jars.append(list(jar))
assert len(jars[0])==len(jars[1])==1 and jars[0][0].value!=jars[1][0].value
PY
request 'Retry familiar protected request' 200 "$BASE_URL/tasks/$TASK_ID" -H "Authorization: Bearer $ACCESS"
request 'Replay OLD refresh rejected' 401 -X POST "$BASE_URL/auth/refresh" -b "$OLD_JAR"
cp "$JAR" "$DEMO_DIR/revoked.jar"
request 'Logout revokes current session and clears cookie' 204 -X POST "$BASE_URL/auth/logout" -b "$JAR" -c "$JAR"
python3 - "$HEADERS" <<'PY'
import sys
h=open(sys.argv[1]).read().lower()
assert 'max-age=0' in h and 'path=/auth' in h and 'expires=thu, 01 jan 1970' in h
PY
request 'Refresh with revoked credential' 401 -X POST "$BASE_URL/auth/refresh" -b "$DEMO_DIR/revoked.jar"
request 'Refresh after clearing browser jar' 401 -X POST "$BASE_URL/auth/refresh" -b "$JAR"
request 'Stateless access remains valid until exp after logout' 200 "$BASE_URL/tasks/$TASK_ID" -H "Authorization: Bearer $ACCESS"
request 'Task DELETE still works' 204 -X DELETE "$BASE_URL/tasks/$TASK_ID" -H "Authorization: Bearer $ACCESS"
echo 'session demo: PASS'

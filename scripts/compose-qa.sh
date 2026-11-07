#!/usr/bin/env bash
# Run on a Docker host with classroom host ports free. Always uses its own new project/volumes.
set -euo pipefail
cd "$(dirname "$0")/.."
python3 scripts/create-env.py
project="classroom18-qa-$(date +%s)-$RANDOM"
compose=(docker compose --project-name "$project" --env-file .env -f deploy/compose.yaml)
cleanup() { "${compose[@]}" down -v --remove-orphans; }
trap cleanup EXIT
"${compose[@]}" config -q
"${compose[@]}" up --build -d --wait --wait-timeout 180
"${compose[@]}" ps -a
bash scripts/demo.sh
state_before="$("${compose[@]}" exec -T postgres psql -U postgres -d task_api -At -c "SELECT (SELECT count(*) FROM users)||':'||(SELECT count(*) FROM tasks)")"
"${compose[@]}" restart api-service
for ((i=0;i<100;i++)); do
  if curl -fsS http://localhost:8080/health >/dev/null; then break; fi
  sleep 0.2
done
"${compose[@]}" exec -T postgres psql -U postgres -d task_api -v ON_ERROR_STOP=1 \
  -c "SELECT id,owner_id,title,done,processing_status FROM tasks ORDER BY id;"
# down/up with same volumes proves row persistence. Redis sessions are deliberately temporary.
"${compose[@]}" down
"${compose[@]}" up -d --wait --wait-timeout 180
"${compose[@]}" exec -T postgres psql -U postgres -d task_api -v ON_ERROR_STOP=1 \
  -c "SELECT count(*) AS preserved_tasks FROM tasks;"
state_after="$("${compose[@]}" exec -T postgres psql -U postgres -d task_api -At -c "SELECT (SELECT count(*) FROM users)||':'||(SELECT count(*) FROM tasks)")"
[[ "$state_before" == "$state_after" ]] || { echo 'users/tasks count changed on recreation' >&2; exit 1; }
bash scripts/demo.sh
echo 'compose cold boot / restart / recreate QA: PASS'

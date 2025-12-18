#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
mkdir -p "$BIN_DIR"

cleanup() {
  set +e
  echo "[cleanup] stopping services and docker postgres"
  if [[ -f "$BIN_DIR/user-service.pid" ]]; then
    kill "$(cat "$BIN_DIR/user-service.pid")" 2>/dev/null || true
    rm -f "$BIN_DIR/user-service.pid"
  fi
  if [[ -f "$BIN_DIR/auth-service.pid" ]]; then
    kill "$(cat "$BIN_DIR/auth-service.pid")" 2>/dev/null || true
    rm -f "$BIN_DIR/auth-service.pid"
  fi
  docker rm -f e2e-postgres >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[e2e] verifying prerequisites (docker, go)"
command -v docker >/dev/null || { echo "docker is required"; exit 1; }
command -v go >/dev/null || { echo "go is required"; exit 1; }

echo "[e2e] starting postgres container"
docker rm -f e2e-postgres >/dev/null 2>&1 || true
docker run -d --name e2e-postgres \
  -e POSTGRES_USER=appuser \
  -e POSTGRES_PASSWORD=apppass \
  -e POSTGRES_DB=appdb \
  -p 5433:5432 \
  postgres:16-alpine >/dev/null

echo "[e2e] waiting for postgres readiness"
for i in {1..60}; do
  if docker exec e2e-postgres pg_isready -U appuser -d appdb -h 127.0.0.1 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[e2e] applying DB migrations"
docker exec -i e2e-postgres psql -U appuser -d appdb -v ON_ERROR_STOP=1 -f - < "$ROOT_DIR/user-service/migrations/001_init.sql"

echo "[e2e] building services"
GOFLAGS=${GOFLAGS:-}
pushd "$ROOT_DIR/user-service" >/dev/null
go build $GOFLAGS -o "$BIN_DIR/user-service" ./cmd/server
popd >/dev/null

pushd "$ROOT_DIR/auth-service" >/dev/null
go build $GOFLAGS -o "$BIN_DIR/auth-service" ./cmd/server
popd >/dev/null

echo "[e2e] starting user-service on :8081"
export DATABASE_URL="postgresql://appuser:apppass@localhost:5433/appdb?sslmode=disable"
ADDR=":8081" "$BIN_DIR/user-service" >/tmp/user-service.log 2>&1 & echo $! > "$BIN_DIR/user-service.pid"

echo "[e2e] starting auth-service on :8082"
export USER_SERVICE_URL="http://localhost:8081"
export JWT_SECRET="dev-secret"
ADDR=":8082" "$BIN_DIR/auth-service" >/tmp/auth-service.log 2>&1 & echo $! > "$BIN_DIR/auth-service.pid"

echo "[e2e] waiting for services"
for i in {1..60}; do
  u=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/healthz || true)
  a=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/healthz || true)
  if [[ "$u" == "200" && "$a" == "200" ]]; then
    echo "[e2e] services are healthy"
    break
  fi
  sleep 1
done

# Verify services are actually healthy
u=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/healthz || true)
a=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/healthz || true)
if [[ "$u" != "200" ]] || [[ "$a" != "200" ]]; then
  echo "[e2e] ERROR: services failed to start (user: $u, auth: $a)"
  echo "[e2e] user-service logs:"
  cat /tmp/user-service.log || true
  echo "[e2e] auth-service logs:"
  cat /tmp/auth-service.log || true
  exit 1
fi

echo "[e2e] create user"
CREATE_STATUS=$(curl -s -o /tmp/create.json -w "%{http_code}" -H 'Content-Type: application/json' \
  -d '{"username":"ci-user","password":"ci-pass-123"}' http://localhost:8081/users)
cat /tmp/create.json
[[ "$CREATE_STATUS" == "200" ]] || { echo "create user failed: $CREATE_STATUS"; exit 1; }

echo "[e2e] login"
LOGIN_STATUS=$(curl -s -o /tmp/login.json -w "%{http_code}" -H 'Content-Type: application/json' \
  -d '{"username":"ci-user","password":"ci-pass-123"}' http://localhost:8082/login)
cat /tmp/login.json
[[ "$LOGIN_STATUS" == "200" ]] || { echo "login failed: $LOGIN_STATUS"; exit 1; }
grep -q 'token' /tmp/login.json || { echo "token not found in login response"; exit 1; }

echo "[e2e] SUCCESS"
exit 0

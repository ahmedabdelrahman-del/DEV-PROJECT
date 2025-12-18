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
echo "[e2e] waiting for container to be fully started"
sleep 3

# Get the container IP for direct connection
POSTGRES_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' e2e-postgres)
echo "[e2e] postgres container IP: $POSTGRES_IP"

echo "[e2e] waiting for postgres readiness (inside container)"
for i in {1..60}; do
  if docker exec e2e-postgres pg_isready -U appuser -d appdb -h 127.0.0.1 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[e2e] verifying postgres accessible from host on port 5433"
for i in {1..30}; do
  if docker exec e2e-postgres pg_isready -U appuser -d appdb -h 0.0.0.0 -p 5432 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Final verification that we can connect from the host
if ! docker exec e2e-postgres pg_isready -U appuser -d appdb -h 0.0.0.0 -p 5432 >/dev/null 2>&1; then
  echo "[e2e] ERROR: postgres not accepting connections"
  docker logs e2e-postgres || true
  exit 1
fi

echo "[e2e] applying DB migrations"
docker exec -i e2e-postgres psql -U appuser -d appdb -v ON_ERROR_STOP=1 -f - < "$ROOT_DIR/user-service/migrations/001_init.sql"

echo "[e2e] verifying port 5433 is listening on host"
netstat -tuln | grep 5433 || ss -tuln | grep 5433 || echo "WARNING: port 5433 not found in netstat/ss"
docker ps | grep e2e-postgres

echo "[e2e] testing connection from host to postgres"
if command -v psql >/dev/null; then
  PGPASSWORD=apppass psql -h localhost -p 5433 -U appuser -d appdb -c "SELECT 1" || echo "WARNING: psql connection test failed"
fi

echo "[e2e] building services"
GOFLAGS=${GOFLAGS:-}
pushd "$ROOT_DIR/user-service" >/dev/null
go build $GOFLAGS -o "$BIN_DIR/user-service" ./cmd/server
popd >/dev/null

pushd "$ROOT_DIR/auth-service" >/dev/null
go build $GOFLAGS -o "$BIN_DIR/auth-service" ./cmd/server
popd >/dev/null

echo "[e2e] starting user-service on :8081"
# Try localhost first, fallback to container IP if localhost doesn't work
export DATABASE_URL="postgresql://appuser:apppass@${POSTGRES_IP}:5432/appdb?sslmode=disable"
echo "[e2e] DATABASE_URL=${DATABASE_URL}"
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

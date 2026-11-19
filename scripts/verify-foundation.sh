#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
scratch=$(mktemp -d "${TMPDIR:-/tmp}/outguessed-validation.XXXXXXXX")
suffix=${scratch##*.}
project="outguessed-validation-${suffix,,}"
compose=(docker compose --env-file .env.example --project-name "$project"
    -f compose.yaml -f "$scratch/compose.yaml" --profile '*')
api_pid=
paused_service=
compose_started=false

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

expect() {
    local expected=$1 actual=$2 check=$3
    [[ $actual == "$expected" ]] || fail "$check ($actual; esperado $expected)"
}

stop_api() {
    local deadline=$((SECONDS + 8)) result=0
    kill -TERM "$api_pid" 2>/dev/null || true
    while kill -0 "$api_pid" 2>/dev/null; do
        if ((SECONDS >= deadline)); then
            printf '%s\n' 'API excedeu o prazo de encerramento' >&2
            kill -KILL "$api_pid" 2>/dev/null || true
            break
        fi
        sleep 0.1
    done
    wait "$api_pid" || result=$?
    api_pid=
    return "$result"
}

cleanup() {
    local result=$?
    if [[ -n $api_pid ]]; then
        stop_api || result=1
    fi
    if $compose_started; then
        if [[ -n $paused_service ]]; then
            "${compose[@]}" unpause "$paused_service" || true
        fi
        "${compose[@]}" down --volumes --remove-orphans || result=1
    fi
    if ((result != 0)) && [[ -f $scratch/api.log ]]; then
        tail -n 50 "$scratch/api.log" >&2
    fi
    rm -rf -- "$scratch"
    exit "$result"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cat > "$scratch/compose.yaml" <<'YAML'
services:
  postgres:
    ports: !override []
  redis:
    ports: !override []
  postgres-test:
    ports: !override ["127.0.0.1::5432"]
  redis-test:
    ports: !override ["127.0.0.1::6379"]
  swagger:
    ports: !override ["127.0.0.1::8080"]
YAML

printf '%s\n' 'Reproduzir o código gerado pelo sqlc'
compose_started=true
"${compose[@]}" run --rm --no-deps --user "$(id -u):$(id -g)" sqlc generate
changes=$(git status --porcelain --untracked-files=all --ignored=matching -- go.mod go.sum internal/postgres/dbgen)
[[ -z $changes ]] || fail "A geração ou as ferramentas alteraram arquivos versionados ou criaram arquivos inesperados:
$changes"

printf '%s\n' 'Aplicar, reaplicar, reverter e restaurar a migration inicial'
"${compose[@]}" up -d --wait --wait-timeout 120 postgres-test redis-test swagger
migrate=("${compose[@]}" run --rm --no-deps -T migrate-test)
test_sql=("${compose[@]}" exec -T postgres-test psql -XAt -v ON_ERROR_STOP=1 -U outguessed_test -d outguessed_test)
schema_query="SELECT to_regnamespace('outguessed') IS NOT NULL"
expect f "$("${test_sql[@]}" -c "$schema_query")" 'Schema antes da migration'
"${migrate[@]}" up
expect t "$("${test_sql[@]}" -c "$schema_query")" 'Schema após a migration'
reapply=$("${migrate[@]}" up 2>&1)
printf '%s\n' "$reapply"
[[ $reapply == *'no change'* ]] || fail 'A reaplicação deve confirmar que não há migrations pendentes'
"${migrate[@]}" down 1
expect f "$("${test_sql[@]}" -c "$schema_query")" 'Schema após rollback'
"${migrate[@]}" up
expect t "$("${test_sql[@]}" -c "$schema_query")" 'Schema após nova aplicação'
expect 1:false "$("${test_sql[@]}" -c "SELECT version::text || ':' || dirty::text FROM public.schema_migrations")" 'Versão da migration e flag dirty'

postgres_addr=$("${compose[@]}" port postgres-test 5432)
redis_addr=$("${compose[@]}" port redis-test 6379)
swagger_addr=$("${compose[@]}" port swagger 8080)
export TEST_DATABASE_URL="postgres://outguessed_test:outguessed_test_local@$postgres_addr/outguessed_test?sslmode=disable"
export TEST_REDIS_URL="redis://$redis_addr/0"
swagger_origin="http://$swagger_addr"

printf '%s\n' 'Executar testes de integração com detector de corridas'
go test -race -count=1 -tags=integration ./...

printf '%s\n' 'Iniciar o executável e verificar a recuperação das dependências'
go build -o "$scratch/api" ./cmd/api
APP_ENV=test HTTP_ADDR=127.0.0.1:0 DATABASE_URL="$TEST_DATABASE_URL" REDIS_URL="$TEST_REDIS_URL" \
    CORS_ALLOWED_ORIGINS="$swagger_origin" LOG_LEVEL=info STARTUP_TIMEOUT=10s DEPENDENCY_TIMEOUT=1s \
    SHUTDOWN_TIMEOUT=5s "$scratch/api" > "$scratch/api.log" 2>&1 &
api_pid=$!

api_addr=
deadline=$((SECONDS + 15))
while [[ -z $api_addr ]] && ((SECONDS < deadline)); do
    kill -0 "$api_pid" 2>/dev/null || fail 'A API encerrou durante a inicialização'
    api_addr=$(jq -Rr 'fromjson? | select(.msg == "Servidor HTTP iniciado") | .addr' "$scratch/api.log")
    if [[ -z $api_addr ]]; then
        sleep 0.1
    fi
done
[[ $api_addr =~ ^127\.0\.0\.1:[0-9]+$ ]] || fail 'Endereço HTTP local não encontrado nos logs'
api_url="http://$api_addr"

check_health() {
    local path=$1 expected=$2 deadline=$((SECONDS + 15)) status=
    while ((SECONDS < deadline)); do
        kill -0 "$api_pid" 2>/dev/null || fail 'O processo original da API encerrou'
        if status=$(curl --silent --noproxy '*' --max-time 3 --write-out '%{http_code}' \
            --header "Origin: $swagger_origin" --dump-header "$scratch/headers" \
            --output "$scratch/body" "$api_url/$path") && [[ $status == "$expected" ]]; then
            if [[ $expected == 200 ]]; then
                jq -e '.status == "ok"' "$scratch/body" > /dev/null
            else
                jq -e '.status == 503 and .code == "SERVICE_UNAVAILABLE"' "$scratch/body" > /dev/null
            fi
            tr -d '\r' < "$scratch/headers" | grep -Fxi "Access-Control-Allow-Origin: $swagger_origin" > /dev/null
            printf '%s respondeu %s\n' "$path" "$status"
            return
        fi
        sleep 0.2
    done
    fail "$path não respondeu $expected dentro do prazo (último status ${status:-sem resposta})"
}

check_health health/live 200
check_health health/ready 200
curl --fail --silent --show-error --noproxy '*' --max-time 3 --retry 5 --retry-connrefused \
    "$swagger_origin/openapi.yaml" --output "$scratch/openapi.yaml"
cmp docs/openapi.yaml "$scratch/openapi.yaml"
for service in postgres-test redis-test; do
    printf 'Interromper %s\n' "$service"
    paused_service=$service
    "${compose[@]}" pause "$service"
    check_health health/live 200
    check_health health/ready 503
    "${compose[@]}" unpause "$service"
    paused_service=
    check_health health/ready 200
done
stop_api

printf '%s\n' 'Verificar persistência PostgreSQL e descarte Redis após recriação'
"${compose[@]}" up -d --wait --wait-timeout 120 postgres redis
dev_sql=("${compose[@]}" exec -T postgres psql -XAt -v ON_ERROR_STOP=1 -U outguessed)
dev_redis=("${compose[@]}" exec -T redis redis-cli --raw -e)
redis_key="outguessed:foundation:$project"
"${dev_sql[@]}" -d postgres -c 'CREATE DATABASE foundation_probe'
"${dev_sql[@]}" -d foundation_probe -c "CREATE TABLE marker (value text); INSERT INTO marker VALUES ('foundation');"
expect OK "$("${dev_redis[@]}" SET "$redis_key" foundation EX 600 NX)" 'Gravação do marcador Redis'
expect foundation "$("${dev_redis[@]}" GET "$redis_key")" 'Marcador Redis antes da recriação'
old_postgres=$("${compose[@]}" ps -q postgres)
old_redis=$("${compose[@]}" ps -q redis)
[[ -n $old_postgres && -n $old_redis ]] || fail 'Containers originais não encontrados'
"${compose[@]}" up -d --force-recreate --wait --wait-timeout 120 postgres redis
new_postgres=$("${compose[@]}" ps -q postgres)
new_redis=$("${compose[@]}" ps -q redis)
[[ -n $new_postgres && $new_postgres != "$old_postgres" ]] || fail 'O container PostgreSQL deve ser recriado'
[[ -n $new_redis && $new_redis != "$old_redis" ]] || fail 'O container Redis deve ser recriado'
expect foundation "$("${dev_sql[@]}" -d foundation_probe -c 'SELECT value FROM marker')" 'Marcador PostgreSQL após recriação'
expect 0 "$("${dev_redis[@]}" EXISTS "$redis_key")" 'Marcador Redis após recriação'
"${dev_sql[@]}" -d postgres -c 'DROP DATABASE foundation_probe'
printf '%s\n' 'Verificações da fundação aprovadas'

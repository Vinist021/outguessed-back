# Backend OutGuessed

Fundação do backend do OutGuessed. A API expõe vivacidade em `/health/live` e prontidão em `/health/ready`. As rotas de negócio usam o prefixo `/api`, sem versão na URL ou no contrato. [Controle de progresso](steps/README.md)

O monólito Go carrega a configuração em `internal/config`, gerencia as conexões e o ciclo de vida em `internal/app` e publica os contratos HTTP com Chi em `internal/httpapi`. O PostgreSQL é acessado com `pgx` e consultas geradas pelo sqlc. O Redis guarda dados temporários. O contrato HTTP é documentado em [docs/openapi.yaml](docs/openapi.yaml).

## Requisitos

- Go 1.27.1 disponível no `PATH`.
- Docker com containers Linux e Docker Compose. A API roda pelo Go instalado no computador.
- PostgreSQL 18.7, Redis 8.2.11, sqlc 1.31.1, golang-migrate 4.20.1 e Swagger UI 5.34.1 são fornecidos pelas imagens fixadas no [Compose](compose.yaml).

Execute os comandos a seguir em PowerShell, a partir da raiz do backend.

## Preparar e iniciar

Copie o exemplo se `.env` ainda estiver ausente.

```powershell
if (-not (Test-Path -LiteralPath .env)) {
    Copy-Item .env.example .env
}
```

Em `development`, a API lê `.env` e dá precedência às variáveis do processo. O `APP_ENV` do processo seleciona o modo. Se a porta `5432` já estiver ocupada, ajuste `POSTGRES_PORT` e a porta de `DATABASE_URL` em `.env` para o mesmo valor, por exemplo `5434`.

| Serviço | Endereço no computador | Endereço na rede Compose |
| --- | --- | --- |
| PostgreSQL de desenvolvimento | `127.0.0.1:5432` ou `POSTGRES_PORT` | `postgres:5432` |
| Redis de desenvolvimento | `127.0.0.1:6379` | `redis:6379` |
| PostgreSQL de teste | `127.0.0.1:5433` | `postgres-test:5432` |
| Redis de teste | `127.0.0.1:6380` | `redis-test:6379` |

A API e os testes Go usam as portas do computador. O migrate roda em container e usa os nomes da rede Compose. As credenciais de exemplo servem aos ambientes locais da tabela.

```powershell
docker compose up -d --wait postgres redis
docker compose exec -T postgres pg_isready -h 127.0.0.1 -U outguessed -d outguessed
docker compose exec -T redis redis-cli ping
docker compose run --rm migrate up
docker compose run --rm migrate version
docker compose run --rm sqlc generate
go run ./cmd/api
```

O PostgreSQL deve aceitar conexões e o Redis deve responder `PONG`. A API abre o listener depois de verificar o schema e as duas dependências. Em outro terminal, consulte a API na porta padrão `8080`.

```powershell
curl.exe -i http://localhost:8080/health/live
curl.exe -i http://localhost:8080/health/ready
```

Em condições normais, ambas as rotas retornam 200. Se uma dependência cair após a inicialização, a prontidão retorna 503 e a vivacidade continua respondendo 200.

## Swagger UI

```powershell
docker compose --profile docs up -d swagger
```

Abra `http://localhost:8081`, selecione um endpoint e use **Try it out** e **Execute**. Se `HTTP_ADDR` usar outra porta, ajuste a variável `port` do servidor no Swagger UI. A origem do Swagger precisa constar em `CORS_ALLOWED_ORIGINS`, como no `.env.example`.

## Testes e verificações

A suíte padrão roda sem PostgreSQL e Redis. `gofmt -l .` deve produzir uma saída vazia.

```powershell
gofmt -l .
go mod verify
go mod tidy -diff
go vet ./...
go vet -tags=integration ./...
go build ./...
go test -count=1 ./...
```

O perfil `test` usa instâncias descartáveis e portas próprias. A suíte de integração exige as duas variáveis abaixo no processo PowerShell e um schema migrado no banco descartável.

```powershell
docker compose --profile test up -d --wait postgres-test redis-test
docker compose run --rm migrate-test up
$env:TEST_DATABASE_URL = 'postgres://outguessed_test:outguessed_test_local@127.0.0.1:5433/outguessed_test?sslmode=disable'
$env:TEST_REDIS_URL = 'redis://127.0.0.1:6380/0'
go test -count=1 -tags=integration ./...
```

Em Linux, com GCC, Bash, `curl`, `jq` e Docker Compose 2.24.4 ou superior, o fluxo automatizado da CI também pode ser executado localmente a partir de um checkout limpo.

```bash
go test -race -count=1 ./...
bash scripts/verify-foundation.sh
```

O script usa um projeto Compose descartável para verificar geração SQL, migrations, integração com `-race`, recuperação da API e persistência após recriação de containers.

## Rollback e encerramento

Para verificar o rollback da migration inicial no ambiente de teste e restaurar o schema:

```powershell
docker compose run --rm migrate-test down 1
docker compose run --rm migrate-test up
```

Encerre a API com `Ctrl+C`. Para parar e remover as instâncias de teste:

```powershell
docker compose --profile test stop postgres-test redis-test
docker compose --profile test rm -f postgres-test redis-test
```

`docker compose stop` para os containers. `docker compose --profile docs down` remove os containers e a rede padrão, preservando o volume PostgreSQL. O Redis usa armazenamento temporário. Para apagar deliberadamente os dados PostgreSQL de desenvolvimento junto com os containers do projeto, execute:

```powershell
docker compose --profile docs down --volumes
```

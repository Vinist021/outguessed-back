# Etapa 02 — Serviços locais

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Disponibilizar PostgreSQL e Redis locais, ferramentas SQL reproduzíveis e um ambiente de teste independente. A API continuará sendo executada pelo Go instalado no computador, com Windows/PowerShell como fluxo de desenvolvimento documentado.

## Pré-requisitos

- [Etapa 01](01-estrutura-inicial.md) concluída e evidências registradas.
- Docker Desktop com containers Linux e Docker Compose disponíveis.
- Acesso ao daemon Docker. Revalidar a restrição observada na inspeção anterior antes de tentar executar os serviços.
- Portas locais previstas disponíveis ou ajustadas de forma consistente na configuração documentada.

## Decisões técnicas

### Imagens e portas

| Serviço | Imagem fixada | Porta no computador | Função |
| --- | --- | --- | --- |
| `postgres` | `postgres:18.7-bookworm` | `127.0.0.1:5432`, ajustável por `POSTGRES_PORT` | Banco de desenvolvimento. |
| `redis` | `redis:8.2.11-bookworm` | `127.0.0.1:6379` | Dados temporários de desenvolvimento. |
| `sqlc` | `sqlc/sqlc:1.31.1` | Sem porta publicada | Gerar código a partir de SQL. |
| `migrate` | `migrate/migrate:v4.20.1` | Sem porta publicada | Aplicar migrations de desenvolvimento. |
| `swagger` | `swaggerapi/swagger-ui:v5.34.1` | `127.0.0.1:8081` | Consultar e experimentar o OpenAPI local. |
| `postgres-test` | A mesma imagem PostgreSQL | `127.0.0.1:5433` | Banco descartável de teste. |
| `redis-test` | A mesma imagem Redis | `127.0.0.1:6380` | Redis descartável de teste. |
| `migrate-test` | A mesma imagem migrate | Sem porta publicada | Aplicar migrations somente no banco de teste. |

A API usará `127.0.0.1:8080` por padrão. Não será incluída como serviço do Compose nesta entrega. Não usar tags `latest` nas imagens.

O Compose está em [`compose.yaml`](../compose.yaml), com nome de projeto `outguessed-back`. `POSTGRES_PORT` altera somente a porta publicada do PostgreSQL de desenvolvimento; a porta interna permanece `5432`.

### Persistência e rede

- Usar volume nomeado apenas para o PostgreSQL de desenvolvimento. Na imagem PostgreSQL 18, montar o volume em `/var/lib/postgresql`, preservando o layout de dados da imagem.
- Desabilitar AOF e snapshots do Redis e não montar volume persistente de dados nele. Usar `tmpfs` em `/data` nos dois Redis, evitando também volumes anônimos da imagem. A recriação do Redis não preservará sessões ou outros dados temporários futuros.
- Serviços de teste terão armazenamento descartável, sem volumes nomeados compartilhados com desenvolvimento. O PostgreSQL de teste usará `tmpfs` em `/var/lib/postgresql`; seus dados serão descartados ao parar o container.
- Publicar portas somente no loopback. Credenciais de exemplo e ausência de senha no Redis destinam-se ao ambiente local.
- A API local usa os endereços do computador. Containers auxiliares usam os nomes dos serviços e as portas internas, como `postgres:5432` e `postgres-test:5432`.
- Configurar healthchecks com `pg_isready` via TCP e `redis-cli ping`, para que a ordem de execução não dependa de pausas arbitrárias. Usar intervalo de 5 segundos, timeout de 3 segundos, período inicial de 10 segundos e 10 tentativas.

### Perfis e ferramentas

- Serviços `postgres` e `redis` formam o ambiente padrão.
- Usar perfil `tools` para `sqlc` e `migrate`. A execução explícita por `docker compose run --rm` deve funcionar sem manter containers de ferramenta ativos.
- Usar perfil `docs` para o Swagger UI, com o OpenAPI montado somente para leitura. Seu funcionamento completo depende do arquivo criado na etapa 04.
- Usar perfil `test` para `postgres-test`, `redis-test` e `migrate-test`.
- Montar migrations como somente leitura nas ferramentas migrate. Montar o repositório no sqlc com permissão de escrita para os arquivos gerados.
- Preparar `db/migrations` com um marcador `.gitkeep`, sem antecipar SQL da etapa 03. Os argumentos de caminho e conexão ficam no `entrypoint` de cada migrate, preservando comandos passados por `docker compose run --rm`. Cada ferramenta migrate aguarda a saúde apenas do seu PostgreSQL.
- Montar o repositório do sqlc em `/src`, também usado como diretório de trabalho. A imagem mantém seu executável em `/workspace/sqlc`; montar o repositório em `/workspace` ocultaria esse executável.
- Preparar o Swagger com `SWAGGER_JSON=/spec/openapi.yaml` e bind somente para leitura de `docs/openapi.yaml`, com `create_host_path: false`. Antes da etapa 04, iniciar esse serviço falhará por ausência do arquivo; o Compose não criará um diretório no lugar dele.
- Os serviços locais devem poder subir nesta etapa com valores de desenvolvimento definidos no Compose; a configuração completa da API e o `.env.example` serão entregues na etapa 03.

## Uso local

Executar os comandos na raiz do backend. O padrão continua sendo a porta `5432`. Neste computador, o serviço Linux `postgresql@18-main.service` já utiliza essa porta; o ajuste acordado para o projeto é `5434`, preservando o PostgreSQL instalado.

No PowerShell, antes dos comandos Compose da sessão:

```powershell
$env:POSTGRES_PORT = "5434"
```

No Bash:

```bash
export POSTGRES_PORT=5434
```

Definir novamente o ajuste ao abrir outro terminal neste computador. Em computadores com `5432` livre, a variável pode permanecer ausente. As URLs da API, quando implementadas na etapa 03, deverão refletir a porta publicada escolhida.

| Serviço | Endereço no computador nesta validação | Endereço entre containers | Banco / usuário |
| --- | --- | --- | --- |
| PostgreSQL de desenvolvimento | `127.0.0.1:5434` | `postgres:5432` | `outguessed` / `outguessed` |
| Redis de desenvolvimento | `127.0.0.1:6379` | `redis:6379` | — |
| PostgreSQL de teste | `127.0.0.1:5433` | `postgres-test:5432` | `outguessed_test` / `outguessed_test` |
| Redis de teste | `127.0.0.1:6380` | `redis-test:6379` | — |

As senhas locais de exemplo estão no Compose. As ferramentas migrate usam exclusivamente os nomes e portas internos, independentemente de `POSTGRES_PORT`. Iniciar somente `postgres` e `redis` no fluxo padrão; para testes, indicar explicitamente `postgres-test` e `redis-test`, deixando `migrate-test` para execução pontual com `run --rm`.

## Checklist de implementação

- [x] Criar o Compose com as imagens, nomes e portas definidos.
- [x] Configurar volume e healthcheck do PostgreSQL de desenvolvimento.
- [x] Configurar Redis sem persistência em disco e com healthcheck.
- [x] Criar os serviços auxiliares e seus perfis, sem executá-los automaticamente.
- [x] Criar serviços de teste descartáveis, sem referências ao volume de desenvolvimento.
- [x] Preparar o perfil de documentação, registrando a dependência do OpenAPI da etapa 04.
- [x] Conferir a diferença entre endereços acessíveis pelo computador e pelos containers.
- [x] Executar as verificações disponíveis desta etapa e revisar o diff.
- [x] Registrar evidências e atualizar o [controle de progresso](README.md).

## Entregáveis

- Ambiente local com PostgreSQL e Redis utilizáveis.
- Ferramentas SQL com versões fixadas, sem instalação adicional no Windows.
- Serviços de teste independentes e perfil de documentação preparado.

## Verificações e critérios de aceite

Comandos na raiz do backend, depois de definir `POSTGRES_PORT` quando necessário:

```powershell
docker compose config --quiet
docker compose --profile "*" config --quiet
docker compose up -d --wait postgres redis
docker compose exec postgres pg_isready -h 127.0.0.1 -U outguessed -d outguessed
docker compose exec redis redis-cli ping
docker compose run --rm sqlc version
docker compose run --rm migrate -version
docker compose --profile test up -d --wait postgres-test redis-test
docker compose exec postgres-test pg_isready -h 127.0.0.1 -U outguessed_test -d outguessed_test
docker compose exec redis-test redis-cli ping
docker compose run --rm migrate-test -version
```

Conferir se o PostgreSQL aceita conexões e se o Redis responde `PONG`. Verificar no Compose resolvido os destinos dos volumes e a publicação exclusiva no loopback. Não registrar a configuração resolvida inteira como evidência se contiver credenciais.

A persistência do PostgreSQL e o descarte de dados Redis serão comprovados por escrita e recriação controlada na etapa 05. Ainda não executar `migrate up` ou `sqlc generate` esperando sucesso antes de existirem as migrations e consultas da etapa 03. Ainda não exigir que o Swagger UI apresente contratos antes da etapa 04.

## Referências

- [Imagem oficial PostgreSQL](https://hub.docker.com/_/postgres).
- [Imagem oficial Redis](https://hub.docker.com/_/redis).
- [Instalação e execução do sqlc](https://docs.sqlc.dev/en/latest/overview/install.html).
- [golang-migrate](https://github.com/golang-migrate/migrate).
- [Swagger UI](https://swagger.io/docs/open-source-tools/swagger-ui/usage/installation/).

## Registro de execução

**Situação:** concluída, com serviços e ferramentas validados.

**Data:** 17/11/2026 (America/Sao_Paulo).

**Referência:** [`compose.yaml`](../compose.yaml) e marcador `db/migrations/.gitkeep`, com alterações locais sem criação de commit.

**Ambiente:** Linux/amd64, Docker Engine 29.9.3 e Docker Compose v5.7.0. O ambiente isolado continua sem acesso ao socket Docker; os comandos que consultam o daemon foram executados com permissão fora do isolamento. Não havia containers ou volumes deste projeto antes da execução.

**Ajuste local:** a porta `5432` já estava ocupada pelo cluster PostgreSQL `18/main` instalado no Linux. A validação usou `POSTGRES_PORT=5434`, conforme decisão confirmada. O serviço `postgresql@18-main.service` permaneceu ativo e não foi alterado. A configuração sem essa variável também foi conferida e preserva `5432` como padrão.

| Comando ou verificação executada | Resultado observado |
| --- | --- |
| `docker version --format '{{.Server.Version}}'` e `docker compose version` | Daemon acessível fora do isolamento; versões 29.9.3 e v5.7.0. |
| `docker compose config --quiet` e `docker compose --profile "*" config --quiet` | Sucesso antes e depois do ajuste da montagem sqlc. |
| Inspeção seletiva do Compose resolvido | Oito serviços com imagens fixadas; somente `postgres` e `redis` ativos por padrão; portas no loopback; padrão `5432` e ajuste `5434` válidos; montagens e perfis coerentes. Nenhuma configuração resolvida completa foi registrada como evidência. |
| `docker compose up -d --wait --wait-timeout 120 postgres redis` | Imagens baixadas e ambos os serviços `healthy`. |
| `docker compose exec -T postgres pg_isready -h 127.0.0.1 -U outguessed -d outguessed` | `127.0.0.1:5432 - accepting connections`, dentro do container. |
| `docker compose exec -T redis redis-cli ping` | `PONG`. |
| `docker compose run --rm sqlc version` | `v1.31.1`, após corrigir a montagem do repositório para `/src`. |
| `docker compose run --rm migrate -version` | `4.20.1`, aguardando o PostgreSQL saudável e sem aplicar migrations. |
| `docker compose --profile test up -d --wait --wait-timeout 120 postgres-test redis-test` | Ambos os serviços de teste `healthy`. |
| `docker compose exec -T postgres-test pg_isready -h 127.0.0.1 -U outguessed_test -d outguessed_test` | `127.0.0.1:5432 - accepting connections`, dentro do container. |
| `docker compose exec -T redis-test redis-cli ping` | `PONG`. |
| `docker compose run --rm migrate-test -version` | `4.20.1`, usando somente o PostgreSQL de teste. |
| Consultas `psql` autenticadas pelo computador nas portas `5434` e `5433` | A consulta `SELECT current_database() \|\| '\|' \|\| current_user \|\| '\|' \|\| 1` retornou o banco, usuário e valor `1` esperados em cada serviço. A consulta a `pg_database` confirmou que cada servidor contém somente seu respectivo banco entre `outguessed` e `outguessed_test`. |
| `redis-cli --raw CONFIG GET appendonly` e `redis-cli --raw CONFIG GET save` nos dois Redis | `appendonly` igual a `no` e `save` vazio. |
| `PING` Redis por conexão TCP do computador nas portas `6379` e `6380` | `PONG` em ambos os serviços. |
| Inspeção dos argumentos migrate e dos quatro containers com `docker inspect` | Conexões internas destinadas aos bancos correspondentes; quatro serviços saudáveis; portas somente no loopback; volume `outguessed-back_postgres-data` exclusivo do PostgreSQL de desenvolvimento em `/var/lib/postgresql`; armazenamento `tmpfs` nos outros três serviços. |
| `docker compose stop postgres-test redis-test` e `docker compose rm -f postgres-test redis-test` | Containers descartáveis de teste parados e removidos após a validação. |
| `docker compose --profile "*" ps -a` e `systemctl is-active postgresql@18-main.service` | Somente PostgreSQL e Redis de desenvolvimento permaneceram no projeto, ambos saudáveis em `5434` e `6379`; PostgreSQL instalado continuou `active`. |
| `git diff --check` e revisão do diff | Sucesso; alterações preexistentes da etapa 01 preservadas. |

A primeira execução do sqlc identificou uma colisão de montagem: `/workspace` ocultava o executável `/workspace/sqlc` da imagem. A montagem foi movida para `/src` e as verificações de configuração e versão passaram na repetição. As ferramentas executadas com `run --rm` não deixaram containers ativos.

**Aceite:** etapa 02 concluída. PostgreSQL e Redis de desenvolvimento permanecem disponíveis; o perfil de teste pode ser iniciado novamente pelos comandos documentados. Migrations e geração SQL ficam para a etapa 03; o Swagger está preparado e aguarda `docs/openapi.yaml` da etapa 04; a comprovação por escrita e recriação controlada permanece na etapa 05.

## Navegação

[Controle de progresso](README.md) · Anterior: [01 — Estrutura inicial](01-estrutura-inicial.md) · Próxima: [03 — Configuração e persistência](03-configuracao-persistencia.md)

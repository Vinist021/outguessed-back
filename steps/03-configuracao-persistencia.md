# Etapa 03 — Configuração e persistência

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Conectar a aplicação aos serviços locais, comprovar o fluxo de migrations e geração SQL sem entidades de negócio e definir a propriedade dos recursos durante a execução.

## Pré-requisitos

- [Etapa 02](02-servicos-locais.md) concluída, com PostgreSQL e Redis saudáveis.
- Estrutura Go da etapa 01 compilável.
- Ferramentas `migrate`, `migrate-test` e `sqlc` disponíveis pelo Compose.

## Decisões técnicas

### Configuração tipada

Centralizar leitura, conversão e validação de ambiente em `internal/config`. O restante da aplicação recebe valores tipados; handlers não consultam variáveis de ambiente.

| Variável | Contrato inicial |
| --- | --- |
| `APP_ENV` | `development`, `test` ou `production`; padrão `development`. |
| `HTTP_ADDR` | Padrão `127.0.0.1:8080`. |
| `DATABASE_URL` | URL PostgreSQL obrigatória, sem valor público de produção predefinido. |
| `REDIS_URL` | URL Redis obrigatória. |
| `CORS_ALLOWED_ORIGINS` | Lista separada por vírgulas com origens explícitas; valores locais descritos na etapa 04. |
| `LOG_LEVEL` | Nível de log; padrão `info`. |
| `STARTUP_TIMEOUT` | Duração positiva; padrão `10s`. |
| `DEPENDENCY_TIMEOUT` | Duração positiva; padrão `2s`. |
| `SHUTDOWN_TIMEOUT` | Duração positiva; padrão `10s`. |

- Decidir se `.env` pode ser lido usando `APP_ENV` do processo, com padrão `development`, antes de abrir esse arquivo.
- Preservar o modo obtido do processo durante o carregamento; um valor de `APP_ENV` no arquivo não pode mudar o modo de execução já escolhido.
- Em desenvolvimento, carregar `.env` quando existir, sem sobrescrever variáveis já presentes no processo. Arquivo malformado deve produzir erro identificável; arquivo ausente não substitui a validação das variáveis obrigatórias.
- Em `test` e `production`, não ler `.env`. O valor efetivo de `APP_ENV` deve ser validado.
- Entregar `.env.example` com os campos da tabela, os parâmetros do Compose e valores exclusivamente locais. Explicar como manter as credenciais e portas do Compose coerentes com as URLs da API.
- Preservar `.env` existente e manter segredos fora do Git. Nesta etapa, os arquivos de configuração serão criados ou ajustados durante a implementação, não pela entrega destes documentos.
- Rejeitar valores ausentes, inválidos ou durações não positivas antes de abrir conexões. Informar a variável inválida sem expor seu conteúdo sensível.

### Migrations

Usar `golang-migrate` com arquivos sequenciais em `db/migrations`:

- `000001_create_outguessed_schema.up.sql`: cria somente o schema `outguessed`.
- `000001_create_outguessed_schema.down.sql`: remove esse schema sem `CASCADE`.
- Não criar tabelas de negócio ou demonstrativas. A tabela técnica de controle criada pelo migrate não é uma entidade do jogo.
- Manter o schema técnico de controle do migrate separado do schema removido pelo rollback, usando seu padrão em `public`.
- Não modificar migrations já aplicadas para introduzir mudanças novas. Novas alterações recebem novos números.
- Aplicar migrations explicitamente antes da API. Não embutir migração automática na inicialização.
- Reaplicação sem pendências deve ser tratada como estado normal pela ferramenta. Não usar `force` como correção automática de estado dirty; diagnosticar a falha antes de qualquer intervenção.
- Testar rollback apenas no ambiente descartável. O rollback inicial deve falhar se o schema contiver objetos dependentes, em vez de removê-los implicitamente.

### sqlc

Configurar `sqlc.yaml` com formato de configuração `2`, engine PostgreSQL, migrations como origem do esquema e consultas em `db/queries`. Gerar código no pacote `internal/postgres/dbgen`, usando `sql_package: pgx/v5`.

A primeira consulta será exclusivamente de prontidão:

```sql
-- name: CheckSchema :one
SELECT (to_regnamespace('outguessed') IS NOT NULL)::boolean AS ready;
```

Essa consulta verifica a estrutura necessária nesta fundação e o acesso real ao PostgreSQL. Não substitui futuras verificações de compatibilidade das migrations de negócio.

O cast explícito preserva a semântica da expressão e permite ao analisador local do sqlc 1.31.1 gerar o retorno Go como `bool`, sem acesso a um banco para inferência de tipos.

Versionar consultas e arquivos gerados. Nunca editar manualmente o resultado do gerador. A geração deve funcionar usando apenas os arquivos SQL locais, sem banco remoto ou serviço sqlc Cloud. A CI posteriormente verificará se a regeneração mantém o repositório sem diferenças.

### Conexões e ciclo de vida

- Usar `pgxpool` e `go-redis/v9`, respeitando cancelamento e prazos de I/O.
- A composição da aplicação cria e fecha os clientes. O serviço de saúde recebe interfaces pequenas para a consulta PostgreSQL e o `PING` Redis.
- Durante a inicialização, verificar PostgreSQL, existência do schema e Redis dentro do prazo global de 10 segundos, com verificações limitadas a 2 segundos cada. Repetições cabem apenas nesse prazo global e devem respeitar cancelamento.
- Só aceitar HTTP depois das verificações iniciais. Esgotado o prazo, fechar recursos já abertos, registrar a causa sanitizada e sair com código diferente de zero.
- Após iniciar, a queda de uma dependência deixa a aplicação não pronta, mas não encerra o processo. Verificações posteriores devem permitir recuperação usando os clientes da aplicação.
- A prontidão deve consultar novamente as dependências, com orçamento total de 2 segundos por requisição, sem criar uma conexão ou um cliente novo para cada chamada.
- No encerramento, marcar a aplicação como não pronta, interromper a aceitação de tráfego e aguardar requisições por até 10 segundos. Fechar os clientes depois de encerrar ou cancelar o trabalho ativo.
- Controlar todas as goroutines criadas pela aplicação. Nenhuma tarefa deve continuar usando um cliente depois de fechado.

### Logs

Usar `slog` com saída JSON e nível configurável. Registrar inicialização, falha, recuperação e encerramento com contexto operacional. Na etapa 04, acrescentar método, caminho, status, duração e identificador das requisições.

Não registrar URLs com senha, variáveis de ambiente completas, credenciais ou payloads sensíveis. Erros internos devem permanecer identificáveis na aplicação, com contexto e encadeamento quando necessário; sua tradução para HTTP ocorrerá apenas na borda.

## Uso local

Na raiz do backend, preparar o arquivo local somente se ele ainda não existir.

PowerShell:

```powershell
if (!(Test-Path .env)) { Copy-Item .env.example .env }
```

Bash:

```bash
test -e .env || cp .env.example .env
```

O exemplo usa a porta PostgreSQL padrão `5432`. Neste computador, ajustar **tanto** `POSTGRES_PORT` quanto a porta em `DATABASE_URL` para `5434`, conforme a etapa 02. A API roda no computador e usa `127.0.0.1`; as ferramentas migrate usam os nomes internos dos serviços e sempre a porta interna `5432`. Os usuários, senhas e bancos das URLs devem corresponder aos configurados no Compose. Alterar as variáveis de inicialização do PostgreSQL não altera automaticamente as credenciais de um volume já existente.

Variáveis presentes no processo prevalecem sobre `.env`, inclusive `POSTGRES_PORT` na interpolação do Compose. O modo da API é escolhido por `APP_ENV` do processo, com padrão `development`; o campo do arquivo nunca troca esse modo. Em `test` e `production`, fornecer as variáveis pelo processo, pois a API não lê `.env`. Uma lista vazia de `CORS_ALLOWED_ORIGINS` representa nenhuma origem autorizada; o tratamento HTTP será implementado na etapa 04.

Após preparar a configuração:

```powershell
docker compose up -d --wait postgres redis
docker compose run --rm migrate up
docker compose run --rm migrate version
docker compose run --rm sqlc generate
go run ./cmd/api
```

Encerrar a API com `Ctrl+C`. Nesta etapa, o roteador continua sem endpoints de saúde; o log de início confirma que as dependências foram verificadas antes da abertura do HTTP. A aplicação não aplica migrations automaticamente.

Para executar os testes reais, iniciar apenas os serviços descartáveis e aplicar a migration de teste:

```powershell
docker compose --profile test up -d --wait postgres-test redis-test
docker compose run --rm migrate-test up
$env:TEST_DATABASE_URL = "postgres://outguessed_test:outguessed_test_local@127.0.0.1:5433/outguessed_test?sslmode=disable"
$env:TEST_REDIS_URL = "redis://127.0.0.1:6380/0"
go test -tags=integration ./...
```

No Bash, definir as mesmas variáveis com `export TEST_DATABASE_URL='...'` e `export TEST_REDIS_URL='...'`. A suíte padrão (`go test ./...`) dispensa serviços externos; a suíte com a tag `integration` exige ambas as variáveis e o banco `outguessed_test`, sem fallback para desenvolvimento. Não executar simultaneamente os cenários que alteram esse schema. Os testes revertem seus objetos de verificação e restauram o schema, sem usar `migrate force`.

O rollback real deve ser executado exclusivamente no banco descartável:

```powershell
docker compose run --rm migrate-test down 1
docker compose run --rm migrate-test up
```

Não remover o volume de desenvolvimento. Ao terminar os testes, os containers descartáveis podem ser parados e removidos pelos comandos documentados na etapa 02.

## Checklist de implementação

- [x] Implementar configuração tipada e carregamento local de `.env` com precedência correta.
- [x] Criar `.env.example` e documentar valores locais, sem sobrescrever o `.env` existente.
- [x] Criar as migrations inicial e de rollback, sem tabelas de negócio.
- [x] Configurar migrate para bancos de desenvolvimento e teste independentes.
- [x] Configurar sqlc e criar a consulta `CheckSchema`.
- [x] Gerar e disponibilizar para versionamento o código SQL, usando-o na verificação PostgreSQL.
- [x] Conectar e gerenciar os clientes PostgreSQL e Redis.
- [x] Implementar inicialização com prazo e falha antes de aceitar HTTP.
- [x] Preparar recuperação de prontidão e encerramento controlado.
- [x] Configurar logs JSON sem dados sensíveis.
- [x] Executar as verificações disponíveis e revisar o diff.
- [x] Registrar evidências e atualizar o [controle de progresso](README.md).

## Entregáveis

- Configuração local reproduzível e validada.
- Migration inicial aplicável e reversível no ambiente de teste.
- Geração SQL executável, sem código ou tabelas demonstrativas.
- Aplicação que controla suas conexões e só aceita tráfego após validar as dependências.

## Verificações e critérios de aceite

Depois da implementação, na raiz do backend:

```powershell
docker compose run --rm migrate up
docker compose run --rm migrate version
docker compose run --rm sqlc generate
go build ./...
go vet ./...
```

No ambiente descartável, aplicar a migration, verificar a existência do schema, executar `down 1`, verificar sua ausência e aplicar novamente. Executar a consulta gerada em ambos os estados e conferir seu resultado booleano.

Verificar a precedência do ambiente sobre `.env`, a ausência de leitura de `.env` em `test` e `production`, a validação de configuração e o encerramento dentro dos prazos. Iniciar a aplicação sem dependência disponível ou sem schema e confirmar que ela não aceita HTTP e retorna erro operacional sanitizado.

Os contratos HTTP de prontidão serão entregues na etapa 04. A demonstração completa de queda e recuperação com serviços reais será consolidada na etapa 05.

## Referências

- [Configuração do sqlc](https://docs.sqlc.dev/en/latest/reference/config.html).
- [Migrations como origem do esquema no sqlc](https://docs.sqlc.dev/en/latest/howto/ddl.html).
- [Funções de informação do PostgreSQL](https://www.postgresql.org/docs/18/functions-info.html).
- [Tutorial PostgreSQL do golang-migrate](https://github.com/golang-migrate/migrate/blob/master/database/postgres/TUTORIAL.md).

## Registro de execução

**Situação:** concluída em 17/11/2026 (America/Sao_Paulo), com alterações locais sem criação de commit.

**Ambiente:** Linux/amd64, Go 1.27.1, sqlc 1.31.1 e migrate 4.20.1. O Go foi executado por `/usr/local/go/bin/go`, pois seu diretório não estava no `PATH` desta sessão. As operações Docker e os testes que abrem conexões foram executados com acesso fora do isolamento. Nenhum `.env` foi criado ou sobrescrito.

**Implementação:** configuração centralizada em `internal/config`, serviço de prontidão em `internal/health`, consulta gerada em `internal/postgres/dbgen` e composição dos recursos em `internal/app`. Os diagnósticos internos do Redis são encaminhados ao `slog` em nível debug com mensagem sanitizada; as falhas operacionais são identificadas pelo serviço de saúde. O exemplo local e os comandos acima documentam a correspondência das portas e credenciais.

| Verificação executada | Resultado observado |
| --- | --- |
| `docker compose --profile '*' config --quiet` | Todos os perfis válidos; os dois migrate usam `search_path=public`, separados do schema de aplicação. |
| `docker compose run --rm migrate up` e `migrate version`, com `POSTGRES_PORT=5434` | Migration aplicada em desenvolvimento; versão `1`. PostgreSQL de desenvolvimento existente preservado na porta `5434`. |
| `docker compose --profile test up -d --wait --wait-timeout 120 postgres-test redis-test` | Serviços descartáveis saudáveis nas portas `5433` e `6380`. |
| `migrate-test up`, nova execução de `up`, `version`, `down 1` e novo `up` pelo Compose | Aplicação bem-sucedida, reaplicação com `no change`, versão `1`, rollback e nova aplicação bem-sucedidos. |
| Consultas de inspeção após rollback e após nova aplicação | O schema ficou ausente após `down 1`; `public.schema_migrations` permaneceu presente. Ao final, desenvolvimento e teste tinham schema presente, zero tabelas no schema `outguessed` e versão `1` com `dirty=false`. |
| `docker compose run --rm --no-deps sqlc generate`, com repetição e comparação SHA-256 | Geração apenas com SQL local, retorno Go `bool` e três arquivos gerados idênticos na repetição. Nenhum código gerado editado manualmente. |
| `go test ./...` e `go test -count=1 -v -tags=integration ./...` | Suítes unitária e de integração aprovadas. Configuração, precedência, modos, erros sanitizados, recuperação, prazos e encerramento verificados. |
| `go test -race -count=1 -tags=integration ./...` | Todos os pacotes aprovados com detector de corridas; integração incluiu consulta gerada nos dois estados do schema, bloqueio do rollback com objeto dependente, inicialização saudável, liberação das conexões PostgreSQL e falhas de PostgreSQL, Redis e schema antes da abertura do HTTP. |
| Teste temporário do executável em processos separados | Configuração inválida, autenticação PostgreSQL inválida, Redis indisponível e schema ausente retornaram código `1`, sem listener HTTP e com logs JSON sanitizados. Inicialização saudável validou dependências antes do HTTP, retornou `404` no roteador ainda vazio e encerrou por SIGTERM com código `0`, dentro dos prazos configurados. |
| `go mod verify`, `go mod tidy -diff`, `go build ./...`, `go vet ./...`, `gofmt -l cmd internal` e `git diff --check` | Verificações aprovadas, sem diferenças de dependências, problemas de formatação ou whitespace. |

A primeira tentativa de `go mod tidy` sofreu uma falha transitória de rede no proxy Go; a repetição concluiu e `go mod tidy -diff` passou. Os testes de rollback com objeto dependente usaram transação revertida no banco descartável, sem deixar a ferramenta migrate em estado dirty e sem executar `force`.

**Aceite:** etapa 03 concluída. A API e os containers de teste foram encerrados após a validação; os serviços de desenvolvimento permanecem ativos. As alterações anteriores dos documentos das etapas 01 e 02 foram preservadas. Endpoints de saúde, erros HTTP, CORS e OpenAPI continuam na etapa 04. A interrupção e restauração dos serviços reais com observação HTTP, a CI e o README principal permanecem na etapa 05.

## Navegação

[Controle de progresso](README.md) · Anterior: [02 — Serviços locais](02-servicos-locais.md) · Próxima: [04 — Contratos HTTP](04-contratos-http.md)

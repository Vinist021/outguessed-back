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
SELECT to_regnamespace('outguessed') IS NOT NULL AS ready;
```

Essa consulta verifica a estrutura necessária nesta fundação e o acesso real ao PostgreSQL. Não substitui futuras verificações de compatibilidade das migrations de negócio.

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

## Checklist de implementação

- [ ] Implementar configuração tipada e carregamento local de `.env` com precedência correta.
- [ ] Criar `.env.example` e documentar valores locais, sem sobrescrever o `.env` existente.
- [ ] Criar as migrations inicial e de rollback, sem tabelas de negócio.
- [ ] Configurar migrate para bancos de desenvolvimento e teste independentes.
- [ ] Configurar sqlc e criar a consulta `CheckSchema`.
- [ ] Gerar e versionar o código SQL, usando-o na verificação PostgreSQL.
- [ ] Conectar e gerenciar os clientes PostgreSQL e Redis.
- [ ] Implementar inicialização com prazo e falha antes de aceitar HTTP.
- [ ] Preparar recuperação de prontidão e encerramento controlado.
- [ ] Configurar logs JSON sem dados sensíveis.
- [ ] Executar as verificações disponíveis e revisar o diff.
- [ ] Registrar evidências e atualizar o [controle de progresso](README.md).

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

**Ainda não executada.** Registrar configuração validada, migrations aplicadas, geração executada, resultados observados e pendências. Não incluir senhas ou URLs de conexão completas nas evidências.

## Navegação

[Controle de progresso](README.md) · Anterior: [02 — Serviços locais](02-servicos-locais.md) · Próxima: [04 — Contratos HTTP](04-contratos-http.md)

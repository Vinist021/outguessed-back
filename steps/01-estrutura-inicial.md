# Etapa 01 — Estrutura inicial

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Criar o módulo Go e a organização mínima da aplicação, estabelecendo responsabilidades claras para receber os serviços e os contratos das próximas etapas.

## Pré-requisitos

- Ler [AGENTS.md](../AGENTS.md), [rules.md](../rules.md) e as decisões gerais do [controle](README.md).
- Ter Go 1.27.1 disponível no PowerShell.
- Conferir o estado do repositório e preservar alterações anteriores, inclusive as alterações em `AGENTS.md` e as remoções de documentos existentes.

## Decisões técnicas

O módulo será `github.com/Vinist021/outguessed-back`. A aplicação será um único processo Go, organizado como monólito modular. Fixar as versões das dependências em `go.mod` e `go.sum`; não adicionar bibliotecas de funcionalidades que ainda não serão implementadas.

### Organização e responsabilidades

| Localização prevista | Responsabilidade |
| --- | --- |
| `cmd/api` | Entrada do processo, chamada de inicialização e código de saída. |
| `internal/app` | Compor dependências e coordenar inicialização e encerramento. |
| `internal/config` | Carregar e validar configuração tipada. |
| `internal/httpapi` | Roteamento Chi, handlers, serialização, erros e middlewares HTTP. |
| `internal/health` | Verificar prontidão, sem depender de detalhes HTTP. |
| `internal/postgres` | Conexão e pool `pgx/v5`, além da integração com consultas geradas. |
| `internal/postgres/dbgen` | Código gerado pelo sqlc, sem edição manual. |
| `internal/redis` | Cliente `go-redis/v9` e operações de infraestrutura necessárias. |
| `db` | Migrations incrementais e consultas SQL de origem. |
| `docs` | Contratos OpenAPI. |

Essa tabela define onde cada responsabilidade ficará. Criar pacotes e diretórios quando tiverem conteúdo concreto; não criar implementações vazias ou dependências artificiais apenas para reproduzir a estrutura inteira nesta etapa.

### Fronteiras

- A entrada delega a coordenação à aplicação. Handlers não criam conexões ou carregam variáveis de ambiente.
- O serviço de saúde recebe apenas as dependências necessárias para realizar verificações. Os adaptadores PostgreSQL e Redis são conectados a ele na composição da aplicação.
- A aplicação controla os recursos e seu encerramento. Operações de I/O recebem `context.Context`.
- Usar Chi, `pgx/v5`, `go-redis/v9` e `log/slog` nas etapas em que forem necessários.
- Criar interfaces pequenas na fronteira que precisa de substituição em testes. Não criar repositórios genéricos, pacotes `util`/`common` ou camadas sem uso concreto.
- Futuros módulos de autenticação, usuários, decks e cartas, tabuleiros e partidas serão organizados por capacidade. Rodadas, turnos, bots e chat permanecerão junto de partidas enquanto não houver necessidade concreta de separação.
- A API será sem versionamento, com futuro prefixo `/api`. Não criar handlers fictícios para domínios ainda ausentes.

### Entrada HTTP entregue nesta etapa

- `cmd/api` configura logs JSON com `log/slog`, captura interrupção e término e chama `app.Run(ctx, "127.0.0.1:8080", logger)`.
- `internal/app` abre o servidor e controla seu ciclo de vida. Falhas operacionais retornam erro e levam o processo ao código de saída 1; cancelamento normal encerra com sucesso.
- `internal/httpapi` fornece um roteador Chi sem rotas cadastradas. Caminhos inexistentes recebem o 404 padrão; os contratos operacionais continuam previstos para a etapa 04.
- O encerramento aguarda requisições por até 10 segundos, com um contexto independente do cancelamento do processo. Ao esgotar o prazo, fecha conexões restantes; a aplicação aguarda a goroutine de execução do servidor.
- O servidor usa os prazos já previstos para HTTP: cabeçalhos em 5 segundos, leitura em 10, escrita em 15 e ociosidade em 60.
- A única dependência externa é `github.com/go-chi/chi/v5 v5.5.0`, fixada em `go.mod` e `go.sum`. A aplicação ainda não lê `.env` nem exige serviços externos.

## Checklist de implementação

- [x] Conferir a versão do Go e o estado inicial do repositório.
- [x] Inicializar o módulo com o caminho definido e Go 1.27.1.
- [x] Criar a entrada em `cmd/api` e a composição mínima em `internal/app`.
- [x] Manter a aplicação compilável, sem depender de PostgreSQL ou Redis nesta etapa.
- [x] Estabelecer o roteador Chi quando a entrada HTTP for criada, sem contratos demonstrativos.
- [x] Fixar apenas as dependências já utilizadas.
- [x] Ajustar o `.gitignore` às saídas Go e manter `.env` ignorado e `.env.example` permitido, preservando regras locais úteis.
- [x] Executar as verificações desta etapa e revisar o diff.
- [ ] Obter sucesso em `go test ./...`, incluindo a execução do teste de `internal/httpapi`.
- [x] Registrar evidências e atualizar o [controle de progresso](README.md).

## Entregáveis

- Módulo Go com dependências reproduzíveis.
- Entrada compilável e organização mínima pronta para receber a infraestrutura.
- Responsabilidades sem ciclos de dependência ou código de negócio antecipado.

## Verificações e critérios de aceite

Executar na raiz do backend, depois que os arquivos correspondentes existirem:

```powershell
go version
go mod tidy
go mod verify
go build ./...
go vet ./...
go test ./...
gofmt -l .
```

O build, o `go vet` e os testes devem terminar com sucesso; `gofmt -l .` não deve listar arquivos Go pendentes de formatação. Os testes desta etapa verificam 404 para caminho inexistente, encerramento por cancelamento e falha de inicialização com endereço ocupado, usando loopback e portas efêmeras. Conferir que o caminho do módulo está correto e que nenhuma funcionalidade de negócio foi criada.

Nesta etapa, build sem testes ainda não comprova comportamento de infraestrutura. Conexões e saúde serão verificadas nas etapas 03 e 04; a cobertura integrada será consolidada na etapa 05.

## Registro de execução

**Data:** 13/11/2026 (America/Sao_Paulo).

**Situação:** implementação entregue; aceite bloqueado pela política de Controle de Aplicativo do Windows durante a execução do teste de `internal/httpapi`.

**Arquivos criados:** `go.mod`, `go.sum`, `cmd/api/main.go`, `internal/app/app.go`, `internal/app/app_test.go`, `internal/httpapi/router.go` e `internal/httpapi/router_test.go`.

**Arquivos atualizados:** `.gitignore`, este documento e `steps/README.md`. A alteração anterior em `AGENTS.md`, as remoções de `DER.txt` e `Redis.md`, os demais documentos e o `.env` local foram preservados. Nenhum commit foi criado.

| Comando executado | Resultado observado |
| --- | --- |
| `go version` | `go version go1.27.1 windows/amd64`. |
| `go mod tidy` | Concluído; Chi v5.5.0 obtido e `go.sum` gerado. Foi necessária permissão ampliada para escrever no cache de módulos. |
| `go mod verify` | Sucesso: `all modules verified`. |
| `gofmt -w cmd internal` | Formatação aplicada aos arquivos Go. |
| `go build ./...` | Sucesso após direcionar os diretórios temporário e de cache para `.cache/` no projeto. |
| `go vet ./...` | Sucesso com os mesmos diretórios locais. |
| `go test ./...` | Falha de execução no Windows: `internal/app` aprovado, `cmd/api` sem testes e execução de `internal/httpapi` bloqueada pelo Controle de Aplicativo. |
| `gofmt -l .` | Sucesso, sem arquivos listados. |
| `git diff --check` | Sucesso; aviso de conversão futura LF/CRLF em `AGENTS.md`, preexistente e preservado. |
| `git check-ignore` | Confirmadas as saídas Go e `.env` ignorados; `.env.example` permitido. |

Para as verificações locais de build, análise estática e a primeira execução dos testes, foram usados:

```powershell
$env:GOCACHE = Join-Path (Get-Location) '.cache/go-build'
$env:GOTMPDIR = Join-Path (Get-Location) '.cache/go-tmp'
```

O diretório `.cache/go-tmp` foi criado com `New-Item -ItemType Directory -Path '.cache/go-tmp' -Force`. Esses ajustes valeram somente nos processos de verificação e usam a regra `.cache/` já existente no `.gitignore`; a configuração persistente do Go não foi alterada.

**Impedimento confirmado:** `go test ./...` foi repetido com permissão ampliada e com o diretório temporário padrão `C:\Users\vini\go-tmp`; o bloqueio de `httpapi.test.exe` persistiu. A consulta de eventos `Microsoft-Windows-CodeIntegrity/Operational`, evento 3077, confirmou que o executável não atende aos requisitos de assinatura da política ativa. As políticas do Windows e os testes foram mantidos.

**Pendência para o aceite:** executar `go test ./...` em um ambiente que permita os binários locais de teste e confirmar todos os pacotes aprovados. Depois, registrar o resultado e concluir a etapa no controle central. PostgreSQL, Redis, saúde e contratos HTTP permanecem nas etapas posteriores.

## Navegação

[Controle de progresso](README.md) · Anterior: [visão geral](README.md) · Próxima: [02 — Serviços locais](02-servicos-locais.md)

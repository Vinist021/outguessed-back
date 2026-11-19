# Fundação do backend — controle de progresso

## Estado atual

- **Planejamento:** documentado nesta pasta.
- **Implementação:** fundação implementada e validada.
- **Última etapa:** [05 — Validação e documentação](05-validacao-documentacao.md), concluída em 19/11/2026.
- **Próxima etapa:** nenhuma nesta entrega.
- **Etapas de implementação concluídas:** 5 de 5.
- **Verificações da aplicação:** testes com detector de corridas, build, análise estática e formatação aprovados. Contratos OpenAPI, CORS e os dois endpoints pelo Swagger UI verificados.
- **Verificações de infraestrutura:** geração SQL reproduzível, migrations, recuperação da prontidão, persistência PostgreSQL e descarte Redis confirmados em checkout limpo e na CI. [Execução aprovada](https://github.com/Vinist021/outguessed-back/actions/runs/44639390072)

Estes documentos registram o planejamento e sua execução. A existência de um documento não significa que sua implementação ou seus testes foram concluídos. Os checklists são marcados somente conforme as evidências de cada etapa.

## Sequência e pendências

Executar as etapas na ordem abaixo. Iniciar uma etapa somente depois de concluir e registrar o aceite da anterior. Se uma verificação exigir uma capacidade prevista para depois, verificar o resultado parcial indicado no documento e reservar a integração completa para a etapa 05.

| Etapa | Documento | Pré-requisito | Status | Pendências principais |
| --- | --- | --- | --- | --- |
| 01 | [Estrutura inicial](01-estrutura-inicial.md) | Ler o guia e as regras atuais | Concluída | Nenhuma; aceite registrado após aprovação da suíte completa em Linux |
| 02 | [Serviços locais](02-servicos-locais.md) | Etapa 01 concluída | Concluída | Nenhuma nesta etapa; Swagger preparado para o OpenAPI da etapa 04 |
| 03 | [Configuração e persistência](03-configuracao-persistencia.md) | Etapa 02 concluída | Concluída | Nenhuma nesta etapa; serviço de prontidão preparado para os contratos HTTP da etapa 04 |
| 04 | [Contratos HTTP](04-contratos-http.md) | Etapa 03 concluída | Concluída | Nenhuma nesta etapa. Saúde, erros, CORS e Swagger UI validados |
| 05 | [Validação e documentação](05-validacao-documentacao.md) | Etapa 04 concluída | Concluída | Nenhuma nesta entrega |

## Objetivo e decisões gerais

Entregar uma fundação executável e verificável para o backend OutGuessed, respeitando [AGENTS.md](../AGENTS.md) e [rules.md](../rules.md). As regras de negócio continuam sendo definidas por `rules.md`; estes documentos especificam sua infraestrutura inicial.

- Monólito modular em Go 1.27.1, com módulo `github.com/Vinist021/outguessed-back`.
- Chi para HTTP, `pgx/v5` com SQL gerado pelo sqlc, `go-redis/v9` e logs com `log/slog`.
- Desenvolvimento no Windows/PowerShell: API no Go local; PostgreSQL, Redis e ferramentas auxiliares em containers.
- Comandos diretos de Go e Docker Compose, sem Taskfile ou scripts de automação obrigatórios.
- Futuro ambiente público com uma instância da API, frontend na mesma origem e proxy reverso para `/api`. A publicação não faz parte desta entrega.
- **API sem versionamento:** rotas de negócio sob `/api`, sem versões em caminhos, cabeçalhos, parâmetros ou negociação de conteúdo. Endpoints de saúde fora desse prefixo.
- OpenAPI e exemplos versionados no Git. A versão do formato OpenAPI e o campo `info.version` são metadados do documento e não introduzem versionamento da API.
- PostgreSQL com dados persistentes; Redis descartável, sem persistência em disco. A perda do Redis não pode alterar o estado oficial do jogo.
- Compose com portas no loopback e ambiente de teste independente em memória. `POSTGRES_PORT` mantém `5432` como padrão; neste computador, usar `5434` para preservar o PostgreSQL instalado. [Comandos PowerShell e Bash](02-servicos-locais.md#uso-local).
- Migrations aplicadas explicitamente antes da API; sqlc com código gerado versionado.
- Documentação e mensagens humanas em português; campos JSON, códigos, pacotes e variáveis em inglês.
- CI com verificações estáticas e integração real, usando serviços isolados do desenvolvimento.

## Limites desta entrega

A fundação não implementa autenticação, sessões reais, recursos de usuários, decks, tabuleiros, partidas, bots, chat ou WebSocket. Não cria tabelas de negócio nem endpoints demonstrativos desses domínios. Não modifica o frontend, não publica serviços e não acrescenta microserviços, filas, Kubernetes ou coordenação distribuída.

Os módulos de negócio serão criados quando suas funcionalidades forem implementadas. Políticas de senha, limites de texto, rate limit, mecanismo de autenticação e contratos de WebSocket serão definidos nas entregas correspondentes, respeitando as regras existentes.

## Como atualizar o progresso

| Status | Quando usar |
| --- | --- |
| Pendente | A implementação da etapa ainda não começou. |
| Em andamento | Há trabalho efetivo na etapa e seus critérios ainda não foram atendidos. |
| Bloqueada | Existe um impedimento concreto registrado, com condição necessária para prosseguir. |
| Concluída | Checklist atendido, verificações executadas e evidências registradas. |

Ao começar, atualizar a etapa atual e o status da tabela. Durante a execução, marcar somente tarefas efetivamente concluídas no documento da etapa e registrar o que falta. Ao terminar, preencher as evidências, atualizar o número de etapas concluídas e indicar a próxima etapa. Ao concluir a etapa 05, registrar o aceite final e deixar a próxima etapa como “nenhuma nesta entrega”.

Não marcar uma etapa como concluída porque seu código foi escrito ou porque um teste foi planejado. Não registrar comandos como aprovados sem executá-los. Se uma alteração invalidar uma evidência anterior, registrar a regressão e corrigir o status.

## Impedimentos

A execução inicial da etapa 01 encontrou um bloqueio confirmado do Controle de Aplicativo do Windows na execução de `httpapi.test.exe`. O bloqueio persistiu com permissão ampliada e com o diretório temporário padrão do Go; os eventos de integridade de código do Windows confirmaram a exigência de assinatura. Esse registro é histórico: a pendência de aceite foi atendida em 13/11/2026 com `go test -count=1 -v ./...` aprovado em todos os pacotes em Linux com Go 1.27.1. A política do Windows não foi alterada nem revalidada.

O ambiente isolado restringe o socket Docker e a abertura de conexões locais. As verificações das etapas 02 a 05 que exigiam esses recursos ocorreram fora do isolamento. Nas etapas anteriores, a validação de desenvolvimento usou `POSTGRES_PORT=5434` porque o PostgreSQL instalado ocupa `5432`. A etapa 05 usou portas isoladas alocadas pelo Docker.

| Etapa afetada | Impedimento | Condição para resolver | Situação |
| --- | --- | --- | --- |
| 01 | Histórico: Windows impediu a execução de `httpapi.test.exe` por política de integridade de código | Executar `go test ./...` em ambiente que permita os binários de teste e obter sucesso em todos os pacotes | Condição de aceite atendida em Linux em 13/11/2026; política do Windows não revalidada |
| 02 a 05 | Acesso ao daemon Docker restrito no ambiente isolado | Consultar o daemon e iniciar os serviços necessários fora do isolamento | Acesso confirmado e verificações concluídas em todas as etapas afetadas |

## Evidências de conclusão

Registrar data, referência de código quando houver, comandos executados, resultados e pendências remanescentes. Não incluir credenciais, URLs com senha ou outros segredos.

| Etapa | Data | Referência | Verificações e resultado | Pendências |
| --- | --- | --- | --- | --- |
| 01 | 13/11/2026 | Commit `9dc564c`; formalização somente documental, sem novo commit | Go 1.27.1 em Linux: `go mod verify`, `go mod tidy -diff`, `go build ./...`, `go vet ./...`, `gofmt -l cmd internal` e `git diff --check` aprovados; `go test -count=1 -v ./...` aprovou os três testes de `internal/app` e `internal/httpapi`. [Registro completo](01-estrutura-inicial.md#validação-e-aceite--linux). | Nenhuma nesta etapa; etapa 02 disponível para início |
| 02 | 17/11/2026 | `compose.yaml` e marcador `db/migrations/.gitkeep`; alterações locais sem commit | Compose padrão e todos os perfis válidos; quatro serviços saudáveis; conexões PostgreSQL autenticadas, `PONG` Redis, isolamento, montagens e loopback conferidos; sqlc 1.31.1 e ambos os migrate 4.20.1 executados; revisão e `git diff --check` aprovados. [Registro completo](02-servicos-locais.md#registro-de-execução). | Nenhuma nesta etapa; etapa 03 disponível para início |
| 03 | 17/11/2026 | Configuração, migrations, sqlc, clientes e testes; alterações locais sem commit | Migration `1` limpa em desenvolvimento e teste; rollback/reaplicação e proteção de dependentes aprovados; geração SQL reproduzível; build, vet, dependências, testes e detector de corridas aprovados; executável recusou inicializações inválidas sem HTTP e encerrou por SIGTERM com logs JSON sanitizados. [Registro completo](03-configuracao-persistencia.md#registro-de-execução). | Nenhuma nesta etapa; etapa 04 disponível para início |
| 04 | 19/11/2026 | Roteador HTTP, testes e `docs/openapi.yaml` em alterações locais | Testes com detector de corridas, requisições HTTP locais, logs sanitizados, CORS e validação OpenAPI aprovados. Ambos os endpoints responderam 200 no Swagger UI. [Registro completo](04-contratos-http.md#registro-de-execução). | Nenhuma nesta etapa. Etapa 05 disponível para início |
| 05 | 19/11/2026 | Commit `5d7105f` | Testes com detector de corridas, geração SQL, migrations, recuperação, persistência e Swagger UI validados em checkout limpo. CI aprovada. [Registro completo](05-validacao-documentacao.md#registro-de-execução-e-aceite-final) · [Execução da CI](https://github.com/Vinist021/outguessed-back/actions/runs/44639390072) | Nenhuma nesta entrega |

## Navegação

[Etapa 04 — concluída](04-contratos-http.md) · [Etapa 05 — validação e aceite final](05-validacao-documentacao.md)

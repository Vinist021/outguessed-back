# Fundação do backend — controle de progresso

## Estado atual

- **Planejamento:** documentado nesta pasta.
- **Implementação:** etapa 01 implementada, com aceite bloqueado na execução dos testes pelo Windows.
- **Etapa atual:** [01 — Estrutura inicial](01-estrutura-inicial.md).
- **Próxima etapa:** [02 — Serviços locais](02-servicos-locais.md), após concluir o aceite da etapa 01.
- **Etapas de implementação concluídas:** 0 de 5.
- **Verificações da aplicação:** módulo, build, `go vet` e formatação aprovados; testes de `internal/app` aprovados; execução do teste de `internal/httpapi` bloqueada pelo Controle de Aplicativo do Windows.

Estes documentos registram o planejamento e sua execução. A existência de um documento não significa que sua implementação ou seus testes foram concluídos. Os checklists são marcados somente conforme as evidências de cada etapa.

## Sequência e pendências

Executar as etapas na ordem abaixo. Iniciar uma etapa somente depois de concluir e registrar o aceite da anterior. Se uma verificação exigir uma capacidade prevista para depois, verificar o resultado parcial indicado no documento e reservar a integração completa para a etapa 05.

| Etapa | Documento | Pré-requisito | Status | Pendências principais |
| --- | --- | --- | --- | --- |
| 01 | [Estrutura inicial](01-estrutura-inicial.md) | Ler o guia e as regras atuais | Bloqueada | Executar o teste de `internal/httpapi` sem bloqueio do Controle de Aplicativo do Windows e concluir `go test ./...` |
| 02 | [Serviços locais](02-servicos-locais.md) | Etapa 01 concluída | Pendente | Compose, serviços locais, ferramentas e ambiente de teste |
| 03 | [Configuração e persistência](03-configuracao-persistencia.md) | Etapa 02 concluída | Pendente | Configuração, migrations, sqlc, conexões e ciclo de vida |
| 04 | [Contratos HTTP](04-contratos-http.md) | Etapa 03 concluída | Pendente | Saúde, erros, CORS e OpenAPI com Swagger UI |
| 05 | [Validação e documentação](05-validacao-documentacao.md) | Etapa 04 concluída | Pendente | Testes completos, CI, README principal e aceite final |

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

A etapa 01 encontrou um bloqueio confirmado do Controle de Aplicativo do Windows na execução de `httpapi.test.exe`. O bloqueio persistiu com permissão ampliada e com o diretório temporário padrão do Go; os eventos de integridade de código do Windows confirmaram a exigência de assinatura. O build e os testes de `internal/app` foram aprovados, mas a suíte completa ainda não terminou com sucesso.

Na inspeção anterior, o ambiente do agente não conseguiu acessar o daemon Docker por restrição de permissão. Isso ainda precisa ser revalidado antes da etapa 02; não comprova que o Docker do usuário esteja parado. As etapas 02 a 05 continuam pendentes e não foram iniciadas.

| Etapa afetada | Impedimento | Condição para resolver | Situação |
| --- | --- | --- | --- |
| 01 | Windows impede a execução de `httpapi.test.exe` por política de integridade de código | Executar `go test ./...` em ambiente que permita os binários de teste e obter sucesso em todos os pacotes | Confirmado em 13/11/2026; aceite bloqueado |
| 02, 03 e 05 | Acesso ao daemon Docker não validado pelo agente | Conseguir consultar o daemon e iniciar os serviços de teste | A revalidar antes da execução |

## Evidências de conclusão

Registrar data, referência de código quando houver, comandos executados, resultados e pendências remanescentes. Não incluir credenciais, URLs com senha ou outros segredos.

| Etapa | Data | Referência | Verificações e resultado | Pendências |
| --- | --- | --- | --- | --- |
| 01 | 13/11/2026 | Código local em `cmd/api`, `internal/app`, `internal/httpapi`, `go.mod` e `go.sum`; sem commit | `go mod tidy`, `go mod verify`, `go build ./...`, `go vet ./...`, formatação e revisão aprovados; `internal/app` aprovado; `go test ./...` falhou ao executar o binário de `internal/httpapi` por política do Windows. [Registro completo](01-estrutura-inicial.md#registro-de-execução). | Resolver o impedimento de execução e concluir a suíte antes do aceite |
| 02 | — | — | Não executadas | Implementação não iniciada |
| 03 | — | — | Não executadas | Implementação não iniciada |
| 04 | — | — | Não executadas | Implementação não iniciada |
| 05 | — | — | Não executadas | Implementação não iniciada |

## Navegação

[Etapa 01 — implementação e validação pendente](01-estrutura-inicial.md) · [Aceite final na etapa 05](05-validacao-documentacao.md)

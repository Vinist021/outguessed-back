# Etapa 05 — Validação e documentação

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Consolidar a validação da fundação, automatizar as verificações no GitHub e documentar um fluxo reproduzível de preparação, execução e teste.

## Pré-requisitos

- [Etapa 04](04-contratos-http.md) concluída, com API e Swagger UI operacionais.
- Etapas anteriores com suas evidências registradas.
- Serviços de teste isolados e acesso ao daemon Docker.
- Execução da CI disponível no repositório GitHub; criar o workflow faz parte desta etapa futura.

## Decisões técnicas

### Estratégia de testes

Usar `go test` para testes unitários e de contrato. Reservar a build tag `integration` aos testes que exigem PostgreSQL ou Redis reais.

- A suíte padrão não exige serviços externos.
- A suíte de integração exige `TEST_DATABASE_URL` e `TEST_REDIS_URL` apontando aos serviços descartáveis do perfil `test`. A ausência dessas variáveis deve resultar em falha clara, sem substituir silenciosamente pelas URLs de desenvolvimento.
- Testes de migrations e rollback usam exclusivamente o banco descartável. Não usar limpeza destrutiva em banco ou Redis do desenvolvimento.
- Usar chaves Redis exclusivas por teste e limpar apenas as chaves criadas pelo próprio teste. Não depender da ordem de execução entre testes independentes.
- Não executar em paralelo cenários que alteram o mesmo schema ou interrompem os mesmos serviços.
- Validar o documento OpenAPI com uma biblioteca Go de validação e comparar respostas HTTP com os schemas documentados. A especificação continua sendo a fonte dos contratos.
- Os testes de falha não criam endpoints demonstrativos, não dependem de atrasos arbitrários e não escondem problemas por meio de skips automáticos.

### Matriz mínima de cenários

| Área | Cenários e resultados esperados |
| --- | --- |
| Configuração | Obrigatórios ausentes, valores inválidos, precedência do processo, `.env` local e ausência de leitura em `test`/`production`. |
| Inicialização | Dependências disponíveis permitem iniciar; banco, Redis ou schema ausentes impedem aceitação de HTTP dentro do prazo definido. |
| Migrations | Subida em banco vazio, reaplicação sem pendências, rollback inicial e nova aplicação. |
| sqlc | Geração reproduzível, compilação e execução real de `CheckSchema` com schema presente e ausente. |
| Redis | Conexão e escrita/leitura com limpeza de chaves exclusivas dos testes. |
| Saúde | Vivacidade independente das dependências e prontidão coerente com o estado atual. |
| Falha e recuperação | Interromper e restaurar PostgreSQL e Redis de teste; observar prontidão 503/200 sem reiniciar a API e vivacidade 200. |
| HTTP | Contratos 200, 404, 405, 500 e 503, tipos de conteúdo, `Allow`, identificadores e ausência de detalhes internos. |
| CORS | Origem permitida, origem não permitida, preflight e requisição sem `Origin`; credenciais desabilitadas. |
| Encerramento | Prontidão indisponível, requisições encerradas ou canceladas no prazo e recursos liberados. |
| Persistência local | Dados PostgreSQL sobrevivem à recriação do container com o volume; Redis não recupera os dados temporários após recriação. |
| Documentação | OpenAPI válido, respostas coerentes e Swagger UI capaz de executar os endpoints locais. |

A verificação da persistência de desenvolvimento deve utilizar um banco descartável de verificação dentro do servidor ou outro marcador isolado, sem introduzir tabelas demonstrativas nas migrations ou modificar dados do usuário. Limpar o marcador ao terminar. Diferenciar recriação de container de simples reinício do processo.

### GitHub Actions

Criar workflow para `push` e `pull_request`, com runner Linux e Go 1.27.1. Usar as mesmas versões de serviços e ferramentas definidas no Compose.

Executar verificação de formatação sem reescrita, `go mod verify`, `go vet`, build, testes unitários e integração real. Habilitar o detector de corridas no Linux e disponibilizar o compilador C necessário no runner. Windows deve continuar suportado pelo fluxo local sem exigir `-race`.

Aplicar migrations no ambiente de teste antes de executar cenários dependentes do schema. Regerar SQL e falhar se houver diferenças, inclusive novos arquivos não versionados no diretório gerado. Mudanças inesperadas em `go.mod` ou `go.sum` também devem ser identificadas, sem atualizar dependências automaticamente na CI.

Implementar um teste de fumaça da aplicação em processo separado: iniciar a API com as URLs de teste, interromper cada dependência isoladamente, verificar vivacidade e prontidão, restaurar o serviço e conferir recuperação. Encerrar API e serviços ao final, inclusive em falha.

### README principal

Atualizar o README da raiz do backend somente quando esta etapa for implementada. Ele deve mostrar requisitos, versões, arquitetura resumida e comandos reais do projeto para:

1. Preparar as variáveis locais a partir do exemplo, respeitando um `.env` já existente.
2. Subir PostgreSQL e Redis e verificar disponibilidade.
3. Aplicar migrations explicitamente e gerar código SQL.
4. Iniciar a API e conferir saúde.
5. Habilitar e acessar Swagger UI.
6. Executar testes unitários, integração e verificações estáticas.
7. Encerrar os serviços, fazer rollback no ambiente apropriado e remover dados locais deliberadamente.

Explicar a diferença entre parar containers e remover o volume PostgreSQL, entre endereços do computador e da rede Compose e entre o perfil padrão e o de teste. Manter rotas de negócio sob `/api`, sem versionamento. Incluir link para o [controle de progresso](README.md).

## Checklist de implementação

- [ ] Implementar testes de configuração e de contratos HTTP.
- [ ] Implementar integração com migrations, consulta gerada e Redis reais.
- [ ] Implementar cenários de falha, recuperação e encerramento.
- [ ] Validar persistência PostgreSQL e descarte Redis sem afetar dados do usuário.
- [ ] Validar OpenAPI e compatibilidade das respostas.
- [ ] Criar o workflow GitHub Actions com geração SQL e detector de corridas no Linux.
- [ ] Garantir isolamento e limpeza dos recursos usados pela CI.
- [ ] Atualizar o README principal com os comandos efetivamente disponíveis.
- [ ] Executar o fluxo completo em checkout limpo ou ambiente equivalente sem saídas de build e configuração local preexistentes.
- [ ] Confirmar execução aprovada da CI e revisar o diff final.
- [ ] Registrar evidências e concluir o [controle de progresso](README.md).

## Entregáveis

- Testes automatizados significativos para a infraestrutura e seus contratos.
- Workflow de CI executado com sucesso, incluindo integração real.
- README operacional reproduzível e controle de progresso atualizado com evidências.

## Verificações e critérios de aceite

Comandos locais previstos após implementar as suítes e os serviços:

```powershell
go mod verify
go vet ./...
go build ./...
gofmt -l .
go test ./...
docker compose run --rm sqlc generate
docker compose --profile test up -d --wait postgres-test redis-test
docker compose run --rm migrate-test up
```

Definir `TEST_DATABASE_URL` e `TEST_REDIS_URL` no processo PowerShell com os valores dos serviços descartáveis antes de executar:

```powershell
go test -tags=integration ./...
```

O carregamento automático de `.env` pela API não deve ser pressuposto pelo comando `go test`. O README deve conter instruções explícitas para fornecer essas variáveis, coerentes com as credenciais e portas configuradas.

No Linux da CI, executar também:

```sh
go test -race ./...
go test -race -tags=integration ./...
```

Concluir somente quando o fluxo documentado funcionar a partir de checkout limpo, os cenários da matriz forem atendidos, a regeneração não alterar arquivos e a CI estiver aprovada. Não considerar apenas a existência do workflow como evidência de aprovação.

## Registro de execução e aceite final

**Ainda não executada.** Registrar a referência do código validado, o resultado dos comandos, o endereço da execução da CI e eventuais limitações. Não declarar aprovado o que não foi executado.

Ao concluir, marcar a etapa 05 e as anteriores como concluídas apenas se suas evidências continuarem válidas. Atualizar o controle para “fundação implementada e validada”, com 5 de 5 etapas concluídas e nenhuma próxima etapa nesta entrega. Funcionalidades de negócio exigirão novo planejamento.

## Navegação

[Controle de progresso](README.md) · Anterior: [04 — Contratos HTTP](04-contratos-http.md) · Próxima: [aceite final no controle](README.md) — não há etapa 06 nesta entrega.

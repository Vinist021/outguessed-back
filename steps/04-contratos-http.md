# Etapa 04 — Contratos HTTP

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Expor os contratos operacionais iniciais, padronizar erros e correlação de requisições e disponibilizar OpenAPI com Swagger UI local.

## Pré-requisitos

- [Etapa 03](03-configuracao-persistencia.md) concluída, com configuração, conexões, migration e consulta gerada verificadas.
- Serviço de saúde disponível para o transporte HTTP.
- Perfil `docs` do Compose preparado na etapa 02.

## Decisões técnicas

### API sem versionamento

Rotas futuras de negócio usarão exclusivamente o prefixo `/api`. Não incluir versões em caminhos, cabeçalhos, parâmetros ou negociação de conteúdo. Não implementar endpoints fictícios para autenticação ou recursos de negócio apenas para preencher o prefixo.

Os endpoints de saúde ficam fora de `/api`. No futuro ambiente público, o proxy reverso encaminhará `/api` para a API, enquanto a exposição de saúde será definida operacionalmente na implantação. Não implementar o proxy ou modificar o frontend nesta entrega.

### Endpoints iniciais

| Método e caminho | Condição | Status | Resposta |
| --- | --- | --- | --- |
| `GET /health/live` | Processo HTTP ativo | 200 | `{"status":"ok"}` |
| `GET /health/ready` | PostgreSQL, schema e Redis disponíveis; aplicação fora do encerramento | 200 | `{"status":"ok"}` |
| `GET /health/ready` | Dependência indisponível, schema ausente ou encerramento iniciado | 503 | Problem Details com `code: SERVICE_UNAVAILABLE` |

- Respostas de sucesso usam `application/json`. Não incluem nomes de hosts, versões ou detalhes internos das dependências.
- A vivacidade não consulta banco ou Redis. Uma queda posterior de dependência não pode transformá-la em falha enquanto o servidor HTTP estiver ativo.
- A prontidão usa a consulta gerada e `PING` Redis, sem modificar dados e respeitando o orçamento da etapa 03. Durante o encerramento, pode retornar 503 imediatamente.
- Consultar as dependências a cada chamada de prontidão; não apresentar um resultado antigo como confirmação atual.
- Usar `Cache-Control: no-store` nas respostas de saúde.
- Os endpoints operacionais não exigem autenticação nesta fundação.

### Formato dos erros

Usar [Problem Details, conforme RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html), com `Content-Type: application/problem+json` e os campos abaixo:

| Campo | Contrato |
| --- | --- |
| `type` | URN estável do tipo do problema, como `urn:outguessed:problem:service-unavailable`. |
| `title` | Título curto em português, estável para o tipo de problema. |
| `status` | Número igual ao status HTTP da resposta. |
| `detail` | Explicação sanitizada em português, sem erro bruto de infraestrutura. |
| `instance` | URN da ocorrência, no formato `urn:outguessed:request:<requestId>`. |
| `code` | Código estável em inglês, destinado ao tratamento pelo cliente. |
| `requestId` | Identificador gerado pelo servidor e também retornado em `X-Request-ID`. |
| `errors` | Lista opcional de objetos `{field, code, message}` para futuras validações de campos; omitir quando não se aplicar. |

Exemplo de indisponibilidade:

```json
{
  "type": "urn:outguessed:problem:service-unavailable",
  "title": "Serviço indisponível",
  "status": 503,
  "detail": "O serviço não está pronto para receber requisições.",
  "instance": "urn:outguessed:request:identificador-exemplo",
  "code": "SERVICE_UNAVAILABLE",
  "requestId": "identificador-exemplo"
}
```

Mapeamentos iniciais:

| Status | `code` | Sufixo de `type` | Significado |
| --- | --- | --- | --- |
| 404 | `NOT_FOUND` | `not-found` | Caminho inexistente. |
| 405 | `METHOD_NOT_ALLOWED` | `method-not-allowed` | Método incompatível com um caminho existente; manter o cabeçalho `Allow`. |
| 500 | `INTERNAL_ERROR` | `internal-error` | Falha inesperada, com mensagem pública genérica. |
| 503 | `SERVICE_UNAVAILABLE` | `service-unavailable` | Aplicação não pronta. |

Centralizar escrita e tradução dos erros na borda HTTP. Usar o mesmo contrato para erros do roteador e recuperação de panics. Registrar a causa interna sanitizada com o identificador, sem retorná-la ao cliente.

### Requisições e logs

- Gerar um identificador opaco por requisição no servidor, sem confiar em um identificador arbitrário enviado pelo cliente.
- Disponibilizá-lo no contexto, no cabeçalho `X-Request-ID`, no erro HTTP quando houver e nos logs correspondentes.
- Registrar método, caminho sem query string, status, duração e identificador. Não registrar corpo, credenciais, cookies ou cabeçalhos sensíveis.
- Manter o identificador disponível também para 404, 405 e 500.
- Configurar prazos de servidor HTTP: leitura de cabeçalhos de 5 segundos, leitura de 10 segundos, escrita de 15 segundos e conexão ociosa de 60 segundos. Quando WebSocket for implementado, revisar os prazos próprios desse transporte na entrega correspondente.

### CORS

Permitir a lista explícita fornecida por `CORS_ALLOWED_ORIGINS`. O exemplo local terá:

- `http://localhost:5173` e `http://127.0.0.1:5173` para Vite.
- `http://localhost:8081` e `http://127.0.0.1:8081` para Swagger UI.

Não usar origem curinga nem `Access-Control-Allow-Credentials: true`. Expor `X-Request-ID` ao navegador. Permitir os métodos implementados nesta fundação e os cabeçalhos técnicos necessários, como `Content-Type`; não antecipar uma política de autenticação.

Origem permitida deve receber os cabeçalhos e o preflight correspondentes. Origem fora da lista não recebe autorização CORS; manter o tratamento HTTP normal, deixando o navegador aplicar a restrição. Requisições sem `Origin`, como verificações por linha de comando, continuam funcionando. Não confundir CORS com autenticação ou autorização.

### OpenAPI e Swagger UI

- Criar a especificação em `docs/openapi.yaml`, com formato OpenAPI 3.0.3, textos em português e os contratos implementados nesta etapa.
- Definir os schemas de sucesso e Problem Details, códigos, exemplos, cabeçalhos e respostas 404/405/500 aplicáveis, além de 503 na prontidão.
- O campo obrigatório `info.version`, inicialmente `0.1.0`, identifica a revisão do documento. Não deve ser usado para introduzir versionamento de acesso à API.
- Configurar o servidor local da especificação para a API em `http://localhost:8080`, permitindo ajuste da porta pela configuração de servidor do documento.
- Fazer o container Swagger UI ler o arquivo montado como somente leitura e disponibilizar “Try it out” contra a API local.
- Não servir a interface pelo Go e não exigir um novo build JavaScript.

## Checklist de implementação

- [x] Implementar os dois endpoints de saúde e sua serialização.
- [x] Implementar Problem Details e os quatro mapeamentos iniciais.
- [x] Implementar correlação de requisições e logs HTTP sanitizados.
- [x] Configurar recuperação de panics, 404, 405 e prazos HTTP.
- [x] Implementar CORS com origens explícitas e sem credenciais.
- [x] Criar OpenAPI coerente com os handlers, sem versionamento da API.
- [x] Concluir a integração do perfil `docs` com o Swagger UI.
- [x] Executar verificações de contrato e navegação no Swagger UI.
- [x] Registrar evidências e atualizar o [controle de progresso](README.md).

## Entregáveis

- Endpoints de saúde operacionais e erros consistentes.
- Correlação entre resposta HTTP e log.
- OpenAPI e Swagger UI utilizáveis no ambiente local.

## Verificações e critérios de aceite

Depois de iniciar serviços, aplicar migrations e iniciar a API:

```powershell
go run ./cmd/api
```

Em outro terminal:

```powershell
curl.exe -i http://localhost:8080/health/live
curl.exe -i http://localhost:8080/health/ready
curl.exe -i http://localhost:8080/caminho-inexistente
curl.exe -i -X POST http://localhost:8080/health/live
docker compose --profile docs up -d swagger
```

Conferir status, tipos de conteúdo, `X-Request-ID`, `Allow` em 405 e os campos do erro. Abrir `http://localhost:8081` e executar os endpoints pela interface. Verificar CORS para origem permitida, origem não permitida e requisição sem origem, além de validar o OpenAPI.

Não criar uma rota pública que provoque panic ou erro interno apenas para testar 500. Usar handlers controlados nos testes. A indisponibilidade e recuperação com serviços reais serão consolidadas na etapa 05.

## Registro de execução

**Concluída em 19/11/2026.** A validação usou Go 1.27.1 em Linux e os serviços locais com PostgreSQL na porta `5434` e Redis na `6379`. A API de validação usou variáveis de processo derivadas de `.env.example` e foi encerrada por SIGTERM após os testes. O Swagger UI foi parado ao final.

| Verificação | Resultado observado |
| --- | --- |
| `go test -race -count=1 ./...` | Todos os pacotes passaram. Testes controlados cobriram 500 por panic, 503 por falha de dependência, schema ausente e encerramento. Confirmaram consultas novas a cada prontidão, vivacidade independente e recuperação de 503 para 200. |
| `go mod verify`, `go mod tidy -diff`, `go vet ./...`, `go build ./...`, formatação e `git diff --check` | Verificações aprovadas. |
| `docker compose run --rm migrate version` com `POSTGRES_PORT=5434` | PostgreSQL saudável e migration `1` aplicada em desenvolvimento. |
| `curl` contra a API local | As duas rotas de saúde responderam 200 com JSON e `Cache-Control: no-store`. Caminho inexistente respondeu 404 e método incompatível respondeu 405 com `Allow: GET`, ambos com Problem Details. Os corpos corresponderam aos exemplos OpenAPI após substituir o identificador de ocorrência. |
| CORS e logs da API local | As quatro origens do exemplo local foram autorizadas. Requisições sem origem e de origem recusada mantiveram o status da rota. Preflight autorizado retornou 204 e o recusado seguiu para 405. Os logs correlacionaram método, caminho sem query, status e duração pelo `X-Request-ID` gerado no servidor. |
| `docker compose --profile "*" config --quiet` e `openapi-spec-validator --schema 3.0` | Configuração Compose e [especificação OpenAPI](../docs/openapi.yaml) válidas. |
| `docker compose --profile docs up -d swagger` e Chrome | A montagem de `/spec/openapi.yaml` estava somente para leitura. Em `http://localhost:8081`, o **Try it out** executou os dois endpoints e exibiu 200, corpo JSON e `X-Request-ID`. |

A interrupção e a restauração de PostgreSQL e Redis reais com observação HTTP integram o aceite da [etapa 05](05-validacao-documentacao.md).

## Navegação

[Controle de progresso](README.md) · Anterior: [03 — Configuração e persistência](03-configuracao-persistencia.md) · Próxima: [05 — Validação e documentação](05-validacao-documentacao.md)

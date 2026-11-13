# Etapa 02 — Serviços locais

**Status inicial:** Pendente. O andamento oficial é registrado no [controle de progresso](README.md).

## Objetivo

Disponibilizar PostgreSQL e Redis locais, ferramentas SQL reproduzíveis e um ambiente de teste independente. A API continuará sendo executada pelo Go instalado no Windows.

## Pré-requisitos

- [Etapa 01](01-estrutura-inicial.md) concluída e evidências registradas.
- Docker Desktop com containers Linux e Docker Compose disponíveis.
- Acesso ao daemon Docker. Revalidar a restrição observada na inspeção anterior antes de tentar executar os serviços.
- Portas locais previstas disponíveis ou ajustadas de forma consistente na configuração documentada.

## Decisões técnicas

### Imagens e portas

| Serviço | Imagem fixada | Porta no computador | Função |
| --- | --- | --- | --- |
| `postgres` | `postgres:18.7-bookworm` | `127.0.0.1:5432` | Banco de desenvolvimento. |
| `redis` | `redis:8.2.11-bookworm` | `127.0.0.1:6379` | Dados temporários de desenvolvimento. |
| `sqlc` | `sqlc/sqlc:1.31.1` | Sem porta publicada | Gerar código a partir de SQL. |
| `migrate` | `migrate/migrate:v4.20.1` | Sem porta publicada | Aplicar migrations de desenvolvimento. |
| `swagger` | `swaggerapi/swagger-ui:v5.34.1` | `127.0.0.1:8081` | Consultar e experimentar o OpenAPI local. |
| `postgres-test` | A mesma imagem PostgreSQL | `127.0.0.1:5433` | Banco descartável de teste. |
| `redis-test` | A mesma imagem Redis | `127.0.0.1:6380` | Redis descartável de teste. |
| `migrate-test` | A mesma imagem migrate | Sem porta publicada | Aplicar migrations somente no banco de teste. |

A API usará `127.0.0.1:8080` por padrão. Não será incluída como serviço do Compose nesta entrega. Não usar tags `latest` nas imagens.

### Persistência e rede

- Usar volume nomeado apenas para o PostgreSQL de desenvolvimento. Na imagem PostgreSQL 18, montar o volume em `/var/lib/postgresql`, preservando o layout de dados da imagem.
- Desabilitar AOF e snapshots do Redis e não montar volume de dados nele. A recriação do Redis não preservará sessões ou outros dados temporários futuros.
- Serviços de teste terão armazenamento descartável, sem volumes nomeados compartilhados com desenvolvimento.
- Publicar portas somente no loopback. Credenciais de exemplo e ausência de senha no Redis destinam-se ao ambiente local.
- A API local usa os endereços do computador. Containers auxiliares usam os nomes dos serviços e as portas internas, como `postgres:5432` e `postgres-test:5432`.
- Configurar healthchecks com `pg_isready` e `redis-cli ping`, para que a ordem de execução não dependa de pausas arbitrárias.

### Perfis e ferramentas

- Serviços `postgres` e `redis` formam o ambiente padrão.
- Usar perfil `tools` para `sqlc` e `migrate`. A execução explícita por `docker compose run --rm` deve funcionar sem manter containers de ferramenta ativos.
- Usar perfil `docs` para o Swagger UI, com o OpenAPI montado somente para leitura. Seu funcionamento completo depende do arquivo criado na etapa 04.
- Usar perfil `test` para `postgres-test`, `redis-test` e `migrate-test`.
- Montar migrations como somente leitura nas ferramentas migrate. Montar o repositório no sqlc com permissão de escrita para os arquivos gerados.
- Os serviços locais devem poder subir nesta etapa com valores de desenvolvimento definidos no Compose; a configuração completa da API e o `.env.example` serão entregues na etapa 03.

## Checklist de implementação

- [ ] Criar o Compose com as imagens, nomes e portas definidos.
- [ ] Configurar volume e healthcheck do PostgreSQL de desenvolvimento.
- [ ] Configurar Redis sem persistência em disco e com healthcheck.
- [ ] Criar os serviços auxiliares e seus perfis, sem executá-los automaticamente.
- [ ] Criar serviços de teste descartáveis, sem referências ao volume de desenvolvimento.
- [ ] Preparar o perfil de documentação, registrando a dependência do OpenAPI da etapa 04.
- [ ] Conferir a diferença entre endereços acessíveis pelo computador e pelos containers.
- [ ] Executar as verificações disponíveis desta etapa e revisar o diff.
- [ ] Registrar evidências e atualizar o [controle de progresso](README.md).

## Entregáveis

- Ambiente local com PostgreSQL e Redis utilizáveis.
- Ferramentas SQL com versões fixadas, sem instalação adicional no Windows.
- Serviços de teste independentes e perfil de documentação preparado.

## Verificações e critérios de aceite

Comandos previstos na raiz do backend:

```powershell
docker compose config --quiet
docker compose up -d --wait postgres redis
docker compose exec postgres pg_isready
docker compose exec redis redis-cli ping
docker compose run --rm sqlc version
docker compose run --rm migrate -version
docker compose --profile test up -d --wait postgres-test redis-test
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

**Ainda não executada.** Registrar acesso ao daemon, comandos e resultados efetivamente observados, além de qualquer ajuste local necessário. Não marcar a etapa concluída sem validar os serviços.

## Navegação

[Controle de progresso](README.md) · Anterior: [01 — Estrutura inicial](01-estrutura-inicial.md) · Próxima: [03 — Configuração e persistência](03-configuracao-persistencia.md)

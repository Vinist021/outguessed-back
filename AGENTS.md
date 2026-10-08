# AGENTS.md — backend OutGuessed

## Fonte de verdade e escopo

- Leia `rules.md` antes de alterar comportamento do jogo. Ele é normativo; este guia orienta a implementação e não cria regras de negócio.
- O repositório ainda não contém código Go nem contratos HTTP/WebSocket, esquema SQL ou mecanismo de sessão definidos. Verifique o que já existe antes de criar estrutura, comandos ou convenções.
- Implemente um monólito modular em Go. Prefira a menor solução que preserve as regras e permita testes; não antecipe microserviços, filas, Kubernetes ou coordenação distribuída.

## Organização e responsabilidades

- Organize por capacidade: autenticação, usuários, decks e cartas, tabuleiros e partidas. Mantenha rodadas, turnos, bots e chat junto de partidas enquanto não houver motivo concreto para separá-los.
- Handlers Chi e conexões `coder/websocket` recebem entradas, autenticam, validam o formato, chamam a aplicação e traduzem resultados. Não calculam acertos, movimento, vencedor ou transições.
- A lógica de aplicação valida autorização e estado atual e executa as regras de `rules.md`. Separe cálculos determinísticos de acesso a banco, relógio, aleatoriedade e serviços externos para facilitar testes.
- A persistência usa PostgreSQL com `pgx` e consultas SQL explícitas geradas por `sqlc`; Redis usa `go-redis`. Não coloque regras do jogo em SQL, handlers ou adaptadores Redis.
- Crie interfaces pequenas nos pontos onde uma dependência real precisa ser substituída ou isolada. Evite camadas, repositórios genéricos e abstrações para necessidades hipotéticas.

## Go, nomes e erros

- Use pacotes curtos em minúsculas, nomes Go em `MixedCaps` e terminologia estável de `rules.md`. Evite pacotes `util`/`common`, repetição do nome do pacote em símbolos e dependências circulares.
- Passe `context.Context` nas operações de I/O, respeite cancelamento e configure prazos em chamadas externas. Não mantenha goroutines ou temporizadores sem dono e sem encerramento definido.
- Retorne erros esperados de forma identificável; acrescente contexto com `%w` e use `errors.Is`/`errors.As` quando necessário. Faça a conversão para erro HTTP ou WebSocket somente na borda; não use `panic` para falhas esperadas.
- Use `log/slog` para logs estruturados com identificadores e contexto operacional, sem registrar senhas, sessões, gabaritos ocultos, payloads sensíveis ou segredos.

## Regras, validação e segurança

- O backend é autoritativo: clientes enviam intenções, nunca identidade arbitrária, posição, resultado ou transição oficial. Revalide cada mutação contra o estado persistido no instante da execução.
- Valide formato e limites técnicos de entrada na borda; valide permissão, elegibilidade e invariantes na aplicação. Não invente valores para política de senha, tamanho de texto ou rate limit: defina-os separadamente quando o contrato técnico for criado.
- Identifique humanos por sessão autenticada ou sessão válida de convidado; bots são exclusivos do backend. Preserve a distinção entre sessão, presença e conexão e autorize cada ação HTTP e WebSocket, inclusive o papel `CONTROLLER`/`OBSERVER`.
- Use Argon2id para hashes de senha, configuração segura para credenciais e dados externos sempre como entrada não confiável. Não exponha detalhes internos em respostas nem grave segredos no repositório.
- Respeite propriedade e visibilidade de decks e tabuleiros, o sigilo de cartas e gabaritos e o histórico mínimo previsto em `rules.md`. Não introduza pontuação numérica.
- Na primeira entrega, disponibilize apenas `EXACT`. Não exponha tabuleiros com `PLAYER_JUDGE` ou `AI` até implementar a integração de IA e todos os fallbacks exigidos em `rules.md`; não altere essas regras para contornar a dependência.

## Estado, concorrência e recuperação

- PostgreSQL é a fonte do estado oficial das partidas e do histórico. Redis guarda somente sessões, presença, conexões, heartbeats e chat temporário; sua perda não pode mudar posições, avaliações concluídas, vencedor ou estado oficial.
- Use transações curtas, restrições de banco e controle de concorrência para operações compostas, incluindo presença única, entrada, início com snapshot, resposta, avaliação, movimento, vitória e substituição de bots. Repetições e mensagens fora de ordem não podem duplicar efeitos.
- Persista o necessário para reconstruir partidas ativas: snapshot imutável, ordem, rodada, carta, dicas, turno, avaliação, sorteio de bot, estados, prazos ou tempo restante e pausas. Goroutines, canais e temporizadores coordenam a execução, mas não são fonte durável.
- Após reinício, recupere o último estado confirmado sem descontar o período de indisponibilidade dos cronômetros. Respeite também a pausa sem humanos conectados e a tolerância de reconexão conforme `rules.md`.
- Ao chegar a estado terminal, consolide apenas o resultado e histórico permitidos; descarte chat e detalhes transitórios de cartas, turnos e avaliações.
- Crie migrations incrementais com `golang-migrate`, revise o SQL e mantenha consultas `sqlc` e código Go coerentes. Não edite migrations já aplicadas para introduzir mudanças novas.

## HTTP, WebSocket e contratos

- Use HTTP para autenticação, recursos e histórico; use WebSocket para intenções e atualizações da partida. As mesmas regras de identidade e autorização valem nos dois transportes.
- Controle o ciclo de vida das conexões, a escolha de `CONTROLLER`, a reconexão e a ressincronização por snapshot autoritativo. Não considere ordem de chegada, conexão ativa ou estado em memória como confirmação de uma ação.
- Confirme a mutação no PostgreSQL antes de publicar atualizações. Restrinja cada visão aos dados autorizados, especialmente gabaritos e conteúdo de decks. Chat é temporário e não integra o histórico.
- Defina e documente endpoints, eventos e payloads quando forem implementados; mantenha a documentação HTTP em OpenAPI. Não presuma contratos que `rules.md` deixa em aberto.

## Testes e fluxo de trabalho

- Use `go test` para regras puras e integrações relevantes. Priorize máquina de estados, avaliação `EXACT`, concorrência e idempotência, limites de vagas e bots, reconexão, pausas, recuperação e sigilo de dados.
- Para mudanças em SQL ou tempo real, teste os efeitos observáveis e casos de falha com PostgreSQL e Redis quando necessário. Use Docker Compose para o ambiente de desenvolvimento quando ele existir; acrescente OpenTelemetry apenas se houver necessidade de observabilidade.
- Antes de mudar código, leia o módulo afetado e `rules.md`; depois, execute as verificações configuradas e revise o diff. Não declare testes aprovados sem executá-los nem enfraqueça verificações para ocultar falhas.

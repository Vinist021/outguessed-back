
# OutGuessed — Especificação de Regras de Negócio

  

## 1. Objetivo e escopo

  

Este documento é a fonte normativa das regras de negócio da primeira versão do **OutGuessed**, um jogo de perguntas e respostas em tempo real para pessoas e bots.

  

Ele define:

  

- identidade, autenticação, sessão e presença;

- decks, cartas e tabuleiros;

- criação, entrada e condução de partidas;

- rodadas, turnos, dicas, avaliações e movimento;

- estados da partida e condições de encerramento;

- reconexão, abandono, expulsão e entrada tardia;

- inclusão, atuação e remoção de bots para partidas solo ou com poucos jogadores;

- chat, persistência e histórico;

- divisão de responsabilidades entre backend, HTTP, WebSocket, PostgreSQL e Redis.

  

Os termos **deve**, **não deve**, **somente** e **é obrigatório** indicam comportamento normativo.

  

Não fazem parte desta versão:

  

- verificação de e-mail;

- recuperação de senha;

- espectadores;

- equipes;

- administradores ou moderação externa à partida;

- contestação ou revisão de avaliações;

- pontuação numérica;

- contratos concretos de endpoints HTTP, eventos WebSocket ou payloads;

- limites técnicos de tamanho de texto, política de senha e rate limit.

  

---

  

## 2. Princípios fundamentais

  

**RN-001 — Backend autoritativo**

  

O backend é a única autoridade sobre validade de ações, estados, respostas, avaliações, movimento, ordem dos jogadores, resultado e encerramento da partida.

  

**RN-002 — Cliente envia intenções**

  

O cliente envia intenções, como entrar, marcar-se pronto, revelar uma dica ou responder. Ele nunca envia como verdade oficial pontuação, posição, correção, vencedor ou transição de estado.

  

**RN-003 — Identidade confiável**

  

Toda identidade humana usada em uma ação do cliente deve ser obtida do contexto autenticado ou da sessão de convidado validada pelo backend, nunca de um identificador arbitrário informado pelo cliente. A identidade e as ações dos bots são criadas exclusivamente pelo backend.

  

**RN-004 — Responsabilidades de HTTP e WebSocket**

  

HTTP é usado principalmente para autenticação, consulta de histórico e operações de recursos. WebSocket é usado para intenções e atualizações em tempo real da partida. Essa divisão não altera as regras de autorização.

  

**RN-005 — Fontes de estado**

  

PostgreSQL é a fonte do estado autoritativo e do histórico persistente. Redis armazena somente estado temporário de sessão, presença, conexões e chat.

  

**RN-006 — Concorrência e idempotência**

  

Toda mutação deve validar o estado oficial no momento da execução. Entregas repetidas, conexões simultâneas ou mensagens fora de ordem não podem produzir respostas, avaliações, movimentos ou transições duplicadas.

  

---

  

## 3. Glossário

  

| Termo | Definição |

| --- | --- |

| Usuário | Pessoa com conta permanente cadastrada. |

| Convidado | Pessoa identificada por uma sessão temporária, sem conta permanente. |

| Jogador | Competidor humano (usuário ou convidado) ou bot participando de uma partida. |

| Bot | Competidor controlado exclusivamente pelo backend, sem conta ou sessão humana. |

| Competidor jogável | Humano `ACTIVE` conectado ou bot `ACTIVE`; a elegibilidade para a carta ou turno atual é verificada separadamente. Humano `DISCONNECTED` não conta enquanto estiver offline. |

| Sessão | Credencial temporária que identifica um usuário ou convidado. |

| Presença | Vínculo atual de um jogador humano com uma partida. |

| Conexão | Canal WebSocket de um dispositivo; não equivale a sessão nem presença. |

| Anfitrião | Humano responsável pelo controle administrativo: usuário cadastrado no lobby; após o início, também pode ser um convidado sucessor. |

| Deck | Coleção de cartas pertencente a um usuário. |

| Carta | Conteúdo de uma rodada: dica principal, dicas adicionais e resposta esperada. |

| Tabuleiro | Definição das casas e das configurações de jogo aplicáveis à partida. |

| Partida | Sessão multiplayer completa, do lobby ao estado terminal. |

| Rodada | Ciclo completo de uma carta, desde sua seleção até acerto ou encerramento sem movimento. |

| Turno | Oportunidade individual de um jogador dentro de uma rodada. |

| Juiz | Jogador humano que avalia respostas em `PLAYER_JUDGE` ou no fallback da IA. |

| Snapshot | Cópia imutável do deck, das cartas e do tabuleiro usada por uma partida iniciada. |

  

As relações conceituais são:

  

```text

Sessão != Presença != Conexão

  

Partida

└── Rodadas

└── Turnos

```

  

---

  

## 4. Tipos e estados conceituais

  

### 4.1 Estado da partida

  

```text

WAITING

└── STARTING

└── QUESTION

├── EVALUATING

│ └── REVEAL

└── REVEAL

├── QUESTION

└── SCOREBOARD

├── QUESTION

└── FINISHED

  

Qualquer estado não terminal pode chegar a CANCELLED

quando uma regra automática de cancelamento for satisfeita.

```

  

| Estado | Significado |

| --- | --- |

| `WAITING` | Lobby aberto para configuração, entrada e prontidão. |

| `STARTING` | Configurações já congeladas e contagem regressiva inicial em andamento. |

| `QUESTION` | Turno ativo, permitindo ao respondente revelar dica e responder. |

| `EVALUATING` | Resposta registrada e aguardando avaliação. |

| `REVEAL` | Exibição do palpite, resultado e, quando cabível, gabarito. |

| `SCOREBOARD` | Exibição das posições ao fim de uma rodada; não representa pontuação numérica. |

| `FINISHED` | Partida concluída com vencedor. |

| `CANCELLED` | Partida encerrada sem vencedor. |

  

### 4.2 Modos de avaliação

  

| Valor | Significado |

| --- | --- |

| `EXACT` | Comparação textual normalizada pelo backend. |

| `PLAYER_JUDGE` | Avaliação por um jogador juiz fixado para a carta. |

| `AI` | Avaliação por IA, com fallback para juiz humano ou `EXACT` se não houver juiz elegível. |

  

### 4.3 Estado do participante

  

| Estado | Participa da rotação? | Ocupa vaga? | Pode retornar? |

| --- | --- | --- | --- |

| `ACTIVE` | Sim, se já elegível para a rodada | Sim | Já está em jogo |

| `DISCONNECTED` | O turno atual pode continuar; turnos futuros são pulados | Sim | Sim, por reconexão |

| `ABANDONED` | Não | Não | Sim, como entrada tardia |

| `KICKED` | Não | Não | Não |

| `REMOVED` | Não | Não | Não; exclusivo de bot removido |

  

Um humano tardio pode estar `ACTIVE` e possuir `eligibleFromRound` apontando para a rodada seguinte. A substituição de bot que deixaria a carta sem respondente elegível e o bot automático seguem as exceções das RN-106 e RN-134. Durante a pausa por ausência de humanos, o prazo de um turno `DISCONNECTED` também fica suspenso.

  

### 4.4 Papel da conexão

  

| Papel | Permissões |

| --- | --- |

| `CONTROLLER` | Pode enviar intenções do jogador. |

| `OBSERVER` | Pode receber o estado, mas não executar ações da presença. |

  

### 4.5 Estado de rodada, turno e avaliação

  

O modelo persistente deve conseguir representar, sem impor um formato de banco ou DTO:

- rodada ativa ou concluída;

- carta e ciclo de embaralhamento usados;

- iniciador da rodada e jogadores elegíveis;

- dicas adicionais reveladas;

- turno ativo, respondente, início, prazo e término;

- resposta, timeout, passe de bot ou interrupção do turno;

- modo de avaliação efetivo;

- juiz atual e todos os jogadores que tiveram acesso ao gabarito;

- resultado da avaliação;

- motivo de encerramento da rodada;

- movimento produzido e posição resultante.

  

Motivos conceituais de encerramento da rodada:

  

| Motivo | Resultado |

| --- | --- |

| `CORRECT_ANSWER` | A carta foi acertada e produziu movimento. |

| `ZERO_MOVEMENT` | Todas as dicas adicionais foram reveladas e não restou movimento possível. |


  

### 4.6 Motivo de encerramento da partida

  

| Estado terminal | Motivo | Vencedor |

| --- | --- | --- |

| `FINISHED` | `VICTORY` | Obrigatório |

| `CANCELLED` | `HOST_CANCELLED` | Ausente |

| `CANCELLED` | `LOBBY_IDLE_TIMEOUT` | Ausente |

| `CANCELLED` | `NO_ELIGIBLE_HOST` | Ausente |

| `CANCELLED` | `NO_HUMAN_AVAILABLE` | Ausente |

  

---

  

## 5. Identidade, autenticação, sessão e presença

  

### 5.1 Contas e autenticação

  

**RN-007 — Cadastro de conta**

  

Uma conta é criada com e-mail e senha válidos. O e-mail é obrigatório e único no sistema.

  

**RN-008 — Cadastro não autentica**

  

O cadastro concluído não cria sessão autenticada. O usuário deve executar o login posteriormente.

  

**RN-009 — Login**

  

Somente credenciais válidas criam uma sessão autenticada.

  

**RN-010 — Múltiplas sessões**

  

Uma conta pode manter várias sessões simultâneas em dispositivos diferentes.

  

**RN-011 — Expiração e renovação**

  

Sessões autenticadas expiram após 7 dias e sessões de convidado após 24 horas. O prazo é deslizante e renovado por atividade válida da sessão.

  

### 5.2 Convidados e nomes de exibição

  

**RN-012 — Identidade de convidado**

  

Um convidado recebe identificador temporário próprio e escolhe um nome de exibição ao entrar em uma partida.

  

**RN-013 — Nome único na partida**

  

O nome de exibição deve ser único entre todos os participantes da partida, sejam usuários, convidados ou bots. A comparação ignora caixa, acentos e espaços excedentes. Um usuário cadastrado deve escolher outro nome para a partida se houver conflito.

  

### 5.3 Logout e múltiplos dispositivos

  

**RN-014 — Logout local**

  

Logout invalida somente a sessão do dispositivo que o solicitou.

  

**RN-015 — Conexão controladora**

  

A conexão válida mais recente da identidade na partida assume `CONTROLLER`. Outras conexões da mesma identidade ficam como `OBSERVER`.

  

**RN-016 — Transferência entre dispositivos**

  

Quando a conexão controladora encerra sua sessão, a conexão observadora válida mais recente assume o controle. Se não houver outra conexão participante, o logout explícito causa abandono imediato.

  

**RN-017 — Expiração involuntária**

  

Expiração de sessão ou perda involuntária da última conexão inicia a tolerância de reconexão; não é tratada como logout voluntário.

  

### 5.4 Presença

  

**RN-018 — Separação de conceitos**

  

A existência de sessão ou conexão WebSocket não implica presença em partida, e a perda de uma conexão não remove imediatamente a presença.

  

**RN-019 — Presença única**

  

Uma identidade humana pode possuir no máximo uma presença ativa ou reservada em uma partida, independentemente do número de sessões e conexões. Bots não possuem presença humana.

  

**RN-020 — Unicidade atômica**

  

A criação e a troca de presença devem ser validadas atomicamente para impedir entrada concorrente da mesma identidade em duas partidas.

  

---

  

## 6. Decks, cartas e tabuleiros

  

### 6.1 Propriedade e visibilidade

  

**RN-021 — Criação de conteúdo**

  

Somente usuários cadastrados podem criar decks e tabuleiros personalizados.

  

**RN-022 — Proprietário**

  

Todo deck ou tabuleiro personalizado possui exatamente um usuário proprietário.

  

**RN-023 — Edição e exclusão**

  

Somente o proprietário pode alterar ou excluir seu deck, suas cartas ou seu tabuleiro personalizado.

  

**RN-024 — Visibilidade**

  

O proprietário escolhe entre visibilidade `PUBLIC` e `PRIVATE`. Conteúdo privado só pode ser selecionado pelo proprietário; conteúdo público pode ser selecionado por qualquer usuário autenticado.

  

**RN-025 — Conteúdo protegido do deck**

  

Antes da partida, um usuário não proprietário vê apenas metadados do deck público, como nome, descrição, autor e quantidade de cartas. Dicas, cartas e respostas esperadas permanecem ocultas.

  

### 6.2 Cartas e decks válidos

  

**RN-026 — Estrutura da carta**

  

Cada carta contém exatamente uma dica principal pública, uma resposta esperada e uma coleção de dicas adicionais.

  

**RN-027 — Quantidade de dicas**

  

Uma carta válida possui de 2 a 20 dicas adicionais.

  

**RN-028 — Deck utilizável**

  

Um deck precisa conter pelo menos 10 cartas válidas para iniciar uma partida.

  

**RN-029 — Validação no início**

  

Todas as cartas e a quantidade mínima do deck são revalidadas quando o anfitrião solicita o início. Uma falha mantém a partida em `WAITING` e não cria snapshot parcial.

  

### 6.3 Tabuleiros

  

**RN-030 — Origem dos tabuleiros**

  

O sistema oferece tabuleiros oficiais imutáveis. Usuários cadastrados também podem criar tabuleiros personalizados sujeitos às mesmas regras de propriedade e visibilidade dos decks.

  

**RN-031 — Tamanho do tabuleiro**

  

Todo tabuleiro possui entre 20 e 200 casas. A casa inicial é a posição zero e não conta como uma das casas de progresso.

  

**RN-032 — Configuração normativa**

  

Nesta versão, as únicas configurações variáveis do tabuleiro são:

  

| Campo conceitual | Valores válidos |

| --- | --- |

| `boardSize` | Inteiro de 20 a 200 |

| `evaluationMode` | `EXACT`, `PLAYER_JUDGE` ou `AI` |

| `answerTimeSeconds` | `30`, `60` ou `90` |

  

Fórmula de movimento, quantidade de jogadores, ordem, dicas e condições de vitória são regras globais e não podem ser personalizadas pelo tabuleiro.

  

**RN-033 — Tempo selecionado no lobby**

  

O tabuleiro fornece o valor inicial de `answerTimeSeconds`. O anfitrião pode substituí-lo por 30, 60 ou 90 segundos durante `WAITING`; o valor final integra a configuração congelada da partida.

  

### 6.4 Snapshot

  

**RN-034 — Congelamento no início**

  

Na transição para `STARTING`, o backend cria um snapshot imutável de deck, cartas, tabuleiro e configuração final.

  

**RN-035 — Independência de alterações futuras**

  

Alterações ou exclusões posteriores do conteúdo original não afetam partidas que já possuem snapshot.

  

**RN-036 — Validade até o término**

  

O snapshot completo é mantido enquanto a partida estiver ativa. Após o estado terminal, somente os dados definidos para o resultado final precisam permanecer.

  

---

  

## 7. Criação da partida e lobby

  

### 7.1 Criação e código

  

**RN-037 — Quem pode criar**

  

Somente usuário cadastrado pode criar uma partida.

  

**RN-038 — Entrada automática do criador**

  

O criador torna-se anfitrião e jogador da partida, ocupando automaticamente a primeira presença do lobby.

  

**RN-039 — Presença anterior impede criação**

  

A criação falha se o usuário já possuir presença ativa ou reservada em outra partida.

  

**RN-040 — Código da partida**

  

O backend gera um código não editável e único entre partidas não terminais. O código deixa de aceitar entrada no estado terminal e pode ser reutilizado futuramente.

  

### 7.2 Capacidade, entrada e nomes

  

**RN-041 — Limites de jogadores**

  

Uma partida precisa de pelo menos 2 competidores jogáveis para iniciar e para jogar sem pausa. O limite normal é de 8 vagas ocupadas por humanos `ACTIVE` ou `DISCONNECTED` e bots `ACTIVE`. Somente a reposição automática durante desconexões pode criar um nono competidor provisório, sempre bot, conforme a seção 17.

  

**RN-042 — Entrada válida**

  

Para um humano entrar, a partida deve existir, não estar terminal, não ter vitória registrada, aceitar o tipo de entrada solicitado e não bloquear aquela identidade. É necessária uma vaga humana livre ou, se as oito vagas normais estiverem ocupadas e houver bot não provisório, a remoção atômica do bot adicionado mais recentemente. Um bot provisório na nona vaga não permite uma nona vaga humana: nesse caso, a nova entrada é rejeitada até a liberação de uma das oito vagas normais.

  

**RN-043 — Uma posição por identidade**

  

Uma identidade não pode ocupar simultaneamente duas posições na mesma partida.

  

**RN-044 — Nome disponível**

  

A entrada somente é concluída depois de validar a unicidade normalizada do nome de exibição, inclusive contra os nomes visíveis dos bots.

  

### 7.3 Configuração e prontidão

  

**RN-045 — Alterações no lobby**

  

Durante `WAITING`, o anfitrião pode trocar o deck, o tabuleiro ou o tempo de resposta entre os valores permitidos, além de adicionar e remover bots dentro do limite normal de oito vagas.

  

**RN-046 — Alteração remove prontidão**

  

Qualquer alteração de deck, tabuleiro, tempo ou quantidade de bots remove a marcação de pronto de todos os jogadores humanos não anfitriões.

  

**RN-047 — Prontidão para iniciar**

  

Para iniciar, devem existir de 2 a 8 competidores, incluindo pelo menos o anfitrião humano conectado. Todos os demais humanos devem estar conectados e marcados como prontos; bots estão sempre prontos. Um humano `DISCONNECTED` com vaga reservada impede o início até reconectar ou ser removido, mesmo que haja bots suficientes.

  

**RN-048 — Anfitrião não marca pronto**

  

O comando de início representa a confirmação do anfitrião; ele não precisa de uma marcação de pronto separada.

  

**RN-049 — Validação autoritativa do início**

  

O backend revalida humanos e bots, presenças humanas, prontidão, capacidade, deck, cartas, tabuleiro e configuração antes de aceitar o início.

  

**RN-050 — Falha de início**

  

Se qualquer pré-condição falhar, a partida permanece em `WAITING`, nenhum snapshot parcial é publicado e o backend informa o motivo da rejeição.

  

### 7.4 Inatividade do lobby

  

**RN-051 — Expiração do lobby**

  

Uma partida em `WAITING` sem presença conectada nem atividade válida por 30 minutos é movida para `CANCELLED` com motivo `LOBBY_IDLE_TIMEOUT`.

  

Heartbeats de conexões válidas e ações válidas do lobby renovam a atividade. Tentativas rejeitadas não renovam o prazo.

  

---

  

## 8. Poderes e sucessão do anfitrião

  

**RN-052 — Cancelamento manual**

  

O anfitrião pode cancelar manualmente somente em `WAITING`. O resultado é `CANCELLED` com motivo `HOST_CANCELLED`.

  

**RN-053 — Expulsão**

  

O anfitrião pode expulsar outro participante humano em qualquer estado não terminal. A remoção de bots segue as regras específicas da seção 17.

  

**RN-054 — Consequências da expulsão**

  

O participante humano expulso recebe estado `KICKED`, sai da rotação, libera sua presença e vaga, preserva sua posição para o resultado e fica impedido de retornar à mesma partida.

  

**RN-055 — Expulsão do respondente**

  

Se o expulso for o respondente atual, seu turno é interrompido sem resposta correta nem movimento. A partida segue para `REVEAL` e depois para o próximo respondente, salvo se outra regra causar cancelamento.

  

**RN-056 — Expulsão do juiz**

  

Se o expulso for o juiz, o backend seleciona o próximo juiz humano elegível. Se não houver substituto, a avaliação pendente passa para IA com fallback `EXACT`, sem encerrar a rodada por falta de juiz.

  

**RN-057 — Saída do anfitrião**

  

O anfitrião mantém o papel durante a tolerância de desconexão. Se abandonar, o papel passa ao próximo humano ativo e conectado com conta cadastrada na ordem cíclica. Depois de `STARTING`, se não houver cadastrado elegível, passa ao próximo convidado ativo e conectado. Bots nunca assumem esse papel.

  

**RN-058 — Ausência de sucessor**

  

Em `WAITING`, se não houver usuário cadastrado elegível para assumir, a partida vai para `CANCELLED` com motivo `NO_ELIGIBLE_HOST`, mesmo que convidados permaneçam. Depois do início, a ausência temporária de sucessor conectado segue a pausa e o cancelamento por falta de humano disponível da seção 17.

  

**RN-059 — Quantidade insuficiente**

  

Depois do início, se restar um humano conectado e menos de dois competidores jogáveis, o backend adiciona imediatamente um bot. Humanos `DISCONNECTED`, `ABANDONED` e `KICKED` não contam como jogáveis; bots `ACTIVE` contam. Sem nenhum humano conectado, aplicam-se a pausa e o cancelamento da seção 17, em vez de permitir que bots joguem sozinhos.

  

---

  

## 9. Início e máquina de estados

  

**RN-060 — Transição inicial**

  

Após validação e snapshot bem-sucedidos, a partida muda de `WAITING` para `STARTING`.

  

**RN-061 — Duração de STARTING**

  

`STARTING` dura 3 segundos e avança automaticamente para `QUESTION` sob controle do backend.

  

**RN-062 — Estado de avaliação**

  

Uma resposta aceita leva a partida para `EVALUATING`. Enquanto estiver nesse estado, não são aceitas novas respostas ou revelações de dica. A remoção de bot pode descartar sua resposta ainda pendente, conforme a seção 17.

  

Em `EXACT`, a avaliação pode ser imediata, mas a transição lógica ainda deve ser registrada de forma atômica.

  

**RN-063 — Duração de REVEAL**

  

`REVEAL` dura 5 segundos. Nenhuma ação de jogo altera o resultado durante esse período.

  

**RN-064 — Fluxo após erro**

  

Resposta incorreta, timeout, passe de bot ou interrupção do respondente passa por `REVEAL` e retorna a `QUESTION` com o próximo respondente da mesma rodada. Não há `SCOREBOARD` enquanto a carta continuar ativa.

  

**RN-065 — Duração e função de SCOREBOARD**

  

`SCOREBOARD` dura 5 segundos e ocorre somente quando a rodada termina. Ele exibe posições e resultado da rodada, nunca pontuação numérica.

  

**RN-066 — Próxima rodada**

  

Depois de `SCOREBOARD`, o backend inicia outra rodada em `QUESTION`, exceto quando uma vitória já tiver sido registrada.

  

**RN-067 — Sequência de vitória**

  

Ao alcançar a última casa, a vitória é registrada imediatamente, inclusive para bot. A partida ainda executa o último `REVEAL` e `SCOREBOARD` antes de entrar em `FINISHED`. Depois do registro da vitória, novas entradas e remoções de bots são rejeitadas; a reconexão de humanos já participantes continua permitida para acompanhar o resultado.

  

**RN-068 — Estados terminais**

  

`FINISHED` é exclusivo de partida com vencedor por chegada ao fim. Todo encerramento sem vencedor usa `CANCELLED` com motivo explícito.

  

**RN-069 — Imutabilidade terminal**

  

`FINISHED` e `CANCELLED` não aceitam entrada, chat ou mutações de jogo. Somente consultas autorizadas ao resultado permanecem disponíveis.

  

---

  

## 10. Ordem dos jogadores, rodadas e cartas

  

### 10.1 Ordem

  

**RN-070 — Sorteio inicial**

  

Ao entrar em `STARTING`, o backend sorteia uma vez a ordem dos humanos e bots existentes. Essa ordem permanece cíclica durante a partida, com entradas tardias anexadas ao final.

  

**RN-071 — Iniciador da rodada**

  

A primeira rodada começa com o primeiro jogador sorteado. Cada rodada posterior começa com o jogador seguinte ao iniciador da rodada anterior, independentemente de quem realizou o último turno ou acertou a carta.

  

**RN-072 — Participantes removidos ou indisponíveis**

  

Jogadores `ABANDONED`, `KICKED`, `REMOVED`, ainda não elegíveis ou desconectados fora do turno corrente são pulados ao selecionar respondentes. Bots adicionados durante uma rodada podem ser selecionados já no próximo turno dessa carta.

  

### 10.2 Seleção de cartas

  

**RN-073 — Sorteio sem reposição**

  

Cada rodada seleciona aleatoriamente uma carta ainda não usada no ciclo atual do snapshot do deck.

  

**RN-074 — Novo ciclo do deck**

  

Depois que todas as cartas forem usadas, o backend reembaralha o deck completo e inicia novo ciclo. Uma carta não se repete antes do esgotamento do ciclo atual.

  

### 10.3 Duração da rodada

  

**RN-075 — Uma carta por rodada**

  

Uma rodada utiliza uma única carta, que permanece ativa durante quantos turnos forem necessários até ocorrer `CORRECT_ANSWER` ou `ZERO_MOVEMENT`.

  

**RN-076 — Um respondente por turno**

  

Somente o jogador selecionado para o turno pode revelar dica adicional e responder. Todos os humanos conectados veem a dica principal e todas as dicas adicionais já reveladas; bots usam o mesmo estado autoritativo sem conexão própria.

  

**RN-077 — Rotação dentro da rodada**

  

Após um turno sem acerto, o próximo respondente elegível é selecionado pela ordem cíclica. No modo juiz, o juiz e qualquer jogador que tenha visto o gabarito são pulados.

  

---

  

## 11. Turno, dicas, resposta e movimento

  

### 11.1 Cronômetro e ação do turno

  

**RN-078 — Prazo do turno**

  

Cada turno utiliza o `answerTimeSeconds` congelado na configuração da partida.

  

**RN-079 — Dica opcional**

  

Antes de responder, o jogador humano pode escolher e revelar no máximo uma dica adicional ainda oculta. O bot revela uma dica no início de seu turno sempre que houver alguma, conforme a seção 17.

  

**RN-080 — Dica não altera o prazo**

  

Revelar uma dica não pausa, reinicia nem acrescenta tempo ao cronômetro.

  

**RN-081 — Uma resposta por turno**

  

O primeiro envio de resposta válido encerra a possibilidade de novas respostas naquele turno. Reenvios e entregas duplicadas não produzem nova avaliação.

  

**RN-082 — Novas tentativas em turnos futuros**

  

Um humano que errou pode responder novamente quando a ordem retornar a ele, inclusive sem que uma nova dica tenha sido revelada.

  

**RN-083 — Ausência de progressão forçada**

  

Nos turnos humanos, o backend não revela dicas automaticamente e não limita a quantidade total de turnos da carta. Em partida sem bot, se ninguém optar por nova dica e ninguém acertar, a rodada pode continuar indefinidamente. A revelação obrigatória no turno do bot é a exceção definida na seção 17.

  

### 11.2 Fórmula de movimento

  

Considere:

  

-  `H`: quantidade total de dicas adicionais da carta;

-  `R`: quantidade de dicas adicionais já reveladas, incluindo a revelada no turno atual.

  

```text

movimento por acerto = H - R

```

  

A dica principal é sempre pública e não integra `R`.

  

**RN-084 — Movimento por acerto**

  

Uma resposta correta move o respondente exatamente `H - R` casas.

  

**RN-085 — Erro e timeout**

  

Resposta incorreta, ausência de resposta no prazo, passe de bot ou interrupção do turno não produz movimento, recuo ou outra penalidade de posição.

  

**RN-086 — Última dica**

  

Se a revelação fizer `R = H`, o movimento possível torna-se zero e a rodada termina imediatamente com `ZERO_MOVEMENT`, sem aceitar resposta naquele turno.

  

**RN-087 — Limite do tabuleiro**

  

Se o movimento alcançar ou ultrapassar a última casa, a posição é limitada à última casa e o jogador vence. Não é necessário obter movimento exato e não existe retorno do excedente.

  

**RN-088 — Atualização atômica**

  

Avaliação correta, movimento, nova posição, encerramento da rodada e eventual vitória devem ser registrados como uma única operação consistente.

  

---

  

## 12. Avaliação de respostas

  

### 12.1 Regra comum

  

**RN-089 — Modo fixo da partida**

  

O `evaluationMode` vem da configuração congelada do tabuleiro e vale para toda a partida. O jogador não escolhe o modo durante a rodada. Respostas de bots seguem o sorteio autoritativo da seção 17; respostas humanas usam o modo configurado e seus fallbacks.

  

**RN-090 — Resultado definitivo**

  

Uma avaliação concluída é definitiva e não pode ser contestada ou revertida pelo anfitrião ou pelos demais jogadores, inclusive se o respondente for um bot posteriormente removido.

  

**RN-091 — Registro da avaliação**

  

O backend registra a resposta, o modo efetivamente usado, o resultado, o avaliador quando houver e os timestamps necessários antes de aplicar movimento. Uma resposta de bot ainda pendente em `EVALUATING` pode ser descartada por remoção imediata; nesse caso, não produz avaliação concluída nem movimento.

  

### 12.2 Valor exato

  

**RN-092 — Normalização textual**

  

No modo `EXACT`, resposta enviada e resposta esperada passam pela mesma normalização antes da comparação:

  

1. normalização Unicode;

2. conversão uniforme de caixa;

3. remoção de diacríticos;

4. substituição de pontuação por separadores;

5. remoção de espaços nas bordas;

6. redução de sequências de espaços a um único espaço.

  

Após a normalização, os textos devem ser iguais. Cada carta possui uma única resposta esperada nesta versão.

  

### 12.3 Jogador juiz

  

**RN-093 — Seleção do juiz**

  

Em `PLAYER_JUDGE`, o juiz é escolhido somente quando a primeira resposta humana precisar de avaliação. O primeiro humano ativo, conectado e elegível depois do iniciador na ordem cíclica, diferente do respondente e sem acesso prévio ao gabarito, torna-se juiz fixo da carta. Respostas de bots dispensam juiz; se não houver humano elegível, aplica-se a avaliação automática da RN-097.

  

**RN-094 — Acesso ao gabarito e inelegibilidade**

  

O juiz recebe a resposta esperada e não pode ser respondente naquela rodada. Todo substituto que receba o gabarito também fica inelegível para responder à carta.

  

**RN-095 — Prazo do juiz**

  

Depois que uma resposta aguarda julgamento, o juiz possui 30 segundos para marcá-la correta ou incorreta.

  

**RN-096 — Substituição do juiz**

  

Se o juiz estiver indisponível, for removido ou não decidir no prazo, o backend escolhe, pela ordem cíclica, o próximo humano ativo, conectado e elegível que não seja o respondente atual e ainda não tenha visto o gabarito. O substituto torna-se o juiz fixo da carta, fica inelegível para respondê-la e recebe um novo prazo integral de 30 segundos.

  

**RN-097 — Ausência de juiz elegível**

  

Se não houver juiz humano elegível, inclusive em partida solo, a resposta humana pendente é avaliada por IA em até 15 segundos. Falha, timeout ou resultado inválido da IA aciona comparação `EXACT` da RN-092. A rodada continua com o resultado obtido; não se encerra por ausência de juiz.

  

### 12.4 Inteligência artificial

  

**RN-098 — Prazo da IA**

  

Em `AI` e na avaliação automática da RN-097, o backend aguarda por no máximo 15 segundos uma resposta válida e binária do mecanismo de avaliação.

  

**RN-099 — Saída da IA não é confiável**

  

O backend deve validar e normalizar a saída da IA antes de usá-la. Falha, timeout, formato inválido ou resultado inconclusivo acionam fallback.

  

**RN-100 — Fallback para juiz**

  

No fallback do modo `AI`, o próximo humano elegível após o respondente atual torna-se juiz. A rodada passa a usar `PLAYER_JUDGE` até a carta terminar, e quem acessar o gabarito não pode responder àquela carta. Se não existir juiz humano elegível, o backend usa `EXACT` da RN-092. Na avaliação automática da RN-097, a falha da IA leva diretamente a `EXACT`.

  

### 12.5 Visibilidade das respostas

  

**RN-101 — Palpite incorreto público**

  

Após uma avaliação incorreta, todos os humanos conectados veem o texto enviado e o resultado incorreto durante `REVEAL`.

  

**RN-102 — Segredo do gabarito**

  

O gabarito permanece oculto dos respondentes humanos até a rodada terminar. Ele é revelado ao fim por acerto ou movimento zero. O uso interno da resposta esperada para um acerto sorteado de bot não concede acesso ao gabarito a outro jogador.

  

---

  

## 13. Entrada tardia

  

**RN-103 — Entrada durante a partida**

  

Novos humanos podem entrar em `STARTING`, `QUESTION`, `EVALUATING`, `REVEAL` ou `SCOREBOARD`, desde que a partida não esteja terminal, não tenha vitória registrada e haja vaga segundo a RN-042. Bots são adicionados manualmente apenas em `WAITING` ou automaticamente após o início conforme a seção 17.

  

**RN-104 — Posição inicial tardia**

  

O humano tardio e o bot adicionado após o início entram na posição zero, independentemente da posição dos demais.

  

**RN-105 — Inserção na ordem**

  

O humano tardio e o bot adicionado após o início são anexados ao final da ordem sorteada existente. A ordem dos participantes anteriores não é alterada.

  

**RN-106 — Elegibilidade a partir da próxima rodada**

  

O humano tardio recebe o estado atual para acompanhamento, mas em regra só pode ser selecionado como respondente ou juiz a partir da rodada seguinte à sua entrada. Se substituir um bot e não restar outro respondente elegível na carta atual, torna-se elegível já na próxima seleção de turno dessa carta para evitar bloqueio da rodada. O bot adicionado automaticamente pode responder na rodada atual, a partir da próxima seleção de turno, e nunca pode ser juiz.

  

**RN-107 — Capacidade reservada**

  

Humanos `ACTIVE` e `DISCONNECTED` e bots `ACTIVE` contam para o limite normal de oito. Humanos `ABANDONED` ou `KICKED` e bots `REMOVED` não ocupam vaga. A nona vaga provisória é exclusiva do bot automático da seção 17.

  

---

  

## 14. Desconexão, reconexão e abandono

  

### 14.1 Desconexão involuntária

  

**RN-108 — Tolerância de reconexão**

  

A perda involuntária da última conexão válida altera o participante para `DISCONNECTED` e reserva identidade, vaga e presença por 2 minutos.

  

**RN-109 — Turno corrente**

  

Se o humano desconectar durante seu turno e ainda houver outra pessoa conectada, o cronômetro continua normalmente. Ele pode retomar o turno se reconectar antes do prazo; caso contrário, ocorre timeout sem movimento. Se ninguém permanecer conectado, os cronômetros são pausados conforme a seção 17.

  

**RN-110 — Turnos futuros offline**

  

Enquanto estiver `DISCONNECTED`, o participante é pulado em turnos posteriores e em novos papéis de juiz.

  

**RN-111 — Expiração da tolerância**

  

Se não reconectar em 2 minutos, o humano passa para `ABANDONED`, sai da rotação e libera presença e vaga. Sua posição e participação permanecem disponíveis para o resultado. Se não houver outra pessoa conectada nem reserva válida, a partida é cancelada por `NO_HUMAN_AVAILABLE`.

  

### 14.2 Saída voluntária e retorno

  

**RN-112 — Saída voluntária**

  

Sair explicitamente da partida ou fazer logout da última sessão participante causa abandono imediato, sem tolerância. Se nenhuma pessoa permanecer conectada ou dentro da tolerância de reconexão, a partida iniciada é cancelada imediatamente por `NO_HUMAN_AVAILABLE`.

  

**RN-113 — Retorno de abandonado**

  

Um participante humano `ABANDONED`, mas não `KICKED`, pode retornar enquanto a partida estiver ativa e sem vitória registrada, desde que não esteja presente em outra partida e exista vaga livre ou substituível por bot conforme a RN-042.

  

**RN-114 — Reativação do mesmo registro**

  

O retorno reativa a mesma identidade de participante, preserva os eventos anteriores, redefine sua posição atual para zero, move-a para o final da ordem e aplica elegibilidade somente na rodada seguinte, salvo se substituir bot e incidir a exceção da RN-106.

  

Um convidado só recupera a mesma identidade enquanto sua sessão temporária ainda for válida.

  

### 14.3 Ressincronização

  

**RN-115 — Snapshot de reconexão**

  

Ao reconectar, o cliente recebe um snapshot autoritativo contendo ao menos estado da partida, participantes identificados explicitamente como humanos ou bots, posições, rodada, carta visível, dicas reveladas, turno, prazos, eventual pausa por ausência de humanos e chat temporário.

  

O cliente deve substituir estado local divergente pelo snapshot recebido.

  

---

  

## 15. Chat

  

**RN-116 — Participantes autorizados**

  

Todos os participantes humanos ativos ou temporariamente desconectados podem receber o chat enquanto a partida não estiver terminal. Somente conexões autenticadas e atualmente conectadas podem enviar. Bots não enviam nem recebem mensagens de chat.

  

**RN-117 — Chat durante perguntas**

  

O chat permanece habilitado em todas as fases não terminais, inclusive `QUESTION` e `EVALUATING`. Colaboração, sugestões e blefes não são proibidos pela regra de jogo.

  

**RN-118 — Distribuição autoritativa**

  

O backend valida o remetente, associa a mensagem à partida e a distribui aos participantes. O cliente não escolhe arbitrariamente outra identidade ou partida de destino.

  

**RN-119 — Recuperação temporária**

  

Reconectados e participantes tardios recebem todas as mensagens ainda retidas da partida em andamento.

  

**RN-120 — Descarte terminal**

  

O chat é descartado quando a partida entra em `FINISHED` ou `CANCELLED` e não faz parte do histórico persistente.

  

---

  

## 16. Persistência, recuperação e histórico

  

### 16.1 Estado autoritativo ativo

  

**RN-121 — Persistência da partida ativa**

  

PostgreSQL deve manter o estado necessário para recuperar uma partida ativa, incluindo:

  

- humanos e bots, seus estados, anfitrião, ordem e eventual vaga provisória;

- configuração e snapshots do deck e tabuleiro;

- ciclo e cartas já usadas;

- rodada, carta, iniciador e dicas reveladas;

- turno, respondente, tempo restante, ação e sorteio definitivo de bot;

- juiz, jogadores que viram o gabarito e avaliação pendente ou concluída;

- posições e eventual vencedor registrado;

- estado da partida, eventual pausa por ausência de humanos e motivo terminal, quando aplicável.

  

**RN-122 — Papel do Redis**

  

Redis mantém sessões, presença, conexões controladoras/observadoras, heartbeats e chat temporário. Perda de Redis não pode alterar posições, avaliações concluídas, vencedor ou estado persistido da partida.

  

### 16.2 Reinício do backend

  

**RN-123 — Recuperação após indisponibilidade**

  

Após reinício ou falha do backend, a partida é reconstruída a partir do último estado autoritativo persistido.

  

**RN-124 — Pausa de cronômetros**

  

O tempo de indisponibilidade do backend não é descontado dos jogadores. O tempo restante de turno, IA, juiz, ação de bot ou fase automática é persistido e retomado quando o serviço voltar. A pausa por ausência de humanos, por si só, não suspende a tolerância de reconexão de 2 minutos; se a indisponibilidade do backend impedir a reconexão, o prazo restante da tolerância também é preservado até o serviço voltar.

  

### 16.3 Resultado e histórico

  

**RN-125 — Resultado de partida finalizada**

  

O resultado de `FINISHED` contém todos os humanos que participaram, inclusive `ABANDONED` e `KICKED`, e os bots não removidos, seus tipos de participante, posições finais, vencedor humano ou bot e motivo `VICTORY`. Bots `REMOVED` não constam do resultado.

  

**RN-126 — Resultado de partida cancelada**

  

O resultado de `CANCELLED` contém todos os humanos que participaram, inclusive `ABANDONED` e `KICKED`, e os bots não removidos, seus tipos de participante, posições no momento do cancelamento e motivo, sem vencedor. Bots `REMOVED` não constam do resultado.

  

**RN-127 — Classificação**

  

A classificação inclui todos os humanos que participaram e os bots não removidos, ordenados pela maior posição. Jogadores na mesma casa compartilham a mesma colocação; não existe desempate por acertos, ordem ou tempo.

  

**RN-128 — Histórico mínimo**

  

Depois do estado terminal, permanecem apenas identificação da partida, estado e motivo terminal, participantes e seus tipos constantes do resultado, posições, vencedor opcional e dados temporais essenciais do resultado. Humanos abandonados ou expulsos permanecem; bots removidos são excluídos do histórico final.

  

Detalhes de cartas, dicas, turnos, palpites, avaliações, movimentos intermediários e chat não fazem parte do histórico permanente.

  

**RN-129 — Ausência de round_results permanente**

  

Não existe histórico permanente separado de `round_results`. Resultados de rodada podem existir durante a partida para consistência e recuperação, mas são descartados após a consolidação do resultado final.

  

**RN-130 — Consulta por usuários cadastrados**

  

Um participante autenticado pode consultar o resultado final das partidas das quais participou, inclusive se abandonou ou foi expulso.

  

**RN-131 — Consulta por convidados**

  

Um convidado pode consultar o resultado somente enquanto sua sessão temporária válida permitir comprovar a identidade participante.

  

---

  

## 17. Bots e partidas solo

### 17.1 Identidade, inclusão e vagas

**RN-132 — Identidade do bot**

Cada bot tem identificador próprio, tipo de participante explicitamente identificado como bot e nome visível único na partida, atribuído pelo backend como `Bot 1`, `Bot 2` e assim por diante, pulando nomes já usados na partida segundo a normalização da RN-013. Nomes de bots removidos não são reutilizados na mesma partida. Bot não possui conta, sessão, presença humana, conexão, chat, prontidão manual nem poderes de anfitrião ou juiz. Somente o backend executa suas ações.

**RN-133 — Inclusão manual no lobby**

Em `WAITING`, o anfitrião pode adicionar bots até completar oito competidores ou removê-los, inclusive deixando menos de dois competidores no lobby. Cada bot ocupa uma vaga normal, começa na posição zero e está sempre pronto. Toda inclusão ou remoção desmarca a prontidão dos demais humanos; o início só é aceito quando as condições da RN-047 voltarem a ser atendidas.

**RN-134 — Reposição automática após o início**

De `STARTING` até o estado terminal, se restar exatamente um humano conectado e menos de dois competidores jogáveis, o backend adiciona um bot imediatamente e de forma atômica com a mudança que causou a falta de competidores. Humanos desconectados conservam a própria vaga durante a tolerância, mas não são jogáveis. O bot tardio entra na posição zero, ao fim da ordem e pode agir na carta atual a partir da próxima seleção de turno, sem interromper o turno em andamento.

**RN-135 — Nona vaga provisória**

Quando as oito vagas normais já estiverem ocupadas ou reservadas, a RN-134 pode criar um único bot provisório como nono competidor. Essa vaga nunca admite humano. Antes de uma vitória, se uma vaga normal for liberada enquanto o bot ainda for necessário, ele passa a ocupá-la; se deixar de ser necessário e a partida exceder oito competidores, o bot provisório é removido imediatamente. Não pode haver mais de um bot provisório.

**RN-136 — Entrada humana tem prioridade**

Se as oito vagas normais estiverem ocupadas e houver bot não provisório, a entrada de um humano remove atomicamente o bot adicionado mais recentemente para liberar vaga, mesmo que ele esteja no turno. Com um bot provisório na nona vaga e oito vagas humanas ocupadas ou reservadas, a nova entrada é rejeitada até a liberação de uma vaga humana. Quando houver vaga livre, humanos entram sem remover bots; a volta de um humano que já tinha vaga reservada não cria vaga nova. Fora da condição de excesso da RN-135, bots existentes permanecem após reconexões.

**RN-137 — Remoção de bot**

O anfitrião pode remover bots em qualquer estado não terminal antes do registro de vitória. Depois do início, a remoção isolada é rejeitada se deixar menos de dois competidores jogáveis; a substituição atômica por humano é permitida. Se o bot estiver em `QUESTION`, seu turno é interrompido sem movimento e segue para `REVEAL`; se sua resposta ainda estiver em `EVALUATING`, ela é descartada sem resultado nem movimento e o fluxo segue para `REVEAL`. Dicas já reveladas, avaliações concluídas e movimentos registrados não são revertidos. O bot removido recebe `REMOVED`, libera a vaga, desaparece da classificação ativa e não consta do resultado final.

### 17.2 Turno e avaliação

**RN-138 — Dica do bot**

No início de cada turno do bot, o backend revela a primeira dica adicional ainda oculta na ordem da carta, se houver. Essa revelação conta no limite de uma dica por turno. Se for a última, a rodada termina imediatamente com `ZERO_MOVEMENT`, sem aguardar cinco segundos nem aceitar resposta.

**RN-139 — Sorteio de acerto**

Se a rodada continuar, cinco segundos após o início do turno o backend realiza um único sorteio uniforme. Com `R` dicas adicionais já reveladas de um total `H`, a probabilidade de acerto é `0,10 + 0,50 × R/H`. No acerto, o bot envia a resposta esperada; caso contrário, passa sem enviar palpite. O passe não movimenta o bot e segue por `REVEAL`, que informa o passe sem revelar o gabarito, até o próximo respondente.

**RN-140 — Resultado autoritativo do bot**

O sorteio da RN-139 determina definitivamente o resultado da resposta do bot, sem julgamento humano ou da IA. O acerto usa a fórmula comum de movimento e pode registrar vitória. O backend persiste o sorteio, a ação e o resultado antes de avançar, para que reenvios e reinícios não gerem novo sorteio nem ação duplicada. A remoção durante `EVALUATING` só descarta um resultado ainda não concluído.

### 17.3 Ausência de humanos e sucessão

**RN-141 — Pausa sem humano conectado**

Se não houver humano conectado em partida iniciada e nenhuma vitória registrada, o backend pausa turnos, avaliações, ações de bot e fases automáticas, preservando seus tempos restantes. A tolerância individual de reconexão de dois minutos continua correndo enquanto o backend estiver disponível. Reconexão ou entrada válida de qualquer humano retoma a partida; quando todas as reservas expirarem sem retorno, ou se a última pessoa sair voluntariamente sem outra reserva, a partida é cancelada com `NO_HUMAN_AVAILABLE`. Bots não jogam sozinhos. Se já houver vitória registrada, `REVEAL` e `SCOREBOARD` prosseguem até `FINISHED`, mesmo sem humano conectado; a vitória não pode ser substituída por cancelamento.

**RN-142 — Sucessão após o início**

No lobby, somente um usuário cadastrado pode ser anfitrião. Depois do início, na saída definitiva do anfitrião, um humano cadastrado ativo e conectado tem prioridade; se não houver, o próximo convidado ativo e conectado na ordem assume os poderes do anfitrião. Bots são ignorados. Se não houver sucessor conectado, a sucessão aguarda a reconexão ou entrada de um humano, ou o cancelamento da RN-141.

---

## 18. Fluxos normativos resumidos

  

### 18.1 Criação e início

  

```text

Usuário autenticado sem outra presença

-> cria partida e entra como anfitrião

-> seleciona deck, tabuleiro e tempo

-> humanos entram e ficam prontos; o anfitrião pode incluir bots já prontos

-> backend revalida todas as condições

-> cria snapshots e sorteia a ordem

-> STARTING por 3 segundos

-> inicia a primeira rodada em QUESTION

```

  

### 18.2 Turno com resposta incorreta

  

```text

QUESTION

-> jogador pode revelar até uma dica

-> envia uma resposta

-> EVALUATING

-> resultado incorreto

-> REVEAL por 5 segundos

-> próximo respondente da mesma carta

-> QUESTION

```

  

### 18.3 Timeout

  

```text

QUESTION

-> prazo expira

-> nenhum movimento

-> REVEAL por 5 segundos

-> próximo respondente da mesma carta

-> QUESTION

```

  

### 18.4 Acerto sem vitória

  

```text

QUESTION

-> EVALUATING

-> resposta correta

-> movimento = dicas adicionais - dicas reveladas

-> REVEAL por 5 segundos

-> SCOREBOARD por 5 segundos

-> próxima carta

-> QUESTION

```

  

### 18.5 Acerto com vitória

  

```text

QUESTION

-> EVALUATING

-> resposta correta

-> posição alcança ou ultrapassa a última casa

-> registra vencedor

-> REVEAL por 5 segundos

-> SCOREBOARD por 5 segundos

-> FINISHED

```

  

### 18.6 Esgotamento de dicas

  

```text

QUESTION

-> jogador revela a última dica adicional

-> movimento possível torna-se zero

-> não aceita resposta

-> revela o gabarito

-> REVEAL por 5 segundos

-> SCOREBOARD por 5 segundos

-> próxima carta

```

  

### 18.7 Desconexão

  

```text

Perda da última conexão

-> DISCONNECTED por até 2 minutos

-> se resta um humano conectado sem outro competidor jogável: bot entra imediatamente

-> turno atual continua contando enquanto há humano conectado

-> turnos futuros do desconectado são pulados

-> reconectou: snapshot e ACTIVE; bot permanece se couber no limite

-> não reconectou: ABANDONED e presença liberada

```

### 18.8 Partida solo e turno do bot

```text

Anfitrião cria partida e adiciona bot no lobby

-> inicia com dois competidores

-> no turno do bot, revela a primeira dica adicional oculta

-> se for a última: ZERO_MOVEMENT e fim da rodada

-> caso contrário, após 5 segundos sorteia acerto com chance 10% + 50% × R/H

-> acerto: resposta esperada, movimento e eventual vitória

-> erro: passe sem palpite nem movimento, REVEAL e próximo turno

```

### 18.9 Todos os humanos desconectados

```text

Último humano conectado perde a conexão, sem vitória registrada

-> partida e cronômetros pausam; reservas de reconexão continuam correndo

-> humano reconecta ou entra: partida retoma com os tempos restantes

-> nenhuma reserva resta: CANCELLED por NO_HUMAN_AVAILABLE

```

### 18.10 Entrada humana em partida cheia com bot

```text

Oito vagas normais ocupadas, incluindo bot não provisório

-> humano solicita entrada

-> bot adicionado mais recentemente sai imediatamente

-> resposta pendente do bot é descartada; resultado concluído permanece

-> humano entra na vaga liberada

```

  

---

  

## 19. Limites da especificação técnica

  

Este documento determina comportamento e responsabilidades, mas não congela:

  

- caminhos ou métodos de endpoints;

- nomes de eventos WebSocket;

- formatos de DTOs e payloads;

- nomes de tabelas, colunas ou chaves Redis;

- mecanismo específico de autenticação;

- provedor ou modelo de IA;

- bibliotecas, frameworks ou estratégia de implantação.

  

Esses contratos devem ser documentados separadamente e precisam respeitar integralmente as regras deste arquivo.

# Estado temporário no Redis

Este documento complementa [DER.txt](DER.txt). Ele descreve dados conceituais do Redis, não tabelas PostgreSQL nem um contrato definitivo de nomes de chaves. As regras normativas estão em [rules.md](rules.md).

| Conjunto lógico | Conteúdo necessário | Vida útil |
| --- | --- | --- |
| Sessões de usuários | Identificador da sessão, usuário, hash da credencial e validade | Sete dias com renovação deslizante por atividade válida; logout invalida só a sessão local. |
| Sessões de convidados | Identificador temporário, hash da credencial e validade | Vinte e quatro horas com renovação deslizante. O identificador validado corresponde a `match_players.guest_identity_id`. |
| Presença operacional | Identidade humana, partida e vínculo temporário de participação | Enquanto estiver ativa ou reservada; removida no abandono, expulsão ou término. |
| Conexões WebSocket | Sessão, identidade, partida, dispositivo, instante de conexão e papel `CONTROLLER` ou `OBSERVER` | Enquanto a conexão válida existir. A conexão válida mais recente controla; uma observadora assume se a controladora sair. |
| Heartbeats | Última atividade válida da conexão e do lobby | Expiram conforme a conexão; heartbeats válidos renovam a atividade do lobby usada na RN-051. |
| Chat da partida | Mensagens ordenadas, remetente humano validado, conteúdo e instante de envio | Temporário durante a partida; todas as mensagens ainda retidas são entregues a quem reconecta ou entra depois. Excluir em `FINISHED` ou `CANCELLED`. |

## Limites de responsabilidade

- O PostgreSQL é a autoridade para estado da partida, participantes, posições, avaliações, vencedor e resultado. A tabela `human_presence_claims` é apenas a guarda transacional de unicidade entre partidas; o vínculo operacional da presença e as conexões ficam no Redis.
- Uma sessão ou conexão não cria presença por si só. Ao entrar ou voltar, o backend valida a sessão e o estado persistido antes de atualizar a visão temporária.
- Bots não têm sessão, presença humana, conexão ou chat. O backend cria sua identidade e executa suas ações.
- A perda do Redis pode exigir nova autenticação e pode perder chat temporário, mas não modifica resultado, posição, avaliação concluída nem estado oficial no PostgreSQL. A recuperação da partida usa o DBML de estado ativo.
- O chat não deve ser copiado para `matches`, `match_players` ou qualquer tabela de histórico. Ao encerrar a partida, excluir o chat e as estruturas temporárias de presença e conexão.

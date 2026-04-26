# Comandos Cliente → Servidor

> Status: `validated` | Fonte: `PROTOCOL.md`

## Comandos principais

| Tipo | Descrição |
|------|-----------|
| `ping` | Keepalive |
| `join_match` | Entrar/criar sala (`roomId`, `pieceType`, `playerId`, etc.) |
| `submit_move` | Mover peça (`fromRow`, `fromCol`, `toRow`, `toCol`; `promotion` obrigatório ao promover peão) |
| `ignite_card` | Mover carta da mão para ignição (`handIndex`) |
| `submit_ignition_targets` | Confirmar alvos para carta que exige targets (`target_pieces`) |
| `draw_card` | Comprar carta do deck (2 mana; só no próprio turno fora de janelas) |
| `confirm_mulligan` | Confirmar mulligan (`handIndices` para devolver) |
| `set_reaction_mode` | Alterar toggle reactions (`mode`: `off`/`on`/`auto`) |
| `queue_reaction` | Enfileirar carta de reação (`handIndex`, opcionais `pieceRow`/`pieceCol`) |
| `resolve_reactions` | Confirmar pass ou disparar resolução da cadeia |
| `resolve_pending_effect` | Resolver efeito pendente de carta com alvo (`destRow`/`destCol` para Zip Line; `targetCardId` para Archmage Arsenal; payload vazio para confirmar lista vazia de busca no deck) |
| `leave_match` | Abandonar partida |
| `stay_in_room` | Permanecer na sala após fim de partida |
| `request_rematch` | Votar rematch |
| `client_fx_hold` | Pausar deadlines do servidor durante animação |
| `client_fx_release` | Retomar deadlines após animação |

## Detalhes importantes

### `join_match`
- Com conta autenticada: obrigatório ter pelo menos 1 deck salvo; usa o **deck de lobby** (`lobbyDeckId`).
- Sem JWT: baralho starter do servidor.
- Com persistência ativa: tenta **carregar** sala do Postgres antes de criar nova em memória.

### `confirm_mulligan`
- `handIndices`: lista 0-based das cartas a devolver; vazia = aceitar todas.
- Quando o **segundo** jogador confirma, inicia o primeiro turno (brancas).

### `submit_move`
- Promoção de peão exige `promotion`: `queen`, `rook`, `bishop` ou `knight`.
- O servidor rejeita promoção sem escolha explícita ou para peça inválida.

### `ignite_card` + `submit_ignition_targets`
- Cartas com `targets > 0`: primeiro `ignite_card` (mão → ignição); snapshot expõe `ignitionTargeting.awaitingTargetChoice`; então `submit_ignition_targets` para trabar os alvos; só depois o servidor abre `ignite_reaction`.

### `resolve_pending_effect`
Efeitos pós-ignição que exigem input ficam em `pendingEffects` no `state_snapshot` (só para o dono).

| Carta | Campo enviado no payload | Notas |
|-------|--------------------------|-------|
| **Zip Line** | `destRow` / `destCol` | Quadrado destino (índices lógicos) |
| **Archmage Arsenal** | `targetCardId` | ID da carta escolhida do deck |
| **Archmage Arsenal** (lista vazia) | *(payload vazio `{}`)* | Confirma que não há alvos legais; mana pago não é reembolsado |

O snapshot expõe `deckSearchChoices: [{ cardId }]` para o dono do efeito de busca; a lista pode ser vazia quando não há alvos legais no deck.

### `debug_match_fixture` (desenvolvimento)
- Só com `ADMIN_DEBUG_MATCH` ativo no servidor.
- `test_environment: true` obrigatório no payload.
- Requer ambos os jogadores conectados antes do primeiro turno.
- Desativa persistência de salas no Postgres.

## Links

- [[Transporte e Envelope]] — envelope JSON e campos do snapshot
- [[Janelas de Reação]] — quando `queue_reaction` e `resolve_reactions` são usados

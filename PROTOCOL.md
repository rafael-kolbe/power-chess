# Power Chess — WebSocket protocol (v2)

Contrato atual entre cliente e servidor. Mudanças de comportamento devem ser refletidas aqui e nos testes de integração.

## Índice

1. [Transporte e HTTP auxiliar](#transporte-e-http-auxiliar)  
2. [Envelope JSON](#envelope-json)  
3. [Servidor → cliente](#servidor--cliente)  
4. [Cliente → servidor](#cliente--servidor)  
5. [Janela de captura e cadeia Counter](#janela-de-captura-e-cadeia-counter)  
6. [Desconexão e timeout de turno](#desconexão-e-timeout-de-turno)  
7. [Reconciliação de fim de partida](#reconciliação-de-fim-de-partida)  
8. [Cobertura de testes](#cobertura-de-testes)  

---

## Transporte e HTTP auxiliar

| Item | Valor |
|------|--------|
| WebSocket | `ws://<host>:8080/ws` |
| Health | `GET http://<host>:8080/healthz` |
| Métricas | `GET http://<host>:8080/metrics` (JSON em memória) |
| Lista de salas (lobby) | `GET http://<host>:8080/api/rooms` |

**Resposta de `/api/rooms`:** `{ "rooms": [ { "roomId", "roomName", "roomPrivate", "connectedA", "connectedB", "gameStarted", "occupiedByColor?" } ] }` — apenas partidas com `matchEnded` falso. Com ocupação `1/2`, `occupiedByColor` pode ser `White` ou `Black`.

### Contas, decks (REST, JWT)

Com `DATABASE_URL` + `JWT_SECRET`, o cliente usa os mesmos endpoints de autenticação (`/api/auth/register`, `/api/auth/login`, `/api/auth/me`) e pode persistir **decks** (máx. 10 por conta, 20 cartas cada, limite de cópias do catálogo). O baralho usado na próxima partida é o **deck do lobby** (`lobbyDeckId` no utilizador), escolhido via UI ou `PUT /api/me/lobby-deck`.

| Método | Caminho | Descrição |
|--------|---------|-----------|
| `GET` | `/api/decks` | Lista `{ "decks": [...], "lobbyDeckId": number \| null }` |
| `POST` | `/api/decks` | Cria deck: `{ "name", "cardIds": string[], "playerSkillId", "sleeveColor" }` |
| `GET` | `/api/decks/{id}` | Detalhe de um deck do utilizador |
| `PUT` | `/api/decks/{id}` | Atualiza nome, cartas, skill, sleeve |
| `DELETE` | `/api/decks/{id}` | Remove; reatribui lobby se necessário |
| `POST` | `/api/decks/validate` | Valida `cardIds` sem gravar |
| `PUT` | `/api/me/lobby-deck` | `{ "deckId": number }` — falha se o utilizador estiver numa sala (partida ativa no servidor) |

Registo de conta cria automaticamente um deck **Default** (composição fixa no servidor) e define o lobby. Utilizadores antigos sem decks recebem backfill ao arranque do servidor.

---

## Envelope JSON

Todas as mensagens usam:

```json
{
  "id": "optional-correlation-id",
  "type": "message_type",
  "payload": {}
}
```

O campo `id` permite correlacionar `ack` / `error` com o pedido e suporta **idempotência** (`requestId` + tipo + jogador + sala).

---

## Servidor → cliente

### `hello`

Enviado ao abrir o WebSocket.

```json
{ "type": "hello" }
```

### `ack`

Confirma processamento do pedido.

```json
{
  "id": "req-123",
  "type": "ack",
  "payload": {
    "requestId": "req-123",
    "requestType": "submit_move",
    "status": "ok|queued|duplicate",
    "code": "",
    "message": ""
  }
}
```

Pedidos duplicados (mesmo `requestId` + tipo + jogador + sala) retornam `status: "duplicate"` sem reaplicar o efeito.

### `error`

```json
{
  "type": "error",
  "payload": {
    "code": "join_required",
    "message": "join_match is required before submit_move"
  }
}
```

**Códigos usados:** `bad_request`, `unknown_message_type`, `join_required`, `action_failed`, `invalid_payload`, `protocol_violation`.

### `state_snapshot`

Broadcast do estado da sala. Campos principais:

| Área | Conteúdo |
|------|-----------|
| Sala | `roomId`, `roomName`, `roomPrivate`, `roomPassword`, `connectedA/B`, `gameStarted` |
| Turno | `turnPlayer`, `turnSeconds`, `turnNumber`, `ignitionOn`, `ignitionCard`, `ignitionOwner`, `ignitionTurnsRemaining` |
| Perspectiva | `viewerPlayerId` — identifica o destinatário deste snapshot (determina visibilidade da mão) |
| Tabuleiro | `board` 8×8 (códigos `wK`, `bP`, `""` vazio), `enPassant`, `castlingRights` |
| Jogadores | `players[]`: `mana`, `maxMana`, `energizedMana`, `maxEnergized`, `handCount`, `cooldownCount`, `graveyardCount`, `strikes`, `deckCount`, `sleeveColor`, `hand` (privado — só no snapshot do próprio jogador), `banishedCards[]`, `graveyardPieces[]` (ordenado Q>R>B>N>P), `cooldownPreview[]` (até 4), `cooldownHiddenCount` |
| Efeitos | `pendingEffects`, `pendingCapture`, `reactionWindow` |
| Fim | `matchEnded`, `winner`, `endReason`, `rematchA/B`, `postMatchMsLeft` |

**Privacidade**: o servidor envia um snapshot por cliente via `BroadcastSnapshot()`; apenas o campo `hand` do próprio jogador é populado; oponentes recebem `hand: null`.

**Campos de zona por jogador:**

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `deckCount` | `int` | Quantidade de cartas restantes no deck |
| `sleeveColor` | `string` | Cor do sleeve: `blue`, `green`, `pink`, `red` |
| `hand` | `CardSnapshotEntry[]` | Mão do jogador (só no snapshot do dono) |
| `banishedCards` | `CardSnapshotEntry[]` | Cartas banidas, topo = mais recente |
| `graveyardPieces` | `string[]` | Peças capturadas pelo oponente (código `wQ`, `bP`, …) ordenadas por importância |
| `cooldownPreview` | `CooldownPreviewEntry[]` | Até 4 cartas com recarga mais próxima de terminar |
| `cooldownHiddenCount` | `int` | Quantidade de cartas na fila de recarga além das 4 exibidas |

**`CardSnapshotEntry`**: `{ cardId, manaCost, ignition, cooldown }`  
**`CooldownPreviewEntry`**: `{ cardId, manaCost, ignition, cooldown, turnsRemaining }`

**Exemplo ilustrativo** (estrutura; valores reais variam):

```json
{
  "type": "state_snapshot",
  "payload": {
    "roomId": "12",
    "roomName": "Let's Play!",
    "roomPrivate": false,
    "roomPassword": "",
    "connectedA": 1,
    "connectedB": 1,
    "gameStarted": true,
    "turnPlayer": "A",
    "turnSeconds": 30,
    "turnNumber": 3,
    "ignitionOn": true,
    "ignitionCard": "double-turn",
    "board": [
      ["bR","bN","bB","bQ","bK","bB","bN","bR"],
      ["bP","bP","bP","bP","bP","bP","bP","bP"],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["wP","wP","wP","wP","wP","wP","wP","wP"],
      ["wR","wN","wB","wQ","wK","wB","wN","wR"]
    ],
    "enPassant": { "valid": false },
    "castlingRights": {
      "whiteKingSide": true,
      "whiteQueenSide": true,
      "blackKingSide": true,
      "blackQueenSide": true
    },
    "players": [
      {
        "playerId": "A",
        "mana": 3,
        "maxMana": 10,
        "energizedMana": 2,
        "maxEnergized": 20,
        "handCount": 3,
        "cooldownCount": 0,
        "graveyardCount": 0,
        "strikes": 0
      },
      {
        "playerId": "B",
        "mana": 2,
        "maxMana": 10,
        "energizedMana": 0,
        "maxEnergized": 20,
        "handCount": 3,
        "cooldownCount": 0,
        "graveyardCount": 0,
        "strikes": 0
      }
    ],
    "pendingEffects": [{ "owner": "A", "cardId": "knight-touch" }],
    "pendingCapture": {
      "active": true,
      "fromRow": 6,
      "fromCol": 4,
      "toRow": 5,
      "toCol": 5,
      "actor": "A"
    },
    "reactionWindow": {
      "open": true,
      "trigger": "on-ignite",
      "actor": "A",
      "eligibleTypes": ["Retribution", "Power"],
      "stackSize": 1
    },
    "matchEnded": false,
    "winner": "",
    "endReason": "",
    "rematchA": false,
    "rematchB": false,
    "postMatchMsLeft": 30000
  }
}
```

**Notas:**

- `enPassant`: quando `valid` é true, o cliente pode usar para highlight; o servidor decide legalidade.  
- `castlingRights`: evita sugerir roque após revogação.  
- `turnSeconds`: duração autoritativa do timer de turno.  
- Após fim de partida: `rematchA` / `rematchB` e `postMatchMsLeft` para a janela pós-partida.

---

## Cliente → servidor

### `ping`

```json
{ "id": "req-1", "type": "ping", "payload": { "timestamp": 1710000000 } }
```

### `join_match`

```json
{
  "id": "req-2",
  "type": "join_match",
  "payload": {
    "roomId": "12",
    "roomName": "Let's Play!",
    "pieceType": "random",
    "playerId": "A",
    "isPrivate": false,
    "password": ""
  }
}
```

- Com persistência habilitada, o servidor tenta **carregar** a sala do Postgres antes de criar outra em memória.  
- `roomId`: string inteira positiva; vazio → nova sala com ID auto-incrementado.  
- `roomName`: nome exibido; vazio → padrão do servidor.  
- `pieceType`: `white` | `black` | `random`.  
- `isPrivate` + `password`: salas privadas na criação/entrada.
- Com conta autenticada e serviço de decks ativo: é obrigatório existir **pelo menos um deck** salvo; caso contrário o pedido falha com `action_failed` / mensagem `no_saved_deck`. O motor da partida usa o deck de lobby desse utilizador (e a skill guardada no deck). Convidados sem JWT continuam com o baralho starter do servidor.

### `submit_move`

```json
{
  "id": "req-3",
  "type": "submit_move",
  "payload": { "fromRow": 6, "fromCol": 4, "toRow": 4, "toCol": 4 }
}
```

### `activate_card`

```json
{ "id": "req-4", "type": "activate_card", "payload": { "handIndex": 0 } }
```

### `draw_card`

Compra uma carta do deck pagando 2 mana. Só permitido no próprio turno, fora de janelas de reação abertas, com pelo menos 1 slot vazio na mão.

```json
{ "id": "req-4b", "type": "draw_card", "payload": {} }
```

### `resolve_pending_effect`

```json
{
  "id": "req-5",
  "type": "resolve_pending_effect",
  "payload": { "pieceRow": 6, "pieceCol": 0 }
}
```

### `queue_reaction`

```json
{
  "id": "req-6",
  "type": "queue_reaction",
  "payload": { "handIndex": 1, "pieceRow": 4, "pieceCol": 4 }
}
```

### `resolve_reactions`

```json
{ "id": "req-7", "type": "resolve_reactions", "payload": {} }
```

### `leave_match`

```json
{ "id": "req-8", "type": "leave_match", "payload": {} }
```

### `stay_in_room`

```json
{ "id": "req-9", "type": "stay_in_room", "payload": {} }
```

Usado após fim de partida quando um jogador permanece só na sala para voltar ao estado de espera.

### `request_rematch`

```json
{ "id": "req-10", "type": "request_rematch", "payload": {} }
```

Com ambos conectados após o fim; quando ambos votam, a partida reinicia com **lados invertidos** (`A` ↔ `B`).

---

## Janela de captura e cadeia Counter

- Captura válida (inclui en passant) pode abrir `capture_attempt` com movimento **pendente**.  
- A primeira resposta na cadeia costuma ser **Counter** do oponente.  
- Reações resolvem em **LIFO**.  
- Sem reações enfileiradas, `resolve_reactions` aplica a captura pendente.  
- **Counterattack** e **Blockade**: validação e efeitos conforme regras do servidor (ver [Cards.md](Cards.md)).

---

## Desconexão e timeout de turno

| Situação | Efeito típico |
|----------|----------------|
| Ambos desconectam | Partida cancelada (`both_disconnected_cancelled` ou equivalente) |
| Um desconecta | Grace ~60 s; ao expirar, vitória do outro (`disconnect_timeout`) |
| `leave_match` com oponente na sala | Vitória do oponente (`left_room`) |
| Timeout de turno | +1 strike no jogador ativo; turno passa; 3 strikes → derrota (`strike_limit`) |

`turnSeconds` no snapshot define o limite por jogada.

---

## Reconciliação de fim de partida

O servidor reavalia xeque-mate/afogamento no tabuleiro ao montar snapshots. Se a sala estava marcada como encerrada por cancelamento de desconexão dupla mas a posição é terminal, `winner` / `endReason` podem ser atualizados para `checkmate` ou `stalemate`. O mesmo vale ao desconectar o último cliente, para não sobrescrever resultado decisivo.

---

## Cobertura de testes

Testes de integração WebSocket cobrem, entre outros:

- `join_match` multi-cliente e broadcast de `state_snapshot`  
- Contrato de `ack` e pedidos duplicados  
- Idempotência por `requestId`  
- Timeout de desconexão e cancelamento  
- Hooks de persistência (save/load)  
- Contadores em `/metrics`  

---

Documentação de produto e roadmap: **[PROJECT.md](PROJECT.md)**.  
Instruções de execução: **[README.md](README.md)**.

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

**Códigos usados:** `bad_request`, `unknown_message_type`, `join_required`, `action_failed`, `invalid_payload`, `protocol_violation`, `debug_disabled` (pedido `debug_match_fixture` quando `ADMIN_DEBUG_MATCH` não está ativo no servidor).

### `activate_card` (servidor → cliente — ativação do efeito)

Enviado **antes** do `state_snapshot` consolidado quando o servidor conclui o passo de **ativação do efeito** de uma carta (sucesso ou falha): ignição que chega a **0**, ou cada carta ao ser **desempilhada** da pilha de reações (resolução **LIFO** — uma carta por vez). Enquanto a pilha não esvazia, o estado de jogo permanece fechado para novas jogadas; cada `activate_card` corresponde a uma carta que acaba de sair da pilha (teste de efeito + animação no cliente), e só então a pilha avança. **Não** abre janela de reação do oponente; é distinto de **ignição** (mão → zona de ignição, C2S `ignite_card`). O cliente pode usar este evento para animação (brilho por tipo + voo para recarga). Vários `activate_card` podem aparecer em sequência no mesmo broadcast (por exemplo pilha de reações + ignição do ator); o cliente deve processá-los **em série** antes de depender do snapshot para posição das cartas.

```json
{
  "type": "activate_card",
  "payload": {
    "playerId": "A",
    "cardId": "energy-gain",
    "cardType": "power",
    "success": true,
    "retainIgnition": false
  }
}
```

- `cardType`: tipo catalogado em minúsculas (`power`, `continuous`, `retribution`, `counter`), quando conhecido.
- `success`: `false` quando o efeito foi **negado** (ex.: Extinguish) ou falhou de outra forma — o resolver **não** aplica o efeito da carta.
- `retainIgnition`: quando `true`, a carta **permanece** na zona de ignição após este passo (pulso **intermédio** de carta **Continuous**: inclui o **primeiro** pulso no **mesmo turno** em que a carta entrou, **após** fechar `ignite_reaction`, e cada início de turno seguinte **desse** jogador até ao pulso **final**). O cliente não deve mover a carta para recarga nem limpar a ignição até um `activate_card` sem `retainIgnition` (resolução final).
- `negatesActivationOf`: ID do jogador cuja **activação** da carta em ignição foi negada por este evento (transição `EffectNegated false → true`). Não-vazio **apenas** quando este evento causou essa transição. O cliente deve mostrar o overlay `negate.png` na carta desse jogador **imediatamente após** o glow desta activação, **antes** de processar o próximo evento `activate_card`. Nota: a **ignição** (entrada no slot + janela de reação) é distinta da **activação** (teste do efeito); o que fica negado é a activação.

### `state_snapshot`

Broadcast do estado da sala. Campos principais:

| Área | Conteúdo |
|------|-----------|
| Sala | `roomId`, `roomName`, `roomPrivate`, `roomPassword`, `connectedA/B`, `gameStarted` |
| Turno | `turnPlayer`, `turnNumber` |
| Abertura | `mulliganPhaseActive` — `true` enquanto os dois jogadores podem confirmar mulligan; `mulliganReturned` — mapa `{"A": n, "B": n}` com quantas cartas cada um já devolveu (`-1` até confirmar); `mulliganDeadlineUnixMs` — instante (epoch ms) em que o servidor confirma automaticamente quem ainda não confirmou (devolução vazia, “keep”). Janela de 15 s a partir do início da fase de mulligan |
| Perspectiva | `viewerPlayerId` — identifica o destinatário deste snapshot (determina visibilidade da mão) |
| Tabuleiro | `board` 8×8 (códigos `wK`, `bP`, `""` vazio), `enPassant`, `castlingRights` |
| Jogadores | `players[]`: `mana`, `maxMana`, `energizedMana`, `maxEnergized`, `handCount`, `cooldownCount`, `graveyardCount`, `deckCount`, `sleeveColor`, `reactionMode` (`off` / `on` / `auto`), `ignitionOn`, `ignitionCard`, `ignitionTurnsRemaining`, `ignitionEffectNegated` (efeito negado enquanto na ignição; visível a ambos), `hand` (privado — só no snapshot do próprio jogador), `banishedCards[]`, `graveyardPieces[]` (ordenado Q>R>B>N>P; na UI: zona **Captura**), `cooldownPreview[]` (fila completa na recarga), `cooldownHiddenCount` (sempre `0`; campo reservado) |
| Efeitos | `pendingEffects`, `activationQueueSize`, `pendingCapture`, `reactionWindow` |
| Debug (admin) | `adminDebugMatch` (capability) |
| Fim | `matchEnded`, `winner`, `endReason`, `rematchA/B`, `postMatchMsLeft` |

**Privacidade**: o servidor envia um snapshot por cliente via `BroadcastSnapshot()`; apenas o campo `hand` do próprio jogador é populado; oponentes recebem `hand: null`.

**Campos de zona por jogador:**

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `deckCount` | `int` | Quantidade de cartas restantes no deck |
| `sleeveColor` | `string` | Cor do sleeve: `blue`, `green`, `pink`, `red` |
| `hand` | `CardSnapshotEntry[]` | Mão do jogador (só no snapshot do dono) |
| `banishedCards` | `CardSnapshotEntry[]` | Cartas banidas, topo = mais recente |
| `graveyardPieces` | `string[]` | Peças capturadas pelo oponente (código `wQ`, `bP`, …) ordenadas por importância; na UI a zona chama-se **Captura** |
| `cooldownPreview` | `CooldownPreviewEntry[]` | Todas as cartas na fila de recarga (ordenadas para HUD; o cliente sobrepõe como na mão) |
| `cooldownHiddenCount` | `int` | Sempre `0` (compatibilidade; não há cartas omitidas do preview) |
| `reactionMode` | `string` | Preferência do assento: `off`, `on`, `auto` — o servidor usa para decidir se abre janela de reação em captura (ver secção abaixo) |
| `ignitionOn` | `bool` | `true` se esse jogador tem carta na sua zona de ignição |
| `ignitionCard` | `string` | ID da carta na ignição (omitido se vazio) |
| `ignitionTurnsRemaining` | `int` | Turnos restantes no contador de ignição (omitido se 0) |
| `ignitionEffectNegated` | `bool` | `true` se o efeito da carta na ignição foi negado (ex.: Extinguish); permanece até a carta sair da ignição |

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
    "turnNumber": 3,
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
        "ignitionOn": true,
        "ignitionCard": "double-turn",
        "ignitionTurnsRemaining": 1
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
      }
    ],
    "pendingEffects": [{ "owner": "A", "cardId": "zip-line", "sourceRow": 6, "sourceCol": 1 }],
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
      "stackSize": 1,
      "stagedCardId": "retribution",
      "stagedOwner": "B"
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
- `reactionWindow.stagedCardId` / `stagedOwner`: carta no topo da pilha de reações e o assento que a jogou (para HUD enquanto a janela está aberta).
- `reactionWindow.stackCards`: lista opcional `{ cardId, owner }` na ordem **fundo → topo** da pilha (a primeira resposta enfileirada vem primeiro). A resolução no servidor é **LIFO** (última entrada resolve primeiro); o cliente pode usar essa lista para animar efeitos em sequência.  
- Após fim de partida: `rematchA` / `rematchB` e `postMatchMsLeft` para a janela pós-partida.

---

## Cliente → servidor

### `ping`

```json
{ "id": "req-1", "type": "ping", "payload": { "timestamp": 1710000000 } }
```

### `client_fx_hold` / `client_fx_release`

Mensagens em par para **pausar no servidor** a avaliação dos deadlines de reação, mulligan e desconexão enquanto o cliente corre animações (ignição, recarga, resolução de pilha, etc.). Cada `client_fx_hold` aumenta uma profundidade na sala; cada `client_fx_release` diminui. Com profundidade zero, o servidor **adianta** os deadlines absolutos pelo tempo de parede que passou desde o hold mais externo. Profundidade máxima 64. Esperado: **cada** cliente ligado envia um hold/release por “onda” de FX que executa localmente (dois jogadores ⇒ dois holds antes dos releases, para os prazos só retomarem quando ambos terminam). Requer `join_match` com assento atribuído. `client_fx_hold` responde com `ack`; após `client_fx_release` que esvazia o hold externo, o servidor emite `state_snapshot` com deadlines atualizados.

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

Para promoção de peão, o cliente deve enviar também `promotion` com um dos valores `queen`, `rook`, `bishop` ou `knight`:

```json
{
  "id": "req-3b",
  "type": "submit_move",
  "payload": { "fromRow": 1, "fromCol": 0, "toRow": 0, "toCol": 0, "promotion": "knight" }
}
```

O servidor rejeita promoções sem escolha explícita ou para peças inválidas.

### `ignite_card`

Move uma carta da **mão** para a **zona de ignição** do jogador (paga mana, abre janelas de reação do oponente quando as regras o exigem). Na terminologia do jogo isto é **ignição**, não ativação do efeito.

```json
{ "id": "req-4", "type": "ignite_card", "payload": { "handIndex": 0 } }
```

- Cartas com `targets` > 0 no catálogo podem usar **só** `handIndex` no primeiro passo (mão → ignição); o snapshot expõe `ignitionTargeting.awaitingTargetChoice` até o jogador enviar os alvos.

### `submit_ignition_targets`

Trava as casas-alvo no tabuleiro para a carta que **já está** na zona de ignição e que exige alvos (`targets` > 0). Só depois disso o servidor pode abrir `ignite_reaction` para o oponente.

```json
{
  "id": "req-4a",
  "type": "submit_ignition_targets",
  "payload": { "target_pieces": [{ "row": 6, "col": 4 }] }
}
```

### `draw_card`

Compra uma carta do deck pagando 2 mana. Só permitido no próprio turno, fora de janelas de reação abertas, com pelo menos 1 slot vazio na mão.

```json
{ "id": "req-4b", "type": "draw_card", "payload": {} }
```

### `confirm_mulligan`

Confirma o mulligan de abertura: as cartas nos índices indicados da mão voltam ao deck, o deck é embaralhado e o jogador compra a mesma quantidade. Só quando `mulliganPhaseActive` está ativo; cada jogador confirma uma vez. Quando o segundo jogador confirma, o servidor inicia o primeiro turno de xadrez (white).

```json
{ "id": "req-4c", "type": "confirm_mulligan", "payload": { "handIndices": [0, 2] } }
```

- `handIndices`: índices 0-based na mão atual; podem ser repetidos na lista (deduplicados no servidor); lista vazia = aceitar as 3 cartas sem devolver nenhuma.

### `set_reaction_mode`

Atualiza a preferência do jogador para **reações em captura** (e futuras janelas alinhadas a este toggle). Pode ser enviado **a qualquer momento** na partida; o servidor aplica já no próximo evento elegível.

```json
{ "id": "req-4d", "type": "set_reaction_mode", "payload": { "mode": "off" } }
```

- `mode`: `off`, `on` ou `auto` (case-insensitive; valores desconhecidos tratados como `on`).
- **`off`**: o servidor **não mantém** janela `capture_attempt` aberta só para pass — aplica a captura de seguida.
- **`on`**: mantém a janela como hoje (oponente pode reagir mesmo sem carta jogável).
- **`auto`**: o servidor só mantém a janela se o oponente tiver resposta identificável conforme o gatilho: em **`capture_attempt`**, pelo menos uma **Counter** na mão (regras económicas); em **`ignite_reaction`**, Retribution e/ou Disruption conforme `eligibleTypes`, além de Counter quando aplicável (condições textuais das Counter em AUTO podem ser fase posterior).

O estado atual vem em cada entrada de `players[].reactionMode` no `state_snapshot`.

### `client_trace` (apenas com `ADMIN_DEBUG_MATCH` ativo)

Envia texto JSON (por exemplo um lote de eventos do navegador) para o **log do processo** no servidor (`log.Printf` com prefixo `client_trace`), útil com sessão Docker em `tee`. Exige `join_match` prévio. Payload: `{ "text": "..." }` (o servidor trunca entradas muito longas).

### `resolve_pending_effect`

Efeitos que exigem alvo **após** a resolução da ignição (ex.: **Zip Line**) ficam em `pendingEffects` no `state_snapshot` **só para o dono** do efeito; para Zip Line inclui `sourceRow` / `sourceCol` (índices lógicos do tabuleiro, mesmo sistema que `submit_move`).

```json
{
  "id": "req-5",
  "type": "resolve_pending_effect",
  "payload": { "destRow": 6, "destCol": 6 }
}
```

Enquanto o jogador tiver um efeito pendente próprio, o servidor rejeita outras ações de jogo desse jogador (ex.: `submit_move`, `draw_card`, nova ignição) até enviar um `resolve_pending_effect` válido ou um destino ilegal (mensagem de erro; o efeito continua pendente).

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

Confirma a resolução da cadeia atual. Só o **responder atual** da janela pode enviar com sucesso.

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

### `debug_match_fixture` (apenas desenvolvimento / staging)

Disponível **somente** se o processo do servidor tiver a variável de ambiente `ADMIN_DEBUG_MATCH` ativa (`1`, `true`, `yes` ou `on`). Caso contrário, qualquer mensagem deste tipo é recusada com `error` código `debug_disabled` — mesmo que o cliente envie `test_environment: true`.

Handshake obrigatório: `test_environment` deve ser `true`. Se for `false` ou omitido com valor falso, o servidor responde com `invalid_payload` (`test_environment must be true`).

Requer `join_match` com ambos os jogadores conectados na mesma sala, e só é aplicável **antes** da partida ter iniciado o primeiro turno (`match already started` caso contrário).

Com `ADMIN_DEBUG_MATCH` ativo, o servidor **não persiste** salas no armazenamento configurado (`SaveRoom` é ignorado): o estado da partida existe só em memória.

Payload: `white` e `black` são obrigatórios. Cada lado tem:

- `deck`: exatamente **20** IDs de cartas num baralho legal de construído (mesmas regras que `POST /api/decks`).
- `hand`: lista de IDs retirados **desse** baralho (cópias suficientes no `deck`); tamanho máximo 5.
- Opcional: `mana`, `maxMana`, `energizedMana`, `maxEnergized` — números inteiros; omitidos mantêm os valores padrão do motor após o preset de mãos.

`white` ↔ jogador `A` (brancas); `black` ↔ jogador `B` (pretas). O estado do tabuleiro é reposto para a posição inicial de xadrez; a fase de mulligan fica ativa com as mãos indicadas (sem novo shuffle). Em seguida cada cliente envia `confirm_mulligan` para fechar a abertura e iniciar o primeiro turno.

```json
{
  "id": "dbg-1",
  "type": "debug_match_fixture",
  "payload": {
    "test_environment": true,
    "white": {
      "deck": ["energy-gain", "knight-touch", "..."],
      "hand": ["knight-touch", "energy-gain", "bishop-touch"],
      "mana": 5,
      "maxMana": 10,
      "energizedMana": 0,
      "maxEnergized": 20
    },
    "black": {
      "deck": ["energy-gain", "knight-touch", "..."],
      "hand": ["retaliate", "backstab", "clairvoyance"],
      "mana": 4,
      "maxMana": 10
    }
  }
}
```

---

## Janelas de resposta: captura (`capture_attempt`) e ignição (`ignite_reaction`)

### Quem abre e quem responde (regra de produto)

- **Abrem janela:** ignição de cartas **Power**, **Continuous** e **Disruption** (o servidor abre `ignite_reaction` para o oponente quando aplicável; para Disruption no turno próprio, requer alvo válido na ignição do oponente); **Retribution** não inicia jogada e só entra como resposta dentro de `ignite_reaction`; tentativa de **captura** no xadrez (inclui en passant) abre `capture_attempt` com movimento **pendente**.
- **Podem responder:** em **`capture_attempt`**, só **Counter** (consta em `eligibleTypes`). Em **`ignite_reaction`**, **Retribution**, **Disruption** e/ou **Counter** conforme `eligibleTypes`; **Counter** na primeira resposta só quando o catálogo define `MaybeCaptureAttemptOnIgnition` na carta em ignição (hoje **false** para todas até efeitos de captura por ignição existirem).

### `capture_attempt`

- Salvo `reactionMode` do oponente (`off`, ou `auto` sem Retribution/Counter elegível pelas regras económicas) fazer o servidor resolver de imediato com pilha vazia.  
- A **primeira** resposta é do **oponente** ao atacante: **Counter** (somente).
- Na cadeia: só **Counter** após **Counter**. Reações resolvem em **LIFO**.
- Sem reações enfileiradas, `resolve_reactions` aplica a captura pendente.  
- Com reações enfileiradas, a resolução só começa após `resolve_reactions` do responder atual (ordem explícita da cadeia).  
- Sem timeout automático de janela de reação no servidor; a resolução depende de ação explícita (reagir/confirmar).  
- **Counterattack** e **Blockade**: validação e efeitos conforme regras do servidor (ver [Cards.md](Cards.md)).

### `ignite_reaction`

- Ignição de **Power**, **Continuous** ou **Disruption** quando o motor abre a janela: `reactionWindow.actor` = quem ignitou. A **primeira** resposta é do **oponente**: **Retribution** e/ou **Disruption** sempre que constarem em `eligibleTypes`; **Counter** só quando `MaybeCaptureAttemptOnIgnition` for **true** no catálogo para a carta em ignição (efeitos ainda não aplicam capturas por carta; o campo existe para o futuro, ex. *Backstab*).
- O modo `reactionMode` do oponente (`off`, ou `auto` sem carta elegível na mão) pode fazer o servidor resolver de imediato com pilha vazia.
- Enquanto `ignite_reaction` estiver aberta, `submit_move` é rejeitado.
- Cadeia: após **Retribution/Disruption**, só **Retribution** ou **Disruption**; após **Counter**, só **Counter** (quando Counter for permitido na janela).
- Com pilha não vazia, a resolução só começa após `resolve_reactions` do responder atual.

---

## Desconexão

| Situação | Efeito típico |
|----------|----------------|
| Ambos desconectam | Partida cancelada (`both_disconnected_cancelled` ou equivalente) |
| Um desconecta | Orçamento **60 s** de tempo offline **cumulativo** por jogador na partida; vitória do outro (`disconnect_timeout`) no instante `max(detectou + 5 s, detectou + orçamento restante)`; `reconnectDeadlineUnixMs` no snapshot aponta esse instante enquanto o timer está ativo |
| `leave_match` com oponente na sala | Vitória do oponente (`left_room`) |

O turno de xadrez é sem limite de tempo. O único cronômetro exposto no cliente é o da fase de mulligan (`mulliganDeadlineUnixMs`).

---

## Reconciliação de fim de partida

O servidor reavalia xeque-mate/afogamento no tabuleiro ao montar snapshots. Se a sala estava marcada como encerrada por cancelamento de desconexão dupla mas a posição é terminal, `winner` / `endReason` podem ser atualizados para `checkmate` ou `stalemate`. O mesmo vale ao desconectar o último cliente, para não sobrescrever resultado decisivo.

---

## Cobertura de testes

Testes de integração WebSocket cobrem, entre outros:

- `join_match` multi-cliente e broadcast de `state_snapshot`  
- Contrato de `ack` e pedidos duplicados  
- Idempotência por `requestId`  
- `debug_match_fixture` com `ADMIN_DEBUG_MATCH` ligado/desligado  
- Timeout de desconexão e cancelamento  
- `set_reaction_mode` e `players[].reactionMode`  
- Hooks de persistência (save/load)  
- Contadores em `/metrics`  

---

Documentação de produto e roadmap: **[PROJECT.md](PROJECT.md)**.  
Instruções de execução: **[README.md](README.md)**.

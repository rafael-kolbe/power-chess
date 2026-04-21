# Transporte e Envelope

> Status: `validated` | Fonte: `PROTOCOL.md`

## Endpoints

| Item | Valor |
|------|-------|
| WebSocket | `ws://<host>:8080/ws` |
| Health | `GET http://<host>:8080/healthz` |
| Métricas | `GET http://<host>:8080/metrics` |
| Lobby | `GET http://<host>:8080/api/rooms` |

## Envelope JSON (todas as mensagens)

```json
{
  "id": "optional-correlation-id",
  "type": "message_type",
  "payload": {}
}
```

- `id`: correlaciona `ack`/`error` com o pedido; suporta idempotência.
- Pedidos duplicados (mesmo `requestId` + tipo + jogador + sala) → `status: "duplicate"` sem reaplicar efeito.

## Campos-chave do `state_snapshot`

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `turnPlayer` | string | `"A"` ou `"B"` |
| `turnNumber` | int | Número do turno atual |
| `board` | 8x8 array | Códigos `wK`, `bP`, `""` vazio |
| `reactionWindow` | object | Janela aberta, tipo, eligibleTypes, pilha |
| `matchEnded` | bool | Se a partida terminou |
| `winner` | string | `"A"`, `"B"` ou `""` |
| `endReason` | string | `checkmate`, `stalemate`, `disconnect_timeout`, etc. |
| `mulliganPhaseActive` | bool | Fase de mulligan ativa |
| `mulliganDeadlineUnixMs` | int | Deadline da fase de mulligan (epoch ms) |

## Privacidade do snapshot

- Servidor envia **um snapshot por cliente**: `hand` do próprio jogador populado; oponente recebe `hand: null`.

## Campos de zona por jogador

| Campo | Descrição |
|-------|-----------|
| `mana` / `maxMana` | Pool atual e máximo |
| `energizedMana` / `maxEnergized` | Pool energizada |
| `handCount` | Quantidade de cartas na mão |
| `hand` | Array de cartas (só no snapshot do dono) |
| `ignitionOn` / `ignitionCard` | Slot de ignição |
| `ignitionTurnsRemaining` | Contador da carta na ignição |
| `ignitionEffectNegated` | Efeito negado (visível a ambos) |
| `cooldownPreview` | Fila completa de recarga |
| `graveyardPieces` | Peças capturadas (zona "Captura" na UI) |
| `banishedCards` | Cartas banidas |
| `reactionMode` | `off`, `on` ou `auto` |

## Links

- [[Eventos Servidor → Cliente]] — tipos de mensagem S→C
- [[Comandos Cliente → Servidor]] — tipos de mensagem C→S
- [[Janelas de Reação]] — campo `reactionWindow` em detalhe

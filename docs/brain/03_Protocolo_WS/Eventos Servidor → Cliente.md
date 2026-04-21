# Eventos Servidor → Cliente

> Status: `validated` | Fonte: `PROTOCOL.md`

## `hello`
Enviado ao abrir o WebSocket. Sem payload relevante.

## `ack`
Confirma processamento do pedido.
- `status`: `ok` | `queued` | `duplicate`
- `code` e `message` em caso de erro.

## `error`
Códigos usados: `bad_request`, `unknown_message_type`, `join_required`, `action_failed`, `invalid_payload`, `protocol_violation`, `debug_disabled`.

## `state_snapshot`
Broadcast completo do estado da sala. Ver [[Transporte e Envelope]] para campos detalhados. Enviado após toda ação que muda estado.

## `activate_card`
Enviado **antes** do `state_snapshot` quando o servidor conclui a ativação de uma carta:

```json
{
  "type": "activate_card",
  "payload": {
    "playerId": "A",
    "cardId": "energy-gain",
    "cardType": "power",
    "success": true,
    "retainIgnition": false,
    "negatesActivationOf": ""
  }
}
```

| Campo | Descrição |
|-------|-----------|
| `success` | `false` = efeito negado ou falhou |
| `retainIgnition` | `true` = carta Continuous ainda no slot (pulso intermediário) |
| `negatesActivationOf` | ID do jogador cuja ativação foi negada por este evento |

- Vários `activate_card` podem aparecer em sequência (pilha de reações); o cliente processa **em série** antes de depender do snapshot.
- O cliente não deve mover carta para recarga até `activate_card` sem `retainIgnition`.

## Links

- [[Transporte e Envelope]] — envelope e campos do snapshot
- [[Comandos Cliente → Servidor]] — comandos C→S

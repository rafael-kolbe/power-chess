# Desconexão e Fim de Partida

> Status: `validated` | Fonte: `PROJECT.md` (seção "Desconexão e fim de partida"), `PROTOCOL.md`

## Orçamento de desconexão

| Regra | Valor |
|-------|-------|
| Orçamento por jogador por partida | **60 s total** (cumulativo; não reinicia) |
| Grace mínimo por evento | **5 s** antes de declarar vitória ao outro |
| Ambos offline | Partida cancelada sem vencedor |

## Fluxo ao desconectar

1. Servidor detecta desconexão (WebSocket fechado).
2. **Pausa** qualquer fluxo dependente do jogador (jogadas, timers de reação, chains).
3. Jogador conectado **não pode avançar** a partida enquanto pausada.
4. Após `max(detectado + 5s, detectado + orçamento restante)`: vitória do conectado por `disconnect_timeout`.

## Banner de UI

- Área do oponente mostra banner verde com contagem do tempo restante.
- Textos canônicos:
  - **pt-BR**: "Jogador desconectado ({s}s)"
  - **EN**: "Opponent disconnected ({s}s)"
- `{s}` = valor dinâmico em segundos vindo do cliente com base no protocolo.

## Outros fins de partida

| Situação | `endReason` |
|----------|-------------|
| Xeque-mate | `checkmate` |
| Afogamento | `stalemate` |
| Abandonou (`leave_match`) | `left_room` (vitória do oponente) |
| Desconexão (orçamento esgotado) | `disconnect_timeout` |
| Ambos desconectam | `both_disconnected_cancelled` |

## Rematch

- Após fim: ambos têm `postMatchMsLeft` (30 s padrão) para votar rematch.
- Com os dois votos: partida reinicia com **lados invertidos** (A ↔ B).

## Links

- [[Transporte e Envelope]] — campos `matchEnded`, `winner`, `endReason`, `reconnectDeadlineUnixMs`

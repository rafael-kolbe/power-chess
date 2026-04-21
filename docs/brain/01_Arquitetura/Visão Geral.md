# Visão Geral — Power Chess

> Status: `validated` | Fonte: `PROJECT.md`

## Produto

- Multiplayer **1v1**, xadrez clássico + **cartas de poder** + **habilidades de jogador**.
- Backend em Go com WebSockets; estado da partida **autoritativo no servidor**.
- Frontend atual: HTML, CSS e JavaScript (HUD de desenvolvimento).
- Planejado: filas **casual** e **ranqueada** (ELO).

## Pilares técnicos

| Pilar | Decisão |
|-------|---------|
| Autoridade | Servidor valida tudo; cliente não duplica regras críticas |
| Extensibilidade | Efeitos via resolver dedicado + estado genérico serializável |
| Persistência | PostgreSQL + GORM + golang-migrate |
| Transporte | WebSocket JSON com envelope `{ id, type, payload }` |
| Infra dev | Docker Compose (servidor + Postgres) |

## Estrutura de pacotes

```
internal/
├── chess/        motor de xadrez puro (movimentos, xeque, xeque-mate)
├── gameplay/     estado de partida (deck, mana, ignição, recarga, cartas)
├── match/        efeitos de carta, reações, cadeias Counter, resolvers
├── server/       protocolo WS, handlers HTTP, salas, auth, decks
└── ranking/      ELO
```

## Fluxo geral de uma partida

1. Jogadores entram na sala via `join_match`.
2. Servidor embaralha decks; fase de **mulligan** (15 s).
3. Loop de turnos: início de turno → ticks → ações → movimentos → captura → fim de turno.
4. Vitória por xeque-mate, desconexão ou abandono.

## Relacionamentos entre domínios

- [[Turno e Ordem]] depende de [[Mana e Energizada]] e [[Ignição, Ativação e Negação]].
- [[Janelas de Reação]] são abertas durante o turno e afetam [[Sistema de Cartas]].
- [[Transporte e Envelope]] serializa todo o estado para o cliente via `state_snapshot`.
- [[Padrão de Resolvers]] é o contrato de implementação de cada carta.

# Estrutura de Pacotes Go

> Status: `validated` | Fonte: `README.md`, `internal/`

## Mapa de pacotes

| Pacote | Arquivos-chave | Responsabilidade |
|--------|---------------|-----------------|
| `cmd/server` | `main.go` | Entrada: inicia HTTP + WebSocket |
| `internal/server` | `auth.go`, `auth_http.go`, `decks_http.go` | Protocolo WS, handlers HTTP, salas, auth JWT, decks |
| `internal/chess` | `engine.go` | Motor de xadrez puro (movimentos legais, xeque, xeque-mate, en passant, roque) |
| `internal/gameplay` | `state.go`, `cards.go`, `deck.go`, `zones.go`, `opening.go`, `player_skills.go`, `reaction_chain.go`, `reaction_eligibility.go`, `ignite_reaction_eligibility.go` | Estado de partida (deck, mana, ignição, recarga, cartas, abertura/mulligan) |
| `internal/match` | `engine.go`, `resolvers.go`, `reactions.go`, `reaction_runtime.go`, `movement_grants.go`, `persistence.go` | Efeitos de carta, reações, cadeias Counter, resolvers, persistência de estado |
| `internal/match/resolvers/` | `interface.go`, `power/`, `retribution/`, `counter/`, `continuous/`, `disruption/` | Resolvers por tipo de carta |
| `internal/ranking` | `elo.go` | Cálculo de ELO |

## Resolvers implementados

| Carta | Arquivo |
|-------|---------|
| Knight Touch | `resolvers/power/knight_touch.go` |
| Rook Touch | `resolvers/power/rook_touch.go` |
| Bishop Touch | `resolvers/power/bishop_touch.go` |
| Double Turn | `resolvers/power/double_turn.go` |
| Energy Gain | `resolvers/power/energy_gain.go` |
| Piece Swap | `resolvers/power/piece_swap.go` |
| Mana Burn | `resolvers/retribution/mana_burn.go` |
| Extinguish | `resolvers/disruption/extinguish.go` |

## Links

- [[Padrão de Resolvers]] — como adicionar novos resolvers
- [[TDD e Testes]] — organização dos testes

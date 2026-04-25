# TDD e Testes

> Status: `validated` | Fonte: `PROJECT.md`, `README.md`

## Obrigatoriedade

**TDD é obrigatório no backend.** Toda mudança de comportamento começa por um teste.

## Ciclo padrão

1. **Red** — escrever teste que falha
2. **Green** — implementar o mínimo para passar
3. **Refactor** — melhorar mantendo testes verdes

## Comando

```bash
go test ./...
```

Deve estar 100% verde antes de qualquer commit.

## Organização dos testes

| Arquivo | O que testa |
|---------|------------|
| `internal/chess/engine_test.go` | Movimentos legais, xeque, xeque-mate |
| `internal/gameplay/state_test.go` | Estado de partida geral |
| `internal/gameplay/zones_test.go` | Transições de zona (mão, ignição, recarga) |
| `internal/gameplay/opening_test.go` | Mulligan, compra inicial |
| `internal/gameplay/reaction_eligibility_test.go` | Elegibilidade de reação |
| `internal/match/engine_test.go` | Motor de efeitos e cadeias |
| `internal/match/reaction_stack_test.go` | Pilha de reações LIFO |
| `internal/match/resolvers/<tipo>/*_test.go` | Testes por resolver/carta, junto do pacote do resolver |
| `internal/server/auth_*_test.go` | Autenticação e validação |
| `internal/server/debug_match_fixture_test.go` | Fixture de debug (WS) |

## Cobertura de integração WS

- `join_match` multi-cliente e broadcast de `state_snapshot`
- Contrato de `ack` e pedidos duplicados
- Idempotência por `requestId`
- `debug_match_fixture` com `ADMIN_DEBUG_MATCH` ligado/desligado
- Timeout de desconexão e cancelamento
- `set_reaction_mode` e `players[].reactionMode`
- Hooks de persistência (save/load)
- Contadores em `/metrics`

## Regra de branch por carta

- Branch: `feature/<card-id>` (ex.: `feature/backstab`)
- Uma carta por branch/entrega
- Commit só com testes passando

## Links

- [[Padrão de Resolvers]] — onde os testes de resolver se encaixam
- [[Estrutura de Pacotes]] — onde os arquivos de teste vivem

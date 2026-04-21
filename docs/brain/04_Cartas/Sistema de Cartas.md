# Sistema de Cartas

> Status: `validated` | Fonte: `PROJECT.md` (seção "Sistema de cartas")

## Anatomia de uma carta

| Campo | Descrição |
|-------|-----------|
| **ID** | Slug único (ex.: `knight-touch`) |
| **Tipo** | Power, Retribution, Counter, Continuous, Disruption |
| **Custo** | Mana para ignitar |
| **Ignição** | Turnos até resolver o efeito (0 = mesmo turno) |
| **Recarga** | Turnos de cooldown antes de voltar ao deck |
| **Alvos** | Quantas peças precisam ser selecionadas |
| **Duração do efeito** | Turnos que o efeito persiste (0 = instantâneo) |

## Fluxo de vida da carta

```
Deck → Mão → [ignição] → Slot de ignição → [resolução] → Recarga → Deck
                                                ↓ se banida
                                           Banidas (não volta)
```

## Deck e mão

- Compra inicial: **3 cartas** (após os dois jogadores estarem na sala; embaralhamento criptográfico).
- **Mulligan** (estilo Shadowverse): devolver qualquer subconjunto → embaralha → compra mesma quantidade.
  - Janela: **15 s**; expirado → auto-confirm (nenhuma carta devolvida).
  - Oponente vê **quantas** cartas foram devolvidas; não vê quais.
- Compra durante partida: paga mana.
- Cartas **banidas** não voltam ao deck (salvo efeito específico).
- **Não pode ignitar** carta se já existir cópia na recarga.

## Limite de cópias por deck

- Conforme catálogo e regras no servidor (`POST /api/decks` valida).
- Deck de lobby: máx. **20 cartas**, máx. **10 decks por conta**.

## Links

- [[Tipos de Cartas]] — comportamento detalhado por tipo
- [[Catálogo de Cartas]] — texto canônico de todas as cartas
- [[Ignição, Ativação e Negação]] — fluxo de uso de carta

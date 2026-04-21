# Glossário — Terminologia Canônica

> Status: `validated` | Use estes termos consistentemente em código, docs e UI.

## Termos de gameplay

| Termo | Definição |
|-------|-----------|
| **Ignição** | Ação de mover uma carta da mão para a zona de ignição (paga mana, abre janela de reação) |
| **Ativação** | Resolver o efeito da carta quando o contador de ignição chega a 0 |
| **Negação** | Estado de uma carta cuja ativação foi impedida; efeito conclui em falha, carta vai à recarga |
| **Recarga** | Cooldown — zona onde a carta aguarda antes de voltar ao deck |
| **Captura** | Zona de peças capturadas pelo oponente (no código: `graveyard`) |
| **Banida** | Carta removida do jogo permanentemente (salvo efeito específico) |
| **Cadeia** | Pilha de reações que resolve em ordem LIFO |
| **Janela** | Período em que o oponente pode reagir (`ignite_reaction` ou `capture_attempt`) |
| **Mana energizada** | Pool gerada ao gastar mana em poderes; usada para habilidade de jogador |
| **Toggle Reactions** | Preferência do jogador: OFF / ON / AUTO — controla se janelas de reação são abertas |
| **Mulligan** | Fase de abertura onde cada jogador pode trocar cartas da mão inicial |

## Termos técnicos

| Termo | Definição |
|-------|-----------|
| **Resolver** | Módulo Go responsável por aplicar o efeito de uma carta específica |
| **Snapshot** | Estado serializado da partida enviado pelo servidor via `state_snapshot` |
| **Envelope** | Estrutura `{ id, type, payload }` de todas as mensagens WebSocket |
| **Budget de response** | Tempo acumulado por assento para reagir; não reinicia entre janelas do mesmo turno |
| **Fixture** | Estado pré-configurado para testes via `debug_match_fixture` |
| `MaybeCaptureAttemptOnIgnition` | Flag de catálogo que define se Counter pode responder em `ignite_reaction` |

## Zonas de uma carta

```
Deck → Mão → Slot de ignição → Recarga (cooldown) → Deck
                              ↘ Banidas
```

## Zonas de peças

```
Tabuleiro → Captura (graveyardPieces) 
```

## Tipos de carta (IDs internos)

| Tipo | Cor | ID interno |
|------|-----|-----------|
| Power | Azul | `power` |
| Retribution | Vermelho | `retribution` |
| Counter | Verde | `counter` |
| Continuous | Roxo | `continuous` |
| Disruption | Laranja | `disruption` |

# Turno e Ordem

> Status: `validated` | Fonte: `PROJECT.md` (seção "Ordem de turno")

## Passos do turno (ordem obrigatória no servidor)

1. **Início do turno** — +1 mana ao pool do jogador ativo (respeitando máximo).
2. **Tick de ignição** — contador da carta no slot de ignição −1; se chegar a 0, ativação do efeito.
3. **Tick de recarga** — contador de cada carta na pilha de recarga −1; ao chegar a 0, volta ao deck.
4. **Janela de ação** — jogador ativo pode: comprar cartas, ignitar Power/Continuous.
5. **Reação a ignição** — oponente pode Retribution (sempre) ou Counter (só se `MaybeCaptureAttemptOnIgnition` = true).
6. **Movimento de peça** no xadrez.
7. **Tentativa de captura** — abre `capture_attempt`; oponente pode reagir com Counter.
8. **Peça capturada** (ou não, após efeitos) → zona Captura.
9. **Fim de turno**.

## Tempo

- Turno principal: **sem timer** (jogador não perde por tempo).
- Não existe timeout automático de reação para fechar `ignite_reaction`/`capture_attempt`.
- Fechamento das janelas de reação ocorre por resposta/confirmação explícita ou auto-resolução por regras de elegibilidade/toggle.

## Ticks de carta no início de turno (detalhe)

| Tipo de carta | Comportamento |
|--------------|---------------|
| Power / não-Continuous | Decrementa contador; resolve **só** quando chega a 0 |
| Continuous | Resolve efeito **a cada turno** do dono enquanto no slot; contador decrementa; ao chegar a 0, recebe mais 1 pulso final, depois vai à recarga |
| Cooldown | −1 em cada entrada de cooldown do jogador que começa o turno; ao chegar a 0, volta ao deck |

## Notas de borda

- `+1 mana` no início do turno só para o jogador **ativo**; não para o oponente.
- Ignição 0: resolve no mesmo snapshot/turno após fechar a janela de reação.
- Múltiplas ignições possíveis se houver mana e slot livre (exceto comportamentos especiais).
- A zona de ignição **ocupada** bloqueia novas ignições do mesmo jogador (salvo Save It For Later).

## Links

- [[Mana e Energizada]] — regras de pool e energizada
- [[Ignição, Ativação e Negação]] — detalhe de ignição/ativação
- [[Janelas de Reação]] — quando e como o oponente pode responder

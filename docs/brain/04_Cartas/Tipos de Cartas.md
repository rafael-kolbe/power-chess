# Tipos de Cartas

> Status: `validated` | Fonte: `PROJECT.md`, `Cards.md`

## Power (verde)

- Uso padrão: ignitar no próprio turno → resolver após contador.
- Abre `ignite_reaction` para o oponente (Ver respostas em [[Janelas de Reação]]).
- Exemplos: Knight Touch, Double Turn, Energy Gain, Piece Swap.

## Retribution (vermelho)

- **Só pode ser usada como resposta** dentro de uma janela já aberta. (Ver respostas em [[Janelas de Reação]])
- Exemplos: Mana Burn, Retaliate, Stop Right There!, Save It For Later.

## Counter (rosa)

- Responde em `capture_attempt` (sempre elegível) e em `ignite_reaction` quando `MaybeCaptureAttemptOnIgnition = true` (atualmente false para todas as cartas).
- Hoje: `MaybeCaptureAttemptOnIgnition = false` em todo o catálogo (aguarda efeitos de captura por ignição).
- Exemplos: Counterattack, Blockade.

## Continuous (azul)

- Efeito ativo **enquanto no slot** de ignição.
- Janela ao oponente **só no turno de entrada** (não a cada tick seguinte).
- Quando contador chega a 0: mais 1 pulso final, depois vai à recarga/banimento.
- Exemplos: Life Drain, Clairvoyance.

## Disruption (laranja)

- Pode ser jogada de duas formas:
  1. **No próprio turno** (alvo: carta no slot de ignição do oponente de turno anterior): sem custo adicional.
  2. **Como resposta em `ignite_reaction`**: custo adicional obrigatório — **banir 1 carta Power da mão** (`banishHandIndex` no protocolo). Sem esse custo, a reação é rejeitada.
- Disruption só como **primeira** resposta da cadeia em `ignite_reaction`.
- Exemplos: Extinguish.

## Links

- [[Catálogo de Cartas]] — texto completo de cada carta
- [[Janelas de Reação]] — janelas em que cada tipo pode atuar
- [[Ignição, Ativação e Negação]] — ciclo de vida

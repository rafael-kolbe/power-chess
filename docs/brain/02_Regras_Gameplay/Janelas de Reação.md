# Janelas de Reação

> Status: `validated` | Fonte: `PROJECT.md` (seções "Janelas de reação", "Toggle Reactions")

## Tipos de janela

| Janela            | Gatilho                                                               | Quem responde                                                                   |
| ----------------- | --------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `ignite_reaction` | Ignição de Power, Continuous ou Disruption pelo oponente | Retribution ou Disruption; Counter só se `MaybeCaptureAttemptOnIgnition = true` |
| `ignite_reaction` | Resposta de Retribution/Disruption dentro da própria cadeia           | Retribution ou Disruption (conforme `eligibleTypes`)                            |
| `capture_attempt` | Tentativa de captura de peça                                          | Counter (somente)                                                               |


## Ordem na cadeia (resolução LIFO)

- `Com resposta:` Ignição de uma carta > Resposta do oponente > Confirmação do primeiro jogador > Animação "Resolving Effects" > Resolução iniciada (LIFO) > Animação "Glow" + Teste success/fail > Se `success` o efeito da carta é chamado aqui; Se `fail`, animação "Glow Black&White" + o efeito não é chamado e nada acontece > Próxima carta da pilha entra em resolução > Animação "Glow" + Teste success/fail > Se `success` o efeito da carta é chamado aqui; Se `fail`, animação "Glow Black&White" + o efeito não é chamado e nada acontece > ... Continua até acabarem as cartas da pilha de reações
- `Sem resposta:` Ignição de uma carta > Oponente confirma com **Confirm Play / Confirmar Jogada** > Resolução iniciada (LIFO) > Animação "Glow" + Teste success/fail > Se `success` o efeito da carta é chamado aqui; Se `fail`, animação "Glow Black&White" + o efeito não é chamado e nada acontece

## Toggle "Reactions" (OFF / ON / AUTO)

| Modo     | Comportamento do servidor                                                                                                                             |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| **OFF**  | Não abre janela enquanto oponente age (confirmação automática por trás); `capture_attempt` também é aceito automaticamente, sem abrir janela |
| **ON**   | Oponente recebe direito de reação mesmo sem resposta viável                                                                                           |
| **AUTO** | Direito de reação só se identificável que o jogador pode responder: cartas na mão + mana atual + regra de cópia na recarga + tipo permitido na janela |

- Qualquer jogador pode alterar o toggle **a qualquer momento**. O toggle não é compartilhado; cada jogador tem o seu.
- O servidor respeita o **novo** estado imediatamente.
- Condições textuais das Counter são validadas ao jogar e ao resolver a carta; o AUTO de `capture_attempt` continua conservador por economia/tipo.

## Counters implementadas

- **Counterattack:** em `capture_attempt`, exige que a peça atacante tenha efeito **Power** ativo. Se resolver, a peça atacante é capturada em vez da peça defendida e o movimento do atacante é consumido.
- **Blockade:** responde a um Counter que capturaria/removeria a peça atacante. Se resolver, nega esse Counter, cancela a captura original, mantém o turno do atacante para mover outra peça e bloqueia a peça atacante original por 1 turno.

## Regra de cópia na recarga

- Um jogador **não pode ignitar** uma carta se já existir **cópia dela** na zona de recarga.
- O AUTO considera essa restrição ao decidir abrir janela.

## Disruption como resposta

Ao jogar Disruption como reação em `ignite_reaction`: **custo adicional obrigatório — banir 1 carta Power da mão** (`banishHandIndex` no protocolo). Sem o custo, a reação é rejeitada.

## Disruption no turno próprio

- Disruption pode ser jogada como carta inicial no turno do jogador, desde que exista **alvo válido na ignição do oponente**.
- Nesse caso, a ignição também abre `ignite_reaction` para o oponente.

## Timeout de reação

- Não existe timeout automático no servidor para encerrar `ignite_reaction`/`capture_attempt`.
- A janela só fecha por ação explícita (responder/confirmar resolução) ou por auto-resolução conforme regras de elegibilidade/toggle.

## Links

- [[Ignição, Ativação e Negação]] — o que dispara as janelas
- [[Tipos de Cartas]] — papéis de cada tipo (Retribution, Counter, Disruption)
- [[Transporte e Envelope]] — campo `reactionWindow` no snapshot

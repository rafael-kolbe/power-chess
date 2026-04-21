# Ignição, Ativação e Negação

> Status: `validated` | Fonte: `PROJECT.md` (seção "Termos canônicos")

## Definições canônicas

| Termo | Definição |
|-------|-----------|
| **Ignição** | Ação de mover uma carta da mão para a zona de ignição com intenção de ativar seu efeito. Consome mana. Abre janela de reação do oponente para Power, Continuous e Disruption (quando Disruption é válida no turno próprio). |
| **Ativação** | Ação de resolver o efeito de uma carta que está na zona de ignição após o tempo indicado (contador chega a 0). O servidor aplica o efeito. |
| **Negação** | O efeito da carta na ignição é marcado como negado; ao resolver, o efeito conclui em **falha** e a carta vai à recarga. O estado `ignitionEffectNegated` permanece até a carta sair da ignição. |

## Fluxo de vida de uma carta

```
Mão → [ignição paga mana] → Slot de ignição → [contador chega a 0] → Ativação
                                                    ↓ se negada
                                              Falha na resolução
                                                    ↓
                                            Recarga (cooldown)
                                                    ↓ contador chega a 0
                                              Fundo do deck
```

## Quem abre janela de reação

| Quem ignita     | Abre janela?                                                                                                                               |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **Power**       | Sim — `ignite_reaction`                                                                                                                    |
| **Continuous**  | Sim — `ignite_reaction` (apenas no turno em que entra; não a cada tick seguinte)                                                           |
| **Retribution** | Não inicia jogada; só entra como resposta dentro de `ignite_reaction` |
| **Counter**     | Não pelo slot (apenas em `capture_attempt`)                                                                                                |
| **Disruption**  | Sim — no próprio turno (somente se houver alvo válido na ignição do oponente); como resposta: custo adicional (banir 1 Power da mão)     |

## Carta Continuous: detalhe

- Janela ao oponente **somente no turno em que a carta entra** no slot.
- Efeito resolve **a cada turno** do dono enquanto no slot (incluindo o primeiro pulso no mesmo turno de entrada, após fechar `ignite_reaction`).
- Quando contador chega a 0: recebe mais **1 pulso final** no próximo início de turno, então vai à recarga/banimento.
- Se negada na ignição: permanece negada **todos os turnos** em que continuar no slot; efeito não "reativa".

## Ignição 0

- Abre janela de resposta, mas já entra na pilha de reações no mesmo turno, como se o contador tivesse chegado a 0.

## Links

- [[Janelas de Reação]] — quem pode responder em cada janela
- [[Tipos de Cartas]] — comportamento por tipo
- [[Turno e Ordem]] — onde a ignição se encaixa no turno

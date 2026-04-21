# Canon — Verdades Imutáveis

> Estas regras nunca são flexibilizadas sem decisão explícita de produto.
> Status: `validated` | Não alterar sem atualizar `PROJECT.md`.

## Regras de xadrez

- O **rei nunca é capturado** diretamente; tentativas são movimentos ilegais (servidor rejeita).
- Movimento e xeque/xeque-mate seguem o **xadrez tradicional**.
- Poderes **podem causar xeque-mate**, salvo se o texto da carta proibir explicitamente.
- Turno de xadrez **sem limite de tempo** (não existe timer principal de turno).

## Autoridade do servidor

- O **servidor é a fonte de verdade** para turno, mana, ignição e cooldown.
- WebSocket events são a fonte de verdade para estado da partida.
- Cliente pode melhorar UX com validação local, mas **decisão final é do servidor**.

## Cartas e efeitos

- **Continuous**: janela ao oponente **só no turno de entrada** no slot.
- **Retribution não inicia jogada**: só pode ser usada como resposta.
- **Disruption pode iniciar jogada** no turno próprio, somente com alvo válido na ignição do oponente.
- **Disruption como resposta** exige banir 1 Power da mão como custo adicional.
- Janelas de reação não encerram por timeout automático no servidor.
- Carta negada permanece negada durante **todos os turnos** que continuar na ignição.
- Cartas **banidas não voltam** ao deck (salvo efeito específico).
- Não pode ignitar carta se há **cópia na recarga**.

## Mana e energizada

- Gastar mana em **compra de carta** não gera mana energizada.
- Habilidade de jogador **não pode ser negada** por cartas.
- Usar habilidade **consome o turno**.

## Deck e texto canônico

- Deck: máx. **20 cartas**; máx. **10 decks por conta**.
- Texto das cartas/habilidades: **preserve exatamente**; nunca parafraseie, "melhore" ou invente.
- `Cards.md` e `PlayerSkills.md` são os documentos de referência de texto.

## Git e entregas

- Push em **`main`** só com **autorização explícita por escrito**.
- Push em **`dev`** só com **consentimento explícito**.
- Push autônomo apenas em **`feature/<feature-name>`** (ou branch acordada).
- **TDD obrigatório**: red → green → refactor antes de qualquer commit de comportamento no backend.
- `go test ./...` deve estar verde antes de commit.

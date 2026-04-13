# Handoff — feature/gameplay (reações, tempo, desconexão)

Use este arquivo quando o contexto da conversa se perder. A fonte canônica detalhada é **[PROJECT.md](../PROJECT.md)** (seções de reações, termos, desconexão).

## Toggle Reactions (header): OFF / ON / AUTO

- **Implementado (captura):** mensagem `set_reaction_mode`, campo `players[].reactionMode` no snapshot, e auto-resolução de `capture_attempt` no servidor conforme `off` / `on` / `auto` (AUTO usa `EligibleForCaptureCounterReactionAUTO` em `internal/gameplay/reaction_eligibility.go`).
- Qualquer jogador pode mudar o toggle **a qualquer momento**; o **servidor** passa a respeitar o novo estado **a partir desse instante** (não só UI).
- **OFF**: **não abrir** janelas de reação só para dar pass — o jogador da vez segue sem micro-interrupções; equivalência conceitual ao oponente não poder reagir naquele trecho.
- **ON**: oponente recebe direito de reação nas ações elegíveis **mesmo** sem carta/mana útil.
- **AUTO**: direito de reação só se houver **caminho plausível** de resposta: cartas na mão, **mana atual**, **não haver cópia da mesma carta na recarga** (bloqueia ignição da cópia na mão), tipo de carta permitido na janela. **Fora do escopo desta feature:** validação das **condições textuais** das Counter cards no AUTO — permanece como **TODO** no código (`internal/match/reactions.go`) para retomada futura.

## Tempo (30s + 30s)

- Turno principal do jogador da vez: **30s** (ou valor do servidor); fim → **+1 strike**, passa a vez.
- Ao abrir direito de reação: **pausa** o timer do jogador da vez; **inicia** timer de reação do oponente (**30s**).
- Fim da reação (passou / jogou carta / chain fechou): pausa timer de reação; **retoma** timer do jogador da vez.
- **Cap** prático ~**60s** por “volta” (30 ativo + 30 reação), salvo chain.
- Timeout da **reação**: oponente **não pode mais reagir** naquele turno às demais jogadas (efeito similar a OFF para o restante do turno), **sem** strike por isso.
- **Durante resolução de chain (stack)**: timers dos **dois** pausados.

## Termos (ignição × ativação) e Retribution

- **Ignição**: mover carta para o slot de ignição **com intenção** de ativar o efeito (custo pago conforme regras).
- **Ativação**: executar o efeito da carta **no slot**, após o tempo (**ignição** em turnos) indicado na carta.
- **Retribution**: só em resposta à **ignição** de uma carta. **Sem** janela só porque a carta **permanece** no slot; **sem** janela em cima da **ativação da própria Retribution** (não concede nova janela genérica ao oponente por esse fato).
- Carta **negada na ignição**: permanece **negada** enquanto estiver no slot; ao resolver, efeito falha e vai à **recarga**. **Continuous** negada na ignição: permanece negada **todo** o tempo no slot.

## Captura (UX + janela)

- **Arrastar** peça sobre a captura **ou** **clicar** na peça alvo com intenção de capturar → abre janela de reação ao oponente.
- Quem ataca **não** confirma de novo; quem **responde** (oponente) confirma **passar** ou **reagir** (cartas).

## Desconexão

- Servidor detecta desconexão **na hora**; **pausa** a partida (nada de jogada válida para o outro enquanto pausado).
- **Grace mínimo de 5 s por disconnect:** só depois de **5 s** desde a detecção o servidor pode declarar vitória do outro **por desconexão** (evita win instantâneo).
- **60s** = orçamento **total** desconectado por jogador por partida (**não** zera a cada queda).
- Banner **verde**: **pt-BR** *“Jogador desconectado (Xs)”* · **EN** *“Opponent disconnected (Xs)”* — **X** dinâmico (protocolo).
- Vitória ao esgotar orçamento (respeitando grace); reconectar **retoma** processo pausado.

## i18n

- Traduzir textos de jogo (EN + pt-BR) salvo exceções que o designer especificar.

## Cartas / efeitos

- Foco inicial: **tipo** de carta e **abertura correta de janelas**; efeitos finos das cartas depois.

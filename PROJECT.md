# Power Chess — contexto do projeto

## Visão geral

- Multiplayer **1v1**, xadrez clássico + **cartas** e **habilidades de jogador**.
- **Backend** em Go com WebSockets; estado da partida autoritativo no servidor.
- **Frontend** atual: HTML, CSS e JavaScript (HUD de desenvolvimento).
- Planejado: filas **casual** e **ranqueada** (ELO).

### Documentos de regras detalhadas

- **[Cards.md](Cards.md)** — cartas iniciais  
- **[PlayerSkills.md](PlayerSkills.md)** — habilidades selecionáveis  
- **[PROTOCOL.md](PROTOCOL.md)** — transporte e snapshots  

---

## Regras de tabuleiro e turno

- Movimento e xeque/xeque-mate seguem o xadrez tradicional.
- O turno de xadrez está **sem limite de tempo**: o jogador da vez pode jogar sem timer principal.
- Vitória por **checkmate** (ou condições de fim expostas no protocolo).
- Poderes podem causar xeque-mate, salvo se o texto da carta proibir.
- O **rei nunca é capturado** diretamente; jogadas que “capturam” o rei são ilegais.
- Peças capturadas vão para **Captura** (zona de peças capturadas pelo oponente; no protocolo ainda `graveyard*`); alguns efeitos podem interagir com isso.

### Termos: ignição, ativação e negação (canônico)

- **Ignição**: ação de **mover uma carta para a zona de ignição** com a **intenção de ativar** seu efeito (custo e validações conforme regras do servidor).
- **Ativação**: ação de **resolver o efeito** de uma carta que está na zona de ignição **após** o tempo indicado na carta (contador de ignição em turnos), quando o servidor aplica o efeito.
- **Retribution Card**: só pode ser usada **como resposta** dentro de uma janela já aberta por outra regra (ver abaixo). **Retribution nunca abre** janela de resposta: não por ignição de Retribution, nem enquanto uma carta **apenas permanece** na ignição sem novo ato de ignição, nem pela **ativação/resolução da própria Retribution** (isso não concede, por si, nova janela ao oponente).
- **Quem abre janela de resposta (servidor):** **Power** (sempre, ao ignitar no fluxo permitido), **Continuous** (sempre, nas mesmas condições), e **Counter** somente no contexto em que essa Counter está ligada a uma **tentativa de captura** (ex.: janela `capture_attempt` após movimento de captura no xadrez — o motor atual não ignita Counter pelo slot como Power).
- **Quem pode responder nessa janela:** em **`capture_attempt`**, só **Counter**. Em **`ignite_reaction`**, **Retribution** e/ou **Counter** conforme `eligibleTypes`; **Counter** na primeira resposta só quando o catálogo marca `MaybeCaptureAttemptOnIgnition` na carta em ignição (hoje **false** para todas até efeitos de captura por ignição existirem).
- **Carta negada na ignição**: permanece **negada** durante **todos** os turnos em que continuar na zona de ignição; ao resolver, o efeito conclui **em falha** e a carta vai à **recarga**. **Continuous**: como o efeito se aplica ao longo dos turnos, se for **negada no momento da ignição**, permanece **negada durante todo** o período em que estiver no slot (efeito não “reativa” enquanto lá estiver).

### Toggle “Reactions” no header: OFF / ON / AUTO

- Qualquer jogador pode alterar o toggle **a qualquer momento**; a partir desse instante o **servidor** deve respeitar o **novo** estado (autoridade no backend, não só preferência de UI).
- **OFF**: enquanto o oponente age, **não** conceder direito de reação — e **não abrir** janelas só para o oponente “dar pass”. O jogador da vez joga **sem micro-interrupções** por reação.
- **ON**: oponente recebe direito de reação nas ações elegíveis **mesmo** que não tenha resposta viável (mana/cartas/condições).
- **AUTO**: direito de reação só se for **identificável** que o jogador pode responder: cartas na **mão**, **mana atual**, **regra de cópia na recarga** (não pode **ignitar** uma carta se já existir **cópia** dela na zona de recarga), e **tipo** de carta permitido na janela. As **condições textuais** das Counter cards no AUTO ficam para **implementação futura**; `TODO` em `internal/match/reactions.go`.

### Tempo: sem relógio de turno, com relógio de response

- Não existe timer principal de turno: o jogador da vez não perde turno por tempo.
- O timer de **response** continua valendo por assento durante janelas de reação.
- Se o **tempo de reação** do oponente **acabar**: ele não pode mais reagir naquele turno (equivalente a OFF até virar o turno).
- Ao iniciar novo turno, o budget de response de ambos os assentos volta ao valor base configurado.
- Durante resolução de chain, timers de response ficam pausados.
- **Observação (decisão de produto):** o servidor mantém budgets por assento para response (A.response e B.response) e esse saldo não reseta entre janelas do mesmo turno.

### Ordem de turno (referência — alinhar ao motor)

1. **Início do turno** — +1 mana ao pool do jogador ativo (e demais ticks do servidor).
2. **Tick de ignição** — contador da carta no slot de ignição −1 (quando aplicável); se chegar a 0, **ativação** do efeito conforme regras.
3. **Tick de recarga** — contador de cada carta na pilha de recarga −1; ao chegar a 0, a carta volta ao fundo do deck.
4. **Janela de ação** (turno do jogador ativo): comprar cartas, **ignição** de **Power** / **Continuous** (e demais tipos permitidos), etc., conforme slot e regras.
5. **Reação a ignição** (se toggle e elegibilidade permitirem): oponente pode **Retribution** (sempre na janela) e **Counter** só quando `MaybeCaptureAttemptOnIgnition` for **true** para a carta em ignição (efeitos ainda não implementados; catálogo mantém **false**). **Continuous**: janela ao oponente **só no turno em que a carta entra** no slot, não a cada tick seguinte enquanto permanecer lá. Cadeia **LIFO** onde aplicável.
6. **Movimento de peça** no xadrez.
7. **Tentativa de captura**: ao **soltar** a peça sobre a captura **ou** ao **clicar** na peça alvo com intenção de capturar, abre-se `capture_attempt` ao oponente (**Counter** como primeira resposta; **Retribution** não abre cadeia em `capture_attempt`). O jogador que **ataca** **não** precisa de um segundo “confirmar ataque”; quem **confirma passar ou reagir** é o **oponente**.
8. **Peça capturada** (ou não, após efeitos) — Captura (`graveyard` no servidor), etc.
9. **Fim de turno**.

> Efeitos completos das cartas estão em `Cards.md`. O motor e o protocolo são a fonte da verdade; números de tempo podem divergir do legado “10 s” até o snapshot unificar em **30 s**.

---

## Mana e mana energizada

- No início da partida (antes do primeiro **início de turno** aplicado pelo servidor), o pool de mana de cada jogador está em **0**; o primeiro ganho de mana no turno vem do passo “início do turno” (+1 mana ao jogador ativo), conforme `state_snapshot`.
- A cada **início de turno** do jogador da vez, o servidor concede **+1 mana** (respeitando o máximo do pool; se já estiver no teto, não aumenta).
- Cada **captura de peça no xadrez** (incluindo en passant) concede **+1 mana** ao jogador que capturou (também respeitando o máximo). Valores finais vêm do `state_snapshot`.
- Pools com **máximo** (ex.: mana 10, energizada 20 — conforme implementação atual).
- Mana gasta em **poderes de carta** gera **mana energizada** (regra geral; exceções no texto das cartas).
- Mana gasta só para **comprar carta** não gera mana energizada (conforme regras do jogo).

## Habilidade especial (mana energizada)

- Ao atingir o máximo da pool de energizada, o jogador pode **gastar toda** a energizada para ativar a habilidade escolhida.
- Cada uso aumenta o teto da pool de energizada para a próxima vez (ex.: +10).
- Só no **próprio turno**; **consome o turno**; **não pode ser negada** por cartas.

---

## Sistema de cartas (poderes)

Cada carta tem: **custo**, **ignição** (turnos até resolver), **recarga** (cooldown), **tipo** (Power, Retribution, Counter, Continuous).

Fluxo típico de ativação:

1. Mana consumida na ativação.  
2. Carta vai ao **slot de ignição** (visível ao oponente).  
3. O oponente pode reagir em janelas permitidas.  
4. Sucesso ou falha → carta vai para **recarga**; ao terminar, volta ao **deck** (exceto banimento e efeitos específicos).

**Ticks no início de turno (servidor):** +1 mana do jogador da vez (até o máximo); **−1** no contador de **ignição** da carta na zona de ignição **desse jogador** (cada assento tem o seu slot; o tick aplica-se à carta desse jogador quando o turno dele começa); **−1** em cada entrada de **cooldown** do jogador que está começando o turno (entradas que chegam a 0 voltam ao deck).

### Janelas de reação (tipos e papéis)

- **Abrem janela:** ignição de **Power** e de **Continuous** (sempre, quando o fluxo do servidor abre `ignite_reaction`); tentativa de **captura** no xadrez abre `capture_attempt`. **Retribution não abre janela.**
- **Respondem:** **Retribution** quando listada em `reactionWindow.eligibleTypes` (tipicamente `ignite_reaction`). **Counter** em **`capture_attempt`** (primeira resposta só Counter) e em **`ignite_reaction`** quando `MaybeCaptureAttemptOnIgnition` for **true** na carta ignitada.
- Na cadeia em `ignite_reaction`: após **Retribution**, só **Retribution**; após **Counter**, só **Counter** quando permitido. Em `capture_attempt`, a cadeia é **só Counter** (ex.: **Counterattack** / **Blockade**). Resolução **LIFO**.
- **Continuous**: oponente só tem janela **no turno em que a carta entra** no slot, não a cada turno seguinte enquanto ela permanecer lá.
- **Ignition 0**: resolve no mesmo snapshot/turno conforme servidor; múltiplas ignições possíveis se houver mana e slot livre.
- A zona de ignição **desse jogador** ocupada bloqueia novas ignições **dele**, exceto comportamentos especiais (ex.: **Save It For Later**).

---

## Deck e mão

- Deck por jogador (tamanho conforme regras atuais no código).
- **Compra inicial**: só depois que **os dois** jogadores estão na sala o servidor embaralha cada deck (RNG criptográfico) e cada um compra 3 cartas. Quem entra primeiro **não** compra antes do oponente.
- **Mulligan** (estilo Shadowverse): cada jogador escolhe qualquer subconjunto da mão inicial para devolver ao deck; o deck é embaralhado de novo e o jogador compra a mesma quantidade de cartas. Ambos veem **quantas** cartas cada um devolveu (não vêem quais cartas). Há **15 s** a partir do início da fase de mulligan; ao expirar, o servidor confirma automaticamente quem ainda não confirmou como se não devolvesse cartas. Após os dois confirmarem (ou após esse auto-confirm), a partida de xadrez prossegue.
- Compra pagando mana conforme implementação (fora da abertura).
- Limite de mão e cópias por carta conforme regras do jogo.
- Cartas **banidas** não voltam ao deck salvo efeito específico.

---

## Desconexão e fim de partida

- O **servidor** deve detectar desconexão (WebSocket fechado / perda de sessão) **de imediato** e **pausar** qualquer fluxo de partida que dependa daquele jogador (jogadas, timers de reação, chains — animações puramente locais no cliente podem terminar, mas estado autoritativo fica **pausado** até retomada).
- Enquanto a partida estiver **pausada por desconexão**, o jogador **ainda conectado não pode avançar** a partida (nada de novo lance válido que dependa do outro lado).
- **Orçamento de desconexão por jogador por partida: 60 s no total** (não reinicia a cada queda: se o jogador cair de novo na mesma partida, o contador **continua de onde parou**).
- **Grace mínimo de 5 s por evento de desconexão:** desde a detecção, o servidor **não** declara vitória ao outro jogador **por desconexão** antes de passarem **no mínimo 5 s** (falhas de rede ou fechar aba por instantes não encerram a partida na hora). **Após** esse grace, se as regras de fim por desconexão forem atendidas (ex.: orçamento de 60 s esgotado), a vitória do conectado pode ser aplicada.
- **Banner verde** na área do oponente, com contagem do tempo restante do orçamento (ou campo equivalente no protocolo). Textos sugeridos: **pt-BR** — *“Jogador desconectado ({s}s)”*; **EN** — *“Opponent disconnected ({s}s)”* (o **`{s}`** é o valor dinâmico em segundos vindo do cliente com base no protocolo).
- Esgotado o orçamento (após o grace, conforme acima): vitória do conectado por desconexão / fim de partida (detalhar em [PROTOCOL.md](PROTOCOL.md)).
- Ambos offline → partida cancelada sem vencedor (conforme `endReason`), salvo regra futura explícita.
- `leave_match` com oponente conectado pode encerrar com vitória do outro.  
- Detalhes em [PROTOCOL.md](PROTOCOL.md) (`matchEnded`, `winner`, `endReason`, rematch).

### Idiomas (UI)

- Textos voltados ao jogador devem existir em **inglês** e **português (pt-BR)** no jogo, salvo exceções que o designer listar explicitamente.

---

## Modos de jogo (planejado / parcial)

- **Casual**: pareamento flexível; ranking pode não contar.  
- **Ranqueado**: pareamento por proximidade de ELO; limite de diferença entre jogadores.

---

## Arquitetura técnica

### Backend

- Go + WebSocket.  
- Docker Compose para desenvolvimento.  
- Postgres para persistência opcional de salas.  
- Evolução possível: cloud + orquestração (fora do escopo imediato).

### Frontend

- Stack atual estática; evolução possível para framework e deploy em CDN.

### Implementação

- **Servidor** é a fonte da verdade para jogadas, mana, cartas e efeitos.  
- Efeitos modelados de forma extensível (definições + resolução), evitando ramificações gigantes por carta.  
- Testes automatizados no backend para regras críticas.

---

## Qualidade e commits

- Toda funcionalidade nova ou bugfix relevante deve incluir **testes**.  
- Commits na **`branch feature/<feature-name>`** quando a entrega estiver **coesa** e **`go test ./...`** (e E2E quando aplicável) estiver verde.

### Política de push (Git)

- **`main`**: não fazer push sem **autorização explícita por escrito**.
- **`dev`**: não fazer push sem **consentimento** explícito.
- **Trabalho corrente**: push automático permitido apenas em **`feature/<feature-name>`** (ou branch de trabalho equivalente acordada). A regra detalhada para agentes está em `.cursor/rules/git-branch-push.mdc`.

---

## Roadmap (próximos passos)

Ordem aproximada; itens podem ser paralelizados onde fizer sentido.

### Contas e persistência de jogador

- **Contas de usuário** e **autenticação** (sessão/JWT ou fluxo equivalente).  
- Ligação conta ↔ partidas, deck salvo, preferências.

### Decks e coleção

- **Montagem de decks** (deck builder) respeitando limites de cópias e regras de formato.  
- Validação no servidor antes de entrar em fila ou sala.

### UI de partida — zonas de cartas e peças

- **Pilha de deck** (comprar / contagem).  
- **Mão** (cartas visíveis só ao dono).  
- **Campo de ignição** e **recarga** (slots claros).  
- **Captura** (peças capturadas pelo oponente).  
- **Pilha de cartas banidas**.

### Interação e feedback

- **Hover** nas cartas (preview, escala, borda).  
- **Drag and drop**: peças no tabuleiro; cartas da mão / para ignição (onde o protocolo permitir).  
- **Mana e mana energizada**: barras, ticks, feedback ao gastar/ganhar.  
- **Animações** de ativação de cartas e transição entre zonas (mão → ignição → recarga → banido).

### Cadeias e resolução

- Visualização de **cadeias** (stack de reações), **resolução em ordem** (LIFO onde aplicável), tempo limite de janela.  
- Movimento visual de cartas entre slots conforme o estado do servidor.

### Áudio e polimento

- **Efeitos sonoros** (jogada, captura, ativação, vitória/derrota).

### Regras e conteúdo

- **Implementação completa** da lógica de **todas as cartas** e **habilidades de jogador** descritas nos documentos, com testes.  
- Manter [Cards.md](Cards.md) / [PlayerSkills.md](PlayerSkills.md) alinhados ao código (`internal/gameplay`).

---

## Arquitetura sugerida para poderes (referência)

- `CardDefinition` + efeitos parametrizados + `CardInstance` (zona: mão, ignição, recarga, deck, banido).  
- Pipeline: validar → consumir recursos → aplicar efeitos → emitir eventos → validar pós-estado.  
- Estado temporário (buffs, janelas) com expiração por turno.

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
- **Tempo por jogada** (padrão): configurável no servidor (ex.: 20–30 s no snapshot).
- Se o jogador não agir a tempo: **+1 strike** e passa o turno.
- **3 strikes** → derrota imediata.
- Vitória por **checkmate** (ou condições de fim expostas no protocolo).
- Poderes podem causar xeque-mate, salvo se o texto da carta proibir.
- O **rei nunca é capturado** diretamente; jogadas que “capturam” o rei são ilegais.
- Peças capturadas vão para o **cemitério** (graveyard); alguns efeitos podem interagir com isso.

### Ordem de turno (referência canônica)

1. **Início do turno** — +1 mana ao pool do jogador ativo.
2. **Tick de ignição** — contador da carta no slot de ignição −1 (animação); se chegar a 0, o efeito ativa.
3. **Tick de recarga** — contador de cada carta na pilha de recarga −1 (animação); ao chegar a 0, a carta volta ao fundo do deck com movimento fluído.
4. **Janela de ação** (opcional, durante o próprio turno): o jogador pode comprar cartas (2 mana/carta, mão < 5) e/ou colocar uma carta **Power** ou **Continuous** na ignição (slot deve estar vazio, salvo `save-it-for-later`).
5. **Janela de Retribution**: se o jogador ativou uma carta, o oponente tem **10 s** para responder com uma carta **Retribution**. Efeitos resolvem em cadeia LIFO.
6. **Movimento de peça**: jogador executa a jogada de xadrez.
7. **Tentativa de captura**: se a jogada capturaria uma peça, abre **janela de Counter** (10 s). O oponente pode responder com carta **Counter**. Efeitos resolvem em cadeia LIFO.
8. **Peça capturada** (ou não, dependendo dos efeitos) — vai ao cemitério do capturado.
9. **Fim de turno**.

> Efeitos completos das cartas e resolução de cadeia detalhada estão em `Cards.md`. O motor é a fonte da verdade.

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

**Ticks no início de turno (servidor):** +1 mana do jogador da vez (até o máximo); **−1** no contador de **ignição** da carta no slot de ignição (compartilhado, uma vez por início de turno de qualquer lado); **−1** em cada entrada de **cooldown** do jogador que está começando o turno (entradas que chegam a 0 voltam ao deck).

### Janelas de reação e Counter

- **Power** e **Continuous**: no turno do jogador.  
- **Retribution** e **Counter**: em **reaction windows** (cadeias).  
- Tentativa de **captura** válida abre janela `capture_attempt` (inclui en passant).  
- Na cadeia de captura, a primeira resposta do oponente costuma ser **Counter**; **Counter** responde a **Counter** onde aplicável.  
- **Counterattack** / **Blockade**: ver texto em [Cards.md](Cards.md) e regras no servidor.  
- **Ignition 0**: resolve no mesmo turno; múltiplas ativações possíveis se houver mana e slot livre.  
- Slot de ignição ocupado bloqueia novas ativações, exceto comportamentos especiais (ex.: **Save It For Later**).

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

- Ambos offline → partida cancelada sem vencedor (conforme `endReason`).  
- Um jogador offline → janela de reconexão (ex.: 60 s); após isso, vitória do outro por timeout de desconexão.  
- `leave_match` com oponente conectado pode encerrar com vitória do outro.  
- Detalhes em [PROTOCOL.md](PROTOCOL.md) (`matchEnded`, `winner`, `endReason`, rematch).

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
- Commits na **`main`** quando a entrega estiver **coesa** e **`go test ./...`** (e E2E quando aplicável) estiver verde.

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
- **Cemitério de peças** (capturas).  
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

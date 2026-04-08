# Power Chess - Project Context

## Visao Geral
- Jogo multiplayer de xadrez 1v1 com poderes especiais.
- Backend em Go com WebSockets.
- Frontend inicial em HTML, CSS e JavaScript.
- Matchmaking com filas unranked e ranked.

## Regras do Tabuleiro e Turnos
- O tabuleiro segue as regras de movimento do xadrez tradicional.
- Cada jogador tem 20 segundos por jogada (padrao).
- Se o jogador nao agir no tempo: recebe 1 strike e passa o turno.
- Com 3 strikes, derrota automatica.
- Vitoria ocorre por checkmate.
- Nenhum poder pode causar checkmate instantaneo; esta jogada deve ser invalidada.
- Pecas capturadas vao para o Graveyard e podem retornar conforme poderes.

## Sistema de Mana e Mana Energizada
- A cada jogada, o jogador ganha +1 mana.
- Cada captura gera +1 mana extra no turno (maximo padrao: +1 por turno).
- Poderes podem alterar esse limite (ex.: +1 turno, +1 extra).
- Pool de mana: maximo padrao 10.
- Pool de mana energizada: maximo padrao 20.
- Se a pool estiver no maximo, nao ganha mana adicional.
- Cada 1 mana gasta em poderes gera 1 mana energizada.
- Mana gasta para comprar carta NAO gera mana energizada.

## Habilidade Especial por Mana Energizada
- Ao atingir o maximo de mana energizada, o jogador pode ativar habilidade especial.
- Essa ativacao consome toda mana energizada.
- A cada uso, o maximo da pool de mana energizada aumenta em +10 para a proxima ativacao.
- Exemplo: 20 -> 30 -> 40...

## Sistema de Poderes (Cartas)
- Cada carta possui: custo de mana, tempo de ignicao (0 a 5 turnos) e recarga (em turnos).
- Ao ativar um poder:
  1. Mana e consumida no momento da ativacao.
  2. Carta vai para o slot de ignicao visivel ao adversario.
  3. Adversario pode influenciar se a ativacao tera sucesso.
  4. Com sucesso ou falha, carta vai para slot de recarga.
  5. Ao terminar recarga, carta retorna ao deck.

## Exemplos de Poderes e Habilidades (Inicial)
- Mover uma peca 2x.
- Adicionar movimento de cavalo a outra peca por 1 turno.
- Negar ignicao de uma carta e ganhar mana energizada igual ao custo da carta negada.
- +3 mana.
- +10 mana energizada.
- Jogar 2x seguidas, pulando o turno do oponente.
- Transportar uma peca para outra casa na mesma linha.
- Ressuscitar 1 peca do graveyard para o proprio campo.
- Limite maximo de cartas na mao +2.
- Capacidade maxima de mana +5.
- Sacrificar 1 peao; se ativacao for bem-sucedida, sacar 2 cartas e ganhar +6 mana.

## Deck e Mao
- Deck de 20 cartas por jogador.
- Compra inicial: 3 cartas.
- Cartas na mao sao ocultas ao adversario (padrao).
- Jogador pode gastar 2 mana para comprar 1 carta, sem limite por turno.
- Limite de mao: 5 cartas.
- Cartas no banimento nao retornam ao deck, exceto por poderes especificos.
- Maximo de 3 copias por carta, salvo cartas limitadas/banidas.

## Modos de Jogo
- Inicialmente apenas 1v1 multiplayer.
- Fila unranked:
  - pareamento sem restricao forte de ranking;
  - resultado nao afeta ranking.
- Fila ranked:
  - pareamento por ranking proximo;
  - diferenca maxima de 450 ELO entre jogadores.

## Arquitetura Tecnica
### Backend
- Golang + WebSockets para partidas em tempo real.
- Docker Compose para ambiente local.
- Exposicao inicial via ngrok.
- Migracao futura para GCP + Kubernetes (escalabilidade).

### Frontend
- HTML + CSS + JavaScript.
- Deploy inicial no Netlify.
- Possivel migracao futura para Next.js e hospedagem no Firebase (ou equivalente).

## Diretrizes de Implementacao
- Validar jogadas no servidor (autoridade do estado da partida).
- Garantir consistencia de turno, strike e timers em ambos clientes.
- Bloquear ativacoes de poderes que resultem em checkmate instantaneo.
- Tratar recursos (mana, mana energizada, cooldowns e ignicao) como estado sincronizado de jogo.
- Cobrir regras criticas com testes automatizados no backend.

## Arquitetura Sugerida para Sistema Adaptavel de Poderes
- Modelar cada carta com metadados e efeitos desacoplados:
  - `CardDefinition` (id, custo, ignicao, recarga, tags).
  - `Effect` (tipo, alvo, duracao, parametros).
  - `CardInstance` (estado em mao, ignicao, recarga, deck, banimento).
- Criar pipeline de resolucao no backend:
  1. validacao de pre-condicoes;
  2. consumo de recursos;
  3. aplicacao de efeitos;
  4. eventos para cliente (WebSocket);
  5. validacao pos-efeito (incluindo bloqueio de checkmate instantaneo).
- Evitar `if/else` gigante por carta; usar registrador de efeitos por tipo.
- Manter efeitos temporarios em estado por turno (buffs/debuffs), com expiracao automatica.
- Persistir estado de partida para reconexao e auditoria de jogadas.

## Qualidade e Testes
- Toda funcionalidade nova deve incluir testes.
- Toda correcao de bug deve incluir teste de regressao.
- Politica de commit: somente com suite relevante 100% passando localmente/CI.

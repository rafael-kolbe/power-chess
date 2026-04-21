# Roadmap

> Status: `draft` | Fonte: `PROJECT.md` (seção "Roadmap")

## Próximos passos (ordem aproximada)

### Contas e persistência de jogador
- [ ] Contas de usuário e autenticação (sessão/JWT)
- [ ] Ligação conta ↔ partidas, deck salvo, preferências

### Decks e coleção
- [ ] Montagem de decks (deck builder) com limites de cópias
- [ ] Validação no servidor antes de fila ou sala

### UI de partida — zonas de cartas e peças
- [ ] Pilha de deck (comprar / contagem)
- [ ] Mão (cartas visíveis só ao dono)
- [ ] Campo de ignição e recarga (slots claros)
- [ ] Captura (peças capturadas pelo oponente)
- [ ] Pilha de cartas banidas

### Interação e feedback
- [ ] Hover nas cartas (preview, escala, borda)
- [ ] Drag and drop: peças no tabuleiro; cartas da mão/para ignição
- [ ] Mana e mana energizada: barras, ticks, feedback ao gastar/ganhar
- [ ] Animações de ativação de cartas e transição entre zonas

### Cadeias e resolução
- [ ] Visualização de cadeias (stack de reações), resolução LIFO, tempo limite de janela
- [ ] Movimento visual de cartas entre slots

### Áudio e polimento
- [ ] Efeitos sonoros (jogada, captura, ativação, vitória/derrota)

### Regras e conteúdo
- [ ] Implementação completa de todas as cartas em `Cards.md`
- [ ] Implementação de todas as habilidades em `PlayerSkills.md`
- [ ] Manter `Cards.md` / `PlayerSkills.md` alinhados ao código

### Modos de jogo
- [ ] **Casual**: pareamento flexível
- [ ] **Ranqueado**: ELO com limite de diferença entre jogadores

## Links

- [[Catálogo de Cartas]] — status de implementação por carta
- [[Estrutura de Pacotes]] — onde cada feature aterra no código

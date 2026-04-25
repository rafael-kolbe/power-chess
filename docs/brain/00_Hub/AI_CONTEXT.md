# AI_CONTEXT — Power Chess

> Ponto de entrada para IA. Leia este arquivo antes de qualquer tarefa no projeto.
> Status: `validated`

## O que é o projeto

Jogo multiplayer **1v1** de xadrez com sistema de **cartas de poder** (TCG-like) e **habilidades de jogador**. Backend autoritativo em Go com WebSocket; frontend em HTML/CSS/JS.

## Documentos fonte de verdade (por ordem de autoridade)

1. `PROJECT.md` — regras de produto, gameplay, roadmap, arquitetura
2. `PROTOCOL.md` — contrato WebSocket v2 completo
3. `Cards.md` — texto canônico de todas as cartas
4. `PlayerSkills.md` — habilidades de jogador
5. `internal/` — código Go (fonte de verdade técnica)

## Notas canônicas do vault (use para contexto)

- [[Canon]] — verdades imutáveis, regras fixas
- [[Glossário]] — terminologia canônica do jogo
- [[Turno e Ordem]] — fluxo de turno passo a passo
- [[Ignição, Ativação e Negação]] — sistema de cartas central
- [[Janelas de Reação]] — quem abre, quem responde
- [[Transporte e Envelope]] — protocolo WS
- [[Padrão de Resolvers]] — como implementar efeitos de carta
- [[TDD e Testes]] — disciplina de testes obrigatória

## Invariantes críticos (nunca violar)

- O **rei nunca é capturado** diretamente; tentativas são movimentos ilegais.
- Poderes podem causar xeque-mate, salvo texto da carta proibir.
- O **servidor é a fonte de verdade** para turno, mana, ignição e cooldown.
- **TDD obrigatório no backend**: red → green → refactor.
- Texto canônico de cartas/habilidades: preserve exatamente; nunca parafraseie.
- Push em `main` ou `dev` só com autorização explícita por escrito.

## Tecnologias

| Camada | Stack |
|--------|-------|
| Backend | Go, WebSocket, GORM |
| Banco | PostgreSQL + golang-migrate |
| Frontend | HTML, CSS, JS (sem framework) |
| Infra | Docker Compose |
| Testes | `go test ./...` (unitários + integração WS) |

## Estrutura de pacotes Go

| Pacote | Responsabilidade |
|--------|-----------------|
| `cmd/server` | Entrada HTTP/WebSocket |
| `internal/server` | Protocolo, handlers, salas |
| `internal/chess` | Motor de xadrez |
| `internal/gameplay` | Estado de partida (deck, mana, ignição, recarga) |
| `internal/match` | Runtime da partida, reações, cadeias Counter, hooks de engine |
| `internal/match/resolvers` | Resolvers de efeitos por tipo de carta |
| `internal/ranking` | ELO |

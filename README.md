# Power Chess

Jogo **multiplayer 1v1** de xadrez com **cartas de poder** e **habilidades de jogador**. O servidor autoritativo é em **Go** (WebSocket); o cliente atual é **HTML/CSS/JS** com HUD de desenvolvimento.

## Documentação

| Documento | Conteúdo |
|-----------|----------|
| [PROJECT.md](PROJECT.md) | Visão do produto, regras de jogo, diretrizes técnicas, **roadmap** |
| [PROTOCOL.md](PROTOCOL.md) | Contrato WebSocket v2 (mensagens, payloads, exemplos) |
| [Cards.md](Cards.md) | Texto das cartas iniciais (espelha o catálogo do servidor) |
| [PlayerSkills.md](PlayerSkills.md) | Habilidades de jogador selecionáveis |

## Requisitos

- Git  
- Go **1.26+** (ver `go.mod`)  
- Node.js 18+ (apenas para testes E2E com Playwright)  
- Docker + Docker Compose (opcional, para Postgres + servidor em container)

## Executar localmente

```bash
go mod tidy
go test ./...
go run ./cmd/server
```

Se você alterar **texto em inglês, custo, ignição, recarga, tipo ou ordem** das cartas em `internal/gameplay/cards.go`, regenere o arquivo estático usado pelo marquee / preview (`web/card-metadata.gen.js`):

```bash
go run ./cmd/export-card-metadata
```

(Equivalente: `go generate ./internal/gameplay`.)

- Health: `http://localhost:8080/healthz`  
- Métricas: `http://localhost:8080/metrics`  
- UI: `http://localhost:8080/`  
- WebSocket: `ws://localhost:8080/ws`

### Docker Compose

```bash
docker compose up --build
```

- Servidor em `:8080`  
- Postgres: host `localhost:5433` → container `:5432` (ver `docker-compose.yml`)

Variáveis úteis: `SERVER_ADDR`, `DATABASE_URL` (persistência de sala em Postgres; sem URL, o servidor segue em memória).

### Instalar Go

- **Linux (apt):** `sudo apt update && sudo apt install -y golang-go`  
- **Linux (snap):** `sudo snap install go --classic`  
- **macOS:** `brew install go`  
- **Windows:** instalador em [go.dev/dl](https://go.dev/dl/)

## Estrutura do repositório

| Pasta / arquivo | Função |
|-----------------|--------|
| `cmd/server` | Entrada HTTP/WebSocket |
| `internal/server` | Protocolo, handlers, salas |
| `internal/chess` | Motor de xadrez |
| `internal/gameplay` | Estado de partida (deck, mana, ignição, recarga, cartas) |
| `internal/match` | Efeitos, reações, cadeias Counter |
| `internal/ranking` | ELO |
| `web/` | Frontend estático (`app.js`, `index.html`, assets) |
| `tests/e2e/` | Testes Playwright |

## Protocolo WebSocket (resumo)

Envelope JSON: `id`, `type`, `payload`. Detalhes completos, códigos de erro e exemplos de `state_snapshot` estão em **[PROTOCOL.md](PROTOCOL.md)**.

Mensagens comuns (cliente → servidor): `ping`, `join_match`, `submit_move`, `activate_card`, `resolve_pending_effect`, `queue_reaction`, `resolve_reactions`, `leave_match`, `stay_in_room`, `request_rematch`.

Servidor → cliente: `hello`, `ack`, `error`, `state_snapshot`.

## Testes

```bash
go test ./...
```

### E2E (Playwright)

```bash
npm install
npx playwright install
npm run test:e2e
```

Com navegador visível: `npm run test:e2e:headed`

## Git e entregas

- Branch principal: **`main`**.  
- **Cada funcionalidade grande** deve ser **commitada** de forma coesa e **enviada** para `origin/main` (`git push origin main`) quando estiver pronta e com testes passando.  
- Antes de commitar: `go test ./...` (e `npm run test:e2e` se alterar UI/protocolo relevante).

## Telemetria e persistência

- **`GET /metrics`**: JSON em memória (`uptimeSeconds`, `requestsByType`, latências, erros).  
- **Postgres**: salas e estado para reconexão; `join_match` tenta restaurar antes de criar sala nova. Ver [PROJECT.md](PROJECT.md).

## Qualidade

- Regras de jogo e validação no **servidor**.  
- Novas funcionalidades e correções devem incluir **testes**.  
- Funções novas em Go: comentários no estilo **GoDoc**.

---

Para regras de negócio, timers, strikes, mana e roadmap de produto, use **[PROJECT.md](PROJECT.md)**.

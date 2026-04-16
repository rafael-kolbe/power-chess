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
- Node.js 18+ (apenas para tooling frontend opcional)  
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

O `Dockerfile` compila com **`-mod=vendor`**: as dependências Go vêm da pasta `vendor/` no repositório, então a imagem **não precisa baixar módulos** (evita falhas quando o DNS da rede do Docker não resolve `proxy.golang.org`).

Depois de alterar `go.mod` / `go.sum`, regenere o vendor e faça commit junto:

```bash
go mod tidy
go mod vendor
go test ./...
docker compose build
```

Subir stack:

```bash
docker compose up --build
```

Cada arranque grava **stdout/stderr** em ficheiros novos (append) em `logs/docker/server/` e `logs/docker/postgres/` (bind mount no repositório, visível no IDE; pasta `logs/` está no `.gitignore`). Nome do ficheiro: `YYYY-MM-DD:HH:MM:SS.log`. Por defeito mantêm-se só os **5 ficheiros mais recentes** em cada pasta (`POWER_CHESS_SESSION_LOG_KEEP` no `.env` / `docker-compose`; `0` desativa a limpeza).

- Servidor em `:8080`  
- Postgres: host `localhost:5433` → container `:5432` (ver `docker-compose.yml`)  
- Copia `.env.example` → `.env` e ajusta. O serviço `server` usa `env_file: .env`; `JWT_SECRET`, `SERVER_ADDR`, `ADMIN_DEBUG_MATCH`, etc. vêm dali. O `DATABASE_URL` **dentro do container** é fixo no `docker-compose.yml` para apontar ao serviço `postgres` (o `.env` continua a poder usar `localhost:5433` para correr o binário Go no host).

Variáveis úteis: `SERVER_ADDR`, `DATABASE_URL` (persistência de sala em Postgres; sem URL, o servidor segue em memória). Opcional: `ADMIN_DEBUG_MATCH` (`1` / `true` / `yes` / `on`) para aceitar o tipo WebSocket `debug_match_fixture` (fixtures de deck/mão/mana para testes) e **desativar persistência** de salas no Postgres (memória só). Em produção deve ficar desligado — ver [PROTOCOL.md](PROTOCOL.md). O cliente pode usar `web/match-test-config.js` (`MATCH_TEST_AUTO_APPLY` / `MATCH_TEST_AUTO_CONFIRM_MULLIGAN`) para auto-enviar o fixture e o mulligan.

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

## Protocolo WebSocket (resumo)

Envelope JSON: `id`, `type`, `payload`. Detalhes completos, códigos de erro e exemplos de `state_snapshot` estão em **[PROTOCOL.md](PROTOCOL.md)**.

Mensagens comuns (cliente → servidor): `ping`, `join_match`, `submit_move`, `ignite_card`, `resolve_pending_effect`, `queue_reaction`, `resolve_reactions`, `leave_match`, `stay_in_room`, `request_rematch`. O servidor pode enviar `activate_card` (efeito após ignição chegar a 0); ver `PROTOCOL.md`.

Servidor → cliente: `hello`, `ack`, `error`, `state_snapshot`.

## Testes

```bash
go test ./...
```

## Git e entregas

- Branch principal: **`main`**.  
- **Cada funcionalidade grande** deve ser **commitada** de forma coesa e **enviada** para `origin/main` (`git push origin main`) quando estiver pronta e com testes passando.  
- Antes de commitar: `go test ./...`.

## Telemetria e persistência

- **`GET /metrics`**: JSON em memória (`uptimeSeconds`, `requestsByType`, latências, erros).  
- **Postgres**: salas e estado para reconexão; `join_match` tenta restaurar antes de criar sala nova. Ver [PROJECT.md](PROJECT.md).

## Qualidade

- Regras de jogo e validação no **servidor**.  
- Novas funcionalidades e correções devem incluir **testes**.  
- Funções novas em Go: comentários no estilo **GoDoc**.

---

Para regras de negócio, timers, mana e roadmap de produto, use **[PROJECT.md](PROJECT.md)**.

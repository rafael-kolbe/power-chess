# Power Chess

Power Chess e um jogo multiplayer 1v1 de xadrez com cartas e habilidades especiais.

As regras de negocio do projeto estao em:
- `PROJECT.md`
- `Cards.md`
- `PlayerSkills.md`

## Requisitos
- Git
- Go (1.26+ recomendado)
- Docker + Docker Compose

## Instalar Golang

### Linux (Ubuntu/Debian)
```bash
sudo apt update
sudo apt install -y golang-go
go version
```

### Linux (Snap - alternativa)
```bash
sudo snap install go --classic
go version
```

### macOS (Homebrew)
```bash
brew update
brew install go
go version
```

### Windows
1. Baixe em [https://go.dev/dl/](https://go.dev/dl/)
2. Execute o instalador `.msi`
3. Verifique:
```powershell
go version
```

## Executar localmente (Go)
```bash
go mod tidy
go test ./...
go run ./cmd/server
```

Servidor:
- HTTP health: [http://localhost:8080/healthz](http://localhost:8080/healthz)
- HTTP metrics: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- WebSocket: `ws://localhost:8080/ws`

## Executar com Docker Compose
```bash
docker compose up --build
```

Servicos:
- `server`: backend Go + WebSocket em `:8080`
- `postgres`: PostgreSQL 16 (container `:5432`, host `:5433`)

Variaveis atuais:
- `SERVER_ADDR` (padrao no compose: `:8080`)
- `DATABASE_URL` (habilita persistencia de sala em Postgres)

## Protocolo WebSocket (base atual)

Formato envelope:
```json
{
  "id": "optional-correlation-id",
  "type": "message_type",
  "payload": {}
}
```

Mensagens cliente -> servidor:
- `ping`
- `join_match`
- `submit_move`
- `activate_card`
- `resolve_pending_effect`
- `queue_reaction`
- `resolve_reactions`

Mensagens servidor -> cliente:
- `hello`
- `ack`
- `error`
- `state_snapshot`

Documento completo do protocolo:
- `PROTOCOL.md`

HUD de desenvolvimento:
- `GET /` (servindo `web/index.html`)
- JavaScript cliente: `web/app.js`
- Controles rapidos no HUD:
  - `Resolve Pending Effect`
  - `Queue Reaction`
  - `Resolve Reactions`
  - Painel de status para `pendingCapture`, `reactionWindow` e `pendingEffects`

### `ack` (v2)
`ack` agora retorna payload padronizado:
```json
{
  "requestId": "same-as-envelope-id",
  "requestType": "submit_move",
  "status": "ok|queued",
  "code": "",
  "message": ""
}
```

### `error` (v2)
`error` retorna `code` + `message` com codigos padrao:
- `bad_request`
- `unknown_message_type`
- `join_required`
- `action_failed`
- `invalid_payload`
- `protocol_violation`

### `state_snapshot` (v2)
O snapshot enviado para HUD contem:
- `board` (matriz 8x8 com codigos como `wK`, `bP`, vazio `""`)
- `players` (mana, energized, handCount, cooldownCount, graveyardCount, strikes)
- `pendingEffects` (efeitos aguardando alvo)
- `reactionWindow` (trigger, tipos permitidos e tamanho da stack)
- `matchEnded`, `winner`, `endReason` (estado final da partida)

## Estrutura de codigo (atual)
- `cmd/server`: bootstrap do servidor HTTP/WebSocket
- `internal/server`: protocolo e handlers de transporte
- `internal/chess`: motor de xadrez (movimentos, check, checkmate, etc.)
- `internal/gameplay`: estado de partida (deck, mana, ignicao, cooldown, skills)
- `internal/match`: orquestracao de efeitos/cartas e reacoes
- `internal/ranking`: calculo ELO

## Roadmap imediato (Beta Server)
- [x] Base de protocolo websocket v2 (ack/error/snapshot)
- [x] Trigger windows de captura e chain Counter (Counterattack/Blockade)
- [x] Persistencia de estado de partida em PostgreSQL (reconexao e retomada)
- [x] Lock de concorrencia por sala e idempotencia por `requestId`
- [x] Testes websocket end-to-end multi-cliente
- [x] Monitoramento/telemetria basica do servidor

## Telemetria basica
- Endpoint `GET /metrics` expoe JSON com:
  - `uptimeSeconds`
  - `totalRequests`
  - `requestsByType`
  - `errorsByCode`
  - `handlerAvgLatencyMs`
  - `handlerLastLatencyMs`
- Metricas sao mantidas em memoria (sem Prometheus por enquanto).

## Persistencia minima (PostgreSQL)
- A sala e salva no Postgres apos mutacoes relevantes e em eventos de desconexao.
- `join_match` tenta restaurar sala persistida antes de criar uma nova em memoria.
- Persistencia e opcional: se `DATABASE_URL` nao estiver configurada, servidor segue em memoria.
- Snapshot persistido inclui estado de xadrez, estado de gameplay, janelas/reacoes pendentes e metadados de fim de partida.

## Cartas e habilidades nao implementadas (intencional)
- Funcionalidades de cartas e habilidades ainda pendentes permanecem como backlog.
- Prioridade atual: estabilizar servidor beta e fluxo de teste multiplayer.

## Qualidade e testes
- Sempre adicionar testes para cada funcionalidade nova/bugfix.
- Antes de commit:
```bash
go test ./...
```
- Documentacao de funcoes novas: obrigatoria (GoDoc).

## Regras de ativacao (resumo)
- `Power` e `Continuous`: ativacao no turno do jogador da vez.
- `Retribution`: ativacao em reaction windows de cadeia.
- `Counter`: ativacao em reaction windows de resposta a captura/condicao da carta.
- `Counterattack`: requer tentativa de captura por peca atacante buffada por `Power`; quando valida, captura a peca atacante e cancela a captura pendente.
- `Blockade`: responde diretamente a `Counterattack`, nega seu efeito, cancela a captura pendente e mantem a peca atacante na casa original.

## Regras de desconexao (beta)
- Se os dois jogadores desconectarem, a partida e cancelada sem vencedor.
- Se um jogador desconectar e nao reconectar em 60s, o outro vence por `disconnect_timeout`.
- O `state_snapshot` expone `matchEnded`, `winner` e `endReason` para HUD e automacoes.
- Slot de ignicao ocupado bloqueia novas ativacoes, exceto `Save It For Later` (comportamento especial de remover a carta ignited sem resolver efeito e reutilizar o slot).
- Cartas com `Ignition: 0` resolvem imediatamente e permitem sequencias no mesmo turno se houver mana e slot livre.
- Habilidades de jogador: somente no proprio turno, consomem turno e nao podem ser negadas.

## UX do HUD (beta)
- Coordenadas nas bordas do tabuleiro (arquivos e ranks), com opcao de coordenadas discretas dentro de cada casa.
- Barras de mana (azul) e mana energizada (vermelho) com texto tipo `3/10` e `15/20`.
- Drag-and-drop de pecas com highlight de movimentos pseudo-legais.
- Toggle `On/Off` no HUD:
  - `On`: jogador pode responder normalmente em qualquer reaction window.
  - `Off`: HUD auto-envia skip da resposta para acelerar testes (Counter, resposta a Power, etc.).
- Timeout de resposta no backend: 10s para janelas de reacao (captura/cartas). Ao expirar, a pilha e resolvida automaticamente.
- Painel com relogio de turno (cliente), mana, mana energizada e strikes para ambos jogadores.

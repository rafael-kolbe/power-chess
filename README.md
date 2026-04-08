# Power Chess

Power Chess e um jogo multiplayer de xadrez com poderes especiais por cartas.

As regras completas do projeto estao em `PROJECT.md`.

## Requisitos
- Git
- Go (recomendado: versao estavel mais recente)
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
1. Baixe o instalador oficial em [https://go.dev/dl/](https://go.dev/dl/)
2. Execute o instalador `.msi`
3. Abra um novo terminal e valide:
```powershell
go version
```

## Clonar e entrar no projeto
```bash
git clone <URL_DO_REPOSITORIO>
cd power-chess
```

## Estrutura esperada (inicio)
- `PROJECT.md`: regras e contexto de negocio do jogo
- `.cursor/rules/`: padroes persistentes para o Cursor
- `README.md`: setup e execucao

## Rodar projeto localmente

Como o projeto esta no inicio, siga esta ordem:

1. **Backend (Go + WebSocket)**
   - Instalar dependencias Go:
   ```bash
   go mod tidy
   ```
   - Rodar local:
   ```bash
   go run ./...
   ```

2. **Docker Compose (quando os servicos existirem)**
   ```bash
   docker compose up --build
   ```

3. **Frontend (HTML/CSS/JS)**
   - Abra o arquivo HTML principal no navegador, ou sirva com um servidor local:
   ```bash
   python3 -m http.server 8080
   ```
   - Acesse: [http://localhost:8080](http://localhost:8080)

## Proximos passos recomendados
- Criar `docker-compose.yml` com servicos de backend e frontend.
- Definir estrutura inicial do backend em Go (`cmd/`, `internal/`).
- Implementar protocolo WebSocket para:
  - estado do tabuleiro,
  - turnos e strikes,
  - mana e poderes,
  - matchmaking 1v1.

# Padrão de Resolvers

> Status: `validated` | Fonte: `PROJECT.md` (seção "Arquitetura sugerida para poderes")

## Regra geral

Cada carta nova deve ter um **resolver dedicado**. Nunca adicionar lógica de carta específica no fluxo central (`engine.go`, `SubmitMove`, `applyMoveCore`).

## Padrão de implementação (6 regras)

### 1. Resolver dedicado por carta
- Criar `internal/match/resolvers/<tipo>/<slug>.go` (ex.: `power/knight_touch.go`).
- Registrar no `DefaultResolvers()` em `resolvers.go` sem alterar o pipeline central.

### 2. Estado de efeito genérico no runtime
- Modelar efeitos temporários como estado estruturado e serializável (ex.: `MovementGrant`).
- Persistir no snapshot do engine (`persistence.go`) para suportar reconexão/restauração.

### 3. Capacidades por composição, não substituição
- Efeitos devem **adicionar capacidades** à peça/jogador (ex.: novo padrão de movimento).
- Não remover comportamento nativo, salvo quando o texto da carta exigir.

### 4. Pontos únicos de aplicação
- Fluxo central só consulta serviços/estados genéricos.
- Regras específicas ficam encapsuladas em resolver + tipos de estado do efeito.

### 5. Ciclo de vida explícito
Definir claramente:
- Criação do efeito
- Manutenção (ex.: acompanhar posição da peça)
- Expiração por turno/condição
- Limpeza quando alvo deixa de ser válido

### 6. TDD obrigatório por carta
- Antes da implementação: testes RED cobrindo ativação, uso do efeito, interação com regras-base e expiração.
- Depois: GREEN mínimo + REFACTOR mantendo cobertura.
- Branch padrão: `feature/<card-id>` (ex.: `feature/knight-touch`).

## Interface do resolver

```go
// Resolver define o contrato para implementar o efeito de uma carta.
type Resolver interface {
    Resolve(ctx ResolveContext) error
}
```

Ver `internal/match/resolvers/interface.go` para a assinatura exata.

## Links

- [[Estrutura de Pacotes]] — onde cada resolver vive
- [[TDD e Testes]] — ciclo red/green/refactor
- [[Catálogo de Cartas]] — texto canônico que guia a implementação

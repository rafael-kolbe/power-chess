package match

import (
	"errors"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// PersistedEngineState is a JSON-serializable snapshot of engine runtime state.
type PersistedEngineState struct {
	Chess          chess.Game                         `json:"chess"`
	Match          gameplay.MatchState                `json:"match"`
	MoveBuffTarget map[gameplay.PlayerID]*chess.Pos   `json:"moveBuffTarget"`
	MoveBuffKind   map[gameplay.PlayerID]MoveBuffKind `json:"moveBuffKind"`
	ExtraMoveLeft  map[gameplay.PlayerID]int          `json:"extraMoveLeft"`
	MovesThisTurn  map[gameplay.PlayerID]int          `json:"movesThisTurn"`
	PendingEffects []PersistedPendingEffect           `json:"pendingEffects"`
	ReactionWindow *ReactionWindow                    `json:"reactionWindow,omitempty"`
	ReactionStack  []PersistedReactionAction          `json:"reactionStack"`
	PendingMove    *PendingMoveAction                 `json:"pendingMove,omitempty"`
}

// PersistedPendingEffect stores pending effect metadata without function pointers.
type PersistedPendingEffect struct {
	Owner  gameplay.PlayerID `json:"owner"`
	CardID gameplay.CardID   `json:"cardId"`
}

// PersistedReactionAction stores stack items in serializable form.
type PersistedReactionAction struct {
	Owner  gameplay.PlayerID     `json:"owner"`
	Card   gameplay.CardInstance `json:"card"`
	Target EffectTarget          `json:"target"`
}

// ExportState serializes the runtime engine state to a persistence-safe shape.
func (e *Engine) ExportState() PersistedEngineState {
	out := PersistedEngineState{
		Chess:          *e.Chess.Clone(),
		Match:          *e.State,
		MoveBuffTarget: map[gameplay.PlayerID]*chess.Pos{},
		MoveBuffKind:   map[gameplay.PlayerID]MoveBuffKind{},
		ExtraMoveLeft:  map[gameplay.PlayerID]int{},
		MovesThisTurn:  map[gameplay.PlayerID]int{},
		ReactionWindow: nil,
		ReactionStack:  make([]PersistedReactionAction, 0, len(e.reactionStack)),
		PendingMove:    nil,
	}
	for pid, pos := range e.moveBuffTarget {
		if pos == nil {
			out.MoveBuffTarget[pid] = nil
			continue
		}
		cp := *pos
		out.MoveBuffTarget[pid] = &cp
	}
	for pid, kind := range e.moveBuffKind {
		out.MoveBuffKind[pid] = kind
	}
	for pid, n := range e.extraMoveLeft {
		out.ExtraMoveLeft[pid] = n
	}
	for pid, n := range e.movesThisTurn {
		out.MovesThisTurn[pid] = n
	}
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		for _, pe := range e.pendingEffects[pid] {
			out.PendingEffects = append(out.PendingEffects, PersistedPendingEffect{
				Owner:  pe.Owner,
				CardID: pe.CardID,
			})
		}
	}
	if e.ReactionWindow != nil {
		rw := *e.ReactionWindow
		rw.EligibleTypes = append([]gameplay.CardType(nil), rw.EligibleTypes...)
		out.ReactionWindow = &rw
	}
	for _, ra := range e.reactionStack {
		out.ReactionStack = append(out.ReactionStack, PersistedReactionAction{
			Owner:  ra.Owner,
			Card:   ra.Card,
			Target: ra.Target,
		})
	}
	if e.pendingMove != nil {
		pm := *e.pendingMove
		out.PendingMove = &pm
	}
	return out
}

// NewEngineFromState recreates a runtime engine from a persisted snapshot.
func NewEngineFromState(snapshot PersistedEngineState) (*Engine, error) {
	matchState := snapshot.Match
	chessState := snapshot.Chess
	e := NewEngine(&matchState, &chessState)
	e.moveBuffTarget = snapshot.MoveBuffTarget
	e.moveBuffKind = snapshot.MoveBuffKind
	e.extraMoveLeft = snapshot.ExtraMoveLeft
	e.movesThisTurn = snapshot.MovesThisTurn
	if e.moveBuffTarget == nil {
		e.moveBuffTarget = map[gameplay.PlayerID]*chess.Pos{}
	}
	if e.moveBuffKind == nil {
		e.moveBuffKind = map[gameplay.PlayerID]MoveBuffKind{}
	}
	if e.extraMoveLeft == nil {
		e.extraMoveLeft = map[gameplay.PlayerID]int{}
	}
	if e.movesThisTurn == nil {
		e.movesThisTurn = map[gameplay.PlayerID]int{}
	}
	if snapshot.ReactionWindow != nil {
		rw := *snapshot.ReactionWindow
		rw.EligibleTypes = append([]gameplay.CardType(nil), rw.EligibleTypes...)
		e.ReactionWindow = &rw
	}
	if snapshot.PendingMove != nil {
		pm := *snapshot.PendingMove
		e.pendingMove = &pm
	}
	for _, pe := range snapshot.PendingEffects {
		resolver, ok := e.resolvers[pe.CardID]
		if !ok {
			return nil, errors.New("missing resolver for persisted pending effect")
		}
		e.pendingEffects[pe.Owner] = append(e.pendingEffects[pe.Owner], PendingEffect{
			Owner:    pe.Owner,
			CardID:   pe.CardID,
			Resolver: resolver,
		})
	}
	for _, ra := range snapshot.ReactionStack {
		resolver, ok := e.resolvers[ra.Card.CardID]
		if !ok {
			return nil, errors.New("missing resolver for persisted reaction stack")
		}
		e.reactionStack = append(e.reactionStack, ReactionAction{
			Owner:    ra.Owner,
			Card:     ra.Card,
			Target:   ra.Target,
			Resolver: resolver,
		})
	}
	return e, nil
}

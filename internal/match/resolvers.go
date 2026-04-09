package match

import (
	"errors"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// DefaultResolvers registers currently implemented card effects.
func DefaultResolvers() map[gameplay.CardID]EffectResolver {
	return map[gameplay.CardID]EffectResolver{
		CardKnightTouch: knightTouchResolver{},
		CardRookTouch:   rookTouchResolver{},
		CardBishopTouch: bishopTouchResolver{},
		CardDoubleTurn:  doubleTurnResolver{},
		CardStopRightThere: stopRightThereResolver{},
		CardExtinguish:     extinguishResolver{},
		CardCounterattack:  counterattackResolver{},
		CardBlockade:       blockadeResolver{},
	}
}

type knightTouchResolver struct{}

func (knightTouchResolver) RequiresTarget() bool { return true }

func (knightTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	if target.PiecePos == nil {
		return errors.New("knight touch requires piece target")
	}
	p := e.Chess.PieceAt(*target.PiecePos)
	if p.IsEmpty() {
		return errors.New("target piece is empty")
	}
	if p.Color != toColor(owner) {
		return errors.New("target must be an owned piece")
	}
	if p.Type == chess.King || p.Type == chess.Knight {
		return errors.New("cannot apply knight touch to king or knight")
	}
	e.SetMoveBuffTarget(owner, MoveBuffKnight, *target.PiecePos)
	return nil
}

type rookTouchResolver struct{}

func (rookTouchResolver) RequiresTarget() bool { return true }

func (rookTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	if target.PiecePos == nil {
		return errors.New("rook touch requires piece target")
	}
	p := e.Chess.PieceAt(*target.PiecePos)
	if p.IsEmpty() {
		return errors.New("target piece is empty")
	}
	if p.Color != toColor(owner) {
		return errors.New("target must be an owned piece")
	}
	if p.Type == chess.King || p.Type == chess.Rook {
		return errors.New("cannot apply rook touch to king or rook")
	}
	e.SetMoveBuffTarget(owner, MoveBuffRook, *target.PiecePos)
	return nil
}

type bishopTouchResolver struct{}

func (bishopTouchResolver) RequiresTarget() bool { return true }

func (bishopTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	if target.PiecePos == nil {
		return errors.New("bishop touch requires piece target")
	}
	p := e.Chess.PieceAt(*target.PiecePos)
	if p.IsEmpty() {
		return errors.New("target piece is empty")
	}
	if p.Color != toColor(owner) {
		return errors.New("target must be an owned piece")
	}
	if p.Type == chess.King || p.Type == chess.Bishop {
		return errors.New("cannot apply bishop touch to king or bishop")
	}
	e.SetMoveBuffTarget(owner, MoveBuffBishop, *target.PiecePos)
	return nil
}

type doubleTurnResolver struct{}

func (doubleTurnResolver) RequiresTarget() bool { return false }

func (doubleTurnResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	_ = target
	e.extraMoveLeft[owner]++
	return nil
}

type stopRightThereResolver struct{}

func (stopRightThereResolver) RequiresTarget() bool { return false }

func (stopRightThereResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	_ = target
	if !e.State.IgnitionSlot.Occupied {
		return errors.New("no ignited card to negate")
	}
	if e.State.IgnitionSlot.ActivationOwner == owner {
		return errors.New("cannot negate your own ignited card")
	}
	return e.State.ResolveIgnition(false)
}

type extinguishResolver struct{}

func (extinguishResolver) RequiresTarget() bool { return false }

func (extinguishResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	_ = target
	if !e.State.IgnitionSlot.Occupied {
		return errors.New("no ignited card to extinguish")
	}
	if e.State.IgnitionSlot.ActivationOwner == owner {
		return errors.New("cannot extinguish your own ignited card")
	}
	return e.State.ResolveIgnition(false)
}

type counterattackResolver struct{}

func (counterattackResolver) RequiresTarget() bool { return false }

func (counterattackResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	_ = owner
	_ = target
	if !e.IsPendingCaptureFromBuffedAttacker() {
		return errors.New("counterattack requires capture attempt by power-buffed attacker")
	}
	return e.CancelPendingCaptureAndCaptureAttacker()
}

type blockadeResolver struct{}

func (blockadeResolver) RequiresTarget() bool { return false }

func (blockadeResolver) Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error {
	_ = e
	_ = owner
	_ = target
	return nil
}


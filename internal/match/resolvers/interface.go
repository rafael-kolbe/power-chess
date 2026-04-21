// Package resolvers defines the shared interface contract between the match engine
// and card effect implementations, avoiding circular imports between the engine
// package and the per-type resolver subpackages.
package resolvers

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// MovementGrantKind describes a movement-pattern extension granted by a resolved card effect.
type MovementGrantKind string

const (
	// MovementGrantKnightPattern grants additional knight-pattern movement.
	MovementGrantKnightPattern MovementGrantKind = "knight_pattern"
	// MovementGrantBishopPattern grants additional bishop-line movement.
	MovementGrantBishopPattern MovementGrantKind = "bishop_pattern"
	// MovementGrantRookPattern grants additional rook-line movement.
	MovementGrantRookPattern MovementGrantKind = "rook_pattern"
)

// EffectTarget holds optional targeting context supplied with a card activation or reaction.
type EffectTarget struct {
	PiecePos    *chess.Pos
	TargetPos   *chess.Pos
	TargetCard  *gameplay.CardID
	TargetRow   *int
	TargetCol   *int
	TargetIndex *int
}

// ResolverEngine is the subset of Engine operations that card resolvers are permitted to call.
// Engine implements this interface; resolver subpackages depend only on this interface, not on
// the concrete Engine type, which prevents circular import dependencies.
type ResolverEngine interface {
	// ConsumeIgnitionTargets pops and returns the locked piece positions for cardID owned by owner.
	ConsumeIgnitionTargets(owner gameplay.PlayerID, cardID gameplay.CardID) []chess.Pos
	// PieceAt returns the chess piece currently occupying pos.
	PieceAt(pos chess.Pos) chess.Piece
	// OwnerColor returns the chess color associated with owner.
	OwnerColor(owner gameplay.PlayerID) chess.Color
	// AddMovementGrant registers a movement-pattern grant for a piece.
	AddMovementGrant(owner gameplay.PlayerID, cardID gameplay.CardID, target chess.Pos, kind MovementGrantKind, durationTurns int)
	// GrantManaFromCardEffect awards bonus mana to pid on successful effect resolution.
	GrantManaFromCardEffect(pid gameplay.PlayerID, amount int)
	// IncrementExtraMoves grants one additional chess move this turn to pid.
	IncrementExtraMoves(pid gameplay.PlayerID)
	// NegateOpponentIgnition resolves opponentPID's ignition slot as a failure (clears slot).
	NegateOpponentIgnition(opponentPID gameplay.PlayerID) error
	// MarkOpponentCardEffectNegated sets the opponent's current ignition card as negated;
	// the card stays in ignition until its normal burn completes; activations resolve as failure.
	// Works regardless of whether the card was just ignited (reaction) or was already burning
	// (initiator turn) — any resolver can call this to apply a negate effect.
	MarkOpponentCardEffectNegated(opponentPID gameplay.PlayerID) error
	// BurnManaFromOpponent drains amount mana from opponentPID's mana pools; regular mana is
	// drained first, then the energized mana pool absorbs any remainder.
	BurnManaFromOpponent(opponentPID gameplay.PlayerID, amount int)
	// IgnitionCardCost returns the ManaCost of the card currently in pid's ignition slot,
	// or 0 if the slot is unoccupied.
	IgnitionCardCost(pid gameplay.PlayerID) int
}

// EffectResolver is the execution contract for card effects.
type EffectResolver interface {
	// RequiresTarget reports whether this resolver waits for resolve_pending_effect target input.
	RequiresTarget() bool
	// Apply executes the card effect via the engine interface.
	Apply(e ResolverEngine, owner gameplay.PlayerID, target EffectTarget) error
}

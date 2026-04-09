package match

import (
	"errors"
	"fmt"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

const (
	CardKnightTouch gameplay.CardID = "knight-touch"
	CardRookTouch   gameplay.CardID = "rook-touch"
	CardBishopTouch gameplay.CardID = "bishop-touch"
	CardDoubleTurn  gameplay.CardID = "double-turn"
	CardStopRightThere gameplay.CardID = "stop-right-there"
	CardExtinguish gameplay.CardID = "extinguish"
	CardCounterattack gameplay.CardID = "counterattack"
	CardBlockade gameplay.CardID = "blockade"
)

type Engine struct {
	Chess *chess.Game
	State *gameplay.MatchState

	moveBuffTarget map[gameplay.PlayerID]*chess.Pos
	moveBuffKind   map[gameplay.PlayerID]MoveBuffKind
	extraMoveLeft    map[gameplay.PlayerID]int
	movesThisTurn    map[gameplay.PlayerID]int
	pendingEffects   map[gameplay.PlayerID][]PendingEffect
	resolvers        map[gameplay.CardID]EffectResolver
	ReactionWindow   *ReactionWindow
	reactionStack    []ReactionAction
	pendingMove      *PendingMoveAction
}

// NewEngine wires chess state, gameplay state and card resolvers into a single match runtime.
func NewEngine(state *gameplay.MatchState, board *chess.Game) *Engine {
	return &Engine{
		State:            state,
		Chess:            board,
		moveBuffTarget:   map[gameplay.PlayerID]*chess.Pos{},
		moveBuffKind:     map[gameplay.PlayerID]MoveBuffKind{},
		extraMoveLeft:    map[gameplay.PlayerID]int{},
		movesThisTurn:    map[gameplay.PlayerID]int{},
		pendingEffects:   map[gameplay.PlayerID][]PendingEffect{},
		resolvers:        DefaultResolvers(),
		reactionStack:    []ReactionAction{},
		pendingMove:      nil,
	}
}

// StartTurn advances gameplay resources and applies any resolved ignition effects.
func (e *Engine) StartTurn(pid gameplay.PlayerID) error {
	if err := e.State.StartTurn(pid); err != nil {
		return err
	}
	e.movesThisTurn[pid] = 0
	return e.processResolvedIgnitions()
}

// EndTurn clears turn-scoped buffs and advances the active player.
func (e *Engine) EndTurn(pid gameplay.PlayerID) error {
	delete(e.moveBuffTarget, pid)
	delete(e.moveBuffKind, pid)
	e.extraMoveLeft[pid] = 0
	e.movesThisTurn[pid] = 0
	return e.State.EndTurn(pid)
}

// SetMoveBuffTarget stores a one-turn movement buff target for the given player.
func (e *Engine) SetMoveBuffTarget(pid gameplay.PlayerID, kind MoveBuffKind, pos chess.Pos) {
	cp := pos
	e.moveBuffTarget[pid] = &cp
	e.moveBuffKind[pid] = kind
}

// ActivateCard validates reaction constraints (if any) and delegates activation to gameplay state.
func (e *Engine) ActivateCard(pid gameplay.PlayerID, handIndex int) error {
	p := e.State.Players[pid]
	if handIndex < 0 || handIndex >= len(p.Hand) {
		return errors.New("invalid hand index")
	}
	def, ok := gameplay.CardDefinitionByID(p.Hand[handIndex].CardID)
	if !ok {
		return errors.New("unknown card definition")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		allowed := false
		for _, t := range e.ReactionWindow.EligibleTypes {
			if def.Type == t {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("card type not allowed in current reaction window")
		}
	} else {
		if def.Type != gameplay.CardTypePower && def.Type != gameplay.CardTypeContinuous && def.ID != "save-it-for-later" {
			return errors.New("only Power and Continuous cards can be activated in normal turn flow")
		}
	}
	if err := e.State.ActivateCard(pid, handIndex); err != nil {
		return err
	}
	return e.processResolvedIgnitions()
}

// ResolvePendingEffect applies the next queued target-dependent effect for the player.
func (e *Engine) ResolvePendingEffect(pid gameplay.PlayerID, target EffectTarget) error {
	queue := e.pendingEffects[pid]
	if len(queue) == 0 {
		return errors.New("no pending effect for player")
	}
	pe := queue[0]
	e.pendingEffects[pid] = queue[1:]
	return pe.Resolver.Apply(e, pe.Owner, target)
}

// SubmitMove executes a move, including temporary movement buffs and extra-move rules.
func (e *Engine) SubmitMove(pid gameplay.PlayerID, m chess.Move) error {
	e.reconcileTurnState()
	if e.State.CurrentTurn != pid {
		return errors.New("not current player turn")
	}
	color := toColor(pid)
	if e.Chess.Turn != color {
		return errors.New("chess turn out of sync with match turn")
	}
	if e.pendingMove != nil {
		return errors.New("cannot submit another move while capture reaction window is pending")
	}

	if e.isCaptureAttempt(pid, m) {
		e.pendingMove = &PendingMoveAction{PlayerID: pid, Move: m}
		e.OpenReactionWindow("capture_attempt", pid, []gameplay.CardType{gameplay.CardTypeCounter})
		return nil
	}
	return e.applyMoveCore(pid, m)
}

// reconcileTurnState keeps gameplay turn metadata aligned with authoritative chess turn.
func (e *Engine) reconcileTurnState() {
	expected := playerFromColor(e.Chess.Turn)
	if e.State.CurrentTurn != expected {
		e.State.CurrentTurn = expected
	}
}

func (e *Engine) applyMoveWithBuffIfAny(board *chess.Game, pid gameplay.PlayerID, m chess.Move) error {
	target := e.moveBuffTarget[pid]
	if target == nil || *target != m.From {
		return board.ApplyMove(m)
	}
	switch e.moveBuffKind[pid] {
	case MoveBuffKnight:
		if !isKnightDelta(m.From, m.To) {
			return fmt.Errorf("knight buff only allows knight pattern from buffed piece")
		}
	case MoveBuffRook:
		if !isRookLikeDelta(m.From, m.To) {
			return fmt.Errorf("rook buff only allows rook-like movement from buffed piece")
		}
	case MoveBuffBishop:
		if !isBishopLikeDelta(m.From, m.To) {
			return fmt.Errorf("bishop buff only allows bishop-like movement from buffed piece")
		}
	default:
		return board.ApplyMove(m)
	}
	return board.ApplyPseudoLegalMove(m)
}

func (e *Engine) handleResolvedEffect(ev *gameplay.ResolvedIgnitionEvent) error {
	if !ev.Success {
		return nil
	}
	resolver, ok := e.resolvers[ev.Card.CardID]
	if !ok {
		return nil
	}
	if resolver.RequiresTarget() {
		e.pendingEffects[ev.Owner] = append(e.pendingEffects[ev.Owner], PendingEffect{
			Owner:    ev.Owner,
			CardID:   ev.Card.CardID,
			Resolver: resolver,
		})
		return nil
	}
	return resolver.Apply(e, ev.Owner, EffectTarget{})
}

func (e *Engine) processResolvedIgnitions() error {
	for _, ev := range e.State.PopResolvedIgnitions() {
		evCopy := ev
		if err := e.handleResolvedEffect(&evCopy); err != nil {
			return err
		}
	}
	return nil
}

// ActivatePlayerSkill executes the selected player skill on the current turn and consumes that turn.
func (e *Engine) ActivatePlayerSkill(pid gameplay.PlayerID) error {
	color := toColor(pid)
	if e.Chess.Turn != color {
		return errors.New("chess turn out of sync with match turn")
	}
	if err := e.State.ActivateSpecialAbility(pid); err != nil {
		return err
	}
	e.Chess.Turn = color.Opponent()
	return nil
}

// applyMoveCore applies a validated move without opening capture trigger windows.
// It is used by normal non-capture flow and pending-move finalization.
func (e *Engine) applyMoveCore(pid gameplay.PlayerID, m chess.Move) error {
	color := toColor(pid)
	isPowerSecondMove := e.movesThisTurn[pid] >= 1 && e.extraMoveLeft[pid] > 0
	if err := e.applyMoveWithBuffIfAny(e.Chess, pid, m); err != nil {
		return err
	}
	e.movesThisTurn[pid]++
	keepTurn := false
	if isPowerSecondMove {
		e.extraMoveLeft[pid]--
	} else if e.movesThisTurn[pid] == 1 && e.extraMoveLeft[pid] > 0 {
		e.Chess.Turn = color
		keepTurn = true
	}
	if !keepTurn {
		if err := e.State.EndTurn(pid); err != nil {
			return err
		}
	}
	delete(e.moveBuffTarget, pid)
	delete(e.moveBuffKind, pid)
	return nil
}

// PendingMoveAction represents a not-yet-applied move waiting for reaction window resolution.
type PendingMoveAction struct {
	PlayerID gameplay.PlayerID
	Move     chess.Move
}

// isCaptureAttempt checks whether a move targets an enemy-occupied destination square.
func (e *Engine) isCaptureAttempt(pid gameplay.PlayerID, m chess.Move) bool {
	fromPiece := e.Chess.PieceAt(m.From)
	toPiece := e.Chess.PieceAt(m.To)
	if fromPiece.IsEmpty() || fromPiece.Color != toColor(pid) {
		return false
	}
	if !toPiece.IsEmpty() && toPiece.Color != fromPiece.Color {
		return true
	}
	// En passant capture is also a capture attempt even when destination is empty.
	return fromPiece.Type == chess.Pawn &&
		m.From.Col != m.To.Col &&
		e.Chess.EnPassant.Valid &&
		m.To == e.Chess.EnPassant.Target
}

// PendingMove returns the current capture-pending move if any.
func (e *Engine) PendingMove() (PendingMoveAction, bool) {
	if e.pendingMove == nil {
		return PendingMoveAction{}, false
	}
	return *e.pendingMove, true
}

// IsPendingCaptureFromBuffedAttacker reports whether pending capture is initiated by a power-buffed piece.
func (e *Engine) IsPendingCaptureFromBuffedAttacker() bool {
	if e.pendingMove == nil || e.ReactionWindow == nil || e.ReactionWindow.Trigger != "capture_attempt" {
		return false
	}
	pm := *e.pendingMove
	target := e.moveBuffTarget[pm.PlayerID]
	if target == nil {
		return false
	}
	return *target == pm.Move.From
}

// CancelPendingCaptureAndCaptureAttacker cancels pending capture and removes attacking piece from board.
func (e *Engine) CancelPendingCaptureAndCaptureAttacker() error {
	if e.pendingMove == nil {
		return errors.New("no pending capture to counter")
	}
	pm := *e.pendingMove
	attacker := e.Chess.PieceAt(pm.Move.From)
	if attacker.IsEmpty() {
		return errors.New("attacker piece is missing")
	}
	e.Chess.SetPiece(pm.Move.From, chess.Piece{})
	delete(e.moveBuffTarget, pm.PlayerID)
	delete(e.moveBuffKind, pm.PlayerID)
	e.pendingMove = nil
	return nil
}

func isKnightDelta(from, to chess.Pos) bool {
	dr := abs(from.Row - to.Row)
	dc := abs(from.Col - to.Col)
	return (dr == 1 && dc == 2) || (dr == 2 && dc == 1)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func toColor(pid gameplay.PlayerID) chess.Color {
	if pid == gameplay.PlayerA {
		return chess.White
	}
	return chess.Black
}

func playerFromColor(c chess.Color) gameplay.PlayerID {
	if c == chess.White {
		return gameplay.PlayerA
	}
	return gameplay.PlayerB
}

type EffectTarget struct {
	PiecePos    *chess.Pos
	TargetPos   *chess.Pos
	TargetCard  *gameplay.CardID
	TargetRow   *int
	TargetCol   *int
	TargetIndex *int
}

// PendingEffect represents a resolved ignition effect waiting for player target input.
type PendingEffect struct {
	Owner    gameplay.PlayerID
	CardID   gameplay.CardID
	Resolver EffectResolver
}

// EffectResolver is the execution contract for card effects.
type EffectResolver interface {
	RequiresTarget() bool
	Apply(e *Engine, owner gameplay.PlayerID, target EffectTarget) error
}

type MoveBuffKind string

const (
	MoveBuffKnight MoveBuffKind = "knight"
	MoveBuffRook   MoveBuffKind = "rook"
	MoveBuffBishop MoveBuffKind = "bishop"
)

func isRookLikeDelta(from, to chess.Pos) bool {
	return from.Row == to.Row || from.Col == to.Col
}

func isBishopLikeDelta(from, to chess.Pos) bool {
	dr := abs(from.Row - to.Row)
	dc := abs(from.Col - to.Col)
	return dr == dc
}

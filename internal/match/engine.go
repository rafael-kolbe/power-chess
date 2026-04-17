package match

import (
	"errors"
	"fmt"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

var errMatchNotStarted = errors.New("match not started")
var errMulliganInProgress = errors.New("mulligan in progress")

// errIfOpeningBlocksGameplay returns an error while the mulligan phase is active or before the first turn has begun.
func (e *Engine) errIfOpeningBlocksGameplay() error {
	if e.State.MulliganPhaseActive {
		return errMulliganInProgress
	}
	if !e.State.Started {
		return errMatchNotStarted
	}
	return nil
}

const (
	CardKnightTouch    gameplay.CardID = "knight-touch"
	CardRookTouch      gameplay.CardID = "rook-touch"
	CardBishopTouch    gameplay.CardID = "bishop-touch"
	CardEnergyGain     gameplay.CardID = "energy-gain"
	CardDoubleTurn     gameplay.CardID = "double-turn"
	CardStopRightThere gameplay.CardID = "stop-right-there"
	CardExtinguish     gameplay.CardID = "extinguish"
	CardCounterattack  gameplay.CardID = "counterattack"
	CardBlockade       gameplay.CardID = "blockade"
)

type Engine struct {
	Chess *chess.Game
	State *gameplay.MatchState

	pendingEffects map[gameplay.PlayerID][]PendingEffect
	resolvers      map[gameplay.CardID]EffectResolver
	ReactionWindow *ReactionWindow
	reactions      *ReactionRuntime
	pendingMove    *PendingMoveAction
	// pendingActivationFX holds server→client activate_card events (effect step after ignition reaches 0).
	pendingActivationFX []ActivationFXEvent
	// movementGrants stores active movement modifiers granted by resolved card effects.
	movementGrants []MovementGrant
	// ignitionTargets stores locked piece targets for the currently ignited card owner.
	ignitionTargets map[gameplay.PlayerID][]chess.Pos
	// ignitionTargetCard stores which card owns ignitionTargets for each seat.
	ignitionTargetCard map[gameplay.PlayerID]gameplay.CardID
	// extraMovesRemaining tracks bonus moves granted by Double Turn (per seat).
	extraMovesRemaining map[gameplay.PlayerID]int
	// doubleTurnEffectTurnsLeft tracks the visual highlight duration for Double Turn (per seat),
	// independent of extraMovesRemaining. Decremented at end of the owner's turn so the
	// highlight persists for the full turn even after both moves have been made.
	doubleTurnEffectTurnsLeft map[gameplay.PlayerID]int
	// disruptionSameTurnGlowPID is set after a Disruption card is placed in ignition on the owner's
	// turn; the room broadcasts that state, then calls FinishDisruptionSameTurnResolveIfPending so
	// clients can run activation glow on the ignition slot before the card moves to cooldown.
	disruptionSameTurnGlowPID gameplay.PlayerID
}

// ActivationFXEvent is one ignition resolution for client animations (glow + fly to cooldown).
type ActivationFXEvent struct {
	Owner   gameplay.PlayerID
	CardID  gameplay.CardID
	Success bool
}

// PullActivationFXEvents returns and clears pending activation broadcast events.
func (e *Engine) PullActivationFXEvents() []ActivationFXEvent {
	out := e.pendingActivationFX
	e.pendingActivationFX = nil
	return out
}

// appendActivationFX records one server→client activate_card animation (effect step: success/fail, then cooldown).
func (e *Engine) appendActivationFX(owner gameplay.PlayerID, id gameplay.CardID, success bool) {
	e.pendingActivationFX = append(e.pendingActivationFX, ActivationFXEvent{
		Owner:   owner,
		CardID:  id,
		Success: success,
	})
}

// CloneMovementGrants returns a shallow copy of active movement grants for snapshots and persistence.
func (e *Engine) CloneMovementGrants() []MovementGrant {
	return append([]MovementGrant(nil), e.movementGrants...)
}

// NewEngine wires chess state, gameplay state and card resolvers into a single match runtime.
func NewEngine(state *gameplay.MatchState, board *chess.Game) *Engine {
	return &Engine{
		State:          state,
		Chess:          board,
		pendingEffects: map[gameplay.PlayerID][]PendingEffect{},
		resolvers:      DefaultResolvers(),
		reactions:      NewReactionRuntime(),
		pendingMove:    nil,
		ignitionTargets: map[gameplay.PlayerID][]chess.Pos{
			gameplay.PlayerA: {},
			gameplay.PlayerB: {},
		},
		ignitionTargetCard:        map[gameplay.PlayerID]gameplay.CardID{},
		extraMovesRemaining:       map[gameplay.PlayerID]int{},
		doubleTurnEffectTurnsLeft: map[gameplay.PlayerID]int{},
	}
}

// StartTurn advances gameplay resources and applies any resolved ignition effects.
func (e *Engine) StartTurn(pid gameplay.PlayerID) error {
	if err := e.State.StartTurn(pid); err != nil {
		return err
	}
	return e.processResolvedIgnitions()
}

// EndTurn advances the active player in gameplay state.
func (e *Engine) EndTurn(pid gameplay.PlayerID) error {
	return e.State.EndTurn(pid)
}

// DrawCard pays the draw-mana cost and moves a card from the player's deck to their hand.
// Drawing is only permitted on the player's own turn and outside an open reaction window.
func (e *Engine) DrawCard(pid gameplay.PlayerID) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	if e.State.CurrentTurn != pid {
		return errors.New("can only draw on your own turn")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return errors.New("cannot draw while a reaction window is open")
	}
	return e.State.DrawCard(pid)
}

// ActivateCard validates reaction constraints (if any) and delegates activation to gameplay state.
func (e *Engine) ActivateCard(pid gameplay.PlayerID, handIndex int) error {
	return e.ActivateCardWithTargets(pid, handIndex, nil)
}

// ActivateCardWithTargets validates optional target pieces and delegates activation to gameplay state.
func (e *Engine) ActivateCardWithTargets(pid gameplay.PlayerID, handIndex int, targetPieces []chess.Pos) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	p := e.State.Players[pid]
	if handIndex < 0 || handIndex >= len(p.Hand) {
		return errors.New("invalid hand index")
	}
	def, ok := gameplay.CardDefinitionByID(p.Hand[handIndex].CardID)
	if !ok {
		return errors.New("unknown card definition")
	}
	// While a reaction window is open, plays resolve through the reaction stack (including the
	// actor's opponent on ignite_reaction), never through ignition activation — ActivateCard on
	// MatchState requires CurrentTurn == pid, which is false for the responder.
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return e.QueueReactionCard(pid, handIndex, EffectTarget{})
	}
	if def.Type == gameplay.CardTypeDisruption {
		return e.applyDisruptionOnOwnTurn(pid, handIndex)
	}
	if def.Type != gameplay.CardTypePower && def.Type != gameplay.CardTypeContinuous {
		return errors.New("only Power and Continuous cards can be activated in normal turn flow")
	}
	if len(targetPieces) > 0 && def.Targets == 0 {
		return errors.New("target_pieces not allowed for this card")
	}
	// Cards that require piece targets: first ignite_card may move hand→ignition only; targets follow via SubmitIgnitionTargets.
	if def.Targets > 0 && len(targetPieces) == 0 {
		if err := e.State.ActivateCard(pid, handIndex); err != nil {
			return err
		}
		e.lockIgnitionTargetPieces(pid, def.ID, nil)
		return nil
	}
	if len(targetPieces) > 0 {
		if err := e.validateIgnitionTargetPieces(pid, def.ID, targetPieces); err != nil {
			return err
		}
	}
	if err := e.State.ActivateCard(pid, handIndex); err != nil {
		return err
	}
	e.lockIgnitionTargetPieces(pid, def.ID, targetPieces)
	if err := e.processResolvedIgnitions(); err != nil {
		return err
	}
	e.maybeOpenIgniteReactionWindow(pid)
	return nil
}

// SubmitIgnitionTargets validates and locks board targets for the ignited card that requires Targets,
// then opens ignite_reaction when the catalog allows retribution against this ignition.
func (e *Engine) SubmitIgnitionTargets(pid gameplay.PlayerID, targetPieces []chess.Pos) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	if e.State.CurrentTurn != pid {
		return errors.New("not your turn")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return errors.New("cannot submit ignition targets during a reaction window")
	}
	p := e.State.Players[pid]
	if !p.Ignition.Occupied {
		return errors.New("ignition slot is empty")
	}
	card := p.Ignition.Card
	def, ok := gameplay.CardDefinitionByID(card.CardID)
	if !ok || def.Targets == 0 {
		return errors.New("this ignition does not require targets")
	}
	if _, locked := e.ignitionTargetCard[pid]; locked {
		return errors.New("ignition targets already submitted")
	}
	if err := e.validateIgnitionTargetPieces(pid, card.CardID, targetPieces); err != nil {
		return err
	}
	e.lockIgnitionTargetPieces(pid, card.CardID, targetPieces)
	e.maybeOpenIgniteReactionWindow(pid)
	return nil
}

func (e *Engine) lockIgnitionTargetPieces(pid gameplay.PlayerID, cardID gameplay.CardID, targetPieces []chess.Pos) {
	if len(targetPieces) == 0 {
		e.ignitionTargets[pid] = nil
		delete(e.ignitionTargetCard, pid)
		return
	}
	copied := make([]chess.Pos, len(targetPieces))
	copy(copied, targetPieces)
	e.ignitionTargets[pid] = copied
	e.ignitionTargetCard[pid] = cardID
}

// consumeIgnitionTargets returns and clears locked ignition targets for a specific owner/card.
func (e *Engine) consumeIgnitionTargets(pid gameplay.PlayerID, cardID gameplay.CardID) []chess.Pos {
	lockedCardID, ok := e.ignitionTargetCard[pid]
	if !ok || lockedCardID != cardID {
		return nil
	}
	targets := e.ignitionTargets[pid]
	delete(e.ignitionTargetCard, pid)
	e.ignitionTargets[pid] = nil
	out := make([]chess.Pos, len(targets))
	copy(out, targets)
	return out
}

func (e *Engine) validateIgnitionTargetPieces(pid gameplay.PlayerID, cardID gameplay.CardID, targetPieces []chess.Pos) error {
	def, ok := gameplay.CardDefinitionByID(cardID)
	if !ok || def.Targets == 0 || len(targetPieces) == 0 {
		return nil
	}
	if len(targetPieces) != def.Targets {
		return errors.New("invalid target_pieces count for card")
	}
	playerColor := toColor(pid)
	for _, pos := range targetPieces {
		if pos.Row < 0 || pos.Row > 7 || pos.Col < 0 || pos.Col > 7 {
			return errors.New("target piece out of board bounds")
		}
		piece := e.Chess.PieceAt(pos)
		if piece.IsEmpty() {
			return errors.New("target piece square is empty")
		}
		if piece.Color != playerColor {
			return errors.New("target piece must belong to the activating player")
		}
	}
	return nil
}

// IgnitionTargetSnapshot returns locked target metadata for reaction-window/snapshot rendering.
func (e *Engine) IgnitionTargetSnapshot() (gameplay.PlayerID, gameplay.CardID, []chess.Pos, bool) {
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		cardID, targets, ok := e.IgnitionTargetsForPlayer(pid)
		if !ok {
			continue
		}
		return pid, cardID, targets, true
	}
	return "", "", nil, false
}

// IgnitionTargetsForPlayer returns locked ignition piece coordinates for one seat, if any.
func (e *Engine) IgnitionTargetsForPlayer(pid gameplay.PlayerID) (gameplay.CardID, []chess.Pos, bool) {
	cardID, ok := e.ignitionTargetCard[pid]
	if !ok {
		return "", nil, false
	}
	targets := e.ignitionTargets[pid]
	if len(targets) == 0 {
		return "", nil, false
	}
	copied := make([]chess.Pos, len(targets))
	copy(copied, targets)
	return cardID, copied, true
}

// maybeOpenIgniteReactionWindow opens ignite_reaction for the opponent when a Power or Continuous
// card enters ignition (including ignition=0 cards, which now wait for response before resolving).
func (e *Engine) maybeOpenIgniteReactionWindow(activator gameplay.PlayerID) {
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return
	}
	slot := &e.State.Players[activator].Ignition
	if !slot.Occupied {
		return
	}
	card := slot.Card
	cardDef, ok := gameplay.CardDefinitionByID(card.CardID)
	if !ok {
		return
	}
	if cardDef.Type != gameplay.CardTypePower && cardDef.Type != gameplay.CardTypeContinuous {
		return
	}
	if cardDef.Targets > 0 {
		lockCard, ok := e.ignitionTargetCard[activator]
		if !ok || lockCard != card.CardID || len(e.ignitionTargets[activator]) != cardDef.Targets {
			return
		}
	}
	eligible := []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypeDisruption}
	if gameplay.MaybeCaptureAttemptOnIgnition(card.CardID) {
		eligible = append(eligible, gameplay.CardTypeCounter)
	}
	e.OpenReactionWindow("ignite_reaction", activator, eligible)
}

// ResolvePendingEffect applies the next queued target-dependent effect for the player.
func (e *Engine) ResolvePendingEffect(pid gameplay.PlayerID, target EffectTarget) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	queue := e.pendingEffects[pid]
	if len(queue) == 0 {
		return errors.New("no pending effect for player")
	}
	pe := queue[0]
	e.pendingEffects[pid] = queue[1:]
	return pe.Resolver.Apply(e, pe.Owner, target)
}

// SubmitMove executes a legal chess move, or defers it when a capture reaction window opens.
func (e *Engine) SubmitMove(pid gameplay.PlayerID, m chess.Move) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
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
	if e.ReactionWindow != nil && e.ReactionWindow.Open && e.ReactionWindow.Trigger == "ignite_reaction" {
		return errors.New("cannot submit move while ignite reaction window is open")
	}

	if e.isCaptureAttempt(pid, m) {
		if !e.moveWouldApplyAuthoritatively(pid, m) {
			return fmt.Errorf("illegal move")
		}
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

// applyDisruptionOnOwnTurn handles a Disruption card played on the owner's turn.
// The card enters the ignition slot like other activations (hand → ignition). Disruption does not
// open ignite_reaction (maybeOpenIgniteReactionWindow only applies to Power/Continuous). Catalog
// Ignition 0 means the effect resolves on the same turn after a room-driven snapshot so clients
// can show the card in the ignition zone during activate_card glow, then cooldown.
func (e *Engine) applyDisruptionOnOwnTurn(pid gameplay.PlayerID, handIndex int) error {
	if e.State.CurrentTurn != pid {
		return errors.New("not your turn")
	}
	opp := gameplay.OppositePlayer(pid)
	if !e.State.Players[opp].Ignition.Occupied {
		return errors.New("disruption cards require the opponent to have a card in ignition")
	}
	if err := e.State.ActivateCard(pid, handIndex); err != nil {
		return err
	}
	e.disruptionSameTurnGlowPID = pid
	return nil
}

// HasPendingDisruptionSameTurnResolve reports whether the room should broadcast an intermediate
// snapshot (card visible in ignition) before FinishDisruptionSameTurnResolveIfPending.
func (e *Engine) HasPendingDisruptionSameTurnResolve() bool {
	return e.disruptionSameTurnGlowPID != ""
}

// FinishDisruptionSameTurnResolveIfPending runs ignition resolution for a Disruption placed on the
// current turn (success=true when no failure path exists). Clears pending state; safe to call when
// no pending disruption (no-op).
func (e *Engine) FinishDisruptionSameTurnResolveIfPending() error {
	if e.disruptionSameTurnGlowPID == "" {
		return nil
	}
	pid := e.disruptionSameTurnGlowPID
	e.disruptionSameTurnGlowPID = ""
	if err := e.State.ResolveIgnitionFor(pid, true); err != nil {
		return err
	}
	return e.processResolvedIgnitions()
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
		e.appendActivationFX(evCopy.Owner, evCopy.Card.CardID, evCopy.Success)
		e.State.SendCardToCooldown(evCopy.Owner, evCopy.Card)
		e.ignitionTargets[evCopy.Owner] = nil
		delete(e.ignitionTargetCard, evCopy.Owner)
	}
	return nil
}

// ActivatePlayerSkill executes the selected player skill on the current turn and consumes that turn.
func (e *Engine) ActivatePlayerSkill(pid gameplay.PlayerID) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	color := toColor(pid)
	if e.Chess.Turn != color {
		return errors.New("chess turn out of sync with match turn")
	}
	if err := e.State.ActivateSpecialAbility(pid); err != nil {
		return err
	}
	e.expireMovementGrantsAfterOwnerTurn(pid)
	delete(e.extraMovesRemaining, pid)
	delete(e.doubleTurnEffectTurnsLeft, pid)
	e.Chess.Turn = color.Opponent()
	return e.StartTurn(e.State.CurrentTurn)
}

// pieceRefFromChessPiece converts a board piece to the compact graveyard descriptor (e.g. wP, bQ).
// Returns false for an empty square or unsupported type.
func pieceRefFromChessPiece(p chess.Piece) (gameplay.PieceRef, bool) {
	if p.IsEmpty() {
		return gameplay.PieceRef{}, false
	}
	color := "w"
	if p.Color == chess.Black {
		color = "b"
	}
	var typ string
	switch p.Type {
	case chess.Pawn:
		typ = "P"
	case chess.Knight:
		typ = "N"
	case chess.Bishop:
		typ = "B"
	case chess.Rook:
		typ = "R"
	case chess.Queen:
		typ = "Q"
	case chess.King:
		typ = "K"
	default:
		return gameplay.PieceRef{}, false
	}
	return gameplay.PieceRef{Color: color, Type: typ}, true
}

// applyMoveCore applies a validated move without opening capture trigger windows.
// It is used by normal non-capture flow and pending-move finalization.
func (e *Engine) applyMoveCore(pid gameplay.PlayerID, m chess.Move) error {
	e.pruneStaleMovementGrants()
	captureForMana := e.isCaptureAttempt(pid, m)
	var captured gameplay.PieceRef
	haveCaptured := false
	if captureForMana {
		fromPiece := e.Chess.PieceAt(m.From)
		target := e.Chess.PieceAt(m.To)
		// En passant: captured pawn sits on EnPassant.PawnPos, not on m.To.
		if fromPiece.Type == chess.Pawn && target.IsEmpty() && m.From.Col != m.To.Col &&
			e.Chess.EnPassant.Valid && m.To == e.Chess.EnPassant.Target {
			ep := e.Chess.PieceAt(e.Chess.EnPassant.PawnPos)
			if ref, ok := pieceRefFromChessPiece(ep); ok && ep.Color != fromPiece.Color {
				captured = ref
				haveCaptured = true
			}
		} else if !target.IsEmpty() && target.Color != fromPiece.Color {
			if ref, ok := pieceRefFromChessPiece(target); ok {
				captured = ref
				haveCaptured = true
			}
		}
	}
	if err := e.applyAuthorizedMove(pid, m); err != nil {
		return err
	}
	e.advanceMovementGrantPosition(pid, m.From, m.To)
	if haveCaptured {
		e.State.AddToGraveyard(pid, captured)
	}
	if captureForMana {
		e.State.GrantManaForChessCapture(pid)
	}
	// If the player has an extra move remaining (granted by Double Turn), consume it
	// without ending the turn. Restore Chess.Turn so the same player moves again.
	if e.extraMovesRemaining[pid] > 0 {
		e.extraMovesRemaining[pid]--
		e.Chess.Turn = toColor(pid)
		e.reconcileTurnState()
		return nil
	}
	e.expireMovementGrantsAfterOwnerTurn(pid)
	if err := e.State.EndTurn(pid); err != nil {
		return err
	}
	if err := e.StartTurn(e.State.CurrentTurn); err != nil {
		return err
	}
	return nil
}

// applyAuthorizedMove executes a move through normal chess legality, or through pseudo-legal
// application when an active effect grants an extra movement pattern for this piece.
func (e *Engine) applyAuthorizedMove(pid gameplay.PlayerID, m chess.Move) error {
	if e.isStandardLegalMove(m) {
		return e.Chess.ApplyMove(m)
	}
	if e.canUseAugmentedMovement(pid, m) {
		return e.Chess.ApplyPseudoLegalMove(m)
	}
	return fmt.Errorf("illegal move")
}

// isStandardLegalMove checks whether m is currently legal under standard chess rules.
func (e *Engine) isStandardLegalMove(m chess.Move) bool {
	for _, cand := range e.Chess.LegalMovesFrom(m.From) {
		if cand.To == m.To {
			return true
		}
	}
	return false
}

// moveWouldApplyAuthoritatively reports whether applyAuthorizedMove would accept m (including
// king-in-check rules) without mutating engine state. Used so capture_attempt is not opened for
// pseudo-legal or pinned captures that are still illegal chess.
func (e *Engine) moveWouldApplyAuthoritatively(pid gameplay.PlayerID, m chess.Move) bool {
	if e.isStandardLegalMove(m) {
		return true
	}
	if !e.canUseAugmentedMovement(pid, m) {
		return false
	}
	cp := e.Chess.Clone()
	return cp.ApplyPseudoLegalMove(m) == nil
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

// DoubleTurnActiveFor returns the PlayerID for whom the Double Turn visual effect is still
// active (i.e. the owner still has turns remaining on the effect). The highlight persists for
// the full owner turn even after the extra move has been used.
// Returns an empty string when no Double Turn effect is active.
func (e *Engine) DoubleTurnActiveFor() gameplay.PlayerID {
	for pid, n := range e.doubleTurnEffectTurnsLeft {
		if n > 0 {
			return pid
		}
	}
	return ""
}

// DoubleTurnTurnsRemainingFor returns how many owner turns the Double Turn effect has left,
// or 0 if no Double Turn effect is active for pid.
func (e *Engine) DoubleTurnTurnsRemainingFor(pid gameplay.PlayerID) int {
	return e.doubleTurnEffectTurnsLeft[pid]
}

// SetExtraMovesRemainingForTest directly sets the extra-move counter and the visual
// effect duration for a player. The effect duration is set to the same value as n
// (simulating that the resolver just fired and granted n extra moves).
// This is intended for tests only; do not call from production code.
func (e *Engine) SetExtraMovesRemainingForTest(pid gameplay.PlayerID, n int) {
	e.extraMovesRemaining[pid] = n
	if n > 0 {
		e.doubleTurnEffectTurnsLeft[pid] = n
	}
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

// EffectTarget is an alias for the type defined in the resolvers package, kept here so
// existing code in internal/match (and its tests) does not need to change its references.
type EffectTarget = matchresolvers.EffectTarget

// EffectResolver is an alias for the interface defined in the resolvers package.
type EffectResolver = matchresolvers.EffectResolver

// PendingEffect represents a resolved ignition effect waiting for player target input.
type PendingEffect struct {
	Owner    gameplay.PlayerID
	CardID   gameplay.CardID
	Resolver EffectResolver
}

// --- matchresolvers.ResolverEngine implementation ---

// ConsumeIgnitionTargets implements matchresolvers.ResolverEngine.
func (e *Engine) ConsumeIgnitionTargets(owner gameplay.PlayerID, cardID gameplay.CardID) []chess.Pos {
	return e.consumeIgnitionTargets(owner, cardID)
}

// PieceAt implements matchresolvers.ResolverEngine.
func (e *Engine) PieceAt(pos chess.Pos) chess.Piece {
	return e.Chess.PieceAt(pos)
}

// OwnerColor implements matchresolvers.ResolverEngine.
func (e *Engine) OwnerColor(owner gameplay.PlayerID) chess.Color {
	return toColor(owner)
}

// AddMovementGrant implements matchresolvers.ResolverEngine.
func (e *Engine) AddMovementGrant(owner gameplay.PlayerID, cardID gameplay.CardID, target chess.Pos, kind matchresolvers.MovementGrantKind, durationTurns int) {
	e.addMovementGrant(MovementGrant{
		Owner:               owner,
		SourceCardID:        cardID,
		Target:              target,
		Kind:                kind,
		RemainingOwnerTurns: durationTurns,
	})
}

// GrantManaFromCardEffect implements matchresolvers.ResolverEngine.
func (e *Engine) GrantManaFromCardEffect(pid gameplay.PlayerID, amount int) {
	e.State.GrantManaFromCardEffect(pid, amount)
}

// IncrementExtraMoves implements matchresolvers.ResolverEngine.
// It increments the extra-move counter AND records the effect duration for the visual highlight.
func (e *Engine) IncrementExtraMoves(pid gameplay.PlayerID) {
	e.extraMovesRemaining[pid]++
	// The visual effect highlight lasts for EffectDuration turns; Double Turn has EffectDuration=1.
	def, ok := gameplay.CardDefinitionByID(CardDoubleTurn)
	effectTurns := 1
	if ok && def.EffectDuration > 0 {
		effectTurns = def.EffectDuration
	}
	if e.doubleTurnEffectTurnsLeft[pid] < effectTurns {
		e.doubleTurnEffectTurnsLeft[pid] = effectTurns
	}
}

// NegateOpponentIgnition implements matchresolvers.ResolverEngine.
func (e *Engine) NegateOpponentIgnition(opponentPID gameplay.PlayerID) error {
	return e.State.ResolveIgnitionFor(opponentPID, false)
}

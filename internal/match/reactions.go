package match

import (
	"errors"

	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

// ReactionWindow defines an open response window (Counter/Retribution) for the current trigger.
type ReactionWindow struct {
	Open          bool
	Trigger       string
	Actor         gameplay.PlayerID
	EligibleTypes []gameplay.CardType
}

// ReactionAction stores a queued reaction card and its chosen targets.
type ReactionAction struct {
	Owner    gameplay.PlayerID
	Card     gameplay.CardInstance
	Target   EffectTarget
	Resolver EffectResolver
}

// ReactionStackEntry describes one queued reaction card for UI previews. The slice returned by
// ReactionStackEntries is ordered bottom-first (first queued card first); server resolution is LIFO.
type ReactionStackEntry struct {
	Owner  gameplay.PlayerID
	CardID gameplay.CardID
}

// ReactionStackEntries returns a copy of queued reaction cards from bottom of stack to top.
func (e *Engine) ReactionStackEntries() []ReactionStackEntry {
	return e.reactions.Entries()
}

// OpenReactionWindow starts a reaction phase constrained by allowed card types.
func (e *Engine) OpenReactionWindow(trigger string, actor gameplay.PlayerID, eligible []gameplay.CardType) {
	e.ReactionWindow = &ReactionWindow{
		Open:          true,
		Trigger:       trigger,
		Actor:         actor,
		EligibleTypes: append([]gameplay.CardType(nil), eligible...),
	}
}

// CloseReactionWindow ends the reaction phase and clears queued reactions.
func (e *Engine) CloseReactionWindow() {
	e.ReactionWindow = nil
	e.reactions.Clear()
}

// QueueReactionCard consumes a card from hand and pushes it to the reaction stack.
// banishHandIndex must be >= 0 when the card being queued is a Disruption type in an
// ignite_reaction window: it identifies a Power card in hand to banish as the mandatory
// ignition cost for that card type. Pass -1 for all non-Disruption reactions.
func (e *Engine) QueueReactionCard(pid gameplay.PlayerID, handIndex int, banishHandIndex int, target EffectTarget) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	if err := e.errIfPlayerHasUnresolvedPendingEffects(pid); err != nil {
		return err
	}
	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		return errors.New("reaction window is not open")
	}
	p := e.State.Players[pid]
	if handIndex < 0 || handIndex >= len(p.Hand) {
		return errors.New("invalid hand index")
	}
	card := p.Hand[handIndex]
	def, ok := gameplay.CardDefinitionByID(card.CardID)
	if !ok {
		return errors.New("unknown card definition")
	}
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
	if e.ReactionWindow.Trigger == "capture_attempt" && e.reactions.Len() == 0 {
		if pid == e.ReactionWindow.Actor {
			return errors.New("capture reaction chain must be started by the opponent")
		}
		if def.Type != gameplay.CardTypeCounter {
			return errors.New("capture reaction chain must start with a Counter card")
		}
	}
	if e.ReactionWindow.Trigger == "ignite_reaction" && e.reactions.Len() == 0 {
		if pid == e.ReactionWindow.Actor {
			return errors.New("ignite reaction must be started by the opponent")
		}
		if def.Type != gameplay.CardTypeRetribution &&
			def.Type != gameplay.CardTypeCounter &&
			def.Type != gameplay.CardTypeDisruption {
			return errors.New("ignite reaction must start with a Retribution, Counter, or Disruption card")
		}
		if def.Type == gameplay.CardTypeDisruption {
			// The actor opened the window precisely because they put a card in ignition,
			// so this should always be true — checked explicitly as a safety guard.
			if !e.State.Players[e.ReactionWindow.Actor].Ignition.Occupied {
				return errors.New("disruption cards require the opponent to have a card in ignition")
			}
			// Disruption type ignition cost: banish 1 Power card from hand when responding
			// during the opponent's ignite_reaction window.
			if banishHandIndex < 0 {
				return errors.New("disruption reaction requires banishing 1 Power card from hand")
			}
			if banishHandIndex == handIndex {
				return errors.New("the banished card must be different from the disruption card")
			}
			if banishHandIndex < 0 || banishHandIndex >= len(p.Hand) {
				return errors.New("invalid banish hand index")
			}
			banishCard := p.Hand[banishHandIndex]
			banishDef, banishOK := gameplay.CardDefinitionByID(banishCard.CardID)
			if !banishOK || banishDef.Type != gameplay.CardTypePower {
				return errors.New("disruption reaction cost requires a Power card to be banished")
			}
			// Banish the Power card before consuming the Disruption card. If the banished
			// card sits before the Disruption card in hand, the Disruption card shifts down.
			if _, err := e.State.BanishCardFromHand(pid, banishHandIndex); err != nil {
				return err
			}
			if banishHandIndex < handIndex {
				handIndex--
			}
		}
	}
	if e.reactions.Len() > 0 {
		prev, _ := e.reactions.Top()
		prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
		if e.ReactionWindow.Trigger == "ignite_reaction" {
			if ok && (prevDef.Type == gameplay.CardTypeRetribution || prevDef.Type == gameplay.CardTypeDisruption) {
				if def.Type != gameplay.CardTypeRetribution && def.Type != gameplay.CardTypeDisruption {
					return errors.New("only Retribution or Disruption cards can respond in this ignite chain")
				}
			}
		}
		if e.ReactionWindow.Trigger == "capture_attempt" {
			if ok && prevDef.Type == gameplay.CardTypeRetribution && def.Type != gameplay.CardTypeRetribution {
				return errors.New("only Retribution cards can respond to Retribution cards")
			}
		}
		if ok && prevDef.Type == gameplay.CardTypeCounter && def.Type != gameplay.CardTypeCounter {
			return errors.New("only Counter cards can respond to Counter cards")
		}
	}
	if err := e.validateReactionTarget(pid, card.CardID, target); err != nil {
		return err
	}
	resolver, ok := e.resolvers[card.CardID]
	if !ok {
		return errors.New("card has no resolver")
	}
	consumed, err := e.State.ConsumeCardFromHand(pid, handIndex)
	if err != nil {
		return err
	}
	e.reactions.Push(ReactionAction{
		Owner:    pid,
		Card:     consumed,
		Target:   target,
		Resolver: resolver,
	})
	return nil
}

// validateReactionTarget checks card-specific target payloads before reaction costs are paid.
func (e *Engine) validateReactionTarget(pid gameplay.PlayerID, cardID gameplay.CardID, target EffectTarget) error {
	if cardID != CardRetaliate {
		return nil
	}
	if target.TargetCard == nil {
		return errors.New("retaliate requires a target cooldown Power card")
	}
	if _, ok := e.retaliateCooldownTarget(pid, *target.TargetCard); !ok {
		return errors.New("retaliate target must be an opponent cooldown Power card they can pay with regular mana")
	}
	return nil
}

// canExtendRetributionFollowUp reports whether pid may queue another Retribution after a
// Retribution or legacy Power reaction, using ignition clearance rules shared by capture and ignite chains.
func (e *Engine) canExtendRetributionFollowUp(pid gameplay.PlayerID, prevDef gameplay.CardDefinition) bool {
	if prevDef.Type != gameplay.CardTypeRetribution && prevDef.Type != gameplay.CardTypePower {
		return false
	}
	p := e.State.Players[pid]
	if p == nil {
		return false
	}
	ignitionOccupied := e.ReactionWindow != nil && e.State.Players[e.ReactionWindow.Actor].Ignition.Occupied
	onCooldown := make(map[string]struct{}, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		onCooldown[string(cd.Card.CardID)] = struct{}{}
	}
	for _, c := range p.Hand {
		def, ok := gameplay.CardDefinitionByID(c.CardID)
		if !ok {
			continue
		}
		if prevDef.Type == gameplay.CardTypeRetribution && def.Type != gameplay.CardTypeRetribution {
			continue
		}
		if prevDef.Type == gameplay.CardTypePower && def.Type != gameplay.CardTypeRetribution {
			continue
		}
		allowed := false
		for _, t := range e.ReactionWindow.EligibleTypes {
			if def.Type == t {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		if _, dup := onCooldown[string(c.CardID)]; dup {
			continue
		}
		if p.Mana < def.Cost {
			continue
		}
		if c.CardID == CardRetaliate && !gameplay.HasValidRetaliateCooldownTarget(e.State, pid) {
			continue
		}
		if !ignitionOccupied || gameplay.CardClearsOpponentIgnitionForChain(c.CardID) {
			return true
		}
	}
	return false
}

// CanPlayerExtendIgniteChain reports whether pid can legally queue another card in the current
// ignite_reaction chain (non-empty stack). Used to auto-resolve when the responding seat has no play.
//
// While the shared ignition slot remains occupied, the chain only continues if pid has a legal
// follow-up that also clears or negates the opponent's ignited card (see
// gameplay.CardClearsOpponentIgnitionForChain). If the slot is free, any legal follow-up suffices.
func (e *Engine) CanPlayerExtendIgniteChain(pid gameplay.PlayerID) bool {
	if e.ReactionWindow == nil || !e.ReactionWindow.Open || e.ReactionWindow.Trigger != "ignite_reaction" {
		return false
	}
	if e.reactions.Len() == 0 {
		return false
	}
	prev, _ := e.reactions.Top()
	prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
	if !ok {
		return false
	}
	if prevDef.Type == gameplay.CardTypeCounter {
		return e.CanPlayerExtendCounterChain(pid)
	}
	return e.canExtendRetributionFollowUp(pid, prevDef)
}

// CanPlayerExtendCaptureReactionChain reports whether pid can extend a non-empty capture_attempt stack.
func (e *Engine) CanPlayerExtendCaptureReactionChain(pid gameplay.PlayerID) bool {
	if e.ReactionWindow == nil || !e.ReactionWindow.Open || e.ReactionWindow.Trigger != "capture_attempt" || e.reactions.Len() == 0 {
		return false
	}
	prev, _ := e.reactions.Top()
	prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
	if !ok {
		return false
	}
	if prevDef.Type == gameplay.CardTypeCounter {
		return e.CanPlayerExtendCounterChain(pid)
	}
	if prevDef.Type == gameplay.CardTypeRetribution {
		return e.canExtendRetributionFollowUp(pid, prevDef)
	}
	return false
}

// CanPlayerExtendCounterChain reports whether pid can legally queue another Counter on a non-empty
// capture_attempt or ignite_reaction stack. While the shared ignition slot is occupied, only Counters that negate a
// stacked Counter (gameplay.CardNegatesOpponentCounterOnCaptureChain) allow the chain to continue.
func (e *Engine) CanPlayerExtendCounterChain(pid gameplay.PlayerID) bool {
	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		return false
	}
	if e.ReactionWindow.Trigger != "capture_attempt" && e.ReactionWindow.Trigger != "ignite_reaction" {
		return false
	}
	if e.reactions.Len() == 0 {
		return false
	}
	prev, _ := e.reactions.Top()
	prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
	if !ok || prevDef.Type != gameplay.CardTypeCounter {
		return false
	}
	p := e.State.Players[pid]
	if p == nil {
		return false
	}
	ignitionOccupied := e.ReactionWindow != nil && e.State.Players[e.ReactionWindow.Actor].Ignition.Occupied
	onCooldown := make(map[string]struct{}, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		onCooldown[string(cd.Card.CardID)] = struct{}{}
	}
	for _, c := range p.Hand {
		def, ok := gameplay.CardDefinitionByID(c.CardID)
		if !ok || def.Type != gameplay.CardTypeCounter {
			continue
		}
		allowed := false
		for _, t := range e.ReactionWindow.EligibleTypes {
			if def.Type == t {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		if _, dup := onCooldown[string(c.CardID)]; dup {
			continue
		}
		if p.Mana < def.Cost {
			continue
		}
		if !ignitionOccupied || gameplay.CardNegatesOpponentCounterOnCaptureChain(c.CardID) {
			return true
		}
	}
	return false
}

// ResolveReactionStack applies queued reactions in LIFO order, one card at a time: each resolution
// emits an activate_card broadcast (effect success/fail step) before SendCardToCooldown, so clients
// can animate in stack order until the pile is empty.
func (e *Engine) ResolveReactionStack() error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	rwTrigger := ""
	if e.ReactionWindow != nil {
		rwTrigger = e.ReactionWindow.Trigger
	}
	for e.reactions.Len() > 0 {
		a, _ := e.reactions.Pop()
		negatesActivationOf, err := e.runWithNegationDetection(a.Owner, func() error {
			return a.Resolver.Apply(e, a.Owner, a.Target)
		})
		if err != nil {
			if !errors.Is(err, matchresolvers.ErrEffectFailed) {
				return err
			}
		}
		// Some reaction effects can fail during resolution if their target condition changed
		// while higher stack entries resolved; the card still leaves the stack.
		success := !errors.Is(err, matchresolvers.ErrEffectFailed)
		e.appendActivationFXNegating(a.Owner, a.Card.CardID, success, negatesActivationOf)
		e.State.SendCardToCooldown(a.Owner, a.Card)
		if e.hasPendingEffects() {
			return nil
		}
	}
	if e.pendingMove != nil {
		pm := *e.pendingMove
		e.pendingMove = nil
		if err := e.applyMoveCore(pm.PlayerID, pm.Move); err != nil {
			return err
		}
	}
	// ignite_reaction close: ignition-0 Power/Continuous finalize here. Continuous with TurnsRemaining > 0
	// also fires its first activation in the same turn (after the window), then stays in the slot.
	if rwTrigger == "ignite_reaction" && e.ReactionWindow != nil {
		act := e.ReactionWindow.Actor
		ig := &e.State.Players[act].Ignition
		if !ig.Occupied {
			// no-op
		} else if ig.TurnsRemaining == 0 {
			success := !ig.EffectNegated
			if err := e.State.ResolveIgnitionFor(act, success); err != nil {
				return err
			}
			if err := e.processResolvedIgnitions(); err != nil {
				return err
			}
		} else {
			def, ok := gameplay.CardDefinitionByID(ig.Card.CardID)
			if ok && def.Type == gameplay.CardTypeContinuous {
				e.State.PulseContinuousIgnitionMidTurn(act)
				if err := e.processResolvedIgnitions(); err != nil {
					return err
				}
			}
		}
	}
	e.CloseReactionWindow()
	return nil
}

// PendingEffects returns a read-only copy of all unresolved target-dependent effects.
func (e *Engine) PendingEffects() []PendingEffect {
	out := []PendingEffect{}
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		queue := e.pendingEffects[pid]
		out = append(out, queue...)
	}
	return out
}

// hasPendingEffects reports whether any player has unresolved target-dependent effects.
func (e *Engine) hasPendingEffects() bool {
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		if len(e.pendingEffects[pid]) > 0 {
			return true
		}
	}
	return false
}

// ReactionWindowSnapshot returns a copy of current reaction window and stack size.
func (e *Engine) ReactionWindowSnapshot() (ReactionWindow, int, bool) {
	if e.ReactionWindow == nil {
		return ReactionWindow{}, 0, false
	}
	cp := *e.ReactionWindow
	cp.EligibleTypes = append([]gameplay.CardType(nil), cp.EligibleTypes...)
	return cp, e.reactions.Len(), true
}

// ReactionStackTopSnapshot returns the last queued reaction (most recent play), if the stack is non-empty.
func (e *Engine) ReactionStackTopSnapshot() (ReactionAction, bool) {
	return e.reactions.Top()
}

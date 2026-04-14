package match

import (
	"errors"

	"power-chess/internal/gameplay"
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
	if len(e.reactionStack) == 0 {
		return nil
	}
	out := make([]ReactionStackEntry, 0, len(e.reactionStack))
	for _, a := range e.reactionStack {
		out = append(out, ReactionStackEntry{Owner: a.Owner, CardID: a.Card.CardID})
	}
	return out
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
	e.reactionStack = nil
}

// QueueReactionCard consumes a card from hand and pushes it to the reaction stack.
func (e *Engine) QueueReactionCard(pid gameplay.PlayerID, handIndex int, target EffectTarget) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
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
	if e.ReactionWindow.Trigger == "capture_attempt" && len(e.reactionStack) == 0 {
		if pid == e.ReactionWindow.Actor {
			return errors.New("capture reaction chain must be started by the opponent")
		}
		if def.Type != gameplay.CardTypeCounter {
			return errors.New("capture reaction chain must start with a Counter card")
		}
	}
	if e.ReactionWindow.Trigger == "ignite_reaction" && len(e.reactionStack) == 0 {
		if pid == e.ReactionWindow.Actor {
			return errors.New("ignite reaction must be started by the opponent")
		}
		if def.Type != gameplay.CardTypeRetribution && def.Type != gameplay.CardTypePower {
			return errors.New("ignite reaction must start with Retribution or Power")
		}
	}
	if len(e.reactionStack) > 0 {
		prev := e.reactionStack[len(e.reactionStack)-1]
		prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
		if e.ReactionWindow.Trigger == "ignite_reaction" {
			if ok && prevDef.Type == gameplay.CardTypeRetribution && def.Type != gameplay.CardTypeRetribution {
				return errors.New("only Retribution cards can respond to Retribution cards")
			}
			if ok && prevDef.Type == gameplay.CardTypePower && def.Type != gameplay.CardTypeRetribution {
				return errors.New("only Retribution cards can respond after a Power reaction")
			}
		}
		if ok && prevDef.Type == gameplay.CardTypeCounter && def.Type != gameplay.CardTypeCounter {
			return errors.New("only Counter cards can respond to Counter cards")
		}
	}
	resolver, ok := e.resolvers[card.CardID]
	if !ok {
		return errors.New("card has no resolver")
	}
	consumed, err := e.State.ConsumeCardFromHand(pid, handIndex)
	if err != nil {
		return err
	}
	e.reactionStack = append(e.reactionStack, ReactionAction{
		Owner:    pid,
		Card:     consumed,
		Target:   target,
		Resolver: resolver,
	})
	return nil
}

// CanPlayerExtendIgniteChain reports whether pid can legally queue another card in the current
// ignite_reaction chain (non-empty stack). Used to auto-resolve when the responding seat has no play.
func (e *Engine) CanPlayerExtendIgniteChain(pid gameplay.PlayerID) bool {
	if e.ReactionWindow == nil || !e.ReactionWindow.Open || e.ReactionWindow.Trigger != "ignite_reaction" {
		return false
	}
	if len(e.reactionStack) == 0 {
		return false
	}
	prev := e.reactionStack[len(e.reactionStack)-1]
	prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
	if !ok {
		return false
	}
	p := e.State.Players[pid]
	if p == nil {
		return false
	}
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
		return true
	}
	return false
}

// ResolveReactionStack applies queued reactions in LIFO order and sends them to cooldown.
func (e *Engine) ResolveReactionStack() error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	rwTrigger := ""
	if e.ReactionWindow != nil {
		rwTrigger = e.ReactionWindow.Trigger
	}
	for i := len(e.reactionStack) - 1; i >= 0; i-- {
		a := e.reactionStack[i]
		if err := a.Resolver.Apply(e, a.Owner, a.Target); err != nil {
			return err
		}
		e.State.SendCardToCooldown(a.Owner, a.Card)
	}
	if e.pendingMove != nil {
		pm := *e.pendingMove
		e.pendingMove = nil
		if err := e.applyMoveCore(pm.PlayerID, pm.Move); err != nil {
			return err
		}
	}
	// Delayed ignition (TurnsRemaining > 0) must keep burning after the reaction window; only
	// ignition-0 cards finalize here (see Energy Gain, Continuous powers).
	if rwTrigger == "ignite_reaction" && e.State.IgnitionSlot.Occupied && e.State.IgnitionSlot.TurnsRemaining == 0 {
		if err := e.State.ResolveIgnition(true); err != nil {
			return err
		}
		if err := e.processResolvedIgnitions(); err != nil {
			return err
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
		for _, pe := range queue {
			out = append(out, pe)
		}
	}
	return out
}

// ReactionWindowSnapshot returns a copy of current reaction window and stack size.
func (e *Engine) ReactionWindowSnapshot() (ReactionWindow, int, bool) {
	if e.ReactionWindow == nil {
		return ReactionWindow{}, 0, false
	}
	cp := *e.ReactionWindow
	cp.EligibleTypes = append([]gameplay.CardType(nil), cp.EligibleTypes...)
	return cp, len(e.reactionStack), true
}

// ReactionStackTopSnapshot returns the last queued reaction (most recent play), if the stack is non-empty.
func (e *Engine) ReactionStackTopSnapshot() (ReactionAction, bool) {
	if len(e.reactionStack) == 0 {
		return ReactionAction{}, false
	}
	return e.reactionStack[len(e.reactionStack)-1], true
}

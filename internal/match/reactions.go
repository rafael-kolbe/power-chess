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
	if def.ID == CardBlockade {
		if e.ReactionWindow.Trigger != "capture_attempt" || e.pendingMove == nil {
			return errors.New("blockade requires an active capture_attempt chain")
		}
		if pid != e.ReactionWindow.Actor {
			return errors.New("blockade can only be played by the attacking player")
		}
		if len(e.reactionStack) == 0 || e.reactionStack[len(e.reactionStack)-1].Card.CardID != CardCounterattack {
			return errors.New("blockade must respond directly to counterattack")
		}
	}
	if len(e.reactionStack) > 0 {
		prev := e.reactionStack[len(e.reactionStack)-1]
		prevDef, ok := gameplay.CardDefinitionByID(prev.Card.CardID)
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

// ResolveReactionStack applies queued reactions in LIFO order and sends them to cooldown.
func (e *Engine) ResolveReactionStack() error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	for i := len(e.reactionStack) - 1; i >= 0; i-- {
		a := e.reactionStack[i]
		if a.Card.CardID == CardBlockade {
			if i-1 < 0 || e.reactionStack[i-1].Card.CardID != CardCounterattack {
				return errors.New("blockade resolution requires preceding counterattack")
			}
			// Blockade negates counterattack and keeps attacker on original square.
			e.pendingMove = nil
			e.State.SendCardToCooldown(a.Owner, a.Card)
			e.State.SendCardToCooldown(e.reactionStack[i-1].Owner, e.reactionStack[i-1].Card)
			i--
			continue
		}
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


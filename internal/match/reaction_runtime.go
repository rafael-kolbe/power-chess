package match

// ReactionRuntime centralizes runtime operations over the reaction stack.
type ReactionRuntime struct {
	stack []ReactionAction
}

// NewReactionRuntime creates an empty runtime stack.
func NewReactionRuntime() *ReactionRuntime {
	return &ReactionRuntime{stack: nil}
}

// NewReactionRuntimeFromActions creates a runtime initialized from existing actions.
func NewReactionRuntimeFromActions(actions []ReactionAction) *ReactionRuntime {
	cp := append([]ReactionAction(nil), actions...)
	return &ReactionRuntime{stack: cp}
}

// Push appends one action on top of the stack.
func (r *ReactionRuntime) Push(action ReactionAction) {
	r.stack = append(r.stack, action)
}

// Pop removes and returns the top action from the stack.
func (r *ReactionRuntime) Pop() (ReactionAction, bool) {
	if len(r.stack) == 0 {
		return ReactionAction{}, false
	}
	idx := len(r.stack) - 1
	action := r.stack[idx]
	r.stack = r.stack[:idx]
	return action, true
}

// Len returns current stack size.
func (r *ReactionRuntime) Len() int {
	return len(r.stack)
}

// Clear empties the stack.
func (r *ReactionRuntime) Clear() {
	r.stack = nil
}

// Actions returns a snapshot of stack actions.
func (r *ReactionRuntime) Actions() []ReactionAction {
	return append([]ReactionAction(nil), r.stack...)
}

// Top returns the top-most queued action.
func (r *ReactionRuntime) Top() (ReactionAction, bool) {
	if len(r.stack) == 0 {
		return ReactionAction{}, false
	}
	return r.stack[len(r.stack)-1], true
}

// Entries returns bottom-first stack entries for UI previews.
func (r *ReactionRuntime) Entries() []ReactionStackEntry {
	if len(r.stack) == 0 {
		return nil
	}
	out := make([]ReactionStackEntry, 0, len(r.stack))
	for _, a := range r.stack {
		out = append(out, ReactionStackEntry{Owner: a.Owner, CardID: a.Card.CardID})
	}
	return out
}

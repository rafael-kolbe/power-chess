export function getReactionState(snapshot) {
    const rw = snapshot?.reactionWindow || {};
    const stackSize = Number(rw.stackSize || 0);
    const open = !!rw.open;
    const waitingFirstResponse = open && rw.actor && Number.isFinite(stackSize) && stackSize === 0;
    const resolvingChain = open && !waitingFirstResponse;
    return { open, waitingFirstResponse, resolvingChain, actor: rw.actor || "" };
}

export function isOpenGameState(snapshot, uiLocks) {
    if (snapshot?.gameStarted !== true || snapshot?.matchEnded) return false;
    if (snapshot?.debugPauseActive) return false;
    if (uiLocks?.turnStartAnimation) return false;
    if (uiLocks?.effectAnimation) return false;
    if (snapshot?.reconnectPendingFor) return false;
    // Do not block the whole UI while reactionWindow.stackSize > 0: the attacker must still play
    // Blockade or confirm resolve_reactions after a Counter; the server rejects illegal actions.
    return true;
}

export function computeClockDisplay(snapshot, params) {
    const { currentTurn, turnSeconds, nowMs } = params;
    const values = { A: turnSeconds, B: turnSeconds };
    const modes = { A: "response", B: "response" };
    if (!snapshot) return { values, modes };

    const turnHolder = snapshot?.turnPlayer || currentTurn || "A";
    const other = turnHolder === "A" ? "B" : "A";
    const mainEnd = Number(snapshot?.turnMainDeadlineUnixMs || 0);
    const pausedMs = Number(snapshot?.turnMainPausedRemainingMs || 0);
    const reactionEnd = Number(snapshot?.reactionDeadlineUnixMs || 0);

    let turnLeft = turnSeconds;
    if (Number.isFinite(mainEnd) && mainEnd > 0) {
        turnLeft = Math.max(0, Math.ceil((mainEnd - nowMs) / 1000));
    } else if (Number.isFinite(pausedMs) && pausedMs > 0) {
        turnLeft = Math.max(0, Math.ceil(pausedMs / 1000));
    }
    values[turnHolder] = turnLeft;
    modes[turnHolder] = "turn";
    modes[other] = "response";

    const r = getReactionState(snapshot);
    if (!r.waitingFirstResponse || !r.actor) {
        values[other] = turnSeconds;
        return { values, modes };
    }

    const responder = r.actor === "A" ? "B" : "A";
    const responseLeft =
        Number.isFinite(reactionEnd) && reactionEnd > 0
            ? Math.max(0, Math.ceil((reactionEnd - nowMs) / 1000))
            : turnSeconds;
    values[responder] = responseLeft;
    modes[responder] = "response";
    modes[r.actor] = "turn";
    return { values, modes };
}

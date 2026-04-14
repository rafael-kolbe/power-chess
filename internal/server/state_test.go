package server

import (
	"testing"
	"time"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

const newRoomFailedFmt = "new room failed: %v"

// syncDisconnectBudgetForTest sets per-seat remaining disconnect budget to d (tests that tune DisconnectBudgetTotal).
func syncDisconnectBudgetForTest(room *RoomSession, d time.Duration) {
	room.DisconnectBudgetTotal = d
	room.ensureDisconnectBudgetMapsUnsafe()
	room.disconnectBudgetRemaining[gameplay.PlayerA] = d
	room.disconnectBudgetRemaining[gameplay.PlayerB] = d
}

// TestMarkRequestOnce ensures request idempotency keys are accepted only once.
func TestMarkRequestOnce(t *testing.T) {
	room, err := NewRoomSession("room-test")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	key := "room-test|A|submit_move|req-1"
	if ok := room.MarkRequestOnce(key); !ok {
		t.Fatalf("first mark should succeed")
	}
	if ok := room.MarkRequestOnce(key); ok {
		t.Fatalf("second mark should be rejected as duplicate")
	}
}

// TestDisconnectFreezePreservesMainClockInSnapshot exposes frozen main turn time during reconnect grace.
func TestDisconnectFreezePreservesMainClockInSnapshot(t *testing.T) {
	room, err := NewRoomSession("room-freeze-main")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	syncDisconnectBudgetForTest(room, time.Minute)
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	remaining := 40 * time.Second
	room.stateM.Lock()
	room.turnDeadline = time.Now().UTC().Add(remaining)
	room.turnDeadlineFor = gameplay.PlayerA
	room.stateM.Unlock()

	room.HandlePlayerDisconnect(gameplay.PlayerA)

	snap := room.SnapshotForPlayerSafe(gameplay.PlayerB)
	if snap.TurnMainDeadlineUnixMs == 0 || snap.TurnMainPausedRemainingMs != 0 {
		t.Fatalf("expected TurnMainDeadlineUnixMs during grace, got deadline=%d paused=%d",
			snap.TurnMainDeadlineUnixMs, snap.TurnMainPausedRemainingMs)
	}
	gotRem := time.Until(time.UnixMilli(snap.TurnMainDeadlineUnixMs))
	slack := 2500 * time.Millisecond
	if gotRem < remaining-slack || gotRem > remaining+slack {
		t.Fatalf("frozen main remaining want ~%v got %v", remaining, gotRem)
	}

	room.RegisterPlayerConnection(gameplay.PlayerA)
	snap2 := room.SnapshotForPlayerSafe(gameplay.PlayerB)
	if snap2.TurnMainDeadlineUnixMs == 0 {
		t.Fatalf("expected resumed TurnMainDeadlineUnixMs after reconnect")
	}
	gotRem2 := time.Until(time.UnixMilli(snap2.TurnMainDeadlineUnixMs))
	if gotRem2 < remaining-slack || gotRem2 > remaining+slack {
		t.Fatalf("resumed main remaining want ~%v got %v", remaining, gotRem2)
	}
}

// TestDisconnectFreezePreservesPausedMainTurn mirrors a main turn paused for capture/ignite reaction during opponent disconnect.
func TestDisconnectFreezePreservesPausedMainTurn(t *testing.T) {
	room, err := NewRoomSession("room-freeze-paused")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	syncDisconnectBudgetForTest(room, time.Minute)
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	room.stateM.Lock()
	room.pausedTurnRemaining = 30 * time.Second
	room.turnDeadlineFor = gameplay.PlayerA
	room.turnDeadline = time.Time{}
	room.stateM.Unlock()

	room.HandlePlayerDisconnect(gameplay.PlayerA)

	snap := room.SnapshotForPlayerSafe(gameplay.PlayerB)
	if snap.TurnMainPausedRemainingMs != 30000 || snap.TurnMainDeadlineUnixMs != 0 {
		t.Fatalf("expected paused main 30000ms in snapshot, got deadline=%d paused=%d",
			snap.TurnMainDeadlineUnixMs, snap.TurnMainPausedRemainingMs)
	}

	room.RegisterPlayerConnection(gameplay.PlayerA)
	snap2 := room.SnapshotForPlayerSafe(gameplay.PlayerB)
	if snap2.TurnMainPausedRemainingMs != 30000 {
		t.Fatalf("want resumed paused 30000ms, got %d", snap2.TurnMainPausedRemainingMs)
	}
}

// TestJoinSeatBlockedDuringReconnectGrace ensures a third party cannot take the vacant seat while the grace timer runs.
func TestJoinSeatBlockedDuringReconnectGrace(t *testing.T) {
	room, err := NewRoomSession("room-join-grace")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	syncDisconnectBudgetForTest(room, time.Minute)
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)
	room.HandlePlayerDisconnect(gameplay.PlayerA)

	_, err = room.joinSeat(gameplay.PlayerA)
	if err == nil {
		t.Fatalf("expected joinSeat to reject while reconnect timer is pending for A")
	}
}

// TestDisconnectTimeoutGivesWinToConnectedPlayer validates single disconnect timeout rule.
func TestDisconnectTimeoutGivesWinToConnectedPlayer(t *testing.T) {
	room, err := NewRoomSession("room-disconnect")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	syncDisconnectBudgetForTest(room, 25*time.Millisecond)
	room.DisconnectMinWinDelay = 5 * time.Millisecond
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	room.HandlePlayerDisconnect(gameplay.PlayerA)
	time.Sleep(50 * time.Millisecond)

	s := room.SnapshotSafe()
	if !s.MatchEnded || s.Winner != string(gameplay.PlayerB) || s.EndReason != "disconnect_timeout" {
		t.Fatalf("expected disconnect timeout win for B, got %+v", s)
	}
}

// TestBothDisconnectedCancelsMatch validates no-winner cancellation when both leave.
func TestBothDisconnectedCancelsMatch(t *testing.T) {
	room, err := NewRoomSession("room-cancel")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	syncDisconnectBudgetForTest(room, 25*time.Millisecond)
	room.DisconnectMinWinDelay = 5 * time.Millisecond
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	room.HandlePlayerDisconnect(gameplay.PlayerA)
	room.HandlePlayerDisconnect(gameplay.PlayerB)
	time.Sleep(25 * time.Millisecond)

	s := room.SnapshotSafe()
	if !s.MatchEnded || s.Winner != "" || s.EndReason != "both_disconnected_cancelled" {
		t.Fatalf("expected cancelled match with no winner, got %+v", s)
	}
}

// TestHandlePlayerLeaveGivesImmediateWin validates explicit room leave immediate winner rule.
func TestHandlePlayerLeaveGivesImmediateWin(t *testing.T) {
	room, err := NewRoomSession("room-leave")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	room.HandlePlayerLeave(gameplay.PlayerA)
	s := room.SnapshotSafe()
	if !s.MatchEnded || s.Winner != string(gameplay.PlayerB) || s.EndReason != "left_room" {
		t.Fatalf("expected immediate leave win for B, got %+v", s)
	}
}

// TestHandlePlayerDisconnectPrefersCheckmateOverCancel ends by board when both leave after mate.
func TestHandlePlayerDisconnectPrefersCheckmateOverCancel(t *testing.T) {
	room, err := NewRoomSession("room-disconnect-mate")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	g := chess.NewEmptyGame(chess.White)
	g.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White})
	g.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.King, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	room.Engine.Chess = g
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)
	room.HandlePlayerDisconnect(gameplay.PlayerA)
	room.HandlePlayerDisconnect(gameplay.PlayerB)
	s := room.SnapshotSafe()
	if !s.MatchEnded || s.EndReason != "checkmate" || s.Winner != string(gameplay.PlayerB) {
		t.Fatalf("expected checkmate for B, got %+v", s)
	}
}

// TestSnapshotUpgradesBothDisconnectedToCheckmate replaces abandonment with board truth.
func TestSnapshotUpgradesBothDisconnectedToCheckmate(t *testing.T) {
	room, err := NewRoomSession("room-upgrade-end")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	g := chess.NewEmptyGame(chess.White)
	g.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White})
	g.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.King, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	room.Engine.Chess = g
	room.stateM.Lock()
	room.matchEnded = true
	room.endReason = "both_disconnected_cancelled"
	room.winner = ""
	room.stateM.Unlock()

	s := room.SnapshotSafe()
	if !s.MatchEnded || s.EndReason != "checkmate" || s.Winner != string(gameplay.PlayerB) {
		t.Fatalf("expected checkmate win for B after upgrade, got %+v", s)
	}
}

// TestEvaluateMatchOutcomeCheckmate marks match end and winner from board state.
func TestEvaluateMatchOutcomeCheckmate(t *testing.T) {
	room, err := NewRoomSession("room-checkmate")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	g := chess.NewEmptyGame(chess.White)
	g.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White})
	g.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.King, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	g.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	room.Engine.Chess = g

	room.EvaluateMatchOutcome()
	s := room.SnapshotSafe()
	if !s.MatchEnded || s.Winner != string(gameplay.PlayerB) || s.EndReason != "checkmate" {
		t.Fatalf("expected checkmate win for B, got %+v", s)
	}
}

// TestResolveReactionTimeoutIfExpired auto-resolves capture window after timeout.
func TestResolveReactionTimeoutIfExpired(t *testing.T) {
	room, err := NewRoomSession("room-reaction-timeout")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	room.Engine.Chess = board
	room.reactionTimeout = 5 * time.Millisecond
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	if err := room.Engine.SubmitMove(gameplay.PlayerA, chess.Move{
		From: chess.Pos{Row: 6, Col: 4},
		To:   chess.Pos{Row: 5, Col: 5},
	}); err != nil {
		t.Fatalf("submit move failed: %v", err)
	}

	// First pass starts timeout window, second pass after sleep should resolve it.
	resolved, err := room.ResolveReactionTimeoutIfExpired(time.Now())
	if err != nil || resolved {
		t.Fatalf("expected unresolved on first pass, resolved=%v err=%v", resolved, err)
	}
	time.Sleep(8 * time.Millisecond)
	resolved, err = room.ResolveReactionTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("timeout resolve failed: %v", err)
	}
	if !resolved {
		t.Fatalf("expected reaction timeout to auto-resolve")
	}
	if room.Engine.Chess.PieceAt(chess.Pos{Row: 5, Col: 5}).Color != chess.White {
		t.Fatalf("expected pending capture applied after timeout")
	}
}

// TestCaptureReactionSkippedWhenOpponentReactionOff ensures capture applies immediately without an open window.
func TestCaptureReactionSkippedWhenOpponentReactionOff(t *testing.T) {
	room, err := NewRoomSession("room-cap-off")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	room.Engine.Chess = board
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.Engine.State.CurrentTurn = gameplay.PlayerA
	room.reactionModeByPlayer[gameplay.PlayerB] = ReactionModeOff

	if err := room.Execute(func() error {
		if err := room.Engine.SubmitMove(gameplay.PlayerA, chess.Move{
			From: chess.Pos{Row: 6, Col: 4},
			To:   chess.Pos{Row: 5, Col: 5},
		}); err != nil {
			return err
		}
		return room.maybeAutoResolveCaptureReactionUnsafe()
	}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if ok && rw.Open {
		t.Fatalf("expected reaction window closed, got %+v", rw)
	}
	if room.Engine.Chess.PieceAt(chess.Pos{Row: 5, Col: 5}).Color != chess.White {
		t.Fatalf("expected capture applied immediately")
	}
}

// TestIgniteReactionSkippedWhenOpponentReactionOff closes ignite_reaction immediately when the responder uses OFF.
func TestIgniteReactionSkippedWhenOpponentReactionOff(t *testing.T) {
	room, err := NewRoomSession("room-ignite-off")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	s := room.Engine.State
	s.MulliganPhaseActive = false
	s.Started = true
	s.CurrentTurn = gameplay.PlayerA
	dt := gameplay.CardInstance{InstanceID: "dt1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	s.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{dt}
	s.Players[gameplay.PlayerA].Mana = 10
	s.Players[gameplay.PlayerB].Mana = 10
	room.reactionModeByPlayer[gameplay.PlayerB] = ReactionModeOff
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	if err := room.Execute(func() error {
		if err := room.Engine.ActivateCard(gameplay.PlayerA, 0); err != nil {
			return err
		}
		if err := room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(time.Now())
		return nil
	}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if ok && rw.Open {
		t.Fatalf("expected ignite reaction window closed, got %+v", rw)
	}
	if !s.IgnitionSlot.Occupied {
		t.Fatalf("expected double-turn to stay in ignition while burning (ignition 1)")
	}
	if s.IgnitionSlot.TurnsRemaining != 1 {
		t.Fatalf("expected 1 burn turn remaining, got %d", s.IgnitionSlot.TurnsRemaining)
	}
}

// TestIgniteReactionWindowWhenOpponentReactionOn keeps ignite_reaction open for responder.
func TestIgniteReactionWindowWhenOpponentReactionOn(t *testing.T) {
	room, err := NewRoomSession("room-ignite-on")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	s := room.Engine.State
	s.MulliganPhaseActive = false
	s.Started = true
	s.CurrentTurn = gameplay.PlayerA
	dt := gameplay.CardInstance{InstanceID: "dt1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	s.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{dt}
	s.Players[gameplay.PlayerA].Mana = 10
	s.Players[gameplay.PlayerB].Mana = 10
	room.reactionModeByPlayer[gameplay.PlayerB] = ReactionModeOn
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	if err := room.Execute(func() error {
		if err := room.Engine.ActivateCard(gameplay.PlayerA, 0); err != nil {
			return err
		}
		if err := room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(time.Now())
		return nil
	}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "ignite_reaction" {
		t.Fatalf("expected open ignite_reaction window, got ok=%v rw=%+v", ok, rw)
	}
	if room.pausedTurnRemaining <= 0 || !room.turnDeadline.IsZero() {
		t.Fatalf("expected main turn clock paused for ignite reaction, paused=%v deadline=%v", room.pausedTurnRemaining, room.turnDeadline)
	}
}

// TestResolveIgniteReactionPathKeepsDelayedBurn mirrors handleResolveReactions + resume after a
// Power card with ignition > 0: the main card must stay in the ignition slot until burn completes.
func TestResolveIgniteReactionPathKeepsDelayedBurn(t *testing.T) {
	room, err := NewRoomSession("room-resolve-delayed-ignite")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	s := room.Engine.State
	s.MulliganPhaseActive = false
	s.Started = true
	s.CurrentTurn = gameplay.PlayerB
	eg := gameplay.CardInstance{InstanceID: "e1", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2}
	s.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{eg}
	s.Players[gameplay.PlayerB].Mana = 10
	s.Players[gameplay.PlayerA].Mana = 10
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	now := time.Now()
	if err := room.Execute(func() error {
		room.resetTurnDeadlineUnsafe(now)
		if err := room.Engine.ActivateCard(gameplay.PlayerB, 0); err != nil {
			return err
		}
		if err := room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(now)
		return nil
	}); err != nil {
		t.Fatalf("activate phase: %v", err)
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "ignite_reaction" {
		t.Fatalf("expected open ignite_reaction window, got ok=%v rw=%+v", ok, rw)
	}

	if err := room.Execute(func() error {
		if err := room.Engine.ResolveReactionStack(); err != nil {
			return err
		}
		room.reactionDeadline = time.Time{}
		room.resumeMainTurnAfterReactionUnsafe(time.Now())
		return nil
	}); err != nil {
		t.Fatalf("resolve phase: %v", err)
	}

	if !s.IgnitionSlot.Occupied {
		t.Fatal("delayed ignition card should remain in slot after empty resolve_reactions-style path")
	}
	if s.IgnitionSlot.TurnsRemaining != 1 {
		t.Fatalf("expected 1 burn turn remaining, got %d", s.IgnitionSlot.TurnsRemaining)
	}
	if len(s.Players[gameplay.PlayerB].Cooldowns) != 0 {
		t.Fatal("card must not enter cooldown before ignition burn finishes")
	}
}

// TestAutoFinalizeIgniteWhenNextCannotExtend closes the ignite chain without waiting for timeout
// when the seat that must respond has no legal Retribution follow-up.
func TestAutoFinalizeIgniteWhenNextCannotExtend(t *testing.T) {
	room, err := NewRoomSession("room-auto-final-ignite")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	s := room.Engine.State
	s.MulliganPhaseActive = false
	s.Started = true
	s.CurrentTurn = gameplay.PlayerA
	kt := gameplay.CardInstance{InstanceID: "k1", CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2}
	ret := gameplay.CardInstance{InstanceID: "r1", CardID: "retaliate", ManaCost: 2, Ignition: 0, Cooldown: 9}
	ext := gameplay.CardInstance{InstanceID: "e1", CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}
	s.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{kt, ext}
	s.Players[gameplay.PlayerA].Mana = 10
	s.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{ret}
	s.Players[gameplay.PlayerB].Mana = 10
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	now := time.Now()
	if err := room.Execute(func() error {
		room.resetTurnDeadlineUnsafe(now)
		if err := room.Engine.ActivateCard(gameplay.PlayerA, 0); err != nil {
			return err
		}
		if err := room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(now)
		return nil
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	if err := room.Execute(func() error {
		if err := room.Engine.QueueReactionCard(gameplay.PlayerB, 0, match.EffectTarget{}); err != nil {
			return err
		}
		if err := room.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if ok && rw.Open {
		t.Fatalf("expected ignite chain auto-finalized when A cannot play Retribution, window still open %+v", rw)
	}
}

// TestIgniteChainFinalizesOnZeroReactionDeadline ensures the timeout loop re-runs finalize when the
// first queued reaction cleared the deadline but finalize had not run yet (regression: full reactionTimeout wait and missing client resolve animation).
func TestIgniteChainFinalizesOnZeroReactionDeadline(t *testing.T) {
	room, err := NewRoomSession("room-ignite-zero-deadline-finalize")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	s := room.Engine.State
	s.MulliganPhaseActive = false
	s.Started = true
	s.CurrentTurn = gameplay.PlayerB
	eg := gameplay.CardInstance{InstanceID: "e1", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2}
	ret := gameplay.CardInstance{InstanceID: "r1", CardID: "retaliate", ManaCost: 2, Ignition: 0, Cooldown: 9}
	s.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{eg}
	s.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ret}
	s.Players[gameplay.PlayerA].Mana = 10
	s.Players[gameplay.PlayerB].Mana = 10
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	now := time.Now()
	if err := room.Execute(func() error {
		room.resetTurnDeadlineUnsafe(now)
		if err := room.Engine.ActivateCard(gameplay.PlayerB, 0); err != nil {
			return err
		}
		if err := room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(now)
		return nil
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	if err := room.Execute(func() error {
		if err := room.Engine.QueueReactionCard(gameplay.PlayerA, 0, match.EffectTarget{}); err != nil {
			return err
		}
		room.noteReactionChainStartedUnsafe()
		// Skip maybeAutoFinalizeIgniteChainIfStuckUnsafe — the timeout tick must still close the chain.
		return nil
	}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	resolved, err := room.ResolveReactionTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("ResolveReactionTimeoutIfExpired: %v", err)
	}
	if !resolved {
		t.Fatal("expected ignite chain finalized on zero reaction deadline without waiting wall clock")
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if ok && rw.Open {
		t.Fatalf("expected reaction window closed, got %+v", rw)
	}
	if !s.IgnitionSlot.Occupied {
		t.Fatal("delayed ignition card should remain in slot after reaction resolve")
	}
	if s.IgnitionSlot.TurnsRemaining != 1 {
		t.Fatalf("expected 1 burn turn remaining on energy-gain, got %d", s.IgnitionSlot.TurnsRemaining)
	}
}

// TestCaptureReactionWindowWhenOpponentReactionOn keeps the reaction window open until resolve.
func TestCaptureReactionWindowWhenOpponentReactionOn(t *testing.T) {
	room, err := NewRoomSession("room-cap-on")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	room.Engine.Chess = board
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.Engine.State.CurrentTurn = gameplay.PlayerA
	room.reactionModeByPlayer[gameplay.PlayerB] = ReactionModeOn
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	if err := room.Execute(func() error {
		if err := room.Engine.SubmitMove(gameplay.PlayerA, chess.Move{
			From: chess.Pos{Row: 6, Col: 4},
			To:   chess.Pos{Row: 5, Col: 5},
		}); err != nil {
			return err
		}
		if err := room.maybeAutoResolveCaptureReactionUnsafe(); err != nil {
			return err
		}
		room.pauseMainTurnIfReactionWindowOpenUnsafe(time.Now())
		return nil
	}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	rw, _, ok := room.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "capture_attempt" {
		t.Fatalf("expected open capture_attempt window, got ok=%v rw=%+v", ok, rw)
	}
	if room.pausedTurnRemaining <= 0 || !room.turnDeadline.IsZero() {
		t.Fatalf("expected main turn clock paused for capture reaction, paused=%v deadline=%v", room.pausedTurnRemaining, room.turnDeadline)
	}
}

// TestResolveTurnTimeoutIfExpiredAddsStrikeAndPassesTurn validates timeout strike and turn handoff.
func TestResolveTurnTimeoutIfExpiredAddsStrikeAndPassesTurn(t *testing.T) {
	room, err := NewRoomSession("room-turn-timeout")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.Engine.State.TurnSeconds = 1
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	// First call initializes deadline.
	resolved, err := room.ResolveTurnTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("init timeout failed: %v", err)
	}
	if resolved {
		t.Fatalf("first pass should not resolve timeout")
	}

	time.Sleep(1100 * time.Millisecond)
	resolved, err = room.ResolveTurnTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("timeout resolve failed: %v", err)
	}
	if !resolved {
		t.Fatalf("expected timeout resolution")
	}
	s := room.SnapshotSafe()
	if s.TurnPlayer != string(gameplay.PlayerB) {
		t.Fatalf("expected turn passed to B, got %+v", s)
	}
	strikesA := -1
	for _, p := range s.Players {
		if p.PlayerID == string(gameplay.PlayerA) {
			strikesA = p.Strikes
			break
		}
	}
	if strikesA != 1 {
		t.Fatalf("expected A to have 1 strike, got %+v", s.Players)
	}
}

// TestResolveTurnTimeoutIfExpiredLosesOnThirdStrike validates strike-limit end condition.
func TestResolveTurnTimeoutIfExpiredLosesOnThirdStrike(t *testing.T) {
	room, err := NewRoomSession("room-turn-timeout-loss")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.Engine.State.Players[gameplay.PlayerA].Strikes = 2
	room.Engine.State.TurnSeconds = 1
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	_, _ = room.ResolveTurnTimeoutIfExpired(time.Now())
	time.Sleep(1100 * time.Millisecond)
	resolved, err := room.ResolveTurnTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("timeout loss resolve failed: %v", err)
	}
	if !resolved {
		t.Fatalf("expected timeout loss resolution")
	}
	s := room.SnapshotSafe()
	if !s.MatchEnded || s.Winner != string(gameplay.PlayerB) || s.EndReason != "strike_limit" {
		t.Fatalf("expected strike_limit win for B, got %+v", s)
	}
}

// TestResolveMulliganTimeoutIfExpiredAutoKeeps ensures the mulligan window auto-confirms empty returns and starts the match.
func TestResolveMulliganTimeoutIfExpiredAutoKeeps(t *testing.T) {
	room, err := NewRoomSession("room-mulligan-timeout")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.stateM.Lock()
	room.Engine.State.MulliganPhaseActive = true
	room.Engine.State.MulliganConfirmed = map[gameplay.PlayerID]bool{
		gameplay.PlayerA: false,
		gameplay.PlayerB: false,
	}
	room.Engine.State.MulliganReturnedCount = map[gameplay.PlayerID]int{
		gameplay.PlayerA: -1,
		gameplay.PlayerB: -1,
	}
	room.Engine.State.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{}
	room.Engine.State.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	room.mulliganDeadline = time.Now().Add(-time.Second)
	room.stateM.Unlock()

	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	resolved, err := room.ResolveMulliganTimeoutIfExpired(time.Now())
	if err != nil {
		t.Fatalf("mulligan timeout: %v", err)
	}
	if !resolved {
		t.Fatalf("expected mulligan auto resolution")
	}
	s := room.SnapshotSafe()
	if s.MulliganPhaseActive {
		t.Fatalf("expected mulligan phase ended")
	}
	if s.TurnPlayer != string(gameplay.PlayerA) {
		t.Fatalf("expected first chess turn for white (A), got %s", s.TurnPlayer)
	}
	if s.TurnMainDeadlineUnixMs == 0 {
		t.Fatalf("expected TurnMainDeadlineUnixMs after mulligan resolution, got 0")
	}
}

// TestSetDebugPauseResumePreservesPausedTurnRemaining ensures debug pause does not inflate the
// frozen main-turn slice (that slice must stay capped by the pre-pause budget).
func TestSetDebugPauseResumePreservesPausedTurnRemaining(t *testing.T) {
	room, err := NewRoomSession("room-debug-pause-paused-main")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)
	room.stateM.Lock()
	room.Engine.State.MulliganPhaseActive = false
	room.Engine.State.Started = true
	room.Engine.State.CurrentTurn = gameplay.PlayerA
	room.pausedTurnRemaining = 25 * time.Second
	room.turnDeadlineFor = gameplay.PlayerA
	room.turnDeadline = time.Time{}
	t0 := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	room.setDebugPauseUnsafe(true, t0)
	room.setDebugPauseUnsafe(false, t0.Add(10*time.Second))
	got := room.pausedTurnRemaining
	room.stateM.Unlock()
	want := 25 * time.Second
	if got != want {
		t.Fatalf("pausedTurnRemaining want %v got %v", want, got)
	}
}

// TestRequestRematchSwapsSides ensures accepted rematch swaps player colors.
func TestRequestRematchSwapsSides(t *testing.T) {
	room, err := NewRoomSession("room-rematch-swap")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.stateM.Lock()
	room.matchEnded = true
	room.connectedByPlayer[gameplay.PlayerA] = 1
	room.connectedByPlayer[gameplay.PlayerB] = 1
	clientA := &Client{playerID: gameplay.PlayerA}
	clientB := &Client{playerID: gameplay.PlayerB}
	room.clients[clientA] = struct{}{}
	room.clients[clientB] = struct{}{}
	room.Players["conn-a"] = gameplay.PlayerA
	room.Players["conn-b"] = gameplay.PlayerB
	room.SetPlayerDisplayNameUnsafe(gameplay.PlayerA, "alice")
	room.SetPlayerDisplayNameUnsafe(gameplay.PlayerB, "bob")
	room.stateM.Unlock()

	accepted, err := room.RequestRematch(gameplay.PlayerA)
	if err != nil {
		t.Fatalf("request rematch A failed: %v", err)
	}
	if accepted {
		t.Fatalf("expected rematch pending after first vote")
	}
	accepted, err = room.RequestRematch(gameplay.PlayerB)
	if err != nil {
		t.Fatalf("request rematch B failed: %v", err)
	}
	if !accepted {
		t.Fatalf("expected rematch accepted after second vote")
	}

	if clientA.playerID != gameplay.PlayerB {
		t.Fatalf("expected former A client to become B, got %s", clientA.playerID)
	}
	if clientB.playerID != gameplay.PlayerA {
		t.Fatalf("expected former B client to become A, got %s", clientB.playerID)
	}
	if room.Players["conn-a"] != gameplay.PlayerB || room.Players["conn-b"] != gameplay.PlayerA {
		t.Fatalf("expected player address map swapped, got %+v", room.Players)
	}
	snap := room.SnapshotSafe()
	if snap.PlayerAName != "bob" || snap.PlayerBName != "alice" {
		t.Fatalf("expected display names swapped with seats, got A=%q B=%q", snap.PlayerAName, snap.PlayerBName)
	}
}

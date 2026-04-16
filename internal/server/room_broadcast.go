package server

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// broadcastActivateCardEvents sends server→client activate_card frames (effect resolution) to every client.
func (r *RoomSession) broadcastActivateCardEvents(evts []match.ActivationFXEvent) {
	if len(evts) == 0 {
		return
	}
	transport := NewWSTransport()
	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for _, ev := range evts {
		env := transport.BuildActivateCardEnvelope(ev)
		for c := range r.clients {
			_ = c.send(env)
		}
	}
}

// BroadcastSnapshot sends each connected client a snapshot tailored to their player seat.
// Clients with no assigned seat receive a generic (no hand) snapshot.
func (r *RoomSession) BroadcastSnapshot() {
	r.stateM.Lock()
	evts := r.Engine.PullActivationFXEvents()
	snapA := r.SnapshotForPlayer(gameplay.PlayerA)
	snapB := r.SnapshotForPlayer(gameplay.PlayerB)
	snapGeneric := r.SnapshotForPlayer("")
	r.stateM.Unlock()

	r.broadcastActivateCardEvents(evts)

	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		switch c.playerID {
		case gameplay.PlayerA:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapA)})
		case gameplay.PlayerB:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapB)})
		default:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapGeneric)})
		}
	}
}

// broadcastPrecomputedSnapshots sends pre-built player snapshots to all clients without pulling
// activation FX events. Used to broadcast an intermediate state (staged reaction card) before
// auto-finalization resolves the stack, so clients animate the card in the ignition zone.
func (r *RoomSession) broadcastPrecomputedSnapshots(snapA, snapB, snapGeneric StateSnapshotPayload) {
	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		switch c.playerID {
		case gameplay.PlayerA:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapA)})
		case gameplay.PlayerB:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapB)})
		default:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapGeneric)})
		}
	}
}

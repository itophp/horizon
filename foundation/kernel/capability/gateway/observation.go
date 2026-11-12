package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strconv"

	canonical "horizon/foundation/kernel/memory"
	"horizon/foundation/system/identity"
)

// observeDenied records an authorization decision without input, credentials,
// error text or a fabricated invocation ID. It never affects the decision.
func (g *Gateway) observeDenied(p *identity.Principal, request Request, capability, reason string) {
	if g.observations == nil {
		return
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		slog.Warn("memory observation dropped", "code", "id_unavailable")
		return
	}
	g.observations.Submit(canonical.Envelope{EventID: "denied:" + hex.EncodeToString(raw[:]), EventType: "capability.invocation.denied",
		TenantID: p.TenantID, Actor: canonical.Actor{ID: strconv.FormatInt(p.Account.ID, 10), Type: "user"}, DepartmentID: p.Account.DepartmentID,
		OccurredAt: g.now(), Source: canonical.Source{Type: "capability", ID: capability}, Result: "denied", CorrelationID: request.CorrelationID,
		CausationID: request.CausationID, Depth: request.TriggerDepth, Sensitivity: "internal", Payload: map[string]string{"reason": reason}})
}

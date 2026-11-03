package contracts

import "strings"

type Kind string
type Layer string
type Effect string

const (
	KindService        Kind  = "service"
	KindSQL            Kind  = "sql"
	KindCommand        Kind  = "command"
	KindExternal       Kind  = "external"
	KindEvent          Kind  = "event"
	LayerAtomic        Layer = "atomic"
	LayerOrchestration Layer = "orchestration"
	// None means no business side effect. A committed transaction is not reversible.
	EffectNone         Effect = "none"
	EffectCompensable  Effect = "compensable"
	EffectIrreversible Effect = "irreversible"
)

// Normalized applies conservative defaults. Writes describe transitive business
// impact too; Layer never supplies a default and does not assert SQL atomicity.
func (d Definition) Normalized() Definition {
	if d.Kind == "" {
		switch strings.Split(d.Executor, ":")[0] {
		case "sql":
			d.Kind = KindSQL
		case "cmd":
			d.Kind = KindCommand
		case "ext":
			d.Kind = KindExternal
		case "event":
			d.Kind = KindEvent
		default:
			d.Kind = KindService
		}
	}
	if d.Reads == nil {
		d.Reads = append([]string{}, d.EntityNames()...)
	}
	if d.Writes == nil && !d.ReadOnly {
		d.Writes = append([]string{}, d.EntityNames()...)
	}
	if d.Effect == "" {
		if d.ReadOnly {
			d.Effect = EffectNone
		} else {
			d.Effect = EffectIrreversible
		}
	}
	if d.ReadOnly {
		d.RetrySafe = true
	}
	return d
}

func (d Definition) taxonomyProblems() []string {
	var out []string
	if d.Layer != LayerAtomic && d.Layer != LayerOrchestration {
		out = append(out, "layer must be atomic or orchestration")
	}
	switch d.Kind {
	case "", KindService, KindSQL, KindCommand, KindExternal, KindEvent:
	default:
		out = append(out, "invalid kind")
	}
	switch d.Effect {
	case "", EffectNone, EffectCompensable, EffectIrreversible:
	default:
		out = append(out, "invalid effect")
	}
	if d.ReadOnly && len(d.Writes) > 0 {
		out = append(out, "read-only capability declares writes")
	}
	if !d.ReadOnly && d.Effect == EffectNone {
		out = append(out, "write capability cannot declare effect none")
	}
	return out
}

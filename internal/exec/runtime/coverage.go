package runtime

import (
	"cmp"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Coverage is what the runs of an exploration, or the schedules of a check,
// entered and took: every state entered and every transition taken by any run,
// not only by the witness of each outcome, with the number of runs that did so.
type Coverage struct {
	States      []CoverageEntry
	Transitions []CoverageEntry
}

// CoverageEntry is one state entered or one transition taken. Path qualifies it
// by the instance that ran the machine — `Cov::Sys#1.m.run: viaA`,
// `Cov::Sys#1.m.run: s0 -> viaB` — so one definition instantiated many times is
// told apart; Declaration qualifies it by the declaration — `Cov::M::run::viaA`,
// `Cov::M::run::s0 -> viaB`. Runs counts the runs that entered or took it.
type CoverageEntry struct {
	Path        string
	Declaration string
	Runs        int
}

// Empty reports whether nothing was entered or taken.
func (c Coverage) Empty() bool {
	return len(c.States) == 0 && len(c.Transitions) == 0
}

// merge adds one run's coverage to the total, counting the run for every entry
// it holds.
func (c *Coverage) merge(run Coverage) {
	c.States = mergeCoverageEntries(c.States, run.States)
	c.Transitions = mergeCoverageEntries(c.Transitions, run.Transitions)
}

// mergeCoverageEntries adds the entries of one run to the sorted total, both
// lists sorted by path then declaration, and returns the total still sorted.
func mergeCoverageEntries(into, run []CoverageEntry) []CoverageEntry {
	if len(run) == 0 {
		return into
	}
	out := make([]CoverageEntry, 0, len(into)+len(run))
	i, j := 0, 0
	for i < len(into) && j < len(run) {
		switch c := compareCoverageEntries(into[i], run[j]); {
		case c < 0:
			out = append(out, into[i])
			i++
		case c > 0:
			out = append(out, run[j])
			j++
		default:
			entry := into[i]
			entry.Runs += run[j].Runs
			out = append(out, entry)
			i++
			j++
		}
	}
	out = append(out, into[i:]...)
	return append(out, run[j:]...)
}

func compareCoverageEntries(a, b CoverageEntry) int {
	return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Declaration, b.Declaration))
}

// coverageKey identifies a state entered, or with `to` set a transition taken,
// by the object running the machine, the machine and the state path, so a run
// notes each once however often it enters or takes it.
type coverageKey struct {
	object    int64
	behavior  *symbols.Symbol
	state, to string
}

// noteStateEntered records that the object's machine entered the state at the
// path, independently of tracing; the spelling is settled when first noted and
// the key kept cheap, since a run enters states far more often than it covers
// new ones.
func (ctx *Context) noteStateEntered(object *Instance, machine *symbols.Symbol, path string) {
	ctx.noteCoverage(object, machine, path, "")
}

// noteTransitionTaken records that the object's machine took the transition
// between the named states, as the trace spells them.
func (ctx *Context) noteTransitionTaken(object *Instance, machine *symbols.Symbol, from, to string) {
	if to == "" {
		return
	}
	ctx.noteCoverage(object, machine, from, to)
}

func (ctx *Context) noteCoverage(object *Instance, machine *symbols.Symbol, state, to string) {
	key := coverageKey{behavior: machine, state: state, to: to}
	if object != nil {
		key.object = object.ID
	}
	if _, noted := ctx.coverage[key]; noted {
		return
	}
	if ctx.coverage == nil {
		ctx.coverage = make(map[coverageKey]CoverageEntry)
	}
	ctx.coverage[key] = ctx.spellCoverage(object, machine, state, to)
}

// spellCoverage names an entry by the instance path of the object running the
// machine and by the machine's declaration.
func (ctx *Context) spellCoverage(object *Instance, machine *symbols.Symbol, state, to string) CoverageEntry {
	what := state
	declared := strings.ReplaceAll(state, ".", "::")
	if to != "" {
		what = state + " -> " + to
		declared = what
	}
	var owner, declaration string
	if object != nil {
		owner = ctx.objectPath(object.ID)
	}
	if name := machineName(object, machine); name != "" {
		if owner != "" {
			owner += "."
		}
		owner += name
	}
	if machine != nil {
		declaration = ctx.QualifiedName(machine) + "::" + declared
	} else {
		declaration = declared
	}
	return CoverageEntry{Path: owner + ": " + what, Declaration: declaration, Runs: 1}
}

// runCoverage lists what the run entered and took so far, in canonical order,
// each counted for this one run.
func (ctx *Context) runCoverage() Coverage {
	var out Coverage
	for key, entry := range ctx.coverage {
		if key.to == "" {
			out.States = append(out.States, entry)
		} else {
			out.Transitions = append(out.Transitions, entry)
		}
	}
	slices.SortFunc(out.States, compareCoverageEntries)
	slices.SortFunc(out.Transitions, compareCoverageEntries)
	return out
}

// machineName is the name the object gives the behavior it runs — the usage
// name of an exhibited state machine — falling back to the behavior's own.
func machineName(object *Instance, behavior *symbols.Symbol) string {
	if behavior == nil {
		return ""
	}
	if object != nil {
		for _, b := range object.Behaviors() {
			if b.Symbol == behavior || (b.State != nil && b.State.StateMachineSymbol() == behavior) {
				if b.Name != "" {
					return b.Name
				}
				break
			}
		}
	}
	return behavior.Name
}

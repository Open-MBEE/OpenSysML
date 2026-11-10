package runtime

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Coverage is what the runs of an exploration, or the schedules of a check,
// entered and took: every state entered and every transition taken by any run,
// not only by the witness of each outcome, with the number of runs that did so.
type Coverage struct {
	States      []CoverageEntry
	Transitions []CoverageEntry
}

// CoverageEntry is one state entered or transition taken, under the instance that ran
// the machine (`Cov::Sys#1.m.run: s0 -> viaB`) and its declaration (`Cov::M::run::s0 -> viaB`).
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

// coverageKey is a state entered, or with `to` set a transition taken, by the object
// running the machine, the machine and the states' qualified paths: noted once per run.
type coverageKey struct {
	object    int64
	behavior  *symbols.Symbol
	state, to string
}

// noteStateEntered records that the object's machine entered the state at the path,
// its names outermost first, independently of tracing; its spelling is settled once.
func (ctx *Context) noteStateEntered(object *Instance, machine *symbols.Symbol, path []string) {
	ctx.noteCoverage(object, machine, path, nil)
}

// noteTransitionTaken records that the object's machine took the transition
// between the states at the two paths, each its names outermost first.
func (ctx *Context) noteTransitionTaken(object *Instance, machine *symbols.Symbol, from, to []string) {
	if len(to) == 0 {
		return
	}
	ctx.noteCoverage(object, machine, from, to)
}

func (ctx *Context) noteCoverage(object *Instance, machine *symbols.Symbol, state, to []string) {
	// A probe's work is rolled back: what it entered, the run never did.
	if ctx.probes > 0 {
		return
	}
	key := coverageKey{behavior: machine, state: source.QualifiedNameOf(state), to: source.QualifiedNameOf(to)}
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

// spellCoverage names an entry by the object's instance path with the names as
// written, and by the machine's declaration with each name quoted on its own.
func (ctx *Context) spellCoverage(object *Instance, machine *symbols.Symbol, state, to []string) CoverageEntry {
	what, declared := strings.Join(state, "."), source.QualifiedNameOf(state)
	if len(to) > 0 {
		what += " -> " + strings.Join(to, ".")
		declared += " -> " + source.QualifiedNameOf(to)
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

// Lines renders the coverage for a text verdict: a summary, then one line per entry
// with its instance path, declaration and how many runs (or `did`, e.g. schedules) did so.
func (c Coverage) Lines(did string) []string {
	if c.Empty() {
		return nil
	}
	lines := []string{fmt.Sprintf("coverage over all %ss: %s entered, %s taken", did,
		coverageCount(len(c.States), "state"), coverageCount(len(c.Transitions), "transition"))}
	for _, entry := range c.States {
		lines = append(lines, "  state "+entry.line(did))
	}
	for _, entry := range c.Transitions {
		lines = append(lines, "  transition "+entry.line(did))
	}
	return lines
}

func (e CoverageEntry) line(did string) string {
	return fmt.Sprintf("%s (%s): %s", e.Path, e.Declaration, coverageCount(e.Runs, did))
}

func coverageCount(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

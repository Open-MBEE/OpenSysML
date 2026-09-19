package smt

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// CopyPrefix starts the name of every variable of the relation's second copy.
// No name the encoding gives a variable carries a `/`, so the copies stay apart.
const CopyPrefix = "B/"

// CopyNames name the two copies as a report and a witness file spell them.
var CopyNames = [2]string{"A", "B"}

// Pair is the relation twice over: copy A as encoded and copy B over renamed
// variables, the two sharing state 0 and so every free input. It is a function
// of the encoded relation alone, whatever the state vector holds.
type Pair struct {
	// Renamed is copy B's variable for each of copy A's, by A's name; a shared
	// variable has no entry.
	Renamed map[string]*solve.Var
	// Query declares and asserts both copies.
	Query *solve.Query
}

// Pair builds the two-copy relation once and keeps it: every variable but state
// 0's is declared again under CopyPrefix, every assertion asserted again over the
// renamed variables, and a value pinned in copy A pinned in copy B too.
func (e *Encoding) Pair() (*Pair, error) {
	if e.pair != nil {
		return e.pair, nil
	}
	shared := make(map[string]bool)
	for _, v := range e.stateVars(e.States[0]) {
		shared[v.Name] = true
	}
	declared := make(map[string]bool, len(e.Query.Vars))
	for _, v := range e.Query.Vars {
		declared[v.Name] = true
	}
	p := &Pair{Renamed: make(map[string]*solve.Var, len(e.Query.Vars))}
	q := *e.Query
	q.Vars = slices.Clone(e.Query.Vars)
	for _, v := range e.Query.Vars {
		if shared[v.Name] {
			continue
		}
		b := *v
		b.Name = CopyPrefix + v.Name
		if declared[b.Name] {
			return nil, &WitnessError{Var: b.Name, Reason: "copy B's variable is already declared"}
		}
		p.Renamed[v.Name] = &b
		q.Vars = append(q.Vars, &b)
	}
	q.Assertions = slices.Clone(e.Query.Assertions)
	for _, a := range e.Query.Assertions {
		from := a.From
		from.Condition = "copy B: " + from.Condition
		q.Assertions = append(q.Assertions, solve.Assertion{Term: p.copy(a.Term), From: from})
	}
	q.Pinned = slices.Clone(e.Query.Pinned)
	for _, pin := range e.Query.Pinned {
		if b, ok := p.Renamed[pin.Var.Name]; ok {
			pin.Var = b
			q.Pinned = append(q.Pinned, pin)
		}
	}
	p.Query = &q
	e.pair = p
	return p, nil
}

// copy is t over copy B's variables, the shared ones kept.
func (p *Pair) copy(t *solve.Term) *solve.Term {
	return solve.Substitute(t, func(v *solve.Var) *solve.Term {
		if b, ok := p.Renamed[v.Name]; ok {
			return solve.VarTerm(b)
		}
		return nil
	})
}

// second reads the model as copy B's run: every value under the name copy A
// declares it by, so copy A's decoder reads copy B's run.
func (p *Pair) second(m model) model {
	out := make(model, len(m))
	for name, v := range m {
		out[name] = v
	}
	for a, b := range p.Renamed {
		if v, ok := m[b.Name]; ok {
			out[a] = v
		} else {
			delete(out, a)
		}
	}
	return out
}

// Sensitivity is the two-copy query for the output: both copies complete within
// k moves with neither bound reached, and the output ends with different values —
// or held in one run and not the other.
func (e *Encoding) Sensitivity(out Output) (*solve.Query, error) {
	p, err := e.Pair()
	if err != nil {
		return nil, err
	}
	k := e.Moves
	complete := and(e.exact(k), e.Completed[k])
	differ := not(eq(solve.VarTerm(out.Var), p.copy(solve.VarTerm(out.Var))))
	if out.Has != nil {
		has, hasB := solve.VarTerm(out.Has), p.copy(solve.VarTerm(out.Has))
		differ = or(not(eq(has, hasB)), and(has, hasB, differ))
	}
	q := *p.Query
	q.Assertions = slices.Clone(p.Query.Assertions)
	for _, a := range []struct {
		term *solve.Term
		role string
	}{
		{complete, "copy A completes"},
		{p.copy(complete), "copy B completes"},
		{differ, "the copies end with different values of " + out.Name},
	} {
		q.Assertions = append(q.Assertions, solve.Assertion{
			Term: a.term,
			From: solve.Provenance{Kind: "action", Element: e.Query.Element, Condition: a.role, Role: solve.RoleRequired},
		})
	}
	return &q, nil
}

// Diverging is a model of a Sensitivity query read back: the two runs and the
// output's value each leaves, spelt as the interpreter spells it.
type Diverging struct {
	Feature string
	// Runs are copy A's and copy B's schedules, each followed to its completion.
	Runs [2]*Witness
	// Values are the output's final values under the two runs, runtime.UnsetText
	// for a run that leaves it holding none.
	Values [2]string
}

// String spells the pair as the check engine spells a divergence, then the move
// the two schedules part at and the choice each makes there.
func (d *Diverging) String() string {
	reason := fmt.Sprintf("%s ends as %s or %s", d.Feature, d.Values[0], d.Values[1])
	step, a, b := d.Parting()
	if step == 0 {
		return reason + "; the two schedules make the same choices"
	}
	return fmt.Sprintf("%s; the schedules part at step %d: %s against %s", reason, step, a, b)
}

// Parting is the first choice at which the two runs differ, and the two choices
// made; Step is 0 when the runs make the same choices.
func (d *Diverging) Parting() (step int, a, b string) {
	as, bs := d.Runs[0].Choices, d.Runs[1].Choices
	for i := 0; i < len(as) || i < len(bs); i++ {
		switch {
		case i >= len(as):
			return bs[i].Step, "no choice left", bs[i].String()
		case i >= len(bs):
			return as[i].Step, as[i].String(), "no choice left"
		case as[i].Kind != bs[i].Kind || as[i].Step != bs[i].Step || as[i].Where != bs[i].Where || as[i].Took != bs[i].Took:
			return min(as[i].Step, bs[i].Step), as[i].String(), bs[i].String()
		}
	}
	return 0, "", ""
}

// DecodePair reads a model of the output's Sensitivity query back as its two runs.
func (e *Encoding) DecodePair(result *solve.Result, out Output) (*Diverging, error) {
	if result == nil || result.Status != solve.StatusSat || len(result.Model) == 0 {
		return nil, ErrNoWitness
	}
	p, err := e.Pair()
	if err != nil {
		return nil, err
	}
	m, err := readModel(result.Model)
	if err != nil {
		return nil, err
	}
	d := &Diverging{Feature: out.Name}
	for i, run := range []model{m, p.second(m)} {
		if d.Runs[i], err = e.decodeRun(run, e.Moves); err != nil {
			return nil, err
		}
		if d.Values[i], _, err = e.output(run, out); err != nil {
			return nil, err
		}
	}
	if d.Values[0] == d.Values[1] {
		return nil, &WitnessError{Var: out.Var.Name, Reason: fmt.Sprintf("both copies end with %s", d.Values[0])}
	}
	return d, nil
}

// LiveSchedule reads a model of Uncertainty back as the run it shows cut by the
// bound, its choices up to move k.
func (e *Encoding) LiveSchedule(result *solve.Result) (*Witness, error) {
	if result == nil || result.Status != solve.StatusSat || len(result.Model) == 0 {
		return nil, ErrNoWitness
	}
	m, err := readModel(result.Model)
	if err != nil {
		return nil, err
	}
	return e.decodeRun(m, e.Moves)
}

// output spells the output's value in the final state of the run m assigns,
// runtime.UnsetText and false when the run leaves it holding none.
func (e *Encoding) output(m model, out Output) (string, bool, error) {
	if out.Has != nil {
		has, err := m.boolean(out.Has.Name)
		if err != nil {
			return "", false, err
		}
		if !has {
			return runtime.UnsetText, false, nil
		}
	}
	v, ok := m[out.Var.Name]
	if !ok {
		return "", false, &WitnessError{Var: out.Var.Name, Reason: unassignedReason}
	}
	text, err := spell(v)
	if err != nil {
		return "", false, &WitnessError{Var: out.Var.Name, Reason: err.Error()}
	}
	return text, true, nil
}

package smt

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// ErrNoWitness is the typed error of a decode over a result that carries no model.
var ErrNoWitness = errors.New("the result carries no model to decode")

// WitnessError is a model the relation's variables do not read back as a run:
// a value missing or of the wrong shape, which is a fault of the encoding.
type WitnessError struct {
	Var    string
	Reason string
}

func (e *WitnessError) Error() string {
	return fmt.Sprintf("smt: witness %s: %s", e.Var, e.Reason)
}

// Witness is a run the solver found, decoded as the interpreter's own choices
// so a replay policy can follow it move for move.
type Witness struct {
	// Mark is the state the query's model points at: the one violating the
	// property, or the one a stutter left short.
	Mark int
	// Steps is how many moves the run makes before Mark, its stutters excluded:
	// the interpreter's steps that reach the state Mark names.
	Steps int
	// Choices are the choice points the interpreter faces on the run, in the
	// order it notes them: at each step the branch a decision took, then the
	// token stepped, each only where the interpreter has several to pick from.
	Choices []runtime.ChoiceTaken
}

// Decode reads a model of a Violation or Failure query back as the run it
// describes, up to the state MarkVar points at.
func (e *Encoding) Decode(result *solve.Result) (*Witness, error) {
	if result == nil || result.Status != solve.StatusSat || len(result.Model) == 0 {
		return nil, ErrNoWitness
	}
	m, err := readModel(result.Model)
	if err != nil {
		return nil, err
	}
	mark, err := m.integer(MarkVar)
	if err != nil {
		return nil, err
	}
	if mark < 0 || mark > e.Moves {
		return nil, &WitnessError{Var: MarkVar, Reason: fmt.Sprintf("%d is not a state of %d moves", mark, e.Moves)}
	}
	w := &Witness{Mark: mark}
	for i := 1; i <= mark; i++ {
		choice, err := m.datatype(e.Choices[i-1].Choice.Name)
		if err != nil {
			return nil, err
		}
		if choice == Stutter {
			break
		}
		t, err := slotOf(choice)
		if err != nil {
			return nil, &WitnessError{Var: e.Choices[i-1].Choice.Name, Reason: err.Error()}
		}
		if t >= len(e.States[i-1].Slots) {
			return nil, &WitnessError{Var: e.Choices[i-1].Choice.Name, Reason: choice + " names no slot"}
		}
		choices, err := e.decodeMove(m, i, t)
		if err != nil {
			return nil, err
		}
		w.Choices = append(w.Choices, choices...)
		w.Steps = i
	}
	return w, nil
}

// decodeMove is what the interpreter notes at step i when the token in slot t
// acts: the branch its decision took among several that held, then which
// token it was among several able to act.
func (e *Encoding) decodeMove(m model, i, t int) ([]runtime.ChoiceTaken, error) {
	prev := e.States[i-1]
	type able struct {
		id   int64
		node ast.Node
	}
	var enabled []able
	var actor able
	for u, slot := range prev.Slots {
		at, err := m.datatype(slot.At.Name)
		if err != nil {
			return nil, err
		}
		if at == Absent {
			continue
		}
		n := slices.Index(e.Flow.Labels, at)
		if n < 0 {
			return nil, &WitnessError{Var: slot.At.Name, Reason: at + " names no node"}
		}
		id, err := m.integer(slot.ID.Name)
		if err != nil {
			return nil, err
		}
		here := able{id: int64(id), node: e.Flow.Nodes[n]}
		if u == t {
			actor = here
		}
		can, err := m.boolean(slot.Able.Name)
		if err != nil {
			return nil, err
		}
		if can {
			enabled = append(enabled, here)
		}
	}
	if actor.node == nil {
		return nil, &WitnessError{Var: prev.Slots[t].At.Name, Reason: "the acting slot is empty"}
	}
	var out []runtime.ChoiceTaken
	if decision, ok := actor.node.(*ast.DecisionNode); ok {
		branch, err := e.decodeBranch(m, i, decision)
		if err != nil {
			return nil, err
		}
		if branch != nil {
			out = append(out, *branch)
		}
	}
	if len(enabled) >= 2 {
		slices.SortFunc(enabled, func(a, b able) int { return int(a.id - b.id) })
		among := make([]string, len(enabled))
		taken := -1
		for j, a := range enabled {
			among[j] = runtime.TokenLabel(a.id, a.node)
			if a.id == actor.id {
				taken = j
			}
		}
		if taken < 0 {
			return nil, &WitnessError{Var: prev.Slots[t].Able.Name, Reason: "the acting token is not able to act"}
		}
		out = append(out, runtime.ChoiceTaken{
			Kind: runtime.ChoiceTokenOrder, Step: i,
			Alternatives: len(among), Taken: taken, Among: among, Took: among[taken],
		})
	}
	return out, nil
}

// decodeBranch is the branch choice the interpreter notes at step i when the
// token at decision picks among the guarded successions that held, nil when
// fewer than two did.
func (e *Encoding) decodeBranch(m model, i int, decision *ast.DecisionNode) (*runtime.ChoiceTaken, error) {
	move := e.Choices[i-1]
	travel, err := m.datatype(move.Travel.Name)
	if err != nil {
		return nil, err
	}
	var among []string
	taken := -1
	for p, edge := range e.Flow.Outgoing[decision] {
		held := move.Held[edge]
		if held == nil {
			continue
		}
		holds, err := m.boolean(held.Name)
		if err != nil {
			return nil, err
		}
		if !holds {
			continue
		}
		if edgeLabel(e.Flow, edge) == travel {
			taken = len(among)
		}
		among = append(among, runtime.BranchLabel(p, e.Flow.Edges[edge].Target))
	}
	if len(among) < 2 {
		return nil, nil
	}
	if taken < 0 {
		return nil, &WitnessError{Var: move.Travel.Name, Reason: travel + " is not a branch that held"}
	}
	return &runtime.ChoiceTaken{
		Kind: runtime.ChoiceDecisionBranch, Step: i, Where: runtime.DecisionPlace(decision),
		Alternatives: len(among), Taken: taken, Among: among, Took: among[taken],
	}, nil
}

// slotOf is the slot a choice value names.
func slotOf(choice string) (int, error) {
	digits, ok := strings.CutPrefix(choice, "slot")
	if !ok {
		return 0, fmt.Errorf("%s names no slot", choice)
	}
	t, err := strconv.Atoi(digits)
	if err != nil || t < 0 {
		return 0, fmt.Errorf("%s names no slot", choice)
	}
	return t, nil
}

// model is a satisfying assignment read back by variable name.
type model map[string]solve.ModelValue

// readModel decodes every assignment of a model.
func readModel(assignments []solve.Assignment) (model, error) {
	m := make(model, len(assignments))
	for _, a := range assignments {
		value, err := solve.DecodeValue(a)
		if err != nil {
			return nil, &WitnessError{Var: a.Var.Name, Reason: err.Error()}
		}
		m[a.Var.Name] = value
	}
	return m, nil
}

func (m model) value(name string, kind solve.SortKind) (solve.ModelValue, error) {
	v, ok := m[name]
	if !ok {
		return solve.ModelValue{}, &WitnessError{Var: name, Reason: "the model assigns it no value"}
	}
	if v.Kind != kind {
		return solve.ModelValue{}, &WitnessError{Var: name, Reason: "the model assigns it a value of another sort"}
	}
	return v, nil
}

func (m model) datatype(name string) (string, error) {
	v, err := m.value(name, solve.SortDatatype)
	return v.Text, err
}

func (m model) boolean(name string) (bool, error) {
	v, err := m.value(name, solve.SortBool)
	return v.Bool, err
}

func (m model) integer(name string) (int, error) {
	v, err := m.value(name, solve.SortInt)
	if err != nil {
		return 0, err
	}
	if !v.Number.Num().IsInt64() {
		return 0, &WitnessError{Var: name, Reason: v.Number.String() + " is outside the integer range"}
	}
	return int(v.Number.Num().Int64()), nil
}

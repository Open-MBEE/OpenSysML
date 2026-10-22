package solve

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// DeadVariationError is a chosen selection fixing a variation point that does not
// exist under the variants it chooses: a feature declared for one variant is a
// variation point only where the holding variation selects that variant.
type DeadVariationError struct {
	// Var is the variation point chosen, Live the selection under which it exists.
	Var  *Var
	Live *Term
}

// Error names the variation point and the selection it exists under.
func (e *DeadVariationError) Error() string {
	return shortName(e.Var) + " is a variation point only when " + liveText(e.Live)
}

// ChooseLive admits a chosen selection's variation points: one that exists only
// under a variant the selection chooses against is refused, and one whose holder
// is left unchosen asserts the selection it exists under, so that choosing it
// selects the variant that has it.
func (q *Query) ChooseLive(chosen map[*Var]string) error {
	if err := checkLive(chosen); err != nil {
		return err
	}
	vars := liveVars(chosen)
	for _, v := range vars {
		q.Assertions = append(q.Assertions, Assertion{
			Term: v.Live,
			From: Provenance{
				Kind:      "feature",
				Element:   v.Name,
				Condition: shortName(v) + " is a variation point only when " + liveText(v.Live) + ", " + PinChosen.phrase(),
				Role:      RolePinned,
				Declared:  v.Symbol,
				File:      v.File,
				Span:      v.Span,
				Location:  v.Location,
			},
		})
	}
	return nil
}

// checkLive refuses a chosen selection that fixes a variation point the variants
// it chooses for the holding variations leave without meaning.
func checkLive(chosen map[*Var]string) error {
	for _, v := range liveVars(chosen) {
		if evalLive(v.Live, chosen) == liveFalse {
			return &DeadVariationError{Var: v, Live: v.Live}
		}
	}
	return nil
}

// liveVars are the chosen variables that exist under some selections only, by name.
func liveVars(chosen map[*Var]string) []*Var {
	vars := make([]*Var, 0, len(chosen))
	for v := range chosen {
		if v.Live != nil {
			vars = append(vars, v)
		}
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
	return vars
}

type liveness int

const (
	liveUnknown liveness = iota
	liveTrue
	liveFalse
)

// evalLive decides a liveness condition from the values chosen so far, unknown
// where a holder it reads is not chosen.
func evalLive(t *Term, chosen map[*Var]string) liveness {
	switch t.Op {
	case OpBool:
		if t.Bool {
			return liveTrue
		}
		return liveFalse
	case OpNot:
		switch evalLive(t.Args[0], chosen) {
		case liveTrue:
			return liveFalse
		case liveFalse:
			return liveTrue
		}
	case OpAnd:
		out := liveTrue
		for _, a := range t.Args {
			switch evalLive(a, chosen) {
			case liveFalse:
				return liveFalse
			case liveUnknown:
				out = liveUnknown
			}
		}
		return out
	case OpOr:
		out := liveFalse
		for _, a := range t.Args {
			switch evalLive(a, chosen) {
			case liveTrue:
				return liveTrue
			case liveUnknown:
				out = liveUnknown
			}
		}
		return out
	case OpEq:
		if holder, value, ok := selectionOf(t); ok {
			if got, chosen := chosen[holder]; chosen {
				if got == value {
					return liveTrue
				}
				return liveFalse
			}
		}
	}
	return liveUnknown
}

// selectionOf reads a `holder == variant` term.
func selectionOf(t *Term) (*Var, string, bool) {
	if t.Op != OpEq || len(t.Args) != 2 || t.Args[0].Op != OpVar || t.Args[1].Op != OpValue {
		return nil, "", false
	}
	return t.Args[0].Var, t.Args[1].Str, true
}

// liveText writes a liveness condition as the selections it asks for.
func liveText(t *Term) string {
	switch t.Op {
	case OpAnd, OpOr:
		parts := make([]string, 0, len(t.Args))
		for _, a := range t.Args {
			parts = append(parts, liveText(a))
		}
		if t.Op == OpAnd {
			return strings.Join(parts, " and ")
		}
		return strings.Join(parts, " or ")
	case OpEq:
		if holder, value, ok := selectionOf(t); ok {
			return shortName(holder) + " selects " + source.QualifiedNameText(lastSegment(value))
		}
	}
	return writeTerm(t)
}

// shortName is the feature path a variable stands for, as the notation writes it.
func shortName(v *Var) string {
	if len(v.Steps) > 0 {
		return pathName(v.Steps)
	}
	return source.QualifiedNameText(lastSegment(v.Name))
}

func lastSegment(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

package solve

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Script renders the query as an SMT-LIB2 script: its logic, declarations,
// assertions each preceded by a comment naming where it came from, and
// `check-sat`. Output is deterministic, so it compares byte for byte.
func Script(q *Query) string {
	var b strings.Builder
	writeScript(&b, q, scriptOptions{})
	return b.String()
}

// CoreScript renders the query as a core-producing script: every assertion
// labelled, unsat cores turned on, and `get-unsat-core` asked after
// `check-sat`, so running the script by hand answers what the driver asks.
// include, when non-nil, asserts only the assertions at those indices, which is
// how a core is reduced without retranslating.
func CoreScript(q *Query, include []int) string {
	return coreScript(q, include, true)
}

// coreScript writes the labelled script, asking for the core only when request
// is set: the driver asks for it itself once the verdict is unsat, so a solver
// is never asked for a core it has none of.
func coreScript(q *Query, include []int, request bool) string {
	var b strings.Builder
	writeScript(&b, q, scriptOptions{labelled: true, core: true, request: request, include: indexSet(include)})
	return b.String()
}

// scriptOptions selects the form of script written. The zero value is the
// plain script, whose bytes no core support perturbs.
type scriptOptions struct {
	// labelled names each assertion, which is what a core can name back.
	labelled bool

	// core turns unsat cores on.
	core bool

	// request asks for the core after `check-sat`.
	request bool

	// include, when non-nil, holds the indices of the assertions to assert.
	include map[int]bool
}

// asserts reports whether the assertion at index i is written.
func (o scriptOptions) asserts(i int) bool {
	return o.include == nil || o.include[i]
}

// indexSet is the set of indices, nil for a nil slice so "every assertion" and
// "no assertion" stay distinct.
func indexSet(include []int) map[int]bool {
	if include == nil {
		return nil
	}
	out := make(map[int]bool, len(include))
	for _, i := range include {
		out[i] = true
	}
	return out
}

// writeScript writes the script itself; every form of script shares it.
func writeScript(b *strings.Builder, q *Query, opts scriptOptions) {
	fmt.Fprintf(b, "; OpenSysML SMT-LIB2 translation of %s %s\n", q.Kind, comment(q.Element))
	b.WriteString("; the runtime evaluator remains normative; solving is an optional extension\n")
	if q.Negated {
		b.WriteString("; the element asserts that its required conditions do not all hold\n")
	}
	if opts.core {
		b.WriteString("; each assertion is named, so an unsat core names the conditions that conflict\n")
		// The option must precede set-logic, as a solver decides then what to track.
		b.WriteString("(set-option :produce-unsat-cores true)\n")
	}
	if q.Optimizes() {
		b.WriteString("; objectives are optimized lexicographically, in the order the analysis declares them:\n")
		b.WriteString("; each is optimized within what the ones before it already settled\n")
		b.WriteString("(set-option :opt.priority lex)\n")
	}
	logic := q.LogicChoice()
	if !logic.Standard {
		fmt.Fprintf(b, "; no SMT-LIB logic covers %s, so the logic set below is %s, "+
			"which the SMT-LIB logic list does not define\n", comment(logic.Why), logic.Name)
	}
	fmt.Fprintf(b, "(set-logic %s)\n", logic.Name)

	for _, s := range q.Sorts {
		values := make([]string, 0, len(s.Values))
		for _, v := range s.Values {
			values = append(values, "("+smtSymbol(v)+")")
		}
		fmt.Fprintf(b, "; %s of %s\n", s.String(), comment(s.Origin))
		fmt.Fprintf(b, "(declare-datatypes ((%s 0)) ((%s)))\n", s.String(), strings.Join(values, " "))
	}

	for _, v := range q.Vars {
		b.WriteString("; " + varComment(v) + "\n")
		fmt.Fprintf(b, "(declare-const %s %s)\n", smtSymbol(v.Name), v.Sort.String())
	}

	for i, a := range q.Assertions {
		if !opts.asserts(i) {
			continue
		}
		b.WriteString("; " + assertionComment(a.From) + "\n")
		if opts.labelled {
			fmt.Fprintf(b, "(assert (! %s :named %s))\n", writeTerm(a.Term), CoreLabel(i))
			continue
		}
		fmt.Fprintf(b, "(assert %s)\n", writeTerm(a.Term))
	}

	for _, obj := range q.Objectives {
		b.WriteString("; " + objectiveComment(obj) + "\n")
		fmt.Fprintf(b, "(%s %s)\n", obj.Direction, writeTerm(obj.Term))
	}

	b.WriteString("(check-sat)\n")
	if opts.request {
		b.WriteString("(get-unsat-core)\n")
	}
}

// objectiveComment describes an objective: which way it is improved, the value
// as written, the units its magnitude is expressed in, and where it was written.
func objectiveComment(obj Objective) string {
	out := obj.Direction.String() + ": "
	if obj.Name != "" {
		out += comment(obj.Name) + " = "
	}
	out += comment(obj.Expression)
	if obj.Unit != "" {
		out += " in " + comment(obj.Unit)
	} else if obj.Dimension != "" {
		out += " in base units of " + comment(obj.Dimension)
	}
	if obj.Location != "" {
		out += ", at " + comment(obj.Location)
	}
	return out
}

// coreLabelPrefix starts every assertion label. No name from a model is written
// with it, so a label cannot collide with a declared symbol.
const coreLabelPrefix = "sy!a"

// CoreLabel names the assertion at index i in a labelled script. The index is
// the label, so a label a solver returns reads back the Assertion itself, with
// the provenance it carries, rather than a table kept beside the query.
func CoreLabel(i int) string {
	return coreLabelPrefix + strconv.Itoa(i)
}

// coreLabelIndex reads a label back to the assertion index it names.
func coreLabelIndex(label string) (int, bool) {
	rest, ok := strings.CutPrefix(label, coreLabelPrefix)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	if err != nil || i < 0 || strconv.Itoa(i) != rest {
		return 0, false
	}
	return i, true
}

// varComment describes a declared variable: the feature it stands for, the
// dimension its magnitude is expressed in, and where it was declared.
func varComment(v *Var) string {
	out := comment(v.Name)
	if v.Dimension != "" {
		out += " in base units of " + comment(v.Dimension)
	}
	if v.Location != "" {
		out += ", declared at " + comment(v.Location)
	}
	return out
}

// assertionComment names what an assertion came from: its role, the condition as
// written, the element stating it, and where it was written.
func assertionComment(p Provenance) string {
	out := p.Role.String() + ": " + comment(p.Condition)
	out += fmt.Sprintf(" — %s %s", p.Kind, comment(p.Element))
	if p.Declared != nil && p.Declared.Name != "" && p.Declared.Name != p.Element {
		out += ", declared by " + comment(p.Declared.Name)
	}
	if p.Location != "" {
		out += ", at " + comment(p.Location)
	}
	return out
}

// comment renders text safely on one comment line.
func comment(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// writeTerm renders a term as an S-expression.
func writeTerm(t *Term) string {
	switch t.Op {
	case OpBool:
		if t.Bool {
			return "true"
		}
		return "false"
	case OpInt:
		return writeInt(t.Int)
	case OpReal:
		return writeRat(t.Real)
	case OpString:
		return `"` + strings.ReplaceAll(t.Str, `"`, `""`) + `"`
	case OpValue:
		return smtSymbol(t.Str)
	case OpVar:
		return smtSymbol(t.Var.Name)
	}
	args := make([]string, 0, len(t.Args))
	for _, arg := range t.Args {
		args = append(args, writeTerm(arg))
	}
	if t.Op == OpInt64 {
		return "(<= " + writeInt(math.MinInt64) + " " + args[0] + " " + writeInt(math.MaxInt64) + ")"
	}
	op := smtOps[t.Op]
	if t.Op == OpNe {
		// `distinct` is SMT-LIB's inequality; the notation writes `!=`.
		op = "distinct"
	}
	return "(" + op + " " + strings.Join(args, " ") + ")"
}

// writeInt renders an integer literal, as SMT-LIB numerals are non-negative.
func writeInt(i int64) string {
	if i < 0 {
		return fmt.Sprintf("(- %d)", uint64(-i))
	}
	return fmt.Sprintf("%d", i)
}

// writeRat renders a real literal exactly: as a decimal when the magnitude has a
// terminating one, else as the quotient it is, so no rounding enters the script.
func writeRat(r *big.Rat) string {
	if r.Sign() < 0 {
		return "(- " + writeRat(new(big.Rat).Neg(r)) + ")"
	}
	if digits, ok := decimalDigits(r.Denom()); ok {
		return r.FloatString(digits)
	}
	return fmt.Sprintf("(/ %s.0 %s.0)", r.Num().String(), r.Denom().String())
}

// decimalDigits returns how many fractional digits write a magnitude of this
// denominator exactly, which only a denominator of 2s and 5s has.
func decimalDigits(den *big.Int) (int, bool) {
	rest, digits := new(big.Int).Set(den), 0
	for _, prime := range []*big.Int{big.NewInt(2), big.NewInt(5)} {
		count := 0
		quo, rem := new(big.Int), new(big.Int)
		for {
			quo.QuoRem(rest, prime, rem)
			if rem.Sign() != 0 {
				break
			}
			rest.Set(quo)
			count++
		}
		digits = max(digits, count)
	}
	if rest.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	// A decimal literal in SMT-LIB2 always writes a fractional digit.
	return max(digits, 1), true
}

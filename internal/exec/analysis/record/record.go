// Package record generates the SysML declarations that record an analysis
// run, a sweep or a Monte-Carlo sample into the model it ran on, as usages of
// the bundled AnalysisRecords library.
package record

import (
	"fmt"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Kind is the run shape a record's `kind` feature reports.
type Kind string

const (
	// KindRun records one run of an analysis case.
	KindRun Kind = "run"
	// KindTrade records one run of an analysis case that evaluated alternatives.
	KindTrade Kind = "trade"
	// KindSweep records the runs of a sweep.
	KindSweep Kind = "sweep"
	// KindRuns records the runs of a Monte-Carlo sample.
	KindRuns Kind = "runs"
)

// Provenance is what a recorded run reports about how it was made.
type Provenance struct {
	// RunAt is when the run was made; recorded in UTC.
	RunAt time.Time

	// Tool names the program that ran it.
	Tool string

	// Command is the invocation text that ran it.
	Command string

	// Kind is the run shape the records report.
	Kind Kind
}

// Subject is how a run's object is recorded: its usage in the model, and its
// text for subjectName.
type Subject struct {
	// Usage is the subject object's qualified name, or "" when it has none.
	Usage string

	// Text is the subject as subjectName reports it.
	Text string
}

// Run is everything one run of a case contributes to its record.
type Run struct {
	// Iteration is the run's position in a sweep or sample; 0 for a single run.
	Iteration int

	// Subject is the object the run was made on.
	Subject Subject

	// Inputs are the values the run bound the case's input parameters to.
	Inputs []runtime.InputBinding

	// Outputs are the values the run's declared outputs came to.
	Outputs []runtime.CalcOutputValue

	// Verdicts are what the run's checks decided.
	Verdicts []runtime.AnalysisVerdict

	// Evaluations are the calc applications the run made.
	Evaluations []runtime.AnalysisEvaluation
}

// Spelling renders a run's values for the text it cannot supply itself.
type Spelling struct {
	// ObjectUsage is the qualified name of the usage an object value is an
	// occurrence of, or "" when the value names no single usage.
	ObjectUsage func(v runtime.Value) string

	// Text is a value's text for the surfaces that record it as a string.
	Text func(v runtime.Value) string

	// Unset reports a value records nothing: null, or a feature holding none.
	Unset func(v runtime.Value) bool
}

// Feature is one member an existing record definition declares.
type Feature struct {
	// Ref marks a `ref part`, an object-valued feature; unset is an attribute.
	Ref bool

	// TypeFQN is the qualified name of the feature's declared type.
	TypeFQN string
}

// Existing is what Generate must fit the records it makes into.
type Existing struct {
	// Package marks the target package already declared.
	Package bool

	// Definition marks the per-case record definition already declared in it.
	Definition bool

	// Attributes are the features the existing definition declares, by name.
	Attributes map[string]Feature

	// NextRun is the first N for which <case>_runN is free (1-based).
	NextRun int
}

// Request is one call to Generate.
type Request struct {
	// Package is the qualified name of the package the records go into, e.g.
	// "Records" or "Mission::Records".
	Package string

	// Case is the qualified name of the analysis case the runs were made of.
	Case string

	// Provenance is what every record reports about how it was made.
	Provenance Provenance

	// Runs are the runs to record, in order.
	Runs []Run

	// Existing is what the target package already holds.
	Existing Existing

	// Spell renders the values the records cannot spell themselves.
	Spell Spelling
}

// Result is what Generate made.
type Result struct {
	// Source is the declaration text: the target package's full nesting down to
	// it, holding the record definition when new and every record usage.
	Source string

	// Definition is the qualified name of the per-case record definition.
	Definition string

	// Records are the qualified names of the record usages, in order.
	Records []string
}

// reservedFeatures are AnalysisRun's own features a parameter name may not take.
var reservedFeatures = map[string]bool{
	"caseName": true, "kind": true, "objective": true, "iteration": true,
	"subject": true, "subjectName": true, "verdicts": true, "evaluations": true,
}

// feature is one member the record definition declares for a run value.
type feature struct {
	name   string
	ref    bool   // object-valued
	typ    string // declared type as written, "" for a ref
	unitOf string // nonempty: this feature is the unit companion of the named one
}

// valueKind classifies how a value is spelled: its declared type and, for a
// quantity, the companion unit feature it needs.
type valueKind int

const (
	kindUnset valueKind = iota
	kindRef
	kindInteger
	kindReal
	kindBoolean
	kindString
	kindEnum
	kindQuantity
)

// shape is how a value is recorded: its feature kind, the literal spelling,
// and — for a quantity — the unit text its companion feature records.
type shape struct {
	kind    valueKind
	literal string
	typ     string
	unit    string
}

// classify decides the feature shape a value asks for.
func classify(v runtime.Value, req *Request) shape {
	if req.Spell.Unset != nil && req.Spell.Unset(v) {
		return shape{kind: kindUnset}
	}
	switch v.Kind {
	case runtime.ValNull:
		return shape{kind: kindUnset}
	case runtime.ValConst:
		if kind, typ, ok := constScalar(v.Const.Kind); ok {
			return shape{kind: kind, typ: typ, literal: semantics.FormatConst(v.Const)}
		}
		// A constant without a literal spelling, Infinity included, is
		// recorded as a string of its text.
		return shape{kind: kindString, typ: "ScalarValues::String", literal: source.StringText(semantics.FormatConst(v.Const))}
	case runtime.ValString:
		return shape{kind: kindString, typ: "ScalarValues::String", literal: source.StringText(v.Str())}
	case runtime.ValEnumLiteral:
		lit := v.Literal()
		enum := semantics.EnumerationOwning(lit)
		fqn := qualifiedName(enum)
		return shape{kind: kindEnum, typ: fqn, literal: source.QualifiedNameText(fqn + "::" + lit.Name)}
	case runtime.ValQuantity:
		q := v.Quantity()
		return shape{kind: kindQuantity, typ: "ScalarValues::Real", literal: semantics.FormatConst(q.Num), unit: q.Unit.String()}
	case runtime.ValInstance, runtime.ValVariant:
		if req.Spell.ObjectUsage != nil {
			if usage := req.Spell.ObjectUsage(v); usage != "" {
				return shape{kind: kindRef, literal: source.QualifiedNameText(usage)}
			}
		}
	}
	// Everything else — a structured value, or an object naming no usage — is
	// recorded by its text.
	return shape{kind: kindString, typ: "ScalarValues::String", literal: source.StringText(spellText(v, req))}
}

// constScalar is the feature kind and ScalarValues type a scalar literal's
// kind is recorded under; a constant without a literal spelling, Infinity
// included, has none and is recorded as a string.
func constScalar(k semantics.ValueKind) (valueKind, string, bool) {
	switch k {
	case semantics.ValInt:
		return kindInteger, "ScalarValues::Integer", true
	case semantics.ValReal:
		return kindReal, "ScalarValues::Real", true
	case semantics.ValBool:
		return kindBoolean, "ScalarValues::Boolean", true
	}
	return 0, "", false
}

// spellText is a value's text for the string features, Text when supplied and
// the runtime's own formatting otherwise.
func spellText(v runtime.Value, req *Request) string {
	if req.Spell.Text != nil {
		return req.Spell.Text(v)
	}
	return runtime.FormatValue(v)
}

// qualifiedName is a symbol's qualified name by its owning chain.
func qualifiedName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	var names []string
	for cur := sym; cur != nil; cur = cur.Owner() {
		if cur.Name == "" {
			break
		}
		names = append([]string{cur.Name}, names...)
	}
	return strings.Join(names, "::")
}

// shortName is the last qualified-name segment of a case's name.
func shortName(fqn string) string {
	segs, ok := source.QualifiedNameSegments(fqn)
	if !ok || len(segs) == 0 {
		return fqn
	}
	return segs[len(segs)-1]
}

// upperFirst capitalizes a name's leading letter.
func upperFirst(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// members are the input and output values a run declares, in order.
func members(r Run) []struct {
	name  string
	value runtime.Value
} {
	var out []struct {
		name  string
		value runtime.Value
	}
	for _, in := range r.Inputs {
		out = append(out, struct {
			name  string
			value runtime.Value
		}{in.Name, in.Value})
	}
	for _, o := range r.Outputs {
		out = append(out, struct {
			name  string
			value runtime.Value
		}{o.Name, o.Value})
	}
	return out
}

// buildFeatures decides the members the record definition needs: every
// distinct member name in order, its shape from the first run supplying a
// value, and a unit companion after each quantity.
func buildFeatures(req *Request) ([]feature, error) {
	var feats []feature
	byName := map[string]int{}
	// companions maps a quantity's unit companion name to its member's, so a
	// real member taking that name — written before or after the quantity —
	// is the collision it is.
	companions := map[string]string{}
	for i := range req.Runs {
		for _, m := range members(req.Runs[i]) {
			if reservedFeatures[m.name] {
				return nil, fmt.Errorf("case %s: parameter %q shares a name with a feature of AnalysisRecords::AnalysisRun", req.Case, m.name)
			}
			sh := classify(m.value, req)
			if q, ok := companions[m.name]; ok {
				return nil, fmt.Errorf("case %s: member %q collides with the unit companion of quantity %q", req.Case, m.name, q)
			}
			if j, ok := byName[m.name]; ok {
				if sh.kind == kindUnset {
					continue
				}
				if err := compatible(&feats[j], sh); err != nil {
					return nil, fmt.Errorf("case %s: member %q: %w", req.Case, m.name, err)
				}
				continue
			}
			feats = append(feats, feature{name: m.name})
			byName[m.name] = len(feats) - 1
			applyShape(&feats[len(feats)-1], sh)
			if sh.kind == kindQuantity {
				unitName := m.name + "Unit"
				if _, taken := byName[unitName]; taken {
					return nil, fmt.Errorf("case %s: member %q collides with the unit companion of quantity %q", req.Case, unitName, m.name)
				}
				feats = append(feats, feature{name: unitName, typ: "ScalarValues::String", unitOf: m.name})
				byName[unitName] = len(feats) - 1
				companions[unitName] = m.name
			}
		}
	}
	return feats, nil
}

// applyShape gives a feature the declared shape a value's first supply asks for.
func applyShape(f *feature, sh shape) {
	f.ref = sh.kind == kindRef
	switch sh.kind {
	case kindUnset:
		f.typ = "ScalarValues::ScalarValue"
	case kindRef:
		f.typ = ""
	default:
		f.typ = source.QualifiedNameText(sh.typ)
	}
}

// compatible checks a later run's value against the shape a feature took.
func compatible(f *feature, sh shape) error {
	switch {
	case f.ref && sh.kind != kindRef:
		return fmt.Errorf("an object value cannot be recorded in the value member")
	case !f.ref && f.typ != "" && sh.kind == kindRef:
		return fmt.Errorf("a non-object value cannot be recorded in the reference member")
	}
	if f.ref || sh.kind == kindRef {
		return nil
	}
	if f.typ == "ScalarValues::ScalarValue" {
		// The first supply was unset; a settled value gives the member its type.
		f.typ = source.QualifiedNameText(sh.typ)
		return nil
	}
	if f.typ != source.QualifiedNameText(sh.typ) {
		return fmt.Errorf("value recorded as %s cannot follow %s", sh.typ, f.typ)
	}
	return nil
}

// Generate renders the declarations recording req's runs.
func Generate(req Request) (Result, error) {
	if len(req.Runs) == 0 {
		return Result{}, fmt.Errorf("case %s: nothing to record", req.Case)
	}
	for _, r := range req.Runs {
		if len(r.Outputs) == 0 && len(r.Verdicts) == 0 && len(r.Evaluations) == 0 {
			return Result{}, fmt.Errorf("case %s produced no outputs to record", req.Case)
		}
	}
	for _, r := range req.Runs {
		seen := map[string]bool{}
		for _, m := range members(r) {
			if seen[m.name] {
				return Result{}, fmt.Errorf("case %s: member %q is both an input and an output", req.Case, m.name)
			}
			seen[m.name] = true
		}
	}

	defName := upperFirst(shortName(req.Case)) + "Run"
	feats, err := buildFeatures(&req)
	if err != nil {
		return Result{}, err
	}

	if req.Existing.Definition {
		if err := checkExisting(&req, feats, defName); err != nil {
			return Result{}, err
		}
	}

	var src strings.Builder
	segs, _ := source.QualifiedNameSegments(req.Package)
	depth := 0
	for _, seg := range segs {
		writeIndent(&src, depth)
		src.WriteString("package ")
		src.WriteString(source.NameText(seg))
		src.WriteString(" {\n")
		depth++
	}

	recordNames := make([]string, len(req.Runs))
	if !req.Existing.Definition {
		writeDefinition(&src, depth, defName, feats)
	}
	next := req.Existing.NextRun
	if next < 1 {
		next = 1
	}
	for i := range req.Runs {
		name := shortName(req.Case) + "_run" + fmt.Sprint(next+i)
		recordNames[i] = name
		writeRecord(&src, depth, name, defName, feats, &req.Runs[i], &req)
	}

	for d := depth - 1; d >= 0; d-- {
		writeIndent(&src, d)
		src.WriteString("}\n")
	}

	records := make([]string, len(recordNames))
	for i, n := range recordNames {
		records[i] = req.Package + "::" + n
	}
	return Result{Source: src.String(), Definition: req.Package + "::" + defName, Records: records}, nil
}

// checkExisting verifies every feature the records need is declared
// compatibly by the existing definition.
func checkExisting(req *Request, feats []feature, defName string) error {
	def := req.Package + "::" + defName
	for _, f := range feats {
		decl, ok := req.Existing.Attributes[f.name]
		if !ok {
			return fmt.Errorf("record definition %s declares no member %q; record into another package with `into`", def, f.name)
		}
		if decl.Ref != f.ref {
			kind := "an attribute"
			if decl.Ref {
				kind = "a reference"
			}
			want := "a reference"
			if !f.ref {
				want = "an attribute"
			}
			return fmt.Errorf("record definition %s declares %s as %s but the run values need %s; record into another package with `into`", def, f.name, kind, want)
		}
		if !f.ref && f.typ != "" && decl.TypeFQN != "" && decl.TypeFQN != f.typ && f.typ != "ScalarValues::ScalarValue" {
			return fmt.Errorf("record definition %s declares %s : %s but the run values need %s : %s; record into another package with `into`", def, f.name, decl.TypeFQN, f.name, f.typ)
		}
	}
	return nil
}

// writeIndent writes depth levels of indentation.
func writeIndent(src *strings.Builder, depth int) {
	src.WriteString(strings.Repeat("    ", depth))
}

// writeDefinition writes the per-case record definition.
func writeDefinition(src *strings.Builder, depth int, name string, feats []feature) {
	writeIndent(src, depth)
	src.WriteString("part def ")
	src.WriteString(source.NameText(name))
	src.WriteString(" :> AnalysisRecords::AnalysisRun {\n")
	for _, f := range feats {
		writeIndent(src, depth+1)
		if f.ref {
			src.WriteString("ref part ")
			src.WriteString(source.NameText(f.name))
			src.WriteString(";\n")
		} else {
			src.WriteString("attribute ")
			src.WriteString(source.NameText(f.name))
			src.WriteString(" : ")
			src.WriteString(f.typ)
			src.WriteString(";\n")
		}
	}
	writeIndent(src, depth)
	src.WriteString("}\n")
}

// writeRecord writes one run's record usage.
func writeRecord(src *strings.Builder, depth int, name, defName string, feats []feature, r *Run, req *Request) {
	writeIndent(src, depth)
	src.WriteString("part ")
	src.WriteString(source.NameText(name))
	src.WriteString(" : ")
	src.WriteString(source.NameText(defName))
	src.WriteString(" {\n")

	writeIndent(src, depth+1)
	src.WriteString("@AnalysisRecords::RecordedRun {\n")
	for _, m := range []struct{ name, value string }{
		{"runAt", source.StringText(req.Provenance.RunAt.UTC().Format(time.RFC3339))},
		{"tool", source.StringText(req.Provenance.Tool)},
		{"command", source.StringText(req.Provenance.Command)},
		{"kind", source.StringText(string(req.Provenance.Kind))},
	} {
		writeIndent(src, depth+2)
		src.WriteString(m.name)
		src.WriteString(" = ")
		src.WriteString(m.value)
		src.WriteString(";\n")
	}
	writeIndent(src, depth+1)
	src.WriteString("}\n")

	writeFeature(src, depth+1, "caseName", source.StringText(req.Case))
	writeFeature(src, depth+1, "kind", source.StringText(string(req.Provenance.Kind)))
	writeFeature(src, depth+1, "'objective'", source.StringText(objectiveOf(r)))
	if r.Iteration > 0 {
		writeIndent(src, depth+1)
		src.WriteString("attribute :>> iteration = ")
		src.WriteString(fmt.Sprint(r.Iteration))
		src.WriteString(";\n")
	}
	if r.Subject.Usage != "" || r.Subject.Text != "" {
		if r.Subject.Usage != "" {
			writeIndent(src, depth+1)
			src.WriteString("ref :>> 'subject' = ")
			src.WriteString(source.QualifiedNameText(r.Subject.Usage))
			src.WriteString(";\n")
		}
		writeFeature(src, depth+1, "subjectName", source.StringText(r.Subject.Text))
	}

	shapeByName := map[string]feature{}
	for _, f := range feats {
		shapeByName[f.name] = f
	}
	for _, m := range members(*r) {
		sh := classify(m.value, req)
		if sh.kind == kindUnset {
			continue
		}
		writeIndent(src, depth+1)
		if sh.kind == kindRef {
			src.WriteString("ref :>> ")
		} else {
			src.WriteString("attribute :>> ")
		}
		src.WriteString(source.NameText(m.name))
		src.WriteString(" = ")
		src.WriteString(sh.literal)
		src.WriteString(";\n")
		if sh.kind == kindQuantity {
			writeFeature(src, depth+1, m.name+"Unit", source.StringText(sh.unit))
		}
	}

	for i, v := range r.Verdicts {
		writeVerdict(src, depth+1, i+1, v)
	}
	for i, e := range r.Evaluations {
		writeEvaluation(src, depth+1, i+1, e, req)
	}

	writeIndent(src, depth)
	src.WriteString("}\n")
}

// objectiveOf is the status the run's objective verdict reports.
func objectiveOf(r *Run) string {
	for _, v := range r.Verdicts {
		if v.Kind == "objective" {
			return v.Status.String()
		}
	}
	return "undecided"
}

// writeFeature writes `attribute :>> name = literal;`.
func writeFeature(src *strings.Builder, depth int, name, literal string) {
	writeIndent(src, depth)
	src.WriteString("attribute :>> ")
	if strings.HasPrefix(name, "'") {
		src.WriteString(name)
	} else {
		src.WriteString(source.NameText(name))
	}
	src.WriteString(" = ")
	src.WriteString(literal)
	src.WriteString(";\n")
}

// writeVerdict writes one verdict record part.
func writeVerdict(src *strings.Builder, depth, n int, v runtime.AnalysisVerdict) {
	writeIndent(src, depth)
	src.WriteString(fmt.Sprintf("part verdict%d : AnalysisRecords::VerdictRecord :> verdicts {\n", n))
	writeFeature(src, depth+1, "kind", source.StringText(v.Kind))
	writeFeature(src, depth+1, "name", source.StringText(v.Name))
	writeFeature(src, depth+1, "status", source.StringText(v.Status.String()))
	if v.Detail != "" {
		writeFeature(src, depth+1, "detail", source.StringText(v.Detail))
	}
	writeIndent(src, depth)
	src.WriteString("}\n")
}

// writeEvaluation writes one evaluation record part.
func writeEvaluation(src *strings.Builder, depth, n int, e runtime.AnalysisEvaluation, req *Request) {
	writeIndent(src, depth)
	src.WriteString(fmt.Sprintf("part evaluation%d : AnalysisRecords::EvaluationRecord :> evaluations {\n", n))
	writeFeature(src, depth+1, "function", source.StringText(e.Function))
	var args []string
	for _, a := range e.Arguments {
		args = append(args, spellText(a, req))
	}
	writeFeature(src, depth+1, "alternative", source.StringText(strings.Join(args, ", ")))
	if sh := classify(e.Result, req); sh.kind == kindInteger || sh.kind == kindReal || sh.kind == kindQuantity {
		writeFeature(src, depth+1, "score", sh.literal)
		writeFeature(src, depth+1, "result", source.StringText(spellText(e.Result, req)))
	} else if e.Error == nil {
		writeFeature(src, depth+1, "result", source.StringText(spellText(e.Result, req)))
	}
	writeFeature(src, depth+1, "selected", boolText(e.Selected))
	writeFeature(src, depth+1, "tied", boolText(e.Tied))
	if e.Error != nil {
		writeFeature(src, depth+1, "error", source.StringText(e.Error.Error()))
	}
	writeIndent(src, depth)
	src.WriteString("}\n")
}

// boolText spells a Boolean literal.
func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

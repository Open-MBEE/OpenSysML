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
	// KindSample records the conclusion a Monte-Carlo sample made, beside the
	// runs it was made of.
	KindSample Kind = "sample"
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

	// Kind is the run shape this record makes; empty, the request's kind.
	Kind Kind

	// Verifications are what a verification case's body and its subcases
	// decided, for a case that is one.
	Verifications []runtime.VerificationVerdict

	// Spell renders the run's values for the text it cannot supply itself, in
	// the context it was made in — a sweep's rows each carry their own.
	Spell Spelling
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

	// Multi marks a feature whose declared multiplicity admits more than one
	// value.
	Multi bool

	// Unique marks a multi-valued feature that holds no two equal values.
	Unique bool
}

// Existing is what Generate must fit the records it makes into.
type Existing struct {
	// Package marks the target package already declared.
	Package bool

	// Definition marks the per-case record definition already declared in it.
	Definition bool

	// Attributes are the features the existing definition declares, by name.
	Attributes map[string]Feature

	// Taken are the N for which <case>_runN is already declared in the package.
	Taken map[int]bool

	// Stem is the name the records and their definition are built on — the
	// case's short name, or an owner-prefixed fallback a sibling case's
	// definition forced.
	Stem string
}

// Request is one call to Generate.
type Request struct {
	// Package is the qualified name of the package the records go into, e.g.
	// "Records" or "Mission::Records".
	Package string

	// Case is the qualified name of the analysis case the runs were made of.
	Case string

	// CaseName is the case's declared name as written, quoting needs included;
	// the records and their definition are named from it. Empty falls back to
	// Case's last qualified-name segment.
	CaseName string

	// Provenance is what every record reports about how it was made.
	Provenance Provenance

	// Runs are the runs to record, in order.
	Runs []Run

	// Existing is what the target package already holds.
	Existing Existing
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
	"subject": true, "subjectName": true, "verdict": true, "verdicts": true,
	"evaluations": true,
}

// feature is one member the record definition declares for a run value.
type feature struct {
	name     string
	ref      bool   // object-valued
	typ      string // declared type as written, "" for a ref
	unitOf   string // nonempty: this feature is the unit companion of the named one
	multi    bool   // declares [0..*]
	repeated bool   // a sequence contains equal elements
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

const (
	scalarValuesString      = "ScalarValues::String"
	scalarValuesReal        = "ScalarValues::Real"
	scalarValuesInteger     = "ScalarValues::Integer"
	scalarValuesBoolean     = "ScalarValues::Boolean"
	scalarValuesScalarValue = "ScalarValues::ScalarValue"
)

// shape is how a value is recorded: its feature kind, the literal spelling,
// whether the member is multi-valued, and — for a quantity — the unit text its
// companion feature records.
type shape struct {
	kind     valueKind
	literal  string
	typ      string
	unit     string
	multi    bool
	repeated bool
}

// classify decides the feature shape a value asks for.
func classify(v runtime.Value, r *Run) shape {
	// A sequence is multi-valued even when it holds nothing, so no unset hook.
	if v.Kind == runtime.ValSequence {
		return classifySequence(v, r)
	}
	if r.Spell.Unset != nil && r.Spell.Unset(v) {
		return shape{kind: kindUnset}
	}
	// An enumeration literal keeps its identity through a scalar payload too.
	if lit := v.EnumerationLiteral(); lit != nil {
		enum := semantics.EnumerationOwning(lit)
		fqn := qualifiedName(enum)
		return shape{kind: kindEnum, typ: fqn, literal: source.QualifiedNameText(fqn + "::" + lit.Name)}
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
		return shape{kind: kindString, typ: scalarValuesString, literal: source.StringText(semantics.FormatConst(v.Const))}
	case runtime.ValString:
		return shape{kind: kindString, typ: scalarValuesString, literal: source.StringText(v.Str())}
	case runtime.ValQuantity:
		q := v.Quantity()
		return shape{kind: kindQuantity, typ: scalarValuesReal, literal: semantics.FormatConst(q.Num), unit: q.Unit.String()}
	case runtime.ValInstance, runtime.ValVariant:
		if r.Spell.ObjectUsage != nil {
			if usage := r.Spell.ObjectUsage(v); usage != "" {
				return shape{kind: kindRef, literal: source.QualifiedNameText(usage)}
			}
		}
	}
	// Everything else — a structured value, or an object naming no usage — is
	// recorded by its text.
	return shape{kind: kindString, typ: scalarValuesString, literal: source.StringText(spellText(v, r))}
}

// classifySequence spells a sequence as a list literal of its elements' shapes: every
// element the same kind — Integer and Real settling to Real, quantities sharing one unit.
// An element that is unset, an object or itself a sequence has no literal spelling
// beside its kind, so the whole value falls back to its text. The empty sequence
// is unset but multi-valued: it settles a member to [0..*] and spells `()`.
func classifySequence(v runtime.Value, r *Run) shape {
	fallback := shape{kind: kindString, typ: scalarValuesString, literal: source.StringText(spellText(v, r))}
	seq := v.Sequence()
	var elements []runtime.Value
	if seq != nil {
		elements = seq.Elements()
	}
	if len(elements) == 0 {
		return shape{kind: kindUnset, multi: true, literal: "()"}
	}
	literals := make([]string, 0, len(elements))
	var settled shape
	for i, element := range elements {
		es := classify(element, r)
		if es.multi || es.kind == kindUnset || es.kind == kindRef {
			return fallback
		}
		switch {
		case i == 0:
			settled = es
		case es.kind == settled.kind && es.unit == settled.unit && (es.kind != kindEnum || es.typ == settled.typ):
			// Same kind; for a quantity es.unit == settled.unit holds the share, and
			// enumeration literals must spell literals of the one enum.
		case (es.kind == kindInteger || es.kind == kindReal) &&
			(settled.kind == kindInteger || settled.kind == kindReal) &&
			numericPair(es.typ, settled.typ):
			settled = shape{kind: kindReal, typ: scalarValuesReal}
		default:
			return fallback
		}
		literals = append(literals, es.literal)
	}
	settled.multi = true
	settled.repeated = hasRepeatedLiteral(literals)
	settled.literal = "(" + strings.Join(literals, ", ") + ")"
	return settled
}

// constScalar is the feature kind and ScalarValues type a scalar literal's
// kind is recorded under; a constant without a literal spelling, Infinity
// included, has none and is recorded as a string.
func constScalar(k semantics.ValueKind) (valueKind, string, bool) {
	switch k {
	case semantics.ValInt:
		return kindInteger, scalarValuesInteger, true
	case semantics.ValReal:
		return kindReal, scalarValuesReal, true
	case semantics.ValBool:
		return kindBoolean, scalarValuesBoolean, true
	}
	return 0, "", false
}

// spellText is a value's text for the string features, Text when supplied and
// the runtime's own formatting otherwise.
func spellText(v runtime.Value, r *Run) string {
	if r.Spell.Text != nil {
		return r.Spell.Text(v)
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

// member is one input or output value a run declares. inOf marks the In
// companion of an inout: one member in the run's value, one in its binding.
type member struct {
	name  string
	value runtime.Value
	inOf  string
}

// members are the input and output values a run declares, in order. A name
// on both sides is one inout parameter: the value it ran to, then an
// <name>In companion holding the value it was bound with.
func members(r Run) []member {
	var out []member
	outs := map[string]bool{}
	for _, o := range r.Outputs {
		outs[o.Name] = true
	}
	inouts := map[string]runtime.Value{}
	for _, in := range r.Inputs {
		if outs[in.Name] {
			inouts[in.Name] = in.Value
			continue
		}
		out = append(out, member{name: in.Name, value: in.Value})
	}
	for _, o := range r.Outputs {
		out = append(out, member{name: o.Name, value: o.Value})
		if v, ok := inouts[o.Name]; ok {
			out = append(out, member{name: o.Name + "In", value: v, inOf: o.Name})
		}
	}
	return out
}

// buildFeatures decides the members the record definition needs: every
// distinct member name in first-encounter order, its shape settled from the
// runs that supply it a value, and a unit companion after each quantity.
func buildFeatures(req *Request) ([]feature, error) {
	// An inout's In companion is the run's own member, but a member a run
	// declares of the same name collides with it, as a Unit companion's does.
	companionOf := map[string]string{}
	for i := range req.Runs {
		for _, m := range members(req.Runs[i]) {
			if m.inOf != "" {
				companionOf[m.name] = m.inOf
			}
		}
	}
	// First pass: settle each member's shape over every run that supplies it.
	var names []string
	shapes := map[string]shape{}
	for i := range req.Runs {
		for _, m := range members(req.Runs[i]) {
			if m.inOf == "" {
				if owner, ok := companionOf[m.name]; ok {
					return nil, fmt.Errorf("case %s: member %q collides with the in companion of inout %q", req.Case, m.name, owner)
				}
			}
			if reservedFeatures[m.name] {
				return nil, fmt.Errorf("case %s: parameter %q shares a name with a feature of AnalysisRecords::AnalysisRun", req.Case, m.name)
			}
			sh := classify(m.value, &req.Runs[i])
			cur, seen := shapes[m.name]
			if !seen {
				names = append(names, m.name)
				shapes[m.name] = sh
				continue
			}
			if sh.kind == kindUnset && !sh.multi {
				continue
			}
			if cur.kind == kindUnset && cur.multi && !sh.multi {
				// An empty sequence claimed the member multi-valued; a single
				// value cannot settle it.
				f := feature{name: m.name}
				applyShape(&f, cur)
				if err := compatible(&f, sh); err != nil {
					return nil, fmt.Errorf("case %s: member %q: %w", req.Case, m.name, err)
				}
			}
			if cur.kind == kindUnset {
				// An unset member takes the settling value's shape; a run's empty
				// sequence keeps the member multi-valued whichever settles it.
				sh.multi = sh.multi || cur.multi
				shapes[m.name] = sh
				continue
			}
			if cur.multi != sh.multi {
				f := feature{name: m.name}
				applyShape(&f, cur)
				if err := compatible(&f, sh); err != nil {
					return nil, fmt.Errorf("case %s: member %q: %w", req.Case, m.name, err)
				}
			}
			// A quantity member takes a plain-number row either order: the
			// row keeps its literal and takes no unit.
			if sh.kind == kindQuantity && (cur.kind == kindInteger || cur.kind == kindReal) {
				shapes[m.name] = sh
				continue
			}
			if cur.kind == kindQuantity && (sh.kind == kindInteger || sh.kind == kindReal) {
				continue
			}
			// Integer and Real are one numeric family for the record
			// definition: either way the member settles to Real, an Integer
			// literal remaining valid under it.
			if numericPair(cur.typ, sh.typ) {
				cur = shape{kind: kindReal, typ: scalarValuesReal, multi: cur.multi, repeated: cur.repeated || sh.repeated}
				shapes[m.name] = cur
				continue
			}
			f := feature{name: m.name}
			applyShape(&f, cur)
			if err := compatible(&f, sh); err != nil {
				return nil, fmt.Errorf("case %s: member %q: %w", req.Case, m.name, err)
			}
		}
	}
	// The two sides of an inout settle to one shape: a quantity side wins over
	// a plain number, Integer and Real settle to Real, anything else must match.
	for companion, owner := range companionOf {
		o, c := shapes[owner], shapes[companion]
		switch {
		case o.multi != c.multi:
			f := feature{name: owner}
			applyShape(&f, o)
			if err := compatible(&f, c); err != nil {
				return nil, fmt.Errorf("case %s: inout %q: %w", req.Case, owner, err)
			}
		case o.kind == kindUnset || c.kind == kindUnset:
		case o.kind == kindQuantity && (c.kind == kindInteger || c.kind == kindReal):
			shapes[companion] = o
		case c.kind == kindQuantity && (o.kind == kindInteger || o.kind == kindReal):
			shapes[owner] = c
		case numericPair(o.typ, c.typ):
			shapes[owner] = shape{kind: kindReal, typ: scalarValuesReal, multi: o.multi, repeated: o.repeated || c.repeated}
			shapes[companion] = shape{kind: kindReal, typ: scalarValuesReal, multi: c.multi, repeated: o.repeated || c.repeated}
		default:
			f := feature{name: owner}
			applyShape(&f, o)
			if err := compatible(&f, c); err != nil {
				return nil, fmt.Errorf("case %s: inout %q: %w", req.Case, owner, err)
			}
		}
	}
	// Emit the features, each quantity's unit companion after it; a member
	// named for one is a collision whatever order they met in.
	units := map[string]string{}
	for _, name := range names {
		if shapes[name].kind == kindQuantity {
			units[name+"Unit"] = name
		}
	}
	var feats []feature
	for _, name := range names {
		if q, ok := units[name]; ok {
			return nil, fmt.Errorf("case %s: member %q collides with the unit companion of quantity %q", req.Case, name, q)
		}
		f := feature{name: name}
		applyShape(&f, shapes[name])
		feats = append(feats, f)
		if shapes[name].kind == kindQuantity {
			feats = append(feats, feature{name: name + "Unit", typ: scalarValuesString, unitOf: name})
		}
	}
	return feats, nil
}

// hasRepeatedLiteral reports whether two elements of a sequence spell the same literal.
func hasRepeatedLiteral(literals []string) bool {
	seen := make(map[string]bool, len(literals))
	for _, literal := range literals {
		if seen[literal] {
			return true
		}
		seen[literal] = true
	}
	return false
}

// numericPair reports whether the types are Integer and Real in either order:
// one numeric family for the record definition, settling to Real.
func numericPair(a, b string) bool {
	return (a == scalarValuesInteger && b == scalarValuesReal) ||
		(a == scalarValuesReal && b == scalarValuesInteger)
}

// applyShape gives a feature the declared shape a value's first supply asks for.
func applyShape(f *feature, sh shape) {
	f.ref = sh.kind == kindRef
	f.multi = sh.multi
	f.repeated = sh.repeated
	switch sh.kind {
	case kindUnset:
		f.typ = scalarValuesScalarValue
	case kindRef:
		f.typ = ""
	default:
		f.typ = source.QualifiedNameText(sh.typ)
	}
}

// compatible checks a later run's value against the shape a feature took.
func compatible(f *feature, sh shape) error {
	if f.multi != sh.multi {
		if sh.multi {
			return fmt.Errorf("a sequence cannot be recorded in a single-valued member")
		}
		return fmt.Errorf("a single value cannot be recorded in a sequence member")
	}
	if sh.kind == kindUnset {
		// An empty sequence fits any multi-valued member; nothing records nothing.
		return nil
	}
	switch {
	case f.ref && sh.kind != kindRef:
		return fmt.Errorf("an object value cannot be recorded in the value member")
	case !f.ref && f.typ != "" && sh.kind == kindRef:
		return fmt.Errorf("a non-object value cannot be recorded in the reference member")
	}
	if f.ref || sh.kind == kindRef {
		return nil
	}
	if f.typ == scalarValuesScalarValue {
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
		if len(r.Outputs) == 0 && len(r.Verdicts) == 0 && len(r.Evaluations) == 0 && len(r.Verifications) == 0 {
			return Result{}, fmt.Errorf("case %s produced no outputs to record", req.Case)
		}
	}
	for _, r := range req.Runs {
		seen := map[string]bool{}
		for _, in := range r.Inputs {
			if seen[in.Name] {
				return Result{}, fmt.Errorf("case %s: member %q is listed twice", req.Case, in.Name)
			}
			seen[in.Name] = true
		}
		seen = map[string]bool{}
		for _, o := range r.Outputs {
			if seen[o.Name] {
				return Result{}, fmt.Errorf("case %s: member %q is listed twice", req.Case, o.Name)
			}
			seen[o.Name] = true
		}
	}

	stem := req.Existing.Stem
	if stem == "" {
		stem = req.CaseName
	}
	if stem == "" {
		stem = shortName(req.Case)
	}
	defName := upperFirst(stem) + "Run"
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
		src.WriteString(source.QualifiedNameText(seg))
		src.WriteString(" {\n")
		depth++
	}

	recordNames := make([]string, len(req.Runs))
	if !req.Existing.Definition {
		writeDefinition(&src, depth, defName, feats, req.Case)
	}
	// Number each record the smallest free N, the package's taken numbers and
	// this batch's both skipped.
	taken := map[int]bool{}
	for n := range req.Existing.Taken {
		taken[n] = true
	}
	next := 1
	for i := range req.Runs {
		for taken[next] {
			next++
		}
		name := stem + "_run" + fmt.Sprint(next)
		taken[next] = true
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
		if decl.Multi != f.multi {
			kind := "single-valued"
			if decl.Multi {
				kind = "a sequence"
			}
			want := "a sequence"
			if !f.multi {
				want = "single-valued"
			}
			return fmt.Errorf("record definition %s declares %s as %s but the run values need %s; record into another package with `into`", def, f.name, kind, want)
		}
		if f.multi && f.repeated && decl.Unique {
			return fmt.Errorf("record definition %s declares %s unique but the run values repeat a value; record into another package with `into`", def, f.name)
		}
		if !f.ref && f.typ != "" && decl.TypeFQN != "" && decl.TypeFQN != f.typ &&
			f.typ != scalarValuesScalarValue && decl.TypeFQN != scalarValuesScalarValue {
			// An Integer literal is valid under a declared Real; the
			// reverse would widen a definition the model owns, so it stays
			// refused.
			if decl.TypeFQN == scalarValuesReal && f.typ == scalarValuesInteger {
				continue
			}
			return fmt.Errorf("record definition %s declares %s : %s but the run values need %s : %s; record into another package with `into`", def, f.name, decl.TypeFQN, f.name, f.typ)
		}
	}
	return nil
}

// writeIndent writes depth levels of indentation.
func writeIndent(src *strings.Builder, depth int) {
	src.WriteString(strings.Repeat("    ", depth))
}

// writeDefinition writes the per-case record definition, its caseName default
// marking the case it records.
func writeDefinition(src *strings.Builder, depth int, name string, feats []feature, caseFQN string) {
	writeIndent(src, depth)
	src.WriteString("part def ")
	src.WriteString(source.NameText(name))
	src.WriteString(" :> AnalysisRecords::AnalysisRun {\n")
	writeIndent(src, depth+1)
	src.WriteString("attribute :>> caseName default = ")
	src.WriteString(source.StringText(caseFQN))
	src.WriteString(";\n")
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
			if f.multi {
				// A record is an observation log: every answered value in reply
				// order, repeats included.
				src.WriteString("[0..*] ordered nonunique")
			}
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
		{"kind", source.StringText(string(kindOf(r, req)))},
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
	writeFeature(src, depth+1, "kind", source.StringText(string(kindOf(r, req))))
	writeFeature(src, depth+1, "'objective'", source.StringText(objectiveOf(r)))
	for _, v := range r.Verifications {
		if !v.Subcase {
			writeFeature(src, depth+1, "verdict", source.StringText(string(v.Kind)))
		}
	}
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
		sh := classify(m.value, r)
		if sh.kind == kindUnset && !sh.multi {
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
	for i, v := range r.Verifications {
		writeVerification(src, depth+1, i+1, v)
	}
	for i, e := range r.Evaluations {
		writeEvaluation(src, depth+1, i+1, e, r)
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

// kindOf is the kind a run's record reports: its own where set, the
// request's otherwise.
func kindOf(r *Run, req *Request) Kind {
	if r.Kind != "" {
		return r.Kind
	}
	return req.Provenance.Kind
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

// writeVerification writes a verification case's body or subcase verdict as
// one more VerdictRecord: its kind says which, its name the case that ran.
func writeVerification(src *strings.Builder, depth, n int, v runtime.VerificationVerdict) {
	kind := "verification"
	if v.Subcase {
		kind = "subcase"
	}
	writeIndent(src, depth)
	src.WriteString(fmt.Sprintf("part verification%d : AnalysisRecords::VerdictRecord :> verdicts {\n", n))
	writeFeature(src, depth+1, "kind", source.StringText(kind))
	writeFeature(src, depth+1, "name", source.StringText(v.Case))
	writeFeature(src, depth+1, "status", source.StringText(string(v.Kind)))
	if v.Detail != "" {
		writeFeature(src, depth+1, "detail", source.StringText(v.Detail))
	}
	writeIndent(src, depth)
	src.WriteString("}\n")
}

// writeEvaluation writes one evaluation record part.
func writeEvaluation(src *strings.Builder, depth, n int, e runtime.AnalysisEvaluation, r *Run) {
	writeIndent(src, depth)
	src.WriteString(fmt.Sprintf("part evaluation%d : AnalysisRecords::EvaluationRecord :> evaluations {\n", n))
	writeFeature(src, depth+1, "function", source.StringText(e.Function))
	var args []string
	for _, a := range e.Arguments {
		args = append(args, spellText(a, r))
	}
	writeFeature(src, depth+1, "alternative", source.StringText(strings.Join(args, ", ")))
	if sh := classify(e.Result, r); sh.kind == kindInteger || sh.kind == kindReal || sh.kind == kindQuantity {
		writeFeature(src, depth+1, "score", sh.literal)
		writeFeature(src, depth+1, "result", source.StringText(spellText(e.Result, r)))
	} else if e.Error == nil {
		writeFeature(src, depth+1, "result", source.StringText(spellText(e.Result, r)))
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

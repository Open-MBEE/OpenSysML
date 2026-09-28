// Package fmi writes the SysML notation an FMU imports as: a calc def computed
// by the `fmi` tool, holding the model's parameters, inputs and outputs as
// ToolVariable-annotated members.
package fmi

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	execfmi "github.com/Open-MBEE/OpenSysML/internal/exec/fmi"
)

// Options are the choices an import makes beyond the model description itself.
type Options struct {
	// URI is what ToolExecution.uri says; required.
	URI string
	// Package names the package the calc def is wrapped in; empty uses the
	// model name, sanitized.
	Package string
}

// ErrURI is the typed error for an import with no URI to write.
var errNoURI = fmt.Errorf("fmi: Options.URI is required; ToolExecution.uri names the FMU the calc computes")

// Notation writes the SysML notation importing the FMU d describes, as a calc
// def named for the model inside a package: parameters and inputs first, then
// the reserved experiment parameters, then the outputs — the first of them the
// calc's return, or `fmi:time` when the FMU declares no outputs.
func Notation(d *execfmi.Description, opts Options) ([]byte, error) {
	if opts.URI == "" {
		return nil, errNoURI
	}
	pkg := opts.Package
	if pkg == "" {
		pkg = sanitize(d.ModelName)
	}
	names := newNamer()
	calcName := names.take(d.ModelName)

	var b strings.Builder
	iface := "model exchange"
	switch {
	case d.CoSimulation != nil:
		iface = "co-simulation"
	case d.ScheduledExecution != nil:
		iface = "scheduled execution"
	}
	identifier := ""
	if i := firstInterface(d); i != nil {
		identifier = ", model identifier " + i.ModelIdentifier
	}
	fmt.Fprintf(&b, "// Imported from %s (FMI %s, %s%s)\n", modelFile(d, opts.URI), d.FMIVersion, iface, identifier)
	fmt.Fprintf(&b, "package %s {\n", pkg)
	b.WriteString("\tprivate import ScalarValues::*;\n\tprivate import AnalysisTooling::*;\n\n")
	fmt.Fprintf(&b, "\tcalc def %s {\n", calcName)
	if d.Description != "" {
		fmt.Fprintf(&b, "\t\tdoc /* %s */\n", docText(d.Description))
	}
	fmt.Fprintf(&b, "\t\tmetadata ToolExecution {\n\t\t\ttoolName = \"fmi\";\n\t\t\turi = %s;\n\t\t}\n", strconv.Quote(opts.URI))

	// The experiment parameters take their identifiers first so a model
	// variable of the same name is suffixed, not them.
	for _, name := range experimentNames(d) {
		names.taken[name] = 1
	}
	// The variables keep document order, skipped ones as comment items in the
	// section their causality would have put them in.
	var inputs, outputs []item
	for _, v := range d.Variables {
		skip, reason := importable(v)
		m := member{v: v}
		switch v.Causality {
		case execfmi.CausalityInput, execfmi.CausalityParameter, execfmi.CausalityStructuralParameter:
			if skip {
				inputs = append(inputs, comment(reason))
			} else {
				m.direction = "in"
				inputs = append(inputs, m)
			}
		case execfmi.CausalityOutput, execfmi.CausalityCalculatedParameter:
			if skip {
				outputs = append(outputs, comment(reason))
			} else {
				m.direction = "out"
				outputs = append(outputs, m)
			}
		default:
			if skip {
				outputs = append(outputs, comment(reason))
			}
		}
	}
	for _, it := range inputs {
		if m, ok := it.(member); ok {
			m.ident = names.take(m.v.Name)
			inputs[indexOf(inputs, it)] = m
		}
	}
	var body strings.Builder
	body.WriteString("\t\t// parameters and inputs of the model, in document order\n")
	for _, it := range inputs {
		it.write(&body)
	}
	body.WriteString("\t\t// the simulation experiment\n")
	writeExperiment(&body, d)
	body.WriteString("\t\t// outputs at stopTime\n")
	firstOut := true
	for _, it := range outputs {
		if m, ok := it.(member); ok {
			m.ident = names.take(m.v.Name)
			if firstOut {
				m.direction = "return"
				firstOut = false
			}
			it = m
		}
		it.write(&body)
	}
	if firstOut {
		body.WriteString("\t\treturn time : Real { @ToolVariable { name = \"fmi:time\"; } }\n")
	}
	b.WriteString(body.String())
	b.WriteString("\t}\n}\n")
	return []byte(b.String()), nil
}

// item is one line of the calc body: a member or a skip comment.
type item interface {
	write(b *strings.Builder)
}

// indexOf finds it in items, for the identifier pass over inputs.
func indexOf(items []item, it item) int {
	for i, x := range items {
		if x == it {
			return i
		}
	}
	return -1
}

// member is one calc parameter written from an FMU variable.
type member struct {
	v         execfmi.Variable
	direction string
	ident     string
}

// comment is one `// …` line documenting a variable that was not imported.
type comment string

func (c comment) write(b *strings.Builder) { fmt.Fprintf(b, "\t\t// %s\n", string(c)) }

// importable reports whether the variable is written, and the comment line a
// skipped one leaves instead.
func importable(v execfmi.Variable) (skip bool, reason string) {
	if v.Array {
		return true, fmt.Sprintf("array %s not imported", v.Name)
	}
	if v.Causality == execfmi.CausalityStructuralParameter {
		return true, fmt.Sprintf("structural parameter %s not imported", v.Name)
	}
	if v.Kind == execfmi.KindBinary || v.Kind == execfmi.KindClock {
		return true, fmt.Sprintf("%s %s not imported: %s is not an imported type", v.Name, v.Kind, v.FMIType)
	}
	return false, ""
}

// write emits the parameter's declaration line.
func (m member) write(b *strings.Builder) {
	v := m.v
	fmt.Fprintf(b, "\t\t%s %s : %s", m.direction, m.ident, sysmlType(v.Kind))
	if v.HasStart && m.direction == "in" {
		fmt.Fprintf(b, " = %s", literal(v))
	}
	fmt.Fprintf(b, " { @ToolVariable { name = %s; } }", strconv.Quote(v.Name))
	if note := memberNote(v); note != "" {
		fmt.Fprintf(b, " // %s", note)
	}
	b.WriteString("\n")
}

// memberNote is the trailing comment a variable's unit or description leaves.
func memberNote(v execfmi.Variable) string {
	var parts []string
	if v.Unit != "" {
		parts = append(parts, v.Unit)
	}
	if v.Description != "" {
		parts = append(parts, v.Description)
	}
	return strings.Join(parts, " — ")
}

// sysmlType is the SysML type a variable's kind writes as; units are not typed.
func sysmlType(k execfmi.Kind) string {
	switch k {
	case execfmi.KindInteger, execfmi.KindEnumeration:
		return "Integer"
	case execfmi.KindBoolean:
		return "Boolean"
	case execfmi.KindString:
		return "String"
	default:
		return "Real"
	}
}

// literal renders a start value as a SysML literal of the variable's kind: a
// Real always with a decimal point or exponent, strings quoted, booleans as
// true/false, integers bare.
func literal(v execfmi.Variable) string {
	switch v.Kind {
	case execfmi.KindReal:
		f, err := strconv.ParseFloat(v.Start, 64)
		if err != nil {
			return v.Start
		}
		text := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	case execfmi.KindInteger, execfmi.KindEnumeration:
		return v.Start
	case execfmi.KindBoolean:
		return strconv.FormatBool(strings.TrimSpace(v.Start) == "true" || v.Start == "1")
	case execfmi.KindString:
		return strconv.Quote(v.Start)
	}
	return v.Start
}

// writeExperiment writes the reserved experiment parameters: startTime and
// stopTime always, stepSize and tolerance only when DefaultExperiment has
// them. Their identifiers are claimed for them before any variable is named,
// so they are written literally.
func writeExperiment(b *strings.Builder, d *execfmi.Description) {
	start, stop, step, tol := 0.0, 1.0, 0.0, 0.0
	hasStep, hasTol := false, false
	if e := d.DefaultExperiment; e != nil {
		if e.HasStart {
			start = e.StartTime
		}
		if e.HasStop {
			stop = e.StopTime
		}
		hasStep, hasTol = e.HasStepSize, e.HasTolerance
		step, tol = e.StepSize, e.Tolerance
	}
	fmt.Fprintf(b, "\t\tin startTime : Real = %s { @ToolVariable { name = \"fmi:startTime\"; } }\n", realLiteral(start))
	fmt.Fprintf(b, "\t\tin stopTime : Real = %s { @ToolVariable { name = \"fmi:stopTime\"; } }\n", realLiteral(stop))
	if hasStep {
		fmt.Fprintf(b, "\t\tin stepSize : Real = %s { @ToolVariable { name = \"fmi:stepSize\"; } }\n", realLiteral(step))
	}
	if hasTol {
		fmt.Fprintf(b, "\t\tin tolerance : Real = %s { @ToolVariable { name = \"fmi:tolerance\"; } }\n", realLiteral(tol))
	}
}

// realLiteral renders a Real literal always carrying a decimal point or
// exponent.
func realLiteral(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0.0"
	}
	text := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(text, ".eE") {
		text += ".0"
	}
	return text
}

// modelFile names the FMU in the header comment: the URI's base name, which
// for a file URI is the file's own name.
func modelFile(d *execfmi.Description, uri string) string {
	name := uri
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return d.ModelName + ".fmu"
	}
	return name
}

// docText escapes a doc comment's terminator.
func docText(text string) string {
	return strings.ReplaceAll(text, "*/", "* /")
}

// firstInterface is whichever interface element the description holds, in the
// order the engine prefers.
func firstInterface(d *execfmi.Description) *execfmi.Interface {
	switch {
	case d.CoSimulation != nil:
		return d.CoSimulation
	case d.ModelExchange != nil:
		return d.ModelExchange
	default:
		return d.ScheduledExecution
	}
}

// namer hands out sanitized identifiers that collide with nothing already given.
type namer struct {
	taken map[string]int
}

func newNamer() *namer { return &namer{taken: make(map[string]int)} }

// take sanitizes raw and claims it, suffixing _2, _3… on a collision.
func (n *namer) take(raw string) string {
	base := sanitize(raw)
	name := base
	for i := 2; n.taken[name] != 0; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	n.taken[name] = 1
	return name
}

// experimentNames are the identifiers the reserved experiment parameters and
// the fmi:time output claim: startTime, stopTime and time always, stepSize and
// tolerance when DefaultExperiment has them.
func experimentNames(d *execfmi.Description) []string {
	names := []string{"startTime", "stopTime", "time"}
	if e := d.DefaultExperiment; e != nil {
		if e.HasStepSize {
			names = append(names, "stepSize")
		}
		if e.HasTolerance {
			names = append(names, "tolerance")
		}
	}
	return names
}

// sanitize maps a name to a SysML identifier: anything but letters, digits and
// underscore becomes `_`, a leading digit gains a `_`, and a keyword gains a
// trailing one.
func sanitize(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" {
		name = "_"
	}
	if c := name[0]; c >= '0' && c <= '9' {
		name = "_" + name
	}
	if keywords[name] {
		name += "_"
	}
	return name
}

// keywords are the SysML v2 reserved words a declaration may not shadow.
var keywords = func() map[string]bool {
	list := []string{
		"about", "abstract", "accept", "action", "actor", "after", "alias", "all",
		"allocate", "allocation", "analysis", "and", "as", "assert", "assign",
		"at", "attribute", "bind", "binding", "by", "calc", "case", "comment",
		"concern", "connect", "connection", "constraint", "cross", "decide",
		"def", "default", "defined", "dependency", "derived", "do", "doc",
		"else", "end", "entry", "enum", "event", "exhibit", "exit", "expose",
		"false", "feature", "featured", "featuring", "filter", "first", "flow",
		"for", "fork", "frame", "from", "if", "implies", "import", "in",
		"include", "individual", "inout", "interface", "is", "item", "join",
		"language", "library", "locale", "loop", "merge", "message", "meta",
		"metadata", "nonunique", "not", "null", "objective", "occurrence", "of",
		"or", "ordered", "out", "package", "parallel", "part", "perform",
		"port", "private", "protected", "public", "readonly", "redefines",
		"ref", "references", "render", "rendering", "rep", "require",
		"requirement", "return", "satisfies", "satisfy", "send", "snapshot",
		"specializes", "staged", "standard", "state", "stream", "subject",
		"subsets", "succession", "then", "timeslice", "to", "transition",
		"true", "type", "until", "use", "variant", "variation", "verification",
		"view", "viewpoint", "when", "while", "xor",
	}
	m := make(map[string]bool, len(list))
	for _, w := range list {
		m[w] = true
	}
	return m
}()

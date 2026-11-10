package queryexec

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The names the QVTS library declares, which a written result view is typed by.
const (
	ResultViewLibrary    = "QVTS"
	ResultViewDefinition = "QVTS::QueryResult"
	OutcomeEstablished   = "established"
)

// DefaultResultViewName names the view of a run whose query has no name.
const DefaultResultViewName = "Answer"

// ResultView is what a surface knows about one query run besides its outcome:
// the four attributes of the QVTS::QueryResult view the run is written as.
type ResultView struct {
	// Name names the view: the query's name, or one the caller chose.
	Name string
	// Query is the query as the surface was given it: a qualified name with its
	// bindings, an inline expression, or SysML v2 API or OSLC query text.
	Query string
	// Source is the model as the surface knows it: file paths or a model hash.
	Source string
	// Provenance identifies the run's inputs; a failed run's step is added to it.
	Provenance string
}

// ResultViewName is the view name of a query: the last segment of its qualified
// name, DefaultResultViewName for a query with none.
func ResultViewName(query string) string {
	segments := symbols.SegmentsOf(query)
	if len(segments) == 0 || !source.IsIdentifier(segments[len(segments)-1]) {
		return DefaultResultViewName
	}
	return segments[len(segments)-1]
}

// Write is the run as one view typed by QVTS::QueryResult exposing the selection, or a failed
// run's frontier and finding elements; false for an error carrying no QVTS record.
func (v ResultView) Write(context Context, result *RowSet, err error) (string, bool) {
	if err != nil {
		var typed *Error
		if !asError(err, &typed) || typed.Outcome == nil {
			return "", false
		}
		return v.WriteOutcome(context, *typed.Outcome), true
	}
	return v.write(context, result, nil), true
}

// WriteOutcome is a failed run as its view: failed is its path or model error.
func (v ResultView) WriteOutcome(context Context, failed Outcome) string {
	return v.write(context, nil, &failed)
}

func (v ResultView) write(context Context, result *RowSet, failed *Outcome) string {
	outcome, provenance := OutcomeEstablished, v.Provenance
	var exposed []string
	unnamed := 0
	if failed != nil {
		outcome = failed.Kind()
		provenance = joinProvenance(provenance, "step "+failed.Step().Label())
		exposed = outcomeElements(context.Index, *failed)
	} else if result != nil {
		for _, row := range result.Rows() {
			name := symbols.FQNOf(row.Element().Declaration())
			if name == "" {
				unnamed++
				continue
			}
			exposed = appendDistinct(exposed, name)
		}
	}
	name := v.Name
	if name == "" {
		name = DefaultResultViewName
	}
	var b strings.Builder
	b.WriteString("package " + source.NameText(upperFirst(name)+"Result") + " {\n")
	b.WriteString("\tprivate import " + ResultViewLibrary + "::*;\n\n")
	view := lowerFirst(name)
	b.WriteString("\tview " + source.NameText(view) + " : QueryResult {\n")
	if unnamed > 0 {
		b.WriteString("\t\t// " + unnamedNote(unnamed) + "\n")
	}
	for _, element := range exposed {
		b.WriteString("\t\texpose " + exposedName(context, view, element) + ";\n")
	}
	b.WriteString("\t\tattribute :>> query = " + source.StringText(v.Query) + ";\n")
	b.WriteString("\t\tattribute :>> source = " + source.StringText(v.Source) + ";\n")
	b.WriteString("\t\tattribute :>> outcome = Outcome::" + outcome + ";\n")
	b.WriteString("\t\tattribute :>> provenance = " + source.StringText(provenance) + ";\n")
	b.WriteString("\t}\n}\n")
	return b.String()
}

// ResultViewQualifiedName is the qualified name of the view Write declares.
func (v ResultView) ResultViewQualifiedName() string {
	name := v.Name
	if name == "" {
		name = DefaultResultViewName
	}
	return source.QualifiedNameOf([]string{upperFirst(name) + "Result", lowerFirst(name)})
}

// exposedName writes element for the view's body, from the root (`$::`) where
// the view, the library or an inherited feature would capture its first segment.
func exposedName(context Context, view, element string) string {
	text := source.QualifiedNameText(element)
	if segments := symbols.SegmentsOf(element); len(segments) > 0 && capturesName(context, view, segments[0]) {
		return "$::" + text
	}
	return text
}

func capturesName(context Context, view, name string) bool {
	if name == view {
		return true
	}
	if context.Index == nil {
		return false
	}
	if len(context.Index.LookupQualified(ResultViewLibrary+"::"+name)) > 0 {
		return true
	}
	for _, definition := range context.Index.LookupQualified(ResultViewDefinition) {
		if context.Model == nil {
			break
		}
		if _, inherited := context.Model.LookupMember(definition, name); inherited {
			return true
		}
	}
	return false
}

func unnamedNote(n int) string {
	if n == 1 {
		return "1 selected element has no qualified name to expose it by."
	}
	return strconv.Itoa(n) + " selected elements have no qualified name to expose them by."
}

func joinProvenance(base, step string) string {
	if base == "" {
		return step
	}
	return base + "; " + step
}

// outcomeElements is what a failed run exposes: where each path failure stopped
// and what each finding is about, those the model declares, in record order.
func outcomeElements(index *symbols.Index, outcome Outcome) []string {
	var out []string
	add := func(name string) {
		if name != "" && index != nil && len(index.LookupQualified(name)) > 0 {
			out = appendDistinct(out, name)
		}
	}
	if outcome.Model != nil {
		for _, finding := range outcome.Model.Findings {
			add(finding.Element)
		}
		for _, failure := range outcome.Model.PathFailures {
			add(failure.Frontier.At)
		}
	}
	if outcome.Path != nil {
		for _, failure := range outcome.Path.Failures {
			add(failure.Frontier.At)
		}
	}
	return out
}

func appendDistinct(xs []string, x string) []string {
	for _, have := range xs {
		if have == x {
			return xs
		}
	}
	return append(xs, x)
}

func upperFirst(s string) string { return mapFirst(s, unicode.ToUpper) }

func lowerFirst(s string) string { return mapFirst(s, unicode.ToLower) }

func mapFirst(s string, f func(rune) rune) string {
	for i, r := range s {
		return string(f(r)) + s[i+len(string(r)):]
	}
	return s
}

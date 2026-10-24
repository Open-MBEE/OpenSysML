// Package codes is the registry of diagnostic codes: every code a diagnostic
// of the parser, the semantic layer, the checker or the runtime carries, with
// the rule it names, how to satisfy the rule and the specification clause or
// validation constraint behind it. The CLI (-explain), the REPL (%explain-code),
// the MCP server and the language server's code descriptions read it, and the
// catalogue in docs/reference/diagnostics.md is generated from it.
package codes

import (
	"fmt"
	"sort"
	"strings"
)

// Tier names the pass family that reports a code, in pipeline order.
type Tier string

// The tiers, in the order the catalogue lists them.
const (
	TierSyntax     Tier = "syntax"
	TierNameRes    Tier = "name-resolution"
	TierType       Tier = "type"
	TierConstraint Tier = "constraint"
	TierLint       Tier = "lint"
	TierMethod     Tier = "method"
	TierDocument   Tier = "document"
	TierWorkspace  Tier = "workspace"
	TierRuntime    Tier = "runtime"
)

// Tiers lists the tiers in catalogue order.
func Tiers() []Tier {
	return []Tier{TierSyntax, TierNameRes, TierType, TierConstraint, TierLint, TierMethod, TierDocument, TierWorkspace, TierRuntime}
}

// tierTitles are the catalogue headings of the tiers.
var tierTitles = map[Tier]string{
	TierSyntax:     "Syntax",
	TierNameRes:    "Name resolution",
	TierType:       "Typing",
	TierConstraint: "Constraints",
	TierLint:       "Lints",
	TierMethod:     "Method rules",
	TierDocument:   "Documents and queries",
	TierWorkspace:  "Workspace",
	TierRuntime:    "Runtime",
}

// Title is the catalogue heading of a tier.
func (t Tier) Title() string { return tierTitles[t] }

// Entry describes one diagnostic code.
type Entry struct {
	// Code is the stable identifier a diagnostic carries, kebab-case.
	Code string
	// Tier is the pass family that reports it.
	Tier Tier
	// Severity is the severity the diagnostic is reported with: "error" or
	// "warning". A code reported at both is listed at the more severe.
	Severity string
	// Summary is the one-line description the listings show.
	Summary string
	// Meaning explains what the diagnostic means and how to satisfy the rule.
	Meaning string
	// Spec names the specification clause or validation constraint the rule
	// comes from, empty for a rule of OpenSysML's own.
	Spec string
	// Kinds, for a family code, are the suffixes the emitted code appends to
	// Code after a dash, each naming one rule of the family.
	Kinds []string
}

// IsFamily reports whether the entry stands for a family of codes `Code-<kind>`.
func (e Entry) IsFamily() bool { return len(e.Kinds) > 0 }

// CatalogueURL is the published catalogue every code is a section of.
const CatalogueURL = "https://runtime.opensysml.org/reference/diagnostics/"

// URL is the catalogue section of a code, what an editor opens from a
// diagnostic's code; a member of a family opens the family's section.
func URL(code string) string {
	if e, _, ok := Lookup(code); ok {
		code = e.Code
	}
	return CatalogueURL + "#" + Anchor(code)
}

// Lookup finds the entry of a code. A member of a family (`document-plan-missing-title`)
// finds the family's entry with kind set to the member's suffix; kind is empty
// otherwise. The second result is false for a code the registry does not know.
func Lookup(code string) (entry Entry, kind string, ok bool) {
	if e, found := byCode[code]; found {
		return e, "", true
	}
	for _, e := range families {
		if !strings.HasPrefix(code, e.Code+"-") {
			continue
		}
		suffix := code[len(e.Code)+1:]
		for _, k := range e.Kinds {
			if k == suffix {
				return e, suffix, true
			}
		}
	}
	return Entry{}, "", false
}

// Known reports whether code is registered, a family member included.
func Known(code string) bool {
	_, _, ok := Lookup(code)
	return ok
}

// All lists the entries in catalogue order: by tier, then by code.
func All() []Entry {
	return append([]Entry(nil), ordered...)
}

// ByTier lists the entries of one tier, by code.
func ByTier(t Tier) []Entry {
	var out []Entry
	for _, e := range ordered {
		if e.Tier == t {
			out = append(out, e)
		}
	}
	return out
}

// Codes lists every code the registry knows, families expanded, sorted.
func Codes() []string {
	var out []string
	for _, e := range ordered {
		if e.IsFamily() {
			for _, k := range e.Kinds {
				out = append(out, e.Code+"-"+k)
			}
			continue
		}
		out = append(out, e.Code)
	}
	sort.Strings(out)
	return out
}

// Explain writes the explanation of a code as `sysml -explain` prints it. The
// error names a code the registry does not know.
func Explain(code string) (string, error) {
	e, kind, ok := Lookup(code)
	if !ok {
		return "", fmt.Errorf("unknown diagnostic code %q; `sysml -explain` lists them", code)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (%s)\n", code, e.Severity, e.Tier)
	fmt.Fprintf(&b, "  %s\n\n", e.Summary)
	b.WriteString(wrap(e.Meaning, 78))
	if kind != "" {
		fmt.Fprintf(&b, "\nThis member of the family names the rule `%s`; the message states what it found.\n", kind)
	}
	if e.Spec != "" {
		fmt.Fprintf(&b, "\nSpecification: %s\n", e.Spec)
	}
	return b.String(), nil
}

// Row is one line of a listing: an emitted code with its severity, tier and
// summary. A family contributes one row per member, named after the rule.
type Row struct {
	Code     string
	Severity string
	Tier     Tier
	Summary  string
}

// Rows lists every code in catalogue order, each family as its base code and
// then its members, so a listing names each code `sysml -explain <code>` accepts.
func Rows() []Row {
	var out []Row
	for _, e := range ordered {
		out = append(out, Row{Code: e.Code, Severity: e.Severity, Tier: e.Tier, Summary: e.Summary})
		// A family's base code is emitted too, for an error outside its kinds.
		for _, k := range e.Kinds {
			out = append(out, Row{Code: e.Code + "-" + k, Severity: e.Severity, Tier: e.Tier, Summary: e.Summary + " (rule " + k + ")"})
		}
	}
	return out
}

// List writes every code with its one-line summary, as `sysml -explain` with
// no code prints them: grouped by tier, in catalogue order.
func List() string {
	var b strings.Builder
	rows := Rows()
	for _, t := range Tiers() {
		first := true
		for _, r := range rows {
			if r.Tier != t {
				continue
			}
			if first {
				fmt.Fprintf(&b, "%s\n", t.Title())
				first = false
			}
			fmt.Fprintf(&b, "  %-56s %-8s %s\n", r.Code, r.Severity, r.Summary)
		}
	}
	return b.String()
}

// wrap breaks text into lines of at most width columns at spaces.
func wrap(text string, width int) string {
	var b strings.Builder
	for _, para := range strings.Split(text, "\n") {
		line := 0
		for _, word := range strings.Fields(para) {
			if line > 0 && line+1+len(word) > width {
				b.WriteString("\n")
				line = 0
			}
			if line > 0 {
				b.WriteString(" ")
				line++
			}
			b.WriteString(word)
			line += len(word)
		}
		b.WriteString("\n")
	}
	return b.String()
}

var (
	byCode   = map[string]Entry{}
	families []Entry
	ordered  []Entry
)

func init() {
	tierOrder := map[Tier]int{}
	for i, t := range Tiers() {
		tierOrder[t] = i
	}
	for _, table := range [][]Entry{syntaxCodes, nameResolutionCodes, typeCodes, constraintCodes, lintCodes, methodCodes, documentCodes, workspaceCodes, runtimeCodes} {
		for _, e := range table {
			if _, dup := byCode[e.Code]; dup {
				panic("codes: " + e.Code + " registered twice")
			}
			byCode[e.Code] = e
			if e.IsFamily() {
				families = append(families, e)
			}
			ordered = append(ordered, e)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Tier != ordered[j].Tier {
			return tierOrder[ordered[i].Tier] < tierOrder[ordered[j].Tier]
		}
		return ordered[i].Code < ordered[j].Code
	})
}

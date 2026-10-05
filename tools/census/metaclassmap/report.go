// Package metaclassmap compares production shapes in the OMG Xtext grammars
// with OpenSysML's reflective model and existing grammar-coverage evidence.
// Its findings are advisory: evidence is input presence, not parser execution,
// and the differential uses source-location heuristics rather than a trace.
package metaclassmap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/census/grammar"
)

// Report holds every production's grammar shape, reflective checks, and
// grammar-to-input differential.
type Report struct {
	PilotTag string          `json:"pilotTag"`
	Totals   Totals          `json:"totals"`
	Grammars []GrammarReport `json:"grammars"`
}

// Totals aggregates shape, reflective-check, and differential counts.
type Totals struct {
	Productions         int            `json:"productions"`
	ProductionsByKind   map[string]int `json:"productionsByKind"`
	ReturnsDefaulted    int            `json:"returnsDefaulted"`
	Datatype            int            `json:"datatype"`
	TypesByStatus       map[string]int `json:"typesByStatus"`
	Assignments         int            `json:"assignments"`
	AssignmentsFound    int            `json:"assignmentsFound"`
	AssignmentsMissing  int            `json:"assignmentsMissing"`
	EnumLiteralsFound   int            `json:"enumLiteralsFound"`
	EnumLiteralsMissing int            `json:"enumLiteralsMissing"`
	Differential        map[string]int `json:"differential"`
	UndecidedByReason   map[string]int `json:"undecidedByReason"`
}

// GrammarReport holds one grammar's productions in declaration order.
type GrammarReport struct {
	Name        string       `json:"name"`
	Totals      Totals       `json:"totals"`
	Productions []Production `json:"productions"`
}

// Production combines one grammar shape with its coverage row and findings.
type Production struct {
	Shape        grammar.Shape     `json:"shape"`
	Coverage     grammar.Row       `json:"coverage"`
	Types        []TypeCheck       `json:"types,omitempty"`
	Assignments  []AssignmentCheck `json:"assignmentChecks,omitempty"`
	Differential Differential      `json:"differential"`
}

// TypeCheck records how a qualified grammar type resolved.
type TypeCheck struct {
	Context string `json:"context"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	FQN     string `json:"fqn,omitempty"`
}

// AssignmentCheck records a feature check for one possible owner.
type AssignmentCheck struct {
	Feature    string `json:"feature"`
	Op         string `json:"op"`
	Value      string `json:"value"`
	Owner      string `json:"owner"`
	DeclaredBy string `json:"declaredBy,omitempty"`
	Status     string `json:"status"`
	Enum       bool   `json:"enum,omitempty"`
}

// Location identifies the grammar-coverage citation used by the differential.
type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Differential is the single evidence bucket assigned to a production.
type Differential struct {
	Bucket     string    `json:"bucket"`
	Reason     string    `json:"reason,omitempty"`
	Location   *Location `json:"location,omitempty"`
	Symbol     string    `json:"symbol,omitempty"`
	SymbolFQN  string    `json:"symbolFqn,omitempty"`
	Metaclass  string    `json:"metaclass,omitempty"`
	Creates    []string  `json:"creates,omitempty"`
	SharedWith []string  `json:"sharedWith,omitempty"`
}

// Summary is the compact baseline: counts and actionable findings, without
// repeating every production row.
type Summary struct {
	PilotTag             string               `json:"pilotTag"`
	Totals               Totals               `json:"totals"`
	Grammars             []GrammarTotals      `json:"grammars"`
	MissingTypes         []TypeFinding        `json:"missingTypes"`
	MissingFeatures      []FeatureFinding     `json:"missingFeatures"`
	MissingEnumLiterals  []EnumLiteralFinding `json:"missingEnumLiterals"`
	DefaultedReturnRules []DefaultedReturn    `json:"defaultedReturnRules"`
	Disagreements        []Disagreement       `json:"disagreements"`
}

// GrammarTotals is a grammar's counts in a compact baseline.
type GrammarTotals struct {
	Name   string `json:"name"`
	Totals Totals `json:"totals"`
}

// TypeFinding is one unresolved grammar type reference.
type TypeFinding struct {
	Grammar    string `json:"grammar"`
	Production string `json:"production"`
	Line       int    `json:"line"`
	Context    string `json:"context"`
	Name       string `json:"name"`
}

// FeatureFinding is one assignment owner that lacks the assigned feature.
type FeatureFinding struct {
	Grammar    string `json:"grammar"`
	Production string `json:"production"`
	Line       int    `json:"line"`
	Feature    string `json:"feature"`
	Owner      string `json:"owner"`
}

// EnumLiteralFinding is one grammar literal absent from its declared enum.
type EnumLiteralFinding struct {
	Grammar    string `json:"grammar"`
	Production string `json:"production"`
	Line       int    `json:"line"`
	Literal    string `json:"literal"`
	Returns    string `json:"returns"`
}

// DefaultedReturn records a non-datatype production using Xtext's rule-name
// default return.
type DefaultedReturn struct {
	Grammar    string `json:"grammar"`
	Production string `json:"production"`
	Line       int    `json:"line"`
	Returns    string `json:"returns"`
}

// Disagreement is one located production whose OpenSysML metaclass differs
// from the metaclass predicted by the grammar.
type Disagreement struct {
	Grammar    string   `json:"grammar"`
	Production string   `json:"production"`
	Location   Location `json:"location"`
	SymbolFQN  string   `json:"symbolFqn"`
	Metaclass  string   `json:"metaclass"`
	Creates    []string `json:"creates"`
}

func newTotals() Totals {
	return Totals{
		ProductionsByKind: map[string]int{
			string(grammar.KindRule): 0, string(grammar.KindFragment): 0,
			string(grammar.KindEnum): 0, string(grammar.KindTerminal): 0,
		},
		TypesByStatus: map[string]int{
			"ecore": 0, "sysml": 0, "kerml": 0, "missing": 0,
		},
		Differential: map[string]int{
			"agree": 0, "disagree": 0, "undecided": 0,
		},
		UndecidedByReason: map[string]int{
			"no-input": 0, "no-element": 0, "no-anchor": 0, "unlocated": 0,
			"no-element-at-anchor": 0, "no-reflective-metaclass": 0,
			"relationship-not-reified": 0, "anchor-shared": 0,
		},
	}
}

func (t *Totals) add(p Production) {
	t.Productions++
	t.ProductionsByKind[string(p.Shape.Kind)]++
	if p.Shape.ReturnsDefaulted {
		t.ReturnsDefaulted++
	}
	if p.Shape.Datatype {
		t.Datatype++
	}
	for _, check := range p.Types {
		t.TypesByStatus[check.Status]++
	}
	for _, check := range p.Assignments {
		if check.Enum {
			continue
		}
		t.Assignments++
		if check.Status == "found" {
			t.AssignmentsFound++
		} else if check.Status == "missing" {
			t.AssignmentsMissing++
		}
	}
	for _, check := range p.Assignments {
		if !check.Enum {
			continue
		}
		if check.Status == "found" {
			t.EnumLiteralsFound++
		} else if check.Status == "missing" {
			t.EnumLiteralsMissing++
		}
	}
	t.Differential[p.Differential.Bucket]++
	if p.Differential.Bucket == "undecided" {
		t.UndecidedByReason[p.Differential.Reason]++
	}
}

// Summary returns the deterministic compact representation used as baseline.
func (r *Report) Summary() *Summary {
	out := &Summary{
		PilotTag:             r.PilotTag,
		Totals:               r.Totals,
		MissingTypes:         []TypeFinding{},
		MissingFeatures:      []FeatureFinding{},
		MissingEnumLiterals:  []EnumLiteralFinding{},
		DefaultedReturnRules: []DefaultedReturn{},
		Disagreements:        []Disagreement{},
	}
	for _, g := range r.Grammars {
		out.Grammars = append(out.Grammars, GrammarTotals{Name: g.Name, Totals: g.Totals})
		for _, p := range g.Productions {
			for _, check := range p.Types {
				if check.Status == "missing" {
					out.MissingTypes = append(out.MissingTypes, TypeFinding{
						Grammar: p.Shape.Grammar, Production: p.Shape.Name, Line: p.Shape.Line,
						Context: check.Context, Name: check.Name,
					})
				}
			}
			for _, check := range p.Assignments {
				if check.Status != "missing" {
					continue
				}
				if check.Enum {
					out.MissingEnumLiterals = append(out.MissingEnumLiterals, EnumLiteralFinding{
						Grammar: p.Shape.Grammar, Production: p.Shape.Name, Line: p.Shape.Line,
						Literal: check.Feature, Returns: p.Shape.Returns,
					})
				} else {
					out.MissingFeatures = append(out.MissingFeatures, FeatureFinding{
						Grammar: p.Shape.Grammar, Production: p.Shape.Name, Line: p.Shape.Line,
						Feature: check.Feature, Owner: check.Owner,
					})
				}
			}
			if p.Shape.Kind == grammar.KindRule && p.Shape.ReturnsDefaulted && !p.Shape.Datatype {
				out.DefaultedReturnRules = append(out.DefaultedReturnRules, DefaultedReturn{
					Grammar: p.Shape.Grammar, Production: p.Shape.Name, Line: p.Shape.Line,
					Returns: p.Shape.Returns,
				})
			}
			if p.Differential.Bucket == "disagree" && p.Differential.Location != nil {
				out.Disagreements = append(out.Disagreements, Disagreement{
					Grammar: p.Shape.Grammar, Production: p.Shape.Name,
					Location: *p.Differential.Location, SymbolFQN: p.Differential.SymbolFQN,
					Metaclass: p.Differential.Metaclass, Creates: p.Differential.Creates,
				})
			}
		}
	}
	return out
}

func writeReports(dir string, report *Report, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{
		"grammar-metaclass-map.json":      append(encoded, '\n'),
		"grammar-metaclass-map-tables.md": []byte(report.Markdown()),
		"grammar-metaclass-map.txt":       []byte(report.Text()),
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, files[name], 0o600); err != nil {
			return err
		}
		fmt.Fprintf(log, "wrote %s\n", path)
	}
	return nil
}

func writeBaseline(path string, report *Report, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report.Summary(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(log, "wrote %s\n", path)
	return nil
}

func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Grammar-to-metaclass map (advisory)\nOMG Xtext grammars at pilot tag %s\n\n", r.PilotTag)
	fmt.Fprintf(&b, "Productions: %d\n", r.Totals.Productions)
	writeCountMap(&b, "Production kinds", r.Totals.ProductionsByKind)
	fmt.Fprintf(&b, "Defaulted returns: %d\nDatatype productions: %d\n", r.Totals.ReturnsDefaulted, r.Totals.Datatype)
	writeCountMap(&b, "Types by status", r.Totals.TypesByStatus)
	fmt.Fprintf(&b, "Missing types: %d\n", r.Totals.TypesByStatus["missing"])
	fmt.Fprintf(&b, "Assignments: %d total, %d found, %d missing\n",
		r.Totals.Assignments, r.Totals.AssignmentsFound, r.Totals.AssignmentsMissing)
	fmt.Fprintf(&b, "Enum literals: %d total, %d found, %d missing\n",
		r.Totals.EnumLiteralsFound+r.Totals.EnumLiteralsMissing,
		r.Totals.EnumLiteralsFound, r.Totals.EnumLiteralsMissing)
	defaulted := 0
	for _, g := range r.Grammars {
		for _, p := range g.Productions {
			if (p.Shape.Kind == grammar.KindRule || p.Shape.Kind == grammar.KindFragment) &&
				p.Shape.ReturnsDefaulted && !p.Shape.Datatype {
				defaulted++
			}
		}
	}
	fmt.Fprintf(&b, "Defaulted non-datatype rules: %d\n", defaulted)
	writeCountMap(&b, "Differential", r.Totals.Differential)
	writeCountMap(&b, "Undecided reasons", r.Totals.UndecidedByReason)
	return b.String()
}

func writeCountMap(b *strings.Builder, title string, counts map[string]int) {
	fmt.Fprintf(b, "\n%s\n", title)
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "  %s: %d\n", key, counts[key])
	}
}

func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Grammar-to-metaclass map\n\nPilot tag: `%s`.\n\n", r.PilotTag)
	for _, g := range r.Grammars {
		fmt.Fprintf(&b, "## %s\n\n| Production | Kind | Returns | Creates | Assignments | Differential | Location |\n|---|---|---|---|---|---|---|\n", g.Name)
		for _, p := range g.Productions {
			loc := ""
			if p.Differential.Location != nil {
				loc = fmt.Sprintf("%s:%d:%d", p.Differential.Location.File,
					p.Differential.Location.Line, p.Differential.Location.Column)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n",
				mdCell(p.Shape.Name), p.Shape.Kind, mdCell(p.Shape.Returns),
				mdCell(strings.Join(p.Shape.Creates, ", ")), mdCell(formatAssignments(p)),
				p.Differential.Bucket, mdCell(loc))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatAssignments(p Production) string {
	var out []string
	for _, check := range p.Assignments {
		mark := "✗"
		if check.Status == "found" {
			mark = "✓"
		}
		out = append(out, fmt.Sprintf("%s %s %s → %s::%s %s", check.Feature, check.Op, check.Value,
			check.Owner, check.Feature, mark))
	}
	return strings.Join(out, "<br>")
}

func mdCell(value string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ").Replace(value)
}

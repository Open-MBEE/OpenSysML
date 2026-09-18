package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Report is the machine-readable result: every production of every grammar with
// its bucket and, where there is one, the input that is evidence for it.
type Report struct {
	PilotTag string          `json:"pilotTag"`
	Roots    []RootStat      `json:"roots"`
	Totals   BucketTotals    `json:"totals"`
	Grammars []GrammarReport `json:"grammars"`
}

// BucketTotals counts productions per bucket. Productions is the denominator no
// ratio may be quoted without.
type BucketTotals struct {
	Productions       int `json:"productions"`
	Evidence          int `json:"evidence"`
	NoEvidence        int `json:"noEvidence"`
	Indistinguishable int `json:"indistinguishable"`
	// Forms counts the literal-bearing alternatives and optional groups inside
	// the productions; UnseenForms those no input has the literals for.
	Forms       int `json:"forms"`
	UnseenForms int `json:"unseenForms"`
}

func (t *BucketTotals) add(row Row) {
	t.Productions++
	switch row.Bucket {
	case BucketEvidence:
		t.Evidence++
	case BucketNoEvidence:
		t.NoEvidence++
	case BucketIndistinguishable:
		t.Indistinguishable++
	}
	t.Forms += len(row.Branches)
	t.UnseenForms += len(row.UnseenBranches())
}

// GrammarReport holds one grammar file's productions, ordered by declaration
// site.
type GrammarReport struct {
	Name        string       `json:"name"`
	Totals      BucketTotals `json:"totals"`
	Productions []Row        `json:"productions"`
}

func buildReport(pilotTag string, grammars []GrammarReport, roots []RootStat) *Report {
	report := &Report{PilotTag: pilotTag, Roots: roots, Grammars: grammars}
	for i := range report.Grammars {
		grammar := &report.Grammars[i]
		grammar.Totals = BucketTotals{}
		for _, row := range grammar.Productions {
			grammar.Totals.add(row)
			report.Totals.add(row)
		}
	}
	return report
}

// Summary is the committed baseline: the counts, plus only the rows a reader has
// to look at — the no-evidence productions and the ones with an unseen form.
// The full per-production table stays in the generated report under build/.
func (r *Report) Summary() *Report {
	out := &Report{PilotTag: r.PilotTag, Roots: r.Roots, Totals: r.Totals}
	for _, grammar := range r.Grammars {
		kept := GrammarReport{Name: grammar.Name, Totals: grammar.Totals}
		for _, row := range grammar.Productions {
			if row.Bucket == BucketNoEvidence || len(row.UnseenBranches()) > 0 {
				kept.Productions = append(kept.Productions, row)
			}
		}
		out.Grammars = append(out.Grammars, kept)
	}
	return out
}

// writeBaseline writes the compact JSON a later run is diffed against.
func writeBaseline(path string, report *Report) error {
	encoded, err := json.MarshalIndent(report.Summary(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return nil
}

// writeReports writes the full JSON, the text summary and the Markdown tables.
func writeReports(dir string, report *Report) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{
		"grammar-coverage-tables.md": []byte(report.Markdown()),
		"grammar-coverage.json":      append(encoded, '\n'),
		"grammar-coverage.txt":       []byte(report.Text()),
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
		fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	}
	return nil
}

// Text renders the human summary.
func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Grammar production coverage (input-presence evidence, not execution coverage)\n")
	fmt.Fprintf(&b, "OMG Xtext grammars at pilot tag %s\n\n", r.PilotTag)

	fmt.Fprintf(&b, "Corpora searched\n")
	for _, root := range r.Roots {
		fmt.Fprintf(&b, "  %-24s %5d file(s) %8d line(s)  %s\n", root.Name, root.Files, root.Lines, root.Dir)
	}
	fmt.Fprintf(&b, "\nBuckets\n")
	fmt.Fprintf(&b, "  %-22s %11s %11s %11s %19s\n", "grammar", "productions", "evidence", "no-evidence", "indistinguishable")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "  %-22s %11d %11d %11d %19d\n", grammar.Name,
			grammar.Totals.Productions, grammar.Totals.Evidence, grammar.Totals.NoEvidence, grammar.Totals.Indistinguishable)
	}
	fmt.Fprintf(&b, "  %-22s %11d %11d %11d %19d\n", "total",
		r.Totals.Productions, r.Totals.Evidence, r.Totals.NoEvidence, r.Totals.Indistinguishable)

	fmt.Fprintf(&b, "\nForms (alternatives and optional groups inside the productions)\n")
	fmt.Fprintf(&b, "  %-22s %11s %11s\n", "grammar", "forms", "unseen")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "  %-22s %11d %11d\n", grammar.Name, grammar.Totals.Forms, grammar.Totals.UnseenForms)
	}
	fmt.Fprintf(&b, "  %-22s %11d %11d\n", "total", r.Totals.Forms, r.Totals.UnseenForms)

	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "\n%s: no-evidence productions\n", grammar.Name)
		empty := true
		for _, row := range grammar.Productions {
			if row.Bucket != BucketNoEvidence {
				continue
			}
			empty = false
			fmt.Fprintf(&b, "  %s:%d %s (%s) missing %s\n", grammar.Name, row.Line, row.Name, row.Kind, quoteAll(row.Missing))
		}
		if empty {
			fmt.Fprintf(&b, "  (none)\n")
		}
	}

	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "\n%s: unseen forms\n", grammar.Name)
		empty := true
		for _, row := range grammar.Productions {
			for _, branch := range row.UnseenBranches() {
				empty = false
				fmt.Fprintf(&b, "  %s:%d %s (%s) needs %s%s\n", grammar.Name, row.Line, row.Name, row.Kind,
					quoteAll(branch.Literals), missingNote(branch))
			}
		}
		if empty {
			fmt.Fprintf(&b, "  (none)\n")
		}
	}
	return b.String()
}

// Markdown renders the tables docs/project/grammar-coverage-tables.md carries.
func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Grammar production coverage: tables\n\n")
	fmt.Fprintf(&b, "<!-- Generated by `go run ./cmd/grammar-coverage`; do not edit by hand. -->\n")
	fmt.Fprintf(&b, "\nInput-presence evidence against the OMG Xtext grammars at pilot tag `%s`, not\n"+
		"execution coverage: see docs/project/grammar-coverage.md for the method, the\n"+
		"caveats and the adjudication.\n", r.PilotTag)
	fmt.Fprintf(&b, "\n## Bucket counts\n\n")
	fmt.Fprintf(&b, "| Grammar | Productions | evidence | no-evidence | indistinguishable | Forms | unseen forms |\n"+
		"|---|---:|---:|---:|---:|---:|---:|\n")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %d | %d |\n", grammar.Name,
			grammar.Totals.Productions, grammar.Totals.Evidence, grammar.Totals.NoEvidence,
			grammar.Totals.Indistinguishable, grammar.Totals.Forms, grammar.Totals.UnseenForms)
	}
	fmt.Fprintf(&b, "| **total** | **%d** | **%d** | **%d** | **%d** | **%d** | **%d** |\n",
		r.Totals.Productions, r.Totals.Evidence, r.Totals.NoEvidence, r.Totals.Indistinguishable,
		r.Totals.Forms, r.Totals.UnseenForms)

	fmt.Fprintf(&b, "\n## Corpora searched\n\n")
	fmt.Fprintf(&b, "| Root | Directory | Files | Lines |\n|---|---|---:|---:|\n")
	for _, root := range r.Roots {
		fmt.Fprintf(&b, "| %s | `%s` | %d | %d |\n", root.Name, root.Dir, root.Files, root.Lines)
	}

	fmt.Fprintf(&b, "\n## No-evidence productions\n")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "\n### `%s`\n\n", grammar.Name)
		fmt.Fprintf(&b, "| Production | Kind | Declared | Missing literals |\n|---|---|---|---|\n")
		rows := 0
		for _, row := range grammar.Productions {
			if row.Bucket != BucketNoEvidence {
				continue
			}
			rows++
			fmt.Fprintf(&b, "| `%s` | %s | `%s:%d` | %s |\n", row.Name, row.Kind, grammar.Name, row.Line, markdownLiterals(row.Missing))
		}
		if rows == 0 {
			fmt.Fprintf(&b, "| _(none)_ | | | |\n")
		}
	}

	fmt.Fprintf(&b, "\n## Unseen forms\n")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "\n### `%s`\n\n", grammar.Name)
		fmt.Fprintf(&b, "| Production | Declared | Form needs | Never in the corpora |\n|---|---|---|---|\n")
		rows := 0
		for _, row := range grammar.Productions {
			for _, branch := range row.UnseenBranches() {
				rows++
				fmt.Fprintf(&b, "| `%s` | `%s:%d` | %s | %s |\n", row.Name, grammar.Name, row.Line,
					markdownLiterals(branch.Literals), markdownLiterals(branch.Missing))
			}
		}
		if rows == 0 {
			fmt.Fprintf(&b, "| _(none)_ | | | |\n")
		}
	}

	fmt.Fprintf(&b, "\n## Every production\n")
	for _, grammar := range r.Grammars {
		fmt.Fprintf(&b, "\n### `%s`\n\n", grammar.Name)
		fmt.Fprintf(&b, "| Production | Kind | Declared | Bucket | Forms (unseen) | Required literals | Evidence |\n|---|---|---|---|---|---|---|\n")
		for _, row := range grammar.Productions {
			fmt.Fprintf(&b, "| `%s`%s | %s | `%s:%d` | %s | %d (%d) | %s | %s |\n",
				row.Name, overrideMark(row), row.Kind, grammar.Name, row.Line, row.Bucket,
				len(row.Branches), len(row.UnseenBranches()),
				markdownLiterals(row.Required), markdownEvidence(row))
		}
	}
	return b.String()
}

// missingNote names the form's literals that occur in no corpus file at all.
func missingNote(branch Branch) string {
	if len(branch.Missing) == 0 {
		return " (all of them occur, never together)"
	}
	return ", never seen: " + quoteAll(branch.Missing)
}

func overrideMark(row Row) string {
	if row.Override {
		return " (`@Override`)"
	}
	return ""
}

func markdownLiterals(literals []string) string {
	if len(literals) == 0 {
		return "—"
	}
	quoted := make([]string, 0, len(literals))
	for _, lit := range literals {
		quoted = append(quoted, "`"+strings.ReplaceAll(lit, "|", "\\|")+"`")
	}
	return strings.Join(quoted, " ")
}

// markdownEvidence renders the first corpus occurrence of each required literal,
// or the reason literal search could not decide.
func markdownEvidence(row Row) string {
	if row.Bucket == BucketIndistinguishable {
		return row.Reason
	}
	if len(row.Evidence) == 0 {
		return "—"
	}
	cited := make([]string, 0, len(row.Evidence))
	for _, citation := range row.Evidence {
		literal := strings.ReplaceAll(citation.Literal, "|", "\\|")
		cited = append(cited, fmt.Sprintf("`%s` at `%s:%d`", literal, citation.File, citation.Line))
	}
	return strings.Join(cited, "<br>")
}

func quoteAll(literals []string) string {
	if len(literals) == 0 {
		return "(nothing)"
	}
	quoted := make([]string, 0, len(literals))
	for _, lit := range literals {
		quoted = append(quoted, fmt.Sprintf("%q", lit))
	}
	return strings.Join(quoted, " ")
}

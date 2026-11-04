package notation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

const (
	baselinePath    = "docs/project/graphical-notation-census-baseline.json"
	censusDocPath   = "docs/project/graphical-notation-census.md"
	defaultTextPath = "build/spec-docs/sysml.txt"
	specPinPath     = "build/spec-docs/pdf/.pilot-pin"
	rowsPath        = "build/graphical-notation-census/rows.md"
)

// Row is one production with its verdict.
type Row struct {
	Production
	Adjudication
}

// Source records which specification text the census measured.
type Source struct {
	PilotTag      string `json:"pilotTag"`
	ReleaseCommit string `json:"releaseCommit"`
	Text          string `json:"text"`
	Clause        string `json:"clause"`
}

// Counts are the verdict counts of a set of rows.
type Counts struct {
	Productions   int `json:"productions"`
	Drawn         int `json:"drawn"`
	Approximate   int `json:"approximate"`
	Undrawn       int `json:"undrawn"`
	NotApplicable int `json:"notApplicable"`
}

func (c *Counts) add(v Verdict) {
	c.Productions++
	switch v {
	case VerdictDrawn:
		c.Drawn++
	case VerdictApproximate:
		c.Approximate++
	case VerdictUndrawn:
		c.Undrawn++
	case VerdictNotApplicable:
		c.NotApplicable++
	}
}

// Totals are the census-wide figures.
type Totals struct {
	Statements        int `json:"statements"`
	PartialStatements int `json:"partialStatements"`
	Counts
}

// ClauseCounts are one sub-clause's figures.
type ClauseCounts struct {
	Number     string `json:"number"`
	Title      string `json:"title"`
	Statements int    `json:"statements"`
	Counts
}

// Gap is a row the census does not find drawn, with its reason.
type Gap struct {
	Clause   string   `json:"clause"`
	Name     string   `json:"name"`
	Partial  bool     `json:"partial,omitempty"`
	Category Category `json:"category"`
	Verdict  Verdict  `json:"verdict"`
	Package  string   `json:"package,omitempty"`
	Reason   string   `json:"reason"`
}

// Baseline is the committed census: the source, the totals, the per-clause
// and per-category counts, and every row that is not drawn.
type Baseline struct {
	Source     Source         `json:"source"`
	Totals     Totals         `json:"totals"`
	Categories map[string]int `json:"categories"`
	Clauses    []ClauseCounts `json:"clauses"`
	NonDrawn   []Gap          `json:"nonDrawn"`
}

// Census is a measured census: the grammar and one row per production.
type Census struct {
	Grammar *Grammar
	Rows    []Row
}

// Measure extracts the grammar from the rendered text and adjudicates every
// production. Every production needs a verdict and every static verdict a
// production, so a changed specification fails rather than shifts the census.
func Measure(root, text string) (*Census, error) {
	content, err := os.ReadFile(text) // #nosec G304 -- the rendered specification text, named on the command line
	if err != nil {
		return nil, err
	}
	g, err := Extract(string(content))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", text, err)
	}
	edges, err := ReadNotationTable(root)
	if err != nil {
		return nil, err
	}
	c := &Census{Grammar: g}
	var problems []string
	seen := map[string]bool{}
	for _, p := range g.Productions {
		seen[p.Name] = true
		adj, err := Adjudicate(p.Name, edges)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		c.Rows = append(c.Rows, Row{Production: p, Adjudication: adj})
	}
	for _, name := range AdjudicatedNames() {
		if !seen[name] {
			problems = append(problems, fmt.Sprintf("the verdict table adjudicates %s, which §8.2.3 no longer states", name))
		}
	}
	for _, name := range edges.Names() {
		if !seen[name] {
			problems = append(problems, fmt.Sprintf("the notation table adjudicates %s, which §8.2.3 does not state", name))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("the census and the specification text disagree:\n  %s", strings.Join(problems, "\n  "))
	}
	return c, nil
}

// Baseline folds the census into the committed shape under the given source.
func (c *Census) Baseline(source Source) *Baseline {
	b := &Baseline{Source: source, Categories: map[string]int{}}
	b.Totals.Statements = len(c.Grammar.Statements)
	b.Totals.PartialStatements = c.Grammar.Partials()
	byClause := map[string]int{}
	for _, cl := range c.Grammar.Clauses {
		byClause[cl.Number] = len(b.Clauses)
		b.Clauses = append(b.Clauses, ClauseCounts{Number: cl.Number, Title: cl.Title, Statements: cl.Statements})
	}
	for _, r := range c.Rows {
		b.Totals.add(r.Verdict)
		b.Clauses[byClause[r.Clause]].add(r.Verdict)
		b.Categories[string(r.Category)]++
		if r.Verdict != VerdictDrawn {
			b.NonDrawn = append(b.NonDrawn, Gap{Clause: r.Clause, Name: r.Name, Partial: r.Partial, Category: r.Category,
				Verdict: r.Verdict, Package: workPackage(r), Reason: r.Reason})
		}
	}
	return b
}

// Work packages group the gaps for follow-up work, one package each.
var packageTitles = []struct{ Letter, Title string }{
	{"A", "§8.2.3.26 view frame and §8.2.3.4 annotation nodes"},
	{"B", "§8.2.3.17–.18 action and state node forms"},
	{"C", "§8.2.3.3/.13 n-ary dots and §8.2.3.16 flow-on-connection"},
	{"D", "§8.2.3.9 sequence glyphs"},
	{"E", "§8.2.3.2 textual-compartment greying"},
	{"F", "everything else"},
}

func workPackage(r Row) string {
	if r.Verdict != VerdictUndrawn && r.Verdict != VerdictApproximate {
		return ""
	}
	switch {
	case r.Clause == "8.2.3.4", r.Name == "root-view", r.Name == "framed-view", strings.HasPrefix(r.Name, "view-frame-"):
		return "A"
	case r.Clause == "8.2.3.17", r.Clause == "8.2.3.18":
		return "B"
	case strings.HasPrefix(r.Name, "n-ary-"), r.Name == "cdot-label", r.Name == "flow-on-connection",
		strings.HasSuffix(r.Name, "-node-r"), strings.HasSuffix(r.Name, "-node-l"):
		return "C"
	case r.Clause == "8.2.3.9", r.Name == "message":
		return "D"
	case r.Name == "element-inside-textual-compartment", r.Name == "ellipsis-at-lower-left-corner":
		return "E"
	}
	return "F"
}

// ReadSource reads the pins the census records: the pilot tag scripts/pilot-pin.sh
// carries and the release commit the downloaded PDFs were rendered from.
func ReadSource(root, text string) (Source, error) {
	pin, err := baseline.ReadPin(root)
	if err != nil {
		return Source{}, err
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(specPinPath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return Source{}, err
	}
	fields := strings.Fields(string(content))
	if len(fields) < 2 || fields[0] != pin.Tag {
		return Source{}, fmt.Errorf("%s pins %q but %s pins %s: re-run scripts/download-spec-examples.sh", specPinPath, strings.TrimSpace(string(content)), baseline.PinPath, pin.Tag)
	}
	return Source{PilotTag: pin.Tag, ReleaseCommit: fields[1], Text: text, Clause: "8.2.3"}, nil
}

func loadBaseline(root string) (*Baseline, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(content, &b); err != nil {
		return nil, fmt.Errorf("%s: %w", baselinePath, err)
	}
	return &b, nil
}

func encodeBaseline(b *Baseline) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeBaseline(root string, b *Baseline) error {
	data, err := encodeBaseline(b)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(baselinePath)), data, 0o644) // #nosec G306 -- a committed documentation file
}

// validate checks the baseline is internally consistent: known verdicts and
// categories, clause counts summing to the totals, the non-drawn rows sorted
// and matching the counts.
func (b *Baseline) validate() error {
	var problems []string
	if b.Source.PilotTag == "" || b.Source.ReleaseCommit == "" || b.Source.Clause != "8.2.3" {
		problems = append(problems, "the source records no pilot tag, release commit or clause 8.2.3")
	}
	var sum Counts
	var statements int
	for i, c := range b.Clauses {
		if c.Number != fmt.Sprintf("8.2.3.%d", i+1) {
			problems = append(problems, fmt.Sprintf("clause %d is %s", i+1, c.Number))
		}
		if c.Drawn+c.Approximate+c.Undrawn+c.NotApplicable != c.Productions {
			problems = append(problems, fmt.Sprintf("%s: the verdicts do not sum to %d productions", c.Number, c.Productions))
		}
		sum.Productions += c.Productions
		sum.Drawn += c.Drawn
		sum.Approximate += c.Approximate
		sum.Undrawn += c.Undrawn
		sum.NotApplicable += c.NotApplicable
		statements += c.Statements
	}
	if len(b.Clauses) != clauseCount {
		problems = append(problems, fmt.Sprintf("%d clauses, want %d", len(b.Clauses), clauseCount))
	}
	if sum != b.Totals.Counts || statements != b.Totals.Statements {
		problems = append(problems, "the clause counts do not sum to the totals")
	}
	categories := 0
	for name, n := range b.Categories {
		if !knownCategory(Category(name)) {
			problems = append(problems, fmt.Sprintf("unknown category %q", name))
		}
		categories += n
	}
	if categories != b.Totals.Productions {
		problems = append(problems, "the category counts do not sum to the productions")
	}
	var gaps Counts
	for i, g := range b.NonDrawn {
		if g.Verdict == VerdictDrawn || !knownVerdict(g.Verdict) {
			problems = append(problems, fmt.Sprintf("%s: a non-drawn row has verdict %q", g.Name, g.Verdict))
		}
		if !knownCategory(g.Category) {
			problems = append(problems, fmt.Sprintf("%s: unknown category %q", g.Name, g.Category))
		}
		if g.Reason == "" {
			problems = append(problems, fmt.Sprintf("%s: no reason", g.Name))
		}
		if (g.Package != "") != (g.Verdict == VerdictUndrawn || g.Verdict == VerdictApproximate) {
			problems = append(problems, fmt.Sprintf("%s: an undrawn or approximate row has a work package, the others none", g.Name))
		}
		if i > 0 && !gapLess(b.NonDrawn[i-1], g) {
			problems = append(problems, fmt.Sprintf("%s: the non-drawn rows are not sorted by clause then name", g.Name))
		}
		gaps.add(g.Verdict)
	}
	if gaps.Approximate != b.Totals.Approximate || gaps.Undrawn != b.Totals.Undrawn || gaps.NotApplicable != b.Totals.NotApplicable {
		problems = append(problems, "the non-drawn rows do not match the totals")
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s is inconsistent:\n  %s", baselinePath, strings.Join(problems, "\n  "))
}

func knownVerdict(v Verdict) bool {
	for _, k := range Verdicts {
		if k == v {
			return true
		}
	}
	return false
}

func knownCategory(c Category) bool {
	for _, k := range Categories {
		if k == c {
			return true
		}
	}
	return false
}

// gapLess orders gaps by clause number then name, the order the baseline keeps.
func gapLess(a, b Gap) bool {
	if a.Clause != b.Clause {
		return clauseIndex(a.Clause) < clauseIndex(b.Clause)
	}
	return a.Name < b.Name
}

func clauseIndex(number string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(number, clausePrefix))
	if err != nil {
		return 0
	}
	return n
}

func (b *Baseline) sortNonDrawn() {
	sort.Slice(b.NonDrawn, func(i, j int) bool { return gapLess(b.NonDrawn[i], b.NonDrawn[j]) })
}

// compare reports every difference between a fresh measurement and the baseline.
func (b *Baseline) compare(fresh *Baseline) error {
	want, err := encodeBaseline(fresh)
	if err != nil {
		return err
	}
	have, err := encodeBaseline(b)
	if err != nil {
		return err
	}
	if bytes.Equal(want, have) {
		return nil
	}
	var problems []string
	if b.Source != fresh.Source {
		problems = append(problems, fmt.Sprintf("source %+v, measured %+v", b.Source, fresh.Source))
	}
	if b.Totals != fresh.Totals {
		problems = append(problems, fmt.Sprintf("totals %+v, measured %+v", b.Totals, fresh.Totals))
	}
	recorded := map[string]Gap{}
	for _, g := range b.NonDrawn {
		recorded[g.Name] = g
	}
	measured := map[string]Gap{}
	for _, g := range fresh.NonDrawn {
		measured[g.Name] = g
		if r, ok := recorded[g.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s (%s) is %s but the baseline records it drawn", g.Name, g.Clause, g.Verdict))
		} else if r != g {
			problems = append(problems, fmt.Sprintf("%s: recorded %+v, measured %+v", g.Name, r, g))
		}
	}
	for _, g := range b.NonDrawn {
		if _, ok := measured[g.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s (%s) is recorded %s but measured drawn or absent", g.Name, g.Clause, g.Verdict))
		}
	}
	if len(problems) == 0 {
		problems = append(problems, "the clause or category counts differ")
	}
	sort.Strings(problems)
	return fmt.Errorf("%s differs from the measurement; re-record with -measure:\n  %s", baselinePath, strings.Join(problems, "\n  "))
}

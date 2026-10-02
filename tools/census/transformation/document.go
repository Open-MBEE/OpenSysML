package transformation

import (
	"fmt"
	"sort"
	"strings"
)

// The census document carries generated blocks between census:begin/end
// markers: this program writes them and -check refuses a hand-edited block.
var generatedBlocks = []string{"source", "summary", "rows", "beyond", "errata", "gaps"}

func blockMarker(name, which string) string {
	return "<!-- census:" + which + " " + name + " -->"
}

// renderBlock is one generated block's content given the baseline.
func renderBlock(name string, b *Baseline) (string, error) {
	switch name {
	case "source":
		return renderSource(b), nil
	case "summary":
		return renderSummary(b), nil
	case "rows":
		return renderRows(b), nil
	case "beyond":
		return renderBeyond(b), nil
	case "errata":
		return renderErrata(b), nil
	case "gaps":
		return renderGaps(b), nil
	}
	return "", fmt.Errorf("unknown census block %q", name)
}

func renderSource(b *Baseline) string {
	s := b.Source
	line := fmt.Sprintf("**Source:** OMG SysML v1 to v2 transformation model, document `%s`, `%s` (`%s`) — %d packages, %d classes, %d mapping classes, %d OCL2.0 operation bodies",
		s.Document, s.File, s.Digest, s.Packages, s.Classes, s.Mappings, s.OCLBodies)
	if s.OCLSpecifications != s.OCLBodies {
		line += fmt.Sprintf(" (of %d OCL2.0 specifications; the other %d are %d postconditions and %d owned rules)",
			s.OCLSpecifications, s.OCLSpecifications-s.OCLBodies, s.OCLPostconditions, s.OCLOwnedRules)
	}
	return line + "."
}

func renderSummary(b *Baseline) string {
	c := b.counts()
	var out strings.Builder
	fmt.Fprintf(&out, "**Census:** %d mapping classes — %d ✅ faithful, %d ⚠️ approximate, %d ❌ not implemented, %d ⛔ deliberate, %d 🚧 known failure, %d ❔ unknown.\n",
		len(b.Mappings), c[StatusFaithful], c[StatusApproximate], c[StatusNotImplemented],
		c[StatusDeliberate], c[StatusKnownFailure], c[StatusUnknown])
	out.WriteString("\n| Package | Mappings | ✅ | ⚠️ | ❌ | ⛔ | 🚧 | ❔ |\n")
	out.WriteString("|---|---|---|---|---|---|---|---|\n")
	totals := map[string]int{}
	for _, p := range b.packageOrder() {
		per := map[string]int{}
		for _, m := range p.Rows {
			per[m.Status]++
			totals[m.Status]++
		}
		fmt.Fprintf(&out, "| %s | %d | %d | %d | %d | %d | %d | %d |\n",
			escape(p.Name), len(p.Rows), per[StatusFaithful], per[StatusApproximate],
			per[StatusNotImplemented], per[StatusDeliberate], per[StatusKnownFailure], per[StatusUnknown])
	}
	fmt.Fprintf(&out, "| **Total** | %d | %d | %d | %d | %d | %d | %d |",
		len(b.Mappings), totals[StatusFaithful], totals[StatusApproximate],
		totals[StatusNotImplemented], totals[StatusDeliberate], totals[StatusKnownFailure], totals[StatusUnknown])
	return out.String()
}

var rowColumns = "| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |"
var rowSeparator = "|---|---|---|---|---|---|---|"

func renderRows(b *Baseline) string {
	var out strings.Builder
	for i, p := range b.packageOrder() {
		if i > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "### %s (%d)\n\n%s\n%s\n", p.Name, len(p.Rows), rowColumns, rowSeparator)
		for j, m := range p.Rows {
			if j > 0 {
				out.WriteString("\n")
			}
			cell := "`" + m.Name + "`"
			if m.Abstract {
				cell += " (abstract)"
			}
			marker, _ := markerFor(m.Status)
			fmt.Fprintf(&out, "| %s | %s | %s | %s | %s | %s | %s |",
				cell, code(m.From), code(m.To), cites(m.Implementation), cites(m.Tests), marker, orDash(m.Reason))
		}
	}
	return out.String()
}

func renderBeyond(b *Baseline) string {
	var out strings.Builder
	out.WriteString("| Migrator behaviour | Implementation | Test |\n")
	out.WriteString("|---|---|---|")
	for _, e := range b.Beyond {
		fmt.Fprintf(&out, "\n| %s | %s | %s |", escape(e.Behaviour), cites(e.Implementation), cites(e.Tests))
	}
	return out.String()
}

// erratumMarker is the reason prefix that lists a row in the errata block.
const erratumMarker = "candidate erratum:"

// errataRows are the rows whose reason names a candidate erratum, in baseline order.
func errataRows(b *Baseline) []Mapping {
	var rows []Mapping
	for _, m := range b.Mappings {
		if i := strings.Index(strings.ToLower(m.Reason), erratumMarker); i >= 0 {
			rows = append(rows, m)
		}
	}
	return rows
}

func renderErrata(b *Baseline) string {
	var out strings.Builder
	out.WriteString("| OMG mapping | v1 source | v2 target | Status | Note |\n")
	out.WriteString("|---|---|---|---|---|")
	for _, m := range errataRows(b) {
		marker, _ := markerFor(m.Status)
		note := strings.TrimSpace(m.Reason[strings.Index(strings.ToLower(m.Reason), erratumMarker)+len(erratumMarker):])
		fmt.Fprintf(&out, "\n| `%s` | %s | %s | %s | %s |", m.Name, code(m.From), code(m.To), marker, orDash(note))
	}
	return out.String()
}

// renderGaps groups the scoped rows that share one verdict, scope and reason
// into a single ranked entry: dozens of sub-mappings carry the same scope, and
// ranking each separately would repeat the same counts.
func renderGaps(b *Baseline) string {
	type gap struct {
		names  []string
		status string
		scope  []string
		reason string
		pssm   int
		fix    int
		total  int
	}
	groups := map[string]*gap{}
	var order []*gap
	for _, m := range b.Mappings {
		if !scoped(m.Status) {
			continue
		}
		key := m.Status + "\x00" + strings.Join(m.Scope, "\x00") + "\x00" + m.Reason
		g, ok := groups[key]
		if !ok {
			g = &gap{status: m.Status, scope: m.Scope, reason: m.Reason}
			for _, tok := range m.Scope {
				c := b.Measurement.Counts[tok]
				g.pssm += c.PSSM
				g.fix += c.Fixtures
			}
			g.total = g.pssm + g.fix
			groups[key] = g
			order = append(order, g)
		}
		g.names = append(g.names, m.Name)
	}
	var gaps []*gap
	for _, g := range order {
		sort.Strings(g.names)
		gaps = append(gaps, g)
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].total != gaps[j].total {
			return gaps[i].total > gaps[j].total
		}
		return gaps[i].names[0] < gaps[j].names[0]
	})
	var out strings.Builder
	out.WriteString("| Rank | OMG mappings | Status | Scope | PSSM suite | Fixtures | Total | Reason |\n")
	out.WriteString("|---|---|---|---|---|---|---|---|")
	for i, g := range gaps {
		marker, _ := markerFor(g.status)
		names := make([]string, len(g.names))
		for j, n := range g.names {
			names[j] = "`" + n + "`"
		}
		fmt.Fprintf(&out, "\n| %d | %d: %s | %s | %s | %d | %d | %d | %s |",
			i+1, len(g.names), strings.Join(names, ", "), marker, cites(g.scope), g.pssm, g.fix, g.total, orDash(g.reason))
	}
	return out.String()
}

func escape(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

func code(s string) string {
	if s == "" {
		return "—"
	}
	return "`" + escape(s) + "`"
}

// cites renders a list of repository citations as backticked spans joined by <br>.
func cites(list []string) string {
	if len(list) == 0 {
		return "—"
	}
	rendered := make([]string, len(list))
	for i, c := range list {
		rendered[i] = "`" + escape(c) + "`"
	}
	return strings.Join(rendered, "<br>")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return escape(s)
}

// rewriteBlocks replaces every marked block's content with the generated text
// and leaves every other byte of the document alone.
func rewriteBlocks(content string, b *Baseline) (string, error) {
	for _, name := range generatedBlocks {
		rendered, err := renderBlock(name, b)
		if err != nil {
			return "", err
		}
		begin := blockMarker(name, "begin")
		end := blockMarker(name, "end")
		bi := strings.Index(content, begin)
		ei := strings.Index(content, end)
		if bi < 0 || ei < 0 || ei < bi {
			return "", fmt.Errorf("%s: missing or misordered %s / %s markers", censusDocPath, begin, end)
		}
		if strings.Contains(content[bi+len(begin):ei], begin) || strings.Count(content, begin) != 1 || strings.Count(content, end) != 1 {
			return "", fmt.Errorf("%s: the census:%s block markers repeat or nest", censusDocPath, name)
		}
		content = content[:bi+len(begin)] + "\n" + rendered + "\n" + content[ei:]
	}
	return content, nil
}

// rowName extracts the backticked mapping name from a census table row's first
// cell, tolerating the ` (abstract)` suffix the generator writes.
func rowName(cell string) (string, bool) {
	cell = strings.TrimSuffix(cell, " (abstract)")
	if !strings.HasPrefix(cell, "`") || !strings.HasSuffix(cell, "`") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(cell, "`"), "`")
	if name == "" || strings.ContainsRune(name, '`') {
		return "", false
	}
	return name, true
}

// tableNames returns every mapping name the document's census rows name,
// read from the first cell of each row inside the `rows` block, plus any
// first cell that fails to read as a name.
func tableNames(content string) (names []string, malformed []string, err error) {
	begin := blockMarker("rows", "begin")
	end := blockMarker("rows", "end")
	bi := strings.Index(content, begin)
	ei := strings.Index(content, end)
	if bi < 0 || ei < 0 || ei < bi {
		return nil, nil, fmt.Errorf("%s: missing %s / %s markers", censusDocPath, begin, end)
	}
	block := content[bi+len(begin) : ei]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitCells(line)
		if len(cells) != 7 || cells[0] == "OMG mapping" || separatorOnly(cells[0]) {
			continue
		}
		if name, ok := rowName(cells[0]); ok {
			names = append(names, name)
		} else {
			malformed = append(malformed, cells[0])
		}
	}
	return names, malformed, nil
}

// splitCells splits a table row on its unescaped pipes.
func splitCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cell.WriteRune('\\')
			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

func separatorOnly(cell string) bool {
	return strings.Trim(cell, ":- ") == ""
}

// checkDocument verifies the document against the baseline: every generated
// block equals what the generator writes, and the census table rows name
// exactly the baseline's mappings.
func checkDocument(content string, b *Baseline) error {
	var problems []string
	rewritten, err := rewriteBlocks(content, b)
	if err != nil {
		return err
	}
	if rewritten != content {
		problems = append(problems, "a generated block is stale; run `go run -C tools ./cmd/transformation-census`")
	}
	names, malformed, err := tableNames(content)
	if err != nil {
		return err
	}
	recorded := make(map[string]bool, len(b.Mappings))
	for _, m := range b.Mappings {
		recorded[m.Name] = true
	}
	seen := map[string]int{}
	for _, name := range names {
		seen[name]++
		if !recorded[name] {
			problems = append(problems, fmt.Sprintf("%s is in the census table but not in %s", name, baselinePath))
		}
	}
	for name, n := range seen {
		if n > 1 {
			problems = append(problems, fmt.Sprintf("%s has %d census rows", name, n))
		}
	}
	for _, m := range b.Mappings {
		if seen[m.Name] == 0 {
			problems = append(problems, fmt.Sprintf("%s is in %s but has no census row", m.Name, baselinePath))
		}
	}
	for _, cell := range malformed {
		problems = append(problems, fmt.Sprintf("the census row %q does not lead with a backticked mapping name", cell))
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s does not match %s:\n  %s", censusDocPath, baselinePath, strings.Join(problems, "\n  "))
}

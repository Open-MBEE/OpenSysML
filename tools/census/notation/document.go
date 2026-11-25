package notation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The census document carries generated blocks between census:begin/end
// markers: -measure writes them and -check refuses a hand-edited block.
var generatedBlocks = []string{"source", "summary", "clauses", "gaps", "packages", "exclusions"}

func blockMarker(name, which string) string {
	return "<!-- census:" + which + " " + name + " -->"
}

func renderBlock(name string, b *Baseline) (string, error) {
	switch name {
	case "source":
		return renderSource(b), nil
	case "summary":
		return renderSummary(b), nil
	case "clauses":
		return renderClauses(b), nil
	case "gaps":
		return renderGaps(b), nil
	case "packages":
		return renderPackages(b), nil
	case "exclusions":
		return renderExclusions(b), nil
	}
	return "", fmt.Errorf("unknown census block %q", name)
}

func renderSource(b *Baseline) string {
	s := b.Source
	return fmt.Sprintf("Measured over `%s`, rendered with `pdftotext -layout` from the SysML v2 specification PDF of pilot release `%s` (SysML-v2-Release commit `%s`), clause §%s.",
		s.Text, s.PilotTag, s.ReleaseCommit, s.Clause)
}

func renderSummary(b *Baseline) string {
	t := b.Totals
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d production statements (%d of them partial `=|` statements) name %d distinct productions: **%d drawn**, **%d approximate**, **%d undrawn**, %d not applicable.\n\n",
		t.Statements, t.PartialStatements, t.Productions, t.Drawn, t.Approximate, t.Undrawn, t.NotApplicable)
	sb.WriteString("| Category | Productions |\n| --- | --- |\n")
	for _, c := range Categories {
		fmt.Fprintf(&sb, "| %s | %d |\n", c, b.Categories[string(c)])
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderClauses(b *Baseline) string {
	var sb strings.Builder
	sb.WriteString("| Clause | Statements | Productions | Drawn | Approximate | Undrawn | Not applicable |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, c := range b.Clauses {
		fmt.Fprintf(&sb, "| §%s %s | %d | %d | %d | %d | %d | %d |\n", c.Number, escape(c.Title), c.Statements, c.Productions, c.Drawn, c.Approximate, c.Undrawn, c.NotApplicable)
	}
	t := b.Totals
	fmt.Fprintf(&sb, "| **Total** | %d | %d | %d | %d | %d | %d |", t.Statements, t.Productions, t.Drawn, t.Approximate, t.Undrawn, t.NotApplicable)
	return sb.String()
}

// renderGaps lists the undrawn and approximate rows under their clause.
func renderGaps(b *Baseline) string {
	var sb strings.Builder
	for _, c := range b.Clauses {
		var rows []Gap
		for _, g := range b.NonDrawn {
			if g.Clause == c.Number && g.Verdict != VerdictNotApplicable {
				rows = append(rows, g)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "#### §%s %s\n\n| Production | Category | Verdict | Package | Reason |\n| --- | --- | --- | --- | --- |\n", c.Number, escape(c.Title))
		for _, g := range rows {
			fmt.Fprintf(&sb, "| `%s`%s | %s | %s | %s | %s |\n", g.Name, partialMark(g.Partial), g.Category, g.Verdict, g.Package, escape(g.Reason))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func partialMark(partial bool) string {
	if partial {
		return " (partial)"
	}
	return ""
}

func renderPackages(b *Baseline) string {
	var sb strings.Builder
	sb.WriteString("| Package | Scope | Undrawn | Approximate | Productions |\n| --- | --- | --- | --- | --- |\n")
	for _, p := range packageTitles {
		var undrawn, approximate int
		var names []string
		for _, g := range b.NonDrawn {
			if g.Package != p.Letter {
				continue
			}
			names = append(names, "`"+g.Name+"`")
			if g.Verdict == VerdictUndrawn {
				undrawn++
			} else {
				approximate++
			}
		}
		fmt.Fprintf(&sb, "| %s | %s | %d | %d | %s |\n", p.Letter, p.Title, undrawn, approximate, strings.Join(names, ", "))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// renderExclusions lists the not-applicable rows grouped by their reason.
func renderExclusions(b *Baseline) string {
	var reasons []string
	byReason := map[string][]string{}
	for _, g := range b.NonDrawn {
		if g.Verdict != VerdictNotApplicable {
			continue
		}
		if _, ok := byReason[g.Reason]; !ok {
			reasons = append(reasons, g.Reason)
		}
		byReason[g.Reason] = append(byReason[g.Reason], fmt.Sprintf("`%s` (§%s)", g.Name, g.Clause))
	}
	var sb strings.Builder
	for _, reason := range reasons {
		fmt.Fprintf(&sb, "- **%s** (%d): %s\n", reason, len(byReason[reason]), strings.Join(byReason[reason], ", "))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func escape(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

func rewriteBlocks(content string, b *Baseline) (string, error) {
	for _, name := range generatedBlocks {
		rendered, err := renderBlock(name, b)
		if err != nil {
			return "", err
		}
		begin, end := blockMarker(name, "begin"), blockMarker(name, "end")
		bi, ei := strings.Index(content, begin), strings.Index(content, end)
		if bi < 0 || ei < 0 || ei < bi {
			return "", fmt.Errorf("%s: missing or misordered %s / %s markers", censusDocPath, begin, end)
		}
		if strings.Count(content, begin) != 1 || strings.Count(content, end) != 1 {
			return "", fmt.Errorf("%s: the census:%s block markers repeat", censusDocPath, name)
		}
		content = content[:bi+len(begin)] + "\n" + rendered + "\n" + content[ei:]
	}
	return content, nil
}

func readDocument(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(censusDocPath))) // #nosec G304 -- fixed repository path
	return string(content), err
}

func writeDocument(root string, b *Baseline) error {
	content, err := readDocument(root)
	if err != nil {
		return err
	}
	rewritten, err := rewriteBlocks(content, b)
	if err != nil {
		return err
	}
	if rewritten == content {
		return nil
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(censusDocPath)), []byte(rewritten), 0o644) // #nosec G306 -- a committed documentation file
}

func checkDocument(content string, b *Baseline) error {
	rewritten, err := rewriteBlocks(content, b)
	if err != nil {
		return err
	}
	if rewritten != content {
		return fmt.Errorf("%s: a generated block is stale; run `go run -C tools ./cmd/notation-census -measure`", censusDocPath)
	}
	return nil
}

// writeRows writes every row with its evidence to the build directory, the
// full table the committed baseline summarises.
func writeRows(root string, c *Census) error {
	var sb strings.Builder
	sb.WriteString("| Clause | Production | Statements | Category | Verdict | Implementation | Forms | Test | Reason |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range c.Rows {
		fmt.Fprintf(&sb, "| %s | `%s`%s | %d | %s | %s | %s | %s | %s | %s |\n", r.Clause, r.Name, partialMark(r.Partial), r.Statements,
			r.Category, r.Verdict, code(r.Implementation), strings.Join(r.Forms, ", "), code(r.Test), escape(r.Reason))
	}
	path := filepath.Join(root, filepath.FromSlash(rowsPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644) // #nosec G306 -- a build report
}

func code(s string) string {
	if s == "" {
		return ""
	}
	return "`" + s + "`"
}

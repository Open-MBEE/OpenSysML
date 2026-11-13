package hideo

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
)

// DocPath is the record whose per-matrix table is rendered from the baseline.
const DocPath = "docs/project/hideo-suite.md"

const (
	countsBegin = "<!-- hideo-suite:begin counts -->\n"
	countsEnd   = "<!-- hideo-suite:end counts -->\n"
)

// Matrix is the matrix a case belongs to: its id up to the first hyphen, as the suite files it.
func Matrix(id string) string {
	matrix, _, _ := strings.Cut(id, "-")
	return matrix
}

// tally counts one suite's outcomes over the cases of one matrix, or of all when matrix is "".
func (b *Baseline) tally(column int, matrix string) map[Outcome]int {
	counts := make(map[Outcome]int)
	for id, outcomes := range b.Cases {
		if matrix == "" || Matrix(id) == matrix {
			counts[outcomes[column]]++
		}
	}
	return counts
}

// figures renders a tally as "pass / fail / unsupported", naming any error or skip after them.
func figures(counts map[Outcome]int) string {
	s := fmt.Sprintf("%d / %d / %d", counts[Pass], counts[Fail], counts[Unsupported])
	for _, o := range []Outcome{Error, Skip} {
		if counts[o] > 0 {
			s += fmt.Sprintf(" (+%d %s)", counts[o], o)
		}
	}
	return s
}

// Table renders the baseline's figures per matrix and suite as a Markdown table.
func (b *Baseline) Table() string {
	matrices := make(map[string]int)
	for id := range b.Cases {
		matrices[Matrix(id)]++
	}
	names := make([]string, 0, len(matrices))
	for name := range matrices {
		names = append(names, name)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("| Matrix | Cases |")
	for _, suite := range Suites {
		out.WriteString(" `" + suite + "` |")
	}
	out.WriteString("\n|---|---:|" + strings.Repeat("---:|", len(Suites)) + "\n")
	row := func(label string, cases int, matrix string) {
		fmt.Fprintf(&out, "| %s | %d |", label, cases)
		for column := range Suites {
			out.WriteString(" " + figures(b.tally(column, matrix)) + " |")
		}
		out.WriteByte('\n')
	}
	for _, name := range names {
		row(name, matrices[name], name)
	}
	row("**All**", len(b.Cases), "")
	return out.String()
}

// Summary renders the baseline's totals, one suite per line.
func (b *Baseline) Summary() string {
	var out strings.Builder
	fmt.Fprintf(&out, "%d cases at %s (pass / fail / unsupported):\n", len(b.Cases), b.Pin.Commit)
	for column, suite := range Suites {
		fmt.Fprintf(&out, "  %-20s %s\n", suite, figures(b.tally(column, "")))
	}
	return out.String()
}

// spliceTable replaces the table between the record's markers with the baseline's.
func spliceTable(doc []byte, b *Baseline) ([]byte, error) {
	begin := bytes.Index(doc, []byte(countsBegin))
	end := bytes.Index(doc, []byte(countsEnd))
	if begin < 0 || end < begin {
		return nil, fmt.Errorf("%s has no %q … %q block", DocPath, strings.TrimSpace(countsBegin), strings.TrimSpace(countsEnd))
	}
	var out bytes.Buffer
	out.Write(doc[:begin+len(countsBegin)])
	out.WriteString(b.Table())
	out.Write(doc[end:])
	return out.Bytes(), nil
}

// WriteDocTable rewrites the per-matrix table of the record at path from the baseline.
func WriteDocTable(path string, b *Baseline) error {
	doc, err := os.ReadFile(path) // #nosec G304 -- callers name the record in this repository
	if err != nil {
		return err
	}
	fresh, err := spliceTable(doc, b)
	if err != nil {
		return err
	}
	return os.WriteFile(path, fresh, 0o600) // #nosec G703 -- same path as read above
}

// CheckDocTable fails unless the record at path states the baseline's figures.
func CheckDocTable(path string, b *Baseline) error {
	doc, err := os.ReadFile(path) // #nosec G304 -- callers name the record in this repository
	if err != nil {
		return err
	}
	fresh, err := spliceTable(doc, b)
	if err != nil {
		return err
	}
	if !bytes.Equal(doc, fresh) {
		return fmt.Errorf("the per-matrix table of %s is not the one %s gives; re-record with -update", DocPath, BaselinePath)
	}
	return nil
}

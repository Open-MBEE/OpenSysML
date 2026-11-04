// Package notation is the census of the SysML v2 graphical notation (§8.2.3):
// it extracts the clause's BNF productions from the rendered specification,
// adjudicates each against what the view renderers draw, and holds a committed
// baseline to the measurement.
package notation

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The clause the census measures, as the rendered specification heads it, and
// the heading that follows it; both are matched as whole lines.
const (
	clausePrefix   = "8.2.3."
	overviewTitle  = "8.2.3.1 Graphical Notation Overview"
	nextClauseHead = "8.3 Abstract Syntax"
	clauseCount    = 27
)

// A production statement is `name =` (complete) or `name =|` (partial), the
// name lowercase with internal hyphens, at the start of a line. pdftotext
// starts a page with a form feed, which may precede a statement or a heading.
var (
	statementLine = regexp.MustCompile(`^\f?([a-z][a-z0-9-]*) =(\|?)(?:\s|$)`)
	headingLine   = regexp.MustCompile(`^\f?8\.2\.3\.([0-9]+) ((?:[A-Za-z,-]+ )*Graphical Notation(?: Overview)?)$`)
)

// Statement is one production statement of the graphical BNF.
type Statement struct {
	Clause  string // the clause number, "8.2.3.4"
	Name    string
	Partial bool // `=|`: the production adds alternatives to one stated elsewhere
	Line    int  // 1-based line in the rendered text
}

// Production is one named production: every statement of its name, the clause
// of the first and whether any statement defines it completely.
type Production struct {
	Name       string   `json:"name"`
	Clause     string   `json:"clause"`
	Partial    bool     `json:"partial,omitempty"`
	Statements int      `json:"statements"`
	Clauses    []string `json:"clauses,omitempty"`
}

// Clause is one sub-clause of §8.2.3 with its title and statement count.
type Clause struct {
	Number     string `json:"number"`
	Title      string `json:"title"`
	Statements int    `json:"statements"`
}

// Grammar is the extracted graphical BNF: the clauses in order, the statements
// in text order and the productions by first appearance.
type Grammar struct {
	Clauses     []Clause
	Statements  []Statement
	Productions []Production
}

// Extract reads the §8.2.3 graphical BNF out of the rendered specification text.
// It fails when the clause's bounds or any of its 27 sub-clause headings are
// missing, when the overview states a production or a later clause none: a text
// from another pin or rendering must not pass as a changed denominator.
func Extract(text string) (*Grammar, error) {
	lines := strings.Split(text, "\n")
	start, end, err := clauseBounds(lines)
	if err != nil {
		return nil, err
	}
	g := &Grammar{}
	byName := map[string]int{}
	clause := 0
	for i := start; i < end; i++ {
		line := strings.TrimRight(lines[i], " \r")
		if m := headingLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n != len(g.Clauses)+1 {
				return nil, fmt.Errorf("line %d: clause %s%s follows %d sub-clauses; the headings must run 1..%d in order", i+1, clausePrefix, m[1], len(g.Clauses), clauseCount)
			}
			g.Clauses = append(g.Clauses, Clause{Number: clausePrefix + m[1], Title: m[2]})
			clause = len(g.Clauses) - 1
			continue
		}
		m := statementLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		st := Statement{Clause: g.Clauses[clause].Number, Name: m[1], Partial: m[2] == "|", Line: i + 1}
		g.Statements = append(g.Statements, st)
		g.Clauses[clause].Statements++
		if j, ok := byName[st.Name]; ok {
			p := &g.Productions[j]
			p.Statements++
			if !st.Partial {
				p.Partial = false
			}
			if p.Clauses[len(p.Clauses)-1] != st.Clause {
				p.Clauses = append(p.Clauses, st.Clause)
			}
			continue
		}
		byName[st.Name] = len(g.Productions)
		g.Productions = append(g.Productions, Production{Name: st.Name, Clause: st.Clause, Partial: st.Partial, Statements: 1, Clauses: []string{st.Clause}})
	}
	if len(g.Clauses) != clauseCount {
		return nil, fmt.Errorf("found %d sub-clause headings of §8.2.3, want %d", len(g.Clauses), clauseCount)
	}
	if g.Clauses[0].Statements != 0 {
		return nil, fmt.Errorf("%s states a production; the rendered text does not match the pinned specification", overviewTitle)
	}
	for _, c := range g.Clauses[1:] {
		if c.Statements == 0 {
			return nil, fmt.Errorf("%s %s states no production; the rendered text does not match the pinned specification", c.Number, c.Title)
		}
	}
	for i := range g.Productions {
		if len(g.Productions[i].Clauses) == 1 {
			g.Productions[i].Clauses = nil
		}
	}
	return g, nil
}

// clauseBounds finds the overview heading and the heading of the clause after
// §8.2.3, skipping the table of contents, whose lines carry page numbers.
func clauseBounds(lines []string) (start, end int, err error) {
	start = -1
	for i, line := range lines {
		line = strings.TrimRight(strings.TrimPrefix(line, "\f"), " \r")
		if start < 0 {
			if line == overviewTitle {
				start = i
			}
			continue
		}
		if line == nextClauseHead {
			return start, i, nil
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("the rendered text has no %q heading line", overviewTitle)
	}
	return 0, 0, fmt.Errorf("the rendered text has no %q heading after %q", nextClauseHead, overviewTitle)
}

// ProductionNames lists the production names in sorted order.
func (g *Grammar) ProductionNames() []string {
	names := make([]string, 0, len(g.Productions))
	for _, p := range g.Productions {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// Partials counts the partial `=|` statements.
func (g *Grammar) Partials() int {
	n := 0
	for _, st := range g.Statements {
		if st.Partial {
			n++
		}
	}
	return n
}

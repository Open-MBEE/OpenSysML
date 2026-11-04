package notation

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// The edge productions are adjudicated once, per form, in the notation table of
// view-rendering-forms.md; TestNotationTableMatchesTheWriters holds the writers
// to it. The census reads that table rather than restating it.
const notationTablePath = "docs/project/view-rendering-forms.md"

const notationTableHeading = "\n### Notation table\n"

var backticked = regexp.MustCompile("`([a-z][a-z0-9-]*)`")

// The table names two rows by their edge rather than the §8.2.3 production:
// `connection` is the connection line (connection-graphical) and `association`
// the connection definition's line; `specialization` is KerML's name beside
// subclassification and no §8.2.3 production.
var tableNames = map[string]string{"connection": "connection-graphical", "association": "connection-def-graphical", "specialization": ""}

// NotationTable is the edge adjudication read from view-rendering-forms.md:
// production name to the status cell and notes of its row.
type NotationTable struct {
	rows map[string]notationRow
}

type notationRow struct {
	status, notes string
}

// The statuses a row may state, as TestNotationTableMatchesTheWriters admits them.
var notationStatuses = map[string]bool{"conformant": true, "approximate": true, "differs from pilot": true}

// ReadNotationTable parses the notation table of view-rendering-forms.md.
func ReadNotationTable(root string) (*NotationTable, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(notationTablePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	return ParseNotationTable(string(content))
}

// ParseNotationTable reads the table rows that follow the "Notation table"
// heading: the first cell names the productions in backticks, the ninth cell
// is the status and the tenth the notes.
func ParseNotationTable(content string) (*NotationTable, error) {
	_, section, found := strings.Cut(content, notationTableHeading)
	if !found {
		return nil, fmt.Errorf("%s has no %q section", notationTablePath, strings.TrimSpace(notationTableHeading))
	}
	t := &NotationTable{rows: map[string]notationRow{}}
	inTable := false
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		cells := strings.Split(strings.Trim(strings.ReplaceAll(line, `\|`, "\x00"), "|"), "|")
		if len(cells) != 10 {
			return nil, fmt.Errorf("%s: notation table row %q has %d cells, want 10", notationTablePath, line, len(cells))
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(strings.ReplaceAll(cells[i], "\x00", "|"))
		}
		if cells[0] == "Production" || strings.Trim(cells[0], "-") == "" {
			inTable = true
			continue
		}
		inTable = true
		if !notationStatuses[cells[8]] {
			return nil, fmt.Errorf("%s: notation table row %q states status %q; want conformant, approximate or differs from pilot", notationTablePath, cells[0], cells[8])
		}
		for _, m := range backticked.FindAllStringSubmatch(cells[0], -1) {
			name := m[1]
			if production, ok := tableNames[name]; ok {
				name = production
			}
			if name != "" {
				t.rows[name] = notationRow{status: cells[8], notes: cells[9]}
			}
		}
	}
	if len(t.rows) == 0 {
		return nil, fmt.Errorf("%s: the notation table names no production", notationTablePath)
	}
	return t, nil
}

// adjudicate turns a table row into the census verdict: an approximate status
// is approximate with the row's notes as the reason (its same-page anchors
// retargeted to the rendering-forms page); conformant or spec-over-pilot is drawn.
func (t *NotationTable) adjudicate(name string) (Adjudication, bool) {
	if t == nil {
		return Adjudication{}, false
	}
	r, ok := t.rows[name]
	if !ok {
		return Adjudication{}, false
	}
	if r.status == "approximate" {
		notes := strings.ReplaceAll(r.notes, "](#", "]("+path.Base(notationTablePath)+"#")
		return approximate(CategoryEdge, "view-rendering-forms.md § Notation table: "+notes, tableEdge), true
	}
	return drawn(CategoryEdge, tableEdge), true
}

// Names lists the productions the table adjudicates, sorted.
func (t *NotationTable) Names() []string {
	names := make([]string, 0, len(t.rows))
	for name := range t.rows {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

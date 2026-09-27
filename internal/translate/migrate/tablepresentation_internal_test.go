package migrate

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A saved filter searches the projected columns its selection names; when the
// query reads none of them the filter is dropped rather than widened to every
// column, and when it reads some the others are noted.
func TestTextFilteredDropsAFilterOverUnreadColumns(t *testing.T) {
	m := &migration{}
	p := &projection{}
	p.property("name", "", 0)
	rows := qlit("rows")
	p.build(rows)

	unread := &lowered{}
	got := m.textFiltered(rows, &sysmlv1.RowFilter{Text: "Pump", Columns: []int{3}}, p, unread)
	if text := strings.Join(got.lines(""), "\n"); text != "rows" {
		t.Fatalf("filter over an unread column = %s, want the rows unfiltered", text)
	}
	wantNote := []string{`the saved row filter "Pump" names only the column 3, which the query does not read, and is dropped: the selection is not read as every column`}
	if !slices.Equal(unread.notes, wantNote) || len(unread.applied) != 0 {
		t.Fatalf("notes = %v, applied = %v", unread.notes, unread.applied)
	}

	partly := &lowered{}
	got = m.textFiltered(rows, &sysmlv1.RowFilter{Text: "Pump", Columns: []int{0, 3}}, p, partly)
	if text := strings.Join(got.lines(""), "\n"); !strings.Contains(text, `columns = ("name")`) {
		t.Fatalf("filter over a read and an unread column = %s, want the read column searched", text)
	}
	if !slices.Equal(partly.notes, []string{"the saved row filter also names the column 3, which the query does not read"}) ||
		!slices.Equal(partly.applied, []string{`the saved row filter "Pump" keeps the rows whose column name matches`}) {
		t.Fatalf("notes = %v, applied = %v", partly.notes, partly.applied)
	}
}

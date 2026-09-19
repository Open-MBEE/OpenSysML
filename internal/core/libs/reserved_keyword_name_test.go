package libs

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestStdlibReservedKeywordNames pins the places where the official OMG library
// spells a declaration name with a reserved keyword. SysML reserves keywords in
// name position, so these are reported — as warnings, not errors, precisely
// because the normative library relies on them and must keep parsing clean.
// The set is pinned so it cannot grow silently: a new entry means either an
// upstream library change or a parser bug that mistook a grammar keyword for a
// name (`binding [1] bind x = y` used to land here).
func TestStdlibReservedKeywordNames(t *testing.T) {
	// Empty: every former site named a declaration with a keyword of the *other*
	// language (`type`, `multiplicity` are KerML.xtext literals absent from
	// SysML.xtext; `frame`, `entry`, `exit`, `accept` the reverse), and a word a
	// grammar never spells is not reserved there.
	var want []string

	src := &embedSource{}
	seen := map[string]bool{}
	for _, path := range src.List() {
		data, err := src.Read(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		p := parser.New(source.New(path, data))
		p.ParseFile()
		for _, w := range p.Warnings {
			name := strings.SplitN(w.Message, " is a reserved keyword", 2)[0]
			seen[fmt.Sprintf("%s: %s", path, name)] = true
		}
	}

	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
		if !seen[w] {
			t.Errorf("no longer warned about, remove it from the pinned set: %s", w)
		}
	}
	var unexpected []string
	for s := range seen {
		if !wantSet[s] {
			unexpected = append(unexpected, s)
		}
	}
	sort.Strings(unexpected)
	for _, s := range unexpected {
		t.Errorf("new reserved-keyword name in the library (parser bug, or upstream change): %s", s)
	}
}

package suggest_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A table swept once answers what the per-name scans answer, so the cheaper
// path is not a different one.
func TestTableAgreesWithScan(t *testing.T) {
	idx := libraryIndex(t)
	table := suggest.NewTable(idx)
	names := suggest.SimpleNames(idx)

	for _, name := range []string{"Integer", "String", "Intger", "Whel", "Zzzznotatype", ""} {
		t.Run(name, func(t *testing.T) {
			if got, want := table.Qualified(name), suggest.Qualified(idx, name); !equal(got, want) {
				t.Errorf("Table.Qualified(%q) = %v, want %v", name, got, want)
			}
			if name == "" {
				return
			}
			if got, want := neighbourNames(table.Neighbours(name)), neighbourNames(suggest.Neighbours(name, names)); !equal(got, want) {
				t.Errorf("Table.Neighbours(%q) = %v, want %v", name, got, want)
			}
		})
	}
}

// A table over no index suggests nothing rather than panicking.
func TestTableWithoutIndex(t *testing.T) {
	table := suggest.NewTable(nil)
	if got := table.Qualified("Integer"); len(got) > 0 {
		t.Errorf("Table.Qualified over no index = %v, want none", got)
	}
	if got := table.Neighbours("Integer"); len(got) > 0 {
		t.Errorf("Table.Neighbours over no index = %v, want none", got)
	}
	if got := table.Unquoted("SA"); len(got) > 0 {
		t.Errorf("Table.Unquoted over no index = %v, want none", got)
	}
}

// A table answers, from what the index declares alone, the names a word is the
// unquoted start of and the paths declaring a name however it is spelled.
func TestTableUnquotedAndDeclared(t *testing.T) {
	p := parser.New(source.New("d.sysml", []byte(`package T {
		part def Rocket;
		part def 'SA-506' :> Rocket;
		part def 'SA 507';
		part def SAT;
		requirement def 'HLR-R001';
		part def 'left::right-X';
	}`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	table := suggest.NewTable(symbols.NewIndexFromDoc("d.sysml", root))

	if got, want := table.Unquoted("SA"), []string{"SA 507", "SA-506"}; !equal(got, want) {
		t.Errorf("Table.Unquoted(SA) = %v, want %v", got, want)
	}
	if got, want := table.Unquoted("HLR"), []string{"HLR-R001"}; !equal(got, want) {
		t.Errorf("Table.Unquoted(HLR) = %v, want %v", got, want)
	}
	for _, word := range []string{"SAT", "S", "Rocket", "SA-506", ""} {
		if got := table.Unquoted(word); len(got) > 0 {
			t.Errorf("Table.Unquoted(%q) = %v, want none", word, got)
		}
	}
	if got, want := table.Declared("SA-506"), []string{"T::SA-506"}; !equal(got, want) {
		t.Errorf("Table.Declared(SA-506) = %v, want %v", got, want)
	}
	// A declared name holding `::` is one name, not a path ending in `right-X`.
	if got, want := table.Unquoted("left"), []string{"left::right-X"}; !equal(got, want) {
		t.Errorf("Table.Unquoted(left) = %v, want %v", got, want)
	}
	if got := table.Unquoted("right"); len(got) > 0 {
		t.Errorf("Table.Unquoted(right) = %v, want none", got)
	}
	if got, want := table.Declared("left::right-X"), []string{"T::left::right-X"}; !equal(got, want) {
		t.Errorf("Table.Declared(left::right-X) = %v, want %v", got, want)
	}
	if got := table.Qualified("SA-506"); len(got) > 0 {
		t.Errorf("Table.Qualified(SA-506) = %v, want none: the path is not typable bare", got)
	}
}

// neighbourNames reads the names off neighbours, in the order they came.
func neighbourNames(hits []suggest.Neighbour) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Name)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

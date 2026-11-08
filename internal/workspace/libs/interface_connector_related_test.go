package libs

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A recorded connector answers relatedFeature, sourceFeature and targetFeature by end
// position: the chain Feature of an end with a symbol of its own, from its recorded
// reference subsetting, and for an end without one (`connect a.b to c`) the feature
// the record names — a plain name, or the feature a chain reaches, since the record
// carries no chain Feature for it.
func TestRecordedConnectorRelatedFeaturesKeepEveryEnd(t *testing.T) {
	const name = "connectors.sysml"
	const text = `package P {
		port def Port;
		part def B { port q : Port; }
		part def A { port p : Port; part inner : B; }
		part def Asm {
			part a : A;
			part b : B;
			part c : A;
			connection plain connect a to b;
			connection chained connect a.inner.q to b.q;
			connection mixed connect c.p to consumer ::> a.inner.q;
			connection named connect supplier ::> a.p to consumer ::> c.p;
			binding bound bind a.p = c.p;
		}
	}`
	idx := NewModelIndex()
	root := parser.New(source.New(name, []byte(text))).ParseFile()
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)
	rec, err := WriteInterface(name, source.KindSysML, idx, r, model, nil, nil)
	if err != nil {
		t.Fatalf("WriteInterface: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec.Scope, name)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	ridx := NewModelIndex()
	ridx.AddRecordedDocument(name, source.KindSysML, recorded, nil)
	rr := resolve.New(ridx)
	rmodel := semantics.NewModel(rr)
	rr.SetModel(rmodel)
	asm := lookupPath(t, ridx.DocumentRoot(name), "P", "Asm")

	// A chain Feature is rendered by the chain it denotes.
	render := func(sym *symbols.Symbol) string {
		if sym.Chain == nil {
			return sym.Name
		}
		links, _ := rmodel.ReflectiveElements(sym, "chainingFeature")
		var parts []string
		for _, link := range links {
			parts = append(parts, link.Name)
		}
		return "chain(" + strings.Join(parts, ".") + ")"
	}
	for _, tc := range []struct {
		connector string
		want      []string
	}{
		{"plain", []string{"a", "b"}},
		{"chained", []string{"q", "q"}},
		{"mixed", []string{"p", "chain(a.inner.q)"}},
		{"named", []string{"chain(a.p)", "chain(c.p)"}},
		{"bound", []string{"p", "p"}},
	} {
		connector, ok := asm.Scope.LookupLocal(tc.connector)
		if !ok {
			t.Fatalf("P::Asm::%s is not recorded", tc.connector)
		}
		for property, want := range map[string][]string{
			"relatedFeature": tc.want,
			"sourceFeature":  tc.want[:1],
			"targetFeature":  tc.want[1:],
		} {
			got, ok := rmodel.ReflectiveElements(connector, property)
			if !ok {
				t.Errorf("%s.%s is not derived from the record", tc.connector, property)
				continue
			}
			var names []string
			for _, sym := range got {
				names = append(names, render(sym))
			}
			if strings.Join(names, ",") != strings.Join(want, ",") {
				t.Errorf("recorded %s.%s = %v, want %v", tc.connector, property, names, want)
			}
		}
	}
}

func lookupPath(t *testing.T, scope *symbols.Scope, path ...string) *symbols.Symbol {
	t.Helper()
	var sym *symbols.Symbol
	for _, part := range path {
		s, ok := scope.LookupLocal(part)
		if !ok {
			t.Fatalf("%q not found along %v", part, path)
		}
		sym, scope = s, s.Scope
	}
	return sym
}

package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func buildScope(t *testing.T, src string) *Scope {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	return Build(root)
}

func TestBuildNewKindSymbols(t *testing.T) {
	cases := []struct {
		src  string
		name string
		want SymbolKind
	}{
		{"item def Widget;", "Widget", SymbolItemDef},
		{"port def P;", "P", SymbolPortDef},
		{"connection def C;", "C", SymbolConnectionDef},
		{"action def A;", "A", SymbolActionDef},
		{"use case def Login;", "Login", SymbolUseCaseDef},
		{"item w;", "w", SymbolItemUsage},
		{"port p;", "p", SymbolPortUsage},
		{"use case checkout;", "checkout", SymbolUseCaseUsage},
	}
	for _, c := range cases {
		scope := buildScope(t, c.src)
		syms := scope.LookupLocalAll(c.name)
		if len(syms) != 1 {
			t.Fatalf("%q: expected 1 symbol for %q, got %d", c.src, c.name, len(syms))
		}
		if syms[0].Kind != c.want {
			t.Fatalf("%q: kind = %v, want %v", c.src, syms[0].Kind, c.want)
		}
	}
}

// Every SysML usage declares a Feature, including a binding and a transition,
// whose symbol kind does not say so on its own; a definition or a package does not.
func TestSymbolIsFeature(t *testing.T) {
	cases := []struct {
		src  string
		path []string
		want bool
	}{
		{"part def D { attribute a; attribute b; binding bb bind a = b; }", []string{"D", "bb"}, true},
		{"state def S { state a; state b; transition t first a then b; }", []string{"S", "t"}, true},
		{"part def D { attribute a; }", []string{"D", "a"}, true},
		{"part def D { part p; }", []string{"D", "p"}, true},
		{"part def D;", []string{"D"}, false},
		{"package P;", []string{"P"}, false},
		{"datatype T;", []string{"T"}, false},
	}
	for _, c := range cases {
		scope := buildScope(t, c.src)
		var sym *Symbol
		for _, name := range c.path {
			syms := scope.LookupLocalAll(name)
			if len(syms) != 1 {
				t.Fatalf("%q: expected 1 symbol for %q, got %d", c.src, name, len(syms))
			}
			sym, scope = syms[0], syms[0].Scope
		}
		if got := sym.IsFeature(); got != c.want {
			t.Errorf("%q: IsFeature() = %v, want %v (kind %v)", c.src, got, c.want, sym.Kind)
		}
	}
}

func TestBuildConnectorEndsDefineNoSymbols(t *testing.T) {
	// Connector-end references must not register any symbols in the enclosing
	// scope beyond the connection usage itself.
	scope := buildScope(t, "part def Sys { part a; part b; connection c connect a to b; }")
	sys := scope.LookupLocalAll("Sys")[0]
	// a, b, c only (ends contribute nothing).
	for _, name := range []string{"a", "b", "c"} {
		if len(sys.Scope.LookupLocalAll(name)) != 1 {
			t.Fatalf("expected %q registered exactly once", name)
		}
	}
	if got := len(sys.Scope.Children()); got != 3 {
		t.Fatalf("expected 3 child scopes (a, b, c), got %d", got)
	}
}

// A KerML binary connector is named only ahead of `from` (KerML.xtext:836):
// `connector eng to t;` adds no second `eng`, and `a ::> eng` is the connector's member.
func TestBuildKerMLBinaryConnectorEndsStayOutOfTheFeaturingType(t *testing.T) {
	root := parser.New(source.New("t.kerml", []byte(`package P {
		class T {
			feature eng; feature t; feature u;
			connector eng to t;
			connector a ::> eng to t;
			connector [0..1] eng to [1..*] u;
			connector c from eng to t;
		}
	}`))).ParseFile()
	pkg := Build(root).LookupLocalAll("P")[0].Scope
	typ := pkg.LookupLocalAll("T")[0].Scope
	for _, name := range []string{"eng", "t", "u", "c"} {
		if got := len(typ.LookupLocalAll(name)); got != 1 {
			t.Errorf("T declares %d symbols named %q, want 1", got, name)
		}
	}
	if got := typ.LookupLocalAll("a"); len(got) != 0 {
		t.Errorf("the end name a leaked into T as %v", got)
	}
	var ends []*Symbol
	for _, child := range typ.Children() {
		ends = append(ends, child.LookupLocalAll("a")...)
	}
	if len(ends) != 1 || ends[0].Kind != SymbolConnectorEnd {
		t.Fatalf("connector scopes declare %v for the end a, want one connectorEnd", ends)
	}
}

// An entry/do/exit action a state declares by name is a feature of that state,
// as the standard library's StateAction relies on, so it must be a member of
// the state's scope and not disappear into the parser's entry/do/exit wrapper.
func TestBuildNamedEntryDoExitActionsAreStateMembers(t *testing.T) {
	scope := buildScope(t, `abstract state def StateAction {
		doc
		/* base */

		entry action entryAction :>> 'entry';
		do action doAction :>> 'do';
		exit action exitAction :>> 'exit';
	}`)

	syms := scope.LookupLocalAll("StateAction")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for StateAction, got %d", len(syms))
	}
	state := syms[0].Scope
	if state == nil {
		t.Fatal("StateAction has no scope")
	}
	for _, name := range []string{"entryAction", "doAction", "exitAction"} {
		if got := len(state.LookupLocalAll(name)); got != 1 {
			t.Errorf("StateAction declares %d symbols named %q, want 1", got, name)
		}
	}
}

// A binding's named ends (`bind e1 ::> a = e2 references b;`) are end features
// of the binding, so they are its members and not the featuring type's; the
// bare ends and the referenced features declare nothing new (SysML.xtext:1000).
func TestBuildBindingConnectorEndsAreTheBindingsMembers(t *testing.T) {
	root := parser.New(source.New("t.sysml", []byte(`package P {
		part def T {
			attribute a; attribute b;
			bind a = b;
			bind e1 ::> a = e2 references b;
			binding bb bind [1] e3 ::> a = b;
		}
	}`))).ParseFile()
	pkg := Build(root).LookupLocalAll("P")[0].Scope
	typ := pkg.LookupLocalAll("T")[0].Scope
	for _, name := range []string{"a", "b", "bb"} {
		if got := len(typ.LookupLocalAll(name)); got != 1 {
			t.Errorf("T declares %d symbols named %q, want 1", got, name)
		}
	}
	for _, name := range []string{"e1", "e2", "e3"} {
		if got := typ.LookupLocalAll(name); len(got) != 0 {
			t.Errorf("the end name %s leaked into T as %v", name, got)
		}
	}
	ends := map[string]int{}
	for _, child := range typ.Children() {
		for _, name := range []string{"e1", "e2", "e3"} {
			for _, sym := range child.LookupLocalAll(name) {
				if sym.Kind != SymbolConnectorEnd {
					t.Errorf("the end %s is a %v, want a connectorEnd", name, sym.Kind)
				}
				ends[name]++
			}
		}
	}
	for _, name := range []string{"e1", "e2", "e3"} {
		if ends[name] != 1 {
			t.Errorf("binding scopes declare %d ends named %s, want 1", ends[name], name)
		}
	}
	bb := typ.LookupLocalAll("bb")[0]
	if got := bb.Scope.LookupLocalAll("e3"); len(got) != 1 || got[0].Kind != SymbolConnectorEnd {
		t.Errorf("bb declares %v for its end e3, want one connectorEnd", got)
	}
}

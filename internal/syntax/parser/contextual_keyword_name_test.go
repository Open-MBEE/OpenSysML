package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseClean parses src and fails when anything is reported: these shapes are
// well-formed, so neither an error nor a reserved-keyword warning belongs here.
func parseClean(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("%s\nerrors = %v, want none", src, p.Diagnostics)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("%s\nwarnings = %v, want none", src, p.Warnings)
	}
	return root
}

// `on` is a name in every position: the word appears in none of the pilot
// implementation's grammars, while the OMG training corpus declares a state
// named `on` and names it as a transition target.
func TestParseOnIsAName(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { state on; accept Sig then on; } }",
		"package P { state def S { state on { entry action light; } accept Sig then on; } }",
		// A source named `on` and the trigger after it.
		"package P { state def S { state on; state off; transition first on accept Sig then off; } }",
		"package P { part def D { attribute on : Boolean; attribute lit = on and not on; } }",
		"package P { part def D { port on : PortDef; } port def PortDef; }",
		"package P { action a { in on : Integer; } }",
		"package P { part def D { part on : On; } part def On; }",
	} {
		parseClean(t, src)
	}
}

// `var` marks a variable feature (KerML.xtext BasicFeaturePrefix,
// `isVariable ?= 'var'`) and is a name everywhere else, as SysML's own grammar
// — which has no `var` at all — implies.
func TestParseVarIsANameOutsideThePrefix(t *testing.T) {
	for _, src := range []string{
		"package P { attribute var : Integer; }",
		"package P { part def D { attribute var : Integer; attribute next = var + 1; } }",
		"package P { action a { attribute var : Integer; assign var := 1; } }",
		"package P { calc def C { in var : Integer; return x : Integer; } }",
		"package P { part def D { part var : Var; } part def Var; }",
	} {
		parseClean(t, src)
	}
}

// The prefix keeps its meaning: `var` before a kind keyword qualifies the
// declaration that follows rather than naming it.
func TestParseVarPrefixQualifiesTheKind(t *testing.T) {
	root := parseClean(t, "package P { part def D { var attribute total : Integer; } }")
	pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	u, ok := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("member parsed to %T, want a usage", def.Members[0].(*ast.Membership).Member)
	}
	if u.Ident.Name != "total" {
		t.Errorf("declared %q, want the name after the prefix", u.Ident.Name)
	}
	if u.PrefixKeyword != "var" {
		t.Errorf("prefix = %q, want \"var\"", u.PrefixKeyword)
	}

	// `derived var feature` — the spelling the Kernel Semantic Library uses —
	// reads the same way: the modifier is kept and the prefix is still recorded,
	// so both spellings describe the feature as variable.
	root = parseClean(t, "package P { datatype D { derived var feature a : D[1]; } }")
	pkg = root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	dt := pkg.Members[0].(*ast.Membership).Member.(*ast.Usage)
	u = dt.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !u.IsDerived || u.PrefixKeyword != "var" || u.Ident.Name != "a" {
		t.Errorf("derived = %v, prefix = %q, name = %q; want true, \"var\", \"a\"", u.IsDerived, u.PrefixKeyword, u.Ident.Name)
	}
}

// A namespace member takes the prefix as a body member does: KerML.xtext's
// FeatureMember appears in both positions.
func TestParseVarPrefixStartsANamespaceMember(t *testing.T) {
	for _, src := range []string{
		"package P { class C; var feature v : C; }",
		"class C; var feature v : C;",
		"package P { class C; derived var feature v : C; }",
		"package P { class C; var #M feature v : C; metaclass M; }",
	} {
		p := New(source.New("test.kerml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Errorf("%s\nerrors = %v, want none", src, p.Diagnostics)
			continue
		}
		members := root.Members
		if pkg, ok := members[0].(*ast.Membership).Member.(*ast.Package); ok {
			members = pkg.Members
		}
		u, ok := members[1].(*ast.Membership).Member.(*ast.Usage)
		if !ok || u.Ident.Name != "v" || !u.IsVariable || u.PrefixKeyword != "var" {
			t.Errorf("%s\nmember = %+v, want a variable feature v", src, members[1])
		}
	}
}

// `chain` is the feature chain modifier only when a name follows it; before
// `=`, `:`, `;`, `[` or a word the grammar reserves — one that continues the
// declaration (`default`, `ordered`, `subsets`, `specializes`) or ends it
// (`about`) — it names the feature, on either side of the kind keyword. In
// `metadata chain about x;` the name is the usage's type (MetadataUsageDeclaration).
func TestParseChainIsANameBeforeAnythingButAName(t *testing.T) {
	for _, src := range []string{
		"package P { attribute chain = 1; attribute pt = chain + 1; }",
		"package P { attribute chain : Integer; }",
		"package P { part def D { attribute chain; attribute chain[*]; } }",
		"package P { part def D { ref chain :> other; attribute other; } }",
		"package P { action a { in chain : Integer; assign chain := 1; } }",
		"package P { attribute chain default 1; }",
		"package P { attribute chain defined by Integer; }",
		"package P { attribute chain subsets other; attribute other; }",
		"package P { attribute chain redefines other; attribute other; }",
		"package P { attribute chain ordered :> other; attribute other : Integer[*]; }",
		"package P { attribute chain nonunique :> other; attribute other : Integer[*]; }",
		"package P { attribute chain ordered nonunique = other; attribute other : Integer[*]; }",
	} {
		root := parseClean(t, src)
		pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
		if u.Ident.Name != "chain" || u.IsChain {
			t.Errorf("%s\ndeclared %q chain=%t, want the feature named chain", src, u.Ident.Name, u.IsChain)
		}
	}

	src := "package P { metadata chain about other; attribute other; }"
	root := parseClean(t, src)
	pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
	if u.Ident.Name != "" || u.IsChain || len(u.Relationships) == 0 || ast.SimpleName(u.Relationships[0].Target) != "chain" {
		t.Errorf("%s\ndeclared %q chain=%t, want an unnamed metadata usage typed by chain", src, u.Ident.Name, u.IsChain)
	}

	for _, src := range []string{
		"package P { attribute chain x : Integer; }",
		"package P { ref chain x : Integer; }",
		"package P { ref chain 'x y' : Integer; }",
		"package P { ref chain <s> x : Integer; }",
		// A keyword of the other language is a name in this one.
		"package P { attribute chain chains : Integer; }",
	} {
		root := parseClean(t, src)
		pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
		if u.Ident.Name == "chain" || !u.IsChain {
			t.Errorf("%s\ndeclared %q chain=%t, want the modifier and the name after it", src, u.Ident.Name, u.IsChain)
		}
	}
}

// The ordering modifiers keep their meaning when they follow the feature named
// chain: `attribute chain ordered` is the ordered feature `chain`.
func TestParseChainKeepsTheOrderingModifiersAfterIt(t *testing.T) {
	root := parseClean(t, "package P { attribute chain ordered nonunique :> other; attribute other : Integer[*]; }")
	pkg := root.(*ast.RootNamespace).Members[0].(*ast.Membership).Member.(*ast.Package)
	u := firstUsage(t, pkg.Members[0].(*ast.Membership).Member)
	if u.Ident.Name != "chain" || u.IsChain || !u.IsOrdered || !u.IsNonunique {
		t.Errorf("declared %q chain=%t ordered=%t nonunique=%t; want chain, false, true, true", u.Ident.Name, u.IsChain, u.IsOrdered, u.IsNonunique)
	}
}

// A KerML type named chain keeps its name before every type relationship the
// grammar spells with a keyword, since none of those words can name a type.
func TestParseKerMLTypeNamedChainKeepsItsName(t *testing.T) {
	for _, src := range []string{
		"package P { class Base; class chain specializes Base; }",
		"package P { class Base; class chain :> Base; }",
		"package P { struct Base; struct chain specializes Base; }",
		"package P { datatype Base; datatype chain specializes Base; }",
		"package P { class Base; class chain conjugates Base; }",
		"package P { class Base; class chain disjoint from Base; }",
		"package P { class Base; class chain unions Base; }",
		"package P { class Base; class chain intersects Base; }",
		"package P { class Base; class chain differences Base; }",
		"package P { feature other; feature chain chains other; }",
		"package P { feature other; feature chain redefines other; }",
		"package P { feature chain ordered nonunique; }",
	} {
		p := New(source.New("test.kerml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
			t.Errorf("%s\nerrors = %v, warnings = %v, want none", src, p.Diagnostics, p.Warnings)
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member)
		if u.Ident.Name != "chain" || u.IsChain {
			t.Errorf("%s\ndeclared %q chain=%t, want the type named chain", src, u.Ident.Name, u.IsChain)
		}
	}
}

// firstUsage returns n when it is a usage, else the first usage n declares.
func firstUsage(t *testing.T, n ast.Node) *ast.Usage {
	t.Helper()
	switch d := n.(type) {
	case *ast.Usage:
		if len(d.Members) > 0 {
			if u, ok := d.Members[0].(*ast.Membership).Member.(*ast.Usage); ok {
				return u
			}
		}
		return d
	case *ast.Definition:
		if u, ok := d.Members[0].(*ast.Membership).Member.(*ast.Usage); ok {
			return u
		}
	}
	t.Fatalf("member parsed to %T, want a usage", n)
	return nil
}

// A KerML connector's ends may be named by words only SysML reserves, so
// `connector frame to state` states two ends and names no connector.
func TestParseKerMLConnectorEndsNamedByOtherLanguageKeywords(t *testing.T) {
	for _, src := range []string{
		"package P { feature frame; feature state; connector frame to state; }",
		"package P { feature frame; feature state; connector [0..1] frame to [1] state; }",
		"package P { feature frame; feature state; connector e references frame to state; }",
		"package P { feature frame; feature state; connector e ::> frame to state; }",
		"package P { feature part; feature state; connector part.frame to state.action; }",
	} {
		p := New(source.New("test.kerml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
			t.Errorf("%s\nerrors = %v, warnings = %v, want none", src, p.Diagnostics, p.Warnings)
			continue
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member)
		if u.Ident.Name == "frame" || u.Ident.Name == "part" || len(u.ConnectorEnds) != 2 {
			t.Errorf("%s\ndeclared %q with %d ends, want two ends and no end taken as the name", src, u.Ident.Name, len(u.ConnectorEnds))
		}
	}
}

// A KerML connector's first end may be a global `$::`-qualified name, which
// cannot be the connector's name either.
func TestParseKerMLConnectorEndsGloballyQualified(t *testing.T) {
	for _, src := range []string{
		"package P { feature a; feature b; connector $::P::a to b; }",
		"package P { feature a; feature b; connector [0..1] $::P::a to [1] b; }",
		"package P { feature a; feature b; connector e references $::P::a to $::P::b; }",
		"package P { feature a; feature b; connector e ::> $::P::a to b; }",
		"package P { feature a; feature b; connector $::P::a.x to b.y; }",
	} {
		p := New(source.New("test.kerml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
			t.Errorf("%s\nerrors = %v, warnings = %v, want none", src, p.Diagnostics, p.Warnings)
			continue
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		u := firstUsage(t, pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member)
		if u.Ident.Name == "a" || u.Ident.Name == "P" || len(u.ConnectorEnds) != 2 {
			t.Errorf("%s\ndeclared %q with %d ends, want two ends and no end taken as the name", src, u.Ident.Name, len(u.ConnectorEnds))
		}
	}
}

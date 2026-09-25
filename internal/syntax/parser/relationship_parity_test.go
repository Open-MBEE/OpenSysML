package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseMember parses one body member in isolation and returns its AST dump.
func parseMember(t *testing.T, member string) (string, []string) {
	t.Helper()
	src := "part def B { attribute x : Real; attribute y : Real; }\npart def A :> B { " + member + " }\n"
	p := New(source.New("parity.sysml", []byte(src)))
	root := p.ParseFile()
	msgs := make([]string, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	return ast.Dump(root), msgs
}

// TestRelationshipKeywordAndSymbolAgree checks that a member written with a
// relationship keyword parses to the same tree as the same member written with
// the keyword's symbol. Both spellings of a feature specialization are one
// notation (SysML.xtext SubsetsKeyword, RedefinesKeyword, ReferencesKeyword,
// CrossesKeyword, DefinedByKeyword), so their trees must be identical.
func TestRelationshipKeywordAndSymbolAgree(t *testing.T) {
	pairs := []struct{ keyword, symbol string }{
		{"redefines x;", ":>> x;"},
		{"redefines x : Real;", ":>> x : Real;"},
		{"redefines x[1];", ":>> x[1];"},
		{"redefines x = 5;", ":>> x = 5;"},
		{"redefines x, y;", ":>> x, y;"},
		{"redefines x.y;", ":>> x.y;"},
		{"redefines x.y = 100;", ":>> x.y = 100;"},
		{"redefines B::x = 5;", ":>> B::x = 5;"},
		{"redefines B::x;", ":>> B::x;"},
		{"redefines x : Real = 5;", ":>> x : Real = 5;"},
		{"redefines x { doc /* d */ }", ":>> x { doc /* d */ }"},
		{"subsets y;", ":> y;"},
		{"subsets B::y;", ":> B::y;"},
		{"subsets y : Real;", ":> y : Real;"},
		{"subsets y[1];", ":> y[1];"},
		{"subsets y = 5;", ":> y = 5;"},
		{"subsets y[0..*] = 5;", ":> y[0..*] = 5;"},
		{"references y;", "::> y;"},
		{"references y : Real;", "::> y : Real;"},
		{"crosses y;", "=> y;"},
		{"defined by Real;", ": Real;"},
		{"attribute redefines x;", "attribute :>> x;"},
		{"attribute subsets y;", "attribute :> y;"},
		{"attribute redefines x = 5;", "attribute :>> x = 5;"},
		{"private redefines x;", "private :>> x;"},
		{"ref redefines x;", "ref :>> x;"},
		{"part redefines x[4];", "part :>> x[4];"},
		{"derived redefines x = 5;", "derived :>> x = 5;"},
		{"attribute <sn> redefines x;", "attribute <sn> :>> x;"},
		{"attribute <sn> subsets y = 5;", "attribute <sn> :> y = 5;"},
		{"attribute <sn> references y;", "attribute <sn> ::> y;"},
		{"attribute <sn> defined by Real;", "attribute <sn> : Real;"},
	}
	for _, pair := range pairs {
		t.Run(pair.keyword, func(t *testing.T) {
			kwDump, kwDiags := parseMember(t, pair.keyword)
			symDump, symDiags := parseMember(t, pair.symbol)
			if len(kwDiags) > 0 {
				t.Fatalf("%q: unexpected diagnostics %v", pair.keyword, kwDiags)
			}
			if len(symDiags) > 0 {
				t.Fatalf("%q: unexpected diagnostics %v", pair.symbol, symDiags)
			}
			if kwDump != symDump {
				t.Errorf("%q and %q parse differently\nkeyword:\n%s\nsymbol:\n%s",
					pair.keyword, pair.symbol, kwDump, symDump)
			}
		})
	}
}

// TestDegenerateRelationshipKeywordDiagnosed checks that a specialization
// written with its keyword and missing its target is reported exactly as the
// symbol spelling is: `redefines;` is no more a feature named `redefines` than
// `:>>;` is, since a reserved word names nothing unquoted.
func TestDegenerateRelationshipKeywordDiagnosed(t *testing.T) {
	pairs := []struct{ keyword, symbol string }{
		{"redefines;", ":>>;"},
		{"redefines = 5;", ":>> = 5;"},
		{"subsets ;", ":> ;"},
		{"references ;", "::> ;"},
		{"crosses ;", "=> ;"},
		{"defined by ;", ": ;"},
	}
	for _, pair := range pairs {
		t.Run(pair.keyword, func(t *testing.T) {
			_, kwDiags := parseMember(t, pair.keyword)
			_, symDiags := parseMember(t, pair.symbol)
			if len(kwDiags) == 0 {
				t.Fatalf("%q parsed without diagnostics, while %q reports %v", pair.keyword, pair.symbol, symDiags)
			}
			if kwDiags[0] != symDiags[0] {
				t.Errorf("%q reports %q, %q reports %q", pair.keyword, kwDiags[0], pair.symbol, symDiags[0])
			}
		})
	}
}

// TestNonMemberRelationshipsRejected checks the relationship clauses that are
// not feature specializations, so begin no body member: `specializes` relates
// two types (SysML.xtext SubclassificationPart) and the type-relationship
// clauses qualify the declaration they follow (TypeRelationshipPart).
func TestNonMemberRelationshipsRejected(t *testing.T) {
	cases := []struct{ member, want string }{
		{"specializes B;", "'specializes' relates two types"},
		{"unions x, y;", "'unions' relates the declaration written before it"},
		{"intersects x, y;", "'intersects' relates the declaration written before it"},
		{"chains x.y;", "'chains' relates the declaration written before it"},
		{"inverse of y;", "'inverse' relates the declaration written before it"},
		{"disjoint from y;", "expected the disjoined type before 'from'"},
	}
	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			dump, diags := parseMember(t, tc.member)
			if len(diags) == 0 {
				t.Fatalf("%q parsed without diagnostics:\n%s", tc.member, dump)
			}
			found := false
			for _, d := range diags {
				if strings.Contains(d, tc.want) {
					found = true
				}
			}
			if !found {
				t.Errorf("%q diagnostics %v do not mention %q", tc.member, diags, tc.want)
			}
			if !strings.Contains(dump, "ErrorNode") {
				t.Errorf("%q should leave an ErrorNode:\n%s", tc.member, dump)
			}
		})
	}
}

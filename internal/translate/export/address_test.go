package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A positional segment is `@` plus a canonical integer; an at-sign name is a name.
func TestAddressTermTellsAPositionFromAnAtSignName(t *testing.T) {
	cases := map[string]string{
		"P::@2":       rdf.OpenSysML + xPositionalName,
		"P::@2::x":    rdf.OpenSysML + xPositionalName,
		"P::@trigger": rdf.OpenSysML + xPositionalName,
		"P::'@abc'":   rdf.SysML + "qualifiedName",
		"P::'@2'":     rdf.SysML + "qualifiedName",
		"P::Named":    rdf.SysML + "qualifiedName",
		"P::@0::'@1'": rdf.OpenSysML + xPositionalName,
	}
	// A declared name beginning with `@` is quoted in an address, so it never
	// reads as a position.
	for _, name := range []string{"@abc", "@2", "@trigger"} {
		if got := identitySegment(name); got != "'"+name+"'" {
			t.Errorf("identitySegment(%q) = %s, want it quoted", name, got)
		}
	}
	if got := identitySegment("Named"); got != "Named" {
		t.Errorf("identitySegment(Named) = %s", got)
	}
	for name, want := range cases {
		if got := addressTerm(name).Value; got != want {
			t.Errorf("addressTerm(%q) = %s, want %s", name, got, want)
		}
	}
}

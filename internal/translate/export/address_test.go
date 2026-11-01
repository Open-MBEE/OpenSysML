package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Unquoted @ segments are positional; quoted @ names stay declared names.
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

func TestPositionalNameScanDoesNotAllocate(t *testing.T) {
	for name, want := range map[string]bool{
		"":                          false,
		"P":                         false,
		"::":                        false,
		"P::":                       false,
		"P::@2":                     true,
		"@trigger":                  true,
		"P::'@2'":                   false,
		"P::'name::@2'":             false,
		`P::'name\'::@2'`:           false,
		`P::'name\\'::@2`:           true,
		"P::'unfinished::@2":        true,
		"P::'closed'next::@0":       true,
		"P::@0::'name::@1'::member": true,
	} {
		t.Run(name, func(t *testing.T) {
			var got bool
			allocs := testing.AllocsPerRun(100, func() { got = isPositionalName(name) })
			if got != want {
				t.Errorf("isPositionalName(%q) = %v, want %v", name, got, want)
			}
			if allocs != 0 {
				t.Errorf("isPositionalName(%q) allocated %g times, want none", name, allocs)
			}
		})
	}
}

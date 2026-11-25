package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Unquoted @ segments are positional; quoted @ names stay declared names, whose
// qualifiedName is spelled as escapedName with the address beside it where they differ.
func TestStateAddressTellsAPositionFromAName(t *testing.T) {
	type stated struct{ positional, qualified, mapping string }
	cases := map[string]stated{
		"P::@2":           {positional: "P::@2"},
		"P::@2::x":        {positional: "P::@2::x"},
		"P::@trigger":     {positional: "P::@trigger"},
		"P::@0::'@1'":     {positional: "P::@0::'@1'"},
		"P::Named":        {qualified: "P::Named"},
		"P::'@abc'":       {qualified: "P::'@abc'"},
		"P::'@2'":         {qualified: "P::'@2'"},
		"P::'x::y'":       {qualified: "P::'x::y'"},
		"P::two words":    {qualified: "P::'two words'", mapping: "P::two words"},
		"P::it's":         {qualified: `P::'it\'s'`, mapping: "P::it's"},
		`P::'\'lead'`:     {qualified: `P::'\'lead'`},
		"P::tab\there::x": {qualified: `P::'tab\there'::x`, mapping: "P::tab\there::x"},
		`P::back\slash`:   {qualified: `P::'back\\slash'`, mapping: `P::back\slash`},
		"P::'@1'::é":      {qualified: "P::'@1'::'é'", mapping: "P::'@1'::é"},
		"P::'a::b'::c d":  {qualified: "P::'a::b'::'c d'", mapping: "P::'a::b'::c d"},
		// An address quoted where Segment would not quote it is stated beside its name.
		"P::'~P'": {qualified: "P::'~P'", mapping: "P::'~P'"},
	}
	// A declared name beginning with `@` is quoted in an address, so it never
	// reads as a position.
	for _, name := range []string{"@abc", "@2", "@trigger"} {
		if got := naming.Segment(name); got != "'"+name+"'" {
			t.Errorf("naming.Segment(%q) = %s, want it quoted", name, got)
		}
	}
	if got := naming.Segment("Named"); got != "Named" {
		t.Errorf("naming.Segment(Named) = %s", got)
	}
	subject := rdf.IRI(rdf.Element + "x")
	for address, want := range cases {
		graph := rdf.NewGraph()
		stateAddress(graph.Add, subject, address)
		var got stated
		got.positional, _ = graph.Lexical(subject, rdf.OpenSysML+xPositionalName)
		got.qualified, _ = graph.Lexical(subject, rdf.SysML+pQualifiedName)
		got.mapping, _ = graph.Lexical(subject, rdf.OpenSysML+xMappingQualifiedName)
		if got != want {
			t.Errorf("stateAddress(%q) states %+v, want %+v", address, got, want)
		}
		if read, ok := addressOf(graph, subject); !ok || read != address {
			t.Errorf("addressOf reads %q back from stateAddress(%q)", read, address)
		}
		// A graph stripped of the sysx: vocabulary keeps the address in its qualifiedName.
		if want.qualified != "" && address != "P::'~P'" {
			bare := rdf.NewGraph()
			bare.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(want.qualified))
			if read, _ := addressOf(bare, subject); read != address {
				t.Errorf("addressOf reads %q from the qualifiedName %q alone, want %q", read, want.qualified, address)
			}
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

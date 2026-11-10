package queryexec

import (
	"errors"
	"strings"
	"testing"
)

func TestResultViewWritesTheRunAsAQueryResultView(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Subsystem;
part telescope : Subsystem { part mirror; part 'mount control'; }
package Duplicates { part a; part a; }
calc def Parts :> Query { Descendants(Named(qualifiedName = "Observatory::telescope")) }
calc def Nothing :> Query { WhereType(Descendants(Named(qualifiedName = "Observatory::telescope")), "StateUsage") }
calc def Misspelled :> Query { Named(qualifiedName = "Observatory::telescop") }
calc def Ambiguous :> Query { Named(qualifiedName = "Observatory::Duplicates::a") }
calc def RootMiss :> Query { Named(qualifiedName = "Observatry") }
`)
	spec := func(name string) ResultView {
		return ResultView{Name: name, Query: "Observatory::" + name, Source: `a "quoted" path.sysml`, Provenance: "sha256:abc"}
	}
	write := func(name string) string {
		t.Helper()
		result, err := fixture.execute(t, name, nil, Options{})
		text, ok := spec(name).Write(fixture.context(), result, err)
		if !ok {
			t.Fatalf("%s: no view written for result %v, error %v", name, result, err)
		}
		again, _ := spec(name).Write(fixture.context(), result, err)
		if again != text {
			t.Errorf("%s: two writes of one run differ", name)
		}
		return text
	}
	header := func(name, view string) string {
		return "package " + name + "Result {\n\tprivate import QVTS::*;\n\n\tview " + view + " : QueryResult {\n"
	}
	attributes := func(name, outcome, provenance string) string {
		return "\t\tattribute :>> query = \"Observatory::" + name + "\";\n" +
			"\t\tattribute :>> source = \"a \\\"quoted\\\" path.sysml\";\n" +
			"\t\tattribute :>> outcome = Outcome::" + outcome + ";\n" +
			"\t\tattribute :>> provenance = \"" + provenance + "\";\n\t}\n}\n"
	}
	selection := "\t\texpose Observatory::telescope::mirror;\n\t\texpose Observatory::telescope::'mount control';\n"
	cases := []struct{ name, view, want string }{
		{"Parts", "parts", selection + attributes("Parts", "established", "sha256:abc")},
		{"Nothing", "nothing", attributes("Nothing", "established", "sha256:abc")},
		{"Misspelled", "misspelled", "\t\texpose Observatory;\n" +
			attributes("Misspelled", "pathError", "sha256:abc; step Named(Observatory::telescop)")},
		{"Ambiguous", "ambiguous", "\t\texpose Observatory::Duplicates;\n" +
			attributes("Ambiguous", "modelError", "sha256:abc; step Named(Observatory::Duplicates::a)")},
		{"RootMiss", "rootMiss", attributes("RootMiss", "pathError", "sha256:abc; step Named(Observatry)")},
	}
	for _, c := range cases {
		if got, want := write(c.name), header(c.name, c.view)+c.want; got != want {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", c.name, got, want)
		}
	}
	if text, ok := (ResultView{}).Write(fixture.context(), nil, errors.New("no record")); ok {
		t.Errorf("an error carrying no QVTS record wrote a view:\n%s", text)
	}
	if got, want := spec("Parts").ResultViewQualifiedName(), "PartsResult::parts"; got != want {
		t.Errorf("qualified name = %q, want %q", got, want)
	}
	for query, want := range map[string]string{"A::B::CabinMass": "CabinMass", "": DefaultResultViewName, "A::'two words'": DefaultResultViewName} {
		if got := ResultViewName(query); got != want {
			t.Errorf("ResultViewName(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestResultViewExposesFromTheRootWhatItsScopeWouldCapture(t *testing.T) {
	fixture := loadExecutionSource(t, "package Observatory {"+executionImports+`
calc def Mass :> Query { Named(qualifiedName = "mass::m") }
calc def Inherited :> Query { Named(qualifiedName = "source::cabin") }
calc def Imported :> Query { Named(qualifiedName = "Outcome::x") }
calc def Plain :> Query { Named(qualifiedName = "Observatory::Mass") }
}
package mass { part m; }
package source { part cabin; }
package Outcome { part x; }
`)
	for name, want := range map[string]string{
		"Mass":      "expose $::mass::m;",
		"Inherited": "expose $::source::cabin;",
		"Imported":  "expose $::Outcome::x;",
		"Plain":     "expose Observatory::Mass;",
	} {
		result, err := fixture.execute(t, name, nil, Options{})
		text, _ := ResultView{Name: name}.Write(fixture.context(), result, err)
		if !strings.Contains(text, "\t\t"+want+"\n") {
			t.Errorf("%s: want %q in:\n%s", name, want, text)
		}
	}
}

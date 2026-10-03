package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const importModel = `package V {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	enum def Grade { enum low; enum high; }
	part def Engine {
		attribute mass : MassValue default = 175 [kg];
		attribute supplier : String;
	}
	part vehicle {
		// the count stays commented
		attribute count : Integer = 3;
		attribute rated : Boolean;
		attribute grade : Grade;
		attribute label : String;
		part engine : Engine;
	}
}`

func importSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(importModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

func writeData(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportSetsDeclaredAndInheritedValues(t *testing.T) {
	s := importSession(t)
	path := writeData(t, "data.csv", "element,count,rated,grade,label,mass [kg],supplier\n"+
		"V::vehicle,7,true,high,42,,\n"+
		"V::vehicle::engine,,,,,180,\"Acme, Inc.\"\n")
	got := run(t, s, "%import "+path)
	wants(t, got,
		"✓ imported 6 values into 2 elements",
		"V::vehicle::count = 7", "V::vehicle::grade = V::Grade::high",
		"V::vehicle::engine::mass = 180 [kg] (redefines V::Engine::mass)")
	text := s.text()
	for _, want := range []string{
		"// the count stays commented", "attribute count : Integer = 7;",
		"attribute rated : Boolean = true;", "attribute grade : Grade = V::Grade::high;",
		`attribute label : String = "42";`, "attribute :>> mass = 180 [kg];",
		`attribute :>> supplier = "Acme, Inc.";`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	wants(t, run(t, s, "%eval V::vehicle::engine::mass"), "180")
}

func TestImportIsAtomic(t *testing.T) {
	s := importSession(t)
	before := s.text()
	path := writeData(t, "data.csv", "element,count\nV::vehicle,7\nV::vehicle,8\n")
	wants(t, run(t, s, "%import "+path), "line 3, column count", "already set", "the model was left unchanged")
	path = writeData(t, "bad.csv", "element,count,label\nV::vehicle,seven,x\n")
	wants(t, run(t, s, "%import "+path), "bad.csv line 2, column count: seven is not an integer")
	path = writeData(t, "unit.csv", "element,mass [m]\nV::vehicle::engine,3\n")
	wants(t, run(t, s, "%import "+path), "unit.csv line 2, column mass [m]", "the model was left unchanged")
	path = writeData(t, "missing.csv", "element,count,wheels\nV::vehicle,7,4\n")
	wants(t, run(t, s, "%import "+path), "column wheels: V::vehicle has no feature named wheels")
	if s.text() != before {
		t.Errorf("a refused import changed the model:\n%s", s.text())
	}
}

func TestImportDryRunAndJSON(t *testing.T) {
	s := importSession(t)
	before := s.text()
	path := writeData(t, "run.json", `{"results": [
		{"id": "vehicle", "n": 9},
		{"id": "vehicle::engine", "perf": {"m": 181.5, "u": "kg"}}]}`)
	mapPath := writeData(t, "map.json", `{"records": "/results", "element": {"column": "id", "prefix": "V::"},
		"features": {"count": {"column": "n"}, "mass": {"path": "/perf/m", "unitPath": "/perf/u"}}}`)
	wants(t, run(t, s, "%import "+path+" map "+mapPath+" dry-run"),
		"dry run: would import 2 values into 2 elements", "V::vehicle::count = 9",
		"V::vehicle::engine::mass = 181.5 [kg]")
	if s.text() != before {
		t.Errorf("a dry run changed the model")
	}
	lines := writeData(t, "run.jsonl", `{"element": "V::vehicle::engine", "mass [kg]": 181.5, "supplier": "Volt"}`+"\n"+
		`{"element": "V::vehicle", "rated": false}`+"\n")
	wants(t, run(t, s, "%import "+lines), "✓ imported 3 values into 2 elements", `supplier = "Volt"`)
	wants(t, s.text(), "attribute rated : Boolean = false;", "attribute :>> mass = 181.5 [kg];")
}

func TestImportReachesInheritedFeatures(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package 'Fleet A' {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	part def Engine { attribute mass : MassValue default = 175 [kg]; }
	part def 'Fuel Tank' { attribute 'fill level' : Real; }
	part def Car { part engine : Engine; part tank : 'Fuel Tank'; }
	part car1 : Car;
	part car2 : Car;
	alias c2 for car2;
}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	path := writeData(t, "cars.csv", "element,mass [kg],fill level\n"+
		"'Fleet A'::car1::engine,180,\n"+
		"'Fleet A'::car1::tank,,0.5\n"+
		"'Fleet A'::c2::engine,190,\n")
	wants(t, run(t, s, "%import "+path),
		"✓ imported 3 values into 3 elements",
		"Fleet A::car1::engine (redefines Fleet A::Car::engine)",
		"Fleet A::car2::engine::mass = 190 [kg] (redefines Fleet A::Engine::mass)")
	wants(t, s.text(), "part :>> engine {", "attribute :>> mass = 180 [kg];", "attribute :>> 'fill level' = 0.5;")
	wants(t, run(t, s, "%eval 'Fleet A'::car1::engine::mass"), "180")
	wants(t, run(t, s, "%eval 'Fleet A'::car2::engine::mass"), "190")
	before := s.text()
	path = writeData(t, "twice.csv", "element,mass [kg]\n'Fleet A'::car1::engine,181\n'Fleet A'::car1::engine,182\n")
	wants(t, run(t, s, "%import "+path), "line 3, column mass [kg]", "already set")
	path = writeData(t, "none.csv", "element,mass [kg]\n'Fleet A'::car1::wheel,4\n")
	wants(t, run(t, s, "%import "+path), "Fleet A::car1 has no member named wheel")
	if s.text() != before {
		t.Errorf("a refused import changed the model:\n%s", s.text())
	}
}

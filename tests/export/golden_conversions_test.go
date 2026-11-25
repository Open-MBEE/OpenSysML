package export_test

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

var update = flag.Bool("update", false, "rewrite the .golden.ttl and .golden.sysml files")

// TestGoldenConversions locks each model's Turtle and the notation it converts
// back to: as written with its source text, `.canonical.golden.sysml` without.
func TestGoldenConversions(t *testing.T) {
	t.Parallel()
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert(name+".ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, turtle)
			}
			if want := string(src); string(back) != want {
				t.Errorf("the model did not come back as written:\n--- want ---\n%s--- got ---\n%s", want, back)
			}
			canonical, err := convert.Convert(name+".ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				canonical = []byte("sysml: " + err.Error() + "\n")
			}
			stem := strings.TrimSuffix(path, ext)
			checkGolden(t, stem+".golden.ttl", turtle)
			checkGolden(t, stem+".golden"+ext, back)
			checkGolden(t, stem+".canonical.golden"+ext, canonical)
		})
	}
}

// TestConvertedNotationParses checks that the notation written from a graph is
// valid SysML: it must parse without a single syntax error.
func TestConvertedNotationParses(t *testing.T) {
	t.Parallel()
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert(name+".ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			p := parser.New(source.New(name+".converted"+ext, back))
			p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Errorf("converted notation does not parse: %v\n%s", p.Diagnostics, back)
			}
		})
	}
}

// textOnlyFixtures are the models whose graph the mapping cannot write back
// without its source text, by the refusal it must keep reporting for them.
var textOnlyFixtures = map[string]string{}

// TestRoundTripIsLossless is the fidelity contract: converting the notation a
// graph produced back to a graph gives the same graph. Notation and RDF say the
// same thing in different words, so this is what "no data lost" means — the
// notation itself may legitimately be spelled differently (a name written
// relative to its scope, a keyword written in place of its symbol).
func TestRoundTripIsLossless(t *testing.T) {
	t.Parallel()
	for _, path := range modelFiles(t) {
		name, ext := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert(name+".ttl", first, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			second, err := convert.Convert(name+ext, back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(first) != string(second) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
			if textOnly, ok := textOnlyFixtures[name]; ok {
				_, err := convert.Convert(name+".ttl", withoutTriples(t, first, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
				var unsupported *export.UnsupportedError
				if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), textOnly) {
					t.Fatalf("the mapping alone should still refuse %s (%s), got: %v", name, textOnly, err)
				}
				return
			}
			if name == "payload_declaration_bodies" {
				if _, err := convert.Convert(name+".ttl", withoutTriples(t, first, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML); err != nil {
					t.Fatalf("the mapping alone should infer flow end forms: %v", err)
				}
				return
			}
			structuralRoundTrip(t, name+ext, first)
		})
	}
}

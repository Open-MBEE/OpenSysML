package export_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

var qualifiedUsagesPath = filepath.Join("testdata", "convert", "qualified_usages.sysml")

// withoutSysxJSON strips every sysx: key from an api-json document.
func withoutSysxJSON(t *testing.T, path string) []byte {
	t.Helper()
	var elements []any
	document, err := convert.Convert(path, mustRead(t, path), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	var scrub func(v any) any
	scrub = func(v any) any {
		switch m := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, item := range m {
				if strings.HasPrefix(k, "sysx:") {
					continue
				}
				out[k] = scrub(item)
			}
			return out
		case []any:
			list := make([]any, 0, len(m))
			for _, item := range m {
				list = append(list, scrub(item))
			}
			return list
		}
		return v
	}
	document, err = json.Marshal(scrub(elements))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// withoutSysx strips every sysx: predicate from a Turtle document, property by
// property, so the typed facts alone carry the notation.
func withoutSysx(t *testing.T, turtle []byte) []byte {
	t.Helper()
	properties := map[string]bool{}
	for _, m := range regexp.MustCompile(`sysx:\w+`).FindAllString(string(turtle), -1) {
		properties[m] = true
	}
	for property := range properties {
		turtle = withoutTriples(t, turtle, property)
	}
	return turtle
}

// The qualified usages' metaclasses carry their keywords: perform, exhibit,
// include, assert and satisfy all come back from the typed facts alone.
func TestQualifiedUsageKeywordsFromMetaclass(t *testing.T) {
	want := mustRead(t, qualifiedUsagesPath)

	back, err := convert.Convert("m.json", withoutSysxJSON(t, qualifiedUsagesPath), convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json without sysx: %v", err)
	}
	if string(back) != string(want) {
		t.Errorf("the metaclasses alone did not spell the keywords:\n--- want ---\n%s\n--- got ---\n%s", want, back)
	}

	turtle, err := convert.Convert(qualifiedUsagesPath, want, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err = convert.Convert("m.ttl", withoutSysx(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("turtle without sysx: %v", err)
	}
	if string(back) != string(want) {
		t.Errorf("the turtle metaclasses alone did not spell the keywords:\n--- want ---\n%s\n--- got ---\n%s", want, back)
	}
}

// The toolkit's compact element form of the same model decodes to the same
// qualified forms: `perform`/`exhibit` named and `a.b :>> x` references, the
// `include`/`assert`/`satisfy` reference forms unnamed.
func TestToolkitQualifiedUsagesDecode(t *testing.T) {
	toolkit := mustRead(t, filepath.Join("testdata", "interchange", "qualified_usages.toolkit.compact.json"))
	back, err := convert.Convert("m.json", toolkit, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	for _, want := range []string{
		"perform action pa", "perform a1;", "perform sub.sa :>> a2;",
		"exhibit state es", "exhibit s1;", "exhibit sub.ss :>> s2;",
		"include use case iu", "include u1;",
		"satisfy requirement sr", "satisfy r1;",
		"assert constraint ac", "assert c1;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the toolkit graph should decode %q\n--- notation ---\n%s", want, back)
		}
	}
}

// A recorded qualifier keyword that contradicts the metaclass is refused:
// `perform` cannot stand on a plain ActionUsage.
func TestQualifierKeywordContradictsMetaclass(t *testing.T) {
	var elements []map[string]any
	if err := json.Unmarshal(withoutSysxJSON(t, qualifiedUsagesPath), &elements); err != nil {
		t.Fatal(err)
	}
	marked := false
	for _, el := range elements {
		if el["@type"] == "ActionUsage" && el["declaredName"] == "a1" {
			el["sysx:declaredKeyword"] = "perform"
			marked = true
		}
	}
	if !marked {
		t.Fatal("no ActionUsage a1 found to mark")
	}
	document, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	_, err = convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an UnsupportedError, got %v", err)
	}
	for _, want := range []string{"`perform` declaration", "the metaclass ActionUsage", "PerformActionUsage"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in error:\n%s", want, err)
		}
	}
}

// The decode of the toolkit's element form re-encodes with the same per-@type
// counts of qualified usages our own graph states.
func TestQualifiedUsageTypeCountsSurviveToolkitDecode(t *testing.T) {
	count := func(document []byte) map[string]int {
		var elements []map[string]any
		if err := json.Unmarshal(document, &elements); err != nil {
			t.Fatalf("the api-json document does not parse: %v", err)
		}
		out := map[string]int{}
		for _, el := range elements {
			if m, ok := el["@type"].(string); ok {
				out[m]++
			}
		}
		return out
	}
	ours := count(mustConvert(t, qualifiedUsagesPath, convert.FormatAPIJSON))
	toolkit := mustRead(t, filepath.Join("testdata", "interchange", "qualified_usages.toolkit.compact.json"))
	back, err := convert.Convert("m.json", toolkit, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	redecoded, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	got := count(redecoded)
	for _, metaclass := range []string{
		"PerformActionUsage", "ExhibitStateUsage", "IncludeUseCaseUsage",
		"AssertConstraintUsage", "SatisfyRequirementUsage",
	} {
		if got[metaclass] != ours[metaclass] {
			t.Errorf("%s: our graph states %d, the toolkit-decoded re-encode states %d", metaclass, ours[metaclass], got[metaclass])
		}
	}
}

func mustConvert(t *testing.T, path string, to convert.Format) []byte {
	t.Helper()
	document, err := convert.Convert(path, mustRead(t, path), convert.FormatSysML, to)
	if err != nil {
		t.Fatalf("to %s: %v", to, err)
	}
	return document
}

// A state's own members are owned by plain FeatureMemberships, perform
// members included — the qualifier exemption covers a transition's effect
// (the legacy shape), not them.
func TestStateBodyPerformKeepsQualifier(t *testing.T) {
	src := "package P {\n\taction def A;\n\taction a1 : A;\n\tstate def S;\n\tstate running : S {\n\t\tperform action step : A;\n\t\tperform a1;\n\t\tdo action stop : A;\n\t}\n}"
	document, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	var scrub func(v any) any
	scrub = func(v any) any {
		switch m := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, item := range m {
				if strings.HasPrefix(k, "sysx:") {
					continue
				}
				out[k] = scrub(item)
			}
			return out
		case []any:
			list := make([]any, 0, len(m))
			for _, item := range m {
				list = append(list, scrub(item))
			}
			return list
		}
		return v
	}
	document, err = json.Marshal(scrub(elements))
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json without sysx: %v", err)
	}
	for _, want := range []string{
		"perform action 'step' : A;",
		"perform a1;",
		"do action stop : A;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("expected %q in:\n%s", want, back)
		}
	}
}

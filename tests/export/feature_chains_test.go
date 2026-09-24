package export_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// apiJSON converts a fixture to the API element form, parsed as a document.
func apiJSON(t *testing.T, path string) []map[string]any {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	format, err := convert.FormatOfPath(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := convert.Convert(path, src, format, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the api-json document does not parse: %v", err)
	}
	return elements
}

func elementsByID(elements []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, el := range elements {
		if id, ok := el["@id"].(string); ok {
			out[id] = el
		}
	}
	return out
}

// refID is the @id a {..} reference names, or "" for a {"@ref": name}.
func refID(v any) string {
	if m, ok := v.(map[string]any); ok {
		if id, ok := m["@id"].(string); ok {
			return id
		}
	}
	return ""
}

func refIDs(v any) []string {
	var out []string
	switch list := v.(type) {
	case []any:
		for _, item := range list {
			out = append(out, refID(item))
		}
	case map[string]any:
		out = append(out, refID(list))
	}
	return out
}

// chainFeatures are the unnamed Feature elements stating chainingFeature links.
func chainFeatures(elements []map[string]any) []map[string]any {
	var out []map[string]any
	for _, el := range elements {
		if el["@type"] == "Feature" {
			if _, ok := el["chainingFeature"]; ok {
				out = append(out, el)
			}
		}
	}
	return out
}

// featureChainingsOf returns a chain feature's FeatureChaining elements in
// ownedRelationship order.
func featureChainingsOf(t *testing.T, byID map[string]map[string]any, chain map[string]any) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, relID := range refIDs(chain["ownedRelationship"]) {
		if rel := byID[relID]; rel != nil && rel["@type"] == "FeatureChaining" {
			out = append(out, rel)
		}
	}
	if len(out) == 0 {
		t.Fatalf("the chain feature %v owns no FeatureChaining", chain["@id"])
	}
	return out
}

// A chain feature owns one FeatureChaining per link, owned by the relationship
// it belongs to, and keeps the derived chainingFeature list beside it; an end
// of an interface is a PortUsage, of a connection a ReferenceUsage.
func TestFeatureChainingEmission(t *testing.T) {
	elements := apiJSON(t, filepath.Join("testdata", "convert", "feature_chains.sysml"))
	byID := elementsByID(elements)
	chains := chainFeatures(elements)
	if len(chains) < 5 {
		t.Fatalf("expected the connector, interface, expression and head chains (>=5), got %d", len(chains))
	}
	for _, chain := range chains {
		id := chain["@id"].(string)
		if strings.HasSuffix(id, "_om") || strings.Contains(id, "_pchain_om") {
			t.Errorf("chain %s owns an OwningMembership rather than the relationship's ownedRelatedElement", id)
		}
		for _, el := range elements {
			if strings.Contains(el["@id"].(string), "_pchain_om") {
				t.Errorf("a _pchain_om OwningMembership remains: %s", el["@id"])
			}
		}
		links := featureChainingsOf(t, byID, chain)
		derived := refIDs(chain["chainingFeature"])
		if len(derived) != len(links) {
			t.Fatalf("chain %s: derived chainingFeature list %v does not match its %d FeatureChainings", id, derived, len(links))
		}
		for i, fc := range links {
			if got := refID(fc["chainingFeature"]); got != derived[i] {
				t.Errorf("chain %s link %d: chainingFeature %v, derived %v", id, i, got, derived[i])
			}
			if got := refID(fc["owningRelatedElement"]); got != id {
				t.Errorf("FeatureChaining %v: owningRelatedElement is %v, not the chain %s", fc["@id"], got, id)
			}
			if got := refID(fc["owner"]); got != id {
				t.Errorf("FeatureChaining %v: owner is %v, not the chain %s", fc["@id"], got, id)
			}
			for _, property := range []string{"source", "target", "relatedElement"} {
				if len(refIDs(fc[property])) == 0 {
					t.Errorf("FeatureChaining %v states no %s", fc["@id"], property)
				}
			}
		}
	}
	// The connector and interface end chains are owned by a ReferenceSubsetting,
	// the head chain by the Redefinition; the value chain is the nested
	// FeatureChainExpression form.
	var sawSubsetting, sawRedefinition, sawChainExpression int
	for _, el := range elements {
		switch el["@type"] {
		case "FeatureChainExpression":
			sawChainExpression++
		case "Feature":
			if _, chained := el["chainingFeature"]; !chained {
				continue
			}
			switch byID[refID(el["owningRelationship"])]["@type"] {
			case "ReferenceSubsetting":
				sawSubsetting++
			case "Redefinition":
				sawRedefinition++
			}
		}
	}
	if sawSubsetting == 0 || sawRedefinition == 0 || sawChainExpression == 0 {
		t.Errorf("chain forms: ReferenceSubsetting-owned=%d Redefinition-owned=%d FeatureChainExpression=%d, want all > 0",
			sawSubsetting, sawRedefinition, sawChainExpression)
	}
	// Ends: the interface's chain ends are PortUsage, the connection's ReferenceUsage.
	var portEnds, refEnds int
	for _, el := range elements {
		if el["isEnd"] != true {
			continue
		}
		switch el["@type"] {
		case "PortUsage":
			portEnds++
		case "ReferenceUsage":
			refEnds++
		}
	}
	if portEnds == 0 || refEnds == 0 {
		t.Errorf("end metaclasses: PortUsage=%d ReferenceUsage=%d, want both > 0", portEnds, refEnds)
	}
}

// An unresolved link is written as {"@ref": name}, never a bare string — in the
// chain feature's derived list and in each FeatureChaining alike.
func TestFeatureChainUnresolvedLinkIsRef(t *testing.T) {
	elements := apiJSON(t, filepath.Join("testdata", "convert", "feature_chains_unresolved.sysml"))
	var sawRef bool
	var check func(v any, where string)
	check = func(v any, where string) {
		switch m := v.(type) {
		case map[string]any:
			for key, item := range m {
				if key == "chainingFeature" {
					switch item := item.(type) {
					case string:
						t.Errorf("%s: chainingFeature is the bare string %q", where, item)
					case map[string]any:
						if _, isRef := item["@ref"]; isRef {
							sawRef = true
						}
					case []any:
						for _, entry := range item {
							if _, ok := entry.(string); ok {
								t.Errorf("%s: chainingFeature lists the bare string %v", where, entry)
							}
							if m, ok := entry.(map[string]any); ok {
								if _, isRef := m["@ref"]; isRef {
									sawRef = true
								}
							}
						}
					}
				}
				check(item, where)
			}
		case []any:
			for _, item := range m {
				check(item, where)
			}
		}
	}
	for _, el := range elements {
		check(el, el["@id"].(string))
	}
	if !sawRef {
		t.Error("expected at least one {\"@ref\": name} link through the undeclared type")
	}
}

// The reader takes the normative FeatureChaining elements, the derived
// chainingFeature list, or both — and refuses the two disagreeing.
func TestFeatureChainReaderForms(t *testing.T) {
	path := filepath.Join("testdata", "convert", "feature_chains.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatSysML)
	if err != nil {
		t.Fatal(err)
	}
	write := func(mutate func([]map[string]any) []map[string]any) []byte {
		document, err := json.Marshal(mutate(apiJSON(t, path)))
		if err != nil {
			t.Fatal(err)
		}
		return document
	}
	decode := func(document []byte) ([]byte, error) {
		return convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	}

	t.Run("normative only", func(t *testing.T) {
		back, err := decode(write(func(elements []map[string]any) []map[string]any {
			for _, el := range elements {
				if _, derived := el["chainingFeature"].([]any); derived {
					delete(el, "chainingFeature")
				}
			}
			return elements
		}))
		if err != nil {
			t.Fatalf("normative-only decode: %v", err)
		}
		if string(back) != string(want) {
			t.Errorf("normative-only decodes differently:\n--- want ---\n%s\n--- got ---\n%s", want, back)
		}
	})

	t.Run("derived only", func(t *testing.T) {
		back, err := decode(write(func(elements []map[string]any) []map[string]any {
			chaining := map[string]bool{}
			var kept []map[string]any
			for _, el := range elements {
				if el["@type"] == "FeatureChaining" {
					chaining[el["@id"].(string)] = true
					continue
				}
				kept = append(kept, el)
			}
			var scrub func(v any) any
			scrub = func(v any) any {
				switch m := v.(type) {
				case map[string]any:
					if id := refID(m); chaining[id] {
						return nil
					}
					for k, item := range m {
						m[k] = scrub(item)
					}
					return m
				case []any:
					var list []any
					for _, item := range m {
						if s := scrub(item); s != nil {
							list = append(list, s)
						}
					}
					return list
				}
				return v
			}
			for i, el := range kept {
				kept[i] = scrub(el).(map[string]any)
			}
			return kept
		}))
		if err != nil {
			t.Fatalf("derived-only decode: %v", err)
		}
		if string(back) != string(want) {
			t.Errorf("derived-only decodes differently:\n--- want ---\n%s\n--- got ---\n%s", want, back)
		}
	})

	t.Run("disagreeing", func(t *testing.T) {
		_, err := decode(write(func(elements []map[string]any) []map[string]any {
			for _, el := range elements {
				list, ok := el["chainingFeature"].([]any)
				if !ok || len(list) < 3 {
					continue
				}
				list[1], list[2] = list[2], list[1]
				return elements
			}
			return elements
		}))
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("expected an UnsupportedError for disagreeing chains, got %v", err)
		}
		if !strings.Contains(err.Error(), "chainingFeature") || !strings.Contains(err.Error(), "chain feature") {
			t.Errorf("the error should name the chain feature and chainingFeature:\n%s", err)
		}
	})
}

// The same graph as Turtle round-trips the chains byte-identically once the
// source text is gone.
func TestFeatureChainTurtleRoundTrip(t *testing.T) {
	path := filepath.Join("testdata", "convert", "feature_chains.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("turtle to notation: %v", err)
	}
	if string(back) != string(src) {
		t.Errorf("the turtle did not round-trip:\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}
}

// The toolkit's compact element form of the same model decodes to the chains
// written as `a.b.c`.
func TestToolkitFeatureChainsDecode(t *testing.T) {
	toolkit, err := os.ReadFile(filepath.Join("testdata", "interchange", "feature_chains.toolkit.compact.json"))
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("m.json", toolkit, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	for _, want := range []string{
		"connection launchVehicle.instrumentUnit.pip to supplier.pip;",
		"connect launchVehicle.instrumentUnit.pip to supplier.pip;",
		"= launchVehicle.instrumentUnit.pip;",
		"redefines launchVehicle.instrumentUnit",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the toolkit graph should decode the chain %q\n--- notation ---\n%s", want, back)
		}
	}
}

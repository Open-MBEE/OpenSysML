package export_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func toolkitFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "interchange", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeToolkitFixture(t *testing.T, name string, warn func(string)) []byte {
	t.Helper()
	graph, err := export.ReadAPIJSON(toolkitFixture(t, name))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	out, err := export.ToSysMLWarn(graph, warn)
	if err != nil {
		t.Fatalf("ToSysMLWarn: %v", err)
	}
	file := source.New(name+".sysml", out)
	p := parser.New(file)
	p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("the decoded notation does not parse: %v\n%s", p.Diagnostics, out)
	}
	ws := model.NewWorkspace()
	ws.Open(name+".sysml", out, 1)
	for _, diagnostic := range ws.Diagnostics(name + ".sysml") {
		if diagnostic.Severity == diag.SeverityError {
			t.Errorf("the decoded notation does not validate: %s\n%s", diagnostic.Message, out)
		}
	}
	return out
}

func TestToolkitFlowEndAndPayload(t *testing.T) {
	out := decodeToolkitFixture(t, "flow_ends.toolkit.full.json", nil)
	want := toolkitFixture(t, "flow_ends.golden.sysml")
	if !bytes.Equal(out, want) {
		t.Fatalf("the decoded flow notation changed:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
	if !strings.Contains(string(out), "flow of Payload from source.holder.payload to target.holder.payload;") {
		t.Fatalf("the flow ends or payload were not written in the flow head:\n%s", out)
	}
	if strings.Contains(string(out), "composite flow") {
		t.Fatalf("the flow has a composite modifier the toolkit parser does not accept:\n%s", out)
	}
}

func TestToolkitFlowEndRefusesUnrepresentableDetails(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]map[string]any)
		want string
	}{
		{
			name: "declared name",
			edit: func(objects []map[string]any) {
				for _, object := range objects {
					if object["@type"] == "FlowEnd" {
						object["declaredName"] = "explicit"
						return
					}
				}
			},
			want: "declares a name",
		},
		{
			name: "mismatched derived name",
			edit: func(objects []map[string]any) {
				for _, object := range objects {
					if object["@type"] == "ReferenceUsage" && object["name"] == "payload" {
						object["name"] = "wrong"
					}
				}
			},
			want: "derived name does not match",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var objects []map[string]any
			if err := json.Unmarshal(toolkitFixture(t, "flow_ends.toolkit.full.json"), &objects); err != nil {
				t.Fatal(err)
			}
			tc.edit(objects)
			data, err := json.Marshal(objects)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := export.ReadAPIJSON(data)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v", err)
			}
			_, err = export.ToSysML(graph)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ToSysML error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestToolkitTransitionClauses(t *testing.T) {
	out := decodeToolkitFixture(t, "transitions.toolkit.full.json", nil)
	want := toolkitFixture(t, "transitions.golden.sysml")
	if !bytes.Equal(out, want) {
		t.Fatalf("the decoded transition notation changed:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
	text := string(out)
	for _, clause := range []string{
		"transition triggered first x accept Go then y;",
		"transition guarded first y accept Go if ready then x;",
		"transition effectful first x do action reset : Reset then y;",
	} {
		if !strings.Contains(text, clause) {
			t.Errorf("the decoded transition notation lacks %q:\n%s", clause, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "first start then x;" {
			return
		}
	}
	t.Fatalf("the entry was not kept as the positional `first start then x;` form:\n%s", text)
}

func TestToolkitInferredSuccessionClause(t *testing.T) {
	out := decodeToolkitFixture(t, "successions.toolkit.full.json", nil)
	want := toolkitFixture(t, "successions.golden.sysml")
	if !bytes.Equal(out, want) {
		t.Fatalf("the decoded succession notation changed:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
	if !strings.Contains(string(out), "succession first firstEvent then secondEvent;") {
		t.Fatalf("the inferred succession lacks its required `first` clause:\n%s", out)
	}
}

func TestToolkitLibraryFallbackWarns(t *testing.T) {
	const oldID = "11111111-2222-4333-8444-555555555555"
	var warnings []string
	// The toolkit fixture's ScalarValues::Real stub has a hand-edited element ID.
	out := decodeToolkitFixture(t, "library_identity.toolkit.full.json", func(message string) {
		warnings = append(warnings, message)
	})
	want := toolkitFixture(t, "library_identity.golden.sysml")
	if !bytes.Equal(out, want) {
		t.Fatalf("the decoded library notation changed:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
	if !strings.Contains(string(out), "library package DocumentLibrary") {
		t.Fatalf("the document-defined library package was not written as notation:\n%s", out)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one library identity warning", warnings)
	}
	for _, part := range []string{oldID, "resolved by its qualified name ScalarValues::Real", "14c0aa22-5489-59b5-b438-ded26e83ba31"} {
		if !strings.Contains(warnings[0], part) {
			t.Errorf("warning %q does not identify %q", warnings[0], part)
		}
	}
}

func TestToolkitUnmatchedLibraryStubRefuses(t *testing.T) {
	graph, err := export.ReadAPIJSON(toolkitFixture(t, "library_unmatched.toolkit.full.json"))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	_, err = export.ToSysMLWarn(graph, nil)
	if err == nil {
		t.Fatal("an unmatched library stub decoded")
	}
	for _, part := range []string{"11111111-2222-4333-8444-555555555555", "Missing::MissingType"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not identify %q", err, part)
		}
	}
}

func TestToolkitLibraryFallbackRequiresCompatibleMetaclass(t *testing.T) {
	const oldID = "11111111-2222-4333-8444-555555555555"
	var objects []map[string]any
	if err := json.Unmarshal(toolkitFixture(t, "library_identity.toolkit.full.json"), &objects); err != nil {
		t.Fatal(err)
	}
	for _, object := range objects {
		if object["@id"] == oldID {
			object["@type"] = "PartDefinition"
		}
	}
	data, err := json.Marshal(objects)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := export.ReadAPIJSON(data)
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	var warnings []string
	_, err = export.ToSysMLWarn(graph, func(message string) {
		warnings = append(warnings, message)
	})
	if err == nil || !strings.Contains(err.Error(), oldID) || !strings.Contains(err.Error(), "ScalarValues::Real") {
		t.Fatalf("ToSysMLWarn error = %v, want the incompatible library identity and stated name", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want no fallback warning for an incompatible metaclass", warnings)
	}
}

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

func TestGeneratedMetamodel(t *testing.T) {
	generated, err := generate(ontology.Classes(), ontology.Properties(), ontology.Enumerations(),
		ontology.Version, ontology.SourceTag, ontology.SourceCommit)
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	if !strings.Contains(source, `class FeatureDirectionKind(str, Enum):`) ||
		!strings.Contains(source, `    IN = "in"`) {
		t.Fatal("generated enumerations are missing")
	}
	if strings.Index(source, "class Type(") > strings.Index(source, "class PartUsage(") {
		t.Fatal("metaclasses are not generated parent-first")
	}
	if !strings.Contains(source, `"partDefinition": "part_definition"`) {
		t.Fatal("generated JSON-key mapping is missing")
	}
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatal("committed generated Python metamodel is stale")
	}
}

func TestCheckRejectsModifiedOutput(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "stale.py")
	if err := os.WriteFile(output, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "run", "./gen/pymetamodel", "-check", "-out", output)
	command.Dir = filepath.Join(root, "tools")
	result, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(result), "is stale; regenerate with") {
		t.Fatalf("generator check output = %s, error = %v", result, err)
	}
}

func TestC3LinearizationOrdersSharedAncestorsOnce(t *testing.T) {
	classes := []ontology.Class{
		{Name: "Element"},
		{Name: "Base", Parents: []string{"Element"}},
		{Name: "Left", Parents: []string{"Base"}},
		{Name: "Right", Parents: []string{"Base"}},
		{Name: "Leaf", Parents: []string{"Left", "Right"}},
	}
	m, err := newModel(classes, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Leaf", "Left", "Right", "Base", "Element"}
	if got := strings.Join(m.mros["Leaf"], ","); got != strings.Join(want, ",") {
		t.Fatalf("Leaf MRO = %v, want %v", m.mros["Leaf"], want)
	}
}

func TestGeneratorRejectsInvalidTables(t *testing.T) {
	tests := []struct {
		name       string
		classes    []ontology.Class
		properties []ontology.Property
		enums      []ontology.Enumeration
		want       string
	}{
		{
			name: "related property redeclaration",
			classes: []ontology.Class{
				{Name: "Element"},
				{Name: "Parent", Parents: []string{"Element"}},
				{Name: "Child", Parents: []string{"Parent"}},
			},
			properties: []ontology.Property{
				testProperty("Parent", "feature"),
				testProperty("Child", "feature"),
			},
			want: "declared by related metaclasses",
		},
		{
			name: "duplicate snake name",
			classes: []ontology.Class{
				{Name: "Element"},
				{Name: "Node", Parents: []string{"Element"}},
			},
			properties: []ontology.Property{
				testProperty("Node", "someName"),
				testProperty("Node", "some_name"),
			},
			want: "same Python name",
		},
		{
			name: "runtime member collision",
			classes: []ontology.Class{
				{Name: "Element"},
				{Name: "Node", Parents: []string{"Element"}},
			},
			properties: []ontology.Property{testProperty("Node", "jsonId")},
			want:       "collides with runtime member",
		},
		{
			name: "class body shadowing",
			classes: []ontology.Class{
				{Name: "Element"},
				{Name: "Node", Parents: []string{"Element"}},
			},
			properties: []ontology.Property{testProperty("Node", "Mapping")},
			want:       "shadows generated class-body name",
		},
		{
			name:    "invalid enum literal",
			classes: []ontology.Class{{Name: "Element"}},
			enums:   []ontology.Enumeration{{Name: "Direction", Literals: []string{"bad-literal"}}},
			want:    "does not produce a valid Python identifier",
		},
		{
			name: "inconsistent C3 order",
			classes: []ontology.Class{
				{Name: "Element"},
				{Name: "O"},
				{Name: "X", Parents: []string{"O"}},
				{Name: "Y", Parents: []string{"O"}},
				{Name: "A", Parents: []string{"X", "Y"}},
				{Name: "B", Parents: []string{"Y", "X"}},
				{Name: "Z", Parents: []string{"A", "B"}},
			},
			want: "C3 linearization failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generate(tt.classes, tt.properties, tt.enums, "version", "tag", "commit")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("generate error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestSnakeCasePreservesAcronymsAndDigits(t *testing.T) {
	for input, want := range map[string]string{
		"ownedFeature": "owned_feature",
		"XMLValue":     "xml_value",
		"part2D":       "part2_d",
		"in":           "in_",
	} {
		if got := toSnake(input); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", input, got, want)
		}
	}
}

func testProperty(class, name string) ontology.Property {
	return ontology.Property{
		Name: name, DefiningClass: class, Kind: ontology.DatatypeProperty,
		Range: xsdNS + "string",
	}
}

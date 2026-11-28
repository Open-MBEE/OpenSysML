package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

var update = flag.Bool("update", false, "rewrite the synthetic golden")

// syntheticGolden is compiled and exercised by the .NET metamodel tests
// (SyntheticGenerationTests), so the redefinition and diamond paths the real
// metamodel does not take are proven to be valid C# that reads correctly.
const syntheticGolden = "client/dotnet/test/testdata/metamodel/synthetic.golden.cs"

func TestGeneratedMetamodel(t *testing.T) {
	generated, err := generate(ontology.Classes(), ontology.Properties(), ontology.Enumerations(),
		ontology.Version, ontology.SourceTag, ontology.SourceCommit)
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	for _, want := range []string{
		"    public enum FeatureDirectionKind\n    {\n        /// <summary>The literal <c>in</c>.</summary>\n        In = 0,",
		"    public interface IElement : IMetamodelElement\n",
		"    public interface IActionUsage : IOccurrenceUsage, IStep\n",
		"        IReadOnlyList<IFeature> OwnedFeature { get; }\n",
		"        PortionKind? PortionKind { get; }\n",
		"        IMetaclass? Metaclass { get; }\n",
		"    public sealed class PartUsage : MetamodelElement, IPartUsage\n",
		"        string? IElement.DeclaredName => ReadText(Metaclasses.Element_declaredName);\n",
		"        public static MetaclassInfo Element { get; } = new(\"Element\", true, typeof(IElement), null, static (id, jsonType) => new global::OpenMBEE.OpenSysML.Metamodel.Elements.Element(id, jsonType));\n",
		"        public static MetaclassInfo PartUsage { get; } = new(\"PartUsage\", false, typeof(IPartUsage), typeof(global::OpenMBEE.OpenSysML.Metamodel.Elements.PartUsage), static (id, jsonType) => new global::OpenMBEE.OpenSysML.Metamodel.Elements.PartUsage(id, jsonType));\n",
		"    internal sealed class Element : MetamodelElement, IElement\n",
		"    internal sealed class Relationship : MetamodelElement, IRelationship\n",
		"            PartUsage.Initialize([ItemUsage], [ItemUsage, OccurrenceUsage, Usage, Feature, Type, Namespace, Element], [PartUsage_partDefinition]);\n",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated metamodel lacks %q", want)
		}
	}
	for _, class := range ontology.Classes() {
		public := strings.Contains(source, "    public sealed class "+class.Name+" : ")
		internal := strings.Contains(source, "    internal sealed class "+class.Name+" : ")
		if public == class.Abstract || internal != class.Abstract {
			t.Errorf("metaclass %s (abstract %t): public element class %t, internal %t", class.Name, class.Abstract, public, internal)
		}
	}
	if strings.Index(source, "            Type.Initialize(") > strings.Index(source, "            PartUsage.Initialize(") {
		t.Error("metaclasses are not initialized parents first")
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
		t.Fatalf("committed %s is stale; regenerate with `%s`", outputPath, regenerate)
	}
}

func TestCheckRejectsModifiedOutputAndWritesCurrentOne(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "Generated.cs")
	run := func(args ...string) (string, error) {
		command := exec.Command("go", append([]string{"run", "./gen/csmetamodel"}, args...)...)
		command.Dir = filepath.Join(root, "tools")
		result, err := command.CombinedOutput()
		return string(result), err
	}
	if err := os.WriteFile(output, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := run("-check", "-out", output); err == nil || !strings.Contains(result, "is stale; regenerate with `make dotnet-metamodel`") {
		t.Fatalf("check of a stale file: output = %s, error = %v", result, err)
	}
	if have, _ := os.ReadFile(output); string(have) != "stale\n" {
		t.Fatal("-check wrote its output")
	}
	if result, err := run("-out", output); err != nil || !strings.Contains(result, "175 metaclasses, 415 properties, 7 enumerations") {
		t.Fatalf("generation: output = %s, error = %v", result, err)
	}
	if result, err := run("-check", "-out", output); err != nil || !strings.Contains(result, "is current") {
		t.Fatalf("check of a current file: output = %s, error = %v", result, err)
	}
}

// syntheticTable holds what the real metamodel lacks: a diamond whose shared
// ancestor declares properties, and same-named redefinitions that narrow a
// range across two generations, besides every kind of range.
func syntheticTable() ([]ontology.Class, []ontology.Property, []ontology.Enumeration) {
	classes := []ontology.Class{
		{Name: "Element", Abstract: true},
		{Name: "Namespace", Parents: []string{"Element"}},
		{Name: "Definition", Parents: []string{"Element"}},
		{Name: "PartDefinition", Parents: []string{"Definition"}},
		{Name: "Base", Abstract: true, Parents: []string{"Element"}},
		{Name: "Left", Parents: []string{"Base"}},
		{Name: "Right", Parents: []string{"Base"}},
		{Name: "Leaf", Parents: []string{"Left", "Right"}},
		{Name: "Narrow", Parents: []string{"Base"}},
		{Name: "Narrower", Parents: []string{"Narrow"}},
	}
	object := func(class, name, rangeClass string, many bool) ontology.Property {
		return ontology.Property{Name: name, DefiningClass: class, Kind: ontology.ObjectProperty,
			Range: sysmlNS + rangeClass, Many: many, Ordered: many}
	}
	datatype := func(class, name, rangeIRI string) ontology.Property {
		return ontology.Property{Name: name, DefiningClass: class, Kind: ontology.DatatypeProperty, Range: rangeIRI}
	}
	owner := object("Element", "owner", "Element", false)
	owner.Derived, owner.Opposite = true, "Element::ownedElement"
	owned := object("Element", "ownedElement", "Element", true)
	owned.Derived, owned.Opposite = true, "Element::owner"
	library := datatype("Element", "isLibraryElement", xsdNS+"boolean")
	library.Lower, library.HasDefault, library.Default = 1, true, "false"
	aliases := datatype("Element", "aliasIds", xsdNS+"string")
	aliases.Many, aliases.Ordered, aliases.Nonunique = true, true, true
	narrowDefinition := object("Narrow", "definition", "PartDefinition", false)
	narrowDefinition.Redefines = []string{"Base::definition"}
	narrowDefinitions := object("Narrow", "definitions", "PartDefinition", true)
	narrowDefinitions.Redefines = []string{"Base::definitions"}
	narrowerDefinition := object("Narrower", "definition", "PartDefinition", false)
	narrowerDefinition.Redefines = []string{"Narrow::definition"}
	leftOnly := datatype("Left", "leftName", xsdNS+"string")
	leftOnly.Subsets = []string{"Element::declaredName"}
	properties := []ontology.Property{
		datatype("Element", "declaredName", xsdNS+"string"), owner, owned, library, aliases,
		object("Base", "definition", "Definition", false),
		object("Base", "definitions", "Definition", true),
		datatype("Base", "direction", sysmlNS+"Direction"),
		datatype("Base", "size", xsdNS+"int"),
		datatype("Base", "weight", owlNS+"real"),
		leftOnly, narrowDefinition, narrowDefinitions, narrowerDefinition,
	}
	enums := []ontology.Enumeration{{Name: "Direction", Literals: []string{"in", "inout", "out"}}}
	return classes, properties, enums
}

func TestSyntheticGolden(t *testing.T) {
	classes, properties, enums := syntheticTable()
	generated, err := generate(classes, properties, enums, "synthetic", "none", "none")
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	for _, want := range []string{
		"    public interface ILeaf : ILeft, IRight\n",
		"        new IPartDefinition? Definition { get; }\n",
		"        new IReadOnlyList<IPartDefinition> Definitions { get; }\n",
		"        IDefinition? IBase.Definition => ReadReference<IPartDefinition>(Metaclasses.Narrower_definition);\n",
		"        IPartDefinition? INarrow.Definition => ReadReference<IPartDefinition>(Metaclasses.Narrower_definition);\n",
		"        IReadOnlyList<IDefinition> IBase.Definitions => ReadList<IPartDefinition>(Metaclasses.Narrow_definitions);\n",
		"        IDefinition? IBase.Definition => ReadReference<IDefinition>(Metaclasses.Base_definition);\n",
		"            Leaf.Initialize([Left, Right], [Left, Right, Base, Element], []);\n",
		"            Narrower_definition.Link([Narrow_definition], [], null);\n",
		"            Element_owner.Link([], [], Element_ownedElement);\n",
		"    internal sealed class Base : MetamodelElement, IBase\n",
		"    public sealed class Leaf : MetamodelElement, ILeaf\n",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("synthetic output lacks %q", want)
		}
	}
	if strings.Count(source, "Direction? IBase.Direction =>") != 6 {
		t.Error("Base's own class and every concrete subclass of it, the diamond's Leaf included, implement IBase.Direction exactly once")
	}
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(syntheticGolden))
	if *update {
		if err := os.WriteFile(path, generated, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatalf("%s is stale; regenerate with go test -C tools ./gen/csmetamodel -run TestSyntheticGolden -update", syntheticGolden)
	}
}

func TestGeneratorRejectsUnsoundTables(t *testing.T) {
	element := ontology.Class{Name: "Element"}
	namespace := ontology.Class{Name: "Namespace", Parents: []string{"Element"}}
	node := ontology.Class{Name: "Node", Parents: []string{"Element"}}
	child := ontology.Class{Name: "Child", Parents: []string{"Node"}}
	redefining := func(class, name, rangeClass string, many bool, redefines ...string) ontology.Property {
		return ontology.Property{Name: name, DefiningClass: class, Kind: ontology.ObjectProperty,
			Range: sysmlNS + rangeClass, Many: many, Redefines: redefines}
	}
	tests := []struct {
		name       string
		classes    []ontology.Class
		properties []ontology.Property
		enums      []ontology.Enumeration
		want       string
	}{
		{
			name:       "related redeclaration without redefinition",
			classes:    []ontology.Class{element, namespace, node, child},
			properties: []ontology.Property{testProperty("Node", "feature"), testProperty("Child", "feature")},
			want:       "property feature is declared by related metaclasses Node and Child without redefining it",
		},
		{
			name:    "redefinition widening the range",
			classes: []ontology.Class{element, namespace, node, child},
			properties: []ontology.Property{
				redefining("Node", "target", "Node", false),
				redefining("Child", "target", "Element", false, "Node::target"),
			},
			want: "Child::target redefines Node::target with a range or multiplicity that does not conform",
		},
		{
			name:    "redefinition changing multiplicity",
			classes: []ontology.Class{element, namespace, node, child},
			properties: []ontology.Property{
				redefining("Node", "target", "Node", false),
				redefining("Child", "target", "Node", true, "Node::target"),
			},
			want: "with a range or multiplicity that does not conform",
		},
		{
			name:    "redefinition changing a datatype",
			classes: []ontology.Class{element, namespace, node, child},
			properties: []ontology.Property{
				testProperty("Node", "label"),
				{Name: "label", DefiningClass: "Child", Kind: ontology.DatatypeProperty, Range: xsdNS + "int", Redefines: []string{"Node::label"}},
			},
			want: "Child::label redefines Node::label with a range or multiplicity that does not conform",
		},
		{
			name: "unrelated declarations meeting",
			classes: []ontology.Class{
				element, namespace,
				{Name: "Left", Parents: []string{"Element"}},
				{Name: "Right", Parents: []string{"Element"}},
				{Name: "Leaf", Parents: []string{"Left", "Right"}},
			},
			properties: []ontology.Property{testProperty("Left", "label"), testProperty("Right", "label")},
			want:       "Leaf inherits label from unrelated metaclasses Left and Right",
		},
		{
			name:       "keys differing only in case",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "xValue"), testProperty("Node", "XValue")},
			want:       "Node has property keys XValue and xValue with the same C# name XValue",
		},
		{
			name:       "keys meeting in case across generations",
			classes:    []ontology.Class{element, namespace, node, child},
			properties: []ontology.Property{testProperty("Node", "xValue"), testProperty("Child", "XValue")},
			want:       "Child has property keys XValue and xValue with the same C# name XValue",
		},
		{
			name:       "runtime member",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "metaclassInfo")},
			want:       "Node::metaclassInfo collides with runtime member MetaclassInfo",
		},
		{
			name:       "runtime method",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "getValue")},
			want:       "Node::getValue collides with runtime member GetValue",
		},
		{
			name:       "runtime extensions",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "extensions")},
			want:       "Node::extensions collides with runtime member Extensions",
		},
		{
			name:       "object member",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "toString")},
			want:       "Node::toString collides with runtime member ToString",
		},
		{
			name:       "interface's own name",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "iNode")},
			want:       "Node::iNode takes the name of its interface INode",
		},
		{
			name:       "key that is no identifier",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{testProperty("Node", "owned-feature")},
			want:       "Node::owned-feature does not produce a C# identifier",
		},
		{
			name:       "unsupported range",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{{Name: "when", DefiningClass: "Node", Kind: ontology.DatatypeProperty, Range: xsdNS + "dateTime"}},
			want:       "Node::when has unsupported range",
		},
		{
			name:       "dangling redefinition",
			classes:    []ontology.Class{element, namespace, node},
			properties: []ontology.Property{redefining("Node", "target", "Node", false, "Gone::target")},
			want:       "Node::target names Gone::target, which no metaclass declares",
		},
		{
			name:       "undeclared owner",
			classes:    []ontology.Class{element, namespace},
			properties: []ontology.Property{testProperty("Ghost", "label")},
			want:       "Ghost::label has no declaring metaclass",
		},
		{
			name:    "lower-case metaclass",
			classes: []ontology.Class{element, namespace, {Name: "node", Parents: []string{"Element"}}},
			want:    `metaclass name "node" is not a PascalCase C# identifier`,
		},
		{
			name:    "underscored metaclass",
			classes: []ontology.Class{element, namespace, {Name: "Part_Usage", Parents: []string{"Element"}}},
			want:    `metaclass name "Part_Usage" is not a PascalCase C# identifier`,
		},
		{
			name:    "element class hiding an interface",
			classes: []ontology.Class{element, namespace, node, {Name: "INode", Parents: []string{"Element"}}},
			want:    "the element class of metaclass INode would hide the interface of metaclass Node",
		},
		{
			name:    "element class hiding a runtime type",
			classes: []ontology.Class{element, namespace, {Name: "Metaclasses", Parents: []string{"Element"}}},
			want:    "the element class of metaclass Metaclasses would hide the runtime type Metaclasses",
		},
		{
			name:    "abstract metaclass's element class hiding a runtime type",
			classes: []ontology.Class{element, namespace, {Name: "MetamodelGraph", Abstract: true, Parents: []string{"Element"}}},
			want:    "the element class of metaclass MetamodelGraph would hide the runtime type MetamodelGraph",
		},
		{
			name:    "registry member",
			classes: []ontology.Class{element, namespace, {Name: "All", Parents: []string{"Element"}}},
			want:    "metaclass All collides with the registry member Metaclasses.All",
		},
		{
			name:    "metaclass and enumeration sharing a name",
			classes: []ontology.Class{element, namespace, node},
			enums:   []ontology.Enumeration{{Name: "INode", Literals: []string{"a"}}},
			want:    "enumeration INode and the interface of metaclass Node share the generated name INode",
		},
		{
			name:    "enumeration shadowing a runtime type",
			classes: []ontology.Class{element, namespace},
			enums:   []ontology.Enumeration{{Name: "MetaProperty", Literals: []string{"a"}}},
			want:    "the runtime type MetaProperty and enumeration MetaProperty share the generated name MetaProperty",
		},
		{
			name:    "literals meeting in case",
			classes: []ontology.Class{element, namespace},
			enums:   []ontology.Enumeration{{Name: "Direction", Literals: []string{"in", "In"}}},
			want:    `enum Direction has literals "in" and "In" with the same C# member In`,
		},
		{
			name:    "literal that is no identifier",
			classes: []ontology.Class{element, namespace},
			enums:   []ontology.Enumeration{{Name: "Direction", Literals: []string{"bad-literal"}}},
			want:    `enum literal "bad-literal" in Direction does not produce a C# identifier`,
		},
		{
			name:    "literal taking the enumeration's name",
			classes: []ontology.Class{element, namespace},
			enums:   []ontology.Enumeration{{Name: "Kind", Literals: []string{"kind"}}},
			want:    `enum literal "kind" in Kind takes the enumeration's own name`,
		},
		{
			name:    "empty enumeration",
			classes: []ontology.Class{element, namespace},
			enums:   []ontology.Enumeration{{Name: "Direction"}},
			want:    "enumeration Direction has no literals",
		},
		{
			name:    "missing Element",
			classes: []ontology.Class{node},
			want:    "metaclass Element is not declared; the runtime names it",
		},
		{
			name:    "missing Namespace",
			classes: []ontology.Class{element, node},
			want:    "metaclass Namespace is not declared; the runtime names it",
		},
		{
			name:    "repeated metaclass",
			classes: []ontology.Class{element, namespace, node, node},
			want:    "metaclass Node is declared more than once",
		},
		{
			name:    "unknown parent",
			classes: []ontology.Class{element, namespace, {Name: "Node", Parents: []string{"Missing"}}},
			want:    "Node has unknown metaclass parent Missing",
		},
		{
			name: "inconsistent C3 order",
			classes: []ontology.Class{
				element, namespace,
				{Name: "O", Parents: []string{"Element"}},
				{Name: "X", Parents: []string{"O"}},
				{Name: "Y", Parents: []string{"O"}},
				{Name: "A", Parents: []string{"X", "Y"}},
				{Name: "B", Parents: []string{"Y", "X"}},
				{Name: "Z", Parents: []string{"A", "B"}},
			},
			want: "C3 linearization failed for Z",
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

func TestGeneratorRejectsFrameworkTypeNames(t *testing.T) {
	classes := []ontology.Class{{Name: "Element", Abstract: true}, {Name: "Namespace", Parents: []string{"Element"}}}
	for _, name := range []string{
		"ArgumentException", "ArgumentNullException", "ArgumentOutOfRangeException", "Array",
		"ArrayBufferWriter", "BindingFlags", "ConcurrentDictionary", "Dictionary", "Enum", "Exception", "File",
		"FrozenDictionary", "FrozenSet", "Func", "HashSet", "IEnumerable", "IEnumerator",
		"IReadOnlyCollection", "IReadOnlyDictionary", "InvalidOperationException", "JsonDocument",
		"JsonDocumentOptions", "JsonElement", "JsonException", "JsonMarshal", "JsonProperty", "JsonValueKind",
		"KeyNotFoundException", "KeyValuePair", "List", "NotNullWhen", "NotNullWhenAttribute",
		"ObjectDisposedException", "ReadOnlyDictionary", "Stack", "Stream", "StringComparer", "Type",
		"Utf8", "Utf8JsonReader", "Utf8JsonWriter",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := generate(classes, nil, []ontology.Enumeration{{Name: name, Literals: []string{"value"}}}, "v", "t", "c")
			want := "enumeration " + name + " would hide framework type " + name
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("generate error = %v, want substring %q", err, want)
			}
		})
	}
	t.Run("interface", func(t *testing.T) {
		_, err := generate(append(classes, ontology.Class{Name: "Enumerable", Parents: []string{"Element"}}), nil, nil, "v", "t", "c")
		if err == nil || !strings.Contains(err.Error(), "the interface of metaclass Enumerable would hide framework type IEnumerable") {
			t.Fatalf("generate error = %v, want a framework type collision", err)
		}
	})
}

func TestGeneratorRejectsInheritedRegistryMembers(t *testing.T) {
	for _, name := range []string{"Equals", "Finalize", "GetHashCode", "GetType", "MemberwiseClone", "ReferenceEquals", "ToString"} {
		t.Run(name, func(t *testing.T) {
			classes := []ontology.Class{{Name: "Element", Abstract: true}, {Name: "Namespace", Parents: []string{"Element"}}, {Name: name, Parents: []string{"Element"}}}
			_, err := generate(classes, nil, nil, "v", "t", "c")
			if err == nil || !strings.Contains(err.Error(), "collides with the registry member Metaclasses."+name) {
				t.Fatalf("generate error = %v, want a registry member collision", err)
			}
		})
	}
}

func TestPropertyKindsMatchTheirRanges(t *testing.T) {
	classes := []ontology.Class{{Name: "Element", Abstract: true}, {Name: "Namespace", Parents: []string{"Element"}}, {Name: "Node", Parents: []string{"Element"}}}
	enums := []ontology.Enumeration{{Name: "Direction", Literals: []string{"in", "out"}}}
	for _, rangeIRI := range []string{xsdNS + "boolean", xsdNS + "string", xsdNS + "int", owlNS + "real", sysmlNS + "Direction", sysmlNS + "Element"} {
		for _, kind := range []ontology.PropertyKind{ontology.ObjectProperty, ontology.DatatypeProperty, -1, 2} {
			for _, many := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/kind=%d/many=%t", rangeIRI, kind, many), func(t *testing.T) {
					property := ontology.Property{Name: "value", DefiningClass: "Node", Kind: kind, Range: rangeIRI, Many: many}
					_, err := generate(classes, []ontology.Property{property}, enums, "v", "t", "c")
					valid := kind == ontology.ObjectProperty && rangeIRI == sysmlNS+"Element" ||
						kind == ontology.DatatypeProperty && rangeIRI != sysmlNS+"Element"
					if valid {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					want := "Node::value has unsupported range"
					if kind != ontology.ObjectProperty && kind != ontology.DatatypeProperty {
						want = fmt.Sprintf("Node::value has unsupported property kind %d", kind)
					}
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("generate error = %v, want substring %q", err, want)
					}
				})
			}
		}
	}
}

func TestGenerationIsDeterministic(t *testing.T) {
	classes, properties, enums := syntheticTable()
	first, err := generate(classes, properties, enums, "v", "t", "c")
	if err != nil {
		t.Fatal(err)
	}
	reversed := func(n int, swap func(i, j int)) {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			swap(i, j)
		}
	}
	reversed(len(classes), func(i, j int) { classes[i], classes[j] = classes[j], classes[i] })
	reversed(len(properties), func(i, j int) { properties[i], properties[j] = properties[j], properties[i] })
	second, err := generate(classes, properties, enums, "v", "t", "c")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("the output depends on the order of the table")
	}
}

func TestPascal(t *testing.T) {
	for input, want := range map[string]string{
		"ownedFeature": "OwnedFeature",
		"isAbstract":   "IsAbstract",
		"in":           "In",
		"inout":        "Inout",
		"x":            "X",
		"XMLValue":     "XMLValue",
	} {
		if got := pascal(input); got != want {
			t.Errorf("pascal(%q) = %q, want %q", input, got, want)
		}
	}
}

func testProperty(class, name string) ontology.Property {
	return ontology.Property{
		Name: name, DefiningClass: class, Kind: ontology.DatatypeProperty,
		Range: xsdNS + "string",
	}
}

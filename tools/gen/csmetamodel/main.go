// Command csmetamodel writes the generated C# view of the SysML metamodel: an
// interface per metaclass, a sealed element class per concrete metaclass (and
// an internal one per abstract metaclass, for elements a document types as it),
// an enum per enumeration and the Metaclasses registry describing them.
//
//	go run -C tools ./gen/csmetamodel [-check] [-out FILE]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/tools/gen/hierarchy"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

const (
	outputPath    = "client/dotnet/src/OpenMBEE.OpenSysML.Metamodel/Generated.cs"
	regenerate    = "make dotnet-metamodel"
	rootNamespace = "OpenMBEE.OpenSysML.Metamodel"
	elementsName  = "Elements"
	sysmlNS       = "https://www.omg.org/spec/SysML#"
	xsdNS         = "http://www.w3.org/2001/XMLSchema#"
	owlNS         = "http://www.w3.org/2002/07/owl#"
)

var objectMembers = []string{
	"Equals", "Finalize", "GetHashCode", "GetType", "MemberwiseClone", "ReferenceEquals", "ToString",
}

// runtimeMembers are the names a generated property may not take: the members
// of IMetamodelElement, which every root interface extends, and of object.
var runtimeMembers = append([]string{"Id", "JsonType", "MetaclassInfo", "GetValue", "Extensions"}, objectMembers...)

// runtimeMetaclasses are the metaclasses the handwritten runtime names: the
// reader reads an unrecognized @type as Element, and its roots are Namespaces.
var runtimeMetaclasses = []string{"Element", "Namespace"}

// runtimeTypes are the handwritten types of the root namespace, and the
// framework types the generated bodies name without qualification.
var runtimeTypes = []string{
	"EnumerationInfo", "GraphReader", "IMetamodelElement", "MalformedDocumentException",
	"MalformedValueException", "MetaProperty", "MetaclassInfo", "Metaclasses", "MetamodelElement",
	"MetamodelException", "MetamodelGraph", "NotSuppliedException", "PropertyRangeKind", "ReadOptions",
	"TypedList", "UnknownJsonKeyException", "UnresolvedReferenceException",
	"IReadOnlyList", elementsName,
}

// frameworkTypes may not be declared in the root namespace, where they would
// hide the framework types the handwritten runtime names without qualification.
var frameworkTypes = []string{
	"ArgumentException", "ArgumentNullException", "ArgumentOutOfRangeException", "Array",
	"ArrayBufferWriter", "BindingFlags", "ConcurrentDictionary", "Dictionary", "Enum", "Exception", "File",
	"FrozenDictionary", "FrozenSet", "Func", "HashSet", "IEnumerable", "IEnumerator",
	"IReadOnlyCollection", "IReadOnlyDictionary", "InvalidOperationException", "JsonDocument",
	"JsonDocumentOptions", "JsonElement", "JsonException", "JsonMarshal", "JsonProperty", "JsonValueKind",
	"KeyNotFoundException", "KeyValuePair", "List", "NotNullWhen", "NotNullWhenAttribute",
	"ObjectDisposedException", "ReadOnlyDictionary", "Stack", "Stream", "StringComparer", "Type",
	"Utf8", "Utf8JsonReader", "Utf8JsonWriter",
}

// registryMembers are the members of the Metaclasses class besides its one
// static property per metaclass, which no metaclass may share a name with.
var registryMembers = append([]string{
	"All", "Enumerations", "Of", "Register", "TryGet", "TryGetEnumeration",
	"MetamodelVersion", "SourceTag", "SourceCommit",
}, objectMembers...)

// typeName is a metaclass or enumeration name the generated code can use as a
// C# type name as is: PascalCase ASCII, so never a C# keyword, and without the
// underscore that separates the registry's internal field names.
var typeName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// memberKey is a JSON key or literal whose PascalCase spelling is a C# identifier.
var memberKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

func main() {
	out := flag.String("out", outputPath, "generated C# file (default "+outputPath+")")
	check := flag.Bool("check", false, "fail if the generated file differs from the output")
	flag.Parse()
	root, err := repo.Root()
	if err != nil {
		fail(err)
	}
	*out = repo.Resolve(root, *out)
	if *out == "" {
		*out = filepath.Join(root, filepath.FromSlash(outputPath))
	}
	data, err := generate(ontology.Classes(), ontology.Properties(), ontology.Enumerations(),
		ontology.Version, ontology.SourceTag, ontology.SourceCommit)
	if err != nil {
		fail(err)
	}
	summary := fmt.Sprintf("%d metaclasses, %d properties, %d enumerations (metamodel %s, pilot %s)",
		len(ontology.Classes()), len(ontology.Properties()), len(ontology.Enumerations()),
		ontology.Version, ontology.SourceTag)
	if *check {
		have, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(have, data) {
			fail(fmt.Errorf("%s is stale; regenerate with `%s`", *out, regenerate))
		}
		fmt.Printf("%s is current: %s\n", *out, summary)
		return
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s: %s\n", *out, summary)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "csmetamodel:", err)
	os.Exit(1)
}

type model struct {
	classes  []ontology.Class // parents first
	enums    []ontology.Enumeration
	classBy  map[string]ontology.Class
	enumBy   map[string]ontology.Enumeration
	h        *hierarchy.Hierarchy
	declared map[string][]ontology.Property // by declaring metaclass, ordered by key
	// lineage holds, per metaclass and JSON key, every declaration of that key
	// the metaclass inherits or makes, most specific first.
	lineage map[string]map[string][]ontology.Property
}

func generate(classes []ontology.Class, properties []ontology.Property, enums []ontology.Enumeration,
	version, tag, commit string) ([]byte, error) {
	m, err := newModel(classes, properties, enums)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `// <auto-generated>
//     Code generated by tools/gen/csmetamodel. DO NOT EDIT.
//     Metamodel: %s
//     Pilot tag: %s
//     Pilot commit: %s
//     %d metaclasses, %d properties, %d enumerations
//     Regenerate with: %s
// </auto-generated>
#nullable enable

using System.Collections.Generic;

namespace %s
{
`, version, tag, commit, len(classes), len(properties), len(enums), regenerate, rootNamespace)
	for _, enum := range m.enums {
		writeEnum(&b, enum)
	}
	for _, class := range m.sortedClasses() {
		if err := m.writeInterface(&b, class); err != nil {
			return nil, err
		}
	}
	if err := m.writeRegistry(&b, version, tag, commit); err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "}\n\nnamespace %s.%s\n{\n", rootNamespace, elementsName)
	for i, class := range m.sortedClasses() {
		if i > 0 {
			b.WriteString("\n")
		}
		if err := m.writeElementClass(&b, class); err != nil {
			return nil, err
		}
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

func writeEnum(b *strings.Builder, enum ontology.Enumeration) {
	fmt.Fprintf(b, "    /// <summary>The metamodel enumeration <c>%s</c>; <see cref=\"EnumerationInfo\"/> maps its members to the literals a document writes.</summary>\n", enum.Name)
	fmt.Fprintf(b, "    public enum %s\n    {\n", enum.Name)
	for i, literal := range enum.Literals {
		fmt.Fprintf(b, "        /// <summary>The literal <c>%s</c>.</summary>\n", literal)
		fmt.Fprintf(b, "        %s = %d,\n", pascal(literal), i)
		if i < len(enum.Literals)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("    }\n\n")
}

func (m *model) writeInterface(b *strings.Builder, class ontology.Class) error {
	kind := "Metaclass"
	if class.Abstract {
		kind = "Abstract metaclass"
	}
	summary := fmt.Sprintf("%s <c>%s</c>", kind, class.Name)
	if len(class.Parents) > 0 {
		names := make([]string, len(class.Parents))
		for i, parent := range class.Parents {
			names[i] = "<c>" + parent + "</c>"
		}
		summary += ", specializing " + strings.Join(names, ", ")
	}
	fmt.Fprintf(b, "    /// <summary>%s.</summary>\n", summary)
	bases := []string{"IMetamodelElement"}
	if len(class.Parents) > 0 {
		bases = bases[:0]
		for _, parent := range class.Parents {
			bases = append(bases, interfaceName(parent))
		}
	}
	fmt.Fprintf(b, "    public interface %s : %s\n    {\n", interfaceName(class.Name), strings.Join(bases, ", "))
	for i, property := range m.declared[class.Name] {
		if i > 0 {
			b.WriteString("\n")
		}
		clrType, err := m.clrType(property)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "        /// <summary>%s</summary>\n", propertyDoc(property))
		fmt.Fprintf(b, "        /// <exception cref=\"NotSuppliedException\">The document does not supply <c>%s</c> for the element.</exception>\n", property.Name)
		modifier := ""
		if len(m.lineage[class.Name][property.Name]) > 1 {
			modifier = "new "
		}
		fmt.Fprintf(b, "        %s%s %s { get; }\n", modifier, clrType, pascal(property.Name))
	}
	b.WriteString("    }\n\n")
	return nil
}

func propertyDoc(property ontology.Property) string {
	parts := []string{"single"}
	if property.Many {
		parts = []string{"many"}
		if property.Ordered {
			parts = append(parts, "ordered")
		}
		if property.Nonunique {
			parts = append(parts, "nonunique")
		}
	}
	if property.Derived {
		parts = append(parts, "derived")
	}
	doc := fmt.Sprintf("<c>%s</c> (%s), JSON key <c>%s</c>.", property.QualifiedName(),
		strings.Join(parts, ", "), property.Name)
	if len(property.Redefines) > 0 {
		targets := make([]string, len(property.Redefines))
		for i, redefined := range property.Redefines {
			targets[i] = "<c>" + redefined + "</c>"
		}
		doc += " Redefines " + strings.Join(targets, ", ") + "."
	}
	return doc
}

func (m *model) writeRegistry(b *strings.Builder, version, tag, commit string) error {
	fmt.Fprintf(b, `    public static partial class Metaclasses
    {
        /// <summary>The version of the metamodel the types were generated from.</summary>
        public const string MetamodelVersion = %q;

        /// <summary>The tag of the pilot implementation release whose <c>SysML.ecore</c> the metamodel table was generated from.</summary>
        public const string SourceTag = %q;

        /// <summary>The commit of that release.</summary>
        public const string SourceCommit = %q;
`, version, tag, commit)
	for _, class := range m.sortedClasses() {
		qualified := "global::" + rootNamespace + "." + elementsName + "." + class.Name
		elementType, factory := "null", "static (id, jsonType) => new "+qualified+"(id, jsonType)"
		if !class.Abstract {
			elementType = "typeof(" + qualified + ")"
		}
		fmt.Fprintf(b, "\n        /// <summary>Metaclass <c>%s</c>.</summary>\n", class.Name)
		fmt.Fprintf(b, "        public static MetaclassInfo %s { get; } = new(%q, %t, typeof(%s), %s, %s);\n",
			class.Name, class.Name, class.Abstract, interfaceName(class.Name), elementType, factory)
	}
	b.WriteString("\n")
	for _, enum := range m.enums {
		literals := make([]string, len(enum.Literals))
		for i, literal := range enum.Literals {
			literals[i] = fmt.Sprintf("%q", literal)
		}
		fmt.Fprintf(b, "        private static readonly EnumerationInfo %s = new(%q, typeof(%s), [%s]);\n",
			enumerationField(enum.Name), enum.Name, enum.Name, strings.Join(literals, ", "))
	}
	b.WriteString("\n")
	for _, class := range m.sortedClasses() {
		for _, property := range m.declared[class.Name] {
			b.WriteString(m.propertyDeclaration(property))
		}
	}
	b.WriteString("\n        static Metaclasses()\n        {\n")
	for _, class := range m.classes {
		fmt.Fprintf(b, "            %s.Initialize([%s], [%s], [%s]);\n", class.Name,
			strings.Join(class.Parents, ", "), strings.Join(m.h.MRO[class.Name][1:], ", "),
			strings.Join(declaredFields(m.declared[class.Name]), ", "))
	}
	for _, class := range m.sortedClasses() {
		for _, property := range m.declared[class.Name] {
			opposite := "null"
			if property.Opposite != "" {
				opposite = propertyField(property.Opposite)
			}
			fmt.Fprintf(b, "            %s.Link([%s], [%s], %s);\n", propertyField(property.QualifiedName()),
				strings.Join(qualifiedFields(property.Redefines), ", "),
				strings.Join(qualifiedFields(property.Subsets), ", "), opposite)
		}
	}
	names := make([]string, 0, len(m.classes))
	for _, class := range m.sortedClasses() {
		names = append(names, class.Name)
	}
	enumerations := make([]string, 0, len(m.enums))
	for _, enum := range m.enums {
		enumerations = append(enumerations, enumerationField(enum.Name))
	}
	fmt.Fprintf(b, "            Register(\n                [%s],\n                [%s]);\n        }\n    }\n",
		strings.Join(names, ", "), strings.Join(enumerations, ", "))
	return nil
}

// propertyDeclaration is the registry's internal field holding property's MetaProperty.
func (m *model) propertyDeclaration(property ontology.Property) string {
	kind, rangeClass, rangeEnum, primitive := "Primitive", "null", "null", "null"
	local, inNamespace := strings.CutPrefix(property.Range, sysmlNS)
	switch _, isClass := m.classBy[local]; {
	case inNamespace && isClass:
		kind, rangeClass = "Metaclass", local
	case inNamespace:
		kind, rangeEnum = "Enumeration", enumerationField(local)
	default:
		valueType, _ := m.valueType(property)
		primitive = "typeof(" + valueType + ")"
	}
	defaultLiteral := "null"
	if property.HasDefault {
		defaultLiteral = fmt.Sprintf("%q", property.Default)
	}
	return fmt.Sprintf("        internal static readonly MetaProperty %s = new(%s, %q, %q, PropertyRangeKind.%s, %s, %s, %s, isMany: %t, isOrdered: %t, isUnique: %t, isDerived: %t, lowerBound: %d, defaultLiteral: %s);\n",
		propertyField(property.QualifiedName()), property.DefiningClass, property.Name, pascal(property.Name),
		kind, rangeClass, rangeEnum, primitive, property.Many, property.Ordered, !property.Nonunique, property.Derived,
		property.Lower, defaultLiteral)
}

func declaredFields(properties []ontology.Property) []string {
	fields := make([]string, len(properties))
	for i, property := range properties {
		fields[i] = propertyField(property.QualifiedName())
	}
	return fields
}

func qualifiedFields(qualified []string) []string {
	fields := make([]string, len(qualified))
	for i, name := range qualified {
		fields[i] = propertyField(name)
	}
	return fields
}

// propertyField is the registry's internal field for "Class::key": Class_key,
// unique because no metaclass name holds an underscore.
func propertyField(qualified string) string {
	return strings.Replace(qualified, "::", "_", 1)
}

func enumerationField(name string) string { return "Enumeration_" + name }

// writeElementClass writes the class of class's elements: public for a concrete
// metaclass; internal for an abstract one, whose elements are those a document
// types as the metaclass itself or as a type the reader maps to it.
func (m *model) writeElementClass(b *strings.Builder, class ontology.Class) error {
	access := "public"
	if class.Abstract {
		access = "internal"
		fmt.Fprintf(b, "    /// <summary>An element read as abstract metaclass <c>%s</c>, which its document types as <c>%s</c> or as a type the reader maps to it. Its properties are read through <see cref=\"%s\"/>, which it implements explicitly.</summary>\n",
			class.Name, class.Name, interfaceName(class.Name))
	} else {
		fmt.Fprintf(b, "    /// <summary>An element of exactly metaclass <c>%s</c>. Its properties are read through <see cref=\"%s\"/>, which it implements explicitly.</summary>\n",
			class.Name, interfaceName(class.Name))
	}
	fmt.Fprintf(b, "    %s sealed class %s : MetamodelElement, %s\n    {\n", access, class.Name, interfaceName(class.Name))
	fmt.Fprintf(b, "        internal %s(string id, string jsonType)\n            : base(Metaclasses.%s, id, jsonType)\n        {\n        }\n",
		class.Name, class.Name)
	var members []ontology.Property
	for _, declarations := range m.lineage[class.Name] {
		members = append(members, declarations...)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].QualifiedName() < members[j].QualifiedName() })
	if len(members) > 0 {
		b.WriteString("\n")
	}
	for _, member := range members {
		effective := m.lineage[class.Name][member.Name][0]
		memberType, err := m.clrType(member)
		if err != nil {
			return err
		}
		read, err := m.read(effective)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "        %s %s.%s => %s;\n", memberType, interfaceName(member.DefiningClass), pascal(member.Name), read)
	}
	b.WriteString("    }\n")
	return nil
}

// read is the expression that reads property, the declaration effective for
// the element's metaclass, from the element's supplied values.
func (m *model) read(property ontology.Property) (string, error) {
	valueType, err := m.valueType(property)
	if err != nil {
		return "", err
	}
	field := "Metaclasses." + propertyField(property.QualifiedName())
	switch {
	case property.Many:
		return fmt.Sprintf("ReadList<%s>(%s)", valueType, field), nil
	case valueType == "string":
		return fmt.Sprintf("ReadText(%s)", field), nil
	case property.Kind == ontology.ObjectProperty:
		return fmt.Sprintf("ReadReference<%s>(%s)", valueType, field), nil
	default:
		return fmt.Sprintf("ReadValue<%s>(%s)", valueType, field), nil
	}
}

func (m *model) clrType(property ontology.Property) (string, error) {
	valueType, err := m.valueType(property)
	if err != nil {
		return "", err
	}
	if property.Many {
		return "IReadOnlyList<" + valueType + ">", nil
	}
	return valueType + "?", nil
}

// valueType is the C# type of one value of property.
func (m *model) valueType(property ontology.Property) (string, error) {
	if property.Kind != ontology.ObjectProperty && property.Kind != ontology.DatatypeProperty {
		return "", fmt.Errorf("%s has unsupported property kind %d", property.QualifiedName(), property.Kind)
	}
	if property.Kind == ontology.DatatypeProperty {
		switch property.Range {
		case xsdNS + "boolean":
			return "bool", nil
		case xsdNS + "string":
			return "string", nil
		case xsdNS + "int":
			return "int", nil
		case owlNS + "real":
			return "double", nil
		}
	}
	if local, ok := strings.CutPrefix(property.Range, sysmlNS); ok {
		if _, exists := m.classBy[local]; exists && property.Kind == ontology.ObjectProperty {
			return interfaceName(local), nil
		}
		if _, exists := m.enumBy[local]; exists && property.Kind == ontology.DatatypeProperty {
			return local, nil
		}
	}
	return "", fmt.Errorf("%s has unsupported range %q for %s", property.QualifiedName(), property.Range, property.Kind)
}

func (m *model) sortedClasses() []ontology.Class {
	sorted := slices.Clone(m.classes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

func newModel(classes []ontology.Class, properties []ontology.Property,
	enums []ontology.Enumeration) (*model, error) {
	m := &model{
		classBy:  make(map[string]ontology.Class, len(classes)),
		enumBy:   make(map[string]ontology.Enumeration, len(enums)),
		declared: make(map[string][]ontology.Property, len(classes)),
		lineage:  make(map[string]map[string][]ontology.Property, len(classes)),
	}
	if err := m.indexTypes(classes, enums); err != nil {
		return nil, err
	}
	h, err := hierarchy.New(classes)
	if err != nil {
		return nil, err
	}
	m.h, m.classes = h, h.Order
	if err := m.indexProperties(properties); err != nil {
		return nil, err
	}
	if err := m.computeLineage(); err != nil {
		return nil, err
	}
	return m, nil
}

// indexTypes validates every name the generated types take, in either namespace.
func (m *model) indexTypes(classes []ontology.Class, enums []ontology.Enumeration) error {
	for _, class := range classes {
		if !typeName.MatchString(class.Name) {
			return fmt.Errorf("metaclass name %q is not a PascalCase C# identifier", class.Name)
		}
		if _, duplicate := m.classBy[class.Name]; duplicate {
			return fmt.Errorf("metaclass %s is declared more than once", class.Name)
		}
		m.classBy[class.Name] = class
	}
	for _, name := range runtimeMetaclasses {
		if _, ok := m.classBy[name]; !ok {
			return fmt.Errorf("metaclass %s is not declared; the runtime names it", name)
		}
	}
	m.enums = slices.Clone(enums)
	sort.Slice(m.enums, func(i, j int) bool { return m.enums[i].Name < m.enums[j].Name })
	for _, enum := range m.enums {
		if !typeName.MatchString(enum.Name) {
			return fmt.Errorf("enumeration name %q is not a PascalCase C# identifier", enum.Name)
		}
		if len(enum.Literals) == 0 {
			return fmt.Errorf("enumeration %s has no literals", enum.Name)
		}
		if _, duplicate := m.enumBy[enum.Name]; duplicate {
			return fmt.Errorf("enumeration %s is declared more than once", enum.Name)
		}
		m.enumBy[enum.Name] = enum
		members := map[string]string{}
		for _, literal := range enum.Literals {
			if !memberKey.MatchString(literal) {
				return fmt.Errorf("enum literal %q in %s does not produce a C# identifier", literal, enum.Name)
			}
			member := pascal(literal)
			if previous, exists := members[member]; exists {
				return fmt.Errorf("enum %s has literals %q and %q with the same C# member %s", enum.Name, previous, literal, member)
			}
			if member == enum.Name {
				return fmt.Errorf("enum literal %q in %s takes the enumeration's own name", literal, enum.Name)
			}
			members[member] = literal
		}
	}
	// Interfaces, enums and the runtime share the root namespace; element
	// classes, which see all of them, the Elements namespace.
	owners := map[string]string{}
	for _, name := range runtimeTypes {
		owners[name] = "the runtime type " + name
	}
	claim := func(name, owner string) error {
		if slices.Contains(frameworkTypes, name) {
			return fmt.Errorf("%s would hide framework type %s", owner, name)
		}
		if previous, taken := owners[name]; taken {
			return fmt.Errorf("%s and %s share the generated name %s", previous, owner, name)
		}
		owners[name] = owner
		return nil
	}
	for _, enum := range m.enums {
		if err := claim(enum.Name, "enumeration "+enum.Name); err != nil {
			return err
		}
	}
	for _, class := range classes {
		if err := claim(interfaceName(class.Name), "the interface of metaclass "+class.Name); err != nil {
			return err
		}
	}
	for _, class := range classes {
		if previous, taken := owners[class.Name]; taken {
			return fmt.Errorf("the element class of metaclass %s would hide %s", class.Name, previous)
		}
		if slices.Contains(registryMembers, class.Name) {
			return fmt.Errorf("metaclass %s collides with the registry member Metaclasses.%s", class.Name, class.Name)
		}
	}
	return nil
}

// indexProperties validates every property's key, range and C# name.
func (m *model) indexProperties(properties []ontology.Property) error {
	qualified := make(map[string]ontology.Property, len(properties))
	for _, property := range properties {
		if _, ok := m.classBy[property.DefiningClass]; !ok {
			return fmt.Errorf("%s has no declaring metaclass", property.QualifiedName())
		}
		if !memberKey.MatchString(property.Name) {
			return fmt.Errorf("%s does not produce a C# identifier", property.QualifiedName())
		}
		if _, duplicate := qualified[property.QualifiedName()]; duplicate {
			return fmt.Errorf("%s is declared more than once", property.QualifiedName())
		}
		qualified[property.QualifiedName()] = property
		name := pascal(property.Name)
		if slices.Contains(runtimeMembers, name) {
			return fmt.Errorf("%s collides with runtime member %s", property.QualifiedName(), name)
		}
		// A type position never resolves to a property, so only the enclosing interface's own name is barred.
		if name == interfaceName(property.DefiningClass) {
			return fmt.Errorf("%s takes the name of its interface %s", property.QualifiedName(), name)
		}
		if _, err := m.valueType(property); err != nil {
			return err
		}
		m.declared[property.DefiningClass] = append(m.declared[property.DefiningClass], property)
	}
	for _, property := range properties {
		names := append(slices.Clone(property.Redefines), property.Subsets...)
		if property.Opposite != "" {
			names = append(names, property.Opposite)
		}
		for _, name := range names {
			if _, ok := qualified[name]; !ok {
				return fmt.Errorf("%s names %s, which no metaclass declares", property.QualifiedName(), name)
			}
		}
	}
	for class := range m.declared {
		sort.Slice(m.declared[class], func(i, j int) bool {
			return m.declared[class][i].Name < m.declared[class][j].Name
		})
	}
	return nil
}

// computeLineage records, per metaclass, each key's declarations along its
// linearization, refusing declarations a C# interface cannot carry soundly:
// one key declared by unrelated metaclasses that meet in a common subclass,
// one redeclared by a subclass without a conforming redefinition, or two keys
// with one C# name.
func (m *model) computeLineage() error {
	for _, class := range m.classes {
		lineage := map[string][]ontology.Property{}
		names := map[string]string{}
		for _, ancestor := range m.h.MRO[class.Name] {
			for _, property := range m.declared[ancestor] {
				if previous, taken := names[pascal(property.Name)]; taken && previous != property.Name {
					return fmt.Errorf("%s has property keys %s and %s with the same C# name %s",
						class.Name, previous, property.Name, pascal(property.Name))
				}
				names[pascal(property.Name)] = property.Name
				lineage[property.Name] = append(lineage[property.Name], property)
			}
		}
		for _, declarations := range lineage {
			for i := 1; i < len(declarations); i++ {
				if err := m.conformingRedefinition(class.Name, declarations[i-1], declarations[i]); err != nil {
					return err
				}
			}
		}
		m.lineage[class.Name] = lineage
	}
	return nil
}

// conformingRedefinition checks that specific, which precedes general in a
// linearization, redeclares it soundly: from a subclass, naming it as
// redefined, with the same multiplicity kind and a conforming range.
func (m *model) conformingRedefinition(class string, specific, general ontology.Property) error {
	if !m.h.IsAncestorOrSelf(specific.DefiningClass, general.DefiningClass) {
		return fmt.Errorf("%s inherits %s from unrelated metaclasses %s and %s",
			class, specific.Name, specific.DefiningClass, general.DefiningClass)
	}
	if !slices.Contains(specific.Redefines, general.QualifiedName()) {
		return fmt.Errorf("property %s is declared by related metaclasses %s and %s without redefining it",
			specific.Name, general.DefiningClass, specific.DefiningClass)
	}
	conforms := specific.Kind == general.Kind && specific.Many == general.Many
	if conforms && specific.Kind == ontology.ObjectProperty {
		narrow, _ := strings.CutPrefix(specific.Range, sysmlNS)
		wide, _ := strings.CutPrefix(general.Range, sysmlNS)
		conforms = m.h.IsAncestorOrSelf(narrow, wide)
	} else if conforms {
		conforms = specific.Range == general.Range
	}
	if !conforms {
		return fmt.Errorf("%s redefines %s with a range or multiplicity that does not conform",
			specific.QualifiedName(), general.QualifiedName())
	}
	return nil
}

func interfaceName(class string) string { return "I" + class }

// pascal upper-cases the first letter of a JSON key or literal.
func pascal(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

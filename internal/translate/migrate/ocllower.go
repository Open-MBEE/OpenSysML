package migrate

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// oclValue is what an OCL node lowers to: a v2 cell expression text of some
// v1 metaclass kind, or a symbolic stand-in for v1 reflection the v2 model
// spells differently (a stereotype instance, its slots, their values).
type oclValue struct {
	text string
	// kind is the v1 metaclass of the elements, or String, Integer, Boolean,
	// EnumerationLiteral; "" when unknown.
	kind string
	// single holds when the value is one element: a bound variable, a head
	// or last, a count.
	single bool
	// variable names the bound variable the value is, "" for any other.
	variable string
	// symbolic reflection: the host elements the reflection is read of, the
	// tag a slot selection named, the stereotype a classifier test named.
	symbol oclSymbol
	host   *oclValue
	tag    string
}

type oclSymbol int

const (
	oclConcrete        oclSymbol = iota
	oclStereotypeInst            // host.appliedStereotypeInstance
	oclSlots                     // host.appliedStereotypeInstance.slot, tag set once selected
	oclSlotValue                 // the value of the selected slots
	oclStereotypes               // host.appliedStereotypeInstance.classifier, tag the name tested
	oclStereotypeCount           // ->size() of the above
)

// oclLowering lowers one expression; row is the cell's row variable, kind its
// v1 metaclass when the chain tells it, "" to lower unchecked.
type oclLowering struct {
	m       *migration
	source  string
	rowKind string
	// prefix qualifies the DocumentQueries operations the expression calls.
	prefix string
	// needs is the most specific v1 metaclass the expression reads each bound
	// variable as, which types the cell's row and the lambda parameters.
	needs map[string]string
	// notes are the approximations the lowering leaned on.
	notes []string
	env   map[string]oclValue
	used  map[string]bool
}

// oclRefusal says why an expression is not lowered, quoting the construct.
type oclRefusal struct{ why string }

func (r *oclRefusal) Error() string { return r.why }

func (l *oclLowering) refuse(n *oclNode, why string) error {
	return &oclRefusal{why: strconv.Quote(n.text) + " " + why}
}

// oclMetaclasses are the v2 metaclasses a cell declares its row and lambda
// variables as, by the v1 metaclass the expression reads them as, most
// specific first; a connector end is a feature of the connection in v2.
var oclMetaclasses = []struct{ v1, v2 string }{
	{"Connector", "KerML::Kernel::Connector"},
	{"ConnectorEnd", "KerML::Core::Feature"},
	{"TypedElement", "KerML::Core::Feature"},
	{"Type", "KerML::Core::Type"},
	{"Namespace", "KerML::Core::Namespace"},
}

// oclMetaclass is the v2 metaclass declared for elements of the v1 metaclass
// kind; Element when kind is unknown or no narrower one stands for it.
func oclMetaclass(kind string) string {
	if kind != "" {
		for _, m := range oclMetaclasses {
			if oclIsA(kind, m.v1) {
				return m.v2
			}
		}
	}
	return "KerML::Root::Element"
}

// oclFunctionPackages are the KerML library packages of the sequence
// functions a cell applies; a lowered expression names them qualified, so the
// model resolves them without an import.
var oclFunctionPackages = map[string]string{
	"size": "SequenceFunctions", "isEmpty": "SequenceFunctions", "notEmpty": "SequenceFunctions",
	"head": "SequenceFunctions", "last": "SequenceFunctions", "includes": "SequenceFunctions",
	"excludes": "SequenceFunctions", "including": "SequenceFunctions", "excluding": "SequenceFunctions",
	"select": "ControlFunctions", "reject": "ControlFunctions", "collect": "ControlFunctions",
	"exists": "ControlFunctions", "forAll": "ControlFunctions",
}

// function spells the `->` function name as the cell applies it: a KerML
// sequence function qualified by its package, or DocumentQueries::Distinct.
func (l *oclLowering) function(name string) string {
	if name == "distinct" {
		return l.prefix + "Distinct"
	}
	return oclFunctionPackages[name] + "::" + name
}

// declare spells a lambda parameter over elements of the v1 metaclass kind,
// typed by it or by the narrower metaclass its body read the variable as.
func (l *oclLowering) declare(name, kind string) string {
	if need := l.needs[name]; need != "" && (kind == "" || oclIsA(need, kind)) {
		kind = need
	}
	delete(l.needs, name)
	return name + " : " + oclMetaclass(kind)
}

// oclNavigation is one v1 metaproperty the v2 model carries: the metaclass
// it belongs to, its v2 feature (or a lowering of its own), and the metaclass
// of what it yields.
type oclNavigation struct {
	meta, prop, feature, result string
}

var oclNavigations = []oclNavigation{
	{"Element", "name", "name", "String"},
	{"Element", "qualifiedName", "qualifiedName", "String"},
	{"Element", "owner", "owner", "Element"},
	{"Element", "ownedElement", "ownedElement", "Element"},
	{"Namespace", "member", "member", "NamedElement"},
	{"Namespace", "ownedMember", "ownedMember", "NamedElement"},
	{"Type", "_typedElementOfType", "", "TypedElement"},
	{"TypedElement", "type", "type", "Type"},
	{"Connector", "end", "connectorEnd", "ConnectorEnd"},
	{"ConnectorEnd", "role", "", "ConnectableElement"},
	{"ConnectorEnd", "partWithPort", "", "Property"},
}

// oclTags are the stereotype tags the v2 model carries structurally, by the
// v1 metaclass of their host: the nested connector end's path is the connect
// expression's chain, a flow property's direction is the feature's.
var oclTags = map[string]oclNavigation{
	"propertyPath": {"ConnectorEnd", "propertyPath", "", "Property"},
	"direction":    {"Property", "direction", "direction", "EnumerationLiteral"},
}

// oclSupertypes lists the metaclasses above each abstract UML metaclass the
// navigations yield; the concrete ones umlSupertypes knows.
var oclSupertypes = map[string][]string{
	"NamedElement":       {"Element"},
	"Namespace":          {"NamedElement", "Element"},
	"Type":               {"Namespace", "NamedElement", "Element"},
	"Classifier":         {"Type", "Namespace", "NamedElement", "Element"},
	"TypedElement":       {"NamedElement", "Element"},
	"ConnectableElement": {"TypedElement", "NamedElement", "Element"},
	"StructuralFeature":  {"TypedElement", "NamedElement", "Element"},
	"Feature":            {"NamedElement", "Element"},
}

// oclIsA tells whether a value of metaclass kind is one of meta.
func oclIsA(kind, meta string) bool {
	if kind == meta || meta == "Element" {
		return true
	}
	if is, known := umlIsA(kind, meta); known {
		return is
	}
	return contains(oclSupertypes[kind], meta)
}

// oclKnownKind tells whether name is a metaclass the lowering reasons about.
func oclKnownKind(name string) bool {
	_, concrete := umlSupertypes[name]
	_, abstract := oclSupertypes[name]
	return concrete || abstract || name == "Element"
}

// lowerOCL lowers the expression src over self: the row variable (with
// self.variable "row"), or the elements a column's chain collects from the
// row; self.kind is their v1 metaclass when known. The result is a cell
// expression text, its operations qualified by prefix, and the v2 metaclass
// the row is declared as: the narrowest the expression reads it as.
func (m *migration) lowerOCL(src, prefix string, self oclValue) (text, rowType string, notes []string, err error) {
	n, why := parseOCL(src)
	if why != "" {
		return "", "", nil, &oclRefusal{why: why}
	}
	l := &oclLowering{m: m, source: src, rowKind: self.kind, prefix: prefix, needs: map[string]string{}, env: map[string]oclValue{}, used: map[string]bool{}}
	l.env["self"] = self
	v, err := l.lower(n)
	if err != nil {
		return "", "", nil, err
	}
	if v.symbol != oclConcrete {
		return "", "", nil, l.refuse(n, "reads stereotype applications, which the v2 model carries as features")
	}
	return v.text, oclMetaclass(l.needs["row"]), l.notes, nil
}

// variable picks a lambda variable name not bound or used yet.
func (l *oclLowering) variable(prefer string) string {
	for i := 0; ; i++ {
		name := prefer
		if i > 0 {
			name += strconv.Itoa(i)
		}
		if _, bound := l.env[name]; !bound && !l.used[name] && name != "row" {
			l.used[name] = true
			return name
		}
	}
}

// each applies body to every element of v, read as the v1 metaclass meta:
// body(e) over the variable e, or directly when v is a single element.
func (l *oclLowering) each(v oclValue, meta string, body func(e string) string) string {
	if v.single {
		return body(v.text)
	}
	kind := v.kind
	if kind == "" || oclIsA(meta, kind) {
		kind = meta
	}
	e := l.variable("e")
	return v.text + "->" + l.function("collect") + " {in " + l.declare(e, kind) + "; " + body(e) + "}"
}

func (l *oclLowering) lower(n *oclNode) (oclValue, error) {
	switch n.kind {
	case oclString:
		return oclValue{text: stringLiteral(n.literal), kind: "String", single: true}, nil
	case oclInteger:
		return oclValue{text: n.literal, kind: "Integer", single: true}, nil
	case oclBoolean:
		return oclValue{text: n.literal, kind: "Boolean", single: true}, nil
	case oclVariable:
		if v, ok := l.env[n.name]; ok {
			return v, nil
		}
		return l.navigate(n, l.env["self"], n.name)
	case oclNavigate:
		src := l.env["self"]
		if n.src != nil {
			var err error
			if src, err = l.lower(n.src); err != nil {
				return oclValue{}, err
			}
		}
		return l.navigate(n, src, n.name)
	case oclCall:
		return l.call(n)
	case oclIterate:
		return l.iterate(n)
	case oclNot:
		operand, err := l.boolean(n.args[0], "not")
		if err != nil {
			return oclValue{}, err
		}
		return oclValue{text: "not " + operand, kind: "Boolean", single: true}, nil
	case oclBinary:
		return l.binary(n)
	}
	return oclValue{}, l.refuse(n, "is not lowered")
}

// boolean lowers a Boolean operand of the operator op, parenthesized where
// the operand binds looser than op does.
func (l *oclLowering) boolean(n *oclNode, op string) (string, error) {
	v, err := l.lower(n)
	if err != nil {
		return "", err
	}
	if v.symbol != oclConcrete {
		return "", l.refuse(n, "is not a Boolean the v2 model can test")
	}
	if v.kind != "Boolean" && v.kind != "" {
		return "", l.refuse(n, "is not a Boolean")
	}
	if n.kind == oclBinary && n.name != op && oclLevel(n.name) <= oclLevel(op) {
		return "(" + v.text + ")", nil
	}
	return v.text, nil
}

// oclLevel is how tightly a binary operator binds; not binds tighter than all.
func oclLevel(op string) int {
	for level, ops := range oclLevels {
		if contains(ops, op) {
			return level
		}
	}
	return len(oclLevels)
}

// oclOperators maps the OCL operators to their v2 spelling.
var oclOperators = map[string]string{"=": "==", "<>": "!=", "and": "and", "or": "or", "xor": "xor", "implies": "implies"}

func (l *oclLowering) binary(n *oclNode) (oclValue, error) {
	switch n.name {
	case "and", "or", "xor", "implies":
		left, err := l.boolean(n.args[0], n.name)
		if err != nil {
			return oclValue{}, err
		}
		right, err := l.boolean(n.args[1], n.name)
		if err != nil {
			return oclValue{}, err
		}
		return oclValue{text: left + " " + n.name + " " + right, kind: "Boolean", single: true}, nil
	}
	left, err := l.lower(n.args[0])
	if err != nil {
		return oclValue{}, err
	}
	right, err := l.lower(n.args[1])
	if err != nil {
		return oclValue{}, err
	}
	if left.symbol == oclStereotypeCount || right.symbol == oclStereotypeCount {
		return l.stereotypeCount(n, left, right)
	}
	op, ok := oclOperators[n.name]
	if !ok {
		return oclValue{}, l.refuse(n, "compares with "+strconv.Quote(n.name)+", which only a stereotype count is compared with")
	}
	if left.symbol != oclConcrete || right.symbol != oclConcrete {
		return oclValue{}, l.refuse(n, "compares stereotype applications, which the v2 model carries as features")
	}
	return oclValue{text: l.operand(n.args[0], left) + " " + op + " " + l.operand(n.args[1], right), kind: "Boolean", single: true}, nil
}

// operand parenthesizes a compound operand.
func (l *oclLowering) operand(n *oclNode, v oclValue) string {
	if n.kind == oclBinary || n.kind == oclNot {
		return "(" + v.text + ")"
	}
	return v.text
}

// stereotypeCount lowers `…classifier->any(x | x.name = 'S')->size() <> 0`
// and its kin to a test of the v2 features the stereotype migrated to.
func (l *oclLowering) stereotypeCount(n *oclNode, left, right oclValue) (oclValue, error) {
	count, literal, op := left, right, n.name
	if right.symbol == oclStereotypeCount {
		count, literal = right, left
		op = map[string]string{"<": ">", ">": "<", "<=": ">=", ">=": "<=", "=": "=", "<>": "<>"}[op]
	}
	if literal.kind != "Integer" || literal.text != "0" {
		return oclValue{}, l.refuse(n, "compares a stereotype count with something other than 0")
	}
	has, err := l.hasStereotype(n, count)
	if err != nil {
		return oclValue{}, err
	}
	switch op {
	case "<>", ">":
		return has, nil
	case "=", "<=":
		return oclValue{text: "not " + has.text, kind: "Boolean", single: true}, nil
	}
	return oclValue{}, l.refuse(n, "compares a stereotype count with "+strconv.Quote(op))
}

// hasStereotype lowers "the host has the stereotype named" to a test over
// the v2 features it migrated to.
func (l *oclLowering) hasStereotype(n *oclNode, v oclValue) (oclValue, error) {
	if v.tag == "" {
		return oclValue{}, l.refuse(n, "tests stereotype applications without naming the stereotype")
	}
	host := v.host
	for host.symbol != oclConcrete {
		host = host.host
	}
	if !host.single {
		return oclValue{}, l.refuse(n, "tests the stereotypes of several elements at once")
	}
	switch {
	case v.tag == "FlowProperty":
		if !l.conforms(host.kind, "Property") {
			return oclValue{}, l.refuse(n, "tests for «FlowProperty» on a "+host.kind+", which is no Property")
		}
		l.need(host.variable, "Property")
		return oclValue{text: host.text + ".direction->" + l.function("notEmpty") + "()", kind: "Boolean", single: true}, nil
	case stereotypeTypes[v.tag].types != nil:
		t := stereotypeTypes[v.tag]
		if t.note != "" {
			l.notes = append(l.notes, t.note)
		}
		types := make([]string, len(t.types))
		for i, typ := range t.types {
			types[i] = stringLiteral(typ)
		}
		return oclValue{text: l.prefix + "WhereType(source = " + host.text + ", type = (" + strings.Join(types, ", ") + "))->" + l.function("notEmpty") + "()", kind: "Boolean", single: true}, nil
	}
	if def := l.m.stereotypeNamed(v.tag); def != nil && l.m.written(def) {
		return oclValue{text: l.prefix + "WhereMetadata(source = " + host.text + ", 'metadata' = (" + stringLiteral(l.m.plainName(def)) + "))->" + l.function("notEmpty") + "()", kind: "Boolean", single: true}, nil
	}
	return oclValue{}, l.refuse(n, "tests for «"+v.tag+"», which no v2 feature stands for")
}

// stereotypeNamed finds the user stereotype of that name the archive bundles.
func (m *migration) stereotypeNamed(name string) *sysmlv1.Element {
	var found *sysmlv1.Element
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		if found != nil {
			return
		}
		if e.Type == "Stereotype" && e.Name == name && m.userStereotype(e) {
			found = e
			return
		}
		for _, child := range e.Children {
			walk(child)
		}
	}
	for _, root := range m.model.Roots {
		walk(root)
	}
	return found
}

// navigate lowers src.prop.
func (l *oclLowering) navigate(n *oclNode, src oclValue, prop string) (oclValue, error) {
	switch src.symbol {
	case oclStereotypeInst:
		switch prop {
		case "slot":
			return oclValue{symbol: oclSlots, host: src.host}, nil
		case "classifier":
			return oclValue{symbol: oclStereotypes, host: src.host}, nil
		}
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of a stereotype instance; only its slots and classifiers are read")
	case oclSlots:
		if prop == "value" && src.tag != "" {
			return oclValue{symbol: oclSlotValue, host: src.host, tag: src.tag}, nil
		}
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of slots; only the value of slots selected by definingFeature.name is read")
	case oclSlotValue:
		if prop == "element" || prop == "instance" {
			return l.tagValue(n, src.host, src.tag)
		}
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of a slot value; only element and instance are read")
	case oclStereotypes, oclStereotypeCount:
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of stereotypes, which the v2 model carries as features")
	}
	if prop == "appliedStereotypeInstance" {
		return oclValue{symbol: oclStereotypeInst, host: &src}, nil
	}
	if src.kind == "String" || src.kind == "Integer" || src.kind == "Boolean" {
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of a "+src.kind)
	}
	if src.kind == "EnumerationLiteral" && prop == "name" {
		// An enumeration literal's name is the string the v2 feature holds.
		return oclValue{text: src.text, kind: "String", single: src.single}, nil
	}
	// A navigation of the kind itself comes first; one of a narrower metaclass
	// the context implies (a member read as a Property) when none is.
	for _, cast := range []bool{false, true} {
		for _, nav := range oclNavigations {
			if nav.prop != prop || !l.conforms(src.kind, nav.meta) || cast != (src.kind != "" && !oclIsA(src.kind, nav.meta)) {
				continue
			}
			l.need(src.variable, nav.meta)
			return l.feature(n, src, nav)
		}
	}
	if src.kind == "" {
		return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+", which no v2 feature stands for")
	}
	return oclValue{}, l.refuse(n, "reads "+strconv.Quote(prop)+" of a "+src.kind+", which no v2 feature stands for")
}

// conforms tells whether elements of kind may be read as meta: they are one,
// or kind is unknown, or meta is the narrower metaclass a cast implied.
func (l *oclLowering) conforms(kind, meta string) bool {
	return kind == "" || oclIsA(kind, meta) || oclIsA(meta, kind)
}

// need records that the expression reads the bound variable as meta.
func (l *oclLowering) need(variable, meta string) {
	if variable == "" {
		return
	}
	if current := l.needs[variable]; current == "" || (oclIsA(meta, current) && meta != current) {
		l.needs[variable] = meta
	}
}

// feature lowers the v2 reading of one navigation.
func (l *oclLowering) feature(n *oclNode, src oclValue, nav oclNavigation) (oclValue, error) {
	if nav.feature != "" {
		return oclValue{text: src.text + "." + nav.feature, kind: nav.result}, nil
	}
	switch nav.prop {
	case "_typedElementOfType":
		text := l.each(src, nav.meta, func(t string) string {
			return l.prefix + "RelatedElements(source = " + t + ", relationshipKind = \"typing\", direction = \"incoming\", maxDepth = 1)"
		})
		return oclValue{text: text, kind: nav.result}, nil
	case "role":
		text := l.each(src, nav.meta, func(e string) string { return e + ".chainingFeature->" + l.function("last") + "()" })
		return oclValue{text: text, kind: nav.result, single: src.single}, nil
	case "partWithPort":
		text := l.each(src, nav.meta, func(e string) string {
			return l.pathText(e) + "->" + l.function("last") + "()"
		})
		return oclValue{text: text, kind: nav.result, single: src.single}, nil
	}
	return oclValue{}, l.refuse(n, "reads "+strconv.Quote(nav.prop)+", which no v2 feature stands for")
}

// tagValue lowers the values of the slot of tag on host.
func (l *oclLowering) tagValue(n *oclNode, host *oclValue, tag string) (oclValue, error) {
	nav, ok := oclTags[tag]
	if !ok {
		return oclValue{}, l.refuse(n, "reads the tag "+strconv.Quote(tag)+", which no v2 feature stands for")
	}
	if !l.conforms(host.kind, nav.meta) {
		return oclValue{}, l.refuse(n, "reads the tag "+strconv.Quote(tag)+" of a "+host.kind+", which carries none")
	}
	l.need(host.variable, nav.meta)
	switch tag {
	case "propertyPath":
		text := l.each(*host, nav.meta, l.pathText)
		return oclValue{text: text, kind: nav.result}, nil
	}
	return oclValue{text: host.text + "." + nav.feature, kind: nav.result}, nil
}

// pathText reads the features a connector end e chains through to the one it
// attaches: the nested connector end's property path.
func (l *oclLowering) pathText(e string) string {
	return e + ".chainingFeature->" + l.function("excluding") + "(" + e + ".chainingFeature->" + l.function("last") + "())"
}

// oclCollectionOps are the argument-free collection operations and their v2
// spelling; the result kind is the source's unless given.
var oclCollectionOps = map[string]struct{ v2, kind string }{
	"size":         {"size", "Integer"},
	"isEmpty":      {"isEmpty", "Boolean"},
	"notEmpty":     {"notEmpty", "Boolean"},
	"first":        {"head", ""},
	"last":         {"last", ""},
	"asSet":        {"distinct", ""},
	"asOrderedSet": {"distinct", ""},
	"asSequence":   {"", ""},
	"asBag":        {"", ""},
	"flatten":      {"", ""},
}

func (l *oclLowering) call(n *oclNode) (oclValue, error) {
	if n.src == nil {
		return oclValue{}, l.refuse(n, "calls "+strconv.Quote(n.name)+", which is not an operation of a value")
	}
	src, err := l.lower(n.src)
	if err != nil {
		return oclValue{}, err
	}
	if !n.arrow {
		switch n.name {
		case "oclAsType":
			if len(n.args) != 1 {
				return oclValue{}, l.refuse(n, "casts without one type")
			}
			typ, _, ok := n.args[0].path()
			if !ok || typ == "self" {
				return oclValue{}, l.refuse(n, "casts to something other than a type name")
			}
			if src.symbol != oclConcrete {
				// Slot, ElementValue and InstanceValue casts narrow reflection
				// the v2 model does not spell.
				return src, nil
			}
			if !oclKnownKind(typ) {
				return oclValue{}, l.refuse(n, "casts to "+typ+", which is not a UML metaclass the lowering knows")
			}
			if !l.conforms(src.kind, typ) {
				return oclValue{}, l.refuse(n, "casts a "+src.kind+" to "+typ+", which it cannot be")
			}
			src.kind = typ
			return src, nil
		case "oclIsKindOf", "oclIsTypeOf":
			return oclValue{}, l.refuse(n, "tests a metaclass; the v2 model classifies by its own metaclasses")
		}
		return oclValue{}, l.refuse(n, "calls the operation "+strconv.Quote(n.name)+", which has no v2 spelling")
	}
	if src.symbol == oclStereotypes && n.name == "size" && len(n.args) == 0 {
		return oclValue{symbol: oclStereotypeCount, host: src.host, tag: src.tag}, nil
	}
	if src.symbol == oclStereotypes && (n.name == "notEmpty" || n.name == "isEmpty") && len(n.args) == 0 {
		has, err := l.hasStereotype(n, src)
		if err != nil {
			return oclValue{}, err
		}
		if n.name == "isEmpty" {
			has.text = "not " + has.text
		}
		return has, nil
	}
	if src.symbol != oclConcrete {
		return oclValue{}, l.refuse(n, "applies "+strconv.Quote(n.name)+" to stereotype applications, which the v2 model carries as features")
	}
	if op, ok := oclCollectionOps[n.name]; ok {
		if len(n.args) != 0 {
			return oclValue{}, l.refuse(n, "applies "+strconv.Quote(n.name)+" with arguments")
		}
		kind := src.kind
		if op.kind != "" {
			kind = op.kind
		}
		if op.v2 == "" {
			return oclValue{text: src.text, kind: kind, single: src.single}, nil
		}
		single := op.kind != "" || n.name == "first" || n.name == "last"
		return oclValue{text: src.text + "->" + l.function(op.v2) + "()", kind: kind, single: single}, nil
	}
	switch n.name {
	case "includes", "excludes", "including", "excluding":
		if len(n.args) != 1 {
			return oclValue{}, l.refuse(n, "applies "+strconv.Quote(n.name)+" without one argument")
		}
		arg, err := l.lower(n.args[0])
		if err != nil {
			return oclValue{}, err
		}
		if arg.symbol != oclConcrete {
			return oclValue{}, l.refuse(n, "applies "+strconv.Quote(n.name)+" to stereotype applications")
		}
		kind := src.kind
		if n.name == "includes" || n.name == "excludes" {
			kind = "Boolean"
		}
		return oclValue{text: src.text + "->" + l.function(n.name) + "(" + arg.text + ")", kind: kind, single: kind == "Boolean"}, nil
	}
	return oclValue{}, l.refuse(n, "applies "+strconv.Quote(n.name)+", which is not a collection operation the lowering knows")
}

// oclIterators maps the OCL iterators to the v2 sequence functions.
var oclIterators = map[string]string{
	"select": "select", "reject": "reject", "exists": "exists", "forAll": "forAll", "collect": "collect", "any": "select",
}

func (l *oclLowering) iterate(n *oclNode) (oclValue, error) {
	fn, ok := oclIterators[n.name]
	if !ok {
		return oclValue{}, l.refuse(n, "iterates with "+strconv.Quote(n.name)+", which is not an iterator the lowering knows")
	}
	src, err := l.lower(n.src)
	if err != nil {
		return oclValue{}, err
	}
	switch src.symbol {
	case oclSlots:
		return l.selectSlots(n, src)
	case oclStereotypes:
		return l.selectStereotypes(n, src)
	case oclConcrete:
	default:
		return oclValue{}, l.refuse(n, "iterates over stereotype applications, which the v2 model carries as features")
	}
	variable := writeName(n.variable)
	if _, bound := l.env[variable]; bound || l.used[variable] || variable == "row" {
		return oclValue{}, l.refuse(n, "binds "+strconv.Quote(n.variable)+", which is bound already")
	}
	l.env[variable] = oclValue{text: variable, kind: src.kind, single: true, variable: variable}
	l.used[variable] = true
	body, err := l.lower(n.args[0])
	delete(l.env, variable)
	declared := l.declare(variable, src.kind)
	if err != nil {
		return oclValue{}, err
	}
	if body.symbol != oclConcrete {
		return oclValue{}, l.refuse(n, "yields stereotype applications, which the v2 model carries as features")
	}
	result := oclValue{text: src.text + "->" + l.function(fn) + " {in " + declared + "; " + body.text + "}", kind: src.kind}
	switch n.name {
	case "exists", "forAll":
		result.kind, result.single = "Boolean", true
	case "collect":
		result.kind = body.kind
	case "any":
		result.text += "->" + l.function("head") + "()"
		result.single = true
	}
	if n.name != "collect" && body.kind != "Boolean" && body.kind != "" {
		return oclValue{}, l.refuse(n, "selects by a "+body.kind+", not a Boolean")
	}
	return result, nil
}

// selectSlots lowers `slot->select(x | x.definingFeature.name = 'tag' and …)`:
// the slots of that tag on the hosts the rest of the condition keeps.
func (l *oclLowering) selectSlots(n *oclNode, src oclValue) (oclValue, error) {
	if n.name != "select" {
		return oclValue{}, l.refuse(n, "iterates over slots with "+strconv.Quote(n.name)+"; only select by definingFeature.name is lowered")
	}
	tag := ""
	var rest []*oclNode
	for _, term := range n.args[0].conjuncts() {
		if name, ok := l.definingFeatureName(term, n.variable); ok && tag == "" {
			tag = name
			continue
		}
		rest = append(rest, term)
	}
	if tag == "" {
		return oclValue{}, l.refuse(n, "selects slots without testing definingFeature.name against a string")
	}
	host := *src.host
	if len(rest) > 0 {
		e := host.text
		if !host.single {
			e = l.variable("e")
		}
		variable := writeName(n.variable)
		l.env[variable] = oclValue{symbol: oclSlots, host: &oclValue{text: e, kind: host.kind, single: true, variable: e}, tag: tag}
		var conditions []string
		var err error
		for _, term := range rest {
			var condition string
			if condition, err = l.boolean(term, "and"); err != nil {
				break
			}
			conditions = append(conditions, condition)
		}
		delete(l.env, variable)
		if err != nil {
			return oclValue{}, err
		}
		if !host.single {
			host = oclValue{text: host.text + "->" + l.function("select") + " {in " + l.declare(e, host.kind) + "; " + strings.Join(conditions, " and ") + "}", kind: host.kind}
		} else {
			return oclValue{}, l.refuse(n, "selects the slots of one element by a condition, which keeps or drops it whole")
		}
	}
	return oclValue{symbol: oclSlots, host: &host, tag: tag}, nil
}

// definingFeatureName reads a term `v.definingFeature.name = 'tag'` of the
// slot variable v, through casts, either way round.
func (l *oclLowering) definingFeatureName(term *oclNode, v string) (string, bool) {
	if term.kind != oclBinary || term.name != "=" {
		return "", false
	}
	for _, side := range [][2]*oclNode{{term.args[0], term.args[1]}, {term.args[1], term.args[0]}} {
		variable, members, ok := side[0].path()
		if ok && variable == v && len(members) == 2 && members[0] == "definingFeature" && members[1] == "name" && side[1].kind == oclString {
			return side[1].literal, true
		}
	}
	return "", false
}

// selectStereotypes lowers `classifier->any(x | x.name = 'S')` and
// `->exists(x | x.name = 'S')`: the stereotype named, or whether it applies.
func (l *oclLowering) selectStereotypes(n *oclNode, src oclValue) (oclValue, error) {
	name := ""
	if term := n.args[0]; term.kind == oclBinary && term.name == "=" {
		for _, side := range [][2]*oclNode{{term.args[0], term.args[1]}, {term.args[1], term.args[0]}} {
			variable, members, ok := side[0].path()
			if ok && variable == n.variable && len(members) == 1 && members[0] == "name" && side[1].kind == oclString {
				name = side[1].literal
			}
		}
	}
	if name == "" {
		return oclValue{}, l.refuse(n, "selects stereotypes by something other than their name")
	}
	selected := oclValue{symbol: oclStereotypes, host: src.host, tag: name}
	switch n.name {
	case "any", "select":
		return selected, nil
	case "exists":
		return l.hasStereotype(n, selected)
	}
	return oclValue{}, l.refuse(n, "iterates over stereotypes with "+strconv.Quote(n.name))
}

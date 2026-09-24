package migrate

import (
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// UML metaclass ancestries, so a metaclass filter can be told statically over
// the source elements a DocGen chain collects, as DocGen tells it.
var (
	umlNamed       = []string{"NamedElement", "Element"}
	umlPackageable = append([]string{"PackageableElement", "ParameterableElement"}, umlNamed...)
	umlType        = append([]string{"Type"}, umlPackageable...)
	umlClassifier  = append([]string{"Classifier", "Namespace", "RedefinableElement", "TemplateableElement"}, umlType...)
	umlClass       = append([]string{"Class", "BehavioredClassifier", "EncapsulatedClassifier", "StructuredClassifier"}, umlClassifier...)
	umlBehavior    = append([]string{"Behavior"}, umlClass...)
	umlDataType    = append([]string{"DataType"}, umlClassifier...)
	umlPackage     = append([]string{"Package", "Namespace", "TemplateableElement"}, umlPackageable...)
	umlFeature     = append([]string{"Feature", "RedefinableElement"}, umlNamed...)
	umlProperty    = append([]string{"Property", "StructuralFeature", "ConnectableElement", "DeploymentTarget", "TypedElement", "MultiplicityElement", "ParameterableElement"}, umlFeature...)
	umlInstance    = append([]string{"InstanceSpecification", "DeployedArtifact", "DeploymentTarget"}, umlPackageable...)
	umlRelation    = append([]string{"DirectedRelationship", "Relationship"}, umlPackageable...)
	umlDependency  = append([]string{"Dependency"}, umlRelation...)
	umlVertex      = append([]string{"Vertex"}, umlNamed...)
	umlNode        = append([]string{"ActivityNode", "RedefinableElement"}, umlNamed...)
	umlAction      = append([]string{"Action", "ExecutableNode"}, umlNode...)
	umlInvocation  = append([]string{"InvocationAction"}, umlAction...)
	umlCall        = append([]string{"CallAction"}, umlInvocation...)
	umlStructured  = append([]string{"StructuredActivityNode", "Namespace", "ActivityGroup"}, umlAction...)
	umlControl     = append([]string{"ControlNode"}, umlNode...)
)

// umlSupertypes lists, per concrete UML metaclass, every metaclass above it.
var umlSupertypes = map[string][]string{
	"Package": umlPackage, "Model": umlPackage, "Profile": umlPackage,
	"Class": umlClass, "Component": umlClass, "Stereotype": umlClass,
	"Node":     append([]string{"DeploymentTarget"}, umlClass...),
	"Device":   append([]string{"Node", "DeploymentTarget"}, umlClass...),
	"Activity": umlBehavior, "StateMachine": umlBehavior, "Interaction": umlBehavior, "OpaqueBehavior": umlBehavior,
	"FunctionBehavior":     append([]string{"OpaqueBehavior"}, umlBehavior...),
	"ProtocolStateMachine": append([]string{"StateMachine"}, umlBehavior...),
	"AssociationClass":     append([]string{"Association", "Relationship"}, umlClass...),
	"Actor":                append([]string{"BehavioredClassifier"}, umlClassifier...),
	"UseCase":              append([]string{"BehavioredClassifier"}, umlClassifier...),
	"Collaboration":        append([]string{"BehavioredClassifier", "StructuredClassifier"}, umlClassifier...),
	"Signal":               umlClassifier, "Interface": umlClassifier, "Artifact": umlClassifier, "InformationItem": umlClassifier,
	"Association": append([]string{"Association", "Relationship"}, umlClassifier...),
	"DataType":    umlDataType, "PrimitiveType": umlDataType, "Enumeration": umlDataType,
	"InstanceSpecification": umlInstance,
	"EnumerationLiteral":    append([]string{"EnumerationLiteral"}, umlInstance...),
	"Property":              umlProperty,
	"Port":                  umlProperty,
	"ExtensionEnd":          umlProperty,
	"Operation":             append([]string{"BehavioralFeature", "Namespace", "TemplateableElement", "ParameterableElement"}, umlFeature...),
	"Reception":             append([]string{"BehavioralFeature", "Namespace"}, umlFeature...),
	"Parameter":             append([]string{"ConnectableElement", "TypedElement", "MultiplicityElement", "ParameterableElement"}, umlNamed...),
	"Connector":             umlFeature,
	"ConnectorEnd":          {"MultiplicityElement", "Element"},
	"Slot":                  {"Element"},
	"Constraint":            umlPackageable,
	"InteractionConstraint": append([]string{"Constraint"}, umlPackageable...),
	"Comment":               {"Element"},
	"Dependency":            umlDependency,
	"Abstraction":           umlDependency, "Realization": append([]string{"Abstraction"}, umlDependency...),
	"Usage": umlDependency, "Deployment": umlDependency, "Manifestation": append([]string{"Abstraction"}, umlDependency...),
	"InterfaceRealization":     append([]string{"Realization", "Abstraction"}, umlDependency...),
	"ComponentRealization":     append([]string{"Realization", "Abstraction"}, umlDependency...),
	"Generalization":           {"DirectedRelationship", "Relationship", "Element"},
	"ElementImport":            {"DirectedRelationship", "Relationship", "Element"},
	"PackageImport":            {"DirectedRelationship", "Relationship", "Element"},
	"PackageMerge":             {"DirectedRelationship", "Relationship", "Element"},
	"ProfileApplication":       {"DirectedRelationship", "Relationship", "Element"},
	"Extension":                append([]string{"Association", "Relationship"}, umlClassifier...),
	"InformationFlow":          umlRelation,
	"Region":                   append([]string{"Namespace", "RedefinableElement"}, umlNamed...),
	"State":                    append([]string{"State", "Namespace", "RedefinableElement"}, umlVertex...),
	"FinalState":               append([]string{"State", "Namespace", "RedefinableElement"}, umlVertex...),
	"Pseudostate":              umlVertex,
	"ConnectionPointReference": umlVertex,
	"Transition":               append([]string{"Namespace", "RedefinableElement"}, umlNamed...),
	"Trigger":                  umlNamed,
	"OpaqueAction":             umlAction, "ValueSpecificationAction": umlAction,
	"ReadStructuralFeatureAction":     append([]string{"StructuralFeatureAction"}, umlAction...),
	"AddStructuralFeatureValueAction": append([]string{"WriteStructuralFeatureAction", "StructuralFeatureAction"}, umlAction...),
	"CallBehaviorAction":              umlCall, "CallOperationAction": umlCall,
	"SendSignalAction":  umlInvocation,
	"AcceptEventAction": umlAction, "AcceptCallAction": append([]string{"AcceptEventAction"}, umlAction...),
	"StructuredActivityNode": umlStructured,
	"ConditionalNode":        umlStructured, "LoopNode": umlStructured, "SequenceNode": umlStructured,
	"ExpansionRegion": umlStructured,
	"ForkNode":        umlControl, "JoinNode": umlControl, "DecisionNode": umlControl, "MergeNode": umlControl,
	"InitialNode": umlControl, "ActivityFinalNode": append([]string{"FinalNode"}, umlControl...),
	"FlowFinalNode":                  append([]string{"FinalNode"}, umlControl...),
	"ActivityParameterNode":          append([]string{"ObjectNode", "TypedElement"}, umlNode...),
	"CentralBufferNode":              append([]string{"ObjectNode", "TypedElement"}, umlNode...),
	"DataStoreNode":                  append([]string{"CentralBufferNode", "ObjectNode", "TypedElement"}, umlNode...),
	"InputPin":                       append([]string{"Pin", "ObjectNode", "TypedElement", "MultiplicityElement"}, umlNode...),
	"OutputPin":                      append([]string{"Pin", "ObjectNode", "TypedElement", "MultiplicityElement"}, umlNode...),
	"ValuePin":                       append([]string{"InputPin", "Pin", "ObjectNode", "TypedElement", "MultiplicityElement"}, umlNode...),
	"ControlFlow":                    append([]string{"ActivityEdge", "RedefinableElement"}, umlNamed...),
	"ObjectFlow":                     append([]string{"ActivityEdge", "RedefinableElement"}, umlNamed...),
	"ActivityPartition":              append([]string{"ActivityGroup"}, umlNamed...),
	"InterruptibleActivityRegion":    {"ActivityGroup", "NamedElement", "Element"},
	"Lifeline":                       umlNamed,
	"Message":                        umlNamed,
	"OccurrenceSpecification":        append([]string{"InteractionFragment"}, umlNamed...),
	"MessageOccurrenceSpecification": append([]string{"OccurrenceSpecification", "InteractionFragment", "MessageEnd"}, umlNamed...),
	"CombinedFragment":               append([]string{"InteractionFragment"}, umlNamed...),
	"InteractionOperand":             append([]string{"InteractionFragment", "Namespace"}, umlNamed...),
	"InteractionUse":                 append([]string{"InteractionFragment"}, umlNamed...),
	"LiteralString":                  append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"LiteralInteger":                 append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"LiteralReal":                    append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"LiteralBoolean":                 append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"LiteralUnlimitedNatural":        append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"LiteralNull":                    append([]string{"LiteralSpecification", "ValueSpecification", "TypedElement"}, umlPackageable...),
	"OpaqueExpression":               append([]string{"ValueSpecification", "TypedElement"}, umlPackageable...),
	"Expression":                     append([]string{"ValueSpecification", "TypedElement"}, umlPackageable...),
	"InstanceValue":                  append([]string{"ValueSpecification", "TypedElement"}, umlPackageable...),
	"TimeEvent":                      append([]string{"Event"}, umlPackageable...),
	"SignalEvent":                    append([]string{"MessageEvent", "Event"}, umlPackageable...),
	"CallEvent":                      append([]string{"MessageEvent", "Event"}, umlPackageable...),
	"ChangeEvent":                    append([]string{"Event"}, umlPackageable...),
	"Duration":                       append([]string{"ValueSpecification", "TypedElement"}, umlPackageable...),
	"DurationConstraint":             append([]string{"IntervalConstraint", "Constraint"}, umlPackageable...),
	"TimeConstraint":                 append([]string{"IntervalConstraint", "Constraint"}, umlPackageable...),
}

// umlIsA tells whether an element of metaclass typ is an instance of meta, and
// whether the ancestry table can tell.
func umlIsA(typ, meta string) (is, known bool) {
	if typ == meta || meta == "Element" {
		return true, true
	}
	supers, ok := umlSupertypes[typ]
	if !ok {
		return false, false
	}
	for _, s := range supers {
		if s == meta {
			return true, true
		}
	}
	return false, true
}

// holderOfType tells whether the element type ref admits e as DocGen's filter
// would: by UML metaclass, or by a stereotype applied to e, or one it
// specializes while derived holds. known is false when that cannot be told.
func (m *migration) holderOfType(ref sysmlv1.ElementRef, e *sysmlv1.Element, derived bool) (keep, known bool) {
	t := ref.Element
	if t == nil {
		return false, false
	}
	if doc, name, ok := standardHref(t.Href); ok && doc == "UML" {
		return umlIsA(e.Type, name)
	}
	name, id := m.stereotypeName(ref), ref.ID
	if name == "" {
		return false, false
	}
	known = true
	for _, s := range e.Stereotypes {
		if s.Name == name {
			return true, true
		}
		if !derived {
			continue
		}
		for _, g := range s.Generals {
			if g.Name == name || g.ID == id || (t.ID != "" && g.ID == t.ID) {
				return true, true
			}
		}
		// Without the definition, what the application specializes is unknown.
		if s.Definition == nil {
			known = false
		}
	}
	return false, known
}

// stereotypeName is the local name of the stereotype ref names, "" when unknown.
func (m *migration) stereotypeName(ref sysmlv1.ElementRef) string {
	if s := m.model.StereotypeRef(ref.ID); s.Name != "" {
		return s.Name
	}
	if t := ref.Element; t != nil {
		if _, name, ok := standardHref(t.Href); ok {
			return name
		}
		if t.Type == "Stereotype" || t.IsProxy() {
			return t.Name
		}
	}
	return ""
}

// diagramOwner is the element that owns d: the one it names, else the one
// whose extension holds it; nil when neither is known.
func diagramOwner(d *sysmlv1.Diagram) *sysmlv1.Element {
	if d.Owner != nil {
		return d.Owner
	}
	return d.Holder
}

// ownedDiagrams are the diagrams owned by e, in model order.
func (m *migration) ownedDiagrams(e *sysmlv1.Element) []*sysmlv1.Diagram {
	if m.diagramsOf == nil {
		m.diagramsOf = map[*sysmlv1.Element][]*sysmlv1.Diagram{}
		for i := range m.model.Diagrams {
			d := &m.model.Diagrams[i]
			if owner := diagramOwner(d); owner != nil {
				m.diagramsOf[owner] = append(m.diagramsOf[owner], d)
			}
		}
	}
	return m.diagramsOf[e]
}

// owned walks the elements e owns to depth, all of them for 0, calling visit
// on each element and each diagram met.
func (m *migration) owned(e *sysmlv1.Element, depth int, visit func(*sysmlv1.Element), diagram func(*sysmlv1.Diagram)) {
	var walk func(e *sysmlv1.Element, level int)
	walk = func(e *sysmlv1.Element, level int) {
		if depth > 0 && level > depth {
			return
		}
		for _, d := range m.ownedDiagrams(e) {
			diagram(d)
		}
		for _, c := range e.Children {
			visit(c)
			walk(c, level+1)
		}
	}
	walk(e, 1)
}

// owners walks from e up through its owners to depth, all of them for 0,
// calling visit on each. The root Model, which is not written, ends the walk.
func owners(e *sysmlv1.Element, depth int, visit func(*sysmlv1.Element)) {
	for i := 0; e != nil && !isTopLevel(e) && (depth == 0 || i < depth); i, e = i+1, e.Parent {
		visit(e)
	}
}

// appendElement adds e to es unless it is there already.
func appendElement(es []*sysmlv1.Element, e *sysmlv1.Element) []*sysmlv1.Element {
	for _, x := range es {
		if x == e {
			return es
		}
	}
	return append(es, e)
}

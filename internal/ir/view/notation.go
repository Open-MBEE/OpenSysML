package view

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Element is a notation element the SysML v2 graphical notation makes
// optional, which a rendering shows or leaves out by name.
type Element string

const (
	// ElementParameterSymbols are the symbols on a node's boundary standing for
	// its parameters: a case's actor, subject and stakeholder parameters and an
	// action's parameter pins (SysML v2 §7.26.5 and the distinguished-parameter
	// alternatives of §8.2.3). Left out, a connection that ended at the
	// parameter ends at the node declaring it.
	ElementParameterSymbols Element = "parameter-symbols"
	// ElementPortSymbols are the port symbols on a part's boundary (§7.26.5,
	// "can include symbols on the boundary of the node to represent ports").
	// Left out, a connection that ended at a port ends at the part owning it.
	ElementPortSymbols Element = "port-symbols"
	// ElementCompartments are the compartments under a node's name: the lines
	// a node's label carries after its title (§7.26.5, "can include compartments").
	ElementCompartments Element = "compartments"
	// ElementEdgeLabels are the names connections, interfaces, flows, bindings
	// and allocations are labelled with (§8.2.3, the optional name of a
	// connection-link and flow-link).
	ElementEdgeLabels Element = "edge-labels"
	// ElementKeywords are the «keyword» lines heading a node (§8.2.3, the
	// optional keyword of a node's name-compartment).
	ElementKeywords Element = "keywords"
	// ElementMultiplicities are the `[n]` multiplicities after a usage's name
	// (§8.2.3, the optional multiplicity of a feature-declaration).
	ElementMultiplicities Element = "multiplicities"
	// ElementTypings are the `: Type` texts after a usage's name and a port's
	// (§8.2.3, the optional feature-typing of a feature-declaration).
	ElementTypings Element = "typings"
	// ElementQualifiers are the qualifying prefixes of a node's name (§8.2.3,
	// a name may be written qualified or not).
	ElementQualifiers Element = "qualifiers"
)

// ElementAll is the name standing for every optional element at once.
const ElementAll = "all"

// elements lists the optional elements in the order they are documented.
var elements = []Element{
	ElementParameterSymbols, ElementPortSymbols, ElementCompartments, ElementEdgeLabels,
	ElementKeywords, ElementMultiplicities, ElementTypings, ElementQualifiers,
}

// Elements lists the optional notation elements in documentation order.
func Elements() []Element { return slices.Clone(elements) }

// ElementNames lists the element names a switch accepts, "all" last.
func ElementNames() string {
	names := make([]string, 0, len(elements)+1)
	for _, element := range elements {
		names = append(names, string(element))
	}
	return strings.Join(append(names, ElementAll), ", ")
}

// Attribute is the name of the Boolean attribute a rendering declared in the
// model shows or hides the element with: `attribute showEdgeLabels : Boolean = true;`.
func (e Element) Attribute() string {
	var b strings.Builder
	b.WriteString("show")
	for _, word := range strings.Split(string(e), "-") {
		b.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return b.String()
}

// ElementOfAttribute is the element a rendering's Boolean attribute switches,
// by the attribute's name; false for a name switching none.
func ElementOfAttribute(name string) (Element, bool) {
	for _, element := range elements {
		if element.Attribute() == name {
			return element, true
		}
	}
	return "", false
}

// Shown is the set of optional notation elements a rendering draws. A nil set
// is the default, DefaultShown; an empty set draws none of them.
type Shown map[Element]bool

// DefaultShown is what a rendering shows when neither the model nor the caller
// says: ports, compartments and keywords, as the notation's examples draw them,
// and none of the parameter symbols, edge labels, multiplicities, typings and
// qualifiers. It is the one place the default is decided.
var DefaultShown = Shown{ElementPortSymbols: true, ElementCompartments: true, ElementKeywords: true}

// AllShown is every optional element, what "all" names: the full notation.
func AllShown() Shown {
	all := make(Shown, len(elements))
	for _, element := range elements {
		all[element] = true
	}
	return all
}

// Has reports whether the set shows element, reading a nil set as the default.
func (s Shown) Has(element Element) bool {
	if s == nil {
		return DefaultShown[element]
	}
	return s[element]
}

// Union is a new set of the elements s shows with those of more added; nil
// when both are nil, the default.
func (s Shown) Union(more Shown) Shown {
	if s == nil && more == nil {
		return nil
	}
	out := make(Shown, len(elements))
	for _, element := range elements {
		if s.Has(element) || more[element] {
			out[element] = true
		}
	}
	return out
}

// Names lists the shown elements in documentation order; empty for none.
func (s Shown) Names() []string {
	var names []string
	for _, element := range elements {
		if s.Has(element) {
			names = append(names, string(element))
		}
	}
	return names
}

// ErrUnknownElement is the sentinel every unknown-element error wraps.
var ErrUnknownElement = errors.New("unknown notation element")

// UnknownElementError reports an element name no optional element has.
type UnknownElementError struct {
	Name string
}

func (e *UnknownElementError) Error() string {
	return fmt.Sprintf("unknown notation element %q; the elements are %s", e.Name, ElementNames())
}

func (e *UnknownElementError) Unwrap() error { return ErrUnknownElement }

// ParseShown reads a comma-separated list of element names, or "all", into
// the set to show: nil for an empty list. A name none has is an
// *UnknownElementError naming the elements there are.
func ParseShown(text string) (Shown, error) {
	return ShownOf(strings.Split(text, ","))
}

// ShownOf is the set to show the element names, or "all", name: nil for
// none. A name none has is an *UnknownElementError naming the elements there are.
func ShownOf(names []string) (Shown, error) {
	var shown Shown
	for _, name := range names {
		name = strings.TrimSpace(name)
		switch {
		case name == "":
			continue
		case name == ElementAll:
			shown = AllShown()
		case !slices.Contains(elements, Element(name)):
			return nil, &UnknownElementError{Name: name}
		default:
			if shown == nil {
				shown = Shown{}
			}
			shown[Element(name)] = true
		}
	}
	return shown, nil
}

// String is the set as ParseShown reads it, "all" for every element.
func (s Shown) String() string {
	names := s.Names()
	if len(names) == len(elements) {
		return ElementAll
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

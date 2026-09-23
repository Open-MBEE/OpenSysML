// Package query contains the transport-neutral element query model and
// evaluator. Frontends adapt their model/index and wire formats to these
// types; query semantics do not depend on a transport.
package query

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const (
	// PropertyID is the element identity property.
	PropertyID = "@id"
	// PropertyType is the metamodel type property.
	PropertyType = "@type"
	// PropertyName is the effective local name property.
	PropertyName = "name"
	// PropertyDeclaredName is the explicitly declared name property.
	PropertyDeclaredName = "declaredName"
	// PropertyShortName is the effective short name property.
	PropertyShortName = "shortName"
	// PropertyDeclaredShortName is the explicitly declared short name property.
	PropertyDeclaredShortName = "declaredShortName"
	// PropertyDocumentation is the body text of each `doc` comment, in order.
	PropertyDocumentation = "documentation"
	// PropertyQualifiedName is the fully qualified name property.
	PropertyQualifiedName = "qualifiedName"
	// PropertyOwner is the owning element's qualified name property.
	PropertyOwner = "owner"
	// PropertyElementType is the resolved declared element type property.
	PropertyElementType = "type"
	// PropertyIsAbstract is the abstractness property.
	PropertyIsAbstract = "isAbstract"
	// PropertyIsIndividual is the `individual` modifier of a definition or usage.
	PropertyIsIndividual = "isIndividual"
	// PropertyMultiplicityLower is the lower multiplicity bound property.
	PropertyMultiplicityLower = "multiplicityLower"
	// PropertyMultiplicityUpper is the upper multiplicity bound property.
	PropertyMultiplicityUpper = "multiplicityUpper"
)

// propertyNames is the closed set of properties supported by element queries.
var propertyNames = []string{
	PropertyID, PropertyType, PropertyName, PropertyDeclaredName,
	PropertyShortName, PropertyDeclaredShortName, PropertyDocumentation,
	PropertyQualifiedName, PropertyOwner, PropertyIsAbstract, PropertyIsIndividual, PropertyElementType,
	PropertyMultiplicityLower, PropertyMultiplicityUpper,
}

// PropertyNames returns the supported property names in stable order.
func PropertyNames() []string {
	out := append([]string(nil), propertyNames...)
	sort.Strings(out)
	return out
}

// IsOrdered reports whether a property supports numeric ordered comparisons.
func IsOrdered(name string) bool {
	return name == PropertyMultiplicityLower || name == PropertyMultiplicityUpper
}

// IsProperty reports whether name belongs to the closed queryable-property set.
func IsProperty(name string) bool {
	for _, candidate := range propertyNames {
		if candidate == name {
			return true
		}
	}
	return false
}

// ErrorKind identifies a query failure.
type ErrorKind int

const (
	// ErrUnknownProperty identifies an unknown query property.
	ErrUnknownProperty ErrorKind = iota + 1
	// ErrMalformed identifies malformed query syntax or structure.
	ErrMalformed
	// ErrUnorderedProperty identifies ordered comparison on an unordered property.
	ErrUnorderedProperty
	// ErrUnparsableValue identifies a value an operator cannot parse.
	ErrUnparsableValue
	// ErrUnknownScope identifies a scope absent from the model.
	ErrUnknownScope
	// ErrNoModel identifies a query without a loaded model.
	ErrNoModel
	// ErrUnsupportedScopedTerm identifies an unsupported nested property query.
	ErrUnsupportedScopedTerm
	// ErrUnsupportedWildcard identifies an unsupported property wildcard.
	ErrUnsupportedWildcard
	// ErrUnsupportedSearchTerms identifies unsupported free-text search.
	ErrUnsupportedSearchTerms
	// ErrUnsupportedLiteral identifies an unsupported literal form.
	ErrUnsupportedLiteral
)

// Error is a typed query failure. A query failure must not be represented as an
// empty result because that would be indistinguishable from a valid no-match.
type Error struct {
	Kind    ErrorKind
	Message string
}

func (e *Error) Error() string { return e.Message }

func errorf(kind ErrorKind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// Operator is a predicate comparison operator.
type Operator string

const (
	Equal        Operator = "="
	NotEqual     Operator = "!="
	Less         Operator = "<"
	Greater      Operator = ">"
	LessEqual    Operator = "<="
	GreaterEqual Operator = ">="
	In           Operator = "in"
)

// Predicate is one unnested property comparison.
type Predicate struct {
	Property string
	Operator Operator
	Values   []string
}

// OrderTerm is one unnested sort key. Desc is true for a '-' prefix.
type OrderTerm struct {
	Property string
	Desc     bool
}

// Query is the common representation used by structured and OSLC queries.
type Query struct {
	Scope   []string
	Select  []string
	Where   []Predicate
	OrderBy []OrderTerm
	// Spelling names, per query property, the name the request asked for it by,
	// where that differs from the query property name.
	Spelling map[string]string
}

// SpellingOf returns the name the request asked for property by, so a frontend
// reports a result in terms its caller can ask the next query in.
func (q Query) SpellingOf(property string) string {
	if spelled, ok := q.Spelling[property]; ok {
		return spelled
	}
	return property
}

// Element is a transport-neutral query result.
type Element struct {
	ID         string
	Type       string
	Properties map[string]string
}

// Model supplies the facts an evaluator needs from a parsed model.
type Model interface {
	Candidates(scope []string) ([]*symbols.Symbol, error)
	Value(sym *symbols.Symbol, property string) ([]string, bool)
	Identity(sym *symbols.Symbol) string
	Type(sym *symbols.Symbol) string
}

// Evaluate evaluates a query against a model, retaining declaration order
// unless explicit order terms are present.
func Evaluate(model Model, q Query) ([]Element, error) {
	if err := validate(q); err != nil {
		return nil, err
	}
	candidates, err := model.Candidates(q.Scope)
	if err != nil {
		return nil, err
	}
	requestedSelect := q.Select
	selected := append([]string(nil), requestedSelect...)
	if len(selected) == 0 {
		selected = PropertyNames()
	} else {
		for _, term := range q.OrderBy {
			found := false
			for _, name := range selected {
				if name == term.Property {
					found = true
					break
				}
			}
			if !found {
				selected = append(selected, term.Property)
			}
		}
	}
	out := make([]Element, 0, len(candidates))
	for _, sym := range candidates {
		matches := true
		for _, p := range q.Where {
			if !matchesPredicate(model, sym, p) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		e := Element{ID: model.Identity(sym), Type: model.Type(sym), Properties: make(map[string]string)}
		for _, name := range selected {
			values, ok := model.Value(sym, name)
			if ok && len(values) != 0 {
				e.Properties[name] = values[0]
			}
		}
		out = append(out, e)
	}
	if len(q.OrderBy) != 0 {
		sort.SliceStable(out, func(i, j int) bool {
			for _, term := range q.OrderBy {
				a, aok := out[i].Properties[term.Property]
				b, bok := out[j].Properties[term.Property]
				if a == b || (!aok && !bok) {
					continue
				}
				if !aok {
					return !term.Desc
				}
				if !bok {
					return term.Desc
				}
				less := func(left, right string) bool {
					if IsOrdered(term.Property) {
						leftValue, leftErr := ordered(left)
						rightValue, rightErr := ordered(right)
						if leftErr == nil && rightErr == nil {
							return leftValue < rightValue
						}
					}
					return left < right
				}
				if term.Desc {
					return less(b, a)
				}
				return less(a, b)
			}
			return false
		})
	}
	if len(requestedSelect) != 0 {
		for i := range out {
			for name := range out[i].Properties {
				found := false
				for _, selectedName := range requestedSelect {
					if name == selectedName {
						found = true
						break
					}
				}
				if !found {
					delete(out[i].Properties, name)
				}
			}
		}
	}
	return out, nil
}

func validate(q Query) error {
	for _, name := range q.Select {
		if !IsProperty(name) {
			return unknownProperty(name)
		}
	}
	for _, term := range q.OrderBy {
		if !IsProperty(term.Property) {
			return unknownProperty(term.Property)
		}
	}
	for _, p := range q.Where {
		if !IsProperty(p.Property) {
			return unknownProperty(p.Property)
		}
		if len(p.Values) == 0 {
			return errorf(ErrMalformed, "query property %q has no value to compare against", p.Property)
		}
		// "*" is the infinite multiplicity bound; anywhere else it is a wildcard
		// no model value spells, so comparing it lexically answers no-match.
		if !IsOrdered(p.Property) {
			for _, value := range p.Values {
				if value == "*" {
					return errorf(ErrUnsupportedWildcard, `value wildcards are not implemented: query property %q cannot be compared against "*"; omit the term to select every element`, p.Property)
				}
			}
		}
		switch p.Operator {
		case Equal, NotEqual, In:
		case Less, Greater, LessEqual, GreaterEqual:
			if !IsOrdered(p.Property) {
				return errorf(ErrUnorderedProperty, "query property %q is not ordered, so %s cannot compare it", p.Property, p.Operator)
			}
			if len(p.Values) != 1 {
				return errorf(ErrMalformed, "%s on %q compares against exactly one value, got %d", p.Operator, p.Property, len(p.Values))
			}
			if _, err := ordered(p.Values[0]); err != nil {
				return errorf(ErrUnparsableValue, "%s on %q needs a number to compare against, got %q", p.Operator, p.Property, p.Values[0])
			}
		default:
			return errorf(ErrMalformed, "query property %q has unsupported operator %q", p.Property, p.Operator)
		}
	}
	return nil
}

func matchesPredicate(model Model, sym *symbols.Symbol, p Predicate) bool {
	values, ok := model.Value(sym, p.Property)
	if !ok {
		return false
	}
	switch p.Operator {
	case Equal:
		return anyEqual(values, p.Values)
	case NotEqual:
		if len(values) == 0 {
			return false
		}
		return !anyEqual(values, p.Values)
	case In:
		return anyEqual(values, p.Values)
	case Less, Greater, LessEqual, GreaterEqual:
		if len(values) == 0 {
			return false
		}
		actual, err := ordered(values[0])
		if err != nil {
			return false
		}
		operand, err := ordered(p.Values[0])
		if err != nil {
			return false
		}
		switch p.Operator {
		case Less:
			return actual < operand
		case Greater:
			return actual > operand
		case LessEqual:
			return actual <= operand
		default:
			return actual >= operand
		}
	}
	return false
}

func anyEqual(values, candidates []string) bool {
	for _, value := range values {
		for _, candidate := range candidates {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

func ordered(text string) (float64, error) {
	if text == "*" {
		return math.Inf(1), nil
	}
	return strconv.ParseFloat(text, 64)
}

func unknownProperty(name string) *Error {
	return errorf(ErrUnknownProperty, "unknown query property %q; queryable properties are %s", name, strings.Join(PropertyNames(), ", "))
}

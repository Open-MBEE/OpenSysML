package flexo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// rdfNil is what the service stores for a JSON null.
const rdfNil = rdf.RDFNS + "nil"

var (
	integerLexical = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	decimalLexical = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]+$`)
)

// Representation is what the service stores of a posted graph: sysml: properties
// only, values as JSON spells them. A diff under it converges with an apply.
type Representation struct{}

// UnrepresentableError is a value the service's JSON commit path has no
// spelling for; the diff refuses it rather than approximating.
type UnrepresentableError struct {
	Term   rdf.Term
	Reason string
}

func (e *UnrepresentableError) Error() string {
	return fmt.Sprintf("%s cannot be written through the SysML v2 API: %s", e.Term, e.Reason)
}

// Carry keeps sysml: properties in the form the service stores them and drops
// every other predicate: it has no place for them.
func (Representation) Carry(triple rdf.Triple) (rdf.Triple, bool, error) {
	if !strings.HasPrefix(triple.Predicate.Value, rdf.SysML) {
		return rdf.Triple{}, false, nil
	}
	object, err := carryTerm(triple.Object)
	if err != nil {
		return rdf.Triple{}, false, err
	}
	return rdf.Triple{Subject: triple.Subject, Predicate: triple.Predicate, Object: object}, true, nil
}

// carryTerm maps an object onto what a JSON round trip stores: numbers typed by
// whether they carry a fraction, strings untyped, IRIs as references.
func carryTerm(term rdf.Term) (rdf.Term, error) {
	if term.IsIRI() {
		switch {
		case term.Value == rdfNil,
			strings.HasPrefix(term.Value, rdf.Element),
			strings.HasPrefix(term.Value, rdf.Expression):
			return term, nil
		}
		return term, &UnrepresentableError{Term: term, Reason: "an IRI is only writable as a reference to an element"}
	}
	if term.Lang != "" {
		return term, &UnrepresentableError{Term: term, Reason: "JSON strings carry no language tag"}
	}
	switch term.Datatype {
	case "", rdf.XSD + "string":
		return rdf.String(term.Value), nil
	case rdf.XSD + "boolean":
		if term.Value == "true" || term.Value == "false" {
			return term, nil
		}
		return term, &UnrepresentableError{Term: term, Reason: "a JSON boolean is spelled true or false"}
	case rdf.XSD + "integer":
		if integerLexical.MatchString(term.Value) {
			return term, nil
		}
		return term, &UnrepresentableError{Term: term, Reason: "not a JSON integer"}
	case rdf.XSD + "decimal":
		switch {
		case decimalLexical.MatchString(term.Value):
			return term, nil
		case integerLexical.MatchString(term.Value):
			return rdf.TypedLiteral(term.Value, rdf.XSD+"integer"), nil
		}
		return term, &UnrepresentableError{Term: term, Reason: "not a JSON number"}
	}
	return term, &UnrepresentableError{Term: term, Reason: "JSON has no value of datatype " + term.Datatype}
}

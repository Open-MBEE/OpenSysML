package export_test

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func mappingShapeAgreesWithForwardMapping() corpusCheck {
	var countsMu sync.Mutex
	compared, disagreements := 0, 0
	return corpusCheck{
		name: "mapping_shape_agrees_with_forward_mapping",
		check: func(t *testing.T, file corpusFile) {
			if _, err := file.conversions.apiJSON(file.path, file.source, "standard", "graph"); err != nil {
				var unsupported *export.UnsupportedError
				if !errors.As(err, &unsupported) {
					t.Fatalf("write API JSON: %v", err)
				}
			}
			mismatches := mappingShapeDisagreements(t, file.path, file.graph)
			countsMu.Lock()
			compared++
			disagreements += mismatches
			countsMu.Unlock()
		},
		summarize: func(t *testing.T) {
			countsMu.Lock()
			defer countsMu.Unlock()
			t.Logf("mapping shape: compared %d models; disagreements: %d", compared, disagreements)
			if disagreements != 0 {
				t.Errorf("mapping-shape predicates disagree with the forward mapping at %d nodes", disagreements)
			}
		},
	}
}

func TestMappingShapeAgreesWithForwardMappingForNestedExpressionBodies(t *testing.T) {
	const nestedExpressionBody = `package Edge {
    attribute def Real;
    calc def Nested {
        in x : Real;
        { in y : Real { doc /* the input */ } attribute k : Real = 2 { attribute j : Real = 3; } private attribute m : Real = k * 2; y * m + x }
    }
}
`
	graph, err := convert.SysMLToRDF("expression-body-declarations-nest.sysml", []byte(nestedExpressionBody))
	if err != nil {
		t.Fatal(err)
	}
	mappingShapeDisagreements(t, "expression-body-declarations-nest.sysml", graph)
}

func mappingShapeDisagreements(t *testing.T, path string, graph *rdf.Graph) int {
	t.Helper()
	named := map[string]bool{}
	var stripped []rdf.Triple
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+"qualifiedName" || triple.Predicate.Value == rdf.OpenSysML+"positionalName" ||
			triple.Predicate.Value == rdf.OpenSysML+"libraryPath" {
			named[triple.Subject.Value] = true
			continue
		}
		stripped = append(stripped, triple)
	}
	withoutNames := rdf.NewGraphOf(stripped, graph.Prefixes)
	var failures []string
	for _, subject := range graph.Subjects() {
		if export.LibraryReference(graph, subject) {
			continue
		}
		want := named[subject.Value]
		got := export.MappingElementSubject(withoutNames, subject)
		if want != got {
			failures = append(failures, mappingShapeFailure(withoutNames, path, "element", subject, rdf.Term{}, want, got))
		}
	}
	seenMembers := map[string]bool{}
	for _, subject := range graph.Subjects() {
		if export.LibraryReference(graph, subject) || !export.MappingMembershipEdgeSubject(withoutNames, subject) {
			continue
		}
		for _, property := range []string{
			"ownedMemberElement", "ownedMemberFeature", "ownedMemberParameter",
			"ownedVariantUsage", "ownedResultExpression", "ownedRelatedElement",
		} {
			for _, member := range graph.Objects(subject, rdf.SysML+property) {
				if !member.IsIRI() {
					continue
				}
				key := subject.Value + "\x00" + member.Value
				if seenMembers[key] {
					continue
				}
				seenMembers[key] = true
				want := named[member.Value]
				got := export.MappingMembershipMemberIsElement(withoutNames, subject, member)
				if want != got {
					failures = append(failures, mappingShapeFailure(withoutNames, path, property, subject, member, want, got))
				}
			}
		}
	}
	for _, failure := range failures {
		t.Error(failure)
	}
	return len(failures)
}

func mappingShapeFailure(graph *rdf.Graph, path, decision string, subject, member rdf.Term, want, got bool) string {
	types := func(term rdf.Term) string {
		values := graph.Objects(term, rdf.RDFType)
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = value.Value
		}
		return strings.Join(parts, ",")
	}
	owner := firstMappingOwner(graph, subject)
	memberType := "n/a"
	if member.Value != "" {
		memberType = types(member)
	}
	return path + ": " + decision + " subject=" + subject.Value +
		" type=[" + types(subject) + "] member=" + member.Value +
		" memberType=[" + memberType + "] owner=" + owner +
		" forward-element=" + strconv.FormatBool(want) + " predicate-element=" + strconv.FormatBool(got)
}

func firstMappingOwner(graph *rdf.Graph, subject rdf.Term) string {
	for _, property := range []string{"owner", "owningRelatedElement", "membershipOwningNamespace"} {
		if owner, ok := graph.Object(subject, rdf.SysML+property); ok {
			return owner.Value
		}
	}
	return ""
}

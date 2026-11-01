package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestMappingShapeAgreesWithForwardMapping(t *testing.T) {
	paths := modelFiles(t)
	exampleRoot := filepath.Join("..", "..", "examples")
	err := filepath.WalkDir(exampleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".sysml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	conversions := corpusConversions(t)

	var compared, skipped, disagreements int
	var countsMu sync.Mutex
	t.Cleanup(func() {
		countsMu.Lock()
		defer countsMu.Unlock()
		t.Logf("compared %d models, skipped %d unconvertible examples; disagreements: %d", compared, skipped, disagreements)
		if disagreements != 0 {
			t.Errorf("mapping-shape predicates disagree with the forward mapping at %d nodes", disagreements)
		}
	})
	for _, path := range paths {
		path := path
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			t.Parallel()
			defer conversions.release(path)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := conversions.sysMLToRDF(path, source)
			if err != nil {
				if strings.HasPrefix(path, exampleRoot+string(filepath.Separator)) {
					countsMu.Lock()
					skipped++
					countsMu.Unlock()
					return
				}
				t.Fatalf("convert fixture %s: %v", path, err)
			}
			if _, err := conversions.apiJSON(path, source, "standard", "graph"); err != nil {
				var unsupported *export.UnsupportedError
				if !errors.As(err, &unsupported) {
					t.Fatalf("write API JSON: %v", err)
				}
			}
			mismatches := mappingShapeDisagreements(t, path, graph)
			countsMu.Lock()
			compared++
			disagreements += mismatches
			countsMu.Unlock()
		})
	}
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
	countsMu.Lock()
	compared++
	disagreements += mappingShapeDisagreements(t, "expression-body-declarations-nest.sysml", graph)
	countsMu.Unlock()
}

func mappingShapeDisagreements(t *testing.T, path string, graph *rdf.Graph) int {
	t.Helper()
	named := map[string]bool{}
	var stripped []rdf.Triple
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+"qualifiedName" || triple.Predicate.Value == rdf.OpenSysML+"positionalName" {
			named[triple.Subject.Value] = true
			continue
		}
		stripped = append(stripped, triple)
	}
	withoutNames := rdf.NewGraphOf(stripped, graph.Prefixes)
	var failures []string
	for _, subject := range graph.Subjects() {
		want := named[subject.Value]
		got := export.MappingElementSubject(withoutNames, subject)
		if want != got {
			failures = append(failures, mappingShapeFailure(withoutNames, path, "element", subject, rdf.Term{}, want, got))
		}
	}
	seenMembers := map[string]bool{}
	for _, subject := range graph.Subjects() {
		if !export.MappingMembershipEdgeSubject(withoutNames, subject) {
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

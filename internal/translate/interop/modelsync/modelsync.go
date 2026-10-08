// Package modelsync is what `sysml -sync` and the REPL's repository commands
// share above reposync: reading a model as the graph a sync compares, cutting
// the graph rooted in one element, stamping a graph with the project branch it
// came from so its notation declares a ProjectRef, and applying a change set
// while advancing the sync state. Neither client diffs or applies on its own.
package modelsync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Repository is a project branch a sync reads and writes: the surface
// sysmlapi.Repository and flexo.Repository both offer.
type Repository interface {
	reposync.Committer
	// Head is the branch's current head commit, empty for a branch no commit
	// has reached yet.
	Head(ctx context.Context) (string, error)
	// Seen is the head the last read stood at or the last commit written.
	Seen() string
	// Resume records the last-seen commit a saved state carries.
	Resume(commit string)
	// Graph reads the branch at its head.
	Graph(ctx context.Context) (*rdf.Graph, error)
	// GraphAt reads the branch as one commit left it.
	GraphAt(ctx context.Context, commit string) (*rdf.Graph, error)
}

// Baseline reads the repository as the state's last-seen commit left it: the
// graph a diff tells repository changes since then apart against. Nil when the
// state names no commit.
func Baseline(ctx context.Context, repo Repository, state *reposync.State) (*rdf.Graph, error) {
	if state == nil || state.LastSeenCommit == "" {
		return nil, nil
	}
	base, err := repo.GraphAt(ctx, state.LastSeenCommit)
	if err != nil {
		return nil, fmt.Errorf("read the repository at last-seen commit %s: %w", state.LastSeenCommit, err)
	}
	return base, nil
}

// Applied is what one apply left behind: the commit it ended on and how many
// changes of each kind landed. Commit is empty when the repository already
// agreed with the model.
type Applied struct {
	Commit                    string
	Created, Updated, Deleted int
	Result                    *reposync.Result
}

// Apply writes an appliable change set and advances the state: to the last
// commit written, or, when nothing needed writing, to the head the repository
// was read at. A partial apply returns the result with the error.
func Apply(ctx context.Context, repo Repository, set *reposync.ChangeSet, state *reposync.State, message string) (*Applied, error) {
	if err := set.Appliable(); err != nil {
		return nil, err
	}
	result, err := reposync.Apply(ctx, repo, set, reposync.ApplyOptions{Message: message})
	if err != nil {
		return &Applied{Result: result}, err
	}
	applied := &Applied{Result: result}
	for _, change := range result.Applied {
		switch change.Kind {
		case reposync.KindCreate:
			applied.Created++
		case reposync.KindUpdate:
			applied.Updated++
		case reposync.KindDelete:
			applied.Deleted++
		}
	}
	if len(result.Applied) == 0 {
		if head := repo.Seen(); head != "" {
			state.LastSeenCommit = head
		}
		return applied, nil
	}
	if err := state.Advance(result); err != nil {
		return applied, err
	}
	applied.Commit = result.LastCommit()
	return applied, nil
}

// Scope predicates: the provenance an exported graph carries on its scope roots,
// the ones reposync.GraphScope reads.
const (
	predProjectID = rdf.OpenSysML + "projectId"
	predBranch    = rdf.OpenSysML + "branch"
	predOrg       = rdf.OpenSysML + "org"
)

// ownerPredicates lead from an element to what owns it, in the forms the
// mapping writes; owningRelationship leads to the relationship, whose own
// owner is the element's.
var ownerPredicates = []string{
	rdf.SysML + "owner",
	rdf.SysML + "owningRelatedElement",
	rdf.SysML + "membershipOwningNamespace",
	rdf.SysML + "owningNamespace",
	rdf.SysML + "owningMembership",
	rdf.SysML + "owningRelationship",
}

// Scoped returns graph with scope stamped on every root element — each
// element of the graph that nothing in it owns and that is not a library
// reference — in place of any scope it carried, so the notation written from
// it declares the ProjectRef a later sync finds the branch by.
func Scoped(graph *rdf.Graph, scope reposync.Scope) *rdf.Graph {
	out := rdf.NewGraphWithCapacity(graph.Len())
	for _, triple := range graph.Triples() {
		switch triple.Predicate.Value {
		case predProjectID, predBranch, predOrg:
			continue
		}
		out.AddTriple(triple)
	}
	if scope.IsZero() {
		return out
	}
	for _, root := range Roots(graph) {
		for _, field := range []struct{ predicate, value string }{
			{predProjectID, scope.ProjectID}, {predBranch, scope.Branch}, {predOrg, scope.Org},
		} {
			if field.value != "" {
				out.Add(root, rdf.IRI(field.predicate), rdf.String(field.value))
			}
		}
	}
	return out
}

// Roots lists the elements the graph declares and nothing in it owns, in the
// graph's subject order: the elements a document's namespace would hold.
func Roots(graph *rdf.Graph) []rdf.Term {
	var roots []rdf.Term
	for _, subject := range graph.Subjects() {
		if !strings.HasPrefix(subject.Value, rdf.Element) || !strings.HasPrefix(graph.Type(subject), rdf.SysML) {
			continue
		}
		if ownerOf(graph, subject).Value != "" || export.LibraryReference(graph, subject) {
			continue
		}
		roots = append(roots, subject)
	}
	return roots
}

// ownerOf is the element or relationship that owns subject in the graph; the
// zero term when the graph states none.
func ownerOf(graph *rdf.Graph, subject rdf.Term) rdf.Term {
	for _, predicate := range ownerPredicates {
		if owner, ok := graph.Object(subject, predicate); ok && owner.IsIRI() {
			return owner
		}
	}
	return rdf.Term{}
}

// Notation writes a graph as SysML notation, each warning the writer raises
// passed to warn.
func Notation(graph *rdf.Graph, warn func(string)) ([]byte, error) {
	return convert.FromGraphWith(graph, convert.FormatSysML, convert.Options{Warn: warn})
}

// NotFoundError is a qualified name no element of the graph carries.
type NotFoundError struct {
	QualifiedName string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no element of the model is named %s", e.QualifiedName)
}

// Rooted cuts the graph down to the element of the given qualified name and
// everything it owns, directly or through its relationships. The element
// becomes a root: the triples tying it to its own owner are dropped, as a
// project's root element has none. Elements the cut references but does not
// own stay references.
func Rooted(graph *rdf.Graph, qualifiedName string) (*rdf.Graph, error) {
	root := rdf.Term{}
	for _, subject := range graph.Subjects() {
		if name, ok := graph.Lexical(subject, rdf.SysML+"qualifiedName"); ok && name == qualifiedName {
			root = subject
			break
		}
	}
	if root.Value == "" {
		return nil, &NotFoundError{QualifiedName: qualifiedName}
	}
	inside := map[rdf.Term]bool{root: true}
	reaches := func(subject rdf.Term) bool {
		seen := map[rdf.Term]bool{}
		var chain []rdf.Term
		for current := subject; current.Value != "" && !seen[current]; current = ownerOf(graph, current) {
			if known, ok := inside[current]; ok {
				for _, c := range chain {
					inside[c] = known
				}
				return known
			}
			seen[current] = true
			chain = append(chain, current)
		}
		for _, c := range chain {
			inside[c] = false
		}
		return false
	}
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		subject := triple.Subject
		if !subject.IsIRI() || !reaches(subject) {
			continue
		}
		if subject == root && isOwnerPredicate(triple.Predicate.Value) {
			continue
		}
		out.AddTriple(triple)
	}
	return out, nil
}

func isOwnerPredicate(predicate string) bool {
	for _, p := range ownerPredicates {
		if predicate == p {
			return true
		}
	}
	return false
}

// readerDerived are the derived properties the notation reader takes from the
// graph rather than recomputing — a collection's order among them — so a
// publish without derived properties keeps them: a project written without
// them could not be loaded back as notation.
// TestWithoutDerivedReadsBackEveryExample pins the list against the reader.
var readerDerived = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`annotatedElement annotatingElement argument chainingFeature condition
		conjugatedPortDefinition connectorEnd effectAction featureChained featureWithValue
		function guardExpression importOwningNamespace input isLibraryElement isReference
		lowerBound membershipOwningNamespace multiplicity operand ownedActorParameter
		ownedAnnotation ownedConcern ownedConstraint ownedEndFeature ownedFeature
		ownedFeatureChaining ownedFeatureMembership ownedImport ownedMember ownedMemberElement
		ownedMemberFeature ownedMemberParameter ownedMembership ownedObjectiveRequirement
		ownedPortConjugator ownedRedefinition ownedReferenceSubsetting ownedRendering
		ownedRequirement ownedResultExpression ownedSpecialization ownedSubclassification
		ownedSubjectParameter ownedSubsetting ownedTyping ownedVariantUsage owner owningFeature
		owningMembership owningNamespace owningType parameter payloadArgument payloadParameter
		qualifiedName referencedElement referencingFeature referent relatedElement relatedFeature
		result source sourceFeature succession target targetFeature transitionFeature
		triggerAction type upperBound value variant variantMembership`) {
		readerDerived[name] = true
	}
}

// WithoutDerived returns graph without the sysml: properties the metamodel
// derives on each subject's metaclass, readerDerived aside: what a publish
// without derived properties sends. The pilot's sends none of them; this
// writer's reader needs most, so only the ones it recomputes are left out.
func WithoutDerived(graph *rdf.Graph) *rdf.Graph {
	out := rdf.NewGraphWithCapacity(graph.Len())
	for _, triple := range graph.Triples() {
		if derived(graph, triple) {
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

// derived tells a derived property's triple, or the collection annotation that
// would restate one, from the properties the reader needs.
func derived(graph *rdf.Graph, triple rdf.Triple) bool {
	name, ok := strings.CutPrefix(triple.Predicate.Value, rdf.SysML)
	if !ok {
		name, ok = strings.CutPrefix(triple.Predicate.Value, rdf.AnnotationJSON)
	}
	if !ok || readerDerived[name] {
		return false
	}
	metaclass, ok := strings.CutPrefix(graph.Type(triple.Subject), rdf.SysML)
	if !ok {
		return false
	}
	property, ok := ontology.PropertyOf(metaclass, name)
	return ok && property.Derived
}

// ErrAmbiguousName is a name several projects share.
var ErrAmbiguousName = errors.New("ambiguous project name")

// AmbiguousNameError names the projects that share one name, so the caller
// can pick one by id.
type AmbiguousNameError struct {
	Name string
	IDs  []string
}

func (e *AmbiguousNameError) Error() string {
	ids := append([]string(nil), e.IDs...)
	sort.Strings(ids)
	return fmt.Sprintf("%d projects are named %q (%s); choose one with --id", len(ids), e.Name, strings.Join(ids, ", "))
}

func (e *AmbiguousNameError) Is(target error) bool { return target == ErrAmbiguousName }

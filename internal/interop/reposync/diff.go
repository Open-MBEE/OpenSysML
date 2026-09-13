package reposync

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// Kind classifies one entry of a change set.
type Kind string

const (
	// KindCreate is an element the repository does not have yet.
	KindCreate Kind = "create"
	// KindUpdate is an element both sides have, with differing properties.
	KindUpdate Kind = "update"
	// KindDelete is an element only the repository has. Applying it needs
	// explicit confirmation; the diff reports it either way.
	KindDelete Kind = "delete"
	// KindConflict is a disagreement the sync must not resolve on its own.
	KindConflict Kind = "conflict"
)

// ConflictKind says what a conflict entry disagrees about.
type ConflictKind string

const (
	// ConflictMissingID: the annotation names an id the repository branch no
	// longer has.
	ConflictMissingID ConflictKind = "missing-id"
	// ConflictRepositoryChanged: the repository element changed since the
	// last-seen commit, so writing over it would lose someone else's change.
	ConflictRepositoryChanged ConflictKind = "repository-changed"
)

// PropertyDelta is one property whose values differ between the two sides.
type PropertyDelta struct {
	Property   string
	Local      []string
	Repository []string
}

// Change is one entry of a change set, keyed by the subject's effective id.
type Change struct {
	Kind          Kind
	ID            string // effective id: the subject IRI's local name
	Subject       string // full subject IRI
	QualifiedName string
	Metaclass     string
	Declared      bool   // the id was declared by an @ElementId annotation
	Normative     bool   // the id is the one the norm fixes for a standard-library element
	MintedID      string // UUID minted for an unannotated create, when minting is on
	Deltas        []PropertyDelta
	Conflict      ConflictKind
	// RequiresConfirmation marks a delete the options did not confirm.
	RequiresConfirmation bool
	// Content is the element as the local graph states it, in the repository's
	// representation: what a create or an update writes. Nil for a delete.
	Content []rdf.Triple
}

// UncarriedProperty is a property of the local graph the repository has no
// place for: left out of the comparison, since no apply could ever settle it.
type UncarriedProperty struct {
	Property string // rendered as in a delta
	Triples  int
}

// ChangeSet is what a diff produced for one project scope.
type ChangeSet struct {
	Scope     Scope
	Changes   []Change
	Uncarried []UncarriedProperty
}

// Carrier is what a repository can hold of a graph; both sides compare
// under it, so a successful apply leaves nothing to diff.
type Carrier interface {
	// Carry maps a triple onto the form the repository stores: ok false when it
	// has no place for the predicate, err when it cannot represent the value.
	Carry(triple rdf.Triple) (carried rdf.Triple, ok bool, err error)
}

// Options steer a diff. Deletes and id minting are explicit opt-ins; neither
// ever happens silently.
type Options struct {
	// Base is the repository graph at the last-seen commit. With it, an element
	// the repository changed since then surfaces as a conflict rather than
	// being written over, and a declared id the branch never had is a create
	// rather than a missing-id conflict.
	Base *rdf.Graph
	// ConfirmDeletes confirms repository-side deletes; off, the diff still
	// reports them but Appliable refuses.
	ConfirmDeletes bool
	// MintIDs mints a UUID for each unannotated element being created, so the
	// repository can address it stably.
	MintIDs bool
	// NewID overrides the UUID source, for deterministic tests.
	NewID func() (string, error)
	// Representation is the repository's; nil compares the graphs as written.
	Representation Carrier
}

// Diff compares the local graph against the repository graph of the same
// project scope and returns the change set keyed by effective element id.
func Diff(local, repository *rdf.Graph, opts Options) (*ChangeSet, error) {
	scope, err := GraphScope(local)
	if err != nil {
		return nil, err
	}
	repoScope, err := GraphScope(repository)
	if err != nil {
		return nil, fmt.Errorf("repository graph: %w", err)
	}
	if !scope.IsZero() && !repoScope.IsZero() {
		if !scope.sameProject(repoScope) {
			return nil, fmt.Errorf("the model is scoped to %s but the repository graph carries %s", scope, repoScope)
		}
		if scope.Branch != "" && repoScope.Branch != "" && scope.Branch != repoScope.Branch {
			return nil, fmt.Errorf("%w: the model names branch %q, the repository graph branch %q", ErrTwoBranches, scope.Branch, repoScope.Branch)
		}
	}

	newID := opts.NewID
	if newID == nil {
		newID = MintUUID
	}

	localView, uncarried, err := viewOf(local, opts.Representation)
	if err != nil {
		return nil, err
	}
	repoView, _, err := viewOf(repository, opts.Representation)
	if err != nil {
		return nil, fmt.Errorf("repository graph: %w", err)
	}
	var baseView map[string]*subjectView
	if opts.Base != nil {
		baseScope, err := GraphScope(opts.Base)
		if err != nil {
			return nil, fmt.Errorf("baseline graph: %w", err)
		}
		// The baseline is checked against the effective scope: the model's,
		// or the repository's when the model is unbound.
		against := scope
		if against.IsZero() {
			against = repoScope
		}
		if !against.IsZero() && !baseScope.IsZero() {
			if !against.sameProject(baseScope) {
				return nil, fmt.Errorf("the sync is scoped to %s but the baseline graph carries %s", against, baseScope)
			}
			if against.Branch != "" && baseScope.Branch != "" && against.Branch != baseScope.Branch {
				return nil, fmt.Errorf("%w: the sync names branch %q, the baseline graph branch %q", ErrTwoBranches, against.Branch, baseScope.Branch)
			}
		}
		baseView, _, err = viewOf(opts.Base, opts.Representation)
		if err != nil {
			return nil, fmt.Errorf("baseline graph: %w", err)
		}
	}

	set := &ChangeSet{Scope: scope, Uncarried: uncarried}
	for _, id := range sortedIDs(localView, repoView) {
		at, inLocal := localView[id]
		was, inRepo := repoView[id]
		base, inBase := baseView[id]
		switch {
		case inLocal && inRepo:
			deltas := propertyDeltas(at, was)
			if len(deltas) == 0 {
				continue
			}
			change := Change{Kind: KindUpdate, Deltas: deltas, Content: at.content}
			if opts.Base != nil {
				switch {
				case !inBase:
					// Both sides added the id since the last-seen graph:
					// writing over the repository's copy would lose it.
					change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged, Deltas: deltas}
				case len(propertyDeltas(was, base)) > 0:
					change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged, Deltas: deltas}
				}
			}
			set.add(change, at)
		case inLocal:
			change := Change{Kind: KindCreate, Content: at.content}
			switch {
			case inBase:
				// The last-seen graph had it and the repository dropped it:
				// re-creating would undo someone else's deletion.
				change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged}
			case at.declared && len(repoView) > 0 && opts.Base == nil:
				// A declared id the branch does not have was either dropped
				// there — a conflict — or never pushed. Only the last-seen
				// graph can tell the two apart; absent it, never guess.
				change = Change{Kind: KindConflict, Conflict: ConflictMissingID}
			}
			if change.Kind == KindCreate && opts.MintIDs && !at.declared && at.mintable {
				minted, err := newID()
				if err != nil {
					return nil, fmt.Errorf("mint an id for %s: %w", at.name(), err)
				}
				change.MintedID = minted
			}
			set.add(change, at)
		default:
			change := Change{Kind: KindDelete, RequiresConfirmation: !opts.ConfirmDeletes}
			if opts.Base != nil {
				switch {
				case !inBase:
					// The repository added it since the last-seen commit:
					// deleting would erase someone else's addition.
					change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged}
				case len(propertyDeltas(was, base)) > 0:
					// The repository changed it since the last-seen commit.
					change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged}
				}
			}
			set.add(change, was)
		}
	}
	return set, nil
}

// add fills a change's identity fields from the subject view and appends it.
func (cs *ChangeSet) add(change Change, view *subjectView) {
	change.ID = view.id
	change.Subject = view.subject
	change.QualifiedName = view.qualifiedName
	change.Metaclass = view.metaclass
	change.Declared = view.declared
	change.Normative = view.normative
	cs.Changes = append(cs.Changes, change)
}

// Conflicts counts the entries the sync refuses to resolve on its own.
func (cs *ChangeSet) Conflicts() int { return cs.count(KindConflict) }

// UnconfirmedDeletes counts the deletes the options did not confirm.
func (cs *ChangeSet) UnconfirmedDeletes() int {
	n := 0
	for _, change := range cs.Changes {
		if change.Kind == KindDelete && change.RequiresConfirmation {
			n++
		}
	}
	return n
}

func (cs *ChangeSet) count(kind Kind) int {
	n := 0
	for _, change := range cs.Changes {
		if change.Kind == kind {
			n++
		}
	}
	return n
}

// NotAppliableError refuses an apply: the set holds conflicts, unconfirmed
// deletes, or both, and a person has to decide them first.
type NotAppliableError struct {
	Conflicts          int
	UnconfirmedDeletes int
}

func (e *NotAppliableError) Error() string {
	var reasons []string
	if e.Conflicts > 0 {
		reasons = append(reasons, fmt.Sprintf("%d conflict(s) must be resolved", e.Conflicts))
	}
	if e.UnconfirmedDeletes > 0 {
		reasons = append(reasons, fmt.Sprintf("%d delete(s) need explicit confirmation", e.UnconfirmedDeletes))
	}
	return strings.Join(reasons, " and ") + " before this change set can be applied"
}

// Appliable reports whether the set can be applied as computed: conflicts and
// unconfirmed deletes refuse an apply, as a *NotAppliableError, until a person
// decides them.
func (cs *ChangeSet) Appliable() error {
	conflicts, unconfirmed := cs.Conflicts(), cs.UnconfirmedDeletes()
	if conflicts == 0 && unconfirmed == 0 {
		return nil
	}
	return &NotAppliableError{Conflicts: conflicts, UnconfirmedDeletes: unconfirmed}
}

// Mints returns the minted ids of a change set, keyed by the derived id each
// one replaces.
func (cs *ChangeSet) Mints() map[string]string {
	mints := map[string]string{}
	for _, change := range cs.Changes {
		if change.MintedID != "" {
			mints[change.ID] = change.MintedID
		}
	}
	return mints
}

// subjectView is one subject reduced to what the comparison needs: its
// identity and its property values, provenance and identity markers excluded.
type subjectView struct {
	subject       string // full IRI
	id            string // effective id: the IRI's local name
	qualifiedName string
	metaclass     string
	declared      bool
	// normative marks the id the norm fixes for a standard-library element:
	// implied by the name, so neither declared nor minted for.
	normative bool
	// mintable marks an element proper: memberships and expression nodes
	// derive their ids and are never minted for.
	mintable bool
	props    map[string][]string
	// content is the subject's type and carried content triples: what an
	// apply writes for it.
	content []rdf.Triple
}

// name names the subject for messages: its qualified name when it has one.
func (v *subjectView) name() string {
	if v.qualifiedName != "" {
		return v.qualifiedName
	}
	return v.id
}

// Predicates that carry identity or provenance rather than element content:
// they decide how subjects are keyed and scoped, not what an element says.
func identityPredicate(iri string) bool {
	return iri == predProjectID || iri == predBranch || iri == predOrg ||
		iri == rdf.OpenSysML+"declaredId"
}

// viewOf indexes a graph by effective element id, reducing each subject to the
// view the comparison works on. The full IRI — scope-qualified or not — only
// carries the id; two subjects sharing one id in one graph is an error. It also
// tallies the properties the representation has no place for; a collection's
// JSON annotation restates typed triples and is reconciled, not tallied.
func viewOf(g *rdf.Graph, rep Carrier) (map[string]*subjectView, []UncarriedProperty, error) {
	g, err := rdf.ReconcileCollections(g)
	if err != nil {
		return nil, nil, err
	}
	views := map[string]*subjectView{}
	uncarried := map[string]int{}
	library := identity.LibraryCatalog(libs.NewModelIndex())
	for _, triple := range g.Triples() {
		if !triple.Subject.IsIRI() {
			continue
		}
		id := rdf.LocalName(triple.Subject.Value)
		view := views[id]
		if view != nil && view.subject != triple.Subject.Value {
			return nil, nil, fmt.Errorf("two subjects carry the effective id %q: %s and %s", id, view.subject, triple.Subject.Value)
		}
		if view == nil {
			view = &subjectView{
				subject: triple.Subject.Value,
				id:      id,
				props:   map[string][]string{},
			}
			views[id] = view
		}
		if triple.Predicate.Value == rdf.RDFType {
			view.metaclass = rdf.LocalName(triple.Object.Value)
			view.content = append(view.content, triple)
			continue
		}
		if identityPredicate(triple.Predicate.Value) || rdf.IsAnnotationJSON(triple.Predicate.Value) {
			continue
		}
		if rep != nil {
			carried, ok, err := rep.Carry(triple)
			if err != nil {
				return nil, nil, fmt.Errorf("%s %s: %w", view.name(), propertyLabel(triple.Predicate.Value), err)
			}
			if !ok {
				uncarried[triple.Predicate.Value]++
				continue
			}
			triple = carried
		}
		view.content = append(view.content, triple)
		// Keyed by full predicate IRI so same-named properties from
		// different namespaces never conflate.
		view.props[triple.Predicate.Value] = append(view.props[triple.Predicate.Value], objectValue(triple.Object))
	}
	for _, view := range views {
		for _, values := range view.props {
			sort.Strings(values)
		}
		if names := view.props[rdf.SysML+"qualifiedName"]; len(names) > 0 {
			view.qualifiedName = strings.Trim(names[0], `"`)
		}
		view.normative = library.NormativeFor(view.id, view.qualifiedName)
		view.declared = declaredID(g, view)
		view.mintable = mintable(view)
	}
	var left []UncarriedProperty
	for _, iri := range sortedKeys(uncarried) {
		left = append(left, UncarriedProperty{Property: propertyLabel(iri), Triples: uncarried[iri]})
	}
	return views, left, nil
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// declaredID mirrors the RDF reader: an explicit declaredId marker, or an id
// that is neither the encoding of the subject's qualified name nor the one the
// norm fixes for it, was declared by an annotation rather than derived.
func declaredID(g *rdf.Graph, view *subjectView) bool {
	if g.BoolValue(rdf.IRI(view.subject), rdf.OpenSysML+"declaredId") {
		return true
	}
	if !view.mintableIRI() || view.qualifiedName == "" || view.normative {
		return false
	}
	return view.id != rdf.EncodeElementID(view.qualifiedName)
}

// mintableIRI reports whether the subject lives in the element namespace,
// where an element proper's id can be declared or minted.
func (v *subjectView) mintableIRI() bool {
	return strings.HasPrefix(v.subject, rdf.Element)
}

// mintable reports whether the subject is an element proper (by rdf:type, which
// an id cannot fake) whose id the norm does not fix.
func mintable(view *subjectView) bool {
	if !view.mintableIRI() || view.normative {
		return false
	}
	return view.metaclass != "OwningMembership" && view.metaclass != "FeatureMembership"
}

// propertyDeltas lists the properties whose value sets differ between two
// views of one subject, the metaclass included as a pseudo-property.
func propertyDeltas(local, repository *subjectView) []PropertyDelta {
	var deltas []PropertyDelta
	if local.metaclass != repository.metaclass {
		deltas = append(deltas, PropertyDelta{
			Property:   "rdf:type",
			Local:      []string{local.metaclass},
			Repository: []string{repository.metaclass},
		})
	}
	names := map[string]bool{}
	for name := range local.props {
		names[name] = true
	}
	for name := range repository.props {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		at, was := local.props[name], repository.props[name]
		if !equalValues(at, was) {
			deltas = append(deltas, PropertyDelta{Property: propertyLabel(name), Local: at, Repository: was})
		}
	}
	return deltas
}

// propertyLabel renders a predicate IRI for the report: the bare name for the
// SysML vocabulary, the full IRI for anything else.
func propertyLabel(iri string) string {
	if name, ok := strings.CutPrefix(iri, rdf.SysML); ok {
		return name
	}
	return iri
}

// objectValue reduces an object term for comparison: a model-internal IRI
// compares by its effective id, so scoped and unscoped spellings of one
// reference are equal; literals and external IRIs compare as written.
func objectValue(term rdf.Term) string {
	if term.IsIRI() && (strings.HasPrefix(term.Value, rdf.Element) || strings.HasPrefix(term.Value, rdf.Expression)) {
		return rdf.LocalName(term.Value)
	}
	return term.String()
}

func equalValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sortedIDs returns the union of both views' effective ids in a stable order,
// so the change set is deterministic.
func sortedIDs(local, repository map[string]*subjectView) []string {
	seen := map[string]bool{}
	for id := range local {
		seen[id] = true
	}
	for id := range repository {
		seen[id] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

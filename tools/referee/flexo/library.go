package flexo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/sysmlapi"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The library measurement publishes the whole bundled standard library to one
// project, as `sysml -stdlib -convert ttl -o flexo://...` does, reads it back
// through the element APIs, and then loads a small user model typing its parts
// by library ids into a second project to see whether those ids resolve: in the
// library project's listing, by direct read, and once the user model is
// re-converted to notation from what the service delivered.

// Project id prefixes of the library measurement's two projects.
const (
	libraryProject     = "opensysml-stdlib"
	libraryUserProject = "opensysml-stdlib-user"
)

// LibraryAnchors are the library elements a user model is most likely to type
// against; each is read directly from the published project by its normative id.
var LibraryAnchors = []string{
	"Base::Anything",
	"ScalarValues::Real",
	"ScalarValues::Integer",
	"ScalarValues::String",
	"ScalarValues::Boolean",
	"ISQBase::mass",
	"ISQBase::MassValue",
	"SI::kilogram",
	"Items::Item",
	"Parts::Part",
	"Actions::Action",
	"Requirements::RequirementCheck",
}

// MetaclassStat is one metaclass's fate: how many elements of it were written
// and how many the listing returned.
type MetaclassStat struct {
	Metaclass string
	Written   int
	Listed    int
}

// AnchorStat is one named library element's fate under direct read.
type AnchorStat struct {
	QualifiedName string
	ID            string
	Listed        bool
	Direct        string // "ok", or "refused(404)"
	Written       int
	Delivered     int
	Lost          []string
}

// TypingStat is one typing of the user model by a library id, as the service
// delivered it and as the library project resolves it.
type TypingStat struct {
	Feature       string // the typed feature's qualified name as written
	TypeID        string // the id the delivered FeatureTyping names
	Delivered     bool   // the user project's listing returned the typing with its type
	InLibrary     bool   // the library project's listing has an element of that id
	LibraryDirect string // "ok" or "refused(404)" for a direct read in the library project
	ResolvedName  string // the qualifiedName the library project delivers for it
	Notation      string // "same-id" when the delivered model, re-converted to notation and read again, types by this id; else what it types by
}

// UserModelReport is the user model's side of the measurement.
type UserModelReport struct {
	Fixture             string
	Accepted            bool
	Refusal             string
	Written             int
	Listed              int
	Typings             []TypingStat
	Notation            bool   // the delivered elements re-converted to notation
	NotationFailure     string // why they did not, when they did not
	ProjectUsage        string // "accepted" or the refusal of the ProjectUsage commit
	ProjectUsagesRead   int    // ProjectUsage elements the projectUsage endpoint returned
	ListedAfterUsage    int    // elements the listing returned once the usage was committed
	LibraryViaUsage     int    // library elements among them
	ProjectUsageMissing string // a reason the usage could not be measured, when one applies
}

// LibraryTimings are wall-clock observations of the run; the test logs them and
// the expectation file leaves them out.
type LibraryTimings struct {
	Convert time.Duration
	Load    time.Duration
	Listing time.Duration
}

// LibraryReport is a whole library measurement.
type LibraryReport struct {
	Files        int
	Graph        GraphStats
	APIJSONStats APIJSONStats
	Load         SideReport // Elements stays empty: a hundred thousand lines is no ratchet a reader adjudicates
	Metaclasses  []MetaclassStat
	Anchors      []AnchorStat
	User         UserModelReport
	Findings     []string
	Timings      LibraryTimings
}

// MeasureLibrary publishes the standard library and measures it as described
// above. The user model is the notation of a model typing parts by library
// elements; its ids are spelled as uuids, as the library's are.
func MeasureLibrary(ctx context.Context, c *Stack, userFixture string, userModel []byte) (*LibraryReport, error) {
	report := &LibraryReport{}

	started := time.Now()
	inputs, err := libraryInputs()
	if err != nil {
		return nil, err
	}
	report.Files = len(inputs)
	graph, err := convert.ModelGraph(inputs, convert.Options{ID: export.IDUUID})
	if err != nil {
		return nil, fmt.Errorf("convert the standard library to RDF: %w", err)
	}
	turtle := rdf.WriteTurtle(graph)
	report.Graph = graphStats(graph, turtle)
	elements, err := export.WriteAPIJSON(graph)
	if err != nil {
		return nil, fmt.Errorf("write the library's api element form: %w", err)
	}
	payloads := apiJSONElements(elements)
	report.APIJSONStats = APIJSONStats{Elements: len(payloads), Bytes: len(elements)}
	report.Timings.Convert = time.Since(started)

	if err := c.EnsureOrg(ctx); err != nil {
		return nil, fmt.Errorf("ensure org %s: %w", c.Config().Org, err)
	}

	written := writtenFromGraph(graph)
	side := SideReport{Name: "graph-load", Written: len(written)}
	projectID, err := uniqueProjectID(libraryProject)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateProject(ctx, projectID, projectID, BranchID); err != nil {
		return nil, fmt.Errorf("create project %s: %w", projectID, err)
	}
	started = time.Now()
	if err := c.LoadTurtle(ctx, projectID, BranchID, turtle, "OpenSysML standard library"); err != nil {
		side.Refusal = refusalReason(err, projectID)
		report.Load = side
		report.Findings = libraryFindings(report)
		return report, nil
	}
	report.Timings.Load = time.Since(started)
	side.Accepted = true

	commits, err := c.Commits(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list commits of %s: %w", projectID, err)
	}
	side.Commits = len(commits)
	started = time.Now()
	commit, listing, err := c.richestCommit(ctx, projectID, commits)
	if err != nil {
		return nil, err
	}
	report.Timings.Listing = time.Since(started)
	side.Pages, side.IgnoredPaging = listing.Responses, listing.IgnoredPaging

	byID := make(map[string]sysmlapi.Element, len(listing.Elements))
	for _, element := range listing.Elements {
		byID[element.ID()] = element
	}
	metaclasses := map[string]*MetaclassStat{}
	properties := map[string]*PropertyStat{}
	for _, id := range sortedIDs(written) {
		element := written[id]
		stat := metaclasses[element.metaclass]
		if stat == nil {
			stat = &MetaclassStat{Metaclass: element.metaclass}
			metaclasses[element.metaclass] = stat
		}
		stat.Written++
		read, listed := byID[id]
		if listed {
			side.Listed++
			stat.Listed++
		}
		measureProperties(&ElementStat{}, element, read, properties)
	}
	for _, element := range listing.Elements {
		if _, ok := written[element.ID()]; !ok {
			side.Extra++
		}
	}
	for _, name := range slices.Sorted(maps.Keys(metaclasses)) {
		report.Metaclasses = append(report.Metaclasses, *metaclasses[name])
	}
	for _, name := range slices.Sorted(maps.Keys(properties)) {
		side.Properties = append(side.Properties, *properties[name])
	}

	roots, err := c.Roots(ctx, projectID, commit)
	if err != nil {
		return nil, fmt.Errorf("read roots of %s: %w", projectID, err)
	}
	side.Roots, side.RootsInModel = len(roots), rootsInModel(written)
	report.Load = side

	ids := qualifiedIDs(payloads)
	for _, name := range LibraryAnchors {
		anchor := AnchorStat{QualifiedName: name, ID: ids[name]}
		if anchor.ID == "" {
			anchor.Direct = "not-in-library"
			report.Anchors = append(report.Anchors, anchor)
			continue
		}
		read, listed := byID[anchor.ID]
		anchor.Listed = listed
		direct, err := c.ElementByID(ctx, projectID, commit, anchor.ID)
		if err == nil {
			anchor.Direct = "ok"
			if !listed {
				read = direct
			}
		} else {
			anchor.Direct = fmt.Sprintf("refused(%d)", sysmlapi.Status(err))
		}
		if element := written[anchor.ID]; element != nil {
			stat := ElementStat{}
			measureProperties(&stat, element, read, map[string]*PropertyStat{})
			anchor.Written, anchor.Delivered, anchor.Lost = len(element.props), stat.Delivered, stat.Lost
		}
		report.Anchors = append(report.Anchors, anchor)
	}

	report.User, err = c.measureUserModel(ctx, userFixture, userModel, projectID, commit, byID)
	if err != nil {
		return nil, err
	}
	report.Findings = libraryFindings(report)
	return report, nil
}

// qualifiedIDs maps each element's qualifiedName to its id, from the api element form.
func qualifiedIDs(payloads []json.RawMessage) map[string]string {
	ids := map[string]string{}
	for _, payload := range payloads {
		var element struct {
			ID            string `json:"@id"`
			QualifiedName string `json:"qualifiedName"`
		}
		if json.Unmarshal(payload, &element) == nil && element.QualifiedName != "" {
			ids[element.QualifiedName] = element.ID
		}
	}
	return ids
}

// measureUserModel loads the user model into its own project and follows each of
// its typings into the library project, then re-converts what came back to
// notation and declares a ProjectUsage of the library.
func (c *Stack) measureUserModel(ctx context.Context, fixture string, model []byte,
	libraryProjectID, libraryCommit string, library map[string]sysmlapi.Element) (UserModelReport, error) {

	user := UserModelReport{Fixture: filepath.Base(fixture)}
	graph, err := convert.ModelGraph([]convert.Input{{Name: user.Fixture, Data: model}}, convert.Options{ID: export.IDUUID})
	if err != nil {
		return user, fmt.Errorf("convert %s to RDF: %w", fixture, err)
	}
	written := writtenFromGraph(graph)
	user.Written = len(written)

	projectID, err := uniqueProjectID(libraryUserProject)
	if err != nil {
		return user, err
	}
	if _, err := c.CreateProject(ctx, projectID, projectID, BranchID); err != nil {
		return user, fmt.Errorf("create project %s: %w", projectID, err)
	}
	if err := c.LoadTurtle(ctx, projectID, BranchID, rdf.WriteTurtle(graph), "OpenSysML library user model"); err != nil {
		user.Refusal = refusalReason(err, projectID)
		return user, nil
	}
	user.Accepted = true

	commits, err := c.Commits(ctx, projectID)
	if err != nil {
		return user, fmt.Errorf("list commits of %s: %w", projectID, err)
	}
	_, listing, err := c.richestCommit(ctx, projectID, commits)
	if err != nil {
		return user, err
	}
	delivered := map[string]sysmlapi.Element{}
	for _, element := range listing.Elements {
		delivered[element.ID()] = element
		if _, ok := written[element.ID()]; ok {
			user.Listed++
		}
	}

	user.Typings = c.followTypings(ctx, graph, written, delivered, libraryProjectID, libraryCommit, library)
	user.readBackNotation(listing.Elements)
	anyElement := ""
	for _, id := range sortedIDs(written) {
		if _, ok := delivered[id]; ok {
			anyElement = id
			break
		}
	}
	if err := c.declareLibraryUsage(ctx, &user, projectID, libraryProjectID, libraryCommit, anyElement, library); err != nil {
		return user, err
	}
	return user, nil
}

// followTypings reports every typing written, followed by the id the service
// delivered for it and whether the library project answers for that id directly.
func (c *Stack) followTypings(ctx context.Context, graph *rdf.Graph, written map[string]*writtenElement, delivered map[string]sysmlapi.Element,
	libraryProjectID, libraryCommit string, library map[string]sysmlapi.Element) []TypingStat {
	var typings []TypingStat
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.SysML+"type" || !triple.Subject.IsIRI() || !triple.Object.IsIRI() {
			continue
		}
		typing := written[rdf.LocalName(triple.Subject.Value)]
		if typing == nil || typing.metaclass != "FeatureTyping" {
			continue
		}
		stat := TypingStat{Feature: typedFeatureName(graph, triple.Subject), TypeID: rdf.LocalName(triple.Object.Value)}
		if read, ok := delivered[typing.id]; ok {
			if id := referenceID(read["type"]); id != "" {
				stat.Delivered, stat.TypeID = true, id
			}
		}
		_, stat.InLibrary = library[stat.TypeID]
		direct, err := c.ElementByID(ctx, libraryProjectID, libraryCommit, stat.TypeID)
		if err == nil {
			stat.LibraryDirect = "ok"
			_ = json.Unmarshal(direct["qualifiedName"], &stat.ResolvedName)
		} else {
			stat.LibraryDirect = fmt.Sprintf("refused(%d)", sysmlapi.Status(err))
		}
		typings = append(typings, stat)
	}
	sort.Slice(typings, func(i, j int) bool { return typings[i].Feature < typings[j].Feature })
	return typings
}

// readBackNotation re-converts what the service delivered to notation and records which
// typings read back by the same ids (the pilot's, so they must read as the library names).
func (user *UserModelReport) readBackNotation(elements []sysmlapi.Element) {
	readback, err := sysmlapi.ElementsGraph(elements)
	if err != nil {
		user.NotationFailure = err.Error()
		return
	}
	notation, err := convert.FromGraph(readback, convert.FormatSysML)
	if err != nil {
		user.NotationFailure = err.Error()
		return
	}
	reread, err := convert.ModelGraph([]convert.Input{{Name: user.Fixture, Data: notation}}, convert.Options{ID: export.IDUUID})
	if err != nil {
		user.NotationFailure = "the notation does not read back: " + err.Error()
		return
	}
	user.Notation = true
	retyped := typingTargets(reread)
	for i := range user.Typings {
		t := &user.Typings[i]
		switch id := retyped[t.Feature]; {
		case id == t.TypeID:
			t.Notation = "same-id"
		case id == "":
			t.Notation = "absent"
		default:
			t.Notation = "other-id:" + id
		}
	}
}

// declareLibraryUsage commits a ProjectUsage of the library at the commit it was read
// at, asks it back through anyElement (a delivered user element) and lists what it reaches.
func (c *Stack) declareLibraryUsage(ctx context.Context, user *UserModelReport, projectID, libraryProjectID, libraryCommit, anyElement string,
	library map[string]sysmlapi.Element) error {
	usage := map[string]any{
		"@type": "Commit",
		"change": []map[string]any{{
			"@type": "DataVersion",
			"payload": map[string]any{
				"@id":         randomUUID(),
				"@type":       "ProjectUsage",
				"usedProject": map[string]string{"@id": libraryProjectID},
				"usedCommit":  map[string]string{"@id": libraryCommit},
			},
		}},
	}
	body, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	usageCommit, err := c.PostChanges(ctx, projectID, BranchID, body)
	if err != nil {
		user.ProjectUsage = refusalReason(err, projectID)
		return nil
	}
	user.ProjectUsage = "accepted"
	if anyElement == "" {
		user.ProjectUsageMissing = "no user element to ask the usage of"
		return nil
	}
	target := fmt.Sprintf("%s/projects/%s/commits/%s/elements/%s/projectUsage",
		c.API().BaseURL(), projectID, usageCommit.ID, anyElement)
	raw, _, err := c.API().Do(ctx, http.MethodGet, target, nil, "", map[string]string{"Accept": "application/json"})
	if err != nil {
		user.ProjectUsageMissing = refusalReason(err, projectID)
	} else {
		var usages []json.RawMessage
		if err := json.Unmarshal(raw, &usages); err != nil {
			user.ProjectUsageMissing = "the projectUsage endpoint answered no JSON array"
		}
		user.ProjectUsagesRead = len(usages)
	}
	after, err := c.Elements(ctx, projectID, usageCommit.ID, PageSize)
	if err != nil {
		return fmt.Errorf("list the elements of %s after its usage: %w", projectID, err)
	}
	user.ListedAfterUsage = len(after.Elements)
	for _, element := range after.Elements {
		if _, ok := library[element.ID()]; ok {
			user.LibraryViaUsage++
		}
	}
	return nil
}

// randomUUID mints a version 4 uuid for the ProjectUsage element, the shape the
// service's id rules accept.
func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// typedFeatureName is the qualified name of the feature a FeatureTyping types,
// as the graph writes it, or the typing's own id.
func typedFeatureName(graph *rdf.Graph, typing rdf.Term) string {
	for _, triple := range graph.Triples() {
		if triple.Subject != typing || triple.Predicate.Value != rdf.SysML+"typedFeature" {
			continue
		}
		for _, named := range graph.Triples() {
			if named.Subject == triple.Object && named.Predicate.Value == rdf.SysML+"qualifiedName" {
				return named.Object.Value
			}
		}
	}
	return rdf.LocalName(typing.Value)
}

// referenceID is the @id of a delivered {"@id": ...} value, or empty.
func referenceID(raw json.RawMessage) string {
	var ref struct {
		ID string `json:"@id"`
	}
	if json.Unmarshal(raw, &ref) != nil {
		return ""
	}
	return ref.ID
}

// typingTargets maps each typed feature's qualified name to the id its
// FeatureTyping names, as the graph writes them.
func typingTargets(graph *rdf.Graph) map[string]string {
	targets := map[string]string{}
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.SysML+"type" || !triple.Subject.IsIRI() || !triple.Object.IsIRI() {
			continue
		}
		if !graph.HasProperty(triple.Subject, rdf.SysML+"typedFeature") {
			continue
		}
		targets[typedFeatureName(graph, triple.Subject)] = rdf.LocalName(triple.Object.Value)
	}
	return targets
}

// Text renders the library report as its expectation file.
func (r *LibraryReport) Text(header string) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n[library]\nfiles\t%d\nsubjects\t%d\ntriples\t%d\nbytes\t%d\n",
		r.Files, r.Graph.Subjects, r.Graph.Triples, r.Graph.Bytes)
	for _, ns := range r.Graph.ByNamespace {
		fmt.Fprintf(&b, "triples.%s\t%d\n", ns.Property, ns.Written)
	}
	fmt.Fprintf(&b, "\n[api-json]\nelements\t%d\nbytes\t%d\n", r.APIJSONStats.Elements, r.APIJSONStats.Bytes)

	s := r.Load
	fmt.Fprintf(&b, "\n[%s]\naccepted\t%s\n", s.Name, yesNo(s.Accepted))
	if s.Refusal != "" {
		fmt.Fprintf(&b, "refusal\t%s\n", s.Refusal)
	}
	fmt.Fprintf(&b, "commits\t%d\nelements.written\t%d\nelements.listed\t%d\n"+
		"elements.unexpected\t%d\nlisting.responses\t%d\nlisting.paging-ignored\t%s\n"+
		"roots.reported\t%d\nroots.in-model\t%d\n",
		s.Commits, s.Written, s.Listed, s.Extra, s.Pages, yesNo(s.IgnoredPaging), s.Roots, s.RootsInModel)

	fmt.Fprintf(&b, "\n[%s.metaclasses]\n", s.Name)
	for _, m := range r.Metaclasses {
		fmt.Fprintf(&b, "%s\twritten=%d\tlisted=%d\n", m.Metaclass, m.Written, m.Listed)
	}
	fmt.Fprintf(&b, "\n[%s.properties]\n", s.Name)
	for _, p := range s.Properties {
		fmt.Fprintf(&b, "%s\twritten=%d\tdelivered=%d\tmulti-valued=%d/%d\n",
			p.Property, p.Written, p.Delivered, p.MultiDelivered, p.MultiValued)
	}

	b.WriteString("\n[anchors]\n")
	for _, a := range r.Anchors {
		fmt.Fprintf(&b, "%s\tid=%s\tlisted=%s\tdirect=%s\tproperties=%d/%d",
			a.QualifiedName, orNone(a.ID), yesNo(a.Listed), a.Direct, a.Delivered, a.Written)
		if len(a.Lost) > 0 {
			lost := append([]string(nil), a.Lost...)
			sort.Strings(lost)
			fmt.Fprintf(&b, "\tlost=%s", strings.Join(lost, ","))
		}
		b.WriteString("\n")
	}

	u := r.User
	fmt.Fprintf(&b, "\n[user-model]\nfixture\t%s\naccepted\t%s\n", u.Fixture, yesNo(u.Accepted))
	if u.Refusal != "" {
		fmt.Fprintf(&b, "refusal\t%s\n", u.Refusal)
	}
	fmt.Fprintf(&b, "elements.written\t%d\nelements.listed\t%d\n", u.Written, u.Listed)
	for _, t := range u.Typings {
		fmt.Fprintf(&b, "typing\t%s\ttype=%s\tdelivered=%s\tin-library=%s\tlibrary-direct=%s\tresolves=%s\tnotation=%s\n",
			t.Feature, t.TypeID, yesNo(t.Delivered), yesNo(t.InLibrary), t.LibraryDirect, orNone(t.ResolvedName), orNone(t.Notation))
	}
	fmt.Fprintf(&b, "notation\t%s\n", yesNo(u.Notation))
	if u.NotationFailure != "" {
		fmt.Fprintf(&b, "notation.failure\t%s\n", u.NotationFailure)
	}
	if u.ProjectUsage != "" {
		fmt.Fprintf(&b, "project-usage\t%s\n", u.ProjectUsage)
	}
	if u.ProjectUsage == "accepted" {
		if u.ProjectUsageMissing != "" {
			fmt.Fprintf(&b, "project-usage.unmeasured\t%s\n", u.ProjectUsageMissing)
		} else {
			fmt.Fprintf(&b, "project-usage.read\t%d\nelements.listed-after-usage\t%d\nelements.library-via-usage\t%d\n",
				u.ProjectUsagesRead, u.ListedAfterUsage, u.LibraryViaUsage)
		}
	}

	b.WriteString("\n[findings]\n")
	for _, finding := range r.Findings {
		b.WriteString(finding)
		b.WriteString("\n")
	}
	return b.String()
}

// libraryFindings states what the measurement found, one line each.
func libraryFindings(r *LibraryReport) []string {
	var out []string
	s := r.Load
	if !s.Accepted {
		return append(out, "the library graph was refused: "+s.Refusal)
	}
	if s.Listed == s.Written && s.Extra == 0 {
		out = append(out, fmt.Sprintf("every one of the %d library elements is listed, and nothing else", s.Written))
	} else {
		out = append(out, fmt.Sprintf("%d of %d library elements are listed, with %d unexpected", s.Listed, s.Written, s.Extra))
	}
	var dropped, partial []string
	for _, p := range s.Properties {
		switch {
		case p.Delivered == 0:
			dropped = append(dropped, p.Property)
		case p.Delivered < p.Written:
			partial = append(partial, fmt.Sprintf("%s (%d of %d)", p.Property, p.Delivered, p.Written))
		}
	}
	if len(dropped) > 0 {
		out = append(out, "properties the read path drops entirely: "+strings.Join(dropped, ", "))
	}
	if len(partial) > 0 {
		out = append(out, "properties delivered on some elements only: "+strings.Join(partial, ", "))
	}
	for _, a := range r.Anchors {
		if a.Direct != "ok" {
			out = append(out, fmt.Sprintf("%s is not readable directly: %s", a.QualifiedName, a.Direct))
		}
	}
	return append(out, userModelFindings(r.User)...)
}

// userModelFindings states what loading the user model against the library found.
func userModelFindings(u UserModelReport) []string {
	if !u.Accepted {
		return []string{"the user model was refused: " + u.Refusal}
	}
	var out []string
	resolved, byLibrary := 0, 0
	for _, t := range u.Typings {
		if !t.InLibrary {
			continue
		}
		byLibrary++
		if t.Delivered && t.LibraryDirect == "ok" && t.ResolvedName != "" {
			resolved++
		}
	}
	out = append(out, fmt.Sprintf("%d of %d typings of the user model name library elements, and %d of those resolve in the published library by id", byLibrary, len(u.Typings), resolved))
	for _, t := range u.Typings {
		if !t.InLibrary && t.LibraryDirect == "ok" {
			out = append(out, fmt.Sprintf("a direct read in the library project answers for %s, an id the library does not list: the read is not scoped to the project", t.TypeID))
		}
	}
	kept := 0
	for _, t := range u.Typings {
		if t.InLibrary && t.Notation == "same-id" {
			kept++
		}
	}
	if u.Notation {
		out = append(out, fmt.Sprintf("the delivered user model re-converts to notation that reads back typing %d of %d library elements by the same ids", kept, byLibrary))
	} else {
		out = append(out, "the delivered user model does not re-convert to notation: "+u.NotationFailure)
	}
	if u.ProjectUsage != "accepted" {
		out = append(out, "a ProjectUsage of the library is refused: "+u.ProjectUsage)
	} else if u.ProjectUsageMissing == "" {
		out = append(out, fmt.Sprintf("a ProjectUsage of the library is accepted; the usage endpoint returns %d, and the listing walks into %d library elements through it", u.ProjectUsagesRead, u.LibraryViaUsage))
	}
	return out
}

// libraryInputs is the bundled standard library as one model's conversion inputs.
func libraryInputs() ([]convert.Input, error) {
	files, err := libs.BundledModelFiles()
	if err != nil {
		return nil, err
	}
	inputs := make([]convert.Input, 0, len(files))
	for _, file := range files {
		inputs = append(inputs, convert.Input{Name: file.Name, Data: file.Data})
	}
	return inputs, nil
}

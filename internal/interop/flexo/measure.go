package flexo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/convert"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// One run writes the same model into the stack twice — as the Turtle this
// project produces, and as the SysML v2 JSON the service's own commit path
// takes — and reads both back through the SysML v2 element APIs. The second
// write is the ground truth: it shows what the read path can carry at all, so
// the first write's losses can be attributed to the graph rather than to the
// service.

// BranchID is the default branch the harness asks for when it creates a
// project, so the graph endpoint's path is known rather than generated.
const BranchID = "main"

// PageSize is the page the harness asks the element listing for. It is smaller
// than the fixture, so a service that pages answers in more than one response.
const PageSize = 10

// Project id prefixes of the two sides. Each run mints fresh ids: the service's
// project delete is a soft delete that leaves the Layer 1 branch behind, so
// reusing an id makes the second run conflict. No id reaches the report.
const (
	graphProject     = "opensysml-graph-load"
	referenceProject = "opensysml-json-commit"
)

// uniqueProjectID suffixes a prefix with random hex the service's id rules
// accept.
func uniqueProjectID(prefix string) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("mint a project id: %w", err)
	}
	return prefix + "-" + hex.EncodeToString(suffix), nil
}

// property is one written property: the shape a faithful read would deliver,
// and how many values were written for it.
type property struct {
	kind  string // "reference", "literal", "array", "object" or "null"
	count int    // values written; more than one is multi-valued
}

// writtenElement is one element as this run wrote it, on either side.
type writtenElement struct {
	id        string
	metaclass string
	props     map[string]property // property label to what was written, type excluded
}

// sortedIDs returns the ids of a written payload in a stable order.
func sortedIDs(written map[string]*writtenElement) []string {
	ids := make([]string, 0, len(written))
	for id := range written {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Measure loads model into the stack as Turtle and as JSON changes, reads both
// back, and reports what each side delivered. The reference argument is the
// JSON commit request; only its "change" array is posted.
func Measure(ctx context.Context, c *Client, fixture string, model, reference []byte) (*Report, error) {
	graph, err := convert.SysMLToRDF(filepath.Base(fixture), model)
	if err != nil {
		return nil, fmt.Errorf("convert %s to RDF: %w", fixture, err)
	}
	turtle := rdf.WriteTurtle(graph)

	report := &Report{
		Fixture: filepath.Base(fixture),
		Graph:   graphStats(graph, turtle),
	}

	if err := c.EnsureOrg(ctx); err != nil {
		return nil, fmt.Errorf("ensure org %s: %w", c.Config().Org, err)
	}

	graphWritten := writtenFromGraph(graph)
	report.Load, err = c.measureSide(ctx, "graph-load", graphProject, graphWritten,
		func(project string) error {
			return c.LoadTurtle(ctx, project, BranchID, turtle, "OpenSysML graph load")
		})
	if err != nil {
		return report, err
	}

	changes, referenceWritten, err := referencePayload(reference)
	if err != nil {
		return report, err
	}
	report.Reference, err = c.measureSide(ctx, "json-commit", referenceProject, referenceWritten,
		func(project string) error { _, err := c.PostChanges(ctx, project, "", changes); return err })
	if err != nil {
		return report, err
	}

	report.Findings = findings(report, graphWritten)
	return report, nil
}

// graphStats counts the Turtle before any service sees it, per namespace, so the
// report says what share of the graph is standard vocabulary.
func graphStats(graph *rdf.Graph, turtle []byte) GraphStats {
	stats := GraphStats{Subjects: len(graph.Subjects()), Triples: graph.Len(), Bytes: len(turtle)}
	perNamespace := map[string]int{}
	for _, triple := range graph.Triples() {
		perNamespace[prefixOf(triple.Predicate.Value)]++
	}
	for _, prefix := range sortedKeys(perNamespace) {
		stats.ByNamespace = append(stats.ByNamespace, PropertyStat{Property: prefix, Written: perNamespace[prefix]})
	}
	return stats
}

// prefixOf labels a predicate IRI by the namespace it belongs to, which is what
// decides whether the service reads it at all.
func prefixOf(iri string) string {
	switch {
	case iri == rdf.RDFType:
		return "rdf"
	case strings.HasPrefix(iri, rdf.SysML):
		return "sysml"
	case strings.HasPrefix(iri, rdf.OpenSysML):
		return "sysx"
	case rdf.IsAnnotationJSON(iri):
		return "json"
	default:
		return "other"
	}
}

// label names a predicate the way the report records it: prefixed, so a dropped
// property's namespace is visible in the diff.
func label(iri string) string { return prefixOf(iri) + ":" + rdf.LocalName(iri) }

// writtenFromGraph reduces the graph to the payload view the comparison uses:
// one entry per subject, its metaclass, and its properties with their values'
// kinds. Every subject of the graph is an element as far as the service's
// element listing is concerned, expression nodes included.
func writtenFromGraph(graph *rdf.Graph) map[string]*writtenElement {
	written := make(map[string]*writtenElement)
	for _, triple := range graph.Triples() {
		if !triple.Subject.IsIRI() {
			continue
		}
		id := rdf.LocalName(triple.Subject.Value)
		element := written[id]
		if element == nil {
			element = &writtenElement{id: id, props: map[string]property{}}
			written[id] = element
		}
		if triple.Predicate.Value == rdf.RDFType {
			element.metaclass = rdf.LocalName(triple.Object.Value)
			continue
		}
		if rdf.IsAnnotationJSON(triple.Predicate.Value) {
			// The array spelling of a sysml: collection, not a property of its own.
			continue
		}
		kind := "literal"
		if triple.Object.IsIRI() {
			kind = "reference"
		}
		name := label(triple.Predicate.Value)
		was := element.props[name]
		if was.count > 0 {
			// Several triples on one predicate: a faithful read delivers an array.
			kind = "array"
		}
		element.props[name] = property{kind: kind, count: was.count + 1}
	}
	return written
}

// referencePayload extracts the commit request the harness posts from the
// fixture, and the payload view of it. The fixture's documentation keys are not
// posted: only its change array is.
func referencePayload(fixture []byte) ([]byte, map[string]*writtenElement, error) {
	var parsed struct {
		Change []struct {
			Payload map[string]json.RawMessage `json:"payload"`
		} `json:"change"`
	}
	if err := json.Unmarshal(fixture, &parsed); err != nil {
		return nil, nil, fmt.Errorf("decode reference changes: %w", err)
	}

	written := make(map[string]*writtenElement, len(parsed.Change))
	for _, change := range parsed.Change {
		element := &writtenElement{props: map[string]property{}}
		for name, raw := range change.Payload {
			switch name {
			case "@id":
				_ = json.Unmarshal(raw, &element.id)
			case "@type":
				_ = json.Unmarshal(raw, &element.metaclass)
			default:
				element.props[name] = jsonProperty(raw)
			}
		}
		if element.id == "" {
			return nil, nil, fmt.Errorf("reference change without @id")
		}
		written[element.id] = element
	}

	var request struct {
		Change json.RawMessage `json:"change"`
	}
	if err := json.Unmarshal(fixture, &request); err != nil {
		return nil, nil, err
	}
	changes, err := json.Marshal(map[string]any{"@type": "Commit", "change": request.Change})
	if err != nil {
		return nil, nil, err
	}
	return changes, written, nil
}

// jsonProperty reduces one posted JSON value to a shape and an arity, so a
// posted collection counts as multi-valued the way repeated triples do.
func jsonProperty(raw json.RawMessage) property {
	kind := deliveredKind(raw)
	if kind != "array" {
		return property{kind: kind, count: 1}
	}
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return property{kind: kind, count: 1}
	}
	return property{kind: kind, count: len(members)}
}

// measureSide writes one payload into a fresh project and reads it back through
// the element listing, the direct element read and the roots endpoint.
func (c *Client) measureSide(ctx context.Context, name, projectID string,
	written map[string]*writtenElement, write func(project string) error) (SideReport, error) {

	side := SideReport{Name: name, Written: len(written)}

	projectID, err := uniqueProjectID(projectID)
	if err != nil {
		return side, err
	}
	if _, err := c.CreateProject(ctx, projectID, projectID, BranchID); err != nil {
		return side, fmt.Errorf("create project %s: %w", projectID, err)
	}

	if err := write(projectID); err != nil {
		// A refused payload is a result, not a harness failure: record and stop here.
		side.Elements = append(side.Elements, ElementStat{
			ID:     "<payload>",
			Direct: fmt.Sprintf("write-refused(%d)", Status(err)),
		})
		return side, nil
	}
	side.Accepted = true

	// Leaving each run's project in place keeps the stack inspectable; the skill
	// documents restarting the stack to clear them.
	commits, err := c.Commits(ctx, projectID)
	if err != nil {
		return side, fmt.Errorf("list commits of %s: %w", projectID, err)
	}
	side.Commits = len(commits)

	commit, listing, err := c.richestCommit(ctx, projectID, commits)
	if err != nil {
		return side, err
	}
	delivered := listing.Elements
	side.Pages = listing.Responses
	side.IgnoredPaging = listing.IgnoredPaging

	byID := make(map[string]Element, len(delivered))
	for _, element := range delivered {
		byID[element.ID()] = element
	}
	for id := range written {
		if _, ok := byID[id]; ok {
			side.Listed++
		}
	}

	roots, err := c.Roots(ctx, projectID, commit)
	if err != nil {
		return side, fmt.Errorf("read roots of %s: %w", projectID, err)
	}
	side.Roots = len(roots)
	side.RootsInModel = rootsInModel(written)

	properties := map[string]*PropertyStat{}
	for _, id := range sortedIDs(written) {
		element := written[id]
		stat := ElementStat{ID: id, Type: element.metaclass, Written: len(element.props)}

		read, listed := byID[id]
		stat.Listed = listed

		direct, err := c.ElementByID(ctx, projectID, commit, id)
		switch {
		case err == nil:
			stat.Direct = "ok"
			side.Direct++
			if !listed {
				read = direct
			}
		default:
			stat.Direct = fmt.Sprintf("refused(%d)", Status(err))
		}

		measureProperties(&stat, element, read, properties)
		side.Elements = append(side.Elements, stat)
	}

	for _, element := range delivered {
		if _, ok := written[element.ID()]; !ok {
			side.Extra++
		}
	}

	for _, property := range sortedKeys(properties) {
		side.Properties = append(side.Properties, *properties[property])
	}
	return side, nil
}

// measureProperties records, for one element, which written properties came back
// and which came back in a different shape, accumulating the per-property totals.
func measureProperties(stat *ElementStat, element *writtenElement, read Element, properties map[string]*PropertyStat) {
	for _, name := range sortedKeys(element.props) {
		written := element.props[name]
		total := properties[name]
		if total == nil {
			total = &PropertyStat{Property: name}
			properties[name] = total
		}
		total.Written++
		if written.count > 1 {
			total.MultiValued++
			if _, ok := read[localName(name)]; ok {
				total.MultiDelivered++
			}
		}

		raw, ok := read[localName(name)]
		if !ok {
			stat.Lost = append(stat.Lost, name)
			continue
		}
		stat.Delivered++
		total.Delivered++

		if want, got := written.kind, deliveredKind(raw); got != want {
			stat.Shape = append(stat.Shape, fmt.Sprintf("%s:%s-as-%s", name, want, got))
		}
	}
}

// localName strips a report label's prefix, giving the JSON key the read path
// would use for it.
func localName(property string) string {
	if cut := strings.Index(property, ":"); cut >= 0 {
		return property[cut+1:]
	}
	return property
}

// deliveredKind classifies a delivered JSON value the same way a written one is
// classified, so the two can be compared.
func deliveredKind(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	switch {
	case strings.HasPrefix(trimmed, "["):
		return "array"
	case strings.HasPrefix(trimmed, "{"):
		if strings.Contains(trimmed, `"@id"`) {
			return "reference"
		}
		return "object"
	case trimmed == "null":
		return "null"
	default:
		return "literal"
	}
}

// richestCommit picks the commit whose element listing returns the most
// elements. The write's commit id is server-generated and the list also holds
// the project's initial commit, so the payload is found rather than assumed.
func (c *Client) richestCommit(ctx context.Context, project string, commits []Commit) (string, Listing, error) {
	best := ""
	var bestListing Listing
	for _, commit := range commits {
		listing, err := c.Elements(ctx, project, commit.ID, PageSize)
		if err != nil {
			return "", Listing{}, fmt.Errorf("list elements of %s commit: %w", project, err)
		}
		if best == "" || len(listing.Elements) > len(bestListing.Elements) {
			best, bestListing = commit.ID, listing
		}
	}
	if best == "" {
		return "", Listing{}, fmt.Errorf("project %s has no commit to read", project)
	}
	return best, bestListing, nil
}

// rootsInModel counts the elements the payload itself gives no owner, which is
// what the roots endpoint would return if it could see the payload's ownership.
// An explicit null owner is no owner.
func rootsInModel(written map[string]*writtenElement) int {
	roots := 0
	for _, element := range written {
		owned := false
		for name, written := range element.props {
			switch localName(name) {
			case "owner", "owningRelatedElement", "owningNamespace":
				owned = owned || written.kind != "null"
			}
		}
		if !owned {
			roots++
		}
	}
	return roots
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

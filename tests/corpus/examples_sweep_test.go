package corpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/modelsync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/corpussubset"
)

// sweepFixtures holds the export fixtures the settled-collections property
// covers besides the examples.
const sweepFixtures = "../export/testdata"

// sweepModel is one model of the sweep, loaded once: read, parsed, and exported
// through convert's single-document path, with that graph written both ways.
type sweepModel struct {
	rel     string // slash path relative to examples/, or the fixture's path
	path    string
	fixture bool // an export fixture: settled collections only
	wanted  bool // iterated by the subset; otherwise only parsed, for its directory's model

	doc        export.ModelDocument
	parsed     bool // no parser diagnostic at all
	graph      *rdf.Graph
	graphErr   error // convert's hop-1 refusal: a syntax error, or the export's
	turtle     []byte
	apiJSON    []byte
	apiJSONErr error
}

// load reads, parses and exports the model as convert.Convert's hop 1 does.
func (m *sweepModel) load() error {
	src, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	file := source.New(m.rel, src)
	p := parser.New(file)
	root := p.ParseFile()
	m.doc = export.ModelDocument{File: file, Root: root}
	m.parsed = len(p.Diagnostics) == 0
	if !m.wanted {
		return nil
	}
	if syntax := convert.SyntaxErrorOf(m.rel, file, p.Diagnostics); syntax != nil {
		m.graphErr = syntax
		return nil
	}
	m.graph, m.graphErr = export.ToRDFWith(file, root, export.IDQualifiedName)
	if m.graphErr != nil {
		return nil
	}
	m.turtle = rdf.WriteTurtle(m.graph)
	m.apiJSON, m.apiJSONErr = export.WriteAPIJSON(m.graph)
	return nil
}

// apiJSONHop1Err is what convert.Convert to api-json would have refused with.
func (m *sweepModel) apiJSONHop1Err() error {
	if m.graphErr != nil {
		return m.graphErr
	}
	return m.apiJSONErr
}

// settledCandidate says whether the model takes the settled-collections property:
// a SysML document that parsed clean, as tests/export's sweep selected them.
func (m *sweepModel) settledCandidate() bool {
	return m.parsed && strings.HasSuffix(m.path, ".sysml")
}

// sweepTally is what one model added to the sweep's per-property counts.
type sweepTally struct {
	settled, agreementExamples, converted, elements, pairs, dropped int
}

// sweepCounts are the per-property tallies the sweep logs and asserts on.
type sweepCounts struct {
	mu    sync.Mutex
	total sweepTally
}

func (c *sweepCounts) add(tally sweepTally) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total.settled += tally.settled
	c.total.agreementExamples += tally.agreementExamples
	c.total.converted += tally.converted
	c.total.elements += tally.elements
	c.total.pairs += tally.pairs
	c.total.dropped += tally.dropped
}

// sweepDirectory is one directory's SysML documents, checked together as one
// model once every one of them has been loaded.
type sweepDirectory struct {
	models  []*sweepModel
	pending atomic.Int32
	broken  atomic.Bool // a document did not parse clean, so the directory is not a model
}

// TestExamplesSweep loads every model under examples/ once and checks every per-model property
// on that load: both round-trip ratchets, settled collections, the gRPC agreement, modelsync's read-back.
func TestExamplesSweep(t *testing.T) {
	subset := corpussubset.Must(t, "../..")
	corpus, totals, ratchet := corpusRoundTripFiles(t)
	if !ratchet && (*updateCorpusRoundTrip || *updateAPIJSONRoundTrip) {
		t.Fatal("re-recording the round-trip expectations needs every corpus downloaded")
	}
	files := subset.Filter(corpus)
	models := make([]*sweepModel, 0, len(corpus))
	for _, rel := range corpus {
		models = append(models, &sweepModel{rel: rel, path: filepath.Join(corpusRoundTripExamples, filepath.FromSlash(rel)), wanted: subset.Wants(rel)})
	}
	models = append(models, sweepFixtureModels(t)...)
	models = sweepSubsetModels(models)
	directories := sweepDirectoriesOf(models)
	if !subset.Whole() {
		t.Logf("%s=%s: %d of %d examples, %d models", corpussubset.EnvVar, subset.Name(), len(files), len(corpus), len(models))
	}

	// An empty semantic cache, as on a fresh checkout and in CI.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	turtle := make([]string, len(models))
	apiJSON := make([]string, len(models))
	failures := make([]error, len(models))
	var counts sweepCounts
	ctx := context.Background()

	// A directory is checked as soon as its last document is loaded, in one of the slots the
	// per-model workers load in: -parallel bounds the models in hand, a whole-directory model among them.
	workers := sweepSlots()
	slots := make(chan struct{}, workers)
	for range workers {
		slots <- struct{}{}
	}
	ready := make(chan *sweepDirectory, len(directories))
	var checking sync.WaitGroup
	checking.Add(1)
	go func() {
		defer checking.Done()
		for d := range ready {
			<-slots
			settled := d.check(t)
			slots <- struct{}{}
			if settled {
				counts.add(sweepTally{settled: 1})
			}
		}
	}()

	next := make(chan int)
	var loading sync.WaitGroup
	for w := 0; w < workers; w++ {
		// Each worker's own service, caching the one model it has in hand: a shared LRU could
		// evict a worker's model between its RPCs, and a larger one would hold models already checked.
		srv, err := grpc.NewService(1, "test")
		if err != nil {
			t.Fatal(err)
		}
		loading.Add(1)
		go func() {
			defer loading.Done()
			for i := range next {
				m := models[i]
				<-slots
				if err := m.load(); err != nil {
					failures[i] = err
				} else if m.wanted {
					if ratchet && !m.fixture {
						turtle[i], failures[i] = corpusRoundTripVerdict(m.rel, m.turtle, m.graphErr)
						if failures[i] == nil {
							apiJSON[i], failures[i] = apiJSONRoundTripVerdict(m.rel, m.apiJSON, m.apiJSONHop1Err())
						}
					}
					counts.add(sweepProperties(t, ctx, srv, m))
				}
				slots <- struct{}{}
				if !strings.HasSuffix(m.path, ".sysml") {
					m.doc = export.ModelDocument{}
					continue
				}
				d := directories[filepath.Dir(m.path)]
				if !m.parsed {
					d.broken.Store(true)
				}
				if d.pending.Add(-1) == 0 {
					ready <- d
				}
			}
		}()
	}
	for _, i := range sweepOrder(models, directories) {
		next <- i
	}
	close(next)
	loading.Wait()
	close(ready)
	checking.Wait()

	gotTurtle := make(map[string]string, len(files))
	gotAPIJSON := make(map[string]string, len(files))
	for i, m := range models {
		switch {
		case failures[i] != nil:
			t.Errorf("%s: %v", m.rel, failures[i])
		case m.wanted && !m.fixture:
			gotTurtle[m.rel] = turtle[i]
			gotAPIJSON[m.rel] = apiJSON[i]
		}
	}

	total := counts.total
	t.Logf("%d models checked", total.settled)
	if total.settled < 50 {
		t.Errorf("only %d models converted; the fixtures moved", total.settled)
	}
	if total.converted == 0 || total.elements == 0 {
		t.Errorf("no example converts to api-json (%d models, %d elements)", total.converted, total.elements)
	}
	t.Logf("%d of %d examples convert; %d elements, %d (element, property) pairs agree",
		total.converted, total.agreementExamples, total.elements, total.pairs)
	if total.dropped == 0 {
		t.Error("no example has a derived property to leave out")
	}

	if !ratchet {
		t.Log("round-trip ratchets not checked: a corpus is absent")
		return
	}
	corpusRoundTripGate.check(t, subset, corpus, files, totals, gotTurtle)
	apiJSONRoundTripGate.check(t, subset, corpus, files, totals, gotAPIJSON)
}

// sweepSubsetModels keeps the models the subset iterates and, as tests/export's sweep did, every
// SysML document of a directory one of them is in, so the directory is still checked whole.
func sweepSubsetModels(models []*sweepModel) []*sweepModel {
	directories := map[string]bool{}
	for _, m := range models {
		if m.wanted && strings.HasSuffix(m.path, ".sysml") {
			directories[filepath.Dir(m.path)] = true
		}
	}
	kept := models[:0]
	for _, m := range models {
		if m.wanted || (strings.HasSuffix(m.path, ".sysml") && directories[filepath.Dir(m.path)]) {
			kept = append(kept, m)
		}
	}
	return kept
}

// sweepProperties runs the per-model properties on one loaded model (failures go to t, safe from a
// worker); settled collections come last, over their own export, once the hop-1 graph is released.
func sweepProperties(t *testing.T, ctx context.Context, srv *grpc.Service, m *sweepModel) (tally sweepTally) {
	if !m.fixture {
		// The downloaded pilot corpora have gates of their own.
		if !strings.HasPrefix(m.rel, "pilot-corpora/") {
			tally.agreementExamples = 1
			if m.apiJSONHop1Err() == nil {
				tally.elements, tally.pairs = elementPropertiesAgree(t, ctx, srv, m)
				tally.converted = 1
			}
		}
		if m.graphErr == nil && !strings.Contains(m.rel, "/") && strings.HasSuffix(m.rel, ".sysml") {
			var err error
			if tally.dropped, err = withoutDerivedReadsBack(m.graph); err != nil {
				t.Errorf("%s: %v", m.rel, err)
			}
		}
	}
	m.graph, m.turtle, m.apiJSON = nil, nil, nil
	if m.settledCandidate() {
		// The settled property is over the model path, which vouches for its collections.
		if graph, err := export.ModelToRDFWith([]export.ModelDocument{m.doc}, export.IDQualifiedName); err == nil {
			if err := settledCollectionsHold(graph); err != nil {
				t.Errorf("%s: %v", m.rel, err)
			} else {
				tally.settled = 1
			}
		}
	}
	return tally
}

// sweepFixtureModels are the SysML export fixtures, which only the
// settled-collections property covers.
func sweepFixtureModels(t *testing.T) []*sweepModel {
	t.Helper()
	var models []*sweepModel
	err := filepath.WalkDir(sweepFixtures, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".sysml") {
			models = append(models, &sweepModel{rel: filepath.ToSlash(path), path: path, fixture: true, wanted: true})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", sweepFixtures, err)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].rel < models[j].rel })
	return models
}

// sweepSlots is how many models the sweep has in hand at once: -parallel's value, as it bounded
// the parallel subtests of tests/export's sweep, so a CI class sizes the memory peak the same way.
func sweepSlots() int {
	if f := flag.Lookup("test.parallel"); f != nil {
		if n, err := strconv.Atoi(f.Value.String()); err == nil && n > 0 {
			return n
		}
	}
	return runtime.GOMAXPROCS(0)
}

// sweepDirectoriesOf groups the SysML models by directory, each group to be
// checked as one model once its documents are loaded.
func sweepDirectoriesOf(models []*sweepModel) map[string]*sweepDirectory {
	directories := map[string]*sweepDirectory{}
	for _, m := range models {
		if !strings.HasSuffix(m.path, ".sysml") {
			continue
		}
		dir := filepath.Dir(m.path)
		d := directories[dir]
		if d == nil {
			d = &sweepDirectory{}
			directories[dir] = d
		}
		d.models = append(d.models, m)
		d.pending.Add(1)
	}
	return directories
}

// sweepOrder loads the largest directories first, so their whole-directory checks overlap
// the remaining per-model work instead of running alone at the end.
func sweepOrder(models []*sweepModel, directories map[string]*sweepDirectory) []int {
	order := make([]int, len(models))
	for i := range order {
		order[i] = i
	}
	size := func(i int) int {
		if d := directories[filepath.Dir(models[i].path)]; d != nil {
			return len(d.models)
		}
		return 0
	}
	sort.SliceStable(order, func(a, b int) bool {
		if sa, sb := size(order[a]), size(order[b]); sa != sb {
			return sa > sb
		}
		return filepath.Dir(models[order[a]].path) < filepath.Dir(models[order[b]].path)
	})
	return order
}

// check runs the settled-collections property over the directory's documents as one model
// (two or more, every one parsed clean) and releases them; reports whether a model was checked.
func (d *sweepDirectory) check(t *testing.T) bool {
	defer func() {
		for _, m := range d.models {
			m.doc = export.ModelDocument{}
		}
	}()
	if len(d.models) < 2 || d.broken.Load() {
		return false
	}
	names := make([]string, len(d.models))
	docs := make([]export.ModelDocument, len(d.models))
	for i, m := range d.models {
		names[i], docs[i] = m.rel, m.doc
	}
	graph, err := export.ModelToRDFWith(docs, export.IDQualifiedName)
	if err != nil {
		return false // a model the export refuses is not converted at all
	}
	if err := settledCollectionsHold(graph); err != nil {
		t.Errorf("%s: %v", strings.Join(names, ","), err)
		return false
	}
	return true
}

// settledCollectionsHold: the encoder's graph says its collections are settled, a copy that does
// not vouch for itself reconciles to itself unchanged, and both write the same api-json bytes.
func settledCollectionsHold(graph *rdf.Graph) error {
	if !graph.CollectionsSettled() {
		return errors.New("the encoder's graph does not say its collections are settled")
	}
	unvouched := rdf.NewGraphOf(graph.Triples(), graph.Prefixes)
	reconciled, err := rdf.ReconcileCollections(unvouched)
	if err != nil {
		return fmt.Errorf("reconciling the encoder's collections: %w", err)
	}
	if reconciled != unvouched {
		return errors.New("reconciling rewrote the encoder's collections, so they were not settled")
	}
	fast, err := export.WriteAPIJSON(graph)
	if err != nil {
		return err
	}
	checkedOut, err := export.WriteAPIJSON(unvouched)
	if err != nil {
		return err
	}
	if !bytes.Equal(fast, checkedOut) {
		return errors.New("api-json differs between the settled graph and the checked one")
	}
	return nil
}

// elementPropertiesAgree checks GetElementProperties answers what the api-json export writes for
// every (element, metamodel property) pair of the model (`sysx:` keys aside); returns the counts.
func elementPropertiesAgree(t *testing.T, ctx context.Context, srv *grpc.Service, m *sweepModel) (elements, pairs int) {
	absolute, err := filepath.Abs(m.path)
	if err != nil {
		t.Errorf("%s: %v", m.rel, err)
		return
	}
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: absolute}})
	if err != nil {
		t.Errorf("%s: ParseFile: %v", m.rel, err)
		return
	}
	var objects []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(m.apiJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&objects); err != nil {
		t.Errorf("%s: api-json: %v", m.rel, err)
		return
	}
	for _, object := range objects {
		id, _ := object["@id"].(string)
		resp, err := srv.GetElementProperties(ctx, &pb.GetElementPropertiesRequest{ModelHash: parsed.ModelHash, ElementId: id})
		if err != nil {
			t.Errorf("%s: GetElementProperties(%s): %v", m.rel, id, err)
			return
		}
		if resp.Error != "" {
			t.Errorf("%s: GetElementProperties(%s): %s", m.rel, id, resp.Error)
			continue
		}
		if metaclass, _ := object["@type"].(string); resp.Element.Metaclass != metaclass {
			t.Errorf("%s: %s is a %s in the export and a %s in the answer", m.rel, id, metaclass, resp.Element.Metaclass)
		}
		elements++
		answered := map[string]*pb.ElementProperty{}
		for _, property := range resp.Properties {
			answered[property.Name] = property
		}
		for name, value := range object {
			if name == "@id" || name == "@type" || strings.HasPrefix(name, "sysx:") {
				continue
			}
			pairs++
			property, ok := answered[name]
			if !ok {
				t.Errorf("%s: %s.%s is exported but not answered", m.rel, id, name)
				continue
			}
			delete(answered, name)
			if want, got := apiJSONSpelling(value), propertySpelling(property); want != got {
				t.Errorf("%s: %s.%s exported as %s, answered as %s", m.rel, id, name, want, got)
			}
		}
		for name := range answered {
			t.Errorf("%s: %s.%s is answered but not exported", m.rel, id, name)
		}
	}
	return elements, pairs
}

// apiJSONSpelling flattens an exported property value to one comparable text:
// element references by id, literals by value, in collection order.
func apiJSONSpelling(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []any:
		parts := make([]string, 0, len(v))
		for _, member := range v {
			parts = append(parts, apiJSONSpelling(member))
		}
		return strings.Join(parts, " | ")
	case map[string]any:
		id, _ := v["@id"].(string)
		return "element " + id
	case json.Number:
		return numberSpelling(v.String())
	case string:
		return "string " + v
	case bool:
		return "bool " + strconv.FormatBool(v)
	}
	return fmt.Sprintf("%T %v", value, value)
}

// propertySpelling flattens an answered property the way apiJSONSpelling does the export.
func propertySpelling(property *pb.ElementProperty) string {
	if property.Unsupported != "" {
		return "unsupported: " + property.Unsupported
	}
	parts := make([]string, 0, len(property.Values))
	for _, value := range property.Values {
		switch v := value.Value.(type) {
		case *pb.ElementPropertyValue_Element:
			parts = append(parts, "element "+v.Element.Id)
		case *pb.ElementPropertyValue_StringValue:
			parts = append(parts, "string "+v.StringValue)
		case *pb.ElementPropertyValue_BoolValue:
			parts = append(parts, "bool "+strconv.FormatBool(v.BoolValue))
		case *pb.ElementPropertyValue_IntValue:
			parts = append(parts, numberSpelling(strconv.FormatInt(v.IntValue, 10)))
		case *pb.ElementPropertyValue_RealValue:
			parts = append(parts, numberSpelling(strconv.FormatFloat(v.RealValue, 'g', -1, 64)))
		case *pb.ElementPropertyValue_Number:
			parts = append(parts, numberSpelling(v.Number))
		default:
			parts = append(parts, fmt.Sprintf("%T", value.Value))
		}
	}
	return strings.Join(parts, " | ")
}

// numberSpelling normalizes a decimal text so 2.0 and 2 compare equal.
func numberSpelling(text string) string {
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return "number " + strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "number " + text
}

// withoutDerivedReadsBack pins modelsync's readerDerived against the notation reader: the model
// publishes without derived properties to the notation it publishes with them; returns the triples left out.
func withoutDerivedReadsBack(graph *rdf.Graph) (dropped int, err error) {
	roundTrip := func(graph *rdf.Graph) ([]byte, error) {
		data, err := export.WriteAPIJSON(graph)
		if err != nil {
			return nil, err
		}
		back, err := export.ReadAPIJSON(data)
		if err != nil {
			return nil, err
		}
		return modelsync.Notation(back, nil)
	}
	want, err := roundTrip(graph)
	if err != nil {
		return 0, nil // a model the publisher cannot read back is not compared
	}
	cut := modelsync.WithoutDerived(graph)
	dropped = graph.Len() - cut.Len()
	got, err := roundTrip(cut)
	if err != nil {
		return dropped, fmt.Errorf("without derived properties: %w", err)
	}
	if !bytes.Equal(got, want) {
		return dropped, errors.New("the notation differs without derived properties")
	}
	return dropped, nil
}

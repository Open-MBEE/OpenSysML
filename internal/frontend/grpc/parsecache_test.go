package grpc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// parseCounter is the engine seam a counting client reads: it observes every
// call the service makes to its parser, with the document each call was for.
type parseCounter struct {
	mu    sync.Mutex
	names []string
}

func (c *parseCounter) record(name string) {
	c.mu.Lock()
	c.names = append(c.names, name)
	c.mu.Unlock()
}

func (c *parseCounter) calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.names...)
}

// countingService stands in for a fresh process: its in-process cache and
// lineage are empty, and only the record cache is shared with the writer.
func countingService(t *testing.T, cache *libs.Cache) (*Service, *parseCounter) {
	t.Helper()
	srv, err := NewService(10, "test", WithRecordCache(cache))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	counter := &parseCounter{}
	srv.parseSource = func(sf *source.SourceFile) parsedSource {
		counter.record(sf.Name())
		return parseSourceFile(sf)
	}
	t.Cleanup(srv.Close)
	return srv, counter
}

func twoDocuments(second string) *pb.ParseSourcesRequest {
	return &pb.ParseSourcesRequest{Documents: inlineDocuments(
		"lib.sysml", sourcesLibrary, "top.sysml", second,
	)}
}

// encodedTreeAndChild parses content and returns its astcodec stream, the root's
// node id and the id of one of its members, for hostile-record fixtures.
func encodedTreeAndChild(t *testing.T, name, content string) ([]byte, uint64, uint64) {
	t.Helper()
	sf := source.NewWithKind(name, []byte(content), source.KindSysML)
	p := parser.New(sf)
	root := p.ParseFile()
	w := pack.NewWriter()
	e := astcodec.NewEncoder(w)
	if err := e.Encode([]ast.Node{root}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rootID, ok := e.ID(root)
	if !ok || len(root.Members) == 0 {
		t.Fatal("the fixture tree has no identifiable root and member")
	}
	childID, ok := e.ID(root.Members[0])
	if !ok {
		t.Fatal("the fixture tree's member is not in its node table")
	}
	return w.Bytes(), rootID, childID
}

// hostileNodeStream declares far more nodes than the density bound allows.
// Filler bytes satisfy pack.Reader's holds check, so Allocate really runs.
func hostileNodeStream(t *testing.T) []byte {
	t.Helper()
	valid, _, _ := encodedTreeAndChild(t, "probe.sysml", "package P { part x; }\n")
	r, err := pack.NewReader(valid)
	if err != nil {
		t.Fatal(err)
	}
	kinds := r.Len()
	const declared = 2000
	w := pack.NewWriter()
	w.Len(kinds)
	for i := 0; i < kinds; i++ {
		w.Len(declared)
	}
	w.Section(func(sub *pack.Writer) {
		for i := 0; i < kinds*declared; i++ {
			sub.Bool(true)
		}
	})
	return w.Bytes()
}

// A process restored from another's records makes zero parse calls, including
// after the writer edited through its active workspace.
func TestPersistentParseCacheRestoresAcrossProcessesWithNoParses(t *testing.T) {
	editedLibrary := sourcesLibrary + "// edited library\n"
	editedTop := sourcesTop + "// edited top\n"
	request := &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", editedLibrary, "top.sysml", editedTop)}
	want := answersOf(t, mustNewService(t, 10), request)

	cache, _ := recordCacheIn(t)
	writer := recordingService(t, cache)
	answersOf(t, writer, twoDocuments(sourcesTop)) // first parse: the set is noted
	answersOf(t, writer, &pb.ParseSourcesRequest{Documents: inlineDocuments(
		"lib.sysml", sourcesLibrary, "top.sysml", editedTop,
	)}) // second: the workspace is built from the records
	answersOf(t, writer, request) // an active-workspace edit; it must be recorded too

	srv, counter := countingService(t, cache) // process B, cold but for the records
	got := answersOf(t, srv, request)
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("a restored model parsed %d documents (%v), want none", len(names), names)
	}
	requireSameAnswers(t, "restored across processes", got, want)
}

// Changing one byte of one document missifies exactly that document's record:
// the other document's tree is restored, so the process parses one document.
func TestPersistentParseCacheReparsesOnlyTheEditedDocument(t *testing.T) {
	base := twoDocuments(sourcesTop)
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), base)

	editedLibrary := strings.Replace(sourcesLibrary, "150", "151", 1)
	if editedLibrary == sourcesLibrary {
		t.Fatal("the fixture edit did not change a byte")
	}
	// Rebuild the request with the one-byte edit in the first document.
	request := &pb.ParseSourcesRequest{Documents: inlineDocuments(
		"lib.sysml", editedLibrary, "top.sysml", sourcesTop,
	)}
	want := answersOf(t, mustNewService(t, 10), request)

	srv, counter := countingService(t, cache)
	got := answersOf(t, srv, request)
	if names := counter.calls(); len(names) != 1 || names[0] != "lib.sysml" {
		t.Fatalf("a one-byte edit parsed %v, want only lib.sysml", names)
	}
	requireSameAnswers(t, "one-byte edit", got, want)
}

// A real mid-size model restores through the same parse-count seam: its stream
// is denser than the small fixtures, so the node-density bound must admit it.
func TestPersistentParseCacheRestoresARealModel(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "self-model", "pipeline.sysml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	request := &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{
		{Source: &pb.SourceDocument_FilePath{FilePath: path}},
	}}
	cache, _ := recordCacheIn(t)
	writer, err := recordingService(t, cache).ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("writing parse: %v", err)
	}

	srv, counter := countingService(t, cache)
	restored, err := srv.ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("restoring parse: %v", err)
	}
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("a real model was re-parsed on restore: %v", names)
	}
	if restored.ModelHash != writer.ModelHash {
		t.Fatalf("restored hash %q differs from %q", restored.ModelHash, writer.ModelHash)
	}
}

// Trees are cached per document, so asking for the same bytes in another order
// restores both trees and rebuilds the index in the new order, parsing neither.
func TestPersistentParseCacheRestoresDocumentsInAnyOrder(t *testing.T) {
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), twoDocuments(sourcesTop))

	swapped := &pb.ParseSourcesRequest{Documents: inlineDocuments(
		"top.sysml", sourcesTop, "lib.sysml", sourcesLibrary,
	)}
	want := answersOf(t, mustNewService(t, 10), swapped)
	srv, counter := countingService(t, cache)
	requireSameAnswers(t, "swapped order", answersOf(t, srv, swapped), want)
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("reordering cached documents parsed %v, want none", names)
	}
}

// A base restored from records still relates an edit document by document: the
// lineage is primed with the restored trees, so only the edit is affected.
func TestPersistentParseCacheKeepsAffectedAcrossProcesses(t *testing.T) {
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), twoDocuments(sourcesTop))

	srv, counter := countingService(t, cache)
	restored, err := srv.ParseSources(context.Background(), twoDocuments(sourcesTop))
	if err != nil {
		t.Fatalf("restored ParseSources: %v", err)
	}
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("the base was not restored: %v", names)
	}

	edited := &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", sourcesLibrary, "top.sysml", strings.Replace(sourcesTop, "Car", "Car2", 1),
		),
		BaseModelHash: restored.ModelHash,
	}
	resp, err := srv.ParseSources(context.Background(), edited)
	if err != nil {
		t.Fatalf("edited ParseSources with a base: %v", err)
	}
	if want := []string{"top.sysml"}; !slices.Equal(resp.Affected, want) {
		t.Fatalf("affected = %q, want %q", resp.Affected, want)
	}
}

// A parse-only restore carries no stamps that could hide an unanalyzed sibling:
// fixing a syntax error must report it as affected, with its fresh diagnostic.
func TestPersistentParseCacheRestoredSyntaxErrorDoesNotHideSiblingDiagnostics(t *testing.T) {
	// The missing semicolon is a syntax error, but A::Fixed is declared in
	// both versions: the sibling's suggestions read the same spellings.
	broken := "package A {\n\tpart def Fixed\n}\n"
	fixedText := "package A {\n\tpart def Fixed;\n}\n"
	unresolved := "package B {\n\tpart x : Missing;\n}\n"
	request := &pb.ParseSourcesRequest{Documents: inlineDocuments("a.sysml", broken, "b.sysml", unresolved)}
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), request) // process A writes both trees

	srv, counter := countingService(t, cache) // process B restores them
	base, err := srv.ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("restored ParseSources: %v", err)
	}
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("the broken base was not restored: %v", names)
	}
	for _, d := range base.Diagnostics {
		if d.Span.GetFile() == "b.sysml" {
			t.Fatalf("a parse-only base published b.sysml's semantic diagnostic: %v", d)
		}
	}

	fixed := &pb.ParseSourcesRequest{
		Documents:     inlineDocuments("a.sysml", fixedText, "b.sysml", unresolved),
		BaseModelHash: base.ModelHash,
	}
	resp, err := srv.ParseSources(context.Background(), fixed)
	if err != nil {
		t.Fatalf("fixed ParseSources with a base: %v", err)
	}
	if !slices.Contains(resp.Affected, "b.sysml") {
		t.Fatalf("b.sysml was never analyzed in the base, yet is not affected: %v", resp.Affected)
	}
	found := false
	for _, d := range resp.Diagnostics {
		if d.Span.GetFile() == "b.sysml" && strings.Contains(d.Message, "Missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("b.sysml's unresolved type is not reported: %v", resp.Diagnostics)
	}
}

// Damage to a stored tree is a miss, not a false hit or a panic: the document
// is parsed again, its record rewritten, and the next process restores it.
func TestPersistentParseCacheRecoversFromADamagedRecord(t *testing.T) {
	request := twoDocuments(sourcesTop)
	cache, dir := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), request)

	key := cache.ParseKey("lib.sysml", "", source.KindSysML, []byte(sourcesLibrary))
	damaged := filepath.Join(dir, "parses", key+".idx")
	if _, err := os.Stat(damaged); err != nil {
		t.Fatalf("the writer left no parse record: %v", err)
	}
	if err := os.WriteFile(damaged, []byte("not a gob stream"), 0o600); err != nil {
		t.Fatal(err)
	}

	want := answersOf(t, mustNewService(t, 10), request)
	srv, counter := countingService(t, cache)
	requireSameAnswers(t, "damaged record", answersOf(t, srv, request), want)
	if names := counter.calls(); len(names) != 1 || names[0] != "lib.sysml" {
		t.Fatalf("a damaged record parsed %v, want only lib.sysml", names)
	}

	restored, again := countingService(t, cache)
	requireSameAnswers(t, "rewritten record", answersOf(t, restored, request), want)
	if names := again.calls(); len(names) != 0 {
		t.Fatalf("the rewritten record was not used, parses: %v", names)
	}
}

// A record with matching metadata but a root id that names a member node is a
// miss: the decoder must run, refuse the tree, and leave the parse to the caller.
func TestPersistentParseCacheRefusesAnImplausibleTree(t *testing.T) {
	request := twoDocuments(sourcesTop)
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), request)

	tree, _, childID := encodedTreeAndChild(t, "lib.sysml", sourcesLibrary)
	key := cache.ParseKey("lib.sysml", "", source.KindSysML, []byte(sourcesLibrary))
	if err := cache.StoreParse(key, &libs.ParseRecord{
		Name: "lib.sysml", Kind: source.KindSysML, Language: "", AST: tree, RootID: childID,
	}); err != nil {
		t.Fatal(err)
	}
	want := answersOf(t, mustNewService(t, 10), request)
	srv, counter := countingService(t, cache)
	requireSameAnswers(t, "implausible tree", answersOf(t, srv, request), want)
	if names := counter.calls(); len(names) != 1 || names[0] != "lib.sysml" {
		t.Fatalf("an implausible tree parsed %v, want only lib.sysml", names)
	}
}

// A hostile record whose node counts would allocate hundreds of megabytes is a
// benign miss: the node table is bounded by the stream's own size.
func TestPersistentParseCacheRefusesAHostileTreeWithoutAllocating(t *testing.T) {
	hostile := hostileNodeStream(t)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 10; i++ {
		root, err := decodeParseTree(hostile, 1)
		if err == nil || root != nil {
			t.Fatal("a hostile node table decoded into a tree")
		}
		if !strings.Contains(err.Error(), "node count") {
			t.Fatalf("the decoder refused before the node-count bound: %v", err)
		}
	}
	runtime.ReadMemStats(&after)
	if delta := after.TotalAlloc - before.TotalAlloc; delta > 4<<20 {
		t.Fatalf("a %d-byte hostile stream allocated %d bytes", len(hostile), delta)
	}

	request := twoDocuments(sourcesTop)
	cache, _ := recordCacheIn(t)
	answersOf(t, recordingService(t, cache), request)
	key := cache.ParseKey("lib.sysml", "", source.KindSysML, []byte(sourcesLibrary))
	if err := cache.StoreParse(key, &libs.ParseRecord{
		Name: "lib.sysml", Kind: source.KindSysML, Language: "", AST: hostile, RootID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	want := answersOf(t, mustNewService(t, 10), request)
	srv, counter := countingService(t, cache)
	requireSameAnswers(t, "hostile tree", answersOf(t, srv, request), want)
	if names := counter.calls(); len(names) != 1 || names[0] != "lib.sysml" {
		t.Fatalf("a hostile tree parsed %v, want only lib.sysml", names)
	}
}

// ParseFile restores its single document the same way ParseSources does.
func TestPersistentParseCacheRestoresParseFile(t *testing.T) {
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: sourcesLibrary}}
	cache, _ := recordCacheIn(t)
	first, err := recordingService(t, cache).ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	srv, counter := countingService(t, cache)
	second, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("ParseFile after restore: %v", err)
	}
	if names := counter.calls(); len(names) != 0 {
		t.Fatalf("a restored ParseFile parsed %v, want none", names)
	}
	if second.ModelHash != first.ModelHash {
		t.Fatalf("restored hash %q differs from %q", second.ModelHash, first.ModelHash)
	}
}

// A service with no record cache still parses every time: the persistent cache
// is opt-in, and nothing about the answer changes when it is absent.
func TestPersistentParseCacheIsOptIn(t *testing.T) {
	request := twoDocuments(sourcesTop)
	want := answersOf(t, mustNewService(t, 10), request)
	srv, err := NewService(10, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	counter := &parseCounter{}
	srv.parseSource = func(sf *source.SourceFile) parsedSource {
		counter.record(sf.Name())
		return parseSourceFile(sf)
	}
	requireSameAnswers(t, "no cache", answersOf(t, srv, request), want)
	if names := counter.calls(); len(names) != 2 {
		t.Fatalf("a service without a record cache parsed %v, want two documents", names)
	}
}

// BenchmarkPersistentParseCache measures the cold parse and the restored parse
// across processes over the same two documents.
func BenchmarkPersistentParseCache(b *testing.B) {
	request := twoDocuments(sourcesTop)
	cache, err := libs.NewCacheIn(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	writer, err := NewService(10, "bench", WithRecordCache(cache))
	if err != nil {
		b.Fatal(err)
	}
	if _, err := writer.ParseSources(context.Background(), request); err != nil {
		b.Fatal(err)
	}
	writer.Close()

	for _, run := range []struct {
		name  string
		cache *libs.Cache
	}{{"cold", nil}, {"restored", cache}} {
		b.Run(run.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				srv, err := NewService(10, "bench", WithRecordCache(run.cache))
				if err != nil {
					b.Fatal(err)
				}
				if _, err := srv.ParseSources(context.Background(), request); err != nil {
					b.Fatal(err)
				}
				srv.Close()
			}
		})
	}
}

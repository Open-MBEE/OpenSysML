package grpc

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// libraryModel names the standard library, so a model analysed against a
// prewarmed index has something to resolve there.
const libraryModel = `package Sweep {
	private import ISQ::*;
	attribute def Reading {
		attribute magnitude : ScalarValues::Real;
	}
	part def Sensor {
		attribute reading : Reading;
	}
	part sensor : Sensor;
	calc def Doubled { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2; }
}`

// prewarmedService returns a service whose shared library index is built, so a
// request is handed an overlay over it rather than building the library.
func prewarmedService(t *testing.T, cacheSize int) *Service {
	t.Helper()
	t.Setenv(IndexPrewarmEnvVar, "4")
	svc := mustNewService(t, cacheSize)
	svc.Prewarm()
	deadline := time.Now().Add(2 * time.Minute)
	for !svc.libIndexes.ready() {
		if time.Now().After(deadline) {
			t.Fatal("the shared library index never built")
		}
		time.Sleep(2 * time.Millisecond)
	}
	return svc
}

// parseContent parses content through the ParseFile RPC, returning the response
// and the model the service cached for it.
func parseContent(t *testing.T, svc *Service, content string) (*pb.ParseFileResponse, *CachedModel) {
	t.Helper()
	resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	cached, ok := svc.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatalf("model %s was not cached", resp.ModelHash)
	}
	return resp, cached
}

// diagLines renders diagnostics as comparable text.
func diagLines(diags []*pb.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		line, col := int32(0), int32(0)
		if d.Span != nil {
			line, col = d.Span.StartLine, d.Span.StartCol
		}
		out = append(out, fmt.Sprintf("%s %d:%d %s", d.Severity, line, col, d.Message))
	}
	return out
}

// sharedBase is a fresh overlay over the service's library base.
func sharedBase(svc *Service) *symbols.Index {
	idx, _ := svc.libIndexes.get()
	return idx
}

// lookupLines renders every name an index knows with the kinds it resolves to,
// which is what a model is analysed against.
func lookupLines(idx *symbols.Index) []string {
	fqns := idx.FQNs()
	sort.Strings(fqns)
	out := make([]string, 0, len(fqns))
	for _, fqn := range fqns {
		var kinds []string
		for _, sym := range idx.LookupQualified(fqn) {
			kinds = append(kinds, fmt.Sprintf("%s/%t", sym.Kind, idx.Library(sym)))
		}
		sort.Strings(kinds)
		out = append(out, fqn+" -> "+strings.Join(kinds, ","))
	}
	return out
}

// TestSharedIndexMatchesFreshlyBuiltIndex proves the shared library index does
// not change what a model resolves against: the same model analysed over a
// prewarmed shared base and over a library built on the request path produces
// the same diagnostics, the same root, and the same qualified lookups over the
// whole index — including what each name resolves to and whether it is library
// content.
func TestSharedIndexMatchesFreshlyBuiltIndex(t *testing.T) {
	demo, err := os.ReadFile("../../examples/combined-behavioral-demo.sysml")
	if err != nil {
		t.Fatalf("read demo model: %v", err)
	}
	models := map[string]string{
		"library_references": libraryModel,
		"behavioral_demo":    string(demo),
		"parse_errors":       "package Broken { part def (((;",
	}

	shared := prewarmedService(t, 10)
	defer shared.Close()

	// Prewarming off is the path where the request itself builds the library.
	t.Setenv(IndexPrewarmEnvVar, "0")
	inline := mustNewService(t, 10)
	defer inline.Close()

	for name, content := range models {
		t.Run(name, func(t *testing.T) {
			sharedResp, sharedModel := parseContent(t, shared, content)
			inlineResp, inlineModel := parseContent(t, inline, content)

			if sharedResp.ModelHash != inlineResp.ModelHash {
				t.Fatalf("model hash differs: %s vs %s", sharedResp.ModelHash, inlineResp.ModelHash)
			}
			if got, want := diagLines(sharedResp.Diagnostics), diagLines(inlineResp.Diagnostics); !equalLines(got, want) {
				t.Errorf("diagnostics differ:\nshared: %v\ninline: %v", got, want)
			}
			if (sharedResp.Root == nil) != (inlineResp.Root == nil) {
				t.Fatalf("root differs: shared %v, inline %v", sharedResp.Root, inlineResp.Root)
			}
			if sharedResp.Root != nil {
				sharedKids := append([]string(nil), sharedResp.Root.ChildIds...)
				inlineKids := append([]string(nil), inlineResp.Root.ChildIds...)
				sort.Strings(sharedKids)
				sort.Strings(inlineKids)
				if !equalLines(sharedKids, inlineKids) {
					t.Errorf("root children differ:\nshared: %v\ninline: %v", sharedKids, inlineKids)
				}
			}
			if got, want := lookupLines(sharedModel.Index), lookupLines(inlineModel.Index); !equalLines(got, want) {
				t.Errorf("qualified lookups differ: %s", firstDifference(got, want))
			}
		})
	}
}

// equalLines reports whether two renderings are the same line for line.
func equalLines(a, b []string) bool {
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

// firstDifference describes where two renderings first disagree.
func firstDifference(a, b []string) string {
	for i := range a {
		if i >= len(b) {
			return fmt.Sprintf("extra line %d: %q", i, a[i])
		}
		if a[i] != b[i] {
			return fmt.Sprintf("line %d: %q vs %q", i, a[i], b[i])
		}
	}
	if len(b) > len(a) {
		return fmt.Sprintf("missing line %d: %q", len(a), b[len(a)])
	}
	return "no difference"
}

// TestParseFileServesEveryModelFromOneLibraryIndex is the regression guard
// against going back to a library index per model: many cache misses build the
// library once between them and share it.
func TestParseFileServesEveryModelFromOneLibraryIndex(t *testing.T) {
	const models = 8
	svc := prewarmedService(t, 10)
	defer svc.Close()

	for i := 0; i < models; i++ {
		parseContent(t, svc, fmt.Sprintf("package P%d { part def Thing%d; }", i, i))
	}

	stats := svc.libIndexes.snapshot()
	if stats.Built != 1 {
		t.Errorf("%d models built %d library indexes, want 1 (stats %+v)", models, stats.Built, stats)
	}
	if stats.Inline != 0 {
		t.Errorf("%d of %d models loaded the library on the request path, want 0 (stats %+v)", stats.Inline, models, stats)
	}
	if stats.Shared != models {
		t.Errorf("served %d models from the shared index, want %d (stats %+v)", stats.Shared, models, stats)
	}
}

// TestParseFileTakesNoIndexOnACacheHit keeps the LRU doing its job: the same
// content twice costs one index, not two.
func TestParseFileTakesNoIndexOnACacheHit(t *testing.T) {
	svc := prewarmedService(t, 10)
	defer svc.Close()

	first, _ := parseContent(t, svc, libraryModel)
	second, _ := parseContent(t, svc, libraryModel)
	if first.ModelHash != second.ModelHash {
		t.Fatalf("same content hashed differently: %s vs %s", first.ModelHash, second.ModelHash)
	}

	stats := svc.libIndexes.snapshot()
	if got := stats.Shared + stats.Inline; got != 1 {
		t.Errorf("two requests for one model took %d indexes, want 1 (stats %+v)", got, stats)
	}
}

// TestCachedModelsOwnTheirIndex holds the rule that makes sharing safe: each
// cached model has an index of its own to write its document into, even though
// they read one library — including when the LRU evicts and when requests arrive
// concurrently.
func TestCachedModelsOwnTheirIndex(t *testing.T) {
	const models = 8
	svc := prewarmedService(t, 3) // cache smaller than the model count, so it evicts
	defer svc.Close()

	var wg sync.WaitGroup
	hashes := make([]string, models)
	for i := 0; i < models; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source: &pb.ParseFileRequest_Content{Content: fmt.Sprintf("package P%d { part def Thing%d; }", i, i)},
			})
			if err != nil {
				t.Errorf("ParseFile %d: %v", i, err)
				return
			}
			hashes[i] = resp.ModelHash
		}(i)
	}
	wg.Wait()

	seen := map[*symbols.Index]string{}
	for _, hash := range hashes {
		cached, ok := svc.cache.Get(hash)
		if !ok {
			continue // evicted, which is the LRU working
		}
		if other, dup := seen[cached.Index]; dup {
			t.Fatalf("models %s and %s share one index", other, hash)
		}
		seen[cached.Index] = hash
	}
	if len(seen) == 0 {
		t.Fatal("no model survived in the cache")
	}

	// One model's document must not be visible through another's index, which is
	// what the shared base would break if a document landed in it.
	for i, hash := range hashes {
		cached, ok := svc.cache.Get(hash)
		if !ok {
			continue
		}
		mine := fmt.Sprintf("P%d", i)
		if syms := cached.Index.LookupQualified(mine); len(syms) == 0 {
			t.Errorf("model %d does not know its own package %s", i, mine)
		}
		for j := 0; j < models; j++ {
			if j == i {
				continue
			}
			other := fmt.Sprintf("P%d", j)
			if syms := cached.Index.LookupQualified(other); len(syms) != 0 {
				t.Errorf("model %d sees %s, another model's package", i, other)
			}
		}
	}
}

// TestLibraryBaseFallsBackToBuildingInline pins that a result never depends on
// how far prewarming got: with prewarming off, the first request builds the
// library itself and the answers are the same ones.
func TestLibraryBaseFallsBackToBuildingInline(t *testing.T) {
	t.Setenv(IndexPrewarmEnvVar, "0")
	svc := mustNewService(t, 10)
	defer svc.Close()
	svc.Prewarm() // prewarming off builds nothing

	if svc.libIndexes.ready() {
		t.Error("prewarming is off, yet the library index is built")
	}
	resp, cached := parseContent(t, svc, libraryModel)
	if len(resp.Diagnostics) != 0 {
		t.Errorf("model reported diagnostics without the pool: %v", diagLines(resp.Diagnostics))
	}
	if syms := cached.Index.LookupQualified("ScalarValues::Real"); len(syms) == 0 {
		t.Error("inline index does not carry the standard library")
	}
	if stats := svc.libIndexes.snapshot(); stats.Inline != 1 || stats.Shared != 0 {
		t.Errorf("stats %+v, want one inline build and no shared checkout", stats)
	}
}

// TestLibraryBaseCloseReleasesTheIndexAndStillServes covers shutdown: closing
// waits for a build in flight, releases the shared index, and leaves the service
// able to answer by building it again.
func TestLibraryBaseCloseReleasesTheIndexAndStillServes(t *testing.T) {
	svc := prewarmedService(t, 10)
	svc.Close()
	svc.Close() // closing twice is not an error

	if svc.libIndexes.ready() {
		t.Error("the shared library index is still held after close")
	}
	_, cached := parseContent(t, svc, libraryModel)
	if syms := cached.Index.LookupQualified("ScalarValues::Real"); len(syms) == 0 {
		t.Error("index built after close does not carry the standard library")
	}
	if stats := svc.libIndexes.snapshot(); stats.Inline != 1 {
		t.Errorf("stats %+v, want the request after close to have built inline", stats)
	}
}

// TestIndexPrewarmFromEnv covers the prewarm setting a deployment asks for,
// including the values that are a typo rather than a number.
func TestIndexPrewarmFromEnv(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: DefaultIndexPrewarm},
		{raw: "   ", want: DefaultIndexPrewarm},
		{raw: "0", want: 0},
		{raw: "2", want: 2},
		{raw: " 6 ", want: 6},
		{raw: "-1", wantErr: true},
		{raw: "many", wantErr: true},
		{raw: "1.5", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			t.Setenv(IndexPrewarmEnvVar, tc.raw)
			got, err := indexPrewarmFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%q was accepted as %d", tc.raw, got)
				}
				if !strings.Contains(err.Error(), IndexPrewarmEnvVar) {
					t.Errorf("error does not name %s: %v", IndexPrewarmEnvVar, err)
				}
				if _, serr := NewService(4, "test"); serr == nil {
					t.Error("NewService accepted an unusable prewarm setting")
				}
				return
			}
			if err != nil {
				t.Fatalf("indexPrewarmFromEnv(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("size %d, want %d", got, tc.want)
			}
		})
	}
}

// TestIndexPrewarmLegacyEnvName: the legacy SYSML_GRPC_INDEX_POOL name still
// sets the prewarm, and the OPENSYSML_ name wins when both are set.
func TestIndexPrewarmLegacyEnvName(t *testing.T) {
	t.Run("legacy name", func(t *testing.T) {
		t.Setenv(IndexPrewarmEnvVar, "")
		t.Setenv(envvar.Legacy(IndexPrewarmEnvVar), "7")
		got, err := indexPrewarmFromEnv()
		if err != nil {
			t.Fatalf("indexPrewarmFromEnv: %v", err)
		}
		if got != 7 {
			t.Errorf("size %d, want the legacy value 7", got)
		}
	})

	t.Run("both set, new name wins", func(t *testing.T) {
		t.Setenv(IndexPrewarmEnvVar, "3")
		t.Setenv(envvar.Legacy(IndexPrewarmEnvVar), "9")
		got, err := indexPrewarmFromEnv()
		if err != nil {
			t.Fatalf("indexPrewarmFromEnv: %v", err)
		}
		if got != 3 {
			t.Errorf("size %d, want the OPENSYSML_ value 3 over the legacy 9", got)
		}
	})
}

// TestLibraryBaseBuildsOnceUnderConcurrentDemand pins that many requests
// arriving at a cold service build one library between them: two builds of one
// library would each parse what the other was about to cache, and would cost two
// copies of it.
func TestLibraryBaseBuildsOnceUnderConcurrentDemand(t *testing.T) {
	var mu sync.Mutex
	built := 0
	base := newLibraryBase(func() (*symbols.Index, libs.Source) {
		mu.Lock()
		built++
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		idx := symbols.NewIndex()
		idx.Freeze()
		return idx, nil
	})
	defer base.close()

	const models = 16
	var wg sync.WaitGroup
	indexes := make([]*symbols.Index, models)
	for i := 0; i < models; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			indexes[i], _ = base.get()
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if built != 1 {
		t.Errorf("%d models built %d libraries, want 1", models, built)
	}
	seen := map[*symbols.Index]bool{}
	for i, idx := range indexes {
		if idx == nil {
			t.Fatalf("model %d was handed no index", i)
		}
		if seen[idx] {
			t.Fatalf("model %d was handed an index another model already has", i)
		}
		seen[idx] = true
	}
}

// TestLibraryBasePrewarmsOnce pins that repeated or concurrent prewarming builds
// one library rather than one per call, and that close waits for the build it
// started rather than leaving it running.
func TestLibraryBasePrewarmsOnce(t *testing.T) {
	var mu sync.Mutex
	built := 0
	base := newLibraryBase(func() (*symbols.Index, libs.Source) {
		mu.Lock()
		built++
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		idx := symbols.NewIndex()
		idx.Freeze()
		return idx, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			base.prewarm()
		}()
	}
	wg.Wait()
	base.close()

	mu.Lock()
	defer mu.Unlock()
	if built != 1 {
		t.Errorf("8 prewarms built %d libraries, want 1", built)
	}
}

// BenchmarkParseFileColdShared measures a first-time model over a prewarmed
// shared library; BenchmarkParseFileColdInline measures the same work with
// prewarming off, where the first request builds the library.
func BenchmarkParseFileColdShared(b *testing.B) {
	benchmarkParseFileCold(b, DefaultIndexPrewarm)
}

func BenchmarkParseFileColdInline(b *testing.B) {
	benchmarkParseFileCold(b, 0)
}

func benchmarkParseFileCold(b *testing.B, prewarm int) {
	b.Setenv(IndexPrewarmEnvVar, fmt.Sprint(prewarm))
	svc, err := NewService(b.N+1, "bench")
	if err != nil {
		b.Fatalf("NewService: %v", err)
	}
	defer svc.Close()
	svc.Prewarm()
	if prewarm > 0 {
		for !svc.libIndexes.ready() {
			time.Sleep(time.Millisecond)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content := fmt.Sprintf("%s\n// model %d\n", libraryModel, i)
		if _, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{Content: content},
		}); err != nil {
			b.Fatalf("ParseFile: %v", err)
		}
	}
}

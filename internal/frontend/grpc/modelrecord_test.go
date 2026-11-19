package grpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"google.golang.org/protobuf/proto"
)

// recordedModels are models whose analysis reports nothing, one finding over
// two documents, and several findings in one document.
var recordedModels = [][]string{
	{"lib.sysml", sourcesLibrary, "top.sysml", sourcesTop},
	{"lib.sysml", "package Lib {\n\tpart def Motor;\n}\n", "top.sysml", sourcesTop},
	{"lib.sysml", sourcesLibrary, "top.sysml", "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t\tpart spare : Wheel;\n\t}\n}\n"},
	{"only.sysml", "package P {\n\tpart x : Missing;\n\tpart def Car {\n\t\tpart w : Wheel;\n\t}\n\tpart car : Car {\n\t\tpart redefines q;\n\t}\n}\n"},
}

func recordingService(t *testing.T, cache *libs.Cache) *Service {
	t.Helper()
	srv, err := NewService(10, "test", WithRecordCache(cache))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func recordCacheIn(t *testing.T) (*libs.Cache, string) {
	t.Helper()
	base := t.TempDir()
	cache, err := libs.NewCacheIn(base)
	if err != nil {
		t.Fatalf("NewCacheIn: %v", err)
	}
	return cache, filepath.Join(base, "sysml-ls", "libs")
}

func recordFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.idx"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// modelAnswers is everything a client reads of a parsed model that its analysis decides.
type modelAnswers struct {
	parse       *pb.ParseSourcesResponse
	diagnostics *pb.DiagnosticsResponse
	apiJSON     string
}

func answersOf(t *testing.T, srv *Service, request *pb.ParseSourcesRequest) modelAnswers {
	t.Helper()
	parsed, err := srv.ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	diagnostics, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: parsed.ModelHash})
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return modelAnswers{parse: parsed, diagnostics: diagnostics, apiJSON: converted.GetContent() + converted.GetError()}
}

func requireSameAnswers(t *testing.T, label string, got, want modelAnswers) {
	t.Helper()
	if !proto.Equal(got.parse, want.parse) {
		t.Errorf("%s: ParseSources answers\n%v\na fresh parse\n%v", label, got.parse, want.parse)
	}
	if !proto.Equal(got.diagnostics, want.diagnostics) {
		t.Errorf("%s: GetDiagnostics answers\n%v\na fresh parse\n%v", label, got.diagnostics, want.diagnostics)
	}
	if got.apiJSON != want.apiJSON {
		t.Errorf("%s: api-json differs from a fresh parse's", label)
	}
}

// A service that finds the record another wrote answers as one that analyzed the
// model itself, in both conformance modes, and so does the one that wrote it.
func TestParseSourcesFromARecordEqualsAFreshParse(t *testing.T) {
	for _, strict := range []bool{false, true} {
		for i, named := range recordedModels {
			request := &pb.ParseSourcesRequest{Documents: inlineDocuments(named...), StrictConformance: strict}
			want := answersOf(t, mustNewService(t, 10), request)
			if i == len(recordedModels)-1 && len(want.diagnostics.Diagnostics) < 2 {
				t.Fatalf("the model meant to carry several diagnostics carries %d", len(want.diagnostics.Diagnostics))
			}
			cache, dir := recordCacheIn(t)
			requireSameAnswers(t, "writing the record", answersOf(t, recordingService(t, cache), request), want)
			if n := len(recordFiles(t, dir)); n != 1 {
				t.Fatalf("strict=%v model %d: %d records written, want 1", strict, i, n)
			}
			requireSameAnswers(t, "reading the record", answersOf(t, recordingService(t, cache), request), want)
		}
	}
}

// plantRecord files, as the record of request's model, one whose first document
// carries a diagnostic no analysis reports.
func plantRecord(t *testing.T, srv *Service, request *pb.ParseSourcesRequest, names ...string) {
	t.Helper()
	probe := mustNewService(t, 10)
	parsed, err := probe.ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	idx, _ := srv.libIndexes.get()
	key, ok := srv.modelRecordKey(parsed.ModelHash, idx)
	if !ok {
		t.Fatal("the service has no key for the model's record")
	}
	rec := &libs.ModelRecord{}
	for _, name := range names {
		rec.Documents = append(rec.Documents, libs.ModelDocument{Name: name, Kind: source.KindOf(name)})
	}
	rec.Documents[0].Diagnostics = []diag.Diagnostic{{
		Severity: diag.SeverityError, Span: source.Span{Offset: 0, Len: 7}, Message: plantedMessage, Source: "planted",
	}}
	if err := srv.records.StoreModel(key, rec); err != nil {
		t.Fatalf("StoreModel: %v", err)
	}
}

const plantedMessage = "planted by the test"

func reportsPlanted(resp *pb.ParseSourcesResponse) bool {
	for _, d := range resp.Diagnostics {
		if d.Message == plantedMessage {
			return true
		}
	}
	return false
}

// A model with a record is not analyzed: what the record holds is what is answered.
func TestParseSourcesReadsTheRecordInsteadOfAnalyzing(t *testing.T) {
	cache, _ := recordCacheIn(t)
	srv := recordingService(t, cache)
	request := &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop)}
	plantRecord(t, srv, request, "lib.sysml", "top.sysml")
	resp, err := srv.ParseSources(context.Background(), request)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if !reportsPlanted(resp) {
		t.Fatalf("the model was analyzed though its record was there: %v", resp.Diagnostics)
	}
}

// A record answers only the model it was filed under: another conformance mode,
// another content, another name or another order of the documents is analyzed.
func TestARecordAnswersOnlyItsOwnModel(t *testing.T) {
	cache, _ := recordCacheIn(t)
	srv := recordingService(t, cache)
	recorded := &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop)}
	plantRecord(t, srv, recorded, "lib.sysml", "top.sysml")
	others := map[string]*pb.ParseSourcesRequest{
		"strict":  {Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop), StrictConformance: true},
		"edited":  {Documents: inlineDocuments("lib.sysml", sourcesLibrary+"// edited\n", "top.sysml", sourcesTop)},
		"renamed": {Documents: inlineDocuments("library.sysml", sourcesLibrary, "top.sysml", sourcesTop)},
		"swapped": {Documents: inlineDocuments("top.sysml", sourcesTop, "lib.sysml", sourcesLibrary)},
		"kerml": {Documents: []*pb.SourceDocument{
			{Source: &pb.SourceDocument_Content{Content: sourcesLibrary}, Name: "lib.sysml", Language: "kerml"},
			{Source: &pb.SourceDocument_Content{Content: sourcesTop}, Name: "top.sysml"},
		}},
	}
	for label, request := range others {
		resp, err := recordingService(t, cache).ParseSources(context.Background(), request)
		if err != nil {
			t.Fatalf("%s: ParseSources: %v", label, err)
		}
		if reportsPlanted(resp) {
			t.Errorf("%s: answered from the record of another model", label)
		}
	}
}

// The model hash does not tell a file read as KerML from an inline document of its
// name and content read as SysML, so the record tells them apart.
func TestARecordIsOfTheLanguageEachDocumentWasReadAs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lib.kerml")
	content := "package Lib {\n\tpart def Engine;\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	inline := &pb.ParseSourcesRequest{Documents: inlineDocuments(path, content)}
	file := &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{{Source: &pb.SourceDocument_FilePath{FilePath: path}}}}
	wantInline := answersOf(t, mustNewService(t, 10), inline)
	wantFile := answersOf(t, mustNewService(t, 10), file)
	if wantInline.parse.ModelHash != wantFile.parse.ModelHash {
		t.Skip("the model hash tells the two apart, so they share no record")
	}
	if proto.Equal(wantInline.diagnostics, wantFile.diagnostics) {
		t.Fatal("the document reports the same in both languages, so the test shows nothing")
	}
	cache, _ := recordCacheIn(t)
	requireSameAnswers(t, "inline", answersOf(t, recordingService(t, cache), inline), wantInline)
	requireSameAnswers(t, "file after inline", answersOf(t, recordingService(t, cache), file), wantFile)
	requireSameAnswers(t, "inline after file", answersOf(t, recordingService(t, cache), inline), wantInline)
}

// A record that is missing, unreadable, or not of the model's documents is a
// miss: the model is analyzed, answers as a fresh parse, and is recorded again.
func TestAMissingOrCorruptRecordFallsBackToAnalysis(t *testing.T) {
	request := &pb.ParseSourcesRequest{Documents: inlineDocuments(recordedModels[2]...)}
	want := answersOf(t, mustNewService(t, 10), request)
	damage := map[string]func(t *testing.T, srv *Service, file string){
		"evicted": func(t *testing.T, _ *Service, file string) {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
		},
		"corrupt": func(t *testing.T, _ *Service, file string) {
			if err := os.WriteFile(file, []byte("not a record"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"truncated": func(t *testing.T, _ *Service, file string) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, data[:len(data)/2], 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"fewer documents": func(t *testing.T, srv *Service, _ string) { plantRecord(t, srv, request, "lib.sysml") },
		"other documents": func(t *testing.T, srv *Service, _ string) { plantRecord(t, srv, request, "lib.sysml", "other.sysml") },
	}
	for label, damageRecord := range damage {
		cache, dir := recordCacheIn(t)
		answersOf(t, recordingService(t, cache), request)
		files := recordFiles(t, dir)
		if len(files) != 1 {
			t.Fatalf("%s: %d records written, want 1", label, len(files))
		}
		srv := recordingService(t, cache)
		damageRecord(t, srv, files[0])
		requireSameAnswers(t, label, answersOf(t, srv, request), want)
		// The analysis that answered is on record again, for the next process.
		requireSameAnswers(t, label+", read again", answersOf(t, recordingService(t, cache), request), want)
		idx, _ := srv.libIndexes.get()
		key, _ := srv.modelRecordKey(want.parse.ModelHash, idx)
		if rec, ok := cache.LoadModel(key); !ok || len(rec.Documents) != 2 || rec.Documents[1].Name != "top.sysml" {
			t.Errorf("%s: the model was not recorded again", label)
		}
	}
}

// A model that does not parse clean is not analyzed, so it has no record; and
// a service without a cache writes none.
func TestNoRecordWithoutAnAnalysisOrACache(t *testing.T) {
	cache, dir := recordCacheIn(t)
	broken := &pb.ParseSourcesRequest{Documents: inlineDocuments("lib.sysml", "package Lib {\n\tpart def Engine {\n", "top.sysml", sourcesTop)}
	requireSameAnswers(t, "unparsed", answersOf(t, recordingService(t, cache), broken), answersOf(t, mustNewService(t, 10), broken))
	answersOf(t, recordingService(t, nil), &pb.ParseSourcesRequest{Documents: inlineDocuments(recordedModels[0]...)})
	if files := recordFiles(t, dir); len(files) != 0 {
		t.Fatalf("records written: %v", files)
	}
}

// BenchmarkParseSourcesRecorded is a service's first parse of examples/self-model,
// analyzing it and reading the record a process before it wrote.
func BenchmarkParseSourcesRecorded(b *testing.B) {
	paths, err := filepath.Glob("../../../examples/self-model/*.sysml")
	if err != nil || len(paths) == 0 {
		b.Fatalf("no model to parse: %v", err)
	}
	request := &pb.ParseSourcesRequest{}
	for _, path := range paths {
		request.Documents = append(request.Documents, &pb.SourceDocument{Source: &pb.SourceDocument_FilePath{FilePath: path}})
	}
	cache, err := libs.NewCacheIn(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	for _, run := range []struct {
		name  string
		cache *libs.Cache
	}{{"analyzed", nil}, {"recorded", cache}} {
		b.Run(run.name, func(b *testing.B) {
			for i := 0; i < b.N+1; i++ {
				b.StopTimer()
				srv, err := NewService(10, "bench", WithRecordCache(run.cache))
				if err != nil {
					b.Fatal(err)
				}
				<-srv.Prewarm()
				// The parse before the first one timed writes the record.
				if i > 0 {
					b.StartTimer()
				}
				if _, err := srv.ParseSources(context.Background(), request); err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				srv.Close()
			}
		})
	}
}

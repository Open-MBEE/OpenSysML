package grpc

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGetServerInfo verifies the service reports its build version and the
// capabilities a client can require.
func TestGetServerInfo(t *testing.T) {
	srv := mustNewService(t, 10)

	resp, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo failed: %v", err)
	}
	if resp.Version != "test" {
		t.Errorf("version = %q, want the version the service was built with", resp.Version)
	}
	if !slices.Contains(resp.Capabilities, CapabilityTypeFacts) {
		t.Errorf("capabilities = %v, want it to contain %q", resp.Capabilities, CapabilityTypeFacts)
	}
	for _, capability := range []string{CapabilityAuthoring, CapabilityInlineLanguage} {
		if !slices.Contains(resp.Capabilities, capability) {
			t.Errorf("capabilities = %v, want it to contain %q", resp.Capabilities, capability)
		}
	}
}

// TestGetServerInfoTypeFactsCapabilityIsHonest verifies the reported
// type_facts capability matches what SymbolInfo actually carries, so a client
// that requires it is not told yes by a build that answers without type facts.
func TestGetServerInfoTypeFactsCapabilityIsHonest(t *testing.T) {
	srv := mustNewService(t, 10)

	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo failed: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityTypeFacts) {
		t.Skip("build does not claim the type_facts capability")
	}

	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: "part def Engine { attribute power = 300.0; }"},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	sym, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "Engine::power",
	})
	if err != nil {
		t.Fatalf("GetSymbol failed: %v", err)
	}
	if sym.Symbol == nil {
		t.Fatalf("GetSymbol returned no symbol: %s", sym.Error)
	}
	if sym.Symbol.TypeInfo == nil {
		t.Error("type_facts is claimed, but a typed attribute carries no type_info")
	}
}

// TestParseFile_ValidSyntax verifies ParseFile on well-formed input
func TestParseFile_ValidSyntax(t *testing.T) {
	srv := mustNewService(t, 10) // Cache size 10

	content := `
package Vehicle {
  part def Engine;
  part def VehicleDef;
  part vehicle : VehicleDef;
}
`

	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-hash-1",
	}

	resp, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if resp.ModelHash == "" {
		t.Error("expected non-empty model_hash")
	}

	if resp.Root == nil {
		t.Fatal("expected Root to be populated")
	}

	if len(resp.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics, got %d", len(resp.Diagnostics))
	}

	if resp.Error != "" {
		t.Errorf("expected no error, got: %s", resp.Error)
	}
}

func TestParseFile_InlineKerMLUsesImplicitLibraryBases(t *testing.T) {
	srv := mustNewService(t, 10)
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:   &pb.ParseFileRequest_Content{Content: "class C; struct S;"},
		Language: "kerml",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("unexpected diagnostic: %s", d.Message)
		}
	}
	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatalf("model %s was not cached", resp.ModelHash)
	}
	for _, name := range []string{"C", "S"} {
		syms := cached.Index.LookupQualified(name)
		if len(syms) != 1 {
			t.Fatalf("LookupQualified(%s) returned %d symbols, want one", name, len(syms))
		}
		if inherited, ok := cached.SymbolContext().Semantics.LookupMember(syms[0], "endShot"); !ok {
			t.Fatalf("%s does not inherit endShot from the KerML library", name)
		} else if got := cached.Index.GetFQN(inherited); got != "Occurrences::Occurrence::endShot" {
			t.Fatalf("inherited endShot for %s = %q, want Occurrences::Occurrence::endShot", name, got)
		}
	}
}

// TestParseFile_SyntaxErrors verifies ParseFile handles malformed input gracefully
func TestParseFile_SyntaxErrors(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `
package Vehicle {
  part def Engine  // Missing semicolon
  invalid syntax here @#$
}
`

	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "test-hash-error",
	}

	resp, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("ParseFile should not return gRPC error, got: %v", err)
	}

	if resp.ModelHash == "" {
		t.Error("expected model_hash even with syntax errors")
	}

	if len(resp.Diagnostics) == 0 {
		t.Error("expected diagnostics for syntax errors")
	}

	// Verify diagnostic structure
	for _, diag := range resp.Diagnostics {
		if diag.Severity == "" {
			t.Error("diagnostic missing severity")
		}
		if diag.Message == "" {
			t.Error("diagnostic missing message")
		}
		if diag.Span == nil {
			t.Error("diagnostic missing span")
		}
	}
}

// TestParseFile_FileNotFound verifies ParseFile returns error when file doesn't exist
func TestParseFile_FileNotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_FilePath{FilePath: "/nonexistent/file.sysml"},
		ContentHash: "test-hash-notfound",
	}

	resp, err := srv.ParseFile(context.Background(), req)
	if err == nil {
		t.Fatal("expected gRPC error for missing file")
	}

	if resp != nil {
		t.Error("expected nil response on file error")
	}
}

// TestParseFile_CacheHit verifies cache reuses parsed models
func TestParseFile_CacheHit(t *testing.T) {
	srv := mustNewService(t, 10)

	content := `package Test { part def A; }`
	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "cache-test-hash",
	}

	// First call - cache miss
	resp1, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("first ParseFile failed: %v", err)
	}

	// Second call - cache hit
	resp2, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("second ParseFile failed: %v", err)
	}

	if resp1.ModelHash != resp2.ModelHash {
		t.Error("expected same model_hash on cache hit")
	}
}

// TestParseFileCachesByContentRead verifies the cache is keyed by what the service
// read: an unchanged parse is reused, and a mismatched hash cannot serve another
// model.
func TestParseFileCachesByContentRead(t *testing.T) {
	srv := mustNewService(t, 10)

	const a = `package A { part def PartA; }`
	const b = `package B { part def PartB; }`

	first, err := srv.ParseFile(context.Background(),
		&pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: a}})
	if err != nil {
		t.Fatalf("ParseFile(a) failed: %v", err)
	}

	cached, ok := srv.cache.Get(first.ModelHash)
	if !ok {
		t.Fatal("parsed model was not cached")
	}

	other, err := srv.ParseFile(context.Background(),
		&pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: b}})
	if err != nil {
		t.Fatalf("ParseFile(b) failed: %v", err)
	}

	// The same content parses to the cached record, even carrying b's hash.
	again, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: a},
		ContentHash: other.ModelHash,
	})
	if err != nil {
		t.Fatalf("second ParseFile(a) failed: %v", err)
	}
	if again.ModelHash != first.ModelHash {
		t.Errorf("model_hash = %q, want %q", again.ModelHash, first.ModelHash)
	}
	if reused, _ := srv.cache.Get(again.ModelHash); reused != cached {
		t.Error("expected the cached model to be reused, not re-parsed")
	}

	sym, err := srv.GetSymbol(context.Background(),
		&pb.GetSymbolRequest{ModelHash: again.ModelHash, SymbolId: "A::PartA"})
	if err != nil || sym.Error != "" {
		t.Fatalf("GetSymbol(A::PartA) = %v, %q", err, sym.GetError())
	}
}

// TestParseFileCachesPerFileName verifies the same content read from two paths
// keeps a record each, so both hit the cache and each names its own file.
func TestParseFileCachesPerFileName(t *testing.T) {
	srv := mustNewService(t, 10)

	dir := t.TempDir()
	const content = `package Dup { part def Thing; }`
	paths := []string{filepath.Join(dir, "one.sysml"), filepath.Join(dir, "two.sysml")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) failed: %v", path, err)
		}
	}

	hashes := map[string]string{}
	for _, path := range paths {
		req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}}
		resp, err := srv.ParseFile(context.Background(), req)
		if err != nil {
			t.Fatalf("ParseFile(%s) failed: %v", path, err)
		}
		hashes[path] = resp.ModelHash

		record, ok := srv.cache.Get(resp.ModelHash)
		if !ok {
			t.Fatalf("model for %s was not cached", path)
		}
		if got := record.Primary().Source.Name(); got != path {
			t.Errorf("cached source name = %q, want %q", got, path)
		}

		// The same path parses to that record rather than replacing it.
		second, err := srv.ParseFile(context.Background(), req)
		if err != nil {
			t.Fatalf("second ParseFile(%s) failed: %v", path, err)
		}
		if reused, _ := srv.cache.Get(second.ModelHash); reused != record {
			t.Errorf("expected %s to hit the cache, not re-parse", path)
		}
	}

	if hashes[paths[0]] == hashes[paths[1]] {
		t.Error("expected a model hash per file name, got one for both")
	}
	// The first file's record survived the second file's parse.
	if record, ok := srv.cache.Get(hashes[paths[0]]); !ok || record.Primary().Source.Name() != paths[0] {
		t.Errorf("first file's record = %v, want one naming %q", record, paths[0])
	}
}

// TestGetSymbol_Found verifies GetSymbol retrieves known symbols
func TestGetSymbol_Found(t *testing.T) {
	srv := mustNewService(t, 10)

	// First parse a model
	content := `
package Vehicle {
  part def Engine;
}
`
	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "symbol-test-hash",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Query for the Engine symbol
	symReq := &pb.GetSymbolRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "Vehicle::Engine",
	}

	symResp, err := srv.GetSymbol(context.Background(), symReq)
	if err != nil {
		t.Fatalf("GetSymbol failed: %v", err)
	}

	if symResp.Error != "" {
		t.Errorf("expected no error, got: %s", symResp.Error)
	}

	if symResp.Symbol == nil {
		t.Fatal("expected symbol to be populated")
	}

	// Symbol kind from symbols.Kind.String() - check it's a definition
	if symResp.Symbol.Kind != "partDef" {
		t.Errorf("expected partDef, got: %s", symResp.Symbol.Kind)
	}
}

// TestGetSymbol_NotFound verifies GetSymbol handles missing symbols
func TestGetSymbol_NotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse a model
	content := `package Vehicle { part def Engine; }`
	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "symbol-notfound-hash",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Query for non-existent symbol
	symReq := &pb.GetSymbolRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "NonExistent::Symbol",
	}

	symResp, err := srv.GetSymbol(context.Background(), symReq)
	if err != nil {
		t.Fatalf("GetSymbol should not return gRPC error, got: %v", err)
	}

	if symResp.Error == "" {
		t.Error("expected error field to be populated for missing symbol")
	}

	if symResp.Symbol != nil {
		t.Error("expected symbol to be nil when not found")
	}
}

// TestGetSymbol_InvalidModelHash verifies GetSymbol fails on unknown model hash
func TestGetSymbol_InvalidModelHash(t *testing.T) {
	srv := mustNewService(t, 10)

	req := &pb.GetSymbolRequest{
		ModelHash: "invalid-hash-not-in-cache",
		SymbolId:  "SomeSymbol",
	}

	resp, err := srv.GetSymbol(context.Background(), req)
	if err == nil {
		t.Fatal("expected gRPC error for invalid model hash")
	}

	if resp != nil {
		t.Error("expected nil response on model not found")
	}
}

// TestGetDiagnostics verifies diagnostics retrieval
func TestGetDiagnostics(t *testing.T) {
	srv := mustNewService(t, 10)

	// Parse model with errors
	content := `package Bad { invalid }`
	parseReq := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: "diag-test-hash",
	}

	parseResp, err := srv.ParseFile(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Get diagnostics
	diagReq := &pb.DiagnosticsRequest{
		ModelHash: parseResp.ModelHash,
	}

	diagResp, err := srv.GetDiagnostics(context.Background(), diagReq)
	if err != nil {
		t.Fatalf("GetDiagnostics failed: %v", err)
	}

	if diagResp.Error != "" {
		t.Errorf("expected no error, got: %s", diagResp.Error)
	}

	if len(diagResp.Diagnostics) == 0 {
		t.Error("expected diagnostics to be populated")
	}
}

// TestGetDiagnostics_InvalidModelHash verifies error on unknown model
func TestGetDiagnostics_InvalidModelHash(t *testing.T) {
	srv := mustNewService(t, 10)

	req := &pb.DiagnosticsRequest{
		ModelHash: "unknown-hash",
	}

	resp, err := srv.GetDiagnostics(context.Background(), req)
	if err == nil {
		t.Fatal("expected gRPC error for invalid model hash")
	}

	if resp != nil {
		t.Error("expected nil response")
	}
}

// TestParseFile_FromFile verifies ParseFile can read from filesystem
func TestParseFile_FromFile(t *testing.T) {
	srv := mustNewService(t, 10)

	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.sysml")
	content := `package FromFile { part def TestPart; }`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_FilePath{FilePath: testFile},
		ContentHash: "file-test-hash",
	}

	resp, err := srv.ParseFile(context.Background(), req)
	if err != nil {
		t.Fatalf("ParseFile from file failed: %v", err)
	}

	if resp.ModelHash == "" {
		t.Error("expected model_hash")
	}

	if len(resp.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics, got %d", len(resp.Diagnostics))
	}
}

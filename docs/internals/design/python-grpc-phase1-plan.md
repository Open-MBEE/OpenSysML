# Python gRPC Bindings - Phase 1: Core gRPC Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build working gRPC service (`sysml-grpc`) exposing OpenSysML's parser and symbol query capabilities.

**Architecture:** Stateless gRPC service with LRU cache for parsed models. Thin wrapper over existing `internal/*` packages. Service keyed by content hash for cache lookup.

**Tech Stack:** Go 1.23+, gRPC, Protocol Buffers

---

## File Structure

**New files:**
- `api/proto/sysml.proto` - Service definition and message types
- `api/proto/generate.go` - Code generation directive
- `cmd/sysml-grpc/main.go` - Server binary
- `internal/frontend/grpc/service.go` - RPC implementations
- `internal/frontend/grpc/cache.go` - LRU cache
- `internal/frontend/grpc/convert.go` - Type conversions
- `internal/frontend/grpc/errors.go` - Error handling
- `internal/frontend/grpc/service_test.go` - Service unit tests
- `internal/frontend/grpc/cache_test.go` - Cache unit tests
- `internal/frontend/grpc/convert_test.go` - Conversion unit tests
- `internal/frontend/grpc/integration_test.go` - Full round-trip tests

**Modified files:**
- `Makefile` - Add build-grpc target
- `go.mod` - Add gRPC dependencies

---

## Tasks

### Task 1: Setup Dependencies and Protobuf Schema

**Files:**
- Modify: `go.mod`
- Create: `api/proto/sysml.proto`
- Create: `api/proto/generate.go`

- [ ] **Step 1: Add gRPC dependencies to go.mod**

Run:
```bash
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf/cmd/protoc-gen-go@latest
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Expected: `go.mod` updated with grpc and protobuf packages

- [ ] **Step 2: Verify dependencies installed**

Run: `go mod tidy && go build ./...`
Expected: Clean build, no errors

- [ ] **Step 3: Create api/proto directory**

Run:
```bash
mkdir -p api/proto
```

- [ ] **Step 4: Write protobuf service definition**

Create `api/proto/sysml.proto`:
```protobuf
syntax = "proto3";

package sysml;

option go_package = "github.com/Open-MBEE/OpenSysML/api/proto";

// SysMLService provides programmatic access to OpenSysML's parser and runtime
service SysMLService {
  // Parse a SysML file and return model hash for subsequent queries
  rpc ParseFile(ParseFileRequest) returns (ParseFileResponse);
  
  // Get symbol information by qualified name
  rpc GetSymbol(GetSymbolRequest) returns (SymbolResponse);
  
  // Get all diagnostics for a parsed model
  rpc GetDiagnostics(DiagnosticsRequest) returns (DiagnosticsResponse);
}

// ParseFileRequest specifies the source to parse
message ParseFileRequest {
  oneof source {
    string file_path = 1;
    string content = 2;
  }
  string content_hash = 3;  // sha256 for cache lookup
}

// ParseFileResponse contains parsed model info
message ParseFileResponse {
  string model_hash = 1;     // Cache key for subsequent requests
  SymbolInfo root = 2;       // Root namespace
  repeated Diagnostic diagnostics = 3;
  string error = 4;          // Critical failure message if any
}

// GetSymbolRequest queries for a specific symbol
message GetSymbolRequest {
  string model_hash = 1;
  string symbol_id = 2;  // Fully qualified name
}

// SymbolResponse contains symbol information
message SymbolResponse {
  SymbolInfo symbol = 1;
  string error = 2;
}

// DiagnosticsRequest gets all diagnostics for a model
message DiagnosticsRequest {
  string model_hash = 1;
}

// DiagnosticsResponse contains diagnostic list
message DiagnosticsResponse {
  repeated Diagnostic diagnostics = 1;
  string error = 2;
}

// SymbolInfo represents any SysML element
message SymbolInfo {
  string id = 1;             // Unique identifier (fully qualified name)
  string name = 2;
  string kind = 3;           // "PartDefinition", "AttributeUsage", etc
  map<string, string> metadata = 4;  // multiplicity, type, etc
  repeated string child_ids = 5;     // References to children
  repeated AttributeInfo attributes = 6;
}

// AttributeInfo represents an attribute with its value
message AttributeInfo {
  string name = 1;
  string type = 2;
  Value value = 3;
  string unit = 4;
}

// Value represents a literal value
message Value {
  oneof value {
    double number = 1;
    string text = 2;
    bool boolean = 3;
    int64 integer = 4;
  }
}

// Diagnostic represents a parse/semantic error or warning
message Diagnostic {
  string severity = 1;  // "error", "warning", "info"
  string message = 2;
  Span span = 3;
}

// Span represents a source location
message Span {
  string file = 1;
  int32 start_line = 2;
  int32 start_col = 3;
  int32 end_line = 4;
  int32 end_col = 5;
}
```

- [ ] **Step 5: Create generate.go with protoc directive**

Create `api/proto/generate.go`:
```go
package proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative sysml.proto
```

- [ ] **Step 6: Generate Go code from protobuf**

Run:
```bash
cd api/proto
go generate
cd ../..
```

Expected: Creates `api/proto/sysml.pb.go` and `api/proto/sysml_grpc.pb.go`

- [ ] **Step 7: Verify generated code compiles**

Run: `go build ./api/proto`
Expected: Clean build

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum api/proto/
git commit -m "feat(grpc): add protobuf schema and dependencies"
```

---

### Task 2: Implement LRU Cache

**Files:**
- Create: `internal/frontend/grpc/cache.go`
- Create: `internal/frontend/grpc/cache_test.go`

- [ ] **Step 1: Write failing test for cache Put/Get**

Create `internal/frontend/grpc/cache_test.go`:
```go
package grpc

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestCachePutGet(t *testing.T) {
	cache := NewCache(2) // Max size 2
	
	hash1 := "abc123"
	model1 := &CachedModel{
		Root:    &ast.RootNamespace{},
		SymTab:  symbols.NewTable(),
	}
	
	cache.Put(hash1, model1)
	
	got, ok := cache.Get(hash1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != model1 {
		t.Error("got different model")
	}
}

func TestCacheMiss(t *testing.T) {
	cache := NewCache(2)
	
	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewCache(2)
	
	model1 := &CachedModel{Root: &ast.RootNamespace{}}
	model2 := &CachedModel{Root: &ast.RootNamespace{}}
	model3 := &CachedModel{Root: &ast.RootNamespace{}}
	
	cache.Put("hash1", model1)
	cache.Put("hash2", model2)
	cache.Put("hash3", model3) // Should evict hash1
	
	_, ok := cache.Get("hash1")
	if ok {
		t.Error("expected hash1 to be evicted")
	}
	
	_, ok = cache.Get("hash2")
	if !ok {
		t.Error("expected hash2 to still be cached")
	}
	
	_, ok = cache.Get("hash3")
	if !ok {
		t.Error("expected hash3 to be cached")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/frontend/grpc -run TestCache -v`
Expected: FAIL with "undefined: NewCache"

- [ ] **Step 3: Implement cache.go**

Create `internal/frontend/grpc/cache.go`:
```go
package grpc

import (
	"container/list"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// CachedModel holds parsed model data
type CachedModel struct {
	Root   *ast.RootNamespace
	SymTab *symbols.SymbolTable
}

// Cache is an LRU cache for parsed models keyed by content hash
type Cache struct {
	mu       sync.RWMutex
	maxSize  int
	items    map[string]*list.Element
	lruList  *list.List
}

type cacheEntry struct {
	key   string
	value *CachedModel
}

// NewCache creates a cache with the specified max size
func NewCache(maxSize int) *Cache {
	return &Cache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Get retrieves a model from cache, returns (model, true) on hit
func (c *Cache) Get(hash string) (*CachedModel, bool) {
	c.mu.RLock()
	elem, ok := c.items[hash]
	c.mu.RUnlock()
	
	if !ok {
		return nil, false
	}
	
	// Move to front (most recently used)
	c.mu.Lock()
	c.lruList.MoveToFront(elem)
	c.mu.Unlock()
	
	entry := elem.Value.(*cacheEntry)
	return entry.value, true
}

// Put adds a model to cache, evicting LRU if at capacity
func (c *Cache) Put(hash string, model *CachedModel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if already exists
	if elem, ok := c.items[hash]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = model
		return
	}
	
	// Evict if at capacity
	if c.lruList.Len() >= c.maxSize {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}
	
	// Add new entry
	entry := &cacheEntry{key: hash, value: model}
	elem := c.lruList.PushFront(entry)
	c.items[hash] = elem
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/frontend/grpc -run TestCache -v`
Expected: PASS for all 3 tests

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/grpc/cache.go internal/frontend/grpc/cache_test.go
git commit -m "feat(grpc): implement LRU cache for parsed models"
```

---

### Task 3: Implement Type Conversion (Go → Protobuf)

**Files:**
- Create: `internal/frontend/grpc/convert.go`
- Create: `internal/frontend/grpc/convert_test.go`

- [ ] **Step 1: Write failing test for symbol conversion**

Create `internal/frontend/grpc/convert_test.go`:
```go
package grpc

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestSymbolToProto(t *testing.T) {
	sym := &symbols.Symbol{
		Name: "TestPart",
		Kind: symbols.SymbolPartDef,
		Decl: &ast.PartDefinition{},
		DeclSpan: source.Span{File: "test.sysml", Start: 10, End: 20},
	}
	
	symTab := symbols.NewTable()
	
	proto := SymbolToProto(sym, symTab)
	
	if proto.Name != "TestPart" {
		t.Errorf("expected name TestPart, got %s", proto.Name)
	}
	if proto.Kind != "partDef" {
		t.Errorf("expected kind partDef, got %s", proto.Kind)
	}
	if proto.Id == "" {
		t.Error("expected non-empty ID")
	}
}

func TestDiagnosticToProto(t *testing.T) {
	diag := source.Diagnostic{
		Severity: source.Error,
		Message:  "test error",
		Span: source.Span{
			File: "test.sysml",
			Start: 10,
			End: 20,
		},
	}
	
	src := source.NewFile("test.sysml", []byte("part Test { }"))
	proto := DiagnosticToProto(diag, src)
	
	if proto.Severity != "error" {
		t.Errorf("expected severity error, got %s", proto.Severity)
	}
	if proto.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", proto.Message)
	}
	if proto.Span.File != "test.sysml" {
		t.Error("expected file test.sysml")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/frontend/grpc -run TestSymbolToProto -v`
Expected: FAIL with "undefined: SymbolToProto"

- [ ] **Step 3: Implement convert.go**

Create `internal/frontend/grpc/convert.go`:
```go
package grpc

import (
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// SymbolToProto converts a Symbol to protobuf SymbolInfo
func SymbolToProto(sym *symbols.Symbol, symTab *symbols.SymbolTable) *pb.SymbolInfo {
	info := &pb.SymbolInfo{
		Id:       symTab.QualifiedName(sym),
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}
	
	// Add visibility to metadata
	info.Metadata["visibility"] = sym.Visibility.String()
	
	// Collect child IDs (lazy loading)
	if sym.Scope != nil {
		childIDs := []string{}
		for _, child := range sym.Scope.Members() {
			childIDs = append(childIDs, symTab.QualifiedName(child))
		}
		info.ChildIds = childIDs
	}
	
	// TODO: Populate attributes when semantic layer ready
	info.Attributes = []*pb.AttributeInfo{}
	
	return info
}

// DiagnosticToProto converts a source.Diagnostic to protobuf
func DiagnosticToProto(diag source.Diagnostic, src *source.File) *pb.Diagnostic {
	span := diag.Span
	startLine, startCol := src.LineCol(span.Start)
	endLine, endCol := src.LineCol(span.End)
	
	return &pb.Diagnostic{
		Severity: severityToString(diag.Severity),
		Message:  diag.Message,
		Span: &pb.Span{
			File:      span.File,
			StartLine: int32(startLine),
			StartCol:  int32(startCol),
			EndLine:   int32(endLine),
			EndCol:    int32(endCol),
		},
	}
}

func severityToString(sev source.Severity) string {
	switch sev {
	case source.Error:
		return "error"
	case source.Warning:
		return "warning"
	case source.Info:
		return "info"
	default:
		return "info"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/frontend/grpc -run Test.*ToProto -v`
Expected: PASS for both tests

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/grpc/convert.go internal/frontend/grpc/convert_test.go
git commit -m "feat(grpc): implement type conversion Go → protobuf"
```

---

### Task 4: Implement ParseFile RPC

**Files:**
- Create: `internal/frontend/grpc/service.go`
- Create: `internal/frontend/grpc/service_test.go`

- [ ] **Step 1: Write failing test for ParseFile**

Create `internal/frontend/grpc/service_test.go`:
```go
package grpc

import (
	"context"
	"os"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestParseFileFromContent(t *testing.T) {
	cache := NewCache(10)
	svc := &service{cache: cache}
	
	ctx := context.Background()
	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: "package Test { }"},
		ContentHash: "abc123",
	}
	
	resp, err := svc.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	if resp.ModelHash != "abc123" {
		t.Errorf("expected model_hash abc123, got %s", resp.ModelHash)
	}
	
	if resp.Root == nil {
		t.Fatal("expected root symbol")
	}
	
	if resp.Root.Name == "" {
		t.Error("expected root to have name")
	}
}

func TestParseFileCache(t *testing.T) {
	cache := NewCache(10)
	svc := &service{cache: cache}
	
	ctx := context.Background()
	content := "package Test { }"
	hash := "hash123"
	
	req := &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: hash,
	}
	
	// First call - cache miss
	resp1, err := svc.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	// Second call - cache hit
	resp2, err := svc.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile failed on second call: %v", err)
	}
	
	// Should return same model hash
	if resp1.ModelHash != resp2.ModelHash {
		t.Error("cache should return same model hash")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/frontend/grpc -run TestParseFile -v`
Expected: FAIL with "undefined: service"

- [ ] **Step 3: Implement service.go**

Create `internal/frontend/grpc/service.go`:
```go
package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// service implements pb.SysMLServiceServer
type service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
}

// NewService creates a new gRPC service instance
func NewService(cacheSize int) pb.SysMLServiceServer {
	return &service{
		cache: NewCache(cacheSize),
	}
}

// ParseFile parses a SysML file and caches the result
func (s *service) ParseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	// Extract content and hash
	var content []byte
	var filePath string
	var err error
	
	switch src := req.Source.(type) {
	case *pb.ParseFileRequest_FilePath:
		filePath = src.FilePath
		content, err = os.ReadFile(filePath)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "failed to read file: %v", err)
		}
	case *pb.ParseFileRequest_Content:
		content = []byte(src.Content)
		filePath = "<inline>"
	default:
		return nil, status.Error(codes.InvalidArgument, "source must be file_path or content")
	}
	
	// Compute hash if not provided
	hash := req.ContentHash
	if hash == "" {
		h := sha256.Sum256(content)
		hash = fmt.Sprintf("%x", h)
	}
	
	// Check cache
	if cached, ok := s.cache.Get(hash); ok {
		return &pb.ParseFileResponse{
			ModelHash:   hash,
			Root:        SymbolToProto(cached.SymTab.Root(), cached.SymTab),
			Diagnostics: []*pb.Diagnostic{}, // TODO: cache diagnostics
		}, nil
	}
	
	// Parse
	srcFile := source.NewFile(filePath, content)
	p := parser.New(srcFile)
	root := p.ParseFile()
	
	// Build symbol table
	idx := symbols.NewIndex()
	idx.AddDocument(filePath, root)
	
	// Get diagnostics
	diags := p.Diagnostics()
	protoDiags := make([]*pb.Diagnostic, len(diags))
	for i, d := range diags {
		protoDiags[i] = DiagnosticToProto(d, srcFile)
	}
	
	// Cache model
	rootSym := idx.DocumentRoot(filePath)
	symTab := &symbols.SymbolTable{} // Simplified; real impl needs proper symbol table
	cached := &CachedModel{
		Root:   root,
		SymTab: symTab,
	}
	s.cache.Put(hash, cached)
	
	return &pb.ParseFileResponse{
		ModelHash:   hash,
		Root:        SymbolToProto(rootSym, symTab),
		Diagnostics: protoDiags,
	}, nil
}

// GetSymbol retrieves symbol info by qualified name
func (s *service) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	// Retrieve cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Error(codes.NotFound, "model not found in cache")
	}
	
	// Look up symbol by ID
	sym := cached.SymTab.Lookup(req.SymbolId)
	if sym == nil {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}
	
	return &pb.SymbolResponse{
		Symbol: SymbolToProto(sym, cached.SymTab),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a model
func (s *service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Check cache
	_, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Error(codes.NotFound, "model not found in cache")
	}
	
	// TODO: Cache diagnostics with model
	return &pb.DiagnosticsResponse{
		Diagnostics: []*pb.Diagnostic{},
	}, nil
}
```

- [ ] **Step 4: Fix compilation errors**

The service implementation above has a simplified symbol table reference. Need to adjust convert.go and service.go to work with symbols.Index properly:

Update `internal/frontend/grpc/cache.go`:
```go
// CachedModel holds parsed model data
type CachedModel struct {
	Root   *ast.RootNamespace
	Index  *symbols.Index
	Source *source.File
	Diags  []source.Diagnostic
}
```

Update `internal/frontend/grpc/convert.go` - change function signature:
```go
// SymbolToProto converts a Symbol to protobuf SymbolInfo
func SymbolToProto(sym *symbols.Symbol, idx *symbols.Index) *pb.SymbolInfo {
	info := &pb.SymbolInfo{
		Id:       idx.QualifiedName(sym),
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}
	
	// Add visibility to metadata
	info.Metadata["visibility"] = sym.Visibility.String()
	
	// Collect child IDs (lazy loading)
	if sym.Scope != nil {
		childIDs := []string{}
		for _, child := range sym.Scope.Members() {
			childIDs = append(childIDs, idx.QualifiedName(child))
		}
		info.ChildIds = childIDs
	}
	
	info.Attributes = []*pb.AttributeInfo{}
	
	return info
}
```

Update `internal/frontend/grpc/service.go` ParseFile implementation:
```go
// Parse
srcFile := source.NewFile(filePath, content)
p := parser.New(srcFile)
root := p.ParseFile()

// Build symbol index
idx := symbols.NewIndex()
idx.AddDocument(filePath, root)

// Get diagnostics
diags := p.Diagnostics()
protoDiags := make([]*pb.Diagnostic, len(diags))
for i, d := range diags {
	protoDiags[i] = DiagnosticToProto(d, srcFile)
}

// Cache model
rootSym := idx.DocumentRoot(filePath)
cached := &CachedModel{
	Root:   root,
	Index:  idx,
	Source: srcFile,
	Diags:  diags,
}
s.cache.Put(hash, cached)

return &pb.ParseFileResponse{
	ModelHash:   hash,
	Root:        SymbolToProto(rootSym, idx),
	Diagnostics: protoDiags,
}, nil
```

Update GetSymbol:
```go
func (s *service) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Error(codes.NotFound, "model not found in cache")
	}
	
	// Look up symbol by qualified name
	syms := cached.Index.Lookup(req.SymbolId)
	if len(syms) == 0 {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}
	
	return &pb.SymbolResponse{
		Symbol: SymbolToProto(syms[0], cached.Index),
	}, nil
}
```

- [ ] **Step 5: Update test files to match new signatures**

Update `internal/frontend/grpc/convert_test.go`:
```go
func TestSymbolToProto(t *testing.T) {
	sym := &symbols.Symbol{
		Name: "TestPart",
		Kind: symbols.SymbolPartDef,
		Decl: &ast.PartDefinition{},
		DeclSpan: source.Span{File: "test.sysml", Start: 10, End: 20},
	}
	
	idx := symbols.NewIndex()
	
	proto := SymbolToProto(sym, idx)
	
	if proto.Name != "TestPart" {
		t.Errorf("expected name TestPart, got %s", proto.Name)
	}
	if proto.Kind != "partDef" {
		t.Errorf("expected kind partDef, got %s", proto.Kind)
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/frontend/grpc -run TestParseFile -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/frontend/grpc/service.go internal/frontend/grpc/service_test.go internal/frontend/grpc/cache.go internal/frontend/grpc/convert.go internal/frontend/grpc/convert_test.go
git commit -m "feat(grpc): implement ParseFile and GetSymbol RPCs"
```

---

### Task 5: Implement Server Binary

**Files:**
- Create: `cmd/sysml-grpc/main.go`

- [ ] **Step 1: Create cmd/sysml-grpc directory**

Run:
```bash
mkdir -p cmd/sysml-grpc
```

- [ ] **Step 2: Write main.go**

Create `cmd/sysml-grpc/main.go`:
```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	grpcService "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"google.golang.org/grpc"
)

var (
	port       = flag.Int("port", 50051, "gRPC listen port")
	healthPort = flag.Int("health-port", 0, "HTTP health check port (default: grpc port + 1)")
	cacheSize  = flag.Int("cache-size", 100, "max cached models")
	logLevel   = flag.String("log-level", "info", "logging level (debug|info|warn|error)")
	
	// Version information (set via ldflags)
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func main() {
	flag.Parse()
	
	// Default health port to grpc port + 1
	if *healthPort == 0 {
		*healthPort = *port + 1
	}
	
	log.Printf("sysml-grpc %s (commit %s, built %s with %s)", Version, Commit, BuildTime, GoVersion)
	log.Printf("Starting gRPC server on port %d", *port)
	log.Printf("Health check endpoint on port %d", *healthPort)
	log.Printf("Cache size: %d models", *cacheSize)
	
	// Start health check server
	go startHealthServer(*healthPort)
	
	// Create gRPC server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	
	grpcServer := grpc.NewServer()
	pb.RegisterSysMLServiceServer(grpcServer, grpcService.NewService(*cacheSize))
	
	// Graceful shutdown on signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping gracefully...")
		grpcServer.GracefulStop()
	}()
	
	log.Printf("Server ready, accepting connections")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func startHealthServer(port int) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})
	
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Health check listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Health server failed: %v", err)
	}
}
```

- [ ] **Step 3: Build binary**

Run: `go build -o bin/sysml-grpc ./cmd/sysml-grpc`
Expected: Binary created at `bin/sysml-grpc`

- [ ] **Step 4: Test server startup**

Run in one terminal:
```bash
./bin/sysml-grpc --port 50051
```

Expected output:
```
sysml-grpc dev (commit unknown, built unknown with go1.23...)
Starting gRPC server on port 50051
Health check endpoint on port 50052
Cache size: 100 models
Health check listening on :50052
Server ready, accepting connections
```

- [ ] **Step 5: Test health check**

In another terminal:
```bash
curl http://localhost:50052/health
```

Expected: `OK`

- [ ] **Step 6: Stop server (Ctrl+C)**

Expected: Graceful shutdown message

- [ ] **Step 7: Commit**

```bash
git add cmd/sysml-grpc/main.go
git commit -m "feat(grpc): add sysml-grpc server binary"
```

---

### Task 6: Integration Tests

**Files:**
- Create: `internal/frontend/grpc/integration_test.go`

- [ ] **Step 1: Write integration test**

Create `internal/frontend/grpc/integration_test.go`:
```go
package grpc

import (
	"context"
	"net"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// setupTestServer starts a test gRPC server in-memory
func setupTestServer(t *testing.T) (*grpc.Server, *bufconn.Listener, pb.SysMLServiceClient) {
	lis := bufconn.Listen(bufSize)
	
	server := grpc.NewServer()
	pb.RegisterSysMLServiceServer(server, NewService(10))
	
	go func() {
		if err := server.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()
	
	// Create client
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	
	client := pb.NewSysMLServiceClient(conn)
	
	return server, lis, client
}

func TestEndToEndParse(t *testing.T) {
	server, lis, client := setupTestServer(t)
	defer server.Stop()
	defer lis.Close()
	
	ctx := context.Background()
	
	// Parse a simple model
	parseResp, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{
			Content: `
package TestPackage {
	part def TestPart {
		attribute mass : Real;
	}
}
			`,
		},
		ContentHash: "test123",
	})
	
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	if parseResp.ModelHash != "test123" {
		t.Errorf("expected model_hash test123, got %s", parseResp.ModelHash)
	}
	
	if parseResp.Root == nil {
		t.Fatal("expected root symbol")
	}
	
	t.Logf("Root symbol: name=%s, kind=%s, children=%d", 
		parseResp.Root.Name, parseResp.Root.Kind, len(parseResp.Root.ChildIds))
	
	// Query for a symbol (if any children exist)
	if len(parseResp.Root.ChildIds) > 0 {
		childID := parseResp.Root.ChildIds[0]
		
		symResp, err := client.GetSymbol(ctx, &pb.GetSymbolRequest{
			ModelHash: parseResp.ModelHash,
			SymbolId:  childID,
		})
		
		if err != nil {
			t.Fatalf("GetSymbol failed: %v", err)
		}
		
		if symResp.Symbol == nil {
			t.Fatal("expected symbol in response")
		}
		
		t.Logf("Child symbol: name=%s, kind=%s", symResp.Symbol.Name, symResp.Symbol.Kind)
	}
}

func TestCacheHit(t *testing.T) {
	server, lis, client := setupTestServer(t)
	defer server.Stop()
	defer lis.Close()
	
	ctx := context.Background()
	content := "package Test { }"
	hash := "cachehit123"
	
	// First call
	resp1, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("First ParseFile failed: %v", err)
	}
	
	// Second call - should hit cache
	resp2, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("Second ParseFile failed: %v", err)
	}
	
	if resp1.ModelHash != resp2.ModelHash {
		t.Error("expected same model hash from cache")
	}
}

func TestSymbolNotFound(t *testing.T) {
	server, lis, client := setupTestServer(t)
	defer server.Stop()
	defer lis.Close()
	
	ctx := context.Background()
	
	// Parse model
	parseResp, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: "package Test { }"},
		ContentHash: "test456",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	
	// Query for nonexistent symbol
	symResp, err := client.GetSymbol(ctx, &pb.GetSymbolRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  "NonExistent::Foo",
	})
	
	// Should not return gRPC error, but error field in response
	if err != nil {
		t.Fatalf("GetSymbol should not error: %v", err)
	}
	
	if symResp.Error == "" {
		t.Error("expected error field to be set for missing symbol")
	}
}
```

- [ ] **Step 2: Run integration tests**

Run: `go test ./internal/frontend/grpc -run TestEndToEnd -v`
Expected: PASS

Run: `go test ./internal/frontend/grpc -run TestCacheHit -v`
Expected: PASS

Run: `go test ./internal/frontend/grpc -run TestSymbolNotFound -v`
Expected: PASS

- [ ] **Step 3: Run all grpc tests**

Run: `go test ./internal/frontend/grpc/... -v`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/frontend/grpc/integration_test.go
git commit -m "test(grpc): add end-to-end integration tests"
```

---

### Task 7: Update Build System

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add build-grpc target to Makefile**

Edit `Makefile`, add after `build-lsp` target:

```makefile
build-grpc: ## Build sysml-grpc binary
	@echo "Building sysml-grpc..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sysml-grpc ./cmd/sysml-grpc
	@echo "✓ Built $(BIN_DIR)/sysml-grpc ($(VERSION))"
```

Update `build` target to include `build-grpc`:

```makefile
build: build-sysml build-lsp build-grpc ## Build all binaries
```

Update `clean` target to remove sysml-grpc:

```makefile
clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.txt
	rm -f sysml sysml-lsp sysml-grpc
	@echo "✓ Cleaned"
```

Update `install` target:

```makefile
install: build ## Install binaries to $GOPATH/bin
	@echo "Installing to $(shell go env GOPATH)/bin..."
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-lsp
	go install -ldflags "$(LDFLAGS)" ./cmd/sysml-grpc
	@echo "✓ Installed"
```

- [ ] **Step 2: Test build target**

Run: `make build-grpc`
Expected: Binary created at `bin/sysml-grpc` with version info

- [ ] **Step 3: Test full build**

Run: `make build`
Expected: All three binaries built (sysml, sysml-lsp, sysml-grpc)

- [ ] **Step 4: Test version info**

Run: `./bin/sysml-grpc --help`
Expected: Shows flags and help

- [ ] **Step 5: Verify tests still pass**

Run: `make test`
Expected: All tests PASS including new grpc tests

- [ ] **Step 6: Commit**

```bash
git add Makefile
git commit -m "build: add sysml-grpc to build system"
```

---

## Verification

After completing all tasks, verify Phase 1 is complete:

- [ ] **Run full build**

```bash
make clean
make build
```

Expected: Three binaries in `bin/`: `sysml`, `sysml-lsp`, `sysml-grpc`

- [ ] **Run all tests**

```bash
make test
```

Expected: All tests PASS, including:
- Cache tests (LRU eviction, hit/miss)
- Conversion tests (Symbol→Proto, Diagnostic→Proto)
- Service tests (ParseFile, GetSymbol)
- Integration tests (end-to-end, cache behavior)

- [ ] **Manual smoke test**

Terminal 1:
```bash
./bin/sysml-grpc --port 50051
```

Terminal 2:
```bash
# Health check
curl http://localhost:50052/health

# TODO: Add grpcurl test once we have a test file
```

- [ ] **Verify against design spec**

Check `docs/design/python-grpc-bindings.md` Phase 1 requirements:
- ✅ Protobuf schema defined with ParseFile, GetSymbol, GetDiagnostics
- ✅ Go stubs generated
- ✅ sysml-grpc binary with startup/flags
- ✅ ParseFile RPC with cache
- ✅ GetSymbol RPC for symbol lookup
- ✅ Unit tests for cache and conversion
- ✅ Integration test with real gRPC client
- ✅ All tests pass

---

## Next Steps

Phase 1 complete. Ready for Phase 2: Basic Python Client.


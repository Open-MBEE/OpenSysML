# Python gRPC Bindings for OpenSysML

**Date:** 2026-08-04  
**Status:** Design  
**Branch:** `python-bindings-grpc`

---

## 1. Overview

### Purpose and Goals

Enable programmatic access to OpenSysML's SysML v2 parser, semantic engine, and execution runtime from Python. Primary use case: interactive model exploration in Jupyter notebooks with visualization, querying, and runtime simulation capabilities.

**Goals:**
- Pythonic API wrapping OpenSysML's full capabilities
- Zero-friction installation and setup (`pip install opensysml`)
- Rich notebook experience with auto-formatting and DataFrame integration
- Full runtime support (parse, evaluate, instantiate, execute, simulate)

### Primary Use Case

**Jupyter Notebook Exploration:**
```python
import opensysml

# Load model
model = opensysml.load("spacecraft.sysml")

# Navigate structure
part = model.find("SPACECRAFT_WET")
children_df = part.children().to_dataframe()

# Query attributes
mass = part.get_attr("unitMass").value

# Runtime execution
instance = opensysml.instantiate("SPACECRAFT_WET")
print(instance.slots)
```

### High-Level Architecture

```
┌─────────────────────────────────────────┐
│ Python Notebook / Script                │
│   import opensysml                        │
│   model = opensysml.load("A1.sysml")     │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│ Python Client Library (opensysml)         │
│  - Pythonic API                         │
│  - Auto-manages service lifecycle       │
│  - Converts protobuf ↔ Python objects   │
│  - DataFrame integration                │
│  - IPython display hooks                │
└──────────────┬──────────────────────────┘
               │ gRPC/protobuf
┌──────────────▼──────────────────────────┐
│ Go gRPC Service (sysml-grpc)            │
│  - Stateless request/response           │
│  - LRU cache for parsed models          │
│  - Thin wrapper over internal/core/*    │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│ OpenSysML Core                          │
│  internal/core/parser                   │
│  internal/core/semantics                │
│  internal/core/runtime                  │
└─────────────────────────────────────────┘
```

---

## 2. Requirements

### Functional Requirements

**Parse & Query:**
- Load SysML files from path or inline content
- Navigate model structure (packages, parts, attributes, relationships)
- Query symbols by name (fully qualified or relative)
- Access attributes with values, types, units
- Collect diagnostics (errors, warnings) from all passes

**Runtime Execution:**
- Evaluate expressions with context bindings
- Instantiate parts into runtime instances
- Execute actions with inputs/outputs
- Simulate state machines with event sequences
- Retrieve execution traces

**Python Integration:**
- Rich proxy objects for SysML entities (lazy loading)
- Convert collections to Pandas DataFrames
- IPython display integration (formatted trees, tables)
- Typed exceptions for runtime errors
- Diagnostic objects for parse/semantic issues

**Lifecycle Management:**
- Auto-download platform-appropriate binary
- Auto-start service on first use
- Health checking before operations
- Graceful shutdown on exit
- Multi-process coordination (shared service)

### Non-Functional Requirements

**Performance:**
- Sub-second response for parse of medium models (~500 elements)
- Cache effectiveness >90% for repeated queries
- Lazy loading minimizes data transfer

**Usability:**
- Single command installation (`pip install opensysml`)
- No manual service management required
- Clear error messages with source locations
- Familiar Pythonic patterns (properties, methods, exceptions)

**Reliability:**
- Service failures don't crash Python process
- Reconnect on connection loss
- Graceful degradation (diagnostics on parse errors)

### Out of Scope (Initial Release)

- Workspace/multi-file project management (stateless only)
- Model editing/generation from Python (read-only)
- Pre-built analysis functions (mass rollup, etc.) - query API only
- Visualization beyond text/DataFrames (matplotlib integration later)
- Incremental parsing/editing

---

## 3. Architecture

### Two-Tier System

**Tier 1: Go gRPC Service**
- New binary `sysml-grpc` alongside `sysml` and `sysml-lsp`
- Exposes OpenSysML internals via gRPC
- Stateless: each request self-contained
- LRU cache for parsed models keyed by content hash

**Tier 2: Python Client Library**
- Package `opensysml` wrapping gRPC client
- Manages service lifecycle transparently
- Provides Pythonic API (not raw protobuf)
- Handles conversions, error mapping, display integration

### Stateless with Caching

**Design choice:** Start with stateless request/response rather than sessions.

**Rationale:**
- Simpler implementation (no session lifecycle management)
- Service can restart without losing Python-side state
- Easier to debug (each call independent)
- Scales naturally (load balance across services later)

**Performance:** LRU cache on service side keyed by `sha256(content)` eliminates repeated parsing overhead.

### Communication Flow

```
1. Python: model = opensysml.load("A1.sysml")
   → Python library reads file, computes sha256
   
2. gRPC: ParseFileRequest(content, hash)
   → Service checks cache[hash]
   → Cache miss: parse with internal/core/parser
   → Cache hit: return cached result
   
3. gRPC: ParseFileResponse(model_hash, root_symbol, diagnostics)
   → Python library creates Model proxy
   
4. Python: part = model.find("SPACECRAFT_WET")
   → gRPC: GetSymbolRequest(model_hash, "SPACECRAFT_WET")
   → Service looks up in cached symbol table
   → gRPC: SymbolResponse(symbol_info, children_ids)
   → Python library creates Symbol proxy with lazy children
   
5. Python: children = part.children()
   → Lazy fetch: gRPC requests for each child_id
   → Returns list of Symbol proxies
```

---

## 4. Go gRPC Service Design

### Binary Name and Location

**Binary:** `cmd/sysml-grpc/main.go`

Follows existing pattern:
- `cmd/sysml/` - REPL
- `cmd/sysml-lsp/` - Language server
- `cmd/sysml-grpc/` - gRPC service (new)

### Service Interface

**Proto definition:** `api/proto/sysml.proto`

```protobuf
service SysMLService {
  // Parse and query
  rpc ParseFile(ParseFileRequest) returns (ParseFileResponse);
  rpc GetSymbol(GetSymbolRequest) returns (SymbolResponse);
  rpc GetDiagnostics(DiagnosticsRequest) returns (DiagnosticsResponse);
  
  // Runtime execution
  rpc EvaluateExpression(EvalRequest) returns (EvalResponse);
  rpc Instantiate(InstantiateRequest) returns (InstantiateResponse);
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteResponse);
  rpc RunStateMachine(StateMachineRequest) returns (StateMachineResponse);
}
```

### Request/Response Patterns

**ParseFile:**
- Request: file path OR inline content + sha256 hash
- Response: model_hash (cache key) + root symbol + diagnostics
- Cache lookup by hash before parsing

**GetSymbol:**
- Request: model_hash + symbol_id (fully qualified name)
- Response: symbol info (name, kind, metadata) + child_ids (lazy)
- Error if model_hash not in cache

**Runtime RPCs:**
- All include model_hash + target symbol/expression
- Responses include result + diagnostics + error field
- Diagnostics collected even on success (warnings)

### Caching Strategy

**Cache key:** `sha256(file_content)` - 32-byte hash

**Cached data:**
- Parsed AST (`*ast.RootNamespace`)
- Symbol table (`*symbols.SymbolTable`)
- Resolved types/semantics (side tables)

**Eviction:** LRU with configurable max size (default 100 models)

**No invalidation:** Stateless model means content changes = new hash = new cache entry

**Memory management:** Models with no active queries eligible for eviction

### Package Structure

```
cmd/sysml-grpc/
  main.go              # Server startup, flags, logging

internal/grpc/
  service.go           # Implements SysMLService interface
  cache.go             # LRU cache with sha256 keys
  convert.go           # internal types → protobuf messages
  errors.go            # Error mapping to gRPC status codes

api/proto/
  sysml.proto          # Service + message definitions
  generate.go          # //go:generate protoc invocation
```

### Integration with Existing Core

**Reuse everything:** Service is thin wrapper, no duplication.

**Parse flow:**
```go
// In service.ParseFile()
src := source.NewFile(req.FilePath, req.Content)
p := parser.New(src)
root := p.ParseFile()  // Existing parser

symtab := symbols.NewTable()
symbols.BuildFromAST(root, symtab)  // Existing symbol builder

// Run semantic passes
passes.RunAll(symtab)

// Cache and convert to protobuf
cache.Put(req.ContentHash, &Model{root, symtab, ...})
resp := convertToProto(root, symtab)
```

**No changes to `internal/core/*`** - service consumes existing APIs.

---

## 5. Protobuf Schema

### Core Message Types

**ParseFileRequest/Response:**
```protobuf
message ParseFileRequest {
  oneof source {
    string file_path = 1;
    string content = 2;
  }
  string content_hash = 3;  // sha256 for cache lookup
}

message ParseFileResponse {
  string model_hash = 1;     // Cache key for subsequent requests
  SymbolInfo root = 2;       // Root namespace
  repeated Diagnostic diagnostics = 3;
  string error = 4;          // Critical failure message
}
```

**Symbol representation:**
```protobuf
message SymbolInfo {
  string id = 1;             // Unique identifier (fully qualified name)
  string name = 2;
  string kind = 3;           // "PartDefinition", "AttributeUsage", etc
  map<string, string> metadata = 4;  // multiplicity, type, etc
  repeated string child_ids = 5;     // References to children (lazy)
  repeated AttributeInfo attributes = 6;
}

message AttributeInfo {
  string name = 1;
  string type = 2;
  Value value = 3;
  string unit = 4;
}
```

**Value representation:**
```protobuf
message Value {
  oneof value {
    double number = 1;
    string text = 2;
    bool boolean = 3;
    int64 integer = 4;
  }
}
```

**Diagnostics:**
```protobuf
message Diagnostic {
  string severity = 1;  // "error", "warning", "info"
  string message = 2;
  Span span = 3;
}

message Span {
  string file = 1;
  int32 start_line = 2;
  int32 start_col = 3;
  int32 end_line = 4;
  int32 end_col = 5;
}
```

### Runtime Message Types

**Instantiation:**
```protobuf
message InstantiateRequest {
  string model_hash = 1;
  string symbol_id = 2;  // Qualified name of part to instantiate
}

message InstantiateResponse {
  Instance instance = 1;
  repeated Diagnostic diagnostics = 2;
  string error = 3;
}

message Instance {
  string symbol_id = 1;
  map<string, Value> slots = 2;      // Attribute values
  repeated Instance parts = 3;        // Child instances
}
```

**Expression evaluation:**
```protobuf
message EvalRequest {
  string model_hash = 1;
  string expression = 2;
  map<string, Value> context = 3;    // Variable bindings
}

message EvalResponse {
  Value result = 1;
  repeated Diagnostic diagnostics = 2;
  string error = 3;
}
```

**Action execution:**
```protobuf
message ExecuteActionRequest {
  string model_hash = 1;
  string action_id = 2;
  map<string, Value> inputs = 3;
}

message ExecuteResponse {
  map<string, Value> outputs = 1;
  repeated string trace = 2;         // Execution trace lines
  repeated Diagnostic diagnostics = 3;
  string error = 4;
}
```

**State machine simulation:**
```protobuf
message StateMachineRequest {
  string model_hash = 1;
  string state_machine_id = 2;
  repeated string events = 3;        // Event sequence to process
}

message StateMachineResponse {
  repeated string state_trace = 1;   // State transitions
  repeated string current_states = 2; // Final active states
  repeated Diagnostic diagnostics = 3;
  string error = 4;
}
```

### Design Rationale

**Lazy loading via IDs:**
- `child_ids` field contains references, not full child data
- Python can fetch children on-demand with `GetSymbol` requests
- Reduces message size for large models

**Metadata map for extensibility:**
- `map<string, string> metadata` allows adding fields without schema changes
- Store multiplicity, visibility, specializations, etc.

**Separate error and diagnostics:**
- `diagnostics` = parse/semantic issues (model may still be usable)
- `error` = critical RPC failure (nil model, cache miss, internal error)

**Content hash strategy:**
- Client computes hash, includes in request
- Service uses as cache key
- Deterministic: same content = same hash = cache hit

---

## 6. Python Client Library Design

### Package Structure

```
opensysml/
  __init__.py         # Public API exports: load, connect, instantiate
  connection.py       # Connection class, lifecycle management
  client.py           # gRPC client wrapper
  model.py            # Model class
  symbol.py           # Symbol, Attribute classes
  instance.py         # Instance class
  diagnostics.py      # Diagnostic class
  display.py          # IPython rich display hooks
  binary.py           # Binary download/management
  dataframe.py        # DataFrame conversion utilities
  proto/              # Generated gRPC stubs (from protoc)
    sysml_pb2.py
    sysml_pb2_grpc.py
```

### Core API Classes

**Connection - Service management:**
```python
class Connection:
    def __init__(self, port=50051, auto_start=True):
        """Connect to service, auto-start if not running."""
        
    def load(self, path: str) -> Model:
        """Load SysML file from path."""
        
    def parse(self, content: str) -> Model:
        """Parse inline SysML content."""
        
    def instantiate(self, symbol_id: str) -> Instance:
        """Instantiate a part by qualified name."""
        
    def eval(self, expr: str, context: dict = None) -> Any:
        """Evaluate expression."""
        
    def close(self):
        """Explicit cleanup."""
```

**Model - Parsed file:**
```python
class Model:
    @property
    def diagnostics(self) -> List[Diagnostic]:
        """Parse/semantic diagnostics."""
        
    @property
    def root(self) -> Symbol:
        """Root namespace."""
        
    def find(self, name: str) -> Optional[Symbol]:
        """Lookup symbol by name."""
        
    def _repr_html_(self) -> str:
        """IPython display: tree + diagnostic summary."""
```

**Symbol - Any SysML element:**
```python
class Symbol:
    @property
    def name(self) -> str:
        
    @property
    def kind(self) -> str:
        """PartDefinition, AttributeUsage, etc."""
        
    def children(self) -> List[Symbol]:
        """All children (lazy loaded)."""
        
    def attributes(self) -> List[Symbol]:
        """Children filtered to attributes."""
        
    def get_attr(self, name: str) -> Optional[Symbol]:
        """Lookup attribute by name."""
        
    def to_dataframe(self) -> pd.DataFrame:
        """Convert children to DataFrame."""
        
    def _repr_html_(self) -> str:
        """IPython display: formatted definition."""
```

**Attribute - Specialized symbol:**
```python
class Attribute(Symbol):
    @property
    def value(self) -> Any:
        """Evaluated value (float, int, str, bool)."""
        
    @property
    def type(self) -> Optional[Symbol]:
        """Type symbol."""
        
    @property
    def unit(self) -> Optional[str]:
        """Unit if present."""
```

**Instance - Runtime instantiation:**
```python
class Instance:
    @property
    def symbol_id(self) -> str:
        """Fully qualified name of instantiated part."""
        
    @property
    def slots(self) -> Dict[str, Any]:
        """Attribute values."""
        
    @property
    def parts(self) -> List[Instance]:
        """Child instances."""
        
    def _repr_html_(self) -> str:
        """IPython display: slots table."""
```

### Error Handling Strategy

**Hybrid approach (requirement from clarifying questions):**

**Parse/semantic errors → Diagnostics collection:**
```python
model = conn.load("bad.sysml")
# Model still returned, even with syntax errors
if model.diagnostics:
    for d in model.diagnostics:
        print(f"{d.severity}: {d.message} at {d.span}")
# Can still explore parsed portions
```

**Runtime errors → Exceptions:**
```python
try:
    instance = conn.instantiate("MissingPart")
except opensysml.RuntimeError as e:
    print(f"Runtime error: {e.message}")
    print(f"Location: {e.span}")
```

**Exception hierarchy:**
```python
class OpenSysMLError(Exception):
    """Base exception."""

class ConnectionError(OpenSysMLError):
    """Service connection failed."""

class RuntimeError(OpenSysMLError):
    """Execution error (instantiate, eval, execute)."""
    def __init__(self, message, span=None):
        self.message = message
        self.span = span
```

### DataFrame Conversions

**Requirement:** Convert collections to Pandas DataFrames for analysis.

**Implementation:**
```python
# In Symbol class
def to_dataframe(self) -> pd.DataFrame:
    """Convert children to DataFrame."""
    children = self.children()
    return pd.DataFrame([
        {
            'name': c.name,
            'kind': c.kind,
            'multiplicity': c.metadata.get('multiplicity'),
            # ... other columns
        }
        for c in children
    ])

# Usage
part = model.find("SPACECRAFT_WET")
children_df = part.to_dataframe()
print(children_df[['name', 'kind', 'multiplicity']])
```

**Specialized conversions:**
```python
# Attributes with values
attrs_df = part.attributes().to_dataframe()
# Columns: name, type, value, unit

# Can be further analyzed
mass_attrs = attrs_df[attrs_df['type'] == 'MassValue']
total = mass_attrs['value'].sum()
```

### IPython Display Integration

**Automatic rich display in Jupyter:**

**Model display:**
```python
>>> model
📄 spacecraft.sysml (1,245 lines)
├─ 3 packages
├─ 47 parts
├─ 12 attributes
└─ ⚠️  2 warnings
```

**Symbol display:**
```python
>>> part
part def SPACECRAFT_WET {
  attribute unitMass: MassValue = 915.37 [kg];
  part pressurant: PRESSURANT;
  // ... 8 more children
}
```

**Instance display:**
```html
<table>
  <tr><th>Slot</th><th>Value</th></tr>
  <tr><td>unitMass</td><td>915.37 kg</td></tr>
  <tr><td>...</td><td>...</td></tr>
</table>
```

**Implementation:**
```python
# In display.py
def _model_repr_html(model):
    # Generate tree view HTML
    pass

# Register with IPython
from IPython.display import display
Model._repr_html_ = _model_repr_html
```

---

## 7. Service Lifecycle Management

### Binary Distribution

**Download strategy:**
- Python package does NOT bundle Go binary (keeps wheel small)
- Binary downloaded on first use from GitHub releases
- Platform detection: `platform.system()` + `platform.machine()`
- Supported platforms: linux-amd64, darwin-amd64, darwin-arm64, windows-amd64

**Storage location:**
- `~/.opensysml/bin/sysml-grpc-{version}-{platform}`
- Version from `opensysml.__version__`

**Download implementation:**
```python
# In binary.py
def ensure_binary():
    binary_path = get_binary_path()
    if os.path.exists(binary_path):
        return binary_path
        
    # Download from GitHub releases
    url = f"https://github.com/Open-MBEE/OpenSysML/releases/download/v{VERSION}/sysml-grpc-{PLATFORM}"
    download(url, binary_path)
    
    # Verify checksum
    verify_checksum(binary_path, CHECKSUMS[PLATFORM])
    
    # Set executable
    os.chmod(binary_path, 0o755)
    
    return binary_path
```

**Checksum verification:**
- Python package includes manifest with sha256 checksums
- Downloaded binary verified before first execution
- Prevents corrupted/tampered downloads

**Offline fallback:**
- User can manually place binary at `~/.opensysml/bin/sysml-grpc`
- Python skips download if binary exists

### Auto-Start and Health Checking

**Service startup on first connect:**
```python
# In connection.py
def _ensure_service():
    # 1. Check if already running
    if _probe_service(port):
        return  # Already running
        
    # 2. Ensure binary exists
    binary = ensure_binary()
    
    # 3. Start subprocess
    proc = subprocess.Popen([binary, "--port", str(port)])
    
    # 4. Wait for health check
    for _ in range(30):  # 30 second timeout
        if _probe_service(port):
            return  # Service ready
        time.sleep(1)
        
    raise ConnectionError("Service failed to start")

def _probe_service(port):
    """Check if service responding."""
    try:
        response = requests.get(f"http://localhost:{port+1}/health")
        return response.status_code == 200
    except:
        return False
```

**Health check endpoint in sysml-grpc:**
```go
// cmd/sysml-grpc/main.go
go func() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    http.ListenAndServe(fmt.Sprintf(":%d", *port+1), nil)
}()
```

### Multi-Process Coordination

**Problem:** Multiple Python processes/notebooks may import opensysml simultaneously.

**Solution:** Lockfile with PID tracking.

**Implementation:**
```python
# In connection.py
LOCK_FILE = os.path.expanduser("~/.opensysml/service.lock")

def _acquire_lock():
    """Acquire lock or detect existing service."""
    if os.path.exists(LOCK_FILE):
        with open(LOCK_FILE) as f:
            pid = int(f.read())
        # Check if process still alive
        if _process_alive(pid):
            return False  # Another process managing service
    
    # Write our PID
    with open(LOCK_FILE, 'w') as f:
        f.write(str(os.getpid()))
    return True

def connect(port=50051, auto_start=True):
    if auto_start:
        is_manager = _acquire_lock()
        _ensure_service()
        if is_manager:
            atexit.register(_cleanup_service)
    
    return Connection(port)

def _cleanup_service():
    """Called on exit of managing process."""
    # Send shutdown signal
    # Remove lockfile
    pass
```

**Behavior:**
- First process to import: starts service, becomes "manager"
- Subsequent processes: detect running service, connect to existing
- Manager process exit: shuts down service
- Non-manager exit: leaves service running

### Graceful Shutdown

**On manager process exit:**
```python
def _cleanup_service():
    """Registered with atexit."""
    try:
        # Send graceful shutdown via gRPC
        stub.Shutdown(ShutdownRequest())
        # Wait up to 5 seconds
        proc.wait(timeout=5)
    except TimeoutExpired:
        # Force kill
        proc.kill()
    finally:
        # Remove lockfile
        os.remove(LOCK_FILE)
```

**Service-side graceful shutdown:**
```go
// In cmd/sysml-grpc/main.go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    grpcServer.GracefulStop()
}()
```

### Manual Mode for Debugging

**User starts service manually:**
```bash
$ sysml-grpc --port 50051 --log-level debug
```

**Python connects without auto-start:**
```python
conn = opensysml.connect(auto_start=False, port=50051)
# Python doesn't manage lifecycle
# Service keeps running after Python exits
```

**Use case:** Debugging service issues, inspecting logs, development.

### Service Flags

**Command-line interface for sysml-grpc:**
```
sysml-grpc [options]

Options:
  --port INT              gRPC listen port (default: 50051)
  --health-port INT       HTTP health check port (default: port+1)
  --cache-size INT        Max cached models (default: 100)
  --log-level LEVEL       Logging: debug|info|warn|error (default: info)
  --log-file PATH         Log output file (default: stderr)
```

---

## 8. Testing Strategy

### Go Service Tests

**Unit tests - `internal/grpc/*_test.go`:**

```go
// cache_test.go
func TestCacheLRUEviction(t *testing.T)
func TestCacheHitRate(t *testing.T)
func TestCacheHashCollision(t *testing.T)

// convert_test.go
func TestSymbolToProto(t *testing.T)
func TestASTToProto(t *testing.T)
func TestDiagnosticToProto(t *testing.T)

// service_test.go (with mock clients)
func TestParseFileWithCache(t *testing.T)
func TestGetSymbolNotFound(t *testing.T)
func TestMalformedRequest(t *testing.T)
```

**Integration tests - `internal/grpc/integration_test.go`:**

```go
func TestEndToEndParse(t *testing.T) {
    // Start real gRPC server on test port
    server := startTestServer(t)
    defer server.Stop()
    
    // Connect with generated client
    conn := dialTestServer(t)
    client := pb.NewSysMLServiceClient(conn)
    
    // Load A1.sysml fixture
    resp, err := client.ParseFile(ctx, &pb.ParseFileRequest{
        FilePath: "testdata/A1.sysml",
    })
    
    // Verify response
    assert.NoError(t, err)
    assert.Empty(t, resp.Diagnostics)
    assert.NotEmpty(t, resp.ModelHash)
    
    // Query symbol
    symResp, err := client.GetSymbol(ctx, &pb.GetSymbolRequest{
        ModelHash: resp.ModelHash,
        SymbolId:  "SPACECRAFT_WET",
    })
    
    assert.NoError(t, err)
    assert.Equal(t, "PartDefinition", symResp.Symbol.Kind)
}

func TestInstantiateAndEval(t *testing.T)
func TestActionExecution(t *testing.T)
func TestStateMachineSimulation(t *testing.T)
```

**Conformance tests - `tests/grpc/conformance_test.go`:**

```go
func TestStdlibViaGRPC(t *testing.T) {
    // Load all stdlib files via gRPC
    for _, file := range stdlibFiles {
        resp := client.ParseFile(ctx, &pb.ParseFileRequest{
            FilePath: file,
        })
        
        // Must have zero errors (warnings OK)
        errors := filterErrors(resp.Diagnostics)
        assert.Empty(t, errors, "stdlib file %s has errors", file)
    }
}

func TestGRPCMatchesDirectParser(t *testing.T) {
    // Parse same file via gRPC and direct parser
    // Compare AST structure, symbol tables, diagnostics
    // Ensure service layer doesn't introduce differences
}
```

### Python Library Tests

**Unit tests - `tests/test_*.py`:**

```python
# test_model.py (with mocked gRPC)
def test_model_find_symbol():
def test_model_diagnostics():
def test_model_repr_html():

# test_symbol.py
def test_symbol_children_lazy_load():
def test_symbol_attributes_filter():
def test_symbol_to_dataframe():

# test_instance.py
def test_instance_slots():
def test_instance_parts():

# test_diagnostics.py
def test_diagnostic_parsing():
def test_diagnostic_severity():

# test_display.py
def test_ipython_repr_html():
def test_tree_rendering():
```

**Integration tests - `tests/integration/test_*.py`:**

```python
# test_roundtrip.py (requires running service or mock)
def test_load_and_query(conn):
    """Full round-trip: load, find, query attributes."""
    model = conn.load("testdata/A1.sysml")
    part = model.find("SPACECRAFT_WET")
    assert part.name == "SPACECRAFT_WET"
    assert part.kind == "PartDefinition"
    
    mass = part.get_attr("unitMass")
    assert mass.value == 915.37

def test_instantiate(conn):
    """Instantiate part, verify slots."""
    instance = conn.instantiate("SPACECRAFT_WET")
    assert "unitMass" in instance.slots
    assert instance.slots["unitMass"] == 915.37

def test_eval_expression(conn):
    result = conn.eval("2 + 2 * 3")
    assert result == 8

def test_error_handling(conn):
    """Verify exceptions raised for runtime errors."""
    with pytest.raises(opensysml.RuntimeError):
        conn.instantiate("NonExistentPart")
```

**Binary management tests - `tests/test_binary.py`:**

```python
def test_download_binary(mock_http):
    """Mock GitHub download, verify checksum."""
    binary_path = ensure_binary()
    assert os.path.exists(binary_path)
    assert os.access(binary_path, os.X_OK)

def test_checksum_verification():
    """Tampered binary fails checksum."""
    with pytest.raises(ChecksumError):
        verify_checksum(bad_binary_path, expected_hash)

def test_service_startup():
    """Start service, verify health check passes."""
    _ensure_service(port=TEST_PORT)
    assert _probe_service(TEST_PORT)

def test_multi_process_coordination():
    """Simulate multiple processes importing opensysml."""
    # Fork processes, verify only one starts service
    pass
```

**Notebook smoke test - `examples/opensysml_demo.ipynb`:**

```python
# Cell 1: Import and load
import opensysml
model = opensysml.load("../testdata/A1.sysml")
model  # Should display rich HTML

# Cell 2: Navigate
part = model.find("SPACECRAFT_WET")
part  # Display part definition

# Cell 3: DataFrame conversion
children_df = part.children().to_dataframe()
children_df.head()

# Cell 4: Instantiate
instance = opensysml.instantiate("SPACECRAFT_WET")
instance  # Display slots table

# Cell 5: Evaluate
result = opensysml.eval("915.37 * 2")
print(f"Result: {result}")
```

**Running notebook test:**
```bash
pytest --nbval examples/opensysml_demo.ipynb
# or
jupyter nbconvert --execute --to notebook examples/opensysml_demo.ipynb
```

### CI Pipeline Updates

**Go service CI:**
```yaml
# .circleci/config.yml or GitHub Actions
- name: Build sysml-grpc
  run: make build-grpc
  
- name: Test gRPC service
  run: go test ./internal/grpc/...
  
- name: Build multi-platform binaries
  run: |
    GOOS=linux GOARCH=amd64 go build -o bin/sysml-grpc-linux-amd64 ./cmd/sysml-grpc
    GOOS=darwin GOARCH=amd64 go build -o bin/sysml-grpc-darwin-amd64 ./cmd/sysml-grpc
    GOOS=darwin GOARCH=arm64 go build -o bin/sysml-grpc-darwin-arm64 ./cmd/sysml-grpc
    GOOS=windows GOARCH=amd64 go build -o bin/sysml-grpc-windows-amd64.exe ./cmd/sysml-grpc
    
- name: Upload release assets
  if: startsWith(github.ref, 'refs/tags/v')
  # Upload binaries to GitHub releases
```

**Python library CI:**
```yaml
- name: Setup Python
  uses: actions/setup-python@v4
  with:
    python-version: ['3.8', '3.9', '3.10', '3.11', '3.12']
    
- name: Install dependencies
  run: |
    pip install -e .
    pip install pytest pytest-mock pytest-nbval mypy black
    
- name: Lint
  run: |
    black --check opensysml/
    mypy opensysml/
    
- name: Test
  run: pytest tests/
  
- name: Test notebook
  run: pytest --nbval examples/opensysml_demo.ipynb
```

---

## 9. Implementation Phases

### Phase 1: Core gRPC Service (Go)

**Goal:** Working gRPC service with parse and query capabilities.

**Tasks:**
1. Define protobuf schema in `api/proto/sysml.proto`
   - Service interface with ParseFile, GetSymbol, GetDiagnostics
   - Core message types (SymbolInfo, Diagnostic, Value, Span)
   - Generate Go stubs with `protoc`

2. Implement `cmd/sysml-grpc/main.go`
   - Server startup with flags (port, cache-size, log-level)
   - gRPC server registration
   - HTTP health check endpoint
   - Graceful shutdown on signals

3. Implement `internal/grpc/service.go`
   - `ParseFile` RPC implementation calling `internal/core/parser`
   - `GetSymbol` RPC querying cached symbol tables
   - Error handling and gRPC status codes

4. Implement `internal/grpc/cache.go`
   - LRU cache with sha256 keys
   - Store parsed AST + symbol tables
   - Eviction policy

5. Implement `internal/grpc/convert.go`
   - Convert `*ast.Node` → `SymbolInfo` protobuf
   - Convert `source.Diagnostic` → `Diagnostic` protobuf
   - Convert Go values → `Value` protobuf

6. Add tests
   - Unit tests for cache, conversion functions
   - Integration test: start server, send ParseFile, verify response
   - Use A1.sysml as test fixture

**Definition of done:**
- `make build-grpc` produces `bin/sysml-grpc`
- `sysml-grpc --port 50051` starts and responds to health checks
- Integration test loads A1.sysml, queries SPACECRAFT_WET symbol
- All tests pass: `go test ./internal/grpc/...`

---

### Phase 2: Basic Python Client

**Goal:** Python library that connects to service and parses models.

**Tasks:**
1. Generate Python stubs from protobuf
   - `protoc --python_out=opensysml/proto sysml.proto`
   - `protoc --grpc_python_out=opensysml/proto sysml.proto`

2. Implement `opensysml/client.py`
   - Wrap gRPC channel and stub
   - `parse_file(path)` → Model
   - `get_symbol(model_hash, symbol_id)` → SymbolInfo

3. Implement `opensysml/model.py`
   - Model class wrapping ParseFileResponse
   - Properties: `root`, `diagnostics`
   - `find(name)` method

4. Implement `opensysml/symbol.py`
   - Symbol proxy class
   - Properties: `name`, `kind`, `metadata`
   - `children()` lazy loading via `get_symbol` RPC
   - `attributes()` filtered children

5. Implement `opensysml/diagnostics.py`
   - Diagnostic class wrapping protobuf
   - Properties: `severity`, `message`, `span`

6. Implement `opensysml/connection.py`
   - Connection class (manual service management only)
   - `__init__(port)` connects to existing service
   - `load(path)` → Model

7. Add unit tests with mocked gRPC
   - Mock ParseFileResponse, test Model creation
   - Mock GetSymbolResponse, test Symbol navigation
   - Test diagnostic parsing

**Definition of done:**
- `pip install -e .` installs opensysml package
- Can connect to manually-started `sysml-grpc`
- `model = conn.load("A1.sysml")` works
- `part = model.find("SPACECRAFT_WET")` works
- Unit tests pass: `pytest tests/`

---

### Phase 3: Auto-Lifecycle Management

**Goal:** Seamless installation and auto-start.

**Tasks:**
1. Implement `opensysml/binary.py`
   - `ensure_binary()` downloads from GitHub releases
   - Platform detection (linux/darwin/windows, amd64/arm64)
   - Checksum verification
   - Storage in `~/.opensysml/bin/`

2. Update `opensysml/connection.py`
   - `_ensure_service()` auto-starts if not running
   - `_probe_service()` health check polling
   - `atexit` registration for cleanup
   - Lockfile-based multi-process coordination

3. Update `opensysml/__init__.py`
   - Public API: `load(path)`, `connect()`, `instantiate()`
   - Auto-connect on first use

4. Add binary management tests
   - Mock HTTP download
   - Test checksum verification
   - Test service startup
   - Test multi-process scenarios

**Definition of done:**
- `import opensysml` auto-downloads binary on first use
- `model = opensysml.load("A1.sysml")` works without manual service start
- Multiple Python processes can import concurrently
- Service shuts down when last process exits
- Tests pass: `pytest tests/test_binary.py`

---

### Phase 4: Runtime APIs

**Goal:** Full runtime capabilities (eval, instantiate, execute, simulate).

**Tasks:**
1. Extend protobuf schema
   - Add EvalRequest/Response
   - Add InstantiateRequest/Response, Instance message
   - Add ExecuteActionRequest/Response
   - Add StateMachineRequest/Response

2. Implement runtime RPCs in `internal/grpc/service.go`
   - `EvaluateExpression` calls `internal/core/runtime.Eval`
   - `Instantiate` calls runtime instantiation
   - `ExecuteAction` calls action executor
   - `RunStateMachine` calls state executor

3. Implement `opensysml/instance.py`
   - Instance class wrapping protobuf
   - Properties: `symbol_id`, `slots`, `parts`

4. Update `opensysml/connection.py`
   - `eval(expr, context)` → value
   - `instantiate(symbol_id)` → Instance
   - `execute_action(action_id, inputs)` → outputs
   - `run_state_machine(sm_id, events)` → trace

5. Add error handling
   - Raise `opensysml.RuntimeError` for execution failures
   - Collect diagnostics for warnings

6. Add integration tests
   - Test expression evaluation
   - Test instantiation with A1.sysml
   - Test action execution (if fixtures available)
   - Test state machine simulation

**Definition of done:**
- `instance = opensysml.instantiate("SPACECRAFT_WET")` returns Instance
- `result = opensysml.eval("2 + 2")` returns 4
- Runtime errors raise typed exceptions with location info
- Integration tests pass for all runtime features

---

### Phase 5: Rich Display & DataFrames

**Goal:** Excellent notebook UX.

**Tasks:**
1. Implement `opensysml/dataframe.py`
   - `to_dataframe()` for Symbol collections
   - Columns: name, kind, multiplicity, type, value, etc.
   - Handle nested structures

2. Implement `opensysml/display.py`
   - `_repr_html_()` for Model (tree + diagnostics summary)
   - `_repr_html_()` for Symbol (formatted definition)
   - `_repr_html_()` for Instance (feature values table)
   - Text fallback for non-notebook use

3. Update classes to use display hooks
   - Add `_repr_html_()` methods to Model, Symbol, Instance
   - Add `to_dataframe()` to Symbol

4. Create demo notebook
   - `examples/opensysml_demo.ipynb`
   - Load A1.sysml
   - Navigate, query, display
   - DataFrame conversions
   - Instantiation

5. Add display tests
   - Test HTML generation
   - Test DataFrame schema
   - Run notebook with `pytest --nbval`

**Definition of done:**
- `model` in Jupyter shows rich tree view
- `part` displays formatted definition
- `children_df = part.children().to_dataframe()` works
- Demo notebook runs end-to-end
- Display tests pass

---

### Phase 6: Distribution & CI

**Goal:** Public release on PyPI and GitHub.

**Tasks:**
1. Setup multi-platform binary builds
   - CI job building for linux-amd64, darwin-amd64, darwin-arm64, windows-amd64
   - Compute sha256 checksums
   - Upload to GitHub releases on version tags

2. Create Python package metadata
   - `setup.py` or `pyproject.toml`
   - Version manifest with binary checksums
   - Dependencies: grpcio, protobuf, pandas, requests
   - Optional: ipython for notebook support

3. Setup PyPI publishing
   - Build wheel with `python -m build`
   - Upload to PyPI (test.pypi.org first, then pypi.org)
   - Versioning strategy (match OpenSysML version?)

4. Update documentation
   - Add Python usage to main README
   - Create `docs/PYTHON_API.md` with examples
   - Add notebook examples to `examples/`

5. CI integration
   - Add Go gRPC tests to existing CI
   - Add Python tests (multiple Python versions)
   - Add notebook smoke test
   - Cross-platform validation

**Definition of done:**
- `pip install opensysml` works from PyPI
- Binaries auto-download from GitHub releases
- CI builds and tests both Go and Python
- Documentation complete
- Release tagged and published

---

## 10. Open Questions and Future Work

### Open Questions

**Q1: Binary versioning and compatibility**
- How to handle version mismatch between Python package and Go binary?
- Strategy: Python package pins to specific binary version, downloads exact match
- Future: Add version negotiation in gRPC handshake

**Q2: Performance benchmarks**
- What are acceptable latency targets?
- Need baseline measurements: parse time, instantiation time, query time
- Action: Add benchmarking suite in Phase 1

**Q3: Error message fidelity**
- How much diagnostic context to include in Python exceptions?
- Current design: Include message + span, full trace available in diagnostics list
- Validate with real usage

**Q4: DataFrame schema stability**
- What columns should `to_dataframe()` always include?
- Proposal: name, kind, multiplicity (core); other columns optional/metadata-driven
- Need user feedback on common analysis patterns

### Future Work (Post-Initial Release)

**Workspace Management:**
- Add stateful workspace API (Phase 2 design from architecture discussion)
- Support multi-file projects with imports
- Incremental updates when files change

**Analysis Toolkit:**
- Pre-built analysis functions (mass rollup, connectivity checks, etc.)
- Built on query API, demonstrated as examples initially
- Promote to library functions based on usage patterns

**Model Generation:**
- Write API: create parts, set attributes, save to .sysml
- Enables round-trip workflows (import from CAD → modify in Python → export)

**Visualization:**
- Matplotlib integration for hierarchy plots, state diagrams
- Graphviz for relationships
- 3D geometry visualization if CAD data available

**Performance Optimizations:**
- Batch symbol queries (fetch multiple symbols in one RPC)
- Streaming for large models
- Client-side caching (Python-side cache layer)

**Advanced Runtime Features:**
- Step-by-step action execution (debugger-like)
- State machine visualization during simulation
- Constraint checking and violation reporting

**Language Feature Parity:**
- Full KerML expression support
- Metadata annotation querying
- View/viewpoint rendering
- Requirement verification traces

---

## Summary

This design provides a complete Python interface to OpenSysML via gRPC, enabling programmatic model exploration, analysis, and runtime simulation. The stateless architecture with caching balances simplicity and performance. Auto-lifecycle management delivers seamless notebook UX. Six implementation phases allow incremental delivery of value.

**Next steps:**
1. User review and approval of this design
2. Create detailed implementation plan (writing-plans skill)
3. Begin Phase 1 implementation


# SysML v2 Execution Environment

A complete, production-grade SysML v2 implementation in Go—spanning the full lifecycle from authoring to execution, delivering the integrated tooling experience systems engineers expect from modern language ecosystems.

## Quick Start

**Get started in 5 minutes:** [Quick Start Guide](docs/QUICKSTART.md)

### Install

**Download pre-built binaries:**
```bash
# Linux x64
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/sysml-linux-amd64.tar.gz
tar xzf sysml-linux-amd64.tar.gz && sudo mv sysml-linux-amd64 /usr/local/bin/sysml

# macOS (see Quick Start for all platforms)
```

**Or build from source:**
```bash
go build -o sysml ./cmd/sysml
./sysml
```

### Try it

**Interactive modeling:**
```bash
$ sysml
sysml> part Wheel { attribute diameter = 16.0; }
✓ Wheel

sysml> %instantiate Wheel
Created instance: Wheel (ID: 1)

sysml> %slots Wheel
Instance: Wheel (ID: 1)
  diameter: 16.0
```

**Behavioral execution:**
```bash
sysml> calc add { in x; in y; return x + y; }
✓ add

sysml> %calc add 10 20
✓ add(10, 20)
  = 30

sysml> constraint ValidSpeed { assert 65 <= 120; }
✓ ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Action & state debugging:**
```bash
sysml> %action MyWorkflow
Action: MyWorkflow
Tokens: 1
State: Ready

sysml> %step
Tokens: 3  (fork created parallel paths)

sysml> %tokens
Token 1: processA { input: 100 }
Token 2: processB { input: 100 }
Token 3: processC { input: 100 }

sysml> %continue
✓ Completed
Result: 360

sysml> %state TrafficLight
State machine: TrafficLight
Current: red
Time: 0.0s

sysml> %advance
Current: green (Time: 30.0s)
```

**See [examples/repl-behavioral-demo.sysml](examples/repl-behavioral-demo.sysml) for comprehensive demos.**

---

## What is This?

**Think Python/Rust/Go tooling, but for SysML v2:**

- **Language Server** — First-class IDE support (VS Code, IntelliJ, Emacs, etc.) with live diagnostics, semantic hover, go-to-definition, intelligent completion, and workspace-wide symbol search.
- **Interactive REPL** — Exploratory modeling environment: define models incrementally, evaluate expressions on-the-fly, instantiate parts, run calculations, inspect runtime state—like IPython/Jupyter for systems engineering.
- **Execution Runtime** — Not just a validator: instantiate parts, evaluate constraints against concrete values, execute calc/analysis cases, simulate behavioral models (actions, state machines).
- **Modern Toolchain** — Dependency management (local + remote git), incremental compilation, bundled standard library, persistent semantic caches—`cargo`/`go mod` ergonomics for systems modeling.

## Goals

- **Performance:** Sub-millisecond parsing, single static binary, no JVM/Eclipse runtime
- **Completeness:** Full OMG SysML v2 spec compliance (textual notation + KerML)
- **Executable models:** Instantiate, evaluate, simulate—turn specifications into running systems
- **Real-world ergonomics:** Multi-file projects, incremental analysis, rich diagnostics

## Status

**Active development. Core infrastructure operational:**

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral grammar) | ✅ Complete |
| Symbol resolution & type system | ✅ Complete |
| Validation passes (syntax → constraints) | ✅ Complete |
| Expression evaluator & instance model (runtime Tiers 1-3) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (Phase C1-5: all behavioral bodies) | ✅ Complete |
| **Calc invocation, constraint & requirement evaluation** | ✅ **Complete** |
| **Action execution engine (Tier 5)** | ✅ **Complete** |
| **State machine runtime (Tier 5)** | ✅ **Complete** |
| **REPL debugging commands** | ✅ **Complete** |
| Standard library bundling & caching | 🚧 In progress |
| LSP server implementation | 🚧 In progress |

**Current commit:** All tests pass (`go test ./...`), builds clean (`go build ./...`).
**Test coverage:** 116 tests covering parsers, semantics, runtime (actions, states, instances).

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Frontends: LSP Server │ Interactive REPL               │
├─────────────────────────────────────────────────────────┤
│  Workspace: Multi-file projects, dependency management  │
├─────────────────────────────────────────────────────────┤
│  Semantic Engine: Types, resolution, validation         │
├─────────────────────────────────────────────────────────┤
│  Execution Runtime: Expressions, instances, behaviors   │
├─────────────────────────────────────────────────────────┤
│  Parser/Lexer: Hand-written recursive descent           │
├─────────────────────────────────────────────────────────┤
│  AST: Syntax-only, immutable (semantics in side tables) │
└─────────────────────────────────────────────────────────┘
```

**Key design principles:**
- **Incremental & lazy:** Parse immediately, resolve semantics on-demand (gopls/rust-analyzer precedent)
- **Immutable AST:** All semantic state lives in side tables keyed by node/symbol
- **Pluggable validation:** Tiered passes (syntax → names → types → constraints)
- **Separated concerns:** Static analysis pipeline feeds execution runtime

## Module Structure

```
github.com/Open-MBEE/Systemica
├── cmd/
│   ├── sysml-lsp/          # LSP server binary
│   └── sysml/              # Interactive REPL binary
├── internal/core/
│   ├── source/             # Source files, spans, line indexing
│   ├── lexer/              # Hand-written scanner
│   ├── parser/             # Recursive-descent parser
│   ├── ast/                # Syntax tree nodes
│   ├── symbols/            # Symbol tables, scope trees
│   ├── resolve/            # Name resolution (lazy, memoized)
│   ├── semantics/          # Type system, conformance, multiplicity
│   ├── passes/             # Validation passes (syntax → constraints)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   └── libs/               # Standard library bundling & caching
├── internal/lsp/           # LSP protocol implementation
├── internal/repl/          # REPL loop implementation
├── docs/                   # Design specs, architecture docs
└── testdata/               # Test fixtures (.sysml, .kerml)
```

## Technology

- **Language:** Go 1.25+ (goroutines for concurrency, single static binary, proven LSP track record)
- **Parser:** Hand-written recursive descent (zero overhead, full error recovery, sub-ms parses)
- **Grammar source:** OMG pilot Xtext grammars (`SysML.xtext` + `KerMLExpressions`)
- **Spec reference:** [OMG SysML v2.0 (September 2025)](https://www.omg.org/spec/SysML/2.0)
- **CI/CD:** CircleCI for automated builds, tests, and releases

## Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases page](https://github.com/Open-MBEE/Systemica/releases).

**Supported platforms:**
- Linux (x64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x64)

**Release process:**
- Every commit: Build + test
- Tagged releases (`v*`): Multi-platform binaries published to GitHub Releases

## Building

```bash
# Build all binaries
go build ./...

# Run tests
go test ./...

# Build LSP server
go build -o bin/sysml-lsp ./cmd/sysml-lsp

# Build REPL
go build -o bin/sysml ./cmd/sysml
```

## Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)** — Get up and running in 5 minutes
- **[Architecture](docs/ARCHITECTURE.md)** — Complete system architecture, core pipeline, runtime tiers
- **[Examples](examples/)** — Runtime demos and behavioral model examples

## License

Apache 2.0

## Contributing

Project currently in active development. Contribution guidelines forthcoming.

## Contact

[Contact information to be added]

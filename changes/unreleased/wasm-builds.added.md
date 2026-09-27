- **The commands build for both WebAssembly targets, and run there.** `make build-wasm` links
  `sysml`, `sysml-lsp` and `sysml-grpc` for `wasip1` (WASI preview 1) and `js`, with the version
  stamps a release build passes, and `make wasm-check` compiles and vets the whole tree for both
  and executes the modules under Node. Where a WebAssembly host cannot do what was asked, the
  command says so by name instead of failing on the platform's own words: starting a process
  answers `a WebAssembly build cannot start external processes` for the solver, external
  engines, PDF rendering and `-compile`, and `sysml-grpc -transport grpc` and `-transport connect`
  refuse with the reason and point at `-transport stdio`, which serves. The prompt reads lines
  through a plain line reader where readline needs a terminal, so history, completion, `Ctrl-C`
  and terminal width are the one thing a native build keeps. No release ships an artifact; the
  reference page (`docs/reference/wasm.md`) is what to build, how to run it and what differs.

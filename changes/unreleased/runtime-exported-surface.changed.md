- `internal/exec/runtime` no longer exports the identifiers only its own files use: 325
  functions, methods, types, constants and variables that nothing outside the package
  referenced are unexported, and nine methods nothing called at all are removed. The API
  the CLI, REPL, gRPC service, LSP, analysis, document and code-generation packages consume,
  the identifiers `docs/reference/api.md` documents, and those other packages' tests exercise
  the runtime through keep their names.

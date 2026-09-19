- **The referees live in the tools module.** The fUML and PSSM referees (`internal/fuml`,
  `internal/pssm`) are `tools/referee/fuml` and `tools/referee/pssm`; the pilot differential,
  rejection, Xpect and execution oracles (`cmd/pilot-diff`, `cmd/pilot-reject`, `cmd/pilot-xpect`,
  `cmd/pilot-exec-diff`) are `tools/referee/{diff,reject,xpect,exec}` with one thin `main` each under
  `tools/cmd/`, so every referee is run with `go run -C tools ./cmd/<name>`. What they share moved
  beside them: the XMI reader is `internal/core/xmi`, the corpus errata registry is
  `tools/oracle/errata`, the repository-root and develop-commit lookups every tool carried a copy of
  are `tools/oracle/repo`, and the report files and verdict buckets are `tools/oracle/report`. The
  committed baselines record the new corpus paths; no figure moved.

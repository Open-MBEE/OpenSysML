- **The architecture self-model now describes the analysis framework.** `examples/self-model`
  gains `AnalysisFramework` — the seven question kinds and three freedoms, the five-step
  evidence scale and ten claims, the per-owner engine registry, the dispatcher with its `auto`,
  `all` and named selections, the seven-field plan budget with `OPENSYSML_JOBS`, the worker
  fleet, and the four engines this build registers with what each answers, what bounds it and
  the strongest evidence it can produce — wired into `AnalysisPipeline` and asked from the REPL,
  the service and the command line through the `%engine`/`%jobs`/`%engines` commands, the
  `engine` field, `ListEngines` and the `engines` capability, and the `-engine`/`-jobs`/`-engines`
  flags. The runtime's split of model-derived from run-derived state is modelled with the snapshot
  store and the exploration queue it makes possible. Five behaviors join the model — one question
  answered, a behavior's outcomes explored over a fleet of workers, the evidence ladder, a worker's
  life in a plan and a snapshot as a mark between steps — with four invariants over them
  (`questionsHaveOneContract`, `evidenceIsHonest`, `runsAreIsolated`, `snapshotsAreRunState`), the
  gates that verify them, eight views and an architecture-document section with an engine table
  generated from the model. `go test ./examples/` holds every new fact to `internal/core/analysis`
  and the runtime — engine names and descriptions, kinds, strengths, claims, budget fields, the jobs
  variable and its parsing, the selections, the exploration defaults, the snapshot refusals and the
  surfaces' commands, flags, field, RPC and capability — and exercises worker isolation: two jobs of
  one plan get runtime models of their own, two runs on one worker get contexts of their own.

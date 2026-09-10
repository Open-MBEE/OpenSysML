- **Each analysis plan runs on a resolver and semantic model of its own.** The analysis
  framework builds one `analysis.Worker` per plan over the shared frozen index and every
  context a run owns on it, so two plans on one model in one process share nothing that
  memoizes; the result carries how many workers a plan built and how long that took, printed
  nowhere yet. A run-owned context takes the budget's `Steps` as `OPENSYSML_MAX_STEPS` and
  `Memory` as `OPENSYSML_MAX_ELEMENTS` at construction and leaves every other bound at the
  context's own; a zero field is the context's own too. The gRPC service builds a worker per
  request instead of serializing every runtime request on a model behind one shared pair, and
  the REPL session answers completion and its getters while an exploration runs on contexts of
  its own — a second command still waits. No flag, command, RPC, field or line of output
  changed.

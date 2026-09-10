- **Every analysis question now goes through one framework.** `internal/core/analysis` holds
  the engine contract the analysis framework design note fixes — `Question`, `Engine`, `Result`,
  `Claim`, `Strength`, `Bounds`, `Budget` — a registry whose duplicate registrations and missing
  engines are typed errors, and `auto` dispatch that consults the engines covering a question
  strongest first, records each refusal and each run-time *not covered* answer in its plan, and
  stops on a run's error. The interpreter (`run`), `explore`, the parameter sweep (`sweep`) and
  the SMT solver (`solve`) register as engines over the code that already existed, and the REPL
  session and the gRPC service put every constraint, requirement and satisfaction check, calc,
  analysis and verification run, action and state execution, exploration, sweep and `%check`,
  `%explain`, `%solve`, `%configure` and `%optimize` query to their registry. No flag, command,
  RPC, field or line of output changed; the framework's own surface — engine listing and
  selection, the standing line on verdicts, parallel runs — is still to come.

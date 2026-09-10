- **Every verdict now states its standing, and the analysis engines can be listed and chosen.**
  Each verdict the CLI, the REPL and `-json` report is followed by a `standing:` line — the
  claim, the strength of the evidence behind it (*not covered*, *observed*, *witnessed*,
  *bounded*, *proved*) and what earned it, such as `holds (observed: 1 run under reverse)` or
  `outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))`; a budget the
  engine reached is named and lowers the strength, never a proof. `sysml -engines` and `%engines`
  table the engines of the build with the authority each carries, the questions it answers and
  whether its process was found (`solve` reports the solver it discovered); `-engine
  <name>|auto|all` and `%engine` select the engine every check is put to: `auto` (the default)
  is the dispatch every check had, a name puts the question to that engine alone with its refusal
  as the verdict, and `all` puts it to every covering engine in name order and composes their
  answers — a witnessed violation stands over any universal claim, a universal claim an execution
  refutes is a disagreement resolved in the interpreter's favor with the refuted result demoted to
  *not covered*, and an engine cancelled by the plan's deadline is kept in the plan with the bound
  it reached. `-engine explore` is `-schedule explore`. `-json` checks gain `plan` (the selection,
  the composed standing, each engine consulted and its status, the disagreements) and
  `results[]` (one entry per engine that answered: `engine`, `claim`, `strength`, `bounds`,
  `witness`, `standing`) beside the keys they always carried. The service adds `ListEngines`, an
  `engine` field on the verification, calculation, analysis and sweep requests (unset meaning
  `auto`) and `engine`, `strength` and `bounds` on their responses and on every `Verdict`,
  advertised as the `engines` capability; the Python client takes `engine=` on its verification
  and analysis calls, reads `Verdict.engine`, `.strength` and `.bounds`, and lists engines with
  `Connection.list_engines()`; the Go client takes `WithEngine`/`Engine`, gains `Calculate` — `EvaluateCalc` with
  options — taking `CalcArguments` and `CalcEngine`, reads the `Standing` of
  every verdict, calculation and analysis, and lists engines with `Client.ListEngines`. No
  existing flag, command, RPC, field or key changed its meaning.

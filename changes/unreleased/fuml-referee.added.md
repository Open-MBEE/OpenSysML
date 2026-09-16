- **The fUML test activities referee the action executor.** `cmd/fuml-referee` translates
  every expressible activity of the pinned fUML reference implementation's test model into a
  `fuml::<Activity>` action definition by rule — parameters and their multiplicities, control
  and object flows with an enabling succession beside each flow whose target has no control
  predecessor, forks, joins, merges and guarded decisions, value specifications, nested
  behavior calls with same-named parameters spelled apart, the primitive and list library
  functions KerML has counterparts for — runs it under every schedule the runtime's explorer
  reaches, and requires the values left in its output parameters to be the ones the reference
  implementation recorded, as a multiset where the fUML parameter is unordered; the reference's
  firing sequence being among the reachable ones is reported and never a verdict. Each activity
  is filed as `pass`, `fail`, `not-expressible` or `differs-by-design` (an action the reference
  fires once per object token), the counts are pinned in `docs/project/fuml-referee-baseline.json`
  and checked in CI over the downloaded suite by `go run ./cmd/fuml-referee -check`, and the
  9 `fail` rows are activities the emitter does not yet translate (object creation,
  structural-feature actions, accept-event actions, active classes), attributed as such. The
  `-json`, `-filter`, `-keep` and `-jobs` flags report, narrow, retain the emitted models and
  parallelize the run; a filtered run never updates the baseline. `docs/project/fuml-referee.md`
  documents the translation rules and the adjudication of every row, the spec-compliance action
  section gains its row, and the precise-semantics alignment note gains row A15 for per-token
  re-firing.

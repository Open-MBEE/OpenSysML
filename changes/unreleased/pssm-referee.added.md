- **The OMG PSSM state-machine test suite runs as an advisory referee.**
  `cmd/pssm-referee` downloads nothing itself: `./scripts/download-pssm-suite.sh` fetches the
  pinned `PSSM_TestSuite.xmi` (OMG `ptc/18-11-06`, 103 tests) over HTTPS, checks its sha256 and
  places it under the ignored `build/pssm/`; nothing from the suite is committed. The referee
  reads the suite's XMI, classifies every test — standard notation, this project's `fork`/`join`/
  history/`defer` extensions, the terminate gap, or no SysML v2 spelling — translates each
  expressible test into textual notation in memory by the construct table of the precise-semantics
  alignment note (every `trace("…")` appends to a `String` attribute `log`), runs it under the
  runtime's own state-machine driver with the `explore` schedule, and compares the set of `log`
  values reachable against the suite's expected traces in both directions. Each test is filed as
  `pass`, `fail`, `not-expressible`, `terminate-gap` or `differs-by-design` (the last only through
  a committed table mapping the test to a note row that differs by design, never inferred from a
  failure); a typed runtime error or an exhausted budget is a `fail` that names it. `-jobs`
  explores in parallel with byte-identical output, `-filter` selects tests, `-keep` writes the
  translated models for debugging, `-json` prints the full report, `-update` records the baseline
  and `-check` fails when the bucket counts move. `docs/project/pssm-referee.md` records the pin,
  the licence reading, the construct mapping as implemented, the counts with the date and
  `develop` commit they were measured on, and every test's bucket and reason; the pull-request
  workflow provisions the suite and gates on the counts. A pass checks that the runtime reproduces
  UML behavior where the model has a defensible SysML v2 mapping and is never evidence of SysML v2
  conformance. The state-machine driver the conformance tests used (`PerformState`, `Explore`,
  `QueuedEvent`) is now exported by `internal/core/runtime` so both harnesses share it; no
  runtime behavior changed.

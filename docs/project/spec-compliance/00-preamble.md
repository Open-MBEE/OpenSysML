# SysML v2 Specification Compliance

**Purpose:** Document implementation coverage of SysML v2 / KerML behavioral semantics. UML 2.5.1 is cited only as reference semantics for an OpenSysML extension the SysML v2 notation has no production for and the bundled KerML semantic library (`internal/workspace/libs/stdlib/`) no performance for.

**Related:** [`TESTING.md`](../internals/testing.md) (test contracts), [`ARCHITECTURE.md`](../internals/architecture.md) (runtime architecture), [grammar-coverage.md](grammar-coverage.md) (which OMG grammar productions our test inputs exercise — input-presence evidence, which implies nothing about compliance either way)

**Constraint names:** a backticked `validate…` name is a constraint of the validation census, spelled as [`constraint-manifest.json`](constraint-manifest.json) names it (its raw pilot identifier or the formal document's name, where the manifest records one, resolve to the same row), and a rule row citing one bare claims the census row's status as its own. Where a row describes a different rule, or one sub-case, that happens to share the name, the citation is followed by `(census faithful)` stating the census row's status, and the row's own status stands apart. A constraint the specification names and the pilot does not is written `spec:validate…` and must be stated by the bundled `SysMLValidation` library. `go run -C tools ./cmd/validation-census -check` (under `make docs-counts`) refuses a cited name that resolves to none of these and a bare citation whose row status differs from the census.

**Clause numbers:** a `§` reference reads the specification editions the pilot pin ships (`scripts/pilot-pin.sh`, release 2026-09): SysML v2.1 Beta 2 and KerML 1.1 Beta 2 (the `build/spec-docs/` texts `scripts/download-spec-examples.sh` renders). A row that deliberately reads an older edition names it (`KerML 1.0, formal/2026-03-01`). Re-point the numbers when the pin moves to a release whose documents changed.

---

# Internals

Implementation documentation for contributors who are changing the code.
User-facing behavior is documented in [the guide](../guide/) and
[the reference](../reference/).

- **[Architecture](architecture.md)** — the pipeline, the tiers, the test contracts
- **[Testing](testing.md)** — the test contracts each kind of change must satisfy
- **[Performance](performance.md)** — profiling, and what a large model costs
- **[An execution-owned IR](execution-ir-design.md)** — a proposal: lowered graphs that own
  their identity and carry no syntax-tree pointers, with the inventory, the migration and the
  open decisions
- **[Design notes](design/README.md)** — how each subsystem was built, and which
  normative reference it follows
- **[Plans](notes/README.md)** — the staged work behind each subsystem, kept for the reasoning
  it records

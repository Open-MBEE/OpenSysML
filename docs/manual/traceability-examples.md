# Requirements Traceability Examples

Four traceability reports, each a complete model with its rendered output,
graded from a flat requirement list to a multi-team program. Each tier adds
one kind of structure to the model and the queries that read it, so a report
of your own can start from the tier that matches its shape. All four are
committed under `examples/` and kept in lockstep with the binary by a test
that re-renders every source and compares it against the committed output.

| Tier | Model | Adds | Source | Rendered |
|---|---|---|---|---|
| 1 | Three rover requirements, three parts | `satisfy`, a matrix column with `aggregate = "any"`, the requirements nothing covers | [`trace-1-basic.sysml`](examples/trace-1-basic.sysml) | [`trace-1-basic.md`](examples/trace-1-basic.md) |
| 2 | A lander specification nested two levels deep | The requirement tree, nested parts satisfying leaves, verification cases with pass and fail verdicts, `Union` and `Except` over coverage sets | [`trace-2-hierarchy.sysml`](examples/trace-2-hierarchy.sysml) | [`trace-2-hierarchy.md`](examples/trace-2-hierarchy.md) |
| 3 | Mission, system and subsystem requirements | Derivation chains walked one hop and to their ends in both directions, refinement, lineage columns, the uncovered chain ends | [`trace-3-derivation.sysml`](examples/trace-3-derivation.sysml) | [`trace-3-derivation.md`](examples/trace-3-derivation.md) |
| 4 | An orbiter program across six packages | Requirements owned by four teams, one kept inside a design package, cross-package derivation and refinement, a ten-column matrix grouped by team, list/count/any columns side by side, the critical gaps | [`trace-4-program.sysml`](examples/trace-4-program.sysml) | [`trace-4-program.md`](examples/trace-4-program.md) |

Render any of them from the repository root, to Markdown, HTML or PDF:

```console
$ sysml docs/manual/examples/trace-4-program.sysml \
    -render-document ProgramReport::ProgramTraceability -o program.md
$ sysml docs/manual/examples/trace-4-program.sysml \
    -render-document ProgramReport::ProgramTraceability \
    -doc-form html -doc-toc -o program.html
$ sysml docs/manual/examples/trace-4-program.sysml \
    -render-document ProgramReport::ProgramTraceability \
    -doc-form pdf -doc-toc -doc-number-sections -o program.pdf
```

The documents to render are `RoverBasic::BasicReport`,
`LanderHierarchy::HierarchyReport`, `RangeDerivation::DerivationReport` and
`ProgramReport::ProgramTraceability`. Every query in them also runs on its
own with `-run-query`, which is the quickest way to see what one step of a
report returns:

```console
$ sysml docs/manual/examples/trace-3-derivation.sysml \
    -run-query 'RangeDerivation::AllDerived req=RangeDerivation::specification::mission::range'
$ sysml docs/manual/examples/trace-4-program.sysml \
    -run-query ProgramReport::CriticalUncovered
```

## Tier 1: which part satisfies each requirement

`RoverBasic` holds three flat requirements and a rover of three parts, two
`satisfy` statements, and a report of one table and one list. The table is a
`Project` over the requirements with two `RelatedColumn`s reading the
satisfaction relationship inward: `satisfiedBy` lists the satisfying parts,
`satisfied` reduces the same traversal to a Boolean with `aggregate = "any"`.
The list is `WhereRelated(..., exists = false)` — the requirements the
traversal reaches nothing from. The root the queries walk is a `part` holding
the requirements, bound as `in root = specification`; a package cannot be
bound, so a specification that is only a package needs a part to hold it.

## Tier 2: a requirement tree with verdicts

`LanderHierarchy` nests requirements two levels deep (`L-2` holds `L-2.1` and
`L-2.2`) and nests the parts that satisfy them the same way, so `Descendants`
with a depth limit collects the tree and `satisfiedBy` cells show the nested
path. Verification cases weigh, fire and drop the design: `hotFire` fails
against the declared thrust, `weighLander` and `dropTest` pass. The report's
coverage section builds its sets from `WhereRelated` and combines them —
`Union` for "unsatisfied or unverified", `Except` for "satisfied but not
verified" — and closes with the verdict table filtered to what came out
false. The tree is ordered by `shortName`, which puts `L-2.1` under `L-2`;
ordering by `qualifiedName` would sort the nested requirements by their
parents' names instead.

## Tier 3: derivation chains

`RangeDerivation` derives system requirements from mission requirements and
subsystem requirements from those, with `#derivation connection`s, and
refines two leaves with analyses through `#refinement dependency` targets
written as qualified names (`specification::system::energyBudget`). The
lineage table puts `derivedFrom` (one hop inward), `derives` (one hop
outward), `descendants` (three hops outward, counted) and `refinedBy` beside
each requirement; the chain sections bind one requirement at a time —
`in req = specification.mission.range`, in dot notation — and walk outward
to one hop or to the chain's end, or inward back to the mission requirement.
The last table filters the leaves — the requirements deriving nothing
further — to those no part satisfies, which is where a derivation tree's
uncovered ends show.

## Tier 4: a program across packages

`ProgramRequirements` holds stakeholder needs, system requirements and
subsystem requirements owned by the Program, Systems, Power, Comms and
Thermal teams; `PowerDesign`, `CommsDesign` and `ThermalDesign` satisfy,
refine and verify them from their own packages, and the power team keeps a
derived requirement of its own beside its design. The report's matrix is one
`Project` over the union of both requirement roots with ten columns —
ownership and priority beside `derivedFrom`, a counted `descendants`,
`refinedBy`, `satisfiedBy`, a counted `verifications` and a Boolean
`satisfied` — grouped by `team`. The coverage sections take the union of the
unsatisfied and unverified sets and subtract the requirements that derive
others, since those are met through their children, to find the uncovered
leaves; filter those to the critical ones; and filter the unsatisfied set by
an incoming refinement to find the requirements under analysis that nothing
satisfies yet. The verdict tables union the three design packages' verdicts
into one.

Two details of the model carry over to reports of this shape. A `satisfy`
statement is itself a `RequirementUsage`, so collecting every requirement
under a root that also holds `satisfy` statements takes `Except` against
`WhereType(..., type = "SatisfyRequirementUsage")`, or the satisfy usages turn
up as blank rows. And `stakeholder` is a keyword: a part holding the
stakeholder needs is named something else.

In PDF, the ten-column matrix lands on landscape pages while the narrower
coverage and verdict tables stay portrait; see
[Outputs](outputs.md#wide-tables).

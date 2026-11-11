# Magic MDK

The Cameo Systems Modeler plugin of the OpenSysML MDK: it runs, verifies and analyzes a model with
OpenSysML inside Cameo, and is the successor to OpenMBEE's Model Development Kit (MDK) for SysML v2
work. OpenSysML MDK is the whole kit; Magic MDK is this plugin. Where MDK is still
installed, the plugin also plugs OpenSysML into MDK's DocGen as a set of «JavaExtension»
queries (see *MDK DocGen bridge* below). The capability-by-capability comparison with MDK, and
the order the gaps are being closed in, is
[`docs/project/mdk-parity.md`](../../docs/project/mdk-parity.md).

## Build

Install the Java client and its dependencies before building the plugin:

```sh
mvn -q -f client/java/pom.xml -pl :opensysml -am install -DskipTests
mvn -f editors/mdk/pom.xml verify
```

Continuous integration compiles against the compile-only OpenAPI stubs. The stubs are not packaged
in the plugin. A licensed Cameo installation can be checked with
`CAMEO_HOME=/path/to/Cameo editors/mdk/scripts/compile-against-cameo.sh`.

## Local compile against a real Cameo

Set `CAMEO_HOME` to a licensed installation. The helper expands the host libraries and compiles
the plugin against Java 17:

```sh
CAMEO_HOME=/opt/Cameo editors/mdk/scripts/compile-against-cameo.sh
```

## Stubs

`openapi-stubs` contains compile-only signatures for the Cameo 2026x OpenAPI, written from the
vendor's Javadoc. `mdk-api-stubs` does the same for the handful of MDK classes the bridge
extends (`Query`, `DocGenElement`, `DocumentElement`, `DBParagraph`, `DBText`, `DBTable`), written
from the `develop` branch of [Open-MBEE/mdk](https://github.com/Open-MBEE/mdk), which publishes no
API artifact. Both are provided to the build only and are never included in a runtime jar or the
distribution; a signature that drifts from the vendor's is caught by
`scripts/compile-against-cameo.sh` on a licensed machine, not by CI.

## Compatibility

The one jar runs on the 2024x line (2024x through 2024x Refresh3, JDK 17) and on 2026x (JDK 21):
the plugin and the Java client are compiled with `--release 17`, and `OpenApiFloorTest` runs
`jdeps` over the compiled plugin and checks every `com.nomagic.*` and `com.dassault_systemes.*`
class it links against the public 2024x OpenAPI class index in `compat/openapi-classes-2024x.txt`.
The three classes of the SysML v2 textual export (`v2.V2Exporter`, `v2.V2Elements`,
`selection.V2Selections`) are the only exemption — the service they use exists only on 2026x,
`PathSelector` probes for it before any of them is loaded, and the test checks them against the
2026x Refresh1 index instead. `scripts/refresh-openapi-index.sh <release>...` regenerates the
index files from `https://jdocs.nomagic.com/<release>/allclasses-index.html`.
The gate is class-level: a member that 2026x added to a class 2024x also has is caught only by
`scripts/compile-against-cameo.sh` on a licensed 2024x installation, so run that before a release.

2022x Refresh2 and earlier run on JDK 11 and cannot load the jar; see
`docs/internals/design/mdk-plugin.md` §1.1 for what supporting them would take.

## Model paths and identity

The v1 path exports a clean existing `.mdzip` directly; otherwise it exports the whole project so
cross-references survive. This whole-project export is intentional because a selected-element-only
export can omit required references. V1 identity is qualified-name-only: the plugin does not yet
map results through the per-element `id → target` entries of the migration report
`Connection.migrateFile` returns, so there is no source-ID mapping; ambiguous candidates are
retained rather than silently discarded. The report's entries are read in one direction only,
from the selected element's name to the declarations written for it, which is how *Run query*
finds a migrated DocGen table's query (below). The report's summary line is surfaced as a
diagnostic of the run. V2 textual export is used only when both the textual service and a v2
selection are available.

## Export as SysML v2

*OpenSysML ▸ Export as SysML v2…* on a selected element writes the whole project as SysML v2
text to a `.sysml` file of the user's choosing. A v1 project goes through the same `.mdzip`
export and migration the operations use; every diagram comes out as a `view` usage laid out by
`DiagramLayout` metadata (`@Canvas`, `Layout about …`, `Route about …`) from the project's own
diagram symbols, and the action offers to lay the views out from an MTIP (HUDS XML) export of the
project instead. The migration report lands beside the file as `<name>.migration-report.txt`,
with any image files the views embed under their relative paths; files the export would replace
beside the model (another export's images or report) are listed for confirmation first, and the
export is staged and moved into place, so a failure leaves the previous files as they were. A
v2 project's text is written
as `SysMLTextualNotationService.exportTextual` returns it — no report, and no geometry, which the
vendor keeps outside the text (`docs/internals/design/mdk-mtip.md` §6). The completion dialog
counts the service's diagnostics on the written text: a v1 model that migrates but does not parse
clean is a migration finding, and the report says what was approximated.

## Run query

*OpenSysML ▸ Run query* runs the QVTS document queries a selected element stands for through the
service's `RunDocumentQuery` ([the QVTS guide](../../docs/guide/qvts.md)). On the v2 path the
element is a `calc def` specializing `DocumentQueries::Query`, directly or through other
query definitions. On the v1 path the SysML v1 migrator writes a project's DocGen documents as
`DocumentQueries::Document` definitions whose tables and lists compute their rows with
`calc rows : '<Document> <Title> Rows'` members typed by query definitions of its own
([SysML v1 migration](../../docs/reference/sysml-v1-migration.md) § DocGen documents); the
plugin follows the migration report from the selected element to what was written for it (a
`«TableStructure»` node to its `part … table`, a `«Document»` class to its Document definition)
and runs every query found below that declaration, so a DocGen table runs as one query and a
Document as all of its. An element that stands for no query — a v1 `«View»` whose document
section holds no table, say — is reported as an error naming what was looked for.

A query's `in` parameters are asked for first, one field per parameter, read by the declared
type: an element parameter (`KerML::Root::Element`, or any non-scalar type) takes the qualified
name of a Cameo element and starts out as the selected element when it is the only element
parameter without a value of its own; a `String` takes quoted or bare text, the numeric and
`Boolean` types their literals, and a parameter of many values a `,`-separated list. A parameter
with a value in its declaration may stay blank. The service checks the bindings against the
declarations, so a missing or mistyped one is reported in its own words. The same bindings are
written for the DocGen bridge as `name = value; name = value, value` (or positionally, in
declaration order) in the `arguments` tag.

The answer is the query's columns and rows, shown as a table under the outcomes in the results
window: one outcome per query (`n rows of qualifiedName, mass`, or `no rows`), one row per
element the query selected. Every row names the element it was computed for, and every cell that
holds one element reference or the `qualifiedName` a column projects names that element, so a
double-click selects it in the containment tree and each row annotates its element with the
row's values. No rows is an informational success — in QVTS an empty selection is a value — so
the run is `INFO`, never a failure.

A query the model has no answer for — a misspelled `Named` source, a feature some selected
element lacks, a name two elements share — ran and reported, so it is `FAILED`, not `ERROR`. The
Java client throws a `QueryOutcomeException` carrying the service's structured outcome, and
`results/QueryFailure` maps it to the query's own outcome (the service's message) followed by
one outcome per path failure and per model finding, worded as the CLI words them
([the QVTS guide](../../docs/guide/qvts.md)):

```text
path error at Named(Lander::lander::cabni): expected exactly one element with that qualified name; at Lander::lander; members: [cabin (PartUsage), cabinEngine (ConnectionUsage), engine (PartUsage), engineBay (ConnectionUsage), scienceBay (PartUsage)]; nearest: cabin
path error at Feature(mass): expected a feature named mass on the element; at Lander::lander::cabinEngine
model error at Named(Duplicates::repeated): Duplicates violates ambiguousName: Duplicates::repeated (declared 2 times)
```

Each of those outcomes names the element the path stopped at (the frontier's `at`) or the
element the finding is about, so the element is annotated as an error and selected from the
results window; on the v1 path the name is mapped back to the Cameo element the same way a
row's is. The service sends at most eight candidates per list and eight failures per step: a
list that was cut ends `… and N more`, and failures that were left out are counted in a last
outcome (`… and 3 more path errors at Feature(mass)`). A candidate list with nothing in it is
left out. The queries beside a failed one still answer and keep their tables. A refusal that
carries no outcome — a parameter left unbound, say — stays one `ERROR` outcome carrying the
service's message, and outranks `FAILED` for the run's status.

### The answer as a view

Every query that ran also comes back as a `QVTS::QueryResult` view
([the QVTS guide](../../docs/guide/qvts.md#write-the-answer-as-a-view)): the selected elements
exposed, and the query, the source, the outcome and the provenance as its four attributes. A
query that reported a path or model error has a view too, of that outcome, exposing the
elements the failure is about. After *Run query* the plugin writes each view to a file named by
the query, `<model>.<Query.Qualified.Name>.result.sysml`, replacing the one an earlier run of
that query wrote (a query whose name or text holds more than plain names also carries a short
digest of it, `…~1a2b3c4d.result.sysml`, so two queries never share a file):

- **v2 project** — next to the project file, so `lander.mdzip` gets
  `lander.QVTSDemo.CabinMass.result.sysml`.
- **v1 project** — beside the `.sysml` the project was last written to by *Export as SysML
  v2…* in this Cameo session, which is where the migration report is, so the view parses with
  the migrated model it names; before any export, next to the project file.

The results window lists each file as an informational diagnostic. A project that was never
saved has nowhere to write, which is a warning, as is a file that cannot be written; neither
changes the run's status.

## Render document

*OpenSysML ▸ Render document…* renders what the selected element stands for as Markdown and
HTML into a folder you choose, then opens the folder:

- a `part def` specializing `DocumentQueries::Document` (v2), through the service's
  `RenderDocument`;
- a v1 DocGen `«Document»`, through the `Document` definition the migration wrote for it,
  found by the migration report as *Run query* finds a query;
- a view typed by `QVTS::QueryResult`, through `RenderView`: its four attributes as the header
  and the elements it exposes as the rows. Add a written `.result.sysml` to the project's
  model to render it.

The files are named by what was rendered (`QVTSDemo.MassReport.md`, `.html`). An element that
is none of these is reported as such, and nothing is written.

Not in scope here: PDF from Cameo (the converters are the CLI's, `sysml -render-document
-doc-form pdf`), and transformations or proposals, which OpenSysML has no evaluator for yet.

## Results window and annotations

Each project has one OpenSysML results window. It displays operation metadata, outcomes,
diagnostics, state schedules, and the rows a query answered. Result outcomes are mapped to Cameo annotations with passed,
failed, and error severities; annotations from the prior application are removed before new ones
are added.

## MDK DocGen bridge

MDK's DocGen loads «JavaExtension» queries from `plugins/org.openmbee.mdk/extensions/`, so the
distribution drops `magic-mdk-bridge-<n>.jar` there and nothing extra needs installing. The
bridge defines one `Query` per operation, in package `org.openmbee.mdk.magic.docgen`:
`Instantiate`, `ExecuteAction`, `ExecuteState`, `Verify`, `EvaluateCalc`, `RunAnalysis` and
`RunQuery`.

To use one in a document:

1. Make sure the project uses MDK's *SysML Extensions* profile, then right-click the model root
   and choose *OpenSysML ▸ Add MDK DocGen extension stereotypes*. This creates, once, a profile
   `Magic MDK DocGen` applied to the model, holding a stereotype per operation; each
   specialises MDK's «JavaExtension» and is named after the class MDK must load
   (e.g. `org.openmbee.mdk.magic.docgen.Verify`). `EvaluateCalc` and `RunQuery` carry an
   `arguments` tag: a comma-separated argument list for the calc, the query's parameter
   bindings as `name = value; …` for the query.
2. In a viewpoint method, apply the stereotype to an activity (or to a call behavior action
   whose behavior carries it) and feed it the elements to run on, exactly as for any other DocGen
   query. Targets may be UML elements (the SysML v1 path) or SysML v2 elements (the textual
   path), as for the context-menu operations.

For every target the generated section gets a one-line summary (operation, element, status,
elapsed time, final time), a table of outcomes with their status, for `RunQuery` one table per
query answered (its columns the properties projected, one row per element selected, shown even
when empty) and, for a query that reported a path or model error, the same failure text the
results window shows as rows of the outcome table, a table of diagnostics with
their source locations, and the state schedule when the operation produced one. A target that
fails to run yields an error paragraph in its place; the rest of the document is unaffected.

The bridge jar lives in MDK's extension classloader and holds neither the Java client nor the
service binary: it finds the OpenSysML plugin through Cameo's plugin registry and calls its one
facade method, `MagicMdkPlugin.docGen(BaseElement, String, String)`, whose result is a map of JDK
types. Only Cameo and JDK types cross between the two plugins, so the plugin keeps its own
classloader and the bridge is inert — never loaded — when MDK is not installed.

## Distribution

Naming a pinned OpenSysML release stages its service binaries and zips the Resource Manager
layout to `dist/target/Magic_MDK_<version>.zip`:

```sh
mvn -f editors/mdk/pom.xml -Dopensysml.version=v1.2.3 package
```

`BinaryStager` downloads the five `sysml-grpc` release assets from GitHub, verifies each against
the SHA-256 table the Java client jar ships (`release-digests.json`; an unpinned version or a
mismatching asset fails the build) and writes `bin/DIGESTS`. `PluginDescriptorWriter` generates
`plugin.xml` with a `<library>` entry for the plugin jar and every runtime jar, so the plugin's
own classloader (`ownClassloader="true"`, `class-lookup="LocalFirst"`) sees exactly what ships.
`-Dopensysml.binaries=<dir>` stages the five binaries from a directory instead, for a build the
caller has already verified — the nightly workflow packages each night's `sysml-grpc` this way,
under the night's tag, and keeps the zip as the run's `cameo-plugin` artifact (see
[the snapshot page](../../docs/project/nightly.md#what-it-contains)):

```
plugins/org.openmbee.mdk.magic/plugin.xml
plugins/org.openmbee.mdk.magic/magic-mdk-plugin-<n>.jar
plugins/org.openmbee.mdk.magic/lib/*.jar
plugins/org.openmbee.mdk.magic/bin/sysml-grpc-<os>-<arch>[.exe], DIGESTS
plugins/org.openmbee.mdk/extensions/magic-mdk-bridge-<n>.jar
data/resourcemanager/MDR_Plugin_Magic_MDK_91120_descriptor.xml
```

The bridge jar is listed in no `plugin.xml`: MDK scans its `extensions/` directory itself, and
Cameo ignores the directory when MDK is absent.

## Tests

Run the compile-only suite with `mvn -f editors/mdk/pom.xml verify`; it covers the plugin and
the bridge (facade flattening, plugin lookup by descriptor id, DocBook rendering, per-target
failure handling). The service-backed tests
use the repository's `bin/sysml-grpc`; require that binary with:

```sh
mvn -f editors/mdk/pom.xml -pl plugin -am \
  -Dopensysml.requireService=true \
  -Dtest=PipelineEndToEndTest \
  -Dsurefire.failIfNoSpecifiedTests=false test
```

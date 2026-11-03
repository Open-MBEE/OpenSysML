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
read the per-element `id → target` entries of the migration report `Connection.migrateFile`
returns, so there is no source-ID mapping; ambiguous candidates are retained rather than silently
discarded. The report's summary line is surfaced as a diagnostic of the run. V2 textual export is used only
when both the textual service and a v2 selection are available.

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

## Results window and annotations

Each project has one OpenSysML results window. It displays operation metadata, outcomes,
diagnostics, and state schedules. Result outcomes are mapped to Cameo annotations with passed,
failed, and error severities; annotations from the prior application are removed before new ones
are added.

## MDK DocGen bridge

MDK's DocGen loads «JavaExtension» queries from `plugins/org.openmbee.mdk/extensions/`, so the
distribution drops `magic-mdk-bridge-<n>.jar` there and nothing extra needs installing. The
bridge defines one `Query` per operation, in package `org.openmbee.mdk.magic.docgen`:
`Instantiate`, `ExecuteAction`, `ExecuteState`, `Verify`, `EvaluateCalc` and `RunAnalysis`.

To use one in a document:

1. Make sure the project uses MDK's *SysML Extensions* profile, then right-click the model root
   and choose *OpenSysML ▸ Add MDK DocGen extension stereotypes*. This creates, once, a profile
   `Magic MDK DocGen` applied to the model, holding a stereotype per operation; each
   specialises MDK's «JavaExtension» and is named after the class MDK must load
   (e.g. `org.openmbee.mdk.magic.docgen.Verify`). `EvaluateCalc` carries an `arguments` tag,
   a comma-separated argument list.
2. In a viewpoint method, apply the stereotype to an activity (or to a call behavior action
   whose behavior carries it) and feed it the elements to run on, exactly as for any other DocGen
   query. Targets may be UML elements (the SysML v1 path) or SysML v2 elements (the textual
   path), as for the context-menu operations.

For every target the generated section gets a one-line summary (operation, element, status,
elapsed time, final time), a table of outcomes with their status, a table of diagnostics with
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

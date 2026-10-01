# OpenSysML MDK

The Cameo Systems Modeler plugin that runs, verifies and analyzes a model with OpenSysML, and the
successor to OpenMBEE's Model Development Kit (MDK) for SysML v2 work. Where MDK is still
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

## Model paths and identity

The v1 path exports a clean existing `.mdzip` directly; otherwise it exports the whole project so
cross-references survive. This whole-project export is intentional because a selected-element-only
export can omit required references. V1 identity is qualified-name-only: the Java client's
`ConvertResponse` does not provide a migration report, so there is no reliable source-ID mapping;
ambiguous candidates are retained rather than silently discarded. V2 textual export is used only
when both the textual service and a v2 selection are available.

## Results window and annotations

Each project has one OpenSysML results window. It displays operation metadata, outcomes,
diagnostics, and state schedules. Result outcomes are mapped to Cameo annotations with passed,
failed, and error severities; annotations from the prior application are removed before new ones
are added.

## MDK DocGen bridge

MDK's DocGen loads «JavaExtension» queries from `plugins/org.openmbee.mdk/extensions/`, so the
distribution drops `opensysml-mdk-bridge-<n>.jar` there and nothing extra needs installing. The
bridge defines one `Query` per operation, in package `org.openmbee.opensysml.mdk.docgen`:
`Instantiate`, `ExecuteAction`, `ExecuteState`, `Verify`, `EvaluateCalc` and `RunAnalysis`.

To use one in a document:

1. Make sure the project uses MDK's *SysML Extensions* profile, then right-click the model root
   and choose *OpenSysML ▸ Add MDK DocGen extension stereotypes*. This creates, once, a package
   `OpenSysML MDK DocGen` holding a stereotype per operation; each specialises MDK's
   «JavaExtension» and is named after the class MDK must load
   (e.g. `org.openmbee.opensysml.mdk.docgen.Verify`). `EvaluateCalc` carries an `arguments` tag,
   a comma-separated argument list.
2. In a viewpoint method, apply the stereotype to an activity (or to a call behavior action
   whose behavior carries it) and feed it the elements to run on, exactly as for any other DocGen
   query.

For every target the generated section gets a one-line summary (operation, element, status,
elapsed time, final time), a table of outcomes with their status, a table of diagnostics with
their source locations, and the state schedule when the operation produced one. A target that
fails to run yields an error paragraph in its place; the rest of the document is unaffected.

The bridge jar lives in MDK's extension classloader and holds neither the Java client nor the
service binary: it finds the OpenSysML plugin through Cameo's plugin registry and calls its one
facade method, `OpenSysMLPlugin.docGen(Element, String, String)`, whose result is a map of JDK
types. Only Cameo and JDK types cross between the two plugins, so the plugin keeps its own
classloader and the bridge is inert — never loaded — when MDK is not installed.

## Distribution

Naming a pinned OpenSysML release stages its service binaries and zips the Resource Manager
layout to `dist/target/OpenSysML_MDK_<version>.zip`:

```sh
mvn -f editors/mdk/pom.xml -Dopensysml.version=v1.2.3 package
```

`BinaryStager` downloads the five `sysml-grpc` release assets from GitHub, verifies each against
the SHA-256 table the Java client jar ships (`release-digests.json`; an unpinned version or a
mismatching asset fails the build) and writes `bin/DIGESTS`. `PluginDescriptorWriter` generates
`plugin.xml` with a `<library>` entry for the plugin jar and every runtime jar, so the plugin's
own classloader (`ownClassloader="true"`, `class-lookup="LocalFirst"`) sees exactly what ships:

```
plugins/org.openmbee.opensysml.mdk/plugin.xml
plugins/org.openmbee.opensysml.mdk/opensysml-mdk-plugin-<n>.jar
plugins/org.openmbee.opensysml.mdk/lib/*.jar
plugins/org.openmbee.opensysml.mdk/bin/sysml-grpc-<os>-<arch>[.exe], DIGESTS
plugins/org.openmbee.mdk/extensions/opensysml-mdk-bridge-<n>.jar
data/resourcemanager/MDR_Plugin_OpenSysML_MDK_descriptor.xml
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

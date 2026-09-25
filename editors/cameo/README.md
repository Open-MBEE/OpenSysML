# OpenSysML Cameo plugin

## Build

Install the Java client and its dependencies before building the plugin:

```sh
mvn -q -f client/java/pom.xml -pl opensysml-client -am install -DskipTests
mvn -f editors/cameo/pom.xml verify
```

Continuous integration compiles against the compile-only OpenAPI stubs. The stubs are not packaged
in the plugin. A licensed Cameo installation can be checked with
`CAMEO_HOME=/path/to/Cameo editors/cameo/scripts/compile-against-cameo.sh`.

## Local compile against a real Cameo

Set `CAMEO_HOME` to a licensed installation. The helper expands the host libraries and compiles
the plugin against Java 17:

```sh
CAMEO_HOME=/opt/Cameo editors/cameo/scripts/compile-against-cameo.sh
```

## Stubs

`openapi-stubs` contains compile-only signatures for the Cameo 2026x OpenAPI. It is provided to
the plugin build and is never included in the runtime jar or distribution.

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

## Distribution

Naming a pinned OpenSysML release stages its service binaries and zips the Resource Manager
layout to `dist/target/OpenSysML_Cameo_<version>.zip`:

```sh
mvn -f editors/cameo/pom.xml -Dopensysml.version=v1.2.3 package
```

`BinaryStager` downloads the five `sysml-grpc` release assets from GitHub, verifies each against
the SHA-256 table the Java client jar ships (`release-digests.json`; an unpinned version or a
mismatching asset fails the build) and writes `bin/DIGESTS`. `PluginDescriptorWriter` generates
`plugin.xml` with a `<library>` entry for the plugin jar and every runtime jar, so the plugin's
own classloader (`ownClassloader="true"`, `class-lookup="LocalFirst"`) sees exactly what ships:

```
plugins/org.openmbee.opensysml.cameo/plugin.xml
plugins/org.openmbee.opensysml.cameo/opensysml-cameo-plugin-<n>.jar
plugins/org.openmbee.opensysml.cameo/lib/*.jar
plugins/org.openmbee.opensysml.cameo/bin/sysml-grpc-<os>-<arch>[.exe], DIGESTS
data/resourcemanager/MDR_Plugin_OpenSysML_cameo_descriptor.xml
```

## Tests

Run the compile-only suite with `mvn -f editors/cameo/pom.xml verify`. The service-backed tests
use the repository's `bin/sysml-grpc`; require that binary with:

```sh
mvn -f editors/cameo/pom.xml -pl plugin -am \
  -Dopensysml.requireService=true \
  -Dtest=PipelineEndToEndTest \
  -Dsurefire.failIfNoSpecifiedTests=false test
```

# OpenSysML SysON integration

This integration lets Eclipse SysON run, explore, instantiate, verify and analyze selected
models with OpenSysML. The backend adapts the Java client to SysON's Spring and GraphQL APIs;
the frontend adds the explorer action, operation dialog and mapped run results.

## Layout

- `backend/` — the Spring Boot auto-configuration, exporter, GraphQL mutation, run service,
  result mapping and Validation view service.
- `frontend/` — the `@openmbee/opensysml-syson` npm package with the menu contribution,
  dialog, results panel and extension registry contribution.
- `syson-api-stubs/` — compile-only Sirius Web and SysON API classes for the offline Maven build.

## Build without private artifacts

The `stubs` Maven profile is active by default and does not require GitHub Packages credentials.
Install the Java client and build the backend from the repository root:

```text
mvn -B -f client/java/pom.xml install -DskipTests
mvn -B -f editors/syson/pom.xml install -Dopensysml.requireService=true
```

The backend tests use `bin/sysml-grpc` when it is present. Set `OPENSYSML_GRPC_BINARY`, or
configure `opensysml.binary-path`, when the service binary is elsewhere. The frontend's default
build uses ambient Sirius type declarations and test doubles:

```text
cd editors/syson/frontend
npm ci
npm run typecheck
npm run format:check
npm test
npm run build
```

## Build against real SysON artifacts

Real Sirius Web and SysON artifacts are hosted on GitHub Packages. The
`syson-artifacts.yml` workflow reads the repository secret `GITHUB_PACKAGES_READ_TOKEN`; for a
local build, set `GITHUB_ACTOR` and `GITHUB_TOKEN` to a token with `read:packages`, then use the
supplied Maven settings:

```text
mvn -B -s editors/syson/settings.xml -f editors/syson/pom.xml \
  -Psyson-artifacts -P '!stubs' install -Dopensysml.requireService=true
```

For the frontend, copy `frontend/.npmrc.example` to `frontend/.npmrc` and provide
`GITHUB_TOKEN` in the environment:

```text
cd editors/syson/frontend
npm ci
cp .npmrc.example .npmrc
npm run install:syson
npm run build:syson
```

The opt-in `.github/workflows/syson-artifacts.yml` workflow runs both real-artifact builds.
The real-artifact backend tests use a `src/test/real-java` fake element because the published
`Element` interface is too large to implement directly; the default stub build uses its own
`src/test/stubs-java` fake element.

## Add the integration to SysON

Put the backend jar on the SysON application classpath. Its
`META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports` entry
activates `OpenSysMLAutoConfiguration`; no package-scan change is needed. In the SysON frontend
`index.tsx`, add `addOpenSysMLContributions(sysonExtensionRegistry)` after the existing SysON
contributions.

## Configuration

All backend settings use the `opensysml.*` prefix:

| Property | Meaning |
| --- | --- |
| `opensysml.binaryPath` | Path to the `sysml-grpc` executable. |
| `opensysml.service` | Existing service address as `host:port`. |
| `opensysml.downloadVersion` | OpenSysML release version to download. |
| `opensysml.expectedBinarySha256` | Required SHA-256 digest for a downloaded binary. |
| `opensysml.githubRepo` | GitHub repository used for binary downloads. |
| `opensysml.allowUnpinnedDownload` | Permit downloads without a pinned version and digest. |
| `opensysml.requestTimeout` | Service request timeout. |
| `opensysml.startupTimeout` | Service startup timeout. |
| `opensysml.schedule` | Default schedule for non-exploration operations. |
| `opensysml.performer` | Default execution performer. |
| `opensysml.explore.runs` | Exploration run budget. |
| `opensysml.explore.depth` | Exploration depth budget. |

`explore.runs` and `explore.depth` are encoded as `explore:runs=N,depth=D` and sent as the
Java client's exploration schedule string. An explicit dialog schedule takes precedence.

## GraphQL and tests

The backend adds the `runWithOpenSysML(input: RunWithOpenSysMLInput!)` mutation. It supports
instantiation, action and state execution or exploration, constraint and requirement verification,
satisfaction verification, calculation evaluation, analysis, and instance validation.

Unit tests cover exporting, identity mapping, diagnostic mapping, configuration, GraphQL payloads
and validation results. Real-service integration tests run the mutation against `sysml-grpc` for
instantiation, action execution, verification, diagnostics and stored results. Frontend tests
cover the registry, explorer menu, operation dialog, toast errors, mapped selection, outputs,
traces, verdicts, instances and diagnostics.

Diagnostics that name a qualified name select that element; span-only diagnostics select the root element of the exported document containing the line. The serializer emits no source map.

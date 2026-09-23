# A SysON plugin: OpenSysML as the execution engine behind Eclipse SysON

**Date:** 2026-09-20
**Status:** Discovery, design and implementation notes for `editors/syson/`
**Scope:** a future `editors/syson/`, the Java client under `client/java/opensysml-client`, the wire in `api/proto/sysml.proto`
**SysON pinned at:** [`v2026.9.0`](https://github.com/eclipse-syson/syson/releases/tag/v2026.9.0) (commit `ede4fbc43a607720350b850a862211660a99652a`); `main` was one commit ahead of the tag when this was written and differs in nothing this document relies on

---

## 1. What this is

[Eclipse SysON](https://github.com/eclipse-syson/syson) is a web modeling workbench for SysML v2
built on [Sirius Web](https://github.com/eclipse-sirius/sirius-web): a Spring Boot backend that
holds each project as an EMF resource set persisted as JSON in PostgreSQL, and a React frontend
that draws the explorer, diagrams, tables and forms. SysON edits models; it does not execute
them. OpenSysML executes, verifies and analyzes them but has no graphical editor. The plugin
this document designs lets a SysON user select an element and run, instantiate, execute or
verify it on OpenSysML, see the result inside SysON, and optionally have OpenSysML check the
textual notation SysON imports.

Section 2 answers the eight discovery questions against the pinned SysON checkout, each with
the files that answer it. Section 3 is the architecture, section 4 the phased plan, section 5
the unknowns and risks. Every statement that was not read from a file or run on this machine is
marked **unverified**.

Paths below are relative to the SysON checkout unless they begin with `client/`, `internal/`,
`docs/` or `api/`, which are this repository.

## 2. Discovery

### 2.1 Backend extension: how a jar contributes beans

SysON's backend is one Spring Boot application, and third-party code joins it by being on its
classpath in a package the application scans. There is no service-loader layer and no plugin
registry of its own:

```java
// backend/application/syson-application/src/main/java/org/eclipse/syson/SysONApplication.java
@SpringBootApplication
@ComponentScan(basePackages = { "org.eclipse.syson", "org.eclipse.sirius.web", "org.eclipse.sirius.components" })
public class SysONApplication { ... }
```

So a jar contributes by declaring `@Service`/`@Configuration` classes in a package under one of
those three roots, or by shipping a Spring Boot auto-configuration
(`META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports`) so its own
package name is free. Every SysON module does the former: `SysMLv2DocumentExporter`,
`SysideSysMLTextImporter`, `SysONSysMLValidationService`, `InsertTextualSysMLv2EventHandler`
are plain `@Service` beans implementing Sirius interfaces, and Sirius collects them by type.

The application itself is assembled by Maven, not at runtime:
`backend/application/syson-application/pom.xml` depends on `org.eclipse.sirius:sirius-web-starter`
plus one `org.eclipse.syson` artifact per feature (`syson-sysml-metamodel`, `syson-sysml-import`,
`syson-sysml-export`, `syson-sysml-validation`, `syson-sysml-rest-api-services`, the `syson-*-view`
modules, `syson-frontend`). Adding a plugin to a SysON build therefore means adding one
dependency to that POM (or to a downstream application POM that depends on `syson-application`),
which is how SysON's own features are added. There is no way to drop a jar into a running
SysON distribution; **unverified**: whether the SysON Docker image's classpath can be extended
with a `loader.path` (Spring Boot's `PropertiesLauncher`) without rebuilding — the image build
was not inspected.

The Sirius interfaces a plugin implements (all read in SysON's sources, so present in
`sirius-web` 2026.9.0):

| Contribution | Interface | SysON example |
|---|---|---|
| a GraphQL mutation | `@QueryDataFetcher(type="Mutation", field=…)` + `IDataFetcherWithFieldCoordinates`, schema in `src/main/resources/schema/*.graphqls` | `syson-sysml-import/.../datafetchers/MutationInsertTextualSysMLv2DataFetcher.java`, `schema/syson-import.graphqls` |
| its handler on the editing context | `org.eclipse.sirius.components.collaborative.api.IEditingContextEventHandler` over an `IInput` record | `syson-sysml-import/.../services/InsertTextualSysMLv2EventHandler.java` |
| explorer context-menu entries | `ITreeItemPaletteCustomizer` returning `SingleClickTreeItemTool`s | `syson-tree-explorer-view/.../menu/context/SysONExplorerTreeItemContextMenuEntryProvider.java` |
| a document exporter | `org.eclipse.sirius.web.application.document.services.api.IDocumentExporter` | `syson-sysml-export/.../SysMLv2DocumentExporter.java` |
| a textual loader | `org.eclipse.sirius.web.application.document.services.api.IExternalResourceLoaderService` | `syson-sysml-import/.../upload/SysMLExternalResourceLoaderService.java` |
| validation | `org.eclipse.sirius.components.core.api.IValidationService` | `syson-sysml-validation/.../SysONSysMLValidationService.java` |
| element lookup | `org.eclipse.sirius.components.core.api.IObjectSearchService`, `IIdentityService` | everywhere |

**Versions and JDK.** The root `pom.xml` sets `<java.version>21</java.version>`,
`<sirius.web.version>2026.9.0</sirius.web.version>`, `<syson.version>2026.9.0</syson.version>`
and inherits from `spring-boot-starter-parent` `4.1.0`; CI (`.github/workflows/build.yml`) builds
on Temurin 21. The OpenSysML Java client is compiled for release 17
(`client/java/opensysml-client/pom.xml`, `docs/reference/java-api.md`) and its only compile
dependency is protobuf, so it runs unchanged on the SysON JDK.

The Maven coordinates a plugin depends on, all at `2026.9.0`:

- `org.eclipse.sirius:sirius-web-starter` — pulls the Sirius Web application, collaborative,
  core and EMF modules with the interfaces above;
- `org.eclipse.syson:syson-sysml-metamodel` — the generated SysML v2 EMF metamodel
  (`org.eclipse.syson.sysml.Element` and friends);
- `org.eclipse.syson:syson-sysml-metamodel-services` — `SysMLElementSerializer`, `ElementUtil`;
- `org.eclipse.syson:syson-sysml-export` and `syson-sysml-import` — only if the plugin calls
  `SysMLv2DocumentExporter` or `ISysMLTextImporter` directly rather than through Sirius;
- `org.eclipse.syson:syson-tree-explorer-view` — only to reuse the explorer palette constants.

**Where they come from.** Neither Sirius Web nor SysON publishes to Maven Central. SysON's
`settings.xml` names GitHub Packages repositories for `eclipse-sirius/sirius-web`,
`eclipse-sirius/sirius-emf-json` and `eclipse-syson/syson`, and GitHub Packages requires an
authenticated request even for public packages (`GET …/sirius-web-starter-2026.9.0.pom` answered
`401` on this machine, see §2.8). A plugin build therefore needs a GitHub token in Maven
settings, as SysON's own CI has.

### 2.2 Frontend extension: React contributions

The frontend is two npm workspaces (`frontend/syson`, `frontend/syson-components`, root
`package.json` with Turbo). SysON does not have a frontend plugin mechanism of its own either; it
uses Sirius Web's `ExtensionRegistry`. `frontend/syson/src/index.tsx` builds one registry, adds
its contributions, and renders `<SiriusWebApplication extensionRegistry={…}>`:

```tsx
sysONExtensionRegistry.addComponent(footerExtensionPoint, { identifier: `syson_${footerExtensionPoint.identifier}`, Component: SysONFooter });
root.render(<SiriusWebApplication httpOrigin={httpOrigin} wsOrigin={wsOrigin} theme={sysonTheme}
  extensionRegistryMergeStrategy={new SysONExtensionRegistryMergeStrategy()} extensionRegistry={sysONExtensionRegistry}>
  <DiagramRepresentationConfiguration nodeTypeRegistry={sysONNodeTypeRegistry} />
</SiriusWebApplication>);
```

`frontend/syson-components/src/extensions/registry/SysONExtensionRegistry.tsx` is the catalogue
of extension points SysON already uses, and the two the plugin needs are there:

- **A context-menu action on explorer items.** The backend adds a palette entry with an id (the
  `SingleClickTreeItemTool("newObjectsFromText", …)` in
  `SysONExplorerTreeItemContextMenuEntryProvider`), and the frontend overrides how that id is
  rendered through `treeItemContextMenuEntryOverrideExtensionPoint` from
  `@eclipse-sirius/sirius-components-trees`, with a `TreeItemContextMenuOverrideContribution`
  whose `canHandle(entry)` matches the id and whose `component` is a `forwardRef` `MenuItem`
  (`InsertTextualSysMLv2MenuContribution.tsx`). It receives `editingContextId`, `treeId`,
  `item`, `readOnly`, `expandItem` and `onClose`, and may open a MUI dialog — exactly the shape
  a "Run with OpenSysML…" entry wants. Diagram-side, the same registry has
  `paletteToolOverrideExtensionPoint` (`RotateNodeToolOverriddenContribution.tsx`) for a
  tool on a node and `diagramToolbarActionExtensionPoint` (`SysONDiagramPanelMenu.tsx`) for a
  toolbar button.
- **A results panel.** Nothing in SysON registers a workbench view, so this point is
  **unverified** against SysON code; Sirius Web's `sirius-web-application` exports a
  `workbenchViewContributionExtensionPoint` in its own sources (not read here). The safe design
  for phase 2 is what SysON does for import reports: a dialog opened by the menu entry
  (`NewObjectAsTextDocumentReport.tsx` renders the `messages` of the mutation payload), with
  the workbench view as the later upgrade.

**Build and packaging.** The frontend is one Vite bundle: `frontend/syson`'s `build` script is
`vite build && tsc`, CI copies `frontend/syson/dist/*` into
`backend/application/syson-frontend/src/main/resources/static`, and that jar serves it. There is
no runtime loading of a second bundle, so a plugin's React code has to be compiled into the SysON
frontend. Two ways, both requiring a SysON build: publish an npm package (SysON publishes its own
to `npm.pkg.github.com`, `publishConfig` in `frontend/syson/package.json`) and add it plus one
`addComponent`/`putData` call to a fork of `index.tsx`; or vendor the components into a fork of
`frontend/syson-components`. Either way the artifact a SysON deployment consumes is a rebuilt
`syson-frontend` jar (or a rebuilt image). This is the single largest cost of the plugin and is
called out in §5.

### 2.3 Textual export

The exporter is `org.eclipse.syson.sysml.export.SysMLv2DocumentExporter`
(`backend/application/syson-sysml-export`), an `IDocumentExporter`:

```java
public boolean canHandle(Resource resource, String mediaType) {
    // MediaType.TEXT_HTML, and the resource's first root is an Element
}
public Optional<byte[]> getBytes(Resource resource, String mediaType) {
    // installs a MembershipCacheAdapter, then
    new SysMLElementSerializer(SYSML_TEXTUAL_FORMAT_OPTIONS, status::add).doSwitch(element)
}
```

It serializes **one document (EMF `Resource`) at a time**, from its first root element down.
The serializer itself, `org.eclipse.syson.sysml.metamodel.services.textual.SysMLElementSerializer`
in `backend/services/syson-sysml-metamodel-services`, is an EMF switch (3,258 lines, 82 `case`
methods) that can start at any `Element`, so a plugin can export a subtree by calling it on the
selected element directly — the exporter only wraps it for whole documents. Names are
deresolved to the shortest unambiguous form by a `FileNameDeresolver`.

Over HTTP the exporter is reached through Sirius Web's document download,
`GET /api/editingcontexts/{editingContextId}/documents/{documentId}` with `Accept: text/html`
(`doc/content/modules/developer-guide/examples/download_sysml_file.py`; the recipe notes that
`text/html` is "the media type currently used by SysON to select the textual SysML exporter").
The explorer's **Download** entry on a document is the same call.

**Known gaps**, from the `Status.warning` messages the serializer emits: `BodyExpression`,
`CollectExpression`, `ConditionalExpression`, `ExtentExpression`, `FunctionOperationExpression`,
`IndexExpression`, `MetadataAccessExpression`, `SelectExpression`, `NamedArgumentList`, some
`TransitionFeatureMembership` kinds, a `SuccessionAsUsage` with an invalid number of ends, and a
generic "{0} are not yet handled" for anything else the switch does not cover. Unresolved
proxies produce "Found one proxy". The warnings are only logged (`Status.log(LOGGER)`) — the
download response carries no report, so a plugin that wants the gaps must call the serializer
itself with its own `Consumer<Status>`. Cross-document references are written as the deresolved
name, so a project of several documents exports to several files that must be parsed together,
which `Connection.parseSources` does.

### 2.4 Textual import: the parser

SysON does not parse SysML v2 in Java and uses no ANTLR grammar. It embeds the
[SysIDE](https://github.com/sensmetry/sysml-2ls) CLI as a JavaScript bundle,
`backend/application/syson-sysml-import/src/main/resources/syside-cli.js` (`version: "0.9.0"` in
the bundle; `CHANGELOG.adoc` records the move to 0.9.0), and runs it as a child process with
Node:

```java
// backend/application/syson-sysml-import/src/main/java/org/eclipse/syson/sysml/SysmlToAst.java
final String[] args = { "node", sysIdeInputPath.toString(), "dump", sysmlInputPath.toString() };
```

with a 60-second timeout. The JSON AST it prints is turned into EMF by
`org.eclipse.syson.sysml.ASTTransformer` (`ASTTransformer.convertToElements`). The SysML v2
specification revision SysIDE 0.9.0 implements was not checked (**unverified**); it is a
Langium grammar, which is why a SysON deployment needs `node` on the path.

Two entry points wrap it, and both are where a pre-import check sits:

1. **Uploading a `.sysml`/`.kerml` file** goes through Sirius Web's `uploadDocument` mutation to
   `SysMLExternalResourceLoaderService` (`IExternalResourceLoaderService.canHandle` on the
   extension, then `SysmlToAst.convert` and the transformer), which returns a `LoadingReport`.
2. **"New objects from text"** on an element is SysON's own mutation `insertTextualSysMLv2`
   (`schema/syson-import.graphqls`) handled by `InsertTextualSysMLv2EventHandler`, which calls
   `ISysMLTextImporter.importSysMLText(editingContext, parent, text, messages)`; the one
   implementation is `SysideSysMLTextImporter`, and the `messages` list is what the frontend
   dialog shows.

Both are ordinary Spring beans found by type. A plugin cannot wrap them from outside, but it can
**replace** them: `ISysMLTextImporter` is a single-bean dependency, so a `@Primary` bean (or a
`@ConditionalOnMissingBean` arrangement in a fork) that first runs OpenSysML's `ParseSources`
over the text and prepends its diagnostics as `Message`s before delegating to the SysIDE importer
gives the check on the "new objects from text" path with no SysON change. The upload path is a
list of `IExternalResourceLoaderService`s that Sirius iterates in bean order, so the same trick
needs the plugin's loader to be ordered first and to fall through when it declines; **unverified**
how Sirius orders them.

### 2.5 The standard SysML v2 REST API

SysON serves a partial implementation, and says so:
`doc/content/modules/developer-guide/pages/api/api.adoc` opens with "The SysML v2 API isn't
fully available yet". The implementation is Sirius Web's generic REST layer under
`/api/rest/` (`api-details.adoc`; Swagger at `/swagger-ui/index.html`) with SysON delegates in
`backend/services/syson-sysml-rest-api-services`:

- `SysONObjectRestService` (`IObjectRestServiceDelegate`) — element lookup, and the project's
  elements **excluding** standard-library resources and implied relationships;
- `SysONProjectDataVersioningRestService` (`IProjectDataVersioningRestServiceDelegate`) —
  commits and branches delegated to Sirius; `changes` synthesized from the element list, each
  change id `UUID.nameUUIDFromBytes(commitId + elementId)`;
- `SysMLv2JsonSerializer` registered by `SysMLv2SerializerConfig` — JSON shape `@id`
  (= `Element.getElementId()`), `@type` (the metaclass name), then attributes, then references
  as `{"@id": …}`; arrays of references **omit standard-library elements** and virtual links,
  by comment "to avoid huge responses".

Paths follow the standard (`/api/rest/projects`, `/projects/{p}/commits`,
`/projects/{p}/commits/{c}/elements[/{e}]`, `/projects/{p}/branches`, per the cookbook scripts
under `doc/content/modules/developer-guide/examples/`). Each project has exactly one branch and
one commit, and "creating additional commits is not functional" (`api-details.adoc`) — the
commit id is stable while the content changes underneath it, so there is no history to diff
against. **Authentication:** none; the application property file only sets CORS
(`sirius.components.cors.*` in `application-dev.properties`) and no Spring Security filter chain
exists in the SysON sources. **Unverified**: whether the hosted demo or a Sirius Web profile adds
one.

**Normative UUIDs — yes, and they match OpenSysML.** SysON's bundled libraries
(`backend/application/syson-application-configuration/src/main/resources/{kerml,sysml}.libraries/*.json`)
carry two ids per element: the Sirius object `id` (random, copied per project by
`StandardLibrariesLoader`'s `SysONCopier`) and the SysML `elementId` attribute, which is what the
REST API serves as `@id`. For every element sampled, `elementId` equals what
`internal/semantic/identity/normative.go` (`identity.ElementID`) derives — the RFC 4122 v5 scheme
of the OMG pilot (`uuid5(NAMESPACE_URL, prefix + package)`, then `uuid5(package, qualified name)`):

| element | SysON `elementId` | OpenSysML `identity.ElementID` |
|---|---|---|
| `Base` | `cdd5d1e3-fe4b-52bd-8a01-51a53f22ba47` | same |
| `Base::Anything` | `d5b4e7df-e644-5f2f-b95e-cf6f1f6c076d` | same |
| `Base::things` | `3176ab6a-8d7b-5e14-b263-57bd30f77f78` | same |
| `Base::DataValue` | `eeef8c48-5018-5f4f-b1d7-a199b05d86ed` | same |
| `ScalarValues::Real` | `14c0aa22-5489-59b5-b438-ded26e83ba31` | same |
| `ISQBase::MassValue` | `9cd0e404-efee-50e5-a59b-681065bd188c` | same |

The cookbook's "IDs are randomized and may differ between runs" (`api-cookbook.adoc`) is about
Sirius object ids and user elements, not library `elementId`s. User elements get a random
`elementId` when created in SysON, and the identity table's `SourceDeclared` (an `@ElementId`
annotation) is how OpenSysML would carry one through text (§2.6).

### 2.6 Element identity: `Symbol.id` and EMF elements

On the wire an OpenSysML symbol is identified by its **fully qualified name**:
`api/proto/sysml.proto` `SymbolInfo.id` is documented "Unique identifier (fully qualified name)",
`internal/frontend/grpc/convert.go` sets `Id: idx.GetFQN(sym)`, and `Symbol.id` in the Java
client is that string. A SysON element has three handles: the Sirius object id (`IIdentityService.getId`,
what tree items and GraphQL use), `Element.getElementId()` (the SysML id, what REST serves)
and `Element.getQualifiedName()` (derived, `::`-separated). The SysML metamodel's
`qualifiedName` and `Symbol.id` are the same notion, so the mapping is:

- **SysON → OpenSysML:** export the document(s) to text (§2.3), parse them as one model, and
  address the selected element by `element.getQualifiedName()`. Anonymous elements have no
  qualified name; both sides derive one from position (OpenSysML's `Root`/`FQN` for unnamed
  symbols, SysON's `ElementUtil`), and the two derivations are not guaranteed to agree —
  **unverified**, and a test of the plan in §4.1.
- **OpenSysML → SysON:** a diagnostic or verdict names a qualified name; the plugin resolves it
  in the editing context's resource set (walk `Namespace.getMember()` by segment, or keep the
  `Map<String, Element>` it built while exporting) and then asks `IIdentityService.getId(element)`
  for the id the frontend needs to select or decorate it.
- **Stable identity across edits:** the plugin can write `@ElementId { id = "<elementId>" }`
  annotations while exporting, which OpenSysML reads into `identity.Info.DeclaredID`
  (`internal/semantic/identity/identity.go`), so results keyed by UUID survive a rename. The
  REPL's `-sync-diff`/`-sync-apply` (`docs/reference/cli.md`) already speaks the SysML v2 API
  keyed by effective id, which is what phase 4 builds on.

The text OpenSysML sees also has to contain the standard library. It does, from its own bundle
(`internal/workspace/libs`), and because both sides mint the same normative ids (§2.5) a library
reference in a result maps to the SysON library element without a lookup table.

### 2.7 Validation markers

Sirius Web's validation is pull-based: `IValidationService.validate(IEditingContext)` returns
the diagnostics the **Validation** view shows, and SysON's `SysONSysMLValidationService` runs an
EMF `Diagnostician` over every non-library resource with the `EValidator.Registry`
(`SysMLValidatorRegistrationConfiguration` registers `SysONSysMLValidator` and its `rules`). Two
ways for a plugin to surface OpenSysML diagnostics:

1. **As validation results** — a second `IValidationService` bean (**unverified** that Sirius
   collects several rather than requiring one; if one, the plugin's service wraps SysON's) that
   returns the last OpenSysML run's diagnostics for the editing context as EMF `Diagnostic`s
   whose `data` holds the EObject, mapped from the qualified name (§2.6). Sirius renders
   validation entries per object, and the diagram layer decorates nodes whose semantic element
   has a diagnostic — **unverified** for SysON's own node descriptions; the explorer does not
   decorate.
2. **As mutation messages** — the `IPayload` of the run mutation carries `messages` with
   `MessageLevel`, which the frontend shows in the dialog (the import path does exactly this).
   Immediate, but not attached to elements.

Phase 2 uses both: messages for the run that just happened, a validation service holding the
run's diagnostics until the next run so the Validation view and diagrams show them. Neither is a
persistent marker; the diagnostics live in the plugin's memory per editing context.

### 2.8 Sample models, building SysON, and the checks

**Bundled examples.** SysON ships one example project template, **Batmobile**
(`SysMLv2ProjectTemplatesProvider.BATMOBILE_TEMPLATE_NAME`), loaded from
`backend/application/syson-application-configuration/src/main/resources/templates/Batmobile.json`
by `SysONDefaultResourceProvider.loadBatmobileResource`; the same directory holds
`Batmobile.sysml`, the textual form. The other two templates ("SysMLv2" and "SysMLv2 library")
are empty. There are no further bundled example projects; the textual fixtures are the twenty
`.sysml` files under `backend/application/syson-sysml-import/src/test/resources/ASTTransformerTest/`
and the two under `integration-tests-playwright/playwright/resources/`.

**Building SysON on this machine did not work**, so the exports SysON would produce were not
regenerated and the checks below ran on the textual files in the repository. The machine has
Temurin-compatible OpenJDK 21.0.12, Maven 3.6.3 and Node 24.19.0. The build steps CI uses
(`.github/workflows/build.yml`) are:

```bash
npm ci && npm run build                                             # needs .npmrc token for npm.pkg.github.com
cp -R frontend/syson/dist/* backend/application/syson-frontend/src/main/resources/static
mvn -U -B -e clean verify --settings settings.xml                   # needs USERNAME/PASSWORD for maven.pkg.github.com
```

Two things stop them without credentials: `@eclipse-sirius/*` npm packages and
`org.eclipse.sirius:*` Maven artifacts are on GitHub Packages, which answered `401` to an
anonymous `GET` of `sirius-web-starter-2026.9.0.pom` and of `@eclipse-sirius/sirius-components-core`;
and Maven Central rate-limited this machine (`429` on `spring-boot-starter-parent-4.1.0.pom`,
after which `mvn -pl backend/application/syson-frontend validate --settings settings.xml`
stalled on that download and was stopped after five minutes). A SysON build is feasible with a
GitHub token in `~/.m2/settings.xml` and `.npmrc`, and is the first task of phase 1.

**Checks.** OpenSysML was built with `make build` (`bin/sysml`, `bin/sysml-lsp`, `bin/sysml-grpc`
at `v0.8.1-1275-g301cba525`), and every file above was checked with `bin/sysml -validate <file>`
(`-validate` is the REPL's check-and-exit flag; the exit code is 0 clean, 2 on an error).

| file | result |
|---|---|
| `templates/Batmobile.sysml` | 1 error: `254:16 unresolved reference: Dont_Panic_Batmobile` — the view `batmobileParts` exposes `Dont_Panic_Batmobile::**` but the package is `Batmobile`; a stale name in the template |
| `ASTTransformerTest/convertAliasTest/alias.sysml` | clean |
| `convertAllocationTest/allocation.sysml` | 1 error `Must have at least two related elements` (an `allocation` with a single `part source;`), 1 warning (`source` duplicates an inherited member) — the fixture is a deliberately minimal allocation |
| `convertAssignmentTest/assignment{1,2,3}.sysml` | clean |
| `convertBooleanTest/boolean.sysml` | 2 errors: `26:42 Initialized feature must be variable` (`attribute init1 … := 0`), `53:2 Only a variable feature can be constant` (`constant attribute ro;`) — the fixture exists to exercise every boolean flag of the metamodel, not to be well-formed |
| `convertFeatureTypingTest/featureTyping.sysml` | clean |
| `convertImportTest/import.sysml` | clean |
| `convertInheritanceTest/inheritance.sysml` | clean |
| `convertNamespaceImportTest/{model,namespace}.sysml` | clean, each alone |
| `convertNamespaceImportValueTest/model.sysml` | 2 errors alone (its imports live in the sibling file); **clean** when the directory is loaded as one model |
| `convertNamespaceImportValueTest/namespace.sysml` | clean |
| `convertProjectWithErrors/proxyResolutionError.sysml` | 1 error `unresolved reference: FakeType` — by design, the fixture tests SysON's error path |
| `convertRedefinesTest/redefines.sysml` | 2 syntax errors at `3:36`: `attribute packetSecondaryHeader' redefines …` has a stray `'` — the fixture is not valid notation |
| `convertSubclassificationTest/subclassification.sysml` | clean |
| `convertVisibilityTest/visibility.sysml` | clean |
| `isUniqueFeature/model.sysml` | clean |
| `playwright/resources/SysMLv2WithGeneralView.sysml` | clean |
| `playwright/resources/SysMLv2WithInterconnectionView.sysml` | clean |

Fourteen of twenty-one files check clean; every finding on the other seven is either a
deliberate error fixture or a real defect in the fixture (the Batmobile package name, the stray
quote), none is an OpenSysML parse failure on notation SysON accepts. What this does **not**
establish, because SysON was not built, is how OpenSysML fares on text SysON's serializer
produces from a project edited graphically — the `Status` gaps in §2.3 predict warnings on
models with the listed expression kinds. That is the first measurement of phase 1.

### 2.9 Known SysON export gaps

The textual export SysON produces from a project edited graphically — SysON's
`SysMLElementSerializer`, seen on the Batmobile template — used to drop pieces of
the model rather than serializing them, and OpenSysML reported each as a syntax
error where the gap landed. Four of those gaps are fixed on the Open-MBEE fork
of SysON (branch `integration/textual-export-fixes`, combining the fork's pull
requests 1–3 with `fix/succession-implicit-target`) and are pending upstream; a
SysON built from that branch exports:

- **Feature-chained connector ends** —
  `interface bat2eng : PowerInterface connect battery.powerPort to batmobileEngine.enginePort;`
  (previously `connection bat2eng : PowerInterface connect  to ;`).
- **A succession to an anonymous decision node** — `then decide;` (previously
  `then;` followed by `decide ;`).
- **A succession with an explicit `first` source and an implicit target** —
  `first start then startBatmobile;` followed by `action startBatmobile;`
  (previously the target action was inlined as the invalid
  `first start then action startBatmobile;`, which OpenSysML rejected with
  `expected ';' or '{' after initial node`).
- **A satisfy with no subject** — `assert satisfy 'system components';`
  (previously a dangling `assert satisfy 'system components' by;`).

With those in place a Batmobile export parses far enough that `instantiate`,
`validateInstance`, `verifyRequirement` and `verifySatisfaction` answer
`ok: true`. What still goes wrong is in the serializer, not the plugin or the
parser:

- **A succession to a non-action target is dropped.** `then timeslice charging`
  in `part bm1` exports as `timeslice charging` with the warning `Unable to
  export a SuccessionAsUsage (…) with an implicit target and no following
  action`; the implicit-target fix only inlines `ActionUsage` targets.
- **Redefinition through a feature chain is dropped.**
  `attribute :>> battery.capacity = 40000 [SI::'watt hour'];` exports as
  `attribute = 40000 [SI::'watt hour'];`, an anonymous attribute with no
  redefinition — valid text that no longer says what it meant.
- **`view def` exports as `part def`.** `SysMLElementSerializer` has no
  `caseViewDefinition`, so a `view def` falls to `casePartDefinition` and exports
  as `part def 'Part list' { … }` with its `filter @SysML::PartUsage;` dropped;
  the `view batmobileParts : 'Part list'` usage then fails OpenSysML's `A view
  must be typed by one view definition` check. Similarly
  `expose Dont_Panic_Batmobile::**;` exports as `expose Batmobile;`.
- **Unresolved library proxies** are reported as `Found one proxy
  kermllibrary:///…` warnings; the export itself is unaffected.

What the Batmobile run reports after the fork fixes, attributed:

- **Batmobile model.** `actor driver : Batman;` and `stakeholder pm :
  ProductManagement;` are typed by `item def`s, which OpenSysML rejects as kind
  mismatches (actors and stakeholders are part usages). `'Drive Batmobile'`
  execution stops at the decision guard because `scanEnvironment.status` is
  never given a value.
- **OpenSysML runtime.** `ActivateRocketBooster :> 'Activate rocket booster'`
  inherits the `result` return parameter of the use case def it specializes,
  and `ActionExecutor.checkResultParameters`
  (`internal/exec/runtime/action_subflow.go`) rejects inherited as well as
  declared return parameters: `action ActivateRocketBooster declares 'return
  result'; write 'out result'`. Not fixed here.

Verdicts on standard-library constraints (`ShapeItems`, `Geometry`) that
`validateInstance` reports carry no SysON element: the library is not part of
the project, so there is nothing to select.

## 3. Architecture of `editors/syson/`

### 3.1 Layout

```
editors/syson/
  README.md
  pom.xml
  syson-api-stubs/               compile-only Sirius Web and SysON API classes
  backend/                       org.openmbee:opensysml-syson
    src/main/java/org/openmbee/opensysml/syson/
      OpenSysMLAutoConfiguration.java
      run/RunWithOpenSysMLService.java
      export/ProjectTextExporter.java
      identity/ElementIndex.java
      run/RunWithOpenSysMLInput.java
      run/MutationRunWithOpenSysMLDataFetcher.java
      run/RunResultStore.java
      menu/OpenSysMLTreeItemPaletteCustomizer.java
      validation/OpenSysMLValidationService.java
    src/main/resources/schema/opensysml.graphqls
    src/test/java/...
  frontend/                      @openmbee/opensysml-syson
    src/extension/RunWithOpenSysMLMenuContribution.tsx
    src/dialog/RunWithOpenSysMLDialog.tsx
    src/results/RunResultsPanel.tsx
    src/registry/opensysmlExtensionRegistry.ts
  distribution/                  (not in this phase)
```

The backend module depends on `org.openmbee:opensysml-client:0.1.0-SNAPSHOT` (the Java client,
§2.1 for why it is JDK-compatible), `org.eclipse.sirius:sirius-web-starter`,
`org.eclipse.syson:syson-sysml-metamodel` and `syson-sysml-metamodel-services`, all `2026.9.0`,
the last three `provided` because the SysON application already ships them. It carries no
`sysml-grpc` binary: the client starts one from `OPENSYSML_GRPC_BINARY`, or downloads the pinned
release, or connects to `OPENSYSML_SERVICE` (`ConnectionOptions`, `docs/reference/java-api.md`),
and the configuration bean exposes the same three choices as `opensysml.*` Spring properties. A
deployment with one SysON JVM gets one child `sysml-grpc` per classloader, which is one.

### 3.2 "Run with OpenSysML": the sequence

1. The user right-clicks an element in the explorer. `OpenSysMLTreeItemPaletteCustomizer`
   (an `ITreeItemPaletteCustomizer` whose `canHandle` matches SysON's explorer description id
   and an `Element` that is not from a library) appends `SingleClickTreeItemTool("runWithOpenSysML", "Run with OpenSysML…")`.
2. The frontend's `TreeItemContextMenuOverrideContribution` for that id renders the menu item;
   choosing it opens `RunWithOpenSysMLDialog`, which offers the operations the selected element's
   metaclass admits — `instantiate` for a definition or usage, `executeAction`/`exploreAction`
   for an `ActionDefinition`/`ActionUsage`, `executeState`/`exploreState` for a state, `verifyConstraint`,
   `verifyRequirement`, `verifySatisfaction`, `evaluateCalc`, `runAnalysis` for the
   corresponding kinds, and `validate` for anything — and their arguments (initial values, step
   budget, schedule — `ExecutionOptions` has `schedule` and `performer`).
3. The dialog sends `mutation runWithOpenSysML(input: { id, editingContextId, objectId,
   operation, arguments })`. `MutationRunWithOpenSysMLDataFetcher` converts it to the
   `RunWithOpenSysMLInput` record and dispatches it to the editing context, which serializes it
   with every other edit of that project (Sirius runs one input at a time per editing context).
4. `RunWithOpenSysMLEventHandler`:
   1. resolves `objectId` with `IObjectSearchService` to an `Element`;
   2. `ProjectTextExporter` walks the resource set, skips library resources
      (`ElementUtil.isStandardLibraryResource`) and referenced libraries, serializes each
      remaining resource with `SysMLElementSerializer` and a `Consumer<Status>` that collects
      the gaps of §2.3 as warnings, and records for each named `Element` its qualified name
      → (element, Sirius id) in an `ElementIndex`;
   3. `connection.parseSources(documents)` — one `Model`, its parse cache keyed by content so an
      unchanged project is not re-parsed; the model's `diagnostics()` become the first messages;
   4. the operation: `model.instantiate(qn)`, `model.executeAction(qn, options)`, …, each a
      one-line call on `Model`;
   5. maps the result: a `Diagnostic` with a `Span` is located by re-reading the exporter's
      line map (it knows which element it wrote at which line), one without a span by the
      qualified name in its message when there is one; an `Instantiation`'s root and reachable
      instances, an `ActionRun`/`StateRun` (outputs, trace), an `Exploration`, a `Verification`, a `Value`, are rendered as
      `messages` plus a JSON `result` field of the payload, and stored in `RunResultStore` for
      the editing context;
   6. emits `RunWithOpenSysMLSuccessPayload(id, messages, result)` and a `ChangeDescription` of
      kind `NOTHING` — a run does not change the model — unless the user asked to record the
      minted `@ElementId`s (§2.6), which is a `SEMANTIC_CHANGE`.
5. The dialog renders the payload: messages by level, the verdict, the instance tree or trace as
   a table. `OpenSysMLValidationService.validate(editingContext)` returns the stored
   diagnostics as EMF `Diagnostic`s whose `data` is the `Element` from `ElementIndex`, so the
   Validation view and any diagram decoration show them until the next run replaces them.

### 3.3 Mapping results back

| OpenSysML | SysON |
|---|---|
| `Diagnostic{severity, message, span}` | `Message(text, MessageLevel)` in the payload; EMF `Diagnostic{severity, source="opensysml", data=[element]}` in the validation service |
| `Symbol.id` (qualified name) | `ElementIndex.element(qn)` → `Element`; `IIdentityService.getId(element)` for the frontend |
| `Instantiation.root`/`reachable`, `Value.InstanceReference` | a tree in the dialog; each instance's type qualified name links to the SysON element |
| `Verification` (`verdict`, `verifications`, `instances`, `diagnostics`) | verdict as a message; a decided `false` is a message, an undecided verdict's `error()` is an error |
| `ActionRun`/`StateRun` outputs and trace, `Exploration` runs and choice points | tables in the dialog; step subjects link by qualified name |
| library element in any of the above | the library `Element` by normative `elementId`, no export needed (§2.5) |

Nothing is written into the SysON model by phases 1–3 except, on request, `@ElementId`
annotations; results are transient per editing context.

## 4. Phased plan

### 4.1 Backend adapter

Deliverable: `editors/syson/backend` builds against SysON `2026.9.0`, and a SysON built with it
answers a `runWithOpenSysML` GraphQL mutation for every operation in §3.2 step 2, tested with
Spring tests over SysON's own fixtures.

1. Obtain a GitHub token for Maven and npm and build SysON `v2026.9.0` unmodified (§2.8); record
   the exact commands and time in `editors/syson/README.md`.
2. Export the Batmobile template and a project of each `ASTTransformerTest` fixture through
   `SysMLElementSerializer`, check each export with `bin/sysml -validate`, and record the
   results beside §2.8's table — this measures the exporter gaps and the anonymous-name
   agreement (§2.6) before any code depends on them.
3. `OpenSysMLConfiguration`, `ProjectTextExporter`, `ElementIndex`, the input/handler/fetcher and
   `RunResultStore`; the GraphQL schema; the palette customizer (the entry appears with the
   default rendering, a no-op click, until phase 2).
4. Tests: a Spring Boot test in the module that boots SysON's test application with the plugin
   jar, creates a project from the Batmobile template, runs `instantiate` on a part and
   `verifyRequirement` on a requirement, and asserts the payload.

### 4.2 Execution UI

Deliverable: the menu entry, the dialog, and results in the Validation view.

1. `editors/syson/frontend` as an npm package with the override contribution and the dialog;
   the `distribution/` fork adds the two dependency lines and merges the registry.
2. `OpenSysMLValidationService`; verify which SysON diagram descriptions decorate elements with
   diagnostics (§2.7 unverified) and, if none, add the decoration to the dialog only.
3. A workbench results view replaces the dialog's result section if
   `workbenchViewContributionExtensionPoint` proves usable (§2.2 unverified).

### 4.3 Parser check on import

Deliverable: OpenSysML diagnostics on text a user pastes or uploads, before SysIDE converts it.

1. `CheckingSysMLTextImporter` as `@Primary ISysMLTextImporter`: `connection.parse(text)`
   (with the rest of the project as context through `parseSources`), prepend diagnostics as
   messages, then delegate; an `opensysml.import.block-on-error` property decides whether an
   error stops the import.
2. The same in front of `SysMLExternalResourceLoaderService` for uploads, once the ordering of
   `IExternalResourceLoaderService`s is established (§2.4 unverified); otherwise a fork of that
   class in `distribution/`.
3. Compare the two parsers on the fixtures of §2.8 and record where they disagree.

### 4.4 Structural sync over the SysML v2 API

Deliverable: OpenSysML reads and writes a SysON project through `/api/rest/` instead of text,
keyed by `elementId`.

1. Point `sysml -sync-diff <syson-url>/api/rest/projects/<p>` at a SysON project
   (`internal/translate/interop/reposync`); record what the partial API (§2.5: one commit, arrays
   without library elements, `@type` naming) breaks in the diff and fix it on the OpenSysML side
   where it is a client assumption, or document it as a SysON gap.
2. Mint `@ElementId`s for user elements on export so text and repository identity agree.
3. Track SysON's API completion (commits, branches) on its `main` and re-run; the release pinned
   here does not support it.

## 5. Unknowns and risks

**Unverified, in the order they matter:**

1. How OpenSysML fares on text produced by `SysMLElementSerializer` from graphically edited
   projects — no SysON export was produced on this machine (§2.8).
2. Whether the qualified names SysON derives for anonymous elements agree with OpenSysML's, on
   which result mapping for unnamed elements depends (§2.6).
3. Whether `workbenchViewContributionExtensionPoint` in `sirius-web-application` `2026.9.0` is
   usable for a results view (§2.2); Sirius Web's own sources were not read.
4. Whether any SysON diagram description decorates nodes from `IValidationService` results (§2.7).
5. The order Sirius Web calls `IExternalResourceLoaderService` beans in (§2.4).
6. Whether a SysON Docker image's classpath can be extended without a rebuild (§2.1).
7. Which SysML v2 specification revision SysIDE 0.9.0 implements (§2.4).
8. Whether any SysON deployment profile adds authentication in front of `/api/rest/` (§2.5).

**Risks:**

- **Every deployment is a fork.** Backend jars need a POM line, the frontend needs a rebuilt
  bundle; there is no drop-in path. The `distribution/` module contains the fork, but each SysON
  release means re-applying it and re-pinning `sirius.web.version`; SysON releases roughly
  monthly (the tag is `v2026.9.0`; `CHANGELOG.adoc` lists a version per month).
- **Text is the interchange.** Until phase 4 the plugin round-trips the model through the
  serializer, whose known gaps (§2.3) are in expressions — exactly what constraints, calculations
  and requirements are made of. A model using `ConditionalExpression` or `SelectExpression`
  exports with a warning and a hole, and OpenSysML verifies the hole.
- **GitHub Packages credentials** are needed to build anything at all, including CI for
  `editors/syson/`; a token must be provisioned for the OpenSysML repository's CI.
- **Two parsers.** SysIDE (SysON) and OpenSysML will disagree on some notation; phase 3 makes
  the disagreement visible to the user, which is the point, but it needs a policy for whose
  verdict blocks an import.
- **Node on the backend host** is already required by SysON for SysIDE; the plugin adds a
  `sysml-grpc` binary. Both are process spawns from the JVM, which some deployments forbid; the
  client's `OPENSYSML_SERVICE` mode (a service run elsewhere) is the answer, and the plugin's
  configuration must make it the documented production mode.
- **Long-running RPCs on the editing-context thread.** Sirius serializes inputs per editing
  context, so an `exploreAction` that takes a minute blocks that project's edits for a minute.
  The handler should run the RPC off the dispatcher thread and complete the payload
  asynchronously, or enforce the request timeout from `ConnectionOptions`; which the Sirius
  input contract permits is to be confirmed in phase 1.

## 6. Implementation notes

Where the code departs from §3–§4, and why.

- **Beans are registered by Spring Boot auto-configuration, not by package scan.** The jar's
  classes live under `org.openmbee.opensysml.syson` and are listed in
  `META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports`, so a
  deployment adds the jar and nothing else; putting them under `org.eclipse.syson` (§2.1) would
  have made a foreign jar claim SysON's namespace.
- **Compile-only stubs stand in for the SysON and Sirius Web artifacts by default.** Both are
  published only to GitHub Packages, which needs a token even to read. `syson-api-stubs/` holds
  the exact classes, signatures and constants the backend uses, taken from the `2026.9.0` sources;
  the default `stubs` Maven profile compiles and tests against them, and `-Psyson-artifacts`
  swaps in the real `provided` dependencies through `settings.xml`. The frontend does the same
  with ambient type declarations for `@eclipse-sirius/sirius-components-core` and `-trees`,
  runtime doubles used only by its tests, and `npm run install:syson` / `build:syson` for the
  real packages. The mandatory CI job runs the stub path; `syson-artifacts.yml` runs the real one
  on demand with a `read:packages` token.
- **The palette customizer runs last.** Sirius applies `ITreeItemPaletteCustomizer`s in bean
  order and SysON's own customizer rebuilds the palette instead of extending it, so the
  OpenSysML customizer is ordered `LOWEST_PRECEDENCE` and appends to whatever it receives.
- **The frontend appends to SysON's contribution rather than merging registries.** SysON's
  registry already `putData`s at `treeItem#contextMenuEntryOverride`, and `putData` replaces;
  `addOpenSysMLContributions(registry)` reads the existing entry and re-puts it with the
  OpenSysML contribution appended.
- **Budgets are schedule strings.** The Java client exposes no separate budget fields;
  `opensysml.explore.runs` and `opensysml.explore.depth` are encoded into the exploration schedule
  (`explore:runs=N,depth=D`) that the explore operations send.
- **Results stay in the dialog.** The workbench view extension point (§4.2 step 3) was not
  exercised; the dialog renders outcomes, traces, verdicts, instances and diagnostics, and a
  diagnostic click selects its element. Diagnostics also reach the Validation view through
  `OpenSysMLValidationService`, which serves the last run per editing context.
- **Diagnostic selection:** diagnostics naming a qualified name select that element; span-only
  diagnostics select the root element of the exported document containing the line. The
  serializer emits no source map.
- **Deferred:** building SysON itself and the Batmobile export survey (§4.1 steps 1–2), the
  Spring Boot test over SysON's test application (§4.1 step 4, replaced by unit tests over a
  stubbed export plus integration tests against a real `sysml-grpc`), and phases 3–4.

# A Cameo Systems Modeler plugin: OpenSysML as the execution engine

**Date:** 2026-09-20
**Status:** Discovery and design — nothing under `editors/cameo/` exists yet
**Scope:** a future `editors/cameo/` plugin; the Java client (`client/java/opensysml-client`);
`Convert` from XMI (`internal/translate/xmi`, `internal/translate/migrate`); the results a
Cameo user sees on their own elements

---

## 1. What this is

Cameo Systems Modeler (and MagicDraw, Magic Cyber Systems Engineer and Magic Systems of Systems
Architect, which share one platform and one OpenAPI) is where a large share of SysML v1 models
live. OpenSysML already reads those models — OMG UML XMI 2.5 with the SysML profile, and a
MagicDraw/Cameo `.mdzip` opened in place — and migrates them to SysML v2 notation
([`docs/reference/sysml-v1-migration.md`](../../reference/sysml-v1-migration.md)); it runs,
verifies and analyzes the result over `sysml-grpc`, and the Java client wraps every RPC
([`docs/reference/java-api.md`](../../reference/java-api.md)). The missing piece is the plugin
that lets a Cameo user select a package, choose *Run with OpenSysML*, and read the verdicts on
their own elements.

This note answers the discovery questions a plugin design depends on, from the vendor's public
documentation and Javadoc and from a migration benchmark run over the models that are publicly
available, and then proposes the plugin. Every external claim carries the URL it was read from.
Claims that could not be confirmed from a public page are marked **unverified**. No Cameo
installation was available: nothing here was run against the tool.

### 1.1 The release this targets

The vendor's current documentation site describes **2026x Refresh1** as the latest release of
every CATIA Magic / No Magic product, released on June 26, 2026
([version news](https://docs.nomagic.com/VN/latest/2026x-refresh1-version-news-314179593.html)).
The last release of the 2024x line is **2024x Refresh3**
([2024x Refresh3 version news](https://docs.nomagic.com/spaces/CSM2024xR3/pages/239796953/2024x+Refresh3+Version+News)).
The two differ in the JDK they ship:

| Release | Bundled / recommended JDK | Source |
|---|---|---|
| 2026x Refresh1 | Eclipse Temurin (AdoptOpenJDK) **21.0.10+7**, HotSpot, all OSs | [Java version support, 2026x Refresh1](https://docs.nomagic.com/IL/latest/java-version-support-304005280.html) |
| 2026x | Temurin 21.0.8+9 HotSpot, all OSs | [Java version support, 2026x](https://docs.nomagic.com/IL/2026x/java-version-support-272740412.html) |
| 2024x Refresh3 | Temurin **17.0.14** HotSpot | [Java version support for 2024x Refresh3](https://docs.nomagic.com/spaces/IL2024xR3/pages/249579845/Java+version+support+for+2024x+Refresh3) |

**Pin:** the plugin targets **Cameo Systems Modeler 2024x Refresh3** with its bundled
**Eclipse Temurin 17.0.14 HotSpot** — the last release of the 2024x line, and the JDK the
OpenSysML Java client already requires ([`docs/reference/java-api.md`](../../reference/java-api.md)).
The plugin is compiled with `--release 17` so that the same jar also loads in **2026x /
2026x Refresh1** (JDK 21); nothing in it may need a class that only exists in 2026x. Every
OpenAPI class the v1 path relies on (§2–§6) is cited from the 2024x Refresh3 Javadoc
(`https://jdocs.nomagic.com/2024xRefresh3/`) or documentation where that page was read; a
2026x Refresh1 citation stands in only where the 2024x Refresh3 page was not read, and is then
evidence for the 2026x line, not proof of 2024x behavior. The class list of the 2026x Refresh1
Javadoc index (`https://jdocs.nomagic.com/2026xRefresh1/allclasses-index.html`) was the source
for "no such OpenAPI class exists" statements below; the 2024x Refresh3 index was not searched
the same way, so each such statement is **unverified for 2024x Refresh3**.

## 2. Plugin mechanics

### 2.1 Descriptor: `plugin.xml`

A plugin is a directory under the tool's `plugins/` folder holding a `plugin.xml` descriptor,
its jar(s) and any libraries
([Plugin descriptor, 2024x Refresh3](https://docs.nomagic.com/spaces/DEVG2024xR3/pages/225347166/Plugin+descriptor);
the 2024x Refresh2 page, [here](https://docs.nomagic.com/spaces/DEVG2024xR2/pages/191934885/Plugin%2Bdescriptor),
tabulates the attributes). The fields the design uses:

| Element / attribute | Meaning (from the descriptor page) |
|---|---|
| `plugin id`, `name`, `version`, `provider-name` | identity shown in the Resource/Plugin Manager |
| `plugin class` | the fully qualified `com.nomagic.magicdraw.plugins.Plugin` subclass loaded by the plugin manager |
| `plugin internalVersion` | integer compared when the same plugin is installed twice |
| `requires-api` | the OpenAPI version the plugin needs; `requires-api="1.0"` in the vendor examples |
| `requires` / `requires-plugin` | other plugins that must be loaded first (the SysML plugin, for a plugin that reads SysML stereotypes) |
| `runtime` / `library name="…jar"` | the jars the plugin's classloader sees |
| `ownClassloader="true"` | give this plugin its own classloader (default `false`) |
| `class-lookup="LocalFirst"` | with `ownClassloader`, prefer the plugin's copies of classes over the tool's |

`PluginDescriptor` exposes the same data at run time
([Javadoc, 2024x Refresh3](https://jdocs.nomagic.com/2024xRefresh3/com/nomagic/magicdraw/plugins/PluginDescriptor.html)):
notably `getPluginDirectory()`, which is how the plugin finds the native `sysml-grpc` binary it
ships (§2.4).

### 2.2 Lifecycle: `com.nomagic.magicdraw.plugins.Plugin`

The abstract class has three methods
([Plugin, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/plugins/Plugin.html);
[Plugin classes, 2024x Refresh3](https://docs.nomagic.com/spaces/DEVG2024xR3/pages/225347167/Plugin+classes)):

- `isSupported()` — called first; the plugin is initialized only if it returns `true`. The
  OpenSysML plugin returns `false` when no `sysml-grpc` binary for the host platform is
  available (§2.4), with a log line saying why, rather than failing later.
- `init()` — called at tool start-up; where action configurators, project windows and listeners
  are registered. It must **not** start the `sysml-grpc` child: the Java client starts one
  lazily on first `Connection` use, and an idle child at every Cameo start is what a user would
  notice. `init()` only wires UI.
- `close()` — called before exit; returning `false` vetoes exit. The plugin closes its
  `Connection`, which ends the child (§12), and returns `true`.

`ResourceDependentPlugin` is a second interface for plugins that own a profile the project
depends on
([Javadoc](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/plugins/ResourceDependentPlugin.html)).
The OpenSysML plugin writes nothing into the model (§6), so it does not implement it.

### 2.3 Class loading — the one-child-per-classloader consequence

> All modeling tool plugins (classes and runtime libraries) are loaded by the one classloader.
> If there are plugins that cannot be loaded by the same classloader … their descriptors should
> be defined to use own classloaders.
> — [Plugin class loading, 2026x](https://docs.nomagic.com/DEVG/2026x/plugin-class-loading-254437433.html)

So by default **every plugin shares one classloader**. The Java client starts *one private
`sysml-grpc` child per classloader* ([`docs/reference/java-api.md`](../../reference/java-api.md),
"The service binary"). Two consequences:

1. If two plugins each shade the OpenSysML client into the shared classloader, the second copy's
   classes collide with the first's — a plain Java problem, not an OpenSysML one. If they share
   one copy on the plugin classpath, they share one child and one parse cache.
2. The OpenSysML plugin should set `ownClassloader="true"` and `class-lookup="LocalFirst"`. Its
   dependencies (the client, its protobuf-JSON bodies) then cannot conflict with the tool's or
   other plugins' versions, and the classloader boundary makes the child's ownership explicit:
   one plugin, one classloader, one child, closed in `Plugin.close()`.

The tool's own `PluginUtils.getPlugins()` lists loaded plugins
([Javadoc](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/plugins/PluginUtils.html)),
which is how a future second OpenSysML-based plugin could find and reuse this one's connection
instead of starting a child of its own.

### 2.4 Native binaries per platform

Nothing in the vendor's plugin documentation addresses native executables; a plugin directory is
an ordinary directory, so the plugin can lay out binaries however it likes and locate them from
`PluginDescriptor.getPluginDirectory()`. The Java client resolves the service binary in this
order: `ConnectionOptions.binaryPath(...)`, then `$OPENSYSML_GRPC_BINARY`, then
`~/.opensysml/bin/sysml-grpc` (downloading a release pinned by version and verifying its signed
manifest when absent), then `$PATH`
([`docs/reference/java-api.md`](../../reference/java-api.md)). The release artifacts are named
`sysml-grpc-<goos>-<goarch>[.exe]` for `linux`, `darwin` and `windows`
(`client/java/opensysml-client/src/main/java/org/openmbee/opensysml/internal/ReleasePlatform.java`).

Design: the plugin ships **all** platform binaries under `<plugin dir>/bin/` and passes the one
for the host through `ConnectionOptions.binaryPath(...)`, falling back to the client's download
path only when `bin/` is absent (a "thin" build for users whose IT forbids bundled executables
but allows the signed download). Bundling all platforms keeps one Resource Manager `.zip`
(§2.5) for every OS; the vendor ships one `.zip` per resource, not per OS.

Two platform facts matter and are recorded as risks (§12): the Java client itself drops the
POSIX execute bit on filesystems that keep none ("Windows and some network filesystems keep no
POSIX mode", `BinaryDownloader.java`), and a zip extracted by the Resource Manager may not
preserve modes on macOS/Linux — the plugin must `chmod +x` (Java `Files.setPosixFilePermissions`)
before first launch. On macOS, an executable extracted from a downloaded zip carries the
quarantine attribute and Gatekeeper may refuse it unless it is notarized — **unverified** for
this specific path, as the vendor documents nothing about it; the release binary is
sigstore-signed but not Apple-notarized.

### 2.5 Distribution: Resource Manager `.zip` and descriptor

Plugins are distributed as a zip whose internal layout mirrors the tool's installation directory
(`plugins/<id>/plugin.xml`, `plugins/<id>/*.jar`) plus a resource-manager descriptor at
`data/resourcemanager/MDR_Plugin_<id>_<n>_descriptor.xml`; the Resource/Plugin Manager
(Help ▸ Resource/Plugin Manager) installs it from a local file, a network share or a web server,
and "supports zip archives only"
([How to distribute resources, 2024x Refresh1](https://docs.nomagic.com/display/DEVG2024xR1/How+to+distribute+resources);
[Creating required files and folders structure, 2024x Refresh2](https://docs.nomagic.com/display/DEVG2024xR2/Creating+required+files+and+folders+structure);
[Resource Manager, 2024x](https://docs.nomagic.com/display/MD2024x/Resource+Manager)).
The vendor also offers a Resource Builder wizard (Tools ▸ Development Tools ▸ Build Custom
Resource…) that assembles the same zip. The `editors/cameo/` build produces this zip directly
(§8.2), the same way `editors/vscode` produces a `.vsix`, and the nightly attaches it beside the
`.vsix` ([`docs/project/nightly.md`](../../project/nightly.md)).

## 3. UI contribution points

`ActionsProvider`, `ActionsConfiguratorsManager`, `BrowserContextAMConfigurator` and
`DiagramContextAMConfigurator` are all listed in the 2024x Refresh3 Javadoc index
(`https://jdocs.nomagic.com/2024xRefresh3/allclasses-index.html`); the remaining classes below
were confirmed in the 2026x Refresh1 Javadoc and the developer guide.

**Context menus.** Actions live in `ActionsManager`s configured by *configurators* registered
with `ActionsConfiguratorsManager` from `Plugin.init()`. Three interfaces matter
([Creating new actions, 2024x Refresh2](https://docs.nomagic.com/spaces/DEVG2024xR2/pages/191934931/Creating+new+actions);
[ActionsConfiguratorsManager, 2026x](https://jdocs.nomagic.com/2026x/com/nomagic/magicdraw/actions/ActionsConfiguratorsManager.html)):

- `BrowserContextAMConfigurator.configure(ActionsManager, Tree)` — the containment-tree
  shortcut menu; registered with `addContainmentBrowserContextConfigurator`. The `Tree` gives
  the selected nodes, so *Run with OpenSysML* is offered on a `Package`, `Class` (a Block), a
  `Behavior` or a `Constraint`/`Requirement`.
- `DiagramContextAMConfigurator.configure(ActionsManager, DiagramPresentationElement, PresentationElement[], PresentationElement)`
  — a diagram's shortcut menu with the selected symbols; registered per diagram type with
  `addDiagramContextConfigurator(String diagramType, …)`
  ([DiagramContextAMConfigurator, 2024x Refresh3](https://jdocs.nomagic.com/2024xRefresh3/com/nomagic/magicdraw/actions/DiagramContextAMConfigurator.html)).
- `AMConfigurator` — main menu and toolbars (`addMainMenuConfigurator`), for a *Tools ▸
  OpenSysML* menu with *Run…*, *Verify…*, *Sweep…* and *Show results*.

Actions are `MDAction`/`DefaultBrowserAction`/`DefaultDiagramAction` subclasses added to an
`MDActionsCategory`; the category, not the action, is what the configurator adds.

`ActionsProvider` is the other side of the same mechanism: "the singleton class used for
accessing actions in different parts (diagrams, browsers, main menu and etc.)", with
`getContainmentBrowserContextActions(BrowserTabTree)`,
`getDiagramContextActions(String diagramType, DiagramPresentationElement, PresentationElement[], PresentationElement)`
and `getDiagramShortcutActions(...)`
([ActionsProvider, 2024x Refresh3](https://jdocs.nomagic.com/2024xRefresh3/com/nomagic/magicdraw/actions/ActionsProvider.html)).
It *reads* the configured managers; a plugin *contributes* through the configurators above and
only needs `ActionsProvider` to invoke or inspect an existing action (for example, to run the
tool's own *Validate* after the results are in). The design registers configurators and does
not call `ActionsProvider` directly.

**Docking results panel.** `ProjectWindowsManager` (from
`Application.getInstance().getMainFrame().getProjectWindowsManager()`) adds a `ProjectWindow`
— a Swing component described by a `WindowComponentInfo` (id, name, icon, side, docking state)
— to the active project, and `ProjectWindowsManager.ConfiguratorRegistry.addConfigurator(...)`
from `init()` makes its docking state persist with the project
([ProjectWindowsManager, 2026x](https://jdocs.nomagic.com/2026x/com/nomagic/magicdraw/ui/ProjectWindowsManager.html);
[ProjectWindow, 2024x Refresh3](https://jdocs.nomagic.com/2024xRefresh3/com/nomagic/magicdraw/ui/ProjectWindow.html);
[ProjectWindowsConfigurator, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/ui/ProjectWindowsConfigurator.html)).
This is where the *OpenSysML Results* table (§6) lives, beside the tool's own Validation
Results window. `GUILog` (`Application.getInstance().getGUILog()`) is the message/notification
window for one-line status and hyperlinks
([GUILog](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/core/GUILog.html)).

**Progress and cancellation.** `ProgressStatusRunner.runWithProgressStatus(RunnableWithProgress, String description, boolean allowCancel, int millisToShow)`
runs a task with the tool's progress dialog; the runnable receives a `ProgressStatus` and is
expected to poll `isCancel()`
([ProgressStatusRunner, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/ui/ProgressStatusRunner.html);
[RunnableWithProgress, 2024x Refresh3](https://jdocs.nomagic.com/2024xRefresh3/com/nomagic/task/RunnableWithProgress.html)).
The RPCs are unary, so "cancel" means: stop waiting, discard the answer when it arrives, and —
for a run that will not return — close the `Connection`, which ends the child, and open a new
one for the next run. The client documents `ConnectionOptions` deadlines
([`docs/reference/java-api.md`](../../reference/java-api.md)); the plugin sets one per phase
(export, convert, parse, run) so a cancel is never more than one deadline away. A finer
cancel — a streaming or session RPC — is the surface-parity note's session API
([`api-surface-parity.md`](api-surface-parity.md)), not this plugin's to invent.

**Diagram highlighting (for step-debug later).** Two mechanisms exist. *Annotations*
(`com.nomagic.magicdraw.annotation.Annotation`, `AnnotationManager`) attach a severity, kind,
text and actions to a `BaseElement` or a `PresentationElement`; they are runtime-only ("not
stored in the project"), the manager "takes care of drawing decorations around symbols with
annotations", and the caller must `update()` after adding or removing them and remove them
afterwards ([Annotation, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/annotation/Annotation.html);
[AnnotationManager, 2024x](https://jdocs.nomagic.com/2024x/com/nomagic/magicdraw/annotation/AnnotationManager.html)).
A custom `AnnotationPainter` (`Annotation.addPainter`) can draw the decoration itself. This is
enough to mark "current state", "fired transition" and "failed constraint" on an open diagram
without touching the model. The second mechanism — the Simulation Toolkit's own animation of
active states and tokens — is not in the OpenAPI class index (its public classes are the
`SimulationProfile` stereotype constants and `SimulationManager`/`SimulationHelper`, which
drive *its* engine); reusing that animation for a foreign engine is **unverified** and assumed
unavailable.

## 4. Getting the model out: in-memory XMI, or `.mdzip`

### 4.1 What the OpenAPI offers

The user-facing exporters are File ▸ Export To ▸ *UML XMI 2.5 file*, *Eclipse UML2 (v2–v5)
XMI*, *MOF XMI*, *EMF Ecore* and *MagicDraw Native XML*
([Exporting UML models, 2024x Refresh2](https://docs.nomagic.com/spaces/MD2024xR2/pages/189139865/Exporting+UML+models)).
Of these, only the **Eclipse UML2** exporter is in the OpenAPI:
`BaseEmfUml2XmiPlugin.exportXMI(Project, String destinationDir[, ProgressStatus])` and
`exportModel(Project)` on the versioned `EmfUml2XmiPlugin` singletons
([BaseEmfUml2XmiPlugin, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/emfuml2xmi/BaseEmfUml2XmiPlugin.html);
[v4 EmfUml2XmiPlugin](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/emfuml2xmi/v4/EmfUml2XmiPlugin.html)).
It writes to a directory, not to memory, and it writes the Eclipse UML2 dialect — which
OpenSysML reads (Papyrus `.uml`), but with the SysML profile in the Eclipse namespace and the
whole project, not a selection. **No OpenAPI entry point for the "UML XMI 2.5 file" exporter was
found** in the 2026x Refresh1 class index — the `com.nomagic.magicdraw.export` package there
holds image export only, and `com.nomagic.persistence.XmiExporterDescription` describes a
format's version and required resources rather than performing an export. Treat "export
selected package as OMG XMI 2.5 without a dialog" as **unverified / not available**.

`ProjectsManager` (`Application.getInstance().getProjectsManager()`) offers what the native
format needs
([ProjectsManager, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/core/project/ProjectsManager.html)):

- `saveProject(ProjectDescriptor, boolean silent)` — saves the project to the descriptor's
  location; `ProjectDescriptorsFactory.createLocalProjectDescriptor(Project, File)` makes a
  descriptor for an arbitrary file
  ([ProjectDescriptorsFactory](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/core/project/ProjectDescriptorsFactory.html)).
- `exportModule(Project, Collection<Package> packages, String description, ProjectDescriptor)` —
  "Export local (not teamwork) module into given descriptor": a **subset of packages** written
  as a `.mdzip` of its own.

Whether `saveProject` to a *different* file re-points the open project at that file (as
"Save As" does) is **unverified**; `exportModule` does not have that problem and is the
primary route for a selection.

### 4.2 Decision: `.mdzip` on disk is the reliable route

OpenSysML opens a `.mdzip` in place and reads its `com.nomagic.magicdraw.uml_model.model`
entries as one document; profile-application and stereotype classification only trust the OMG
namespaces (`http://www.omg.org/spec/UML/…`, `…/SysML/…`) and Papyrus's
([`docs/reference/sysml-v1-migration.md`](../../reference/sysml-v1-migration.md)). The benchmark
(§9) settles the namespace question empirically: `DocGen.mdzip`, a Cameo project saved by the
tool, yields 810 mapped and 388 approximated entries out of 1907 — Blocks, value properties,
requirements and constraints are recognized — so **the native `.mdzip` carries SysML stereotype
applications in the OMG namespace**, as the vendor's save format is itself XMI with the standard
profiles. No in-memory step is needed, and the export the plugin performs is:

```
selection (packages)  → ProjectsManager.exportModule(project, packages, "OpenSysML run", tmp.mdzip)
whole project         → ProjectsManager.saveProject(createLocalProjectDescriptor(project, tmp.mdzip), true)
                        (or, when the project is already saved locally and clean, its own file)
tmp.mdzip             → Connection.convertFile(tmp.mdzip, "sysml")        (file_path over the wire)
```

`ConvertRequest.file_path` is read by the `sysml-grpc` child, which runs on the same machine as
Cameo, so a temporary file is enough. Inline `content` with `from_format: "xmi"` is also
accepted by the service (a conformance case parses `vehicle.xmi` inline), and is the route for
an XMI text a future exporter API produces — but a `.mdzip` is a zip and `content` is a string,
so the archive goes by path.

Two limitations carry over from the migration reference: elements of **used projects**
(modules) are not loaded, so references into them stay unmapped — the plugin should export the
*used* modules too and pass every file to `Convert` once the service accepts several sources
for one conversion (§11, phase 3); and Teamwork Cloud projects have no local `.mdzip`, so `saveProject`
to a local descriptor (or `exportModule`) is the only route for them.

## 5. Element identity: from a Cameo element to a v2 qualified name

**Cameo side.** Every `BaseElement` has `getID()`, the persistent element ID that is the
`xmi:id` in the saved project; `Element.getHumanName()` and `getQualifiedName()` on
`NamedElement` give the display and qualified names. (The Javadoc for `BaseElement.getID()` was
not fetched; that `getID()` is the persisted `xmi:id` is the vendor's long-standing contract and
is **verified only indirectly** here — the `.mdzip` fixtures under `tests/migrate/testdata/xmi`
and the benchmark archives carry `xmi:id` values in MagicDraw's `_18_0_…` form, and the report
records them as `id`.)

**OpenSysML side.** The migration report is the per-element accounting
(`internal/translate/migrate/report.go`): one `Entry` per source element with

| field | content |
|---|---|
| `id` | the source `xmi:id` |
| `kind` | the applied stereotype(s) or UML metaclass, e.g. `«Block» Class` |
| `name` | the source qualified name |
| `target` | the **v2 qualified name** the conversion wrote, when it wrote one |
| `verdict` | `mapped`, `approximated`, `unmapped`, `skipped` |
| `note` | why, for anything but `mapped` |

with `Report.Source`, an optional `Exporter` (read from `xmi:Documentation`), and `Count()`,
`Unreferenced()` and `Summary()` over the entries. **`id → target` is exactly the map the
plugin needs**: a verdict OpenSysML reports on `Demo::Vehicle::massLight` is looked up by
`target`, and the entry's `id` is the Cameo element to annotate. Elements the conversion
renamed (duplicate member names, reserved words — the migration reference lists the cases as
*approximated*) are still found this way, which is why the plugin must never reconstruct the
name itself.

**What the service returns today.** The report is written only by the CLI
(`-migration-report <file>`); `ConvertResponse` carries `content`, `diagnostics`, `experimental`
and its notice, and nothing per element
([`docs/reference/sysml-v1-migration.md`](../../reference/sysml-v1-migration.md), §Status;
`api/proto/sysml.proto`, `ConvertResponse`). The surface-parity note already lists "the XMI
migration report through `Convert`" as a stateless operation the wire lacks
([`api-surface-parity.md`](api-surface-parity.md)). Until it lands, the plugin has two interim
options, neither good enough for a release: run `bin/sysml … -migration-report` as a second
child (it is the same binary family the plugin ships), or match by name and accept that renamed
elements are lost. §11 makes the report-over-service change a prerequisite of the results UI.

## 6. Reporting results on the elements

Three vendor mechanisms, used together:

1. **Annotations** (§3) — a runtime `Annotation(severity, kind, text, target)` per verdict,
   `AnnotationManager.update(removed, added)`, and the tool draws the decoration on every
   diagram symbol of the element and in the browser. Severity is an `EnumerationLiteral` of the
   tool's severity enumeration (`Annotation.ERROR`/`WARNING`/`INFO` name the kinds); a failed
   `assert constraint` is an error, a requirement not satisfied a warning, a holding verdict an
   info that is off by default. Annotations carry `NMAction`s, so each decoration offers *Show
   in OpenSysML Results* and *Re-run*.
2. **The Validation Results window** — `ValidationHelper.openValidationWindow(ValidationRunData, String windowID, Collection<RuleViolationResult>)`
   "opens validation window and displays `RuleViolationResult` in it"
   ([ValidationHelper, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/validation/ValidationHelper.html);
   [ValidationRunData, 2026x](https://jdocs.nomagic.com/2026x/com/nomagic/magicdraw/validation/ValidationRunData.html)).
   A `RuleViolationResult` pairs an `Annotation` with the `Constraint` (a validation rule) it
   violates, so this route needs a rule element in the model — an *OpenSysML* validation suite
   profile with one rule per verdict kind (constraint failed, requirement unsatisfied, run
   error) shipped as a read-only module. It buys the tool's own grouping, filtering, and
   "select in browser/diagram" for free. (`com.nomagic.reportwizard.tools.validation.ValidationResult`
   is the Report Wizard's object, not this one.)
3. **A custom results panel** (§3) — the `ProjectWindow` table with columns the validation
   window lacks: engine (`run`, `explore`, `solve`, `sweep`), answer strength, schedule seed,
   replay command, sweep row; double-click selects the element (`SelectionUtilities`/
   `Application.getInstance().getMainFrame().getBrowser()`), and *Copy as `sysml` command* gives
   the replayable CLI line.

Verdicts arrive from the Java client as `Verification` (constraint/requirement, with
`verdict`, `bounded`, the checked object's `instancePath`), `Satisfaction` (every
`assert satisfy`), `Validation` (every constraint of one object) and `Sweep` rows
([`docs/reference/java-api.md`](../../reference/java-api.md), Verification and Parameter
sweeps). Each names the v2 element; §5 maps it back.

## 7. Positioning against the Cameo Simulation Toolkit

### 7.1 What the Simulation Toolkit covers

Magic Model Analyst / Cameo Simulation Toolkit is "an extendable model execution framework based
on OMG fUML and W3C SCXML standards" that executes, animates and debugs SysML models, including
parametrics, with mock-up user interfaces
([documentation home, 2026x](https://docs.nomagic.com/MMA/2026x/magic-model-analyst-cameo-simulation-toolkit-documentation-255624548.html)).
Specifically:

- **fUML 1.3** activity semantics, with the action kinds it supports enumerated
  ([Activity simulation engine, 2026x](https://docs.nomagic.com/MMA/2026x/activity-simulation-engine-255624614.html)).
- **State machines** on W3C SCXML semantics (per the home page above). PSSM conformance is
  **not claimed on any page found**; treat "PSSM" as unverified for the Toolkit. PSCS
  conformance likewise **unverified**; composite-structure behavior (ports, connectors, flows) is
  simulated, but no page found names the PSCS specification.
- **Parametrics**: a built-in math solver (Octave-like syntax) as the default parametric
  evaluator, plus external evaluators — MATLAB, Mathematica, Dymola — and scripting languages
  ([Built-in Math](https://docs.nomagic.com/MMA/latest/built-in-math-304011277.html);
  [Integration with external Evaluators, 2026x](https://docs.nomagic.com/MMA/2026x/integration-with-external-evaluators-255625324.html);
  [Specifying the language for the expression](https://docs.nomagic.com/MMA/2026x/specifying-the-language-for-the-expression-255626372.html)).
  The solver *evaluates*; it does not search for values that satisfy a constraint set.
- **Alf** comes from the separate Alf Plugin, which compiles Alf to fUML activities that the
  Toolkit executes ("Full Conformance" level)
  ([Alf Plugin, 2026x](https://docs.nomagic.com/MAA/2026x/magic-alf-analyst-alf-plugin-documentation-254417242.html);
  [Running a model with Alf, 2024x Refresh3](https://docs.nomagic.com/spaces/ALFP2024xR3/pages/227179165/Running+a+model+with+Alf)).
- Recent additions are the HTML UI, the Result Player, server-side simulation, and Modelica
  export; "certain outdated integrations have been discontinued" in 2026x
  ([2026x version news for the Toolkit](https://docs.nomagic.com/spaces/CST2024xR3/pages/242780666/2026x+Version+News)).

### 7.2 What OpenSysML adds, and what it does not replace

The Toolkit is the interactive simulator of the SysML **v1** model as drawn: animation,
mock-up UIs, external solvers, timelines. OpenSysML executes the **v2** model that the migration
(or, for v2 projects, the textual export — §8) produces, and adds what the Toolkit has no
counterpart for:

| OpenSysML capability | Where it is described | Toolkit counterpart |
|---|---|---|
| **Deterministic, replayable schedules** — every unordered choice recorded as a choice point; `declared` / `seed:<n>` replay; `explore` enumerates every linearization within a budget | [`scheduling.md`](scheduling.md), [`region-order-scheduling.md`](region-order-scheduling.md) | none: one interactive run at a time; no page found describes replaying a schedule |
| **Batch verification** — `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`, `ValidateInstance` over a whole model in one call, no UI | `api/proto/sysml.proto`; [`java-api.md`](../../reference/java-api.md) | validation suites check well-formedness, not requirement satisfaction under execution |
| **`RunSweep`** — a parameter grid or sample, one row per run with inputs, outputs and verdicts | [`java-api.md`](../../reference/java-api.md), Parameter sweeps | none found (the Toolkit runs one configuration; trade studies are a separate product) |
| **SMT-backed constraint solving** — the `solve` engine finds values satisfying a constraint set, and the bounded model checkers ask a solver whether any schedule violates a requirement | [`analysis-framework.md`](analysis-framework.md), [`smt-model-checking.md`](smt-model-checking.md) | the parametric evaluators compute a value from given inputs; they do not solve for unknowns |
| **RDF export** — `Convert` to Turtle; OSLC-shaped `Query` | [`docs/reference/rdf-mapping.md`](../../reference/rdf-mapping.md) | none |
| **Answer strength** — proved / bounded / witnessed / observed / not covered on every result | [`analysis-framework.md`](analysis-framework.md) | none |

What OpenSysML does **not** offer and the plugin must not pretend to: diagram animation of the
v1 model, mock-up UIs, MATLAB/Mathematica/Dymola evaluators, and Alf. A user with a
Toolkit-dependent model keeps using the Toolkit; the OpenSysML plugin sits beside it for
batch, sweep, solve and replay.

## 8. SysML v2 in Cameo — in 2026x it exists, and it changes the plugin

**2024x Refresh3 first, since that is the pin.** Its version news
([2024x Refresh3 Version News](https://docs.nomagic.com/spaces/CSM2024xR3/pages/239796953/2024x+Refresh3+Version+News))
announces no SysML v2 project type or textual import, and its Javadoc index has no textual
notation service (below). No SysML v2 project type or `.sysml` import for 2024x Refresh3 was
found in the public documentation; treat "2024x Refresh3 has no SysML v2 support" as
**unverified (not found)** rather than established. On that release OpenSysML is therefore the
*only* SysML v2 parser the plugin has, and every model reaches it through the v1 migration
(§4–§5).

The 2026x release line ships a **SysML v2 Plugin** with a textual editor and two-way
synchronization between text and diagrams, a **SysML v2 Evaluation Plugin** for static
evaluation, and a free **Community Edition** capped at 500 elements
([SysML v2 Plugin documentation, 2026x](https://docs.nomagic.com/SYSML2P/2026x/sysml-v2-plugin-documentation-254421938.html);
[CATIA Magic/Cameo SysML v2 Solution](https://docs.nomagic.com/SYSML2P/2026x/catia-magic-cameo-sysml-v2-solution-272740940.html)).
SysML v1 and v2 are chosen **per project**, in one installation (same page). Concretely:

- **Textual import/export in the UI**: File ▸ Export To ▸ *SysML v2 Textual Notation* writes
  selected root namespaces as `.sysml` files; File ▸ Import From ▸ *SysML v2 Textual Notation*
  imports a `.sysml` file "into a separate root namespace"
  ([Textual notation import/export](https://docs.nomagic.com/SYSML2P/2026x/textual-notation-import-export-254422195.html)).
- **Textual import/export in the OpenAPI** (2026x Refresh1):
  `SysMLTextualNotationService.exportTextual(Namespace) → String` and
  `importTextual(ModelElementProject, String)`
  ([Javadoc](https://jdocs.nomagic.com/2026xRefresh1/com/dassault_systemes/modeler/magic/sysml/textual/SysMLTextualNotationService.html)),
  and `SysMLProjectHelper` to create or open v2 ("UPS") projects
  ([Javadoc](https://jdocs.nomagic.com/2026xRefresh1/com/dassault_systemes/modeler/magic/sysml/core/SysMLProjectHelper.html)).
  The v2 metamodel is a separate API (`com.dassault_systemes.modeler.kerml.model.kerml.Namespace`,
  the `com.dassault_systemes.modeler.sysml.libraries.standard.*` library classes), not the UML
  `Element` tree.
- **Their own v1→v2 migration** (File ▸ Export To ▸ SysML v2 Model) is "a work in progress,
  covering about 20% of the metamodel", writes an `.xlsx` of not-migrated elements, and does not
  migrate diagrams ([Migration from SysML v1 to SysML v2](https://docs.nomagic.com/SYSML2P/2026x/migration-from-sysml-v1-to-sysml-v2-254423020.html)).
  Whether the 2026x Refresh1 page reports a higher figure is **unverified**; the page fetched
  is the 2026x version.

**Consequence.** The plugin has two front ends and one engine:

| Project kind | How the model reaches OpenSysML | Identity map |
|---|---|---|
| SysML **v1** (2024x Refresh3 and 2026x) | `.mdzip` → `Convert(xmi→sysml)` → `ParseSources` (§4) | migration report `id → target` (§5) |
| SysML **v2** (2026x with the SysML v2 Plugin) | `SysMLTextualNotationService.exportTextual(root)` → `ParseSources` — **no migration** | v2 qualified names are the same on both sides; OpenSysML's `Symbol` answers carry them |

For v2 projects OpenSysML is a *second parser and the execution engine* of text the vendor's own
parser also reads; disagreements between the two parsers are themselves findings (the
`Diagnostic`s from `ParseSources` land in the results panel). Whether `exportTextual` emits
element IDs as comments or `@id` metadata that would give a stronger identity than names is
**unverified**. The plugin's v2 path compiles only against 2026x jars (the 2024x Refresh3
Javadoc index lists no `com.dassault_systemes.modeler.magic.sysml.textual` or `.core`
package, only a handful of diagram classes under `com.dassault_systemes.modeler.magic`), so it
is a separate
module loaded by reflection or a second plugin, and `isSupported()` of the v2 module checks for
the SysML v2 Plugin.

## 9. Migration benchmark

**Question.** Is `Convert` from v1 XMI/`.mdzip` good enough to be the plugin's primary path
for v1 projects?

**Method.** OpenSysML built with `make build`. Twenty models were gathered: the ten fixtures
under `tests/migrate/testdata/xmi/`, seven Papyrus SysML 1.1 test models
(`github.com/bmaggi/Papyrus-SysML11`, `tests/…/model/*.uml` and `samples/*.uml`), two Cameo
`.mdzip` projects from Open-MBEE's MDK (`github.com/Open-MBEE/exec-cameo-mdk`:
`src/main/dist/samples/MDK/DocGen.mdzip`, `src/test/resources/CSyncTest.mdzip`), and the OMG
SysML 1.6 profile itself (`https://www.omg.org/spec/SysML/20181001/SysML.xmi`; a profile, not a
user model, included as the only OMG XMI artifact that resolved). For each:

```bash
bin/sysml <model> -convert sysml -o <out>.sysml -migration-report <out>.report.json
bin/sysml <out>.sysml -validate
```

`-validate` is the CLI's check-and-exit flag (`bin/sysml -h`; there is no `-check` flag).
Diagnostics were counted as lines matching `error:` and `warning:` case-insensitively, excluding
the `no errors` summary. Attempts that did **not** resolve, so the set is what it is:
`github.com/eclipse-papyrus/org.eclipse.papyrus-sysml16` and `…-sysml11` (404),
`github.com/Open-MBEE/mms-test` (404), the `bmaggi/SysML14-Gendoc-Example` model (listed by
the API, raw download 404), `https://www.omg.org/spec/SysML/1.6/SysML.xmi` (404). No vendor
sample `.mdzip` is downloadable without an installation.

**Results.** Every conversion and every validation exited 0; no panics, no `error:`
diagnostics on any converted model.

| file | source | entries | mapped | approx. | unmapped | skipped | check errors | check warnings | top unmapped kinds |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `acquisition.xmi` | repo fixture | 65 | 52 | 5 | 6 | 2 | 0 | 0 | DurationObservation (6) |
| `heater_receptions.xmi` | repo fixture | 81 | 69 | 10 | 2 | 0 | 0 | 0 | Parameter, Reception |
| `meter.xmi` | repo fixture | 130 | 97 | 33 | 0 | 0 | 0 | 0 | — |
| `plant.xmi` | repo fixture | 62 | 56 | 6 | 0 | 0 | 0 | 0 | — |
| `plant_states.xmi` | repo fixture | 112 | 103 | 5 | 3 | 1 | 0 | 3 | TimeEvent, Trigger |
| `ported_calls.xmi` | repo fixture | 53 | 45 | 8 | 0 | 0 | 0 | 0 | — |
| `reactor.xmi` | repo fixture | 79 | 56 | 23 | 0 | 0 | 0 | 0 | — |
| `rig_interactions.xmi` | repo fixture | 88 | 75 | 6 | 6 | 1 | 0 | 0 | Interaction (3), MessageOccurrenceSpecification (2) |
| `simconfig.xmi` | repo fixture | 38 | 30 | 6 | 2 | 0 | 0 | 0 | Slot (2) |
| `vehicle.xmi` | repo fixture (exporter "Example UML Tool") | 95 | 78 | 11 | 3 | 3 | 0 | 2 | «Unit», «QuantityKind» InstanceSpecification |
| `DocGen.mdzip` | Cameo, Open-MBEE MDK | 1907 | 810 | 388 | 336 | 373 | 0 | 28 | Constraint (123), «Viewpoint» Class (116) |
| `CSyncTest.mdzip` | Cameo, Open-MBEE MDK | 9 | 6 | 2 | 0 | 1 | 0 | 0 | — |
| `SysML_Allocate_TEST.uml` | Papyrus 1.1 | 12 | 1 | 8 | 0 | 3 | 0 | 0 | — |
| `SysML_DeriveReqt_TEST.uml` | Papyrus 1.1 | 12 | 1 | 8 | 0 | 3 | 0 | 0 | — |
| `SysML_Satisfy_TEST.uml` | Papyrus 1.1 | 13 | 1 | 9 | 0 | 3 | 0 | 0 | — |
| `SysML_Verify_TEST.uml` | Papyrus 1.1 | 16 | 1 | 5 | 7 | 3 | 0 | 0 | «TestCase» (4), «Verify» Abstraction (3) |
| `ModelWithBDD.uml` | Papyrus 1.1 | 12 | 1 | 0 | 0 | 11 | 0 | 0 | — |
| `ModelWithIBD.uml` | Papyrus 1.1 | 13 | 1 | 1 | 0 | 11 | 0 | 0 | — |
| `ModelWithPD.uml` | Papyrus 1.1 | 14 | 1 | 2 | 0 | 11 | 0 | 0 | — |
| `SysML.xmi` | OMG profile | 2 | 0 | 0 | 1 | 1 | 0 | 0 | Tag |

**Reading it.**

- **On Cameo's own format the classifier works.** `DocGen.mdzip` — a real, 1907-entry Cameo
  project — converts with 63 % of entries mapped or approximated and validates with no errors.
  Its unmapped entries are dominated by two kinds that are *out of scope for execution*: 122
  `Constraint`s whose specification is a UML `Expression` tree ("a UML Expression tree has no v2
  form") and 116 «Viewpoint» classes plus «Expose» dependencies ("viewpoints are not migrated
  yet") — DocGen is a document-generation profile, not a system model. The 28 warnings are all
  `Duplicate of inherited member name` from generalizations the migration kept. The 373 skipped
  entries are profile applications, imports and notation, as the report's `Unreferenced()`
  separates them.
- **On system models, the fixtures, 80–95 % of entries map cleanly**, the remainder are
  *approximated* with a note (return parameters as `out`, untyped properties as reference
  usages, partitions as comments), and the unmapped kinds are exactly what the migration
  reference lists as not yet migrated: interactions, duration observations, time events, units.
  The three `plant_states` warnings are OpenSysML's own extension notice for `history`/`junction`
  pseudostates it wrote ([`pseudostates.md`](pseudostates.md)) — a fact about the target
  notation, not a migration loss.
- **The Papyrus 1.1 rows are a namespace finding, not a migration finding.** Those files
  declare `xmlns:uml="http://www.eclipse.org/uml2/3.0.0/UML"` and the 2010-era SysML profile
  namespace, which the reader does not recognize (it accepts the OMG namespaces and Papyrus
  SysML 1.6's); every element but the root is *skipped* as an unrecognized profile application.
  Papyrus 1.6 samples would have been the fair test; none resolved. A Cameo plugin never sees
  this dialect.
- **Units are not migrated** («Unit»/«QuantityKind» instance specifications in `vehicle.xmi`),
  as the migration reference states; a v1 model whose constraints depend on unit conversion
  evaluates differently until they are.

**Verdict.** For a v1 *system* model saved by Cameo, `Convert` is good enough to be the primary
path: it never fails, every element is accounted for with a verdict and a note, and what is lost
(interactions, viewpoints, units, expression-tree constraints) is nameable and shown to the user
per element rather than silently dropped. The plugin must present *approximated* and *unmapped*
counts before the first run (the pre-flight in §10.4) so the user knows what the engine did not
see; that is the report-over-service prerequisite of §11, phase 3. The benchmark's weakness is
its sample — two Cameo projects, neither a behavioral system model; the plugin's first
integration test against a licensed Cameo (§10.3) should convert the vendor's bundled samples
(`samples/SysML/*.mdzip` in an installation) and re-run this table.

## 10. Architecture of `editors/cameo/`

### 10.1 Module layout

```
editors/cameo/
  README.md                       install, build, run-in-Cameo, licence-free CI
  settings.gradle.kts / build.gradle.kts
  plugin/                         the plugin proper — compiles with --release 17
    src/main/java/org/openmbee/opensysml/cameo/
      OpenSysMLPlugin.java            Plugin: isSupported / init / close
      Engine.java                     owns the one Connection; binary resolution from the plugin dir
      actions/                        Run, Verify, Sweep, ShowResults (MDAction subclasses)
      configurators/                  Browser/Diagram/MainMenu configurators
      export/                         Exporter: exportModule / saveProject → tmp .mdzip
      identity/                       ElementMap: report id → Cameo element, target → id
      results/                        ResultsWindow (ProjectWindow), Annotations, ValidationSuite bridge
    src/main/resources/plugin.xml
    src/main/resources/descriptor.xml  Resource Manager descriptor (templated at build)
  plugin-v2/                      SysML v2 front end; compiles only against 2026x jars
    src/main/java/.../v2/TextualExport.java   SysMLTextualNotationService bridge
  openapi-stubs/                  compile-only stubs of the OpenAPI classes the plugin touches (§10.3)
  bin/                            sysml-grpc-<os>-<arch>[.exe] staged at build; not committed
  dist/                           the Resource Manager .zip
```

The plugin depends on `client/java/opensysml-client` as a project dependency (nothing is
published yet, [`java-api.md`](../../reference/java-api.md)); the Gradle build includes it with
`includeBuild("../../client/java")`.

### 10.2 Build

Gradle (Kotlin DSL), matching the Java client's build rather than Maven; the vendor's own
examples are Ant/Gradle. Two source sets need OpenAPI jars on the compile classpath:

- **Licensed developer machine:** `-PcameoHome=/opt/Cameo` puts `<cameoHome>/lib/*.jar` and
  `<cameoHome>/plugins/**/*.jar` on the compile classpath (`compileOnly`). Nothing from the
  installation is copied into the artifact.
- **CI:** §10.3.

The `distZip` task lays out `plugins/org.openmbee.opensysml/{plugin.xml,*.jar,bin/*}` and
`data/resourcemanager/MDR_Plugin_org_openmbee_opensysml_<n>_descriptor.xml` and zips it; the
nightly attaches it beside the `.vsix` with the same `<version>-nightly-<date>-<commit>` scheme.

### 10.3 Compiling without a licence

The OpenAPI jars are not on Maven Central and the licence forbids redistributing them (the
vendor's public GitHub examples, e.g. the MDK at `github.com/Open-MBEE/exec-cameo-mdk`, resolve
them from a local installation). Two ways to keep CI honest:

1. **Compile-only stubs** (`openapi-stubs/`): hand-written classes with the *signatures* the
   plugin calls — `Plugin`, `PluginDescriptor`, `ActionsConfiguratorsManager`, the three
   configurator interfaces, `MDAction`/`MDActionsCategory`, `Application`, `Project`,
   `ProjectsManager`, `ProjectDescriptorsFactory`, `ProjectWindowsManager`, `ProjectWindow`,
   `WindowComponentInfo`, `ProgressStatusRunner`, `RunnableWithProgress`, `ProgressStatus`,
   `Annotation`, `AnnotationManager`, `ValidationHelper`, `ValidationRunData`,
   `RuleViolationResult`, `BaseElement`/`Element`/`NamedElement`/`Package`, `GUILog`. Every stub
   method body is `throw new UnsupportedOperationException("stub")`. The stubs compile the plugin
   and let its unit tests run against Mockito mocks of the same types; the Javadoc pages cited
   here are the specification each stub is written from. A stub drifts silently when the vendor
   changes a signature — so:
2. **A licensed agent** for the integration lane: a self-hosted runner with a Cameo installation
   and a floating licence runs the same build with `-PcameoHome`, which fails to compile if a
   stub lied, and then runs the plugin headless. Cameo supports headless execution of a plugin
   through `com.nomagic.magicdraw.commandline.CommandLine`/`ProjectCommandLine`
   ([Javadoc](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/commandline/ProjectCommandLine.html)),
   which is how the integration test opens each sample `.mdzip`, runs export → convert → parse →
   verify, and asserts the identity map is total over the report's `mapped` entries. This lane
   is optional and non-blocking for outside contributors, and required for release. **Whether
   the vendor's licence terms permit an unattended CI seat is unverified**; a maintainer with
   the licence agreement must confirm before the lane exists.

### 10.4 The "Run with OpenSysML" sequence

```
user: right-click Package P (or a Block, Behavior, Requirement) ▸ OpenSysML ▸ Run…
  1  configurator resolves the selection to packages, or the owning package of a single element
  2  ProgressStatusRunner.runWithProgressStatus(task, "OpenSysML", allowCancel=true, 0)
  3  task, phase "export":   Exporter → tmp/<project>-<hash>.mdzip        (exportModule / saveProject)
  4  task, phase "convert":  Conversion c = connection.convertFile(tmp, "sysml")
                             c.experimentalNotice → GUILog once per session
                             c.diagnostics → results panel
     pre-flight:             migration report → counts (mapped/approximated/unmapped) and the
                             unmapped kinds; shown in the panel header; user may stop here
  5  task, phase "parse":    Model m = connection.parseSources(List.of(SourceDocument.inline("model.sysml", c.content())))
                             parse diagnostics → results panel (a v1 model that converts but does
                             not parse is a migration bug to report upstream — its .sysml is kept)
  6  task, phase "run":      per the action chosen:
                               Run       → m.instantiate(target); m.runAction / runState (schedule policy from the dialog)
                               Verify    → m.verifySatisfaction(scope) + verifyConstraint/Requirement per selected element
                               Sweep     → m.runSweep(calc, ranges, options) from a small dialog
     each answer's element names → identity map → Cameo elements
  7  results:  Annotations on the mapped elements; RuleViolationResults into the validation
               window (when the suite module is loaded); every row into the OpenSysML Results panel
  8  the tmp .mdzip and .sysml are kept under the tool's temp dir until the next run, with a
     "Reveal converted model" action, because the user will want to read what the engine read
```

`isCancel()` is polled between phases and the current phase's deadline bounds the wait inside
one. The first run after start-up pays the child start (the client starts it lazily); later runs
share the parse cache when the exported archive is unchanged (the client hashes sources).

### 10.5 Results mapping

`ElementMap` is built once per conversion from the report: `Map<String xmiId, Entry>` and
`Map<String target, String xmiId>`; a Cameo `Element` is fetched by ID with `Project.getElementByID(String)`
([Project, 2026x Refresh1](https://jdocs.nomagic.com/2026xRefresh1/com/nomagic/magicdraw/core/Project.html)). Answers carry v2 qualified names (`Symbol`,
`Verification.constraintId`/`requirementId`, `SweepRow`), resolved through the second map. A
name with no entry — a library element, or a name the conversion synthesized — is shown in the
panel without an element link, never dropped.

## 11. Phased plan

| Phase | Delivers | Prerequisites in OpenSysML |
|---|---|---|
| **1. Migration-based execution** | `editors/cameo/plugin` skeleton; `plugin.xml`; browser/diagram/menu actions; export → `Convert` → `ParseSources` → `instantiate`/run/verify; results in `GUILog` and a plain table; stubs lane in CI; nightly `.zip` | none — everything used is on the wire today |
| **2. Results UI** | docking `ProjectWindow` table; `Annotation`s on elements; validation-suite module and `RuleViolationResult` bridge; *Reveal converted model*; per-phase deadlines and cancel | none for the UI; **identity requires phase 3's report** — until then names only |
| **3. Units and report-over-service** | pre-flight counts and per-element migration notes in the panel; unit-bearing constraints evaluated correctly | `Convert` returns the migration report (`ConvertResponse.migration_report`, the `Entry` shape of `report.go`, as [`api-surface-parity.md`](api-surface-parity.md) plans) and accepts several source files for one conversion (used modules); unit migration in `internal/translate/migrate` |
| **4. Step-debug** | step a behavior from Cameo; highlight current state / fired transition / token on the open diagram through `AnnotationPainter`s; choice-point display and reseed | the debugger session API of [`api-surface-parity.md`](api-surface-parity.md) on the wire and in the Java client |
| **v2 front end** (in parallel from phase 1 on a 2026x machine) | `plugin-v2`: `exportTextual` → `ParseSources`, no migration; parser-disagreement report | none |

## 12. Unknowns and risks

Each item says what is known, what is not, and what would settle it.

1. **Windows child process.** `PrivateService` starts `sysml-grpc -exit-with-parent`, holds the
   write end of the child's stdin pipe and never writes; the child exits on end of file, which
   the kernel delivers when the parent dies. The source states that Windows anonymous pipes give
   the same guarantee (`PrivateService.java`, class comment). Two things are unverified on
   Windows *inside Cameo*: whether the tool's own process shutdown (a crash, or `close()` never
   called because another plugin vetoed exit and the user killed the process) still closes the
   handle promptly, and whether a Cameo launched from the vendor's `.exe` launcher inherits
   handles in a way that leaves a second holder of the pipe. Settle by: a licensed Windows
   integration run that kills `csm.exe` and asserts no `sysml-grpc.exe` remains after a few
   seconds. Mitigation if it fails: the child's `-exit-with-parent` gains a parent-PID poll on
   Windows.
2. **Executable bits and quarantine.** Resource Manager extraction may not preserve POSIX modes;
   macOS Gatekeeper may refuse an un-notarized binary from a downloaded zip (§2.4, unverified).
   Settle on licensed macOS/Linux runs; mitigation: `chmod` before launch, and document
   `xattr -d com.apple.quarantine` or ship notarized binaries.
3. **No OpenAPI OMG-XMI exporter.** The design relies on `.mdzip` (§4.2). If `exportModule`
   refuses packages that reference elements outside the selection, or `saveProject` to a new
   descriptor re-points the open project, the export falls back to saving the whole project to
   a temp file and passing the selection as `ParseOptions`/scope to OpenSysML instead. Settle
   with a licensed run.
4. **Migration report not on the wire** (§5). Without it the identity map is by name and
   renamed elements are lost; phase 2's per-element results are only as good as names.
5. **Used projects / Teamwork Cloud.** Modules are not loaded by the reader; a TWC project has
   no local file. Both need the multi-source `Convert` of phase 3 and a licensed TWC test.
6. **Units.** Not migrated; a model whose constraints depend on unit conversion is wrong, not
   just approximate, until `internal/translate/migrate` handles «Unit»/«QuantityKind».
7. **Two JDKs.** 2024x Refresh3 runs the plugin on JDK 17, 2026x Refresh1 on 21; `--release 17`
   covers both, but the `plugin-v2` module's vendor types exist only on 2026x, so the v2
   module must be loaded reflectively or shipped as a second plugin with `requires-plugin` on
   the SysML v2 Plugin.
8. **Licence terms for CI** (§10.3): unverified whether an unattended seat is permitted.
9. **Simulation Toolkit conformance claims** (§7.1): PSSM and PSCS conformance are not claimed
   on any page found; the positioning table says "SCXML-based state machines" and no more. If a
   page is found, the alignment note ([`precise-semantics-alignment.md`](precise-semantics-alignment.md))
   is the place to compare.
10. **Vendor `.mdzip` samples untested.** The benchmark's Cameo rows are two Open-MBEE projects;
    the vendor's bundled samples were not available. First licensed run re-does §9 over them.

## 13. Every unverified claim, in one list

- "No such OpenAPI class exists" statements were checked against the 2026x Refresh1 class index
  only, not 2024x Refresh3 (§1.1).
- That the Resource Manager preserves the POSIX execute bit when extracting a plugin zip on
  macOS/Linux (§2.4) — the design `chmod`s regardless.
- macOS Gatekeeper behaviour for a `sysml-grpc` binary extracted by the Resource Manager (§2.4).
- Reuse of the Simulation Toolkit's diagram animation for a foreign engine (§3) — assumed
  unavailable.
- Any OpenAPI entry point for File ▸ Export To ▸ *UML XMI 2.5 file* (§4.1) — none found.
- `ProjectsManager.saveProject` to a new local descriptor leaving the open project's own
  location unchanged (§4.1).
- `BaseElement.getID()` being the persisted `xmi:id` (§5) — verified only through the fixtures'
  IDs, not from the Javadoc page.
- The Simulation Toolkit's PSSM and PSCS conformance (§7.1).
- That 2024x Refresh3 has no SysML v2 project type or textual import (§8) — not found in its
  version news or Javadoc index, which is absence of evidence only.
- The 2026x Refresh1 figure for the vendor's own v1→v2 migration coverage (§8) — the 2026x page
  says about 20 %.
- Whether `SysMLTextualNotationService.exportTextual` emits element identity beyond names (§8).
- Windows pipe semantics of the child under Cameo's launcher and abnormal shutdown (§12.1).
- Whether the vendor's licence permits an unattended CI seat (§10.3).

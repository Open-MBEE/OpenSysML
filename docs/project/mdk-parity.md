# OpenSysML MDK against OpenMBEE MDK: the capability matrix

OpenSysML MDK (`editors/mdk/`) is positioned as the successor to OpenMBEE's Model Development
Kit ([Open-MBEE/mdk](https://github.com/Open-MBEE/mdk)) for Cameo Systems Modeler users moving to
SysML v2. This record lists what MDK does, what OpenSysML has today for each capability — in the
plugin, in the engine, or nowhere yet — and the order in which the gaps are being closed. It is
the backlog for the parity work and is updated as each row moves; nothing here claims parity
that the code does not have.

MDK's feature set is read from its `develop` source and user documentation. Its two centres of
gravity are **model synchronisation with MMS/Flexo** (commit, update, branches, the
sync-status and validation windows) and **DocGen** (view/viewpoint documents from the model to
DocBook, PDF and View Editor). Around them sit a validation suite with fix actions, element
operations (JSON export/import, reference tree), and a headless command line.

## Status vocabulary

| Status | Meaning |
| --- | --- |
| **Implemented** | the plugin offers it to a Cameo user today |
| **Bridged** | available through MDK's own extension point, so only where MDK is still installed |
| **Partial** | the engine or client has the substance; the plugin surface, or a piece of the semantics, is missing |
| **Not yet** | nothing in OpenSysML serves this |

## The matrix

| MDK capability | What MDK does | OpenSysML today | Status | Order |
| --- | --- | --- | --- | --- |
| Execution and verification | — (MDK executes nothing; the Cameo Simulation Toolkit does) | Instantiate, Execute action, Execute state, Verify, Evaluate calc, Run analysis from the browser and diagram context menus; results window; validation annotations on the elements | **Implemented** | — |
| DocGen «JavaExtension» queries | user-written `Query` classes loaded from `extensions/` into a document | one query per OpenSysML operation, rendered as DocBook summary, outcome and diagnostic tables and schedules; stereotypes created on demand | **Bridged** | — |
| DocGen documents from views and viewpoints | SysML v1 views/viewpoints with the DocGen expression library (sections, paragraphs, tables, images, filters) → DocBook → PDF, and View Editor | SysML v2 views, viewpoints and renderings → Markdown, HTML and PDF in the engine (`internal/doc`, `sysml -render-document`); not reachable from Cameo | **Partial** | 1 |
| Publish the model to MMS/Flexo | *Commit* of the project's elements as MMS JSON, per project and per branch | RDF export of a v2 model that loads into Flexo MMS (`internal/translate/export`; measured by `TestFlexoInterop`); one-way, not reachable from Cameo | **Partial** | 2 |
| Validate the model against MMS and fix | element-by-element diff between Cameo and MMS, listed in a validation window with *Commit*/*Update* fixes | nothing: no element-level diff, no update from Flexo into Cameo | **Not yet** | 4 |
| Branches, tags and project mounts in MMS | create/switch MMS branches and tags, keep mounted modules in sync | nothing | **Not yet** | 4 |
| Sync-status indicators | stereotype-driven markers for elements pending sync | nothing | **Not yet** | 4 |
| Validation suite with fix actions | `ValidationSuite`/`ValidationRule` windows, each violation with fix actions | tiered validation in the engine (`internal/check`) with workspace edits for fixes; the plugin shows run diagnostics in its results window and as annotations, but exposes neither a standalone *Validate* nor fixes | **Partial** | 3 |
| Element JSON export and import | *Export to MMS JSON* / *Import from JSON* on a selection | `Convert` between notation, XMI, RDF and the SysML v2 API JSON in the service; not reachable from Cameo | **Partial** | 5 |
| Reference tree | a browser of outgoing references from an element | nothing in the plugin; the engine's query layer resolves references | **Partial** | 5 |
| Headless command line | `AutomatedViewGeneration`, `AutomatedCommitter` (Cameo command-line plugin classes) | the service and `sysml` CLI run headless, outside Cameo; no `ProjectCommandLine` entry point in the plugin | **Partial** | 5 |
| Options pane | *Options ▸ Environment ▸ MDK* (MMS URL, credentials, logging) | none; the plugin finds its binary beside itself and needs no configuration | **Not yet** | 3 |
| User scripts | Jython/Groovy «UserScript» queries | none planned; the bridge's Java queries and the engine's analysis framework cover the scripted-report use | **Not yet** | — |

## The order, and why

1. **SysML v2 documents in Cameo.** A *Generate document* action on a v2 project's view or
   viewpoint that runs the engine's document renderer and opens the result. The engine already
   produces Markdown, HTML and PDF from v2 views; what is missing is the export of the selected
   view and the file handoff, both of which the existing `ModelSource`/`Engine` path provides.
   This is DocGen's purpose for v2 models and needs nothing from MMS. One session.
2. **Publish to Flexo.** A *Publish to Flexo MMS* action that runs the RDF export and loads it
   into a configured Flexo repository and branch. The export and its Flexo compatibility are
   measured already (`docs/project/rdf-corpus-roundtrip.md`, `TestFlexoInterop`); the work is
   the upload client, the credentials handling and the options pane the next row also needs.
   One session; a Flexo stack is needed to test it.
3. **Validation suite and options.** A *Validate with OpenSysML* action that runs the engine's
   validation tiers over the exported model and lands each diagnostic in Cameo's validation
   window, with the engine's workspace edits offered as fix actions where the edit maps back to a
   Cameo element; and the *Options ▸ Environment* pane for the Flexo endpoint. One to two
   sessions; the mapping of a textual edit onto a v1 element is the hard part and v2 projects
   come first.
4. **Two-way synchronisation.** Element-level diff between the Cameo project and Flexo, with
   update into Cameo, branches and tags, mounts, and sync-status markers. This is MDK's largest
   subsystem and depends on the SysML v2 API's element model in Flexo; it is not a single
   session and should be designed in its own note before it is built.
5. **Element operations and the command line.** JSON export/import of a selection through
   `Convert`, a reference tree over the engine's query layer, and `ProjectCommandLine` entry
   points for unattended document generation and publishing. Each is small once rows 1–3 exist.

## What will not be ported

- **MDK's SysML v1 DocGen expression library.** OpenSysML renders documents from SysML v2 views;
  a v1 project is migrated first (`docs/reference/sysml-v1-migration.md`). Where MDK is
  installed its DocGen keeps working and the bridge adds OpenSysML to it.
- **Jython and Groovy user scripts.** The analysis framework and Java extension queries cover
  the use, without a second interpreter in the plugin.

## Verification status

The plugin and the bridge compile against stubs and pass their unit tests in CI; nothing in
`editors/mdk/` has yet run inside a licensed Cameo or beside an installed MDK, which the design
note records in detail (`docs/internals/design/mdk-plugin.md`). Each row above moves to
**Implemented** only when the feature has been exercised in Cameo.

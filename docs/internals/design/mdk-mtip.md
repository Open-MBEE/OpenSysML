# MTIP and diagram geometry in the Magic MDK

**Status: design note, decisions open.** This records where OpenMBEE's MTIP plugin and its HUDS
XML fit the plan to bring the Magic MDK (`editors/mdk/`,
[mdk-plugin.md](mdk-plugin.md)) to parity with OpenMBEE MDK
([mdk-parity.md](../../project/mdk-parity.md)): what the repository already does with a HUDS
export, what the new *Export as SysML v2* action writes for v1 and v2 projects, what "MTIP
functionality" would have to mean inside the plugin, and the choices that are not made here.
Every claim about MTIP itself is read from its repository and documentation, not from running
it; §8 lists them.

## 1. The parity goal this serves

MDK's two centres of gravity are synchronisation with MMS/Flexo and DocGen; MTIP
([Open-MBEE/mtip-cameo](https://github.com/Open-MBEE/mtip-cameo)) is a third OpenMBEE plugin
for Cameo, which exports a project — or a selected part of it — to HUDS XML and imports HUDS
XML back, so that models move between Cameo and other tools (its README names Enterprise
Architect and a UAF profile among them). Feature parity for the OpenSysML MDK means both: MMS
synchronisation first (parity rows 2 and 4, to be designed in a note of their own), and the
MTIP transfer alongside it (parity row 6). The two meet on one need — a stable identity for
each element on both sides of a transfer — and that is why they are planned together even
though they are built apart.

## 2. What exists today

Verified in this repository:

- **A HUDS reader for the diagram layer.** `internal/translate/mtip` reads a HUDS export's
  diagram records — per diagram, the shown elements with their bounds and the connectors with
  their break points, each keyed by the element's identifier — and nothing else of the file.
  The migrator joins them to the migrated views by the `xmi:id` the migration already resolves
  and writes them as `DiagramLayout` metadata (`@Canvas`, `Layout about …`, `Route about …`;
  [diagram-layout-annotations.md](../../project/diagram-layout-annotations.md)). The roadmap
  recorded the decision this reader embodies: a HUDS export is **an augment to a migration,
  never a second model input**. What `DiagramLayout` has no attribute for (colors, fonts,
  images) is reported and dropped; MTIP records no label geometry, so none is written.
- **The surfaces that take it.** `sysml <model>.mdzip -migrate sysml -layout <mtip.xml>`; the
  Java client's `MigrationOptions.withLayoutFile`/`withLayoutContent`; and, from the plugin,
  the *Export as SysML v2…* action's prompt, which hands a chosen HUDS file to the migration
  as `V1MdzipSource.withLayout`.
- **Geometry without MTIP.** The `.mdzip` the plugin exports carries the project's own
  diagram symbol streams, and the migrator lays every diagram out from them. So for a v1
  project the export action writes `DiagramLayout` with no MTIP file at all; the MTIP prompt
  is for an export whose arrangement the engineer has already settled in HUDS, or whose
  archive lacks the streams.
- **Nothing writes HUDS, in the engine or the plugin, and nothing imports it into Cameo.**

Read from `Open-MBEE/mtip-cameo` (unverified by running it, §8):

- It is a Cameo plugin with a toolbar export (choose a file, save XML) and an import; its
  source is some 560 Java files and about 1 MB, the larger part a SysML and UAF metamodel
  mapping; its README states an Apache-2.0 licence.
- Its README lists a build for Cameo 2024x as current and none for 2026x, while the OpenSysML
  plugin's stubs and design target 2026x Refresh1 ([mdk-plugin.md](mdk-plugin.md) §1.1).

## 3. What the export action writes, per project kind

| Project kind | Text | Diagram geometry | Report |
|---|---|---|---|
| SysML **v1** | the migration of the exported `.mdzip` | `DiagramLayout` in every view, from the archive's symbol streams, or from a chosen MTIP export | `<name>.migration-report.txt` beside the file |
| SysML **v2** (2026x Refresh1 SysML v2 Plugin) | `SysMLTextualNotationService.exportTextual(root)`, verbatim | **none** — the vendor keeps a v2 project's diagrams outside the text (§6) | none: nothing was migrated |

Both go through `Engine.export` → `ModelExport.write` ([mdk-plugin.md](mdk-plugin.md) §10.6),
and the written text is parsed by the service so the completion dialog carries its diagnostics.

## 4. What "MTIP functionality" has to mean in the plugin

Four distinct things, which the parity row keeps apart:

1. **HUDS export from Cameo** of a v1 project or a selection of it, as MTIP does — the whole
   model, not only the diagram layer.
2. **HUDS import into Cameo**, creating or updating v1 elements and diagrams.
3. **HUDS as a layout augment** to a v1 migration — exists (§2).
4. **HUDS as a model input** to the migrator, so that a model that exists only as a HUDS export
   (one that came from another tool through MTIP) reaches SysML v2 without a `.mdzip`. This
   reverses the roadmap's augment-only decision and is not implied by 1–3.

A v2 project has no place in 1, 2 or 4: MTIP's metamodel is SysML v1 and UAF, and a v2
project's interchange format is the textual notation (and the SysML v2 API) rather than HUDS.

## 5. Three ways to get 1 and 2, and what each costs

| Option | What it is | For | Against |
|---|---|---|---|
| **(a) Port** | bring MTIP's exporter and importer into `editors/mdk/` as a module | one plugin for the user; the port can target 2026x where MTIP has no build; same licence family | ~1 MB of Java to own and keep aligned with the vendor API; most of it is metamodel mapping that MTIP's maintainers already keep; duplicates a sibling OpenMBEE project rather than completing it |
| **(b) Bridge** | reach an installed MTIP through reflection, as the DocGen bridge reaches MDK ([mdk-plugin.md](mdk-plugin.md) §14) | smallest; MTIP stays the owner of HUDS | works only where MTIP is installed, so only on releases MTIP builds for (2024x today, not 2026x); MTIP's export is a UI action, and whether it has a callable entry point is unverified |
| **(c) Own writer and reader** | write HUDS from the plugin ourselves, scoped to what OpenSysML's workflows need (elements and the diagram layer the migrator already reads), and read HUDS as a model input in the migrator (item 4) | scoped to our use, one code path per direction, testable against `internal/translate/mtip`'s fixtures | a partial HUDS that other tools' importers may reject; item 4 reverses a roadmap decision and needs a mapping of HUDS's element records onto the migrator's XMI-shaped input |

None is chosen here. The deciding questions are whether MTIP will build for 2026x (which makes
(b) viable on the pinned release), whether anything downstream needs a complete HUDS or only
the layer we read (which separates (c) from (a)), and whether item 4 is wanted at all.

## 6. Diagram geometry for v2 projects — open

A v2 project's `exportTextual` text carries no geometry, so an *Export as SysML v2* of a v2
project writes views without `DiagramLayout`, and the parity row is **Partial** for v2 on that
account. Ways to close it, each unverified against a licensed installation:

- **An OpenAPI geometry pass in the plugin.** If the 2026x Refresh1 OpenAPI exposes a v2
  diagram's presentation elements with bounds and paths (as the UML `PresentationElement`
  tree does for v1), the plugin can read them and write `DiagramLayout` metadata into the
  exported text itself, with no HUDS involved — the same metadata the v1 migration writes, so
  one renderer and one round trip. Whether that API exists for v2 diagrams is the first
  question to answer on a licensed seat.
- **The vendor's own interchange.** If the SysML v2 Plugin exports its diagrams in some form
  (a view-and-layout notation, or the SysML v2 API's view model), reading that is preferable
  to a HUDS detour; nothing of the kind was found in the public documentation.
- **HUDS through MTIP** is not a route for v2 projects (§4).

## 7. The identity that MTIP and MMS synchronisation share

HUDS keys every record by the element's identifier, which for a Cameo export is the `xmi:id`
the migration report maps to the v2 qualified name; MMS/Flexo synchronisation (parity row 4)
needs the same `xmi:id → v2 element` map in both directions to diff and update. The migration
report already keeps it for v1 ([mdk-plugin.md](mdk-plugin.md) §5); for v2 projects identity is
by qualified name until `exportTextual`'s output is known to carry element IDs (unverified,
[mdk-plugin.md](mdk-plugin.md) §13). Whichever of §5's options is taken should reuse that map
rather than keep a second one, and the MMS synchronisation note should be written against it.

## 8. Open decisions and unverified claims

Open decisions:

1. Port, bridge or own writer for HUDS export and import (§5).
2. HUDS as a model input to the migrator, or augment only (§4, item 4).
3. How a v2 project's diagram geometry is read, if at all (§6).
4. Whether the MTIP prompt in the export action stays a yes/no dialog or moves to an options
   pane once the plugin has one (parity row *Options pane*).

Unverified, to be checked on a licensed Cameo 2026x Refresh1 and against `Open-MBEE/mtip-cameo`
by running it:

- MTIP's current release list, licence and size, read from its README and tree at the time of
  writing.
- Whether MTIP's export has a callable entry point, or is only a UI action (§5, option b).
- Whether the OpenAPI exposes v2 diagram symbols and bounds (§6).
- Whether other tools' HUDS importers accept a partial HUDS file (§5, option c).
- That the export action and its MTIP prompt behave in Cameo as the stubs let them compile;
  nothing in `editors/mdk/` has run in a licensed installation yet.

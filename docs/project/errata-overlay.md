# The declared errata overlay

> This is an engineering record. `F<n>` names a row of the divergence census in
> [pilot-differential.md](pilot-differential.md), and an errata entry for an example model takes
> that row's number as its `ID`; an entry for the bundled standard library is named by file and
> line (`SI-137`).
>
> **Oracle figures in this record are as measured when it was written; they are not the current
> baseline.** The current baseline is the generated block in [README](../../README.md) and
> [architecture](../internals/architecture.md), regenerated and gated by `make docs-counts`.

## The gap this closes

Every divergence row against the pinned pilot implementation carries one of the
four categories of [the adjudications record](adjudications.md): *our defect*, *unimplemented
obligation*, *pilot limitation*, *adjudicated divergence*. A fifth case has no
category and no mechanism: **the OMG-published material is itself wrong**. Such a
row was written up in [omg-issues.md](omg-issues.md) and then stayed in the
divergence list for good, because the oracles read the published file exactly as
published — the only two outcomes available were "change our behaviour to match a
defective example" or "carry the row forever".

The overlay adds the missing one: the defect is declared, cited, and the
correction is applied to *what the oracles read a second time*, with both figures
reported.

## What it is

`tools/oracle/errata` is a registry of corrections to published reference material — the OMG
example corpora the oracles read, and the standard library vendored under
`internal/workspace/libs/stdlib`. The entry type, the overlay that applies corrections on read and
the library's own entries are the product's `internal/workspace/libs/errata`; the registry adds the
corpus entries and the corrected copy of a corpus root. One entry is one line of one file; a
file may carry several:

| Field | Meaning |
|---|---|
| `ID` | the row in [omg-issues.md](omg-issues.md) documenting the defect |
| `Path`, `Line` | the defective location, relative to the repository root |
| `AsPublished` | that line's exact bytes, verbatim |
| `Corrected` | what the oracles read instead — **empty means documented without a correction** |
| `Citation` | the specification clause the published text violates |
| `Derivation` | one line deriving the defect from that clause |

The registry as it stands:

| ID | Location | Citation | Shape |
|---|---|---|---|
| F82 | `sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38 | SysML v2 §9.8.9.1 | corrected — `22/2*25.4 + 110 [mm]` → `(22/2*25.4 + 110) [mm]` |
| F83 | `sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml`:25 | SysML v2 §9.8.9.1 | documented without a correction |
| F84 | `sysml-examples/Analysis Examples/Dynamics.sysml`:13 | KerML 7.4.9 | documented without a correction |
| SI-137 | `Domain Libraries/Quantities and Units/SI.sysml`:137 | KerML 7.4.9 | corrected — `eV*m^-2/kg` → `eV*m^2/kg` |
| SI-149 | `Domain Libraries/Quantities and Units/SI.sysml`:149 | KerML 7.4.9 | documented without a correction |
| SI-163 | `Domain Libraries/Quantities and Units/SI.sysml`:163 | KerML 7.4.9 | documented without a correction |
| SI-233 | `Domain Libraries/Quantities and Units/SI.sysml`:233 | KerML 7.4.9 | documented without a correction |
| SI-239 | `Domain Libraries/Quantities and Units/SI.sysml`:239 | KerML 7.4.9 | documented without a correction |
| SI-247 | `Domain Libraries/Quantities and Units/SI.sysml`:247 | KerML 7.4.9 | corrected — `m^3/C*m^3*s^-1*A^-1` → `m^3/C` |
| SI-286 | `Domain Libraries/Quantities and Units/SI.sysml`:286 | KerML 7.4.9 | documented without a correction |
| SI-299 | `Domain Libraries/Quantities and Units/SI.sysml`:299 | KerML 7.4.9 | documented without a correction |
| USCustomaryUnits-255 | `Domain Libraries/Quantities and Units/USCustomaryUnits.sysml`:255 | SysML v2 §9.8.9.1 | corrected — `229835/900 [K]` → `(229835/900) [K]` |

The library entries are derived in [omg-issues.md](omg-issues.md) under "Defects in the vendored
quantity libraries".

A third entry (the non-conforming redefinition at
`sysml-examples/Individuals Examples/AnalysisIndividualExample.sysml`:86) was retired at the
`2026-07` pin: the corpus now publishes the type the entry corrected to, so the as-published
verification failed and the entry was removed rather than re-pointed.

## The invariants, all of them tests

- **The published corpus is never written to.** Corrections are applied to a copy
  under the oracle's own output directory (`errata.Materialize`), which is
  removed after the run; `examples/pilot-corpora/` and the pilot checkout are
  read-only to this mechanism. A test copies a root, corrects the copy and
  asserts the published tree is byte-identical afterwards.
- **The vendored library is never written to either.** Its corrections are applied
  on read, by the `libs.Source` that `Overlay.LibrarySource` wraps around the
  embedded files: `libs.BundledSource` (what `libs.DefaultSource` serves, and what
  `stdlib.snapshot` is generated from) is the published text with the declared
  lines substituted, while `libs.EmbeddedSource` still serves the bytes as
  published. The two digest differently, so the snapshot decodes for the bundled
  text only. A directory named by `OPENSYSML_LIBRARY_PATH` is read as it stands.
- **An entry cannot rot.** `errata.Materialize` (and `errata.ApplyAll` under it)
  fails unless `AsPublished` still matches each entry's line on disk — the
  documented-only entries under the root included, so the report cannot list a
  defect the corpus no longer has — and re-vendoring the corpus invalidates the
  entry loudly instead of silently skipping the correction. A library read checks
  every declared line of the file the same way before substituting any, and fails
  the read rather than serve the file unverified.
- **One entry per line.** Two entries naming the same file and line are refused;
  several entries for one file are applied together.
- **No entry without provenance.** A missing citation, a missing derivation, a
  missing `omg-issues.md` row, a correction identical to the published text, a
  line that does not exist, or a path outside the published roots are all
  rejected — by `Entry.Validate`, or by `errata.New` for the roots — and covered
  by a test.
- **Documented-only entries substitute nothing.** An entry with no `Corrected`
  text is carried for provenance; both figures keep the published line.
- **Errata are not a reclassification route.** The overlay changes no category in
  [the adjudications record](adjudications.md)'s terms and no analyzer behaviour. F82 stays a
  true positive of ours; what the overlay records is that the *examples* are wrong.
- **A library correction is only declared for a line the checker rejects.** Two gates
  in `internal/workspace/model` pin the expression type checker's verdict on the standard
  library as exact sets: all nine findings over the published text, and exactly the
  six documented-only ones over the bundled library. A correction the checker still
  reports at, or a finding that vanishes without an entry, fails a gate.

## Both figures, and which one is the statement

Each oracle reports its census twice: as published, and with the corrections
applied. **The as-published figure is the conformance statement**; the
errata-applied one is a secondary diagnostic, and the generated block in
`README.md` and [architecture](../internals/architecture.md) says so in the same
sentence it prints them. Both come from the same run and the same committed
baseline (`tools/census/doccounts` reads the baselines' `errata` sections), so the
two figures cannot drift apart or be composed from different trees.

Measured with fresh caches when the overlay landed:

| Oracle | As published | With the errata applied |
|---|---|---|
| `pilot-diff` | 353 files, 325 fully agreeing; 32 agreed, 26 only ours, 61 only the pilot's | 353 files, **327** fully agreeing; 32 agreed, **24** only ours, 61 only the pilot's |
| `pilot-xpect` | 428 `.xt` files, 1261 assertions, 1323 rows, 1295 agree, 28 disagree | identical — no declared correction lies under `build/pilot-xpect-corpus` |
| `pilot-reject` | 120 cases: 120 both reject, 0 only the pilot rejects | identical — no declared correction lies under `tools/referee/reject/testdata/negative` |

Where no correction applies, the oracle says so in that many words rather than
printing a coincidentally equal number: *no declared correction lies under
`<root>`, so the errata-applied corpus is byte-identical to the published one and
both figures coincide.*

## The pilot is run against the corrected text too

A correction that clears *our* diagnostic while the reference still reports at
that line is a finding, not a fix, so each corrected root is re-run through
**both** implementations and the per-entry outcome is printed:

```
F82 …/VehicleGeometryAndCoordinateFrames.sysml:38 ours 1->0, pilot 0->0:
  our diagnostic is cleared and the pilot is silent on both texts
```

The pinned pilot performs no dimensional analysis, so it is silent on both texts
of the entry and no pilot verdict has changed yet. When one does, the finding carries
`pilotVerdictChanged: true` and the report says which way it moved.

## Adding an entry

1. Adjudicate the row: is the *published material* wrong, or are we? If the
   answer is "we are", it is a defect of ours and does not belong here.
2. Write the `omg-issues.md` row: published text verbatim, the citation, the
   derivation, and the correction if the intended reading is unambiguous. Do not
   invent one to close a row — document it without a correction instead.
3. Add the `errata.Entry`, copying the published line byte-for-byte, trailing
   whitespace included: a corpus entry to the registry, a library entry to
   `internal/workspace/libs/errata`.
4. `go test ./internal/workspace/libs/errata` and `go test -C tools ./oracle/errata`, then re-run the three oracles with fresh caches
   and `make docs-counts`. A library entry also needs `go generate ./internal/workspace/libs`
   (the snapshot is built from the corrected text) and the two `internal/workspace/model`
   gates moved between their published and bundled sets.

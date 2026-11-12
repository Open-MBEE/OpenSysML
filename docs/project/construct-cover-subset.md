# Construct-Cover Corpus Subset

## Overview

**Corpus:** every `.sysml` and `.kerml` file under `examples/` — the committed models and the
downloaded [training](training-examples.md), [pilot](pilot-corpora.md) and
[specification-example](spec-examples.md) roots, 880 files with the roots at their current pins
(56 committed, 100 training, 213 pilot, 511 specification examples).
**Subset:** `tests/corpus/testdata/construct_cover.txt`, one `examples/`-relative path per line,
sorted: 95 of the 880 files (19 committed, 6 training, 41 pilot, 29 specification examples),
which between them exhibit all 814 syntactic features the whole corpus exhibits.
**Switch:** `OPENSYSML_CORPUS_SUBSET`. Unset, every test that sweeps `examples/` iterates the
whole corpus, as before; `OPENSYSML_CORPUS_SUBSET=construct-cover` makes each of them iterate the
listed files only; any other value fails the sweep rather than silently running the whole corpus.
**Gate:** `TestConstructCover` in `tests/corpus/construct_cover_test.go` re-derives the feature
set of every file of the corpus on every run (it only parses, so it takes under a second with the
corpora present), and fails when a listed file is gone or a feature is exhibited only outside the
subset, naming each such feature and a file that has it.
**Regenerate:** `go test ./tests/corpus -run TestConstructCover -update-construct-cover`, with the
downloaded corpora present and the variable unset.

## Why

Twenty-odd tests walk every model under `examples/` and each parses, resolves and exports every
model on its own; under `-race` they are a third of the suite's test time. Most of them check a
property of the implementation on whatever syntax a model exhibits — that the API JSON export
reads back, that a graph-backed and a symbol-backed structure agree, that a batch load agrees
with a serial one — so what they need from the corpus is its variety of constructs, not its 880
files. The subset is the smallest set of files a greedy cover finds that still exhibits every
construct, so a tier that iterates it loses no construct, only the repetition.

## Features

`tests/testutil/constructcover` parses a file with `internal/syntax/parser` and walks the tree
by reflection, recording one feature for each of:

- the source kind (`kind SysML`, `kind KerML`) and whether the parse had diagnostics
  (`diagnostics`, else `clean`);
- every concrete node type (`node ast.Usage`);
- every parent-field → child-type edge (`edge ast.Membership.Member>ast.Package`), nested
  non-node structs named by their field path;
- every value of a Kind-style enumeration field (`enum ast.Usage.Kind=connection`) and of the
  string fields that hold notation keywords or operators (`enum ast.Usage.PrefixKeyword=assert`,
  `enum ast.OperatorExpr.Operator=>=`);
- every bool flag that is set (`flag ast.Definition.IsAbstract`).

The cover is a deterministic greedy set cover: at each step the file adding the most uncovered
features is taken, a committed file ahead of a downloaded one and the lesser path ahead of the
greater when they tie, so the list is reproducible and useful without the downloads. The counts
above are what the recorder measured; it prints them, and the list's header records them.

## What the switch does to each sweep

`tests/testutil/corpussubset` is the one helper every sweep calls: `Load`/`Must` read the
variable (and, for `construct-cover`, the list), and `Wants`, `WantsPath`, `WantsAny`, `Filter`
and `FilterPaths` answer "iterate this file?". A file outside `examples/` — the fixtures under
`tests/testdata`, `docs/manual/examples`, a package's own `testdata` — is always iterated.

- A sweep whose unit is one file iterates the listed files.
- A sweep whose unit is a directory of models (a demo whose files import each other, compared
  loaded as a batch against loaded serially, or by the CLI against the workspace) iterates a
  directory when any of its files is listed, and loads the whole directory: a partial directory
  would change what the test measures.
- The per-file ratchets — both round trips of `TestExamplesSweep`, the pilot and
  specification-example gates — compare the visited files' verdicts against the expectation file
  and leave the unvisited files' recorded lines unchecked; the recorded root sizes are still
  checked against the whole corpus, so a partial download fails as before. The pilot and
  specification-example gates still open each root whole, because its files import across each
  other and a file's diagnostics depend on its siblings; only the listed files are diagnosed.
- `TestSymbolStructureMatchesGraphStructure` records per-root totals, not per-file verdicts, so
  under the subset each root's disagreement counts must not exceed the recorded ones (a property
  the expectation does not list must not disagree at all); the exact comparison needs the whole
  corpus.
- Every recorder flag (`-update-corpus-roundtrip`, `-update-api-json-roundtrip`,
  `-update-pilot-corpora`, `-update-spec-examples`, `-update-structure-differential`,
  `-update-construct-cover`) refuses to run while the variable is set, so no expectation is ever
  recorded from a partial sweep.
- `TestTrainingExamplesSemanticErrors` ignores the variable: it asserts the whole training
  corpus clean in one batch load, which is cheap, and an assertion over part of a corpus asserts
  nothing. So does `TestDifferentialTrainingCorpus` (`internal/exec/solve`): its input is the
  training models that declare conditions, a semantic selection the syntactic cover does not
  preserve, and the whole run takes seconds.
- `TestConstructCover` ignores the variable too, since it exists to check the list against the
  whole corpus.

## Local runs

```bash
./scripts/download-training-examples.sh
./scripts/download-pilot-corpora.sh
./scripts/download-spec-examples.sh
OPENSYSML_CORPUS_SUBSET=construct-cover go test -count=1 ./tests/corpus ./tests/export ./tests/grpc
```

The subset is only a cover of the *syntax* the corpus exhibits. A change that moves a verdict on
a model whose constructs another file also exhibits is only seen by a whole-corpus run, which is
why the ratchets are re-recorded, and a release is scanned, over the whole corpus.

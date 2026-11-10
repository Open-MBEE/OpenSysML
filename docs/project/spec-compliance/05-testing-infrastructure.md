## Testing Infrastructure

See [`TESTING.md`](../internals/testing.md) for complete test contract details.

**Test counts:** stated once, in the Test Coverage list near the top of this document — the
per-prefix conformance breakdown and the trace, fixture, negative and robustness figures are all
there, and every other page links here rather than restating them (CONTRIBUTING.md).

**Quality Gates:**
- Parser: 111/111 stdlib files clean (94 vendored OMG, 17 OpenSysML extensions)
- Execution conformance: every case passing, with `known_failures.txt` empty
- Training examples: 100/100 files report no semantic errors (the error-only gate records no per-file errors in `tests/corpus/testdata/training_examples_expected.txt`)
- No regressions: All tests pass on every commit

> The training-example gate needs the corpus, which is not vendored: run
> `./scripts/download-training-examples.sh` first. The gate skips while the corpus is
> absent, so run the script before claiming a change is clean locally. CI downloads it
> (`.github/workflows/pr.yml`) and sets `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, which turns
> an absent corpus into a failure, so the gate can no longer skip green there.
> The gate runs against an empty semantic cache (`t.Setenv("XDG_CACHE_HOME", t.TempDir())`),
> so it reports the same 100/100 on any machine.

---

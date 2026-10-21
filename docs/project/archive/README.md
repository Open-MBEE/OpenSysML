# Archive

Measurement records that a later record has superseded. Each was written for the release or the
week it names and is kept as it was taken; the figures are not refreshed. The current release
comparison and the method behind these records are in the [project index](../README.md) and in
[performance](../../internals/performance.md).

| Record | What it measures | Release or date |
| --- | --- | --- |
| [performance-census-2026-09.md](performance-census-2026-09.md) | Week-over-week benchmark movement and whole-binary scaling of the validation path | 2026-09-01, `main` |
| [execution-performance-2026-09.md](execution-performance-2026-09.md) | Expression and calc evaluation throughput, state-machine and instantiation throughput, execution-path profiles | 2026-09-01, `main` |
| [performance-profile-2026-09.md](performance-profile-2026-09.md) | CPU and allocation profile of indexing, name resolution, the interpreters, batch checking, the request paths, the parser and the language server; the first profile-guided-optimization figures | 2026-09-02, `main` at `7ccf3c6f` |
| [performance-release-0.5-vs-0.4.3.md](performance-release-0.5-vs-0.4.3.md) | The 0.5 line against `v0.4.3`, with the fixes the comparison led to | 0.5.0, 2026-09-04 |
| [performance-release-0.6-vs-0.4.2.md](performance-release-0.6-vs-0.4.2.md) | `v0.6.0` against `v0.4.2` | 0.6.0, 2026-09-08 |
| [performance-release-0.8-vs-0.7.0.md](performance-release-0.8-vs-0.7.0.md) | The 0.8.0 candidate against `v0.7.0` | 0.8.0, 2026-09-13 |
| [performance-release-0.9.1-vs-0.9.0.md](performance-release-0.9.1-vs-0.9.0.md) | The 0.9.1 candidate against `v0.9.0`, with the regressions it attributed and fixed | 0.9.1, 2026-09-30 |
| [performance-release-0.9.2-vs-0.9.1.md](performance-release-0.9.2-vs-0.9.1.md) | `release/0.9.2` against `v0.9.1` | 0.9.2, 2026-10-05 |
| [performance-release-0.10.0-vs-0.9.2.md](performance-release-0.10.0-vs-0.9.2.md) | `v0.10.0` against `v0.9.2`, the rows in doubt re-run interleaved | 0.10.0, 2026-10-09 |

## Changelog

The release sections of [CHANGELOG.md](../../../CHANGELOG.md) older than 0.9.0, moved here verbatim
so the live changelog stays short. The live file ends with a link to this archive.

| Record | What it records | Releases |
| --- | --- | --- |
| [CHANGELOG-0.0-0.8.md](CHANGELOG-0.0-0.8.md) | Every release section from the first published release to the last of the 0.8 line | 0.0.4 – 0.8.1, 2026-08-10 to 2026-09-16 |

# OpenSysML Extensions Library — project manifests

The project interchange manifests of
[Open-MBEE/OpenSysML-Extensions-Library](https://github.com/Open-MBEE/OpenSysML-Extensions-Library)
at the commit `scripts/extension-libraries-pin.sh` pins, written here by
`scripts/sync-extension-libraries.sh` together with the units it vendors flat
under `stdlib/OpenSysML Libraries/`:

- `.project.json` (KerML 1.0 Table 12): the project's name, publisher, version,
  license and the standard-library archives it uses.
- `.meta.json` (Table 13): the index from each root namespace to its unit at
  `libraries/<Unit>` and a SHA-256 checksum per recorded file.
- `LICENSE`: the one recorded file that is not a unit.

`internal/workspace/libs/extensions_project_test.go` holds the vendored units
to these manifests: each unit's bytes match its recorded checksum and declare
the root namespace the index names, so the bundled copy is provably the
published project. The manifests live apart from the units because the units
are vendored flat, not under `libraries/`, and a `.project.json` beside them
would make the standard-library directory read as an installed project. Only
this README is written by hand; everything else here comes from the pin.

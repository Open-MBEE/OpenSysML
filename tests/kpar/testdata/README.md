# Project interchange archive fixtures

Real project interchange archives (KerML 1.0 §10.3, `.kpar`) that the tests in
`tests/kpar` read as named inputs, as installed projects under a library root, and
as a library in place of the bundled standard library.

## `omg/`

The ten standard-library archives of the OMG SysML v2 release **2026-08**, taken
unchanged from the `kpar/` directory of
[Systems-Modeling/SysML-v2-Release](https://github.com/Systems-Modeling/SysML-v2-Release)
at commit `fb97b754f29588b8e9c7a35f370880cd15eb29e7`. Their model files are the
same text as `internal/workspace/libs/stdlib` (the bundled copy of that release),
which is what `TestOMGArchivesIndexTheBundledLibrary` relies on.

## `sysand/`

A project as the [sysand](https://pypi.org/project/sysand/) package manager
(version 0.2.1) lays one out: the project's own `.project.json`, which uses
`pkg:sysand/ode4hera/requirements-management`, and the installed projects under
`.sysand/lib`:

- `ode4hera-requirements-management_2.0.1/` — the project directory sysand
  installed (MIT; its `LICENSES/` directory is included).
- `mbse4u-sysmod_5.1.4.kpar` — the project `requirements-management` uses,
  archived with `sysand build` from the directory sysand installed (Apache-2.0).

One usage therefore resolves to a directory and the other to an archive. To
reproduce the layout:

```sh
pip install sysand==0.2.1
sysand init --publisher probe
sysand add pkg:sysand/ode4hera/requirements-management
sysand build .sysand/lib/mbse4u-sysmod_5.1.4   # writes sysmod-5.1.4.kpar
```

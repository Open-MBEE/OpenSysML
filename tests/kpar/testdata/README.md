# Project interchange archive fixtures

Real project interchange archives (KerML 1.0 §10.3, `.kpar`) that the tests in
`tests/kpar` read as named inputs, as installed projects under a library root, and
as a library in place of the bundled standard library.

## `omg/`

The ten standard-library archives of the OMG SysML v2 release **2026-09**, taken
unchanged from the `kpar/` directory of
[Systems-Modeling/SysML-v2-Release](https://github.com/Systems-Modeling/SysML-v2-Release)
at commit `b7a6b77000c9136f8f24822fc9d2cb237c76267f`. Their model files are the
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

## `mbse4u/`

- `sysmod-5.1.5.meta.json` — the `.meta.json` of
  [MBSE4U/sysmod-sysmlv2](https://github.com/MBSE4U/sysmod-sysmlv2) at tag
  `5.1.5` (Apache-2.0), unchanged. sysand wrote it: one checksum object per
  path, as the normative `Meta` schema has it, and `"algorithm": "NONE"` with an
  empty value for every model file, recording no digest. The tests read it as
  metadata and as the `.meta.json` of an archive whose members it names.

## `syspi/`

A project as the [SysPI](https://github.com/Open-MBEE/SysPI) package manager
lays one out after `syspi init -name app && syspi add
pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.6`: the project's own
`.project.json`, whose usage is the ECMA-427 package URL the tool writes
(`pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6`),
its `.project.lock.json` (not read by OpenSysML, kept as the tool leaves it) and
the installed project under `lib/mbse4u-sysmod_5.1.6/` with the `.project.json`
of [MBSE4U/sysmod-sysmlv2](https://github.com/MBSE4U/sysmod-sysmlv2) at tag
`5.1.6` (Apache-2.0), unchanged. The repository's model files are replaced by a
one-declaration `SYSMOD.sysml`, and `App.sysml` imports it: the repository path
spells `MBSE4U/sysmod-sysmlv2` while the installed project is publisher
`mbse4u`, name `sysmod`, so the usage is linked by elimination.

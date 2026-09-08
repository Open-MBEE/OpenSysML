# Windows installer (MSI)

`opensysml.wxs` is the [WiX Toolset v5](https://wixtoolset.org/) source of the
per-machine x64 installer published with every release as
`opensysml-<x.y.z>-windows-amd64.msi` (and, once SignPath signing is
configured, `opensysml-<x.y.z>-windows-amd64-signed.msi`). It is a plain MSI:
no Burn bootstrapper bundle, no custom actions.

## What it installs

| Feature | Files | Default | Notes |
|---|---|---|---|
| **OpenSysML** (required) | `sysml.exe`, `sysml-lsp.exe`, `LICENSE.txt` | on | `%ProgramFiles%\OpenSysML` is appended to the system `PATH`. |
| **gRPC service (sysml-grpc)** | `sysml-grpc.exe` | on | Optional because the release bundle `opensysml-windows-amd64.zip` does not carry it either: the Python/Node/Java/Rust clients download their own verified copy, so it is only needed to run the service by hand. |
| **SMT solver (Z3)** | `z3\z3.exe`, `z3\msvcp140.dll`, `z3\vcruntime140.dll`, `z3\vcruntime140_1.dll`, `z3\LICENSE-z3.txt` | on | `%ProgramFiles%\OpenSysML\z3` is appended to the system `PATH`, so `sysml` finds `z3.exe` by PATH discovery. Deselect it to use your own solver (`OPENSYSML_SMT`). |

The MSI authors no dialogs of its own (see *Licensing*), so double-clicking
it installs every feature with Windows Installer's basic progress UI. Choose
features from an elevated prompt with the standard `ADDLOCAL`/`REMOVE`
properties:

```powershell
msiexec /i opensysml-0.5.0-windows-amd64.msi                      # everything
msiexec /i opensysml-0.5.0-windows-amd64.msi REMOVE=Z3            # bring your own solver
msiexec /i opensysml-0.5.0-windows-amd64.msi ADDLOCAL=Core /qn    # silent, sysml + sysml-lsp only
msiexec /x opensysml-0.5.0-windows-amd64.msi /qn                  # uninstall
```

Feature ids: `Core`, `GrpcService`, `Z3`.

Both `PATH` entries live in the MSI `Environment` table and are removed when
their component is uninstalled. The product shows up in *Apps & features* /
*Add or Remove Programs* as "OpenSysML" by "Open-MBEE", with the project
URL, the handbook as help link and a working uninstaller (SignPath requires
an uninstallable package).

Upgrades: the `UpgradeCode` (`800e1346-9227-42fe-87c1-eb24aff34de2`) is the
product's permanent identity and must never change; `MajorUpgrade` makes a
newer MSI replace an older installation in place, and a downgrade is refused.

### Version mapping

MSI `ProductVersion` must be numeric `major.minor.build` (`major`, `minor`
< 256, `build` < 65536). `scripts/build-msi.sh` takes the release tag and
strips the `v` and any pre-release/build suffix:

| Tag | ProductVersion | Notes |
|---|---|---|
| `v0.4.0` | `0.4.0` | |
| `v0.4.0-rc1` | `0.4.0` | Same `ProductVersion` as the final; `AllowSameVersionUpgrades` lets `v0.4.0` replace `v0.4.0-rc1`, and a second `rc` replaces the first. |
| `v0.4.0+meta` | `0.4.0` | |

The full tag is kept in the `OPENSYSML_TAG` property, the summary
information and the *Add or Remove Programs* comment, and `sysml -version`
still reports it.

## Building

The installer is built by `.github/workflows/release-windows.yml` on a
`windows-latest` runner. To build it by hand you need Windows (or Wine, see
below), the .NET SDK, and:

```bash
dotnet tool install --global wix --version 5.0.2

scripts/build-msi.sh v0.5.0 \
  dist/sysml-windows-amd64.exe dist/sysml-lsp-windows-amd64.exe dist/grpc/sysml-grpc-windows-amd64.exe
# -> dist/opensysml-0.5.0-windows-amd64.msi
wix msi validate dist/opensysml-0.5.0-windows-amd64.msi   # ICE validation
```

The script runs under Git Bash on Windows as well as bash on Linux. It
verifies the pinned Z3 zip against `z3.pin` before extracting anything, and
refuses to build on a mismatch.

### Why not Linux CI

WiX v5 is a .NET tool but needs Windows: on Linux `wix build` rejects every
`Directory/@Name` (`WIX0389`) and its cabinet builder is a Win32 executable
(`wixnative.exe`), which the WiX maintainers confirm is by design
([wixtoolset/issues#7154](https://github.com/wixtoolset/issues/issues/7154)).
That is why the MSI is not produced by the CircleCI release job. For a Linux
dry run the script accepts `WIX_CMD` (the command that runs `wix`) and
`WIX_PATH_PREFIX` (the Wine drive mapped to `/`, normally `Z:`):

```bash
# wix-wine.sh: exec wine dotnet.exe .../.store/wix/5.0.2/wix/5.0.2/tools/net6.0/any/wix.dll "$@"
WIX_CMD=./wix-wine.sh WIX_PATH_PREFIX=Z: scripts/build-msi.sh v0.0.0-test ...
```

The override is not called `WIX` because the WiX v3 installer exports `WIX`
as its installation directory (`C:\Program Files (x86)\WiX Toolset v3.14\`),
and the GitHub `windows-latest` runners ship it preinstalled.

This needs a Windows .NET 6 runtime and 32-bit Wine (`wixnative.exe` is
x86) and works for `wix build`, but `wix msi validate` (ICE) does not run
under Wine: run it on Windows, as CI does.

Inspect an MSI on Linux with [msitools](https://gitlab.gnome.org/GNOME/msitools):

```bash
msiinfo tables  opensysml-0.5.0-windows-amd64.msi
msiinfo export  opensysml-0.5.0-windows-amd64.msi Property | grep -E 'ProductVersion|Manufacturer|UpgradeCode'
msiinfo export  opensysml-0.5.0-windows-amd64.msi Feature
msiinfo export  opensysml-0.5.0-windows-amd64.msi Environment
msiinfo export  opensysml-0.5.0-windows-amd64.msi File
msiextract -C out opensysml-0.5.0-windows-amd64.msi && sha256sum out/PFiles64/OpenSysML/*.exe
```

## Updating the Z3 pin

`z3.pin` is the single place the bundled Z3 is pinned:

```
Z3_VERSION=5.1.0
Z3_ZIP=z3-5.1.0-x64-win.zip
Z3_SHA256=<sha256 of that zip>
```

To move to a new release:

1. Pick the release at <https://github.com/Z3Prover/z3/releases> and its
   `z3-<ver>-x64-win.zip` asset.
2. Download it and compute the hash yourself:
   `curl -fsSLO https://github.com/Z3Prover/z3/releases/download/z3-<ver>/z3-<ver>-x64-win.zip && sha256sum z3-<ver>-x64-win.zip`
3. Update the three values in `z3.pin`.
4. Check that `bin/` in the new zip still ships `z3.exe`, `msvcp140.dll`,
   `vcruntime140.dll`, `vcruntime140_1.dll` and that `z3.exe` imports nothing
   else (`objdump -p bin/z3.exe | grep 'DLL Name'`); adjust the file list in
   `scripts/build-msi.sh` and the `Z3Components` group in `opensysml.wxs` if it
   does. `libz3.dll` is not needed: `z3.exe` links it statically.
5. Build the MSI once and confirm `msiinfo export ... File` lists the Z3 files.
6. Mention the new Z3 version in the changelog fragment.

## Licensing

- **WiX Toolset** is licensed under the Microsoft Reciprocal License
  (MS-RL). It is build tooling only: a plain MSI built with `wix build`
  contains none of WiX's code, so the MS-RL does not attach to the installer
  or to OpenSysML. This is also why the MSI authors no dialogs: the stock
  `WixUI_*` dialog sets embed a small WiX custom-action DLL (`WixUiCa`, for
  the license dialog's Print button), which would put MS-RL code in the MSI.
  The `Binary` and `CustomAction` tables of the built MSI are empty.
- **OpenSysML** is Apache-2.0; the MSI installs `LICENSE.txt`.
- **Z3** is MIT-licensed by Microsoft Corporation. The MSI ships its
  `LICENSE.txt` as `z3\LICENSE-z3.txt` next to `z3.exe`, satisfying the MIT
  notice requirement. The VC++ runtime DLLs in `z3\` come from the same Z3
  release zip, which redistributes them under the Visual Studio runtime
  redistribution terms.
- **Signing.** When SignPath Foundation signing is configured the release
  workflow signs `sysml.exe`, `sysml-lsp.exe`, `sysml-grpc.exe` and the MSI
  itself. `z3.exe` and its DLLs are never signed with the Foundation
  certificate: SignPath's terms allow unsigned upstream open-source binaries
  inside a signed installer but do not allow signing them. The SignPath
  artifact configuration for the MSI must therefore sign only the MSI file.

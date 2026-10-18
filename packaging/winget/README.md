# winget packaging

`OpenMBEE.OpenSysML/` holds the maintained source of the
[Windows Package Manager](https://learn.microsoft.com/windows/package-manager/) manifest set
(version, installer and default-locale manifests, schema 1.10.0). They carry `__TAG__` /
`__VERSION__` / `__SHA256_*__` placeholders and are **not submittable as-is**;
`scripts/render-winget-manifests.sh` substitutes them from a release's `SHA256SUMS.txt`, strips
the template note and writes the `manifests/o/OpenMBEE/OpenSysML/<version>/` layout that
`microsoft/winget-pkgs` expects, following the same convention as
[packaging/homebrew](../homebrew/README.md).

Design:

- **Identifier `OpenMBEE.OpenSysML`** is a proposal. winget identifiers are
  `<Publisher>.<Package>` without punctuation in the publisher segment, so `Open-MBEE` becomes
  `OpenMBEE`; the first submission fixes it forever, so a maintainer must confirm it before
  submitting.
- **Zip installer, portable nested files.** The manifest installs the unsigned CircleCI asset
  `opensysml-windows-amd64.zip` described by `SHA256SUMS.txt`, with `NestedInstallerFiles`
  exposing `sysml.exe` and `sysml-lsp.exe` as the commands `sysml` and `sysml-lsp`. A second
  installer entry pointing at the MSI can be added later once its signing story has settled;
  winget validates MSI installers by `ProductCode`, which changes with every `MajorUpgrade`.
  `sysml-mcp` is not in this manifest: it ships as its own archive
  (`sysml-mcp-windows-amd64.zip`), and a winget installer entry carries one archive per
  architecture; `install.ps1 -Tools sysml-mcp`, Scoop and the MSI install it on Windows.
- **`Dependencies.PackageDependencies: Microsoft.Z3`** as decided by the maintainers, so the
  experimental `%check`/`%explain` commands get a solver on `PATH`. Note: at the time of
  writing `microsoft/winget-pkgs` has **no** `Microsoft.Z3` package (nor any other Z3 package),
  so `winget install` would fail to resolve the dependency until one exists; the maintainer
  submitting the manifest must either submit a Z3 manifest first, point at the identifier Z3
  actually gets, or drop the dependency and rely on the install guide's solver instructions.

## Rendering

```bash
scripts/render-winget-manifests.sh v0.5.0 out/                    # fetches SHA256SUMS.txt
scripts/render-winget-manifests.sh v0.5.0 out/ SHA256SUMS.txt
# -> out/manifests/o/OpenMBEE/OpenSysML/0.5.0/OpenMBEE.OpenSysML{,.installer,.locale.en-US}.yaml
```

The script depends on nothing in this repository except the template directory it reads, so it
works when fetched standalone at a tag.

## Validating

`winget validate` and `wingetcreate` are Windows-only. On Windows:

```powershell
winget validate --manifest out\manifests\o\OpenMBEE\OpenSysML\0.5.0
winget install --manifest out\manifests\o\OpenMBEE\OpenSysML\0.5.0   # needs `winget settings --enable LocalManifestFiles`
```

Offline, on any platform, check the rendered YAML against the published JSON schemas:

```bash
for k in version installer defaultLocale; do
  curl -fsSLo "manifest.$k.json" "https://raw.githubusercontent.com/microsoft/winget-cli/master/schemas/JSON/manifests/v1.10.0/manifest.$k.1.10.0.json"
done
python3 - <<'PY'
import json, yaml, jsonschema
d = "out/manifests/o/OpenMBEE/OpenSysML/0.5.0/OpenMBEE.OpenSysML"
for kind, f in [("version", f"{d}.yaml"), ("installer", f"{d}.installer.yaml"), ("defaultLocale", f"{d}.locale.en-US.yaml")]:
    jsonschema.validate(yaml.safe_load(open(f)), json.load(open(f"manifest.{kind}.json")))
    print(kind, "ok")
PY
```

## Submitting (maintainer procedure, done by hand)

Nothing in this repository writes to `microsoft/winget-pkgs`, and no CI job does either. A
maintainer submits the rendered manifests:

1. Confirm the identifier and the Z3 dependency (see *Design*).
2. Render for the latest release, run `winget validate` and install from the local manifest on
   a Windows machine; check `sysml -version` and `where z3`.
3. Fork [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs), copy the rendered
   `manifests/o/OpenMBEE/OpenSysML/<version>/` directory in, and open a pull request titled
   `New package: OpenMBEE.OpenSysML version <x.y.z>`. The repository's bot validates the
   manifest and installs the package in a sandbox; moderators review new packages. Read its
   `CONTRIBUTING.md` and the
   [manifest authoring guide](https://learn.microsoft.com/windows/package-manager/package/manifest)
   first. `wingetcreate submit` automates the fork/PR step from Windows.
4. For later releases, `wingetcreate update OpenMBEE.OpenSysML --version <x.y.z> --urls <zip url>`
   regenerates the manifests from the previous ones, or render again and repeat step 3.

Once accepted, replace the "coming once accepted" note in `docs/guide/01-install.md` with the
`winget install OpenMBEE.OpenSysML` command.

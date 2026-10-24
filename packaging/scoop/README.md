# Scoop packaging

`opensysml.json` is the maintained source of the [Scoop](https://scoop.sh/) manifest. It
carries `__TAG__` / `__VERSION__` / `__SHA256_*__` placeholders and is **not installable
as-is**; `scripts/render-scoop-manifest.sh` substitutes them from a release's `SHA256SUMS.txt`
and strips the template note, following the same convention as
[packaging/homebrew](../homebrew/README.md).

The manifest installs `opensysml-windows-amd64.zip` (`sysml.exe` and `sysml-lsp.exe`) and
`sysml-mcp-windows-amd64.zip` (`sysml-mcp.exe`, the MCP server for coding agents), the
unsigned CircleCI assets described by `SHA256SUMS.txt`, and shims the three executables onto `PATH`.
It declares `"depends": "z3"` so the `z3` manifest in Scoop's `main` bucket is installed
alongside, which gives the experimental `%check`/`%explain` commands a solver on `PATH` without
configuration; nothing links it and every other command works without it (see
[docs/guide/01-install.md](../../docs/guide/01-install.md#installing-a-solver-optional)).
Users who prefer the signed installer should use the MSI instead ([packaging/msi](../msi/README.md)).

`checkver` watches the GitHub releases, and `autoupdate` rewrites the URL for the new version
and takes the hash from that release's `SHA256SUMS.txt` (Scoop reads a checksum file when
`hash.url` points at one), so a bucket that runs Scoop's `checkver -u` needs no manual edits per
release.

## Rendering

```bash
scripts/render-scoop-manifest.sh v0.5.0 > opensysml.json          # fetches SHA256SUMS.txt
scripts/render-scoop-manifest.sh v0.5.0 SHA256SUMS.txt > opensysml.json
```

The script depends on nothing in this repository except the template path it reads, so it
works when both files are fetched standalone at a tag.

Offline check (Scoop's own `checkver`/`checkhashes` are PowerShell): validate the rendered JSON
against Scoop's schema:

```bash
curl -fsSLO https://raw.githubusercontent.com/ScoopInstaller/Scoop/master/schema.json
python3 -c 'import json,jsonschema;jsonschema.Draft7Validator(json.load(open("schema.json"))).validate(json.load(open("opensysml.json")));print("ok")'
```

## Submitting (maintainer procedure, done by hand)

Nothing in this repository writes to a Scoop bucket, and no CI job does either. A maintainer
submits the rendered manifest:

1. Render for the latest release as above and try it locally:
   `scoop install ./opensysml.json`, `sysml -version`, `where z3`, `scoop uninstall opensysml`.
2. Pick the bucket. [ScoopInstaller/Extras](https://github.com/ScoopInstaller/Extras) is the
   community bucket for GUI-less tools like this one (the `main` bucket is for tools without
   dependencies that pass `scoop checkver` and `checkhashes`; `depends` on `z3` is fine in
   Extras). Read its `CONTRIBUTING.md` first.
3. Fork the bucket, add `bucket/opensysml.json`, run the bucket's `bin/checkver.ps1 opensysml`,
   `bin/checkhashes.ps1 opensysml` and `bin/formatjson.ps1 opensysml`, and open a pull request
   titled `opensysml: Add version <x.y.z>`.
4. Alternatively, or until accepted, host the manifest in an Open-MBEE bucket repository
   (`scoop bucket add open-mbee https://github.com/Open-MBEE/scoop-bucket`), which can run
   Scoop's `Excavator` workflow to apply `autoupdate` itself.

Once accepted, replace the "coming once accepted" note in `docs/guide/01-install.md` with the
`scoop install` command.

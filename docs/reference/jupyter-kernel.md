# Jupyter kernel

`sysml-jupyter-kernel` is a native Jupyter kernel over the OpenSysML REPL session; the
[guide chapter](../guide/12-jupyter.md) shows it in use. This page is the reference: the
command, the kernelspec, what the kernel reports and renders, and how the package installs
it.

## The command

```
sysml-jupyter-kernel -connection-file FILE [-verbose]
sysml-jupyter-kernel -install [-user | -prefix DIR] [-name NAME] [-display-name NAME]
sysml-jupyter-kernel -print-kernelspec
sysml-jupyter-kernel -version | -man
```

| Flag | Meaning |
|---|---|
| `-connection-file FILE` | Serve the connection the file describes. Jupyter passes it. |
| `-install` | Write the kernelspec that starts this binary where Jupyter finds kernels, and exit. |
| `-user` | With `-install`: for this user alone (the default without `-prefix`), under `$JUPYTER_DATA_DIR` when set. |
| `-prefix DIR` | With `-install`: under `DIR/share/jupyter/kernels`, as into a virtual environment. |
| `-name NAME` | With `-install`: the kernelspec's directory name, which notebooks bind to; `sysml` by default. |
| `-display-name NAME` | With `-install`: the name front ends list the kernel under; `SysML v2 (OpenSysML)` by default. |
| `-print-kernelspec` | Write the `kernel.json` `-install` would write to stdout. |
| `-verbose` | Log protocol traffic that is dropped or fails to stderr (the server's log). |
| `-version`, `-man` | The version; the manual page in roff. |

Exit status `0` is a kernel the front end shut down, or a request answered; `1` a channel
that could not be bound; `2` a command line or connection file that could not be acted on.

The bounds and engine settings the `sysml` REPL reads from the environment —
`OPENSYSML_MAX_STEPS`, `OPENSYSML_MAX_ACTION_STEPS`, `OPENSYSML_JOBS`, `OPENSYSML_TOOLS` and the
rest listed in [environment variables](environment.md) — apply to the kernel, read when it
starts. `make build-jupyter-kernel` builds the binary; `make install` installs it with the
other commands and its manual page.

## The kernelspec

`-install` writes this `kernel.json`, naming the binary by its absolute path:

```json
{
  "argv": ["/usr/local/bin/sysml-jupyter-kernel", "-connection-file", "{connection_file}"],
  "display_name": "SysML v2 (OpenSysML)",
  "language": "sysml",
  "interrupt_mode": "message",
  "metadata": { "implementation": "sysml-jupyter-kernel" }
}
```

The pip package installs a kernelspec holding the binary itself and naming it through
`{resource_dir}`, so the spec can be copied or installed into any prefix:

```json
{
  "argv": ["{resource_dir}/sysml-jupyter-kernel", "-connection-file", "{connection_file}"],
  "display_name": "SysML v2 (OpenSysML)",
  "language": "sysml",
  "interrupt_mode": "message",
  "metadata": {
    "implementation": "sysml-jupyter-kernel",
    "package": "jupyter-opensysml-kernel",
    "package_version": "0.9.2"
  }
}
```

Beside `kernel.json`, both installs write `logo-32x32.png` and `logo-64x64.png`, the
OpenSysML mark, which the launcher and the kernel indicator show.

`interrupt_mode: "message"` makes the front end interrupt through the protocol's control
channel rather than with a signal, so a kernel on Windows is interrupted the same way. The
kernelspec's name, `sysml`, is what a notebook's metadata binds to; `language: "sysml"` is
what a front end falls back to when a notebook names a kernel that is not installed.

## What the kernel reports

`kernel_info_reply` carries:

| Field | Value |
|---|---|
| `protocol_version` | `5.3` |
| `implementation` | `sysml-jupyter-kernel` |
| `implementation_version` | The build's version, as `-version` prints it |
| `language_info.name` | `sysml` |
| `language_info.version` | `2.0` |
| `language_info.mimetype` | `text/x-sysml` |
| `language_info.file_extension` | `.sysml` |
| `banner` | The REPL's greeting |

## Cells and the protocol

The kernel serves the shell, control, stdin and IOPub channels and the heartbeat over
ZeroMQ, signs and verifies messages with the connection file's HMAC-SHA256 key, and drops a
message whose signature or framing is wrong, logging it under `-verbose`. It answers:

| Request | Answered by |
|---|---|
| `execute_request` | The cell split as the prompt splits input: each `%` command, declaration and bare expression in order, through the REPL session's own `RunMeta`, `Submit` and `EvalBare`. `execute_input`, `stream`, `display_data`, `execute_result` and `error` are published as the parts run; the first failure ends the cell with `status: error`. With `stop_on_error` (the default) the execute requests already queued behind the failure are answered `aborted`. `silent` cells publish nothing and are not counted. |
| `complete_request` | The REPL's completer over the cursor's line, with `cursor_start` and `cursor_end` as offsets into the cell in code points. |
| `is_complete_request` | `incomplete` while a brace, bracket or parenthesis is open, as the prompt's continuation rule says; `complete` otherwise. |
| `inspect_request` | What `%print` prints for the qualified name at the cursor; `found: false` when it names nothing. |
| `history_request`, `comm_info_request` | Empty: the kernel keeps no history of its own and opens no comms. |
| `interrupt_request` | Stops the cell under way: the run it drives ends at its next step with `KeyboardInterrupt`. Served on the control channel, so it does not wait behind the cell. |
| `shutdown_request` | Ends the kernel, or with `restart` starts an empty session. |
| `kernel_info_request` | The table above. |

`stdin_request` is not used: no REPL command reads from the terminal.

A `%quit` in a cell is noted — the notebook's shutdown ends the kernel — and an unknown
`%` command is an error (`UnknownCommand`), where the prompt would print guidance.

## Rich output

Output with a richer form than plain text is a `display_data` carrying that form beside
`text/plain`; a front end shows the richest form it renders.

| Output | MIME types |
|---|---|
| `%render <view> mermaid` | `text/vnd.mermaid` |
| `%render <view> dot` | `text/vnd.graphviz`, and `image/svg+xml` when a Graphviz `dot` is on the kernel's `PATH` |
| `%render <view> markdown` | `text/markdown` |
| `%render <view> csv` | `text/csv` |
| `%render <view> tsv` | `text/tab-separated-values` |
| `%render <view> plantuml`, `d2` | `text/plain` — the diagram source |
| `%render-document <doc> [form [style]]` | `text/markdown` |
| `%render-document <doc> [form [style]] html` | `text/html`, the HTML backend's fragment for the host page |
| `%features <instance> json` | `application/json` |
| A bare expression | `execute_result` with `text/plain` |
| Everything else | `stream` on `stdout`; a failure is an `error` |

## Installing with pip

```bash
pip install jupyter-opensysml-kernel
python -m jupyter_opensysml_kernel install [--user | --sys-prefix | --prefix DIR | --system]
                                           [--binary PATH | --release TAG]
                                           [--name NAME] [--display-name NAME]
python -m jupyter_opensysml_kernel uninstall [--name NAME]
python -m jupyter_opensysml_kernel kernelspec
```

`install` downloads the kernel binary for the host platform from the GitHub release the
package was built against and verifies it against the SHA-256 digest the package ships
(`jupyter_opensysml_kernel/release-digests.json`, stamped by the release pipeline from the
binaries it built), fetching the release's `.sha256` sidecar as well and requiring it to
agree; a release the package has no pin for is refused rather than trusted, as is a mismatch,
and nothing is written until the bytes have been verified. `--binary` registers a binary
already on disk instead, with no download; `--release` downloads another pinned release. Without a location, an install inside a virtual
or conda environment goes to that environment's prefix, and elsewhere to the user's Jupyter
data directory (`JUPYTER_DATA_DIR` when set).

The package's version is the core release's, and it pins only that release:
`jupyter-opensysml-kernel 0.9.2` installs `sysml-jupyter-kernel` from `v0.9.2`. Nightly
snapshots are published as development versions pinning the night's prerelease.
`OPENSYSML_GITHUB_REPO` points the download at a fork that publishes the same assets.

## Installing with conda

The conda-forge recipe is kept under `packaging/conda` and rendered for a release with
`scripts/render-conda-recipe.sh`. The conda package is per platform: it carries the release's
`sysml-jupyter-kernel` binary and installs the kernelspec under the environment's prefix at
build time, so nothing is downloaded when it is installed. Until the recipe is on
conda-forge, `pip install jupyter-opensysml-kernel` inside the conda environment installs the
same kernel there.

## Release assets

Each release publishes `sysml-jupyter-kernel-<os>-<arch>` for `linux-amd64`, `linux-arm64`,
`darwin-amd64`, `darwin-arm64` and `windows-amd64.exe`, raw with a `.sha256` sidecar and
listed in the signed `SHA256SUMS.txt`, beside the `jupyter_opensysml_kernel` wheel and sdist
that pin them. `install.sh --tools sysml-jupyter-kernel` installs the binary and verifies it
against the manifest; see [downloads](../downloads.md).

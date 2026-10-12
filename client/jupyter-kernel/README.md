# jupyter-opensysml-kernel

A [Jupyter](https://jupyter.org) kernel for SysML v2, backed by the
[OpenSysML](https://github.com/Open-MBEE/OpenSysML) REPL. A notebook cell is a
REPL submission: declarations accumulate into one model across cells, a bare
expression is evaluated, and every `%` command of the `sysml` REPL — `%eval`,
`%instantiate`, `%action`, `%render`, `%render-document`, `%load`, … — works as
it does at the prompt. Views render as Mermaid, Graphviz (SVG), Markdown, CSV
and JSON; documents as Markdown and HTML.

## Install

Python 3.10 or later and a Jupyter front end (JupyterLab, Notebook, VS Code,
`nbconvert`, …) are required.

```bash
pip install jupyter-opensysml-kernel
```

The wheel for your platform (Linux x64 and arm64, macOS Intel and Apple
Silicon, Windows x64) bundles the `sysml-jupyter-kernel` binary of the OpenSysML
release this package was built for and registers the `sysml` kernelspec under
the Python environment it is installed into, so that is the whole install:

```bash
jupyter kernelspec list            # shows: sysml  …/share/jupyter/kernels/sysml
jupyter lab                        # pick "SysML v2 (OpenSysML)"
```

Install with the Python that runs your notebook server (or in its environment);
`pip uninstall` removes the kernel and its kernelspec together.

Where no wheel applies, `pip` installs the sdist, which bundles no binary. Then
one more command downloads the release's binary, verifies it against the SHA-256
digest the package ships for that release, and registers the kernelspec with the
binary inside it:

```bash
python -m jupyter_opensysml_kernel install
```

Nothing unverified is installed: a download that does not hash to the pinned
digest is refused, and so is a release the package pins no digest for. The same
command registers the kernel for the user (`--user`) or under another prefix
(`--prefix DIR`, `--system`) when the notebook server does not run in the
environment the package is in, copying the bundled binary when there is one. To
use a kernel built from source (`go build ./cmd/sysml-jupyter-kernel`) or
installed another way, pass it explicitly:

```bash
python -m jupyter_opensysml_kernel install --binary ./sysml-jupyter-kernel
```

`python -m jupyter_opensysml_kernel uninstall` removes a kernelspec `install`
wrote and the binary in it; `jupyter kernelspec remove sysml` does the same.

The conda-forge recipe for this package is kept in the OpenSysML repository
under `packaging/conda`; once it is accepted there,
`conda install -c conda-forge jupyter-opensysml-kernel` installs the kernel and
its kernelspec into the environment directly. Until then, `pip install` inside
the conda environment registers the kernel under that environment's prefix the
same way.

A development snapshot of this package is published every night as
`jupyter-opensysml-kernel==<next release>.dev<yyyymmdd>`, which `pip install`
never picks up on its own; it carries the kernel of the same night's snapshot.

## Documentation

- [Using SysML in Jupyter](https://runtime.opensysml.org/guide/12-jupyter/) — the guide
  chapter: cells, `%` commands, rich output, interrupts.
- [REPL commands](https://runtime.opensysml.org/reference/repl-commands/) — every `%`
  command the kernel serves.

## Development

```bash
pip install -e '.[dev]'
pytest
mypy jupyter_opensysml_kernel
```

The tests need no network: downloads are served from fixtures, and the wheel
tests build from a staged stand-in binary (`python -m build` is required). The
package's copy of `release-digests.json` is synced from
`client/release-digests.json` by `scripts/sync-release-digests.py`; the release
pipeline stamps the digests of the kernel binaries it built into the copy, then
`scripts/build-jupyter-kernel-dist.sh` builds the sdist and, with each binary
staged under `jupyter_opensysml_kernel/bin/` and
`JUPYTER_OPENSYSML_KERNEL_PLATFORM=<goos>-<goarch>` set, one wheel per platform
(`setup.py` tags the wheel and adds the kernelspec as shared data).

## License

Apache-2.0.

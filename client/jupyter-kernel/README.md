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
python -m jupyter_opensysml_kernel install
```

The second command downloads the `sysml-jupyter-kernel` binary of the OpenSysML
release this package was built for, verifies it against the SHA-256 digest the
package ships for that release, and registers the `sysml` kernelspec with the
binary inside it. Inside a virtual or conda environment the kernelspec goes into
that environment; elsewhere it is installed for the user. `--user`,
`--sys-prefix`, `--prefix DIR` and `--system` choose explicitly.

```bash
jupyter kernelspec list            # shows: sysml  …/share/jupyter/kernels/sysml
jupyter lab                        # pick "SysML v2 (OpenSysML)"
```

Nothing unverified is installed: a download that does not hash to the pinned
digest is refused, and so is a release the package pins no digest for. To use a
kernel built from source (`go build ./cmd/sysml-jupyter-kernel`) or installed
another way, pass it explicitly:

```bash
python -m jupyter_opensysml_kernel install --binary ./sysml-jupyter-kernel
```

`python -m jupyter_opensysml_kernel uninstall` removes the kernelspec and the
binary in it; `jupyter kernelspec remove sysml` does the same.

The conda-forge recipe for this package is kept in the OpenSysML repository
under `packaging/conda`; once it is accepted there,
`conda install -c conda-forge jupyter-opensysml-kernel` installs the kernel and
its kernelspec into the environment directly, with no install command. Until
then, `pip install` inside the conda environment registers the kernel under
that environment's prefix.

A development snapshot of this package is published every night as
`jupyter-opensysml-kernel==<next release>.dev<yyyymmdd>`, which `pip install`
never picks up on its own; it installs the kernel of the same night's snapshot.

## Documentation

- [Using SysML in Jupyter](https://opensysml.org/guide/12-jupyter/) — the guide
  chapter: cells, `%` commands, rich output, interrupts.
- [REPL commands](https://opensysml.org/reference/repl-commands/) — every `%`
  command the kernel serves.

## Development

```bash
pip install -e '.[dev]'
pytest
mypy jupyter_opensysml_kernel
```

The tests need no network: downloads are served from fixtures. The package's
copy of `release-digests.json` is synced from `client/release-digests.json` by
`scripts/sync-release-digests.py`; the release pipeline stamps the digests of
the kernel binaries it built into the copy before the wheel is built.

## License

Apache-2.0.

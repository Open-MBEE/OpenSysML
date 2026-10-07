- **A Jupyter kernel for SysML v2, `sysml-jupyter-kernel`, installable with `pip install
  jupyter-opensysml-kernel`.** The kernel runs notebook cells through the same REPL session
  `sysml` drives, so declarations accumulate across cells, bare expressions evaluate, and
  every `%` command — instantiating, running, debugging, sweeping, rendering — works as at
  the prompt, with the model's state kept between cells. Views render as rich output
  (`text/vnd.mermaid`, Graphviz SVG, Markdown, CSV), `%render-document` as Markdown or, with
  a trailing `html`, as HTML, and `%features … json` as JSON. Tab completion, `Shift-Tab`
  inspection and the console's completeness detection use the REPL's own completer and
  continuation rules; interrupting a cell stops the run at its next step, and later cells
  run on. `python -m jupyter_opensysml_kernel install` registers the `sysml` kernelspec, carrying
  the OpenSysML mark as its icon, with the release's kernel binary, downloaded against the digests the package ships; the binary
  is also a release asset (`sysml-jupyter-kernel-<os>-<arch>`), installable with
  `install.sh --tools sysml-jupyter-kernel`, and `sysml-jupyter-kernel -install` registers
  it by hand. A conda-forge recipe is kept under `packaging/conda`.

# 12. Jupyter notebooks

`sysml-jupyter-kernel` runs SysML v2 in Jupyter notebooks. A notebook cell is read exactly as
the [REPL](04-repl.md) reads what you type: declarations accumulate into one session model,
a bare expression is evaluated, and every `%` command of the prompt — instantiating, running,
debugging, sweeping, checking, rendering — works as it does there, with the model and the
runtime kept from cell to cell. Views and documents come back as rich output the notebook
renders.

The kernel is the REPL's session behind the Jupyter protocol, not a client of the
`sysml-grpc` service: nothing else needs to run, and the cells hold SysML, not Python. To
drive a model *from Python*, use the [`opensysml` client](09-python.md) instead; the two can
share a notebook server.

## Installing

The kernel is a package on PyPI. Its wheel for your platform (Linux x64 and arm64, macOS
Intel and Apple Silicon, Windows x64) carries the release's `sysml-jupyter-kernel` binary
and registers it as a kernelspec named `sysml` under the Python environment it is installed
into, so one command is the whole install:

```bash
pip install jupyter-opensysml-kernel
jupyter kernelspec list        # lists sysml
```

The kernelspec, with the OpenSysML mark as the kernel's icon, lives beside the package
(`<prefix>/share/jupyter/kernels/sysml`) and starts the bundled binary through
`python -m jupyter_opensysml_kernel`, so it goes wherever the package goes:
`pip uninstall` removes both. Run `pip install` with the Python that runs the notebook
server, or in the environment it runs in, and the server sees the kernel.

On a platform without a wheel, `pip` installs from the sdist, which carries no binary; then
`install` downloads the release's binary and checks it against the SHA-256 digest the
package was built with, so a mirror or a tampered download is refused before anything is
written. The same command registers the kernelspec somewhere other than the package's
prefix — for your user, or under another prefix — copying the bundled binary into it when
there is one:

```bash
python -m jupyter_opensysml_kernel install                    # this environment, or the user
python -m jupyter_opensysml_kernel install --sys-prefix       # this environment
python -m jupyter_opensysml_kernel install --user             # ~/.local/share/jupyter
python -m jupyter_opensysml_kernel install --prefix /opt/jupyter
python -m jupyter_opensysml_kernel uninstall                  # removes what install wrote
```

A `sysml-jupyter-kernel` you built yourself (`make build-jupyter-kernel`) or installed with
[`install.sh --tools sysml-jupyter-kernel`](01-install.md) is registered without a download:

```bash
python -m jupyter_opensysml_kernel install --binary bin/sysml-jupyter-kernel
sysml-jupyter-kernel -install              # or let the binary write its own kernelspec
```

With conda, the same package installs from conda-forge once its recipe is accepted there
(the recipe is kept under `packaging/conda`); until then `pip install` into the conda
environment registers the kernel under that environment's prefix the same way.

Then start a notebook server and pick **SysML v2 (OpenSysML)** from the kernel list:

```bash
pip install jupyterlab
jupyter lab
```

The package also carries `jupyterlab-opensysml`, a prebuilt JupyterLab extension, as shared
data (`<prefix>/share/jupyter/labextensions/jupyterlab-opensysml`), where JupyterLab 4 and
Notebook 7 load extensions from without a build step: `jupyter labextension list` shows it
enabled right after `pip install`, with no Node.js and no `jupyter labextension install`.
It highlights SysML v2 and KerML — keywords, comments and `doc` bodies, strings, numbers,
`'unrestricted names'`, `Qualified::Names`, operators — in the cells of a `sysml` notebook,
in `.sysml` and `.kerml` files opened in the editor, and in Markdown code fences tagged
`sysml` or `kerml`; a cell's leading `%command` is marked as the kernel command it is.

## Cells

A cell may hold declarations, `%` commands and expressions, mixed. The kernel splits it as the
prompt does — a `%` command is one line; an expression is answered on its own once its
brackets close; declarations between them go in as one submission, as a file would — and runs
each part in order, stopping at the first that fails.

```sysml
package Vehicles {
  private import ScalarValues::*;
  part def Wheel { attribute diameter : Real; }
  part def Car {
    attribute mass : Real default = 1500.0;
    part wheels : Wheel[4] { attribute :>> diameter = 0.65; }
  }
  part sedan : Car { attribute :>> mass = 1800.0; }
}
```

The next cell sees `Vehicles`; a bare expression evaluates against the model, and the result
is the cell's output:

```sysml
Vehicles::sedan.mass + 100.0
```

```sysml
%instantiate Vehicles::sedan
%features Vehicles::sedan
```

Redeclaring a package replaces the earlier declaration, as `%load` at the prompt does, and
instances built from it are rebuilt on their next use. `%clear` starts the session over;
`%quit` is noted rather than acted on — the notebook's own shutdown ends the kernel.

Everything the prompt prints is the cell's standard output. A declaration that does not
parse, a command that fails, or an expression that cannot be evaluated is an error output
with the prompt's message, and the cells queued behind it (as when running the whole
notebook) are skipped, as notebooks expect.

As at the prompt, a failed cell does not roll the session back: what the session accepted
before the error stays, and a declaration refused for a semantic error is still in the model
with that error reported. `%clear` (or *Restart Kernel*) starts over.

<kbd>Tab</kbd> completes the way the prompt's completer does: `%` commands, their arguments,
and qualified names in the model. <kbd>Shift</kbd>+<kbd>Tab</kbd> on a name shows what
`%print` would print for it. A notebook runs a cell on <kbd>Shift</kbd>+<kbd>Enter</kbd>
whatever it holds; it is `jupyter console` that asks the kernel whether the input is
complete, and there <kbd>Enter</kbd> on an unfinished declaration (an unclosed brace) reads
another line rather than running it.

## Drawing an element

`%viz` draws any named element on demand, with no view declared, in the grammar of the OMG
pilot kernel's `%viz`, so a notebook written for the pilot runs unchanged:

```
%viz [--view=<VIEW>] [--style=<STYLE>...] [<form>] <NAME> [<NAME>...]
```

```
%viz Vehicles::Car
%viz --view Tree --style LR --style ortholine Vehicles::Car Vehicles::Wheel
%viz --view STATE Vehicles::Lamp
```

`VIEW` is `DEFAULT`, `TREE`, `INTERCONNECTION`, `STATE`, `ACTION`, `SEQUENCE`, `MIXED` or `CASE`,
in any letter case. `DEFAULT` — the view when `--view` is absent — chooses the rendering from what
the names resolve to: a state def or usage draws a state diagram, an action def or usage an action
diagram, a case def or usage a case diagram, a part or other structural usage holding a
connection, binding or flow an interconnection diagram, and a definition, a package or a usage
with nothing to connect a tree; names calling for different diagrams draw a mixed one. Several
names draw in one diagram. Names resolve as `%render`'s do: qualified, or simple and in scope.

Each `--style` is a direction (`TB`, `LR`, `RL`, `BT`), a drawing style (`pilot`, `cameo`), a
palette (`okabe-ito`, `viridis`, …) or a port display (`minimal`, `full`). The pilot's other
styles (`ORTHOLINE`, `POLYLINE`, `COMPTREE`, `SHOWINHERITED`, …) are accepted and noted in the
diagram as not drawn, so a pilot notebook runs and nothing is dropped silently; `PUMLCODE` asks for
the PlantUML source, as it does in the pilot. An unknown view or style is refused with the list.

With no form named, a cell shows the diagram — as Mermaid, and as an SVG drawn from DOT when
Graphviz is installed. A form (`text`, `mermaid`, `dot`, `plantuml`, `d2`) shows that form, as
`%render` does. `%viz` is the pilot's spelling of a pseudo-view: `%viz --view STATE P::Lamp` draws
what `%render #state:P::Lamp mermaid` writes, and `%viz P::Car P::Lamp` what a view exposing both
would render. At the `sysml` prompt the same command prints the text rendering.

## Reusing another notebook

Jupyter has no import between notebooks. `%load` has: a path ending in `.ipynb` loads that
notebook's code cells into the session, in notebook order, as if their declarations had been run
here.

```sysml
%load wheels.ipynb
```

```text
loaded wheels.ipynb: 3 of 4 code cells, 2 declarations
  skipped 2 % command lines and 3 expression lines: a loaded notebook declares; its commands are not run and its expressions not evaluated
  skipped cell 4: tagged skip-load
✓ package Wheels
✓ package Cars
```

What is loaded is the model: every code cell's declarations, split as the kernel splits a cell.
Markdown and raw cells are not code. A cell's `%` command lines and its bare expressions are
skipped — a notebook you load must not run its author's `%sweep`, `%save` or `%load`, nor spend
your session evaluating its expressions — and the report counts what it passed over. A cell
tagged `skip-load` (Jupyter's cell tags, in the cell's metadata) is skipped whole; tag the scratch
cells of a notebook others load. The lines of a cell keep their numbers: an error in a loaded cell
is reported as `wheels.ipynb cell 3:2:5` — the notebook, the cell's position among the code
cells, then the line and column within the cell — and `%print` of a loaded name still finds its
text.

To load part of a notebook, name the cells:

```sysml
%load wheels.ipynb --cells 1,3-5
%load wheels.ipynb --cells tag:model
```

`--cells` takes positions among the code cells, counted from 1, as single numbers and ranges, or
`tag:<tag>` for the cells carrying a tag; one `--cells` applies to every notebook the same
`%load` names, and may be written anywhere among the paths (`--cells=tag:model` too). A position
past the notebook's last code cell is refused with the notebook's code-cell count; a tag no cell
carries is refused too.

Loading a notebook again redeclares it, as loading a file again does: what an earlier load of the
whole notebook declared and the notebook no longer holds is gone. Loading picked cells replaces
just those cells and leaves the others as they were.

Only a SysML notebook loads: one whose kernel language (`metadata.kernelspec.language`, else
`metadata.language_info.name`) is `sysml`, or that records none. A Python notebook is refused
with `cannot load analysis.ipynb: a python notebook`; a file that is not nbformat 4 — an older
nbformat 3 notebook, or a file that is no notebook at all — is refused with the reason.

Notebooks load wherever model files do: `%load notebooks/` and `%load '*.ipynb'` pick them up
beside `.sysml` files, `sysml wheels.ipynb` loads one on the command line, and a notebook's
imports are followed to the files beside it as a loaded file's are.

## Rich output

Output that has a richer form than text is sent in that form beside the text, and the
front end shows the richest it can:

| Command | Shown as |
|---|---|
| `%viz <name> [<name>...]` | The diagram: a Mermaid diagram, and an SVG drawing when Graphviz is installed |
| `%viz <form> <name>` | What `%render <view> <form>` shows |
| `%render <view> mermaid` | A Mermaid diagram (JupyterLab 4.1 and later draw it) |
| `%render <view> dot` | An SVG drawing when Graphviz is installed; otherwise the DOT source |
| `%render <view> markdown` | Rendered Markdown |
| `%render <view> csv`, `tsv` | A table, where the front end renders tables |
| `%render-document <doc>` | The document as rendered Markdown |
| `%render-document <doc> html` | The document as HTML, with the table and figure styling of the HTML backend |
| `%features <instance> json` | JSON, shown in the front end's tree view |

`%render <view> plantuml` and `d2` print their source, for a tool that draws them. See
[views and rendering](../manual/outputs.md) for the forms, and the
[REPL commands reference](../reference/repl-commands.md) for every command.

## Interrupting and restarting

**Interrupt** (the stop button, or <kbd>I</kbd> <kbd>I</kbd>) stops the cell: a run the cell
drives — `%continue`, `%step`, a sweep, a solver query — ends at its next step with `KeyboardInterrupt`, and
the next cell runs on with the model unchanged. A declaration being analysed or a file being
read is not interruptible and ends on its own.

**Restart** starts an empty session, as a fresh `sysml` would; **Shutdown** ends the
kernel. Both are the front end's, through the protocol; the kernel also ends on `SIGTERM`.

The bounds the REPL takes from the environment (`OPENSYSML_MAX_STEPS`,
`OPENSYSML_MAX_ACTION_STEPS`, `OPENSYSML_JOBS`, `OPENSYSML_TOOLS` and the rest) apply to the
kernel, read when it starts: set them in the environment of the notebook server. See
[environment variables](../reference/environment.md).

## Troubleshooting

- **The kernel is not listed.** `jupyter kernelspec list` shows what the server sees; the
  spec must be under a path on that list. The wheel registers the kernel under the prefix of
  the `python` that installed it, so install with the Python that runs the notebook server,
  or run `python -m jupyter_opensysml_kernel install --user` (or `--prefix`) to register it
  where the server looks.
- **`pip install` fetched the sdist.** There is no wheel for the platform, so the package
  carries no binary: `python -m jupyter_opensysml_kernel install` downloads the release's
  and registers it, or `--binary` registers one built from source.
- **The install refuses the download.** The package pins the digests of its own release and
  verifies what it fetched against them; a mismatch is reported and nothing is written.
  Behind a mirror, download the release asset by hand, verify it against the release's
  `SHA256SUMS.txt`, and register it with `--binary`.
- **A cell hangs.** Interrupt it. A run runs until it ends or its step budget is spent; set
  `OPENSYSML_MAX_ACTION_STEPS` lower, or use `%step` to drive it a step at a time.
- **Diagrams show as source.** Mermaid is drawn by JupyterLab 4.1 and later, and by Notebook
  7.1 and later; DOT is drawn only where Graphviz is installed on the kernel's machine.
- **Cells are not highlighted.** `jupyter labextension list` must show `jupyterlab-opensysml`
  enabled; it is shared data of the package, so it is found under the prefix of the Python
  that runs JupyterLab or Notebook — install the package with that Python. A notebook server
  started before the install needs a restart, and a browser tab a reload. JupyterLab 3 and
  the classic Notebook do not load JupyterLab 4 extensions.

The protocol the kernel speaks, what `kernel.json` holds, and every option are in the
[Jupyter kernel reference](../reference/jupyter-kernel.md).

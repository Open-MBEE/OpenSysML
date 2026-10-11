# SysML v2 for JupyterLab (OpenSysML)

`jupyterlab-opensysml` is a prebuilt (federated) JupyterLab 4 / Notebook 7 extension that
highlights SysML v2 and KerML. It registers two CodeMirror 6 languages with JupyterLab's
editor language registry — `sysml` (`text/x-sysml`, `.sysml`) and `kerml` (`text/x-kerml`,
`.kerml`) — so the cells of a notebook on the `sysml` kernel, `.sysml` and `.kerml` files
opened in the editor, and Markdown code fences tagged `sysml` or `kerml` are highlighted.

It is not published to npm. The kernel's pip package,
[`jupyter-opensysml-kernel`](../../client/jupyter-kernel), ships the build in every wheel
and the sdist as shared data under `share/jupyter/labextensions/jupyterlab-opensysml`, so
`pip install jupyter-opensysml-kernel` is the whole install: no Node.js, no
`jupyter labextension install`.

## What it highlights

A `StreamLanguage` tokenizer (`src/sysml.ts`) marks reserved keywords, the words that are
keywords only in context (`chain`, `deep`, `done`, `history`, `junction`, `shallow`, and
`var` in KerML), the literals `true`, `false` and `null`, `//` and `/* */` comments, the
bodies of `doc` and `comment` as documentation, strings with their escapes, numbers,
`'unrestricted names'`, the `A::B` of qualified names, operators and punctuation, and a
cell's first token when it is a kernel command (`%help`, `%render`, …).

The keyword and operator tables are `src/syntax.json`, generated from the lexer's keyword
list (`internal/syntax/source.Keywords()`, `lexer.ContextualWords()`) by
`editors/vscode/tools/gengrammar`, the tool that generates the VS Code grammars:

```bash
make vscode-grammar    # regenerates the TextMate grammars and src/syntax.json
```

`go test ./editors/vscode/tools/gengrammar/` fails when the committed table is stale, so the
highlighting cannot drift from the language. Do not edit `src/syntax.json` by hand.

## Build and test

Node.js 22 and the `jupyterlab` Python package (for `jupyter labextension build`) are needed;
end users need neither.

```bash
make jupyterlab-test     # the tokenizer tests over sample snippets (test/sysml.test.ts)
make jupyterlab-build    # client/jupyter-kernel/labextension/, which the pip package ships
```

The build output is not committed. `scripts/build-jupyter-kernel-dist.sh` runs
`make jupyterlab-build` before it builds the sdist and the platform wheels and checks that
all six carry the same bundle; `python -m build` in `client/jupyter-kernel` refuses to build
a distribution without it.

## Development install

`make jupyter-kernel-install` installs the kernel package in editable mode; it does not
build the extension. To see the highlighting in a development environment, build it and let
JupyterLab link the build in place, so a rebuild is picked up on reload:

```bash
make jupyter-kernel-install
make jupyterlab-install        # make jupyterlab-build + jupyter labextension develop --overwrite
jupyter labextension list      # jupyterlab-opensysml ... enabled OK
```

`jupyter labextension develop` symlinks `client/jupyter-kernel/labextension/` into the
environment's `share/jupyter/labextensions/`, which `_jupyter_labextension_paths()` in the
package points it at. Alternatively, `make jupyterlab-build` followed by `pip install
client/jupyter-kernel` (not editable) installs the build as shared data, as a wheel does.

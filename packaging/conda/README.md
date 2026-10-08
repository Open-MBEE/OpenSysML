# conda packaging

`recipe/meta.yaml` is the maintained source of the conda-forge recipe for
`jupyter-opensysml-kernel`, the pip/conda package of the SysML v2 Jupyter kernel. It
carries `__TAG__`, `__VERSION__` and `__SHA256_*__` placeholders and is **not buildable
as-is**; `scripts/render-conda-recipe.sh` fills them from a release's `SHA256SUMS.txt`:

```bash
scripts/render-conda-recipe.sh v0.9.2 > meta.yaml          # fetches the manifest
scripts/render-conda-recipe.sh v0.9.2 dist/SHA256SUMS.txt   # from a local manifest
```

The recipe is per platform rather than `noarch`, because conda-forge packages may not
download anything at install time: each platform's package carries the release's
`sysml-jupyter-kernel` binary for that platform (the same raw asset `pip install` would
download and verify), and the build installs it into the kernelspec under
`$PREFIX/share/jupyter/kernels/sysml` with
`python -m jupyter_opensysml_kernel install --prefix "$PREFIX" --binary ...`. The Python
source is the sdist PyPI serves for the same version, so the two installs are the same
package; both digests are the manifest's.

## Submitting and maintaining

conda-forge takes new packages through
[`conda-forge/staged-recipes`](https://github.com/conda-forge/staged-recipes): the
rendered `meta.yaml` goes into `recipes/jupyter-opensysml-kernel/`, and once merged
conda-forge creates the `jupyter-opensysml-kernel-feedstock` repository, which is then
the place later versions are updated (its bot opens a pull request when it sees a new
PyPI version; the binary digests must be updated in that pull request by hand or by
re-rendering from this template). A release must exist, with the kernel binaries and
the sdist in it, before a version can be submitted.

The package's maintainers listed under `extra.recipe-maintainers` must accept the role
on the staged-recipes pull request.

## Trying the recipe locally

`conda build` (or `rattler-build` after converting with `conda-recipe-manager`) renders
and builds the recipe for the host platform:

```bash
scripts/render-conda-recipe.sh v0.9.2 > /tmp/recipe/meta.yaml
conda build /tmp/recipe
conda create -n sysml-kernel --use-local jupyter-opensysml-kernel jupyterlab
conda run -n sysml-kernel jupyter kernelspec list
```

Until the package is on conda-forge, the pip package installs the same kernel into a
conda environment: `pip install jupyter-opensysml-kernel && python -m
jupyter_opensysml_kernel install` registers it under the active environment's prefix.

# MSYS2 packaging

`PKGBUILD` is the maintained source of a mingw-w64 style
[MSYS2](https://www.msys2.org/) package, `mingw-w64-opensysml`, modelled on the Go packages in
[msys2/MINGW-packages](https://github.com/msys2/MINGW-packages) (`mingw-w64-hugo`,
`mingw-w64-gh`). It carries `__VERSION__` / `__TAGVER__` / `__SHA256_SOURCE__` placeholders (`pkgver` is the tag without `v` and without the pre-release hyphen, `v0.6.0-rc1` → `0.6.0rc1`, which `vercmp` orders before `0.6.0`; `_tagver` keeps `0.6.0-rc1` for the source URL; the renderer only accepts `vX.Y.Z` or `vX.Y.Z-(alpha|beta|rc)N` tags) and is **not
buildable as-is**; `scripts/render-msys2-pkgbuild.sh` substitutes them, following the same
convention as [packaging/homebrew](../homebrew/README.md).

Unlike the Scoop and winget manifests, which install the release zip, the PKGBUILD **builds from
source** with the MinGW Go toolchain (`${MINGW_PACKAGE_PREFIX}-go`) from the GitHub source
archive of the release tag, as MSYS2 requires for all its packages. It installs `sysml.exe`,
`sysml-lsp.exe`, their manual pages and the license into `${MINGW_PREFIX}`, and declares
`depends=("${MINGW_PACKAGE_PREFIX}-z3")` so MSYS2's own Z3 package provides the solver the
experimental `%check`/`%explain` commands look for on `PATH`. The source archive is not listed
in `SHA256SUMS.txt`, so the render script downloads and hashes it; `updpkgsums` on the
rendered file gives the same result. The Go build sets `CGO_ENABLED=0`, as the Makefile's
build targets do, so `-cc` is not a build dependency.

## Rendering

```bash
scripts/render-msys2-pkgbuild.sh v0.5.0 > PKGBUILD                 # fetches the source archive
scripts/render-msys2-pkgbuild.sh v0.5.0 opensysml-0.5.0.tar.gz > PKGBUILD
```

The script depends on nothing in this repository except the template it reads, so it works when
fetched standalone at a tag.

## Validating

`makepkg-mingw` needs an MSYS2 shell. From a `MINGW64`/`UCRT64` shell:

```bash
pacman -S --needed base-devel ${MINGW_PACKAGE_PREFIX}-go
mkdir mingw-w64-opensysml && cp PKGBUILD mingw-w64-opensysml/ && cd mingw-w64-opensysml
MINGW_ARCH=ucrt64 makepkg-mingw -sCLf
pacman -U mingw-w64-ucrt-x86_64-opensysml-*.pkg.tar.zst
sysml -version && which z3
```

Offline, on Linux, `bash -n PKGBUILD` checks the syntax and `namcap PKGBUILD` (Arch's PKGBUILD
linter) the metadata; neither builds the package.

## Submitting (maintainer procedure, done by hand)

Nothing in this repository writes to `msys2/MINGW-packages`, and no CI job does either. A
maintainer submits the rendered PKGBUILD:

1. Render for the latest release and build/install it as above; the package must build on every
   entry in `mingw_arch`.
2. Fork [msys2/MINGW-packages](https://github.com/msys2/MINGW-packages), add
   `mingw-w64-opensysml/PKGBUILD` (with a real `# Maintainer:` line — the project uses the
   issue tracker URL, MSYS2 prefers a person), and open a pull request titled
   `opensysml: new package`. Their CI builds it for every architecture; read the repository's
   contribution guidelines and the
   [package guidelines](https://www.msys2.org/dev/new-package/) first.
3. Later releases bump `pkgver`, reset `pkgrel=1` and update `sha256sums` (`updpkgsums`), or
   render again.

Once accepted, `pacman -S ${MINGW_PACKAGE_PREFIX}-opensysml` can be added to
`docs/guide/01-install.md`.

---
name: testing-man-pages
description: How to end-to-end test the generated manual pages and GNU-style installation (internal/frontend/usage, `<cmd> -man`, `make man` / `make man-check` / `make install-tree`, packaging/man/man1/*.1) on Linux — proving the drift gate is load-bearing, that `man` really renders an installed page, and that rendering is reproducible.
---

# Verifying the generated manual pages and `make install-tree` (Linux)

Each command (`sysml`, `sysml-lsp`, `sysml-grpc`) declares a `usage.Doc` in its `usage.go`;
`internal/frontend/usage` renders both the terminal help (`-help`) and the roff page (`-man`) from it, and
options are enumerated from the command's `flag.FlagSet`. The pages are **committed** under
`packaging/man/man1/*.1` and gated by `make man-check`.

## Provisioning (neither is preinstalled by the blueprint)

```bash
sudo apt-get install -y man-db mandoc      # passwordless sudo works on this box
```

- `/usr/bin/man` on a fresh box is the Ubuntu **"This system has been minimized"** stub: it
  prints a notice and exits 0. Without `man-db`, `man sysml` looks like a page that renders to
  nothing and `man -w sysml` prints no path — a false negative, not a bug in the pages.
- `make man-check` lints with `mandoc -T lint -W warning` if present, else falls back to
  `groff -man -Tutf8 -ww -z`, else prints a *note and passes*. So on a box with neither, the
  lint arm is silently vacuous — always confirm `mandoc` is installed before claiming lint
  coverage (CI installs it explicitly).

## Prove the drift gate is load-bearing (two independent mutations)

`make man-check` runs `TestTheShippedManualPage…` / `TestTheManualPage…` in the three `cmd/`
packages plus the formatter lint. Both of these must FAIL, and `make man` must repair them:

```bash
# 1. mutate a committed page
sed -i 's/SysML v2 and KerML models/SysML models/' packaging/man/man1/sysml.1
make man-check   # → packaging/man/man1/sysml.1 is not what sysml -man now writes; run make man  (make Error 1)
make man && git diff --stat man/   # empty: regeneration is byte-for-byte

# 2. add a flag without regenerating
#    insert fs.Bool("throwaway-flag", …) into cmd/sysml-lsp/usage.go registerFlags
make man-check   # → fails naming packaging/man/man1/sysml-lsp.1
make man && grep throwaway packaging/man/man1/sysml-lsp.1   # → .B \-throwaway\-flag
git checkout cmd/sysml-lsp/usage.go && make man && git status --short   # must end empty
```

Always finish with `git status --short` empty — `make man` rewrites tracked files, so a run that
forgets the final regeneration leaves the branch dirty.

## Help/page agreement (the assertion that actually discriminates)

Compare the *sets* of flags, not spot checks. In the page a hyphen is escaped, and options are
emitted as `.B \-flag` or `.BI \-flag " value"`:

```bash
./bin/$c -help 2>&1 | grep -oE '^  -[a-zA-Z-]+' | tr -d ' ' | sort -u
grep -oE '^\.BI? \\-[a-zA-Z\\-]+' packaging/man/man1/$c.1 | sed -E 's/^\.BI? //; s/\\//g' | sort -u
```

Expected today: `sysml` 34 flags, `sysml-lsp` 7, `sysml-grpc` 12, `diff` silent for each.
Also check `<cmd> -man` exits 0 and writes **0 bytes to stderr** (it is piped into a file by
`make man`, so any stderr chatter or nonzero status would corrupt the committed page).

## Reproducibility

`internal/frontend/usage/meta.go` reads `SOURCE_DATE_EPOCH` (seconds) and otherwise uses a constant
`manDate`. So:

```bash
SOURCE_DATE_EPOCH=1700000000 go run ./cmd/sysml -man > /tmp/a; go run ./cmd/sysml -man > /tmp/b
diff /tmp/a /tmp/b   # exactly one hunk: the .TH line (2023-11-14 vs the constant date)
```

Two renders with the same epoch must be byte-identical (`cmp`) for all three commands.

## `make install-tree` (DESTDIR/prefix/bindir/mandir…)

```bash
make install-tree DESTDIR=/tmp/it prefix=/usr
find /tmp/it -type f -printf '%m %p\n' | sort
# exactly 6 files: 755 usr/bin/{sysml,sysml-lsp,sysml-grpc}, 644 usr/share/man/man1/*.1
make install-tree DESTDIR=/tmp/it2 prefix=/opt/x bindir=/opt/x/sbin man1dir=/opt/x/doc/man1
make install-tree prefix=/tmp/tree
MANPATH=/tmp/tree/share/man man sysml   # SYNOPSIS OPTIONS ENVIRONMENT EXIT STATUS SEE ALSO
```

Override the inner variables (`bindir`, `man1dir`) as well as `prefix` — a target that only
honours `prefix` passes the first check and fails the second.

**Known trap:** the recipe passes `$(DESTDIR)$(bindir)` unquoted to `install`, so a staging path
containing a space splits into words: `make install-tree DESTDIR="/tmp/stage with space"` dies
with ``install: target 'space/usr/bin/sysml' is not a directory`` (make Error 1) **and creates
stray `with/` and `space/` directories inside the repo working tree** — `git status` does not show
them (empty untracked dirs), so clean them up explicitly after probing. If the recipe has since
been quoted, this case must succeed and stage the 6 files under the spaced path.

# The service

Every Python client operation is answered by `sysml-grpc`. With no service
address configured, `opensysml` starts a private child for the current
interpreter and stops it when the last connection closes or the interpreter
exits. The module-level functions share a lazy connection; explicit
`Connection` objects are caller-owned.

## Binary resolution

When it needs to start a service, the client resolves an executable in this
order:

1. `$OPENSYSML_BINARY`, if set. This explicit path must name an executable;
   otherwise resolution fails instead of silently using another binary.
2. The shared cache at `~/.opensysml/bin/sysml-grpc`
   (`sysml-grpc.exe` on Windows).
3. A download of the requested release, or the release this client was built
   against when no version is configured.
4. The first executable `sysml-grpc` on `$PATH`.

`OPENSYSML_GRPC_VERSION` requests another release. An explicit `version=`
argument to `ensure_binary` does the same. The service binary is shared across
client languages; the cache's metadata records its release and repository so a
stale cache is not mistaken for the requested release. A hand-installed
executable cache without release metadata is preserved.

Downloads are verified against a signed release checksum manifest and the
digest pinned in the Python package. A server-provided `.sha256` sidecar alone
is not a trust anchor. If the release or platform asset is unavailable, a
working cache remains in use; otherwise an executable on `$PATH` may be used
with a warning. If none is available, `ConnectionError` names the release
tags tried and suggests building the service, selecting another release,
installing it on `$PATH`, or connecting to a caller-managed service.

A checksum mismatch or invalid manifest signature is an integrity failure and
is never bypassed by the cache or `$PATH` fallback. If no digest is pinned,
the download is refused by default. An explicit
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for any repository)
opts into trusting a same-origin checksum, with a warning.

The digest table is embedded in the Python package as
`opensysml/release-digests.json`, a synced copy of `client/release-digests.json`.
A pin in the checkout alone would not protect an installed wheel. Cache
metadata records the repository, tag and digest alongside the executable so a
same-tag binary from a different repository or release cannot be mistaken for
the requested one. The cache decision and replacement use a shared file lock;
the executable is started through a digest-named link created while that lock
is held, so another client cannot replace it between resolution and startup.

At release time, the repository's pinning script hashes service assets and
checks that any release `.sha256` sidecar agrees. The signed checksum manifest
is verified independently. `--check` re-hashes each pinned release so a
republished asset that differs from its pin is detected.

A binary supplied through `$OPENSYSML_BINARY` or `$PATH` is started as found:
it is not copied into the shared cache and is not checked against pinned
release digests. Naming it or installing it on `$PATH` is the operator's
trust decision. `OPENSYSML_GITHUB_REPO` changes the release repository from
the default `Open-MBEE/OpenSysML`.

## Service ownership

A connection without a service address starts a private child. The child asks
the kernel for a free port and reports its bound address on stdout; the client
does not choose, probe or retry a port. Connections in one interpreter that
need the same service release share one child and its model-parse cache.

Connecting to a service the client did not start is explicit: pass a host and
port, set `$OPENSYSML_SERVICE=host:port`, or pass `auto_start=False` to require
a caller-managed service. The client never stops such a service. A managed
service is checked with `GetServerInfo` against any requested release and
capabilities, but is neither replaced nor stopped to satisfy the check.

### No orphans

The client holds the write end of the child's stdin pipe and never writes to
it. The child reads stdin and exits at end-of-file, so the operating system
closes the pipe when its owner exits, including on `SIGKILL`, `os._exit`, a
fatal interpreter error or a crash during shutdown. On orderly close, the
client closes stdin and signals only the child it started through its
`Popen` object.

On Linux and macOS, the child has its own process session, so a signal to the
client's process group does not reach it. On Windows, process exit closes the
same anonymous pipe. After `fork()`, an `os.register_at_fork` hook closes the
forked process's inherited pipe and disowns the parent's child; the forked
process starts its own service if needed.

### Cost of a private child

Measured on Linux with `client/python/scripts/measure_private_service.py`
(\(n=20\)):

| Operation | p50 | p95 |
| --- | ---: | ---: |
| First connection: spawn, bind, report and handshake | 7.0 ms | 9.1 ms |
| Later connection joining the interpreter's child | 0.6 ms | 1.0 ms |
| Child per connection, rather than one shared | 29.6 ms | 54.6 ms |
| Parsing a model already in the shared child's cache | 0.3 ms | 1.2 ms |
| Same parse in a new child for that connection | 139.8 ms | 269.6 ms |

The shared child avoids repeated startup and parsing. Model instances also
carry their model hash, so calls on a loaded `Model` do not need to reload the
source merely to address that model.

## Latency and real-time behavior

The repository benchmark uses an 8-core x86-64 Linux machine, loopback gRPC,
20 part definitions (808 bytes) and 200 iterations with a warm `Connection`:

| Operation | p50 | p95 | p99 |
| --- | ---: | ---: | ---: |
| `load` / `load_from_content`, cache miss | 35 ms | 56 ms | 60 ms |
| `load` / `load_from_content`, cache hit | 0.5 ms | 1.0 ms | 1.0 ms |
| `eval("2 + 2")` on a cached model | 0.4 ms | 1.0 ms | 1.2 ms |
| `convert` SysML to SysML | 0.7 ms | 1.2 ms | 2.0 ms |
| `convert` SysML to Turtle | 1.1 ms | 1.4 ms | 1.5 ms |
| `convert` Turtle to SysML | 1.2 ms | 1.5 ms | 2.7 ms |

Reproduce the measurements with:

```bash
make build-grpc
bin/sysml-grpc
python client/python/scripts/bench_latency.py --iterations 200
```

Parsing costs more than querying a parsed result because parsing loads the
standard library into a fresh symbol index and runs semantic passes. Reuse a
`Connection`, parse once, query through the returned model, and batch related
requests. The service cache holds 100 models and evicts the least-recently
used; converting reparses its input rather than using the `load` cache.

This is a request/response service, not a hard real-time engine. Runtime step
and time budgets cap execution duration but do not promise a deadline. Latency
tails depend on garbage collection, scheduling, TCP and model size. Treat the
reported p99 as a soft budget, measure the workload's own model sizes and
concurrency, and keep per-sample filtering or windowing in the calling
process rather than making one RPC per sample.

# opensysml

Python client for OpenSysML: parse, inspect and execute SysML v2 models over the
`sysml-grpc` service.

```bash
pip install opensysml             # from PyPI
pip install -e clients/python/          # or from a checkout, at the repository root
```

```python
import opensysml

model = opensysml.load("model.sysml", strict=True)   # raises on error diagnostics
print(model.eval("1 + 2 * 3"))                     # 7

print(model.eval("mass", subject="Demo::sedan"))   # 1200.0 — that object, not the default
                                                   # requires the service's evaluate_subject
                                                   # capability; the client checks it first

vehicle = model["Vehicle"]                         # by short name or FQN
vehicle.attributes()                               # own and inherited, with resolved facts
inst = model.instantiate("Demo::Vehicle")
inst.mass                                          # 1500.0 [kg] — a Quantity

model.verify_satisfaction()                        # every assert satisfy … by …
model.save("model.ttl")                            # RDF Turtle (experimental)
```

Declarations can be authored from notation strings while preserving the
untouched source:

```python
model.edit().add_part_def("", "Vehicle").apply()
model.edit().add_part("Vehicle", "engine", type="Engine").apply()
```

Use `opensysml.loads(text, language="kerml")` for inline KerML content.

Every call goes through the `sysml-grpc` service, which `opensysml` starts automatically from
the first place it finds one; the guide below describes how to install it there.

## Resolving the service binary

Every client resolves the service binary in the same order, and this one is no
exception:

1. **`$OPENSYSML_BINARY`**, when set and non-empty: exactly that path is started.
   If it names something that is not an executable file, that is an error naming
   the variable and the path — an explicit instruction is never a fallback.
2. **The shared cache** `~/.opensysml/bin/sysml-grpc` (`sysml-grpc.exe` on
   Windows), where a verified download puts it and where `make build` can put
   your own build.
3. **A release download** into that cache, when a release is asked for by
   `ensure_binary(version=...)` or `$OPENSYSML_GRPC_VERSION`. A download that was
   asked for and failed is an error, not a reason to try `$PATH`, whose binary is
   of no known release.
4. **`$PATH`**: the first executable `sysml-grpc` (`sysml-grpc.exe` on Windows)
   on it, which is what a package manager or `go install` leaves behind.

With none of those, the error lists everywhere it looked and what would fix it.

A binary from `$OPENSYSML_BINARY` or `$PATH` is used exactly as it is found: it
belongs to no release, so **it is not verified against the pinned digests below**,
not copied into the shared cache, and started at its own path rather than a
digest-named link. Naming it, or installing it on `$PATH`, is trusting it; the
pinned-digest trust model covers downloads only.

## Service ownership

`opensysml` uses a service of its own, and never stops a service it did not
start.

- A connection made without naming a service starts a **private child** of this
  interpreter. The child binds port 0, so the kernel assigns the port, and it
  reports the address it was given on its stdout — no port is chosen, probed or
  retried by the client, and two interpreters starting at once cannot collide.
- One private child serves **every** connection of an interpreter that needs the
  same service release. The first of them starts it; it stops when the last one
  closes, or when the interpreter exits. Sharing it shares its parse cache,
  which is what makes a second connection cheap (see below).
- Its lifetime is this interpreter's. Nothing is recorded on disk about it, no
  other process adopts it, and a service another process left listening is
  neither reused nor cleaned up.
- Connecting to a service `opensysml` did not start is explicit: pass a host and
  port (`opensysml.connect("localhost", 50051)`, or `connect("localhost:50051")`),
  set `$OPENSYSML_SERVICE=host:port`, or pass `auto_start=False` to require a
  service the caller manages. Closing such a connection leaves it running.

### No orphans

The client holds the write end of the child's **stdin pipe** and never writes to
it; the child reads its stdin and shuts down on end of file. Nothing else holds
that write end, so the pipe closes when the owning process goes away — and it is
the kernel that closes it, not any code of ours. That survives what an `atexit`
hook or a supervisor thread does not: `SIGKILL`, `os._exit`, a fatal interpreter
error, and a crash during shutdown. On an orderly close the client also closes
stdin itself and then signals the child, so exit is prompt rather than eventual.

The client signals only through the `Popen` object of the child it started, so no
pid it did not start — including one the operating system has since reused —
can be signalled. That guarantee no longer needs a start-time check to hold,
because there is no pid on disk to re-authenticate.

Per platform:

- **Linux** and **macOS**: the child is started in a session of its own
  (`start_new_session=True`), so a `SIGINT` or `SIGHUP` sent to the client's
  process group does not reach it; stdin is what ends it. Other children the
  client spawns do not inherit the write end, since CPython closes descriptors
  across `subprocess` by default, so it has exactly one holder.
- **Windows**: the operating system closes the same anonymous pipe when the
  owning process exits, however it exits, so the guarantee is unchanged. Windows
  has no `fork()`, so the case below cannot arise there.
- **`fork()`**: the forked child inherits the write end, which would hold the
  service open past its owner. An `os.register_at_fork` hook therefore disowns
  the inherited services in the new process and closes its copy of the pipe: the
  service stays tied to the process that started it, and a forked child that
  connects starts one of its own.

The single limitation is deliberate: a service reached explicitly is not tied to
the client's lifetime, because the client does not own it.

### Cost of a private child

Measured on Linux with `clients/python/scripts/measure_private_service.py` (n=20):

|                                                       |     p50 |     p95 |
| ----------------------------------------------------- | ------: | ------: |
| first connection: spawn, bind, report, handshake      |  7.0 ms |  9.1 ms |
| a later connection joining this interpreter's child   |  0.6 ms |  1.0 ms |
| a child per connection, rather than one shared        | 29.6 ms | 54.6 ms |
| parsing a model the shared child has already parsed   |  0.3 ms |  1.2 ms |
| the same parse in a child of that connection's own    | 139.8 ms | 269.6 ms |

The last two rows are why the child is per interpreter rather than per
connection: a child per connection would not only spawn N times, it would parse
each model N times, against a cache hit some 500x cheaper.

## Pinned release digests

A download is verified against the table in `clients/release-digests.json`, which
pins the SHA-256 of every asset of a release and is the one table every client
verifies against; `opensysml` ships its own synced copy of it as
`opensysml/release-digests.json` and reads it as `binary.PINNED_SHA256`, because a
pin resolved from outside the published wheel would not be a pin. The `.sha256` served beside a
binary comes from whoever served the binary, so it detects corruption but not a
republished release; a pinned digest is independent of that origin. A download
with no pin fails with a message naming the version, rather than falling back to
the served checksum — `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for
any repository) accepts same-origin trust explicitly for what it names, with a
warning.

The cache at `~/.opensysml/bin/sysml-grpc` is shared with the other clients, so
deciding whether it is the release asked for and replacing the binary and its
metadata is done holding `~/.opensysml/bin/sysml-grpc.lock` (`fcntl.lockf`, the
same lock a Java `FileLock` and the Rust client take). Concurrent installers
therefore queue rather than pair one release's bytes with another's record. Because that lock is held
over the download, every response is read bounded — 512 MiB for a binary, 8 MiB
for a checksum, manifest, bundle or release JSON — so an endless body is refused
rather than filling memory while other clients wait.

What a service is started from is not that shared path but a link to it under its
own digest, `~/.opensysml/bin/sysml-grpc-<first 16 hex of its SHA-256>`, made
while the lock is held: the cache is replaced in place, so starting it after the
lock is dropped could start whatever another client installed in between. The
Java and Rust clients name that file the same way, so the three share it, and a
cache that cannot be hashed or linked is started directly with a warning saying so.

At release time, after the service binaries are published and final:

```bash
export GITHUB_TOKEN=...            # the release API rate-limits unauthenticated calls
python scripts/pin_release_checksums.py --version v0.0.9 --write
git commit -am 'chore(clients): pin release digests for v0.0.9'
```

The script downloads every `sysml-grpc-*` asset of that release, hashes what it
downloaded, refuses the release if a `.sha256` sidecar disagrees with the asset
it describes, rewrites the table in place, and syncs it into every client that
ships a copy (`python3 scripts/sync-release-digests.py`, whose `--check` mode CI
runs so a copy cannot drift). `--check` re-hashes the assets of
every pinned release and fails on any disagreement, catching a release
republished with another binary. A opensysml release therefore pins the service
releases published before it; asking for a newer one needs a newer opensysml (or
the explicit opt-in above), and leaves an already-downloaded binary serving
rather than refusing to start — only a digest that *contradicts* a pin is
treated as tampering and refuses to fall back.

## Version

`opensysml/_version.py` is the only declaration: the packaging metadata reads it,
`opensysml.__version__` reports the installed distribution's version, and
`scripts/check_version.py` fails a release whose tag names another version. The
version tests therefore require the tree under test to be the installed
distribution — `pip install -e clients/python/`. A wheel of another version installed
beside the source tree makes them fail with that remedy: the artifact is what is
stale, not the declaration.

## Generated typed classes

`python -m opensysml.generate model.sysml -o model_types.py` emits one class per
SysML *definition*, and the generated hierarchy follows the model's
generalization edges: `specializes`, `subsets` and `redefines` all become base
classes, because Python has a single notion of inheritance. What tells the two
apart is the members, not the bases:

- a **redefinition** reuses the redefined feature's name, so its property
  overrides the base class's property of that name, and takes over the type and
  multiplicity it does not restate (`attribute :>> mass = 2.0;` stays `float`,
  and a redefined `0..*` feature stays a `list[...]`);
- a **subset** under a new name adds a property beside the base class's one, and
  likewise inherits the type and multiplicity it leaves out.

With **multiple supertypes**, bases are emitted in declaration order, a target
named twice appearing once, and Python resolves members left to right by its
usual MRO. A base another declared base already specializes is left implicit —
`Hybrid :> Vehicle, Electric` where `Electric :> Vehicle` emits
`class Hybrid(Electric)`, which Python can linearize and which keeps both
relationships and `Electric`'s properties. Where no order linearizes at all
(two bases specializing a shared pair in opposite orders), rather than emit a
module that fails to import, the generator keeps the bases it can and records
what it left out as a comment on the class, naming the edge:

```python
class Both(One):
    # specializes Demo::Two, left out: Python cannot linearize it with the bases above
```

A base outside the generated model is reported the same way. Both are the model's
hierarchy being wider than Python's, not facts being discarded — the service
reports every edge, and `Symbol.specializations` still carries them all.

**Limitation, unchanged:** only structural usages (`attribute`, `part`, `item`,
`occurrence`, `individual`, `port`, `enum`) become properties. Behavioral and
connector usages — `action`, `state`, `calc`, `constraint`, `requirement`,
`connection`, `flow`, `interface`, `allocation`, `case` — are not instance feature values,
so a generated class has no member for them; reach them through
`model["Demo::Vehicle"]`, `verify_constraint` and `verify_satisfaction`.

## Diagnostics

A `Diagnostic` has `severity`, `message`, `code` and a location (`file`,
`start_line`, `start_column`, `end_line`, `end_column`, or the raw `span`). Branch
on `code`, not on the message text: `"syntax"` for a syntax error, a validation
code such as `"unresolved"` for a finding, `"choice-point"` and
`"guard-unevaluable"` for a run's notes; `""` when the service assigned none.

```python
model = opensysml.load("model.sysml")
unresolved = [d for d in model.diagnostics if d.code == "unresolved"]
```

## Names that shadow builtins

Neither builtin name is a live part of the API any more: the module-level
evaluation function is `opensysml.evaluate`, and the execution error is
`opensysml.ExecutionError`. `opensysml.eval` and `opensysml.errors.RuntimeError`
remain as deprecated aliases that warn on use, out of their modules' `__all__`,
so a star-import binds neither.

```python
import opensysml

opensysml.evaluate("1 + 2", file_path="model.sysml")   # opensysml.eval warns
model.eval("mass", subject="Demo::sedan")            # a method shadows nothing
```

```python
from opensysml import eval          # shadows the builtin in this module — avoid
```

Guidance for this package and for code around it:

- **Call `opensysml.evaluate`.** `opensysml.eval` still works and returns the same
  result, warning `DeprecationWarning`; it goes away in 1.0.0.
- Import the package, not its names, for anything named like a builtin.
- Catch `opensysml.ExecutionError` (or its base `opensysml.OpenSysMLError`), never
  `opensysml.errors.RuntimeError`, which warns and is due for removal.
- Do not name a new public function or exception after a builtin.

0.2.0 therefore publishes `evaluate` as the name to write, with both builtin
names deprecated rather than removed, so code written against 0.1.x keeps
running until 1.0.0.

## Running the tests

```bash
make build                                    # builds bin/sysml-grpc
pip install -e clients/python/ && pip install pytest pytest-mock
python -m pytest clients/python/tests/ -q             # service-backed tests skip
```

Tests that need a service skip when none answers on `localhost:50051` and no
binary is available to spawn one. Where a service *is* provided — as in CI —
export `OPENSYSML_REQUIRE_SERVICE=1`, and its absence fails instead of skipping.

## Documentation

- Using the client:
  [docs/guide/09-clients.md](https://github.com/Open-MBEE/OpenSysML/blob/main/docs/guide/09-clients.md)
  — installing the service binary, loading a model, instances, verification, conversion
  and queries
- The API surface, generated typed classes, latency and the module map:
  [docs/reference/python-api.md](https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/python-api.md)
- Installing from source and running the tests:
  [INSTALL.md](https://github.com/Open-MBEE/OpenSysML/blob/main/clients/python/INSTALL.md)

# Python

`opensysml` is the Python client for parsing, inspecting and executing SysML v2
models through the `sysml-grpc` service. Choose it when you want a Python API,
not a subprocess around the `sysml` command.

## Install

Python 3.10 or newer is required. Install the published package from PyPI:

```bash
python -m pip install opensysml
```

`opensysml` 0.9.1 and earlier do not download the service automatically. Set
`OPENSYSML_GRPC_VERSION=v0.9.1` (or another release tag), or provide a service
binary through `OPENSYSML_BINARY` or `$PATH`.

To work from a repository checkout, run this at its root:

```bash
python -m pip install -e client/python/
```

The PyPI package includes the client code and its release-digest table. It does
not bundle the service executable.

## First model

When no service is configured, the first operation starts a private service
child. If its built-against release is not already cached, the client downloads
that release and verifies it against the digest pinned in the package or, for
a release newer than the package's table, the release's signed checksum
manifest. The cache is shared with the other clients. See [The service](service.md)
for binary resolution and trust details.

```python
import opensysml

source = """\
package Demo {
    part def Vehicle {
        attribute mass default = 1500.0;
    }
    part sedan : Vehicle {
        attribute :>> mass = 1800.0;
    }
}
"""

model = opensysml.loads(source)
if not model.ok:
    for diagnostic in model.diagnostics:
        print(diagnostic)
    raise SystemExit("The model has errors.")

print(model.eval("mass", subject="Demo::sedan"))
```

```text
1800.0
```

`model.ok` is false when any diagnostic has error severity. Inspect
`model.diagnostics` to report warnings and errors, or call
`opensysml.load(path, strict=True)` / `opensysml.loads(text, strict=True)` to
raise `ModelError` when errors are present. A strict-load error retains both
the diagnostics and the partially parsed model.

## Service unavailable

If the built-against release is unavailable, the client keeps a working cache
or tries `sysml-grpc` on `$PATH`. If neither is usable, `ConnectionError`
describes the attempted release and how to fix it, for example:

```text
Could not download the sysml-grpc release ... Looked at: $OPENSYSML_BINARY,
~/.opensysml/bin/sysml-grpc, $PATH.
  fix: build it (`make build-grpc`) and set $OPENSYSML_BINARY to the result, or
       ask for another release by setting $OPENSYSML_GRPC_VERSION ...
```

A missing platform asset or a network outage is an availability failure. It may
use an already-working cache or a binary on `$PATH`; otherwise the connection
fails with the diagnostic above. A checksum mismatch or invalid signature is
an integrity failure, not an availability fallback: the client raises an error
instead of starting unverified release bytes.

To use a build you manage, build from the repository and name it explicitly:

```bash
make build-grpc
export OPENSYSML_BINARY="$PWD/bin/sysml-grpc"
```

An explicitly named binary is used as found and is not checked against release
digests. Alternatively, set `OPENSYSML_GRPC_VERSION` to request another release,
or install `sysml-grpc` on `$PATH`.

## Task guides

- [Models and symbols](models.md)
- [Instances and values](instances.md)
- [Verification and analysis](verification.md)
- [Editing and saving](editing-and-saving.md)
- [Queries and documents](queries.md)
- [Errors](errors.md)
- [The service](service.md)
- [Typed classes](typed-classes.md)
- [Metamodel classes and JSON](metamodel.md)
- [Python API reference](../../reference/python-api.md)

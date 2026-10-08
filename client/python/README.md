# opensysml

`opensysml` is the Python client for the OpenSysML SysML v2 parser, model
analysis and execution service. Load SysML models, inspect symbols and
diagnostics, evaluate values, verify constraints, run behaviors, edit source,
and query model documents from Python.

## Install

Python 3.10 or later is required. Install the published package from PyPI:

```bash
pip install opensysml
```

The runtime dependencies are `grpcio>=1.83.0`, `protobuf>=7.35.1` and
`sigstore>=4.5.0,<5`.

A development snapshot of the client is published every night as
`opensysml==<next release>.dev<yyyymmdd>`, which `pip install opensysml` never
picks up; install one by exact version. It starts the `sysml-grpc` of the same
night's snapshot, whose digests it pins. See [Nightly
snapshots](https://opensysml.opensysml.org/project/nightly/).

## Quickstart

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

## The service

The client talks to a `sysml-grpc` service. You do not have to install or
start one: the first connection starts a private service for the current
interpreter and stops it when the interpreter exits.

To find the service binary, the client checks, in order:

1. `$OPENSYSML_BINARY`, if set.
2. The shared cache at `~/.opensysml/bin/sysml-grpc`.
3. A download of the OpenSysML release this package was built against.
4. A `sysml-grpc` on `$PATH`.

A downloaded binary is verified against a SHA-256 digest shipped inside the
package, so a normal `pip install opensysml` followed by `opensysml.connect()`
needs no environment variables and no extra setup. Downloading another release
is also verified, through its signed checksum manifest; an unverifiable download
is refused rather than trusted.

To use a service you run yourself, pass its address:

```python
model = opensysml.connect("localhost:50051").load("model.sysml")
```

The [service guide](https://opensysml.opensysml.org/clients/python/service/) covers the
cache, offline use, trust configuration and connecting to an external service.

## Documentation

- [Python client guide](https://opensysml.opensysml.org/clients/python/)
- [Models and symbols](https://opensysml.opensysml.org/clients/python/models/)
- [Instances and values](https://opensysml.opensysml.org/clients/python/instances/)
- [Verification and analysis](https://opensysml.opensysml.org/clients/python/verification/)
- [Editing and saving](https://opensysml.opensysml.org/clients/python/editing-and-saving/)
- [Queries and documents](https://opensysml.opensysml.org/clients/python/queries/)
- [Errors](https://opensysml.opensysml.org/clients/python/errors/)
- [The service](https://opensysml.opensysml.org/clients/python/service/)
- [Typed classes](https://opensysml.opensysml.org/clients/python/typed-classes/)
- [API reference](https://opensysml.opensysml.org/reference/python-api/)

To install from a checkout, run tests, regenerate protobufs or pin release
digests, see
[DEVELOPING.md](https://github.com/Open-MBEE/OpenSysML/blob/develop/client/python/DEVELOPING.md).

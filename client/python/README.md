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

The [Python client guide](https://opensysml.org/guide/09-clients/) covers the
cache, offline use, trust configuration and connecting to an external service.

## Documentation

- [Python client guide](https://opensysml.org/guide/09-clients/)
- [API reference](https://opensysml.org/reference/python-api/)
- [Metamodel classes and JSON](https://opensysml.org/reference/python-metamodel/)

To install from a checkout, run tests, regenerate protobufs or pin release
digests, see
[DEVELOPING.md](https://github.com/Open-MBEE/OpenSysML/blob/main/client/python/DEVELOPING.md).

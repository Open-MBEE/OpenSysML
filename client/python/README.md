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

## Service resolution

On its first connection, the client starts a `sysml-grpc` service if none was
configured. It checks `$OPENSYSML_BINARY`, the shared cache, downloads the
release it was built against, then checks `$PATH`. Release downloads are
verified against the digest pinned in the package when one is available. For
a release newer than the package's table, the client verifies the release's
signed checksum manifest and uses its digest. A manifest that disagrees with
a package pin is an integrity failure. Without a package pin or a digest from
a verifiable signed manifest, the download is refused unless
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` opts into trusting a same-origin checksum.
See the [service guide](https://opensysml.org/clients/python/service/) for
cache ownership, offline behavior, trust configuration and external service
setup.

## Documentation

- [Python client guide](https://opensysml.org/clients/python/)
- [Models and symbols](https://opensysml.org/clients/python/models/)
- [Instances and values](https://opensysml.org/clients/python/instances/)
- [Verification and analysis](https://opensysml.org/clients/python/verification/)
- [Editing and saving](https://opensysml.org/clients/python/editing-and-saving/)
- [Queries and documents](https://opensysml.org/clients/python/queries/)
- [Errors](https://opensysml.org/clients/python/errors/)
- [The service](https://opensysml.org/clients/python/service/)
- [Typed classes](https://opensysml.org/clients/python/typed-classes/)
- [API reference](https://opensysml.org/reference/python-api/)

To install from a checkout, run tests, regenerate protobufs or pin release
digests, see
[DEVELOPING.md](https://github.com/Open-MBEE/OpenSysML/blob/develop/client/python/DEVELOPING.md).

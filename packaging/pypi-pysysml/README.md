# pysysml has been renamed to opensysml

This distribution is a placeholder. It holds the `pysysml` name on PyPI so that
installing it reports the rename rather than silently resolving to 0.2.0, the
last release that contains the client.

```bash
pip uninstall pysysml
pip install opensysml
```

Then import the new name:

```python
import opensysml

with opensysml.connect() as client:
    model = client.load("model.sysml")
```

Importing `pysysml` from this version raises `ImportError` with the same
instructions. It does not re-export `opensysml`: a working alias would be a
compatibility shim, and the project renamed rather than aliased.

## What changed with the name

| Old | New |
| --- | --- |
| `pip install pysysml`, `import pysysml` | `pip install opensysml`, `import opensysml` |
| `PYSYSML_*` environment variables | `OPENSYSML_*` |
| `~/.pysysml` state directory | `~/.opensysml` |
| `PySysMLError` | `OpenSysMLError` |
| `pysysml-generate` | `opensysml-generate` |
| `pysysml-v*` release tag | the core's `v*` tag, at the core's version |

The API is otherwise unchanged, as is the `sysml-grpc` service binary and its
wire protocol, so a new client talks to an already-installed service. A
`~/.pysysml` left behind by an old install is never read again and can be
deleted.

Pinning `pysysml==0.2.0` keeps the last real release working, but nothing further
will be published under this name.

- Project: https://github.com/Open-MBEE/OpenSysML
- Package: https://pypi.org/project/opensysml/

# Installing opensysml

## From PyPI

```bash
pip install opensysml
```

Published to [PyPI](https://pypi.org/project/opensysml/) from CircleCI by the core `v*`
release tag, at the core's version: `v0.9.0` publishes `opensysml` 0.9.0. The package
downloads the matching `sysml-grpc` service at runtime, defaulting to the
release it was built against. `OPENSYSML_GRPC_VERSION` (or `version=`)
selects another release:

```bash
pip install opensysml==0.9.0
```

The download is verified against a SHA-256 digest the wheel ships: a release of
`opensysml` is built after that release's `sysml-grpc` binaries and pins their
digests, so installing its own release needs no environment variable and no
`sigstore` at run time. Another release is verified against its signed
`SHA256SUMS.txt` with the `sigstore` dependency, and refused — never downloaded
unverified — when that package is missing (the error says what to install). See
[Pinned release digests](README.md#pinned-release-digests).

See [docs/project/releasing.md](../../docs/project/releasing.md#releasing-opensysml-to-pypi).

## From source

From the repository root:

```bash
# Install in development mode (editable)
pip install -e client/python/

# Or install with dev dependencies
pip install -e "client/python/[dev]"
```

## Running tests

Install with `[dev]`: the lifecycle tests inspect processes through `psutil`,
which the package itself does not need.

From the repository root:

```bash
# Run all tests
pytest client/python/tests/

# Run with verbose output
pytest -v client/python/tests/

# Run specific test file
pytest client/python/tests/test_connection.py

# Run integration tests (requires the sysml-grpc binary)
pytest -m integration client/python/tests/
```

A test that connects without naming a service starts a private `sysml-grpc`
child from `~/.opensysml/bin`, so put a built binary there (`make build-grpc &&
cp bin/sysml-grpc ~/.opensysml/bin/`). To run against a service you started
yourself, set `OPENSYSML_SERVICE=host:port`.

## Package structure

```
client/python/
├── opensysml/          # Package source
│   ├── *.py          # Core modules (connection, model, symbol, etc.)
│   ├── proto/        # Generated protobuf stubs
│   └── release-digests.json  # Pinned service digests, synced from client/;
│                             # a released wheel also pins its own release
├── tests/            # Test suite
├── scripts/          # Release helpers (version check, latency measurement)
├── pyproject.toml    # Package metadata and build configuration
├── README.md         # Package documentation
└── INSTALL.md        # This file
```

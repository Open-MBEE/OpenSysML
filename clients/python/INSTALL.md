# Installing opensysml

## From PyPI

```bash
pip install opensysml
```

Published to [PyPI](https://pypi.org/project/opensysml/) from CircleCI on an
`opensysml-v*` tag, independently of the core `v*` releases: the package downloads the
`sysml-grpc` service it needs at runtime, so a newer core release needs no new package. See
[docs/project/releasing.md](../../docs/project/releasing.md#releasing-opensysml-to-pypi).

## From source

From the repository root:

```bash
# Install in development mode (editable)
pip install -e clients/python/

# Or install with dev dependencies
pip install -e "clients/python/[dev]"
```

## Running tests

Install with `[dev]`: the lifecycle tests inspect processes through `psutil`,
which the package itself does not need.

From the repository root:

```bash
# Run all tests
pytest clients/python/tests/

# Run with verbose output
pytest -v clients/python/tests/

# Run specific test file
pytest clients/python/tests/test_connection.py

# Run integration tests (requires the sysml-grpc binary)
pytest -m integration clients/python/tests/
```

A test that connects without naming a service starts a private `sysml-grpc`
child from `~/.opensysml/bin`, so put a built binary there (`make build-grpc &&
cp bin/sysml-grpc ~/.opensysml/bin/`). To run against a service you started
yourself, set `OPENSYSML_SERVICE=host:port`.

## Package structure

```
clients/python/
├── opensysml/          # Package source
│   ├── *.py          # Core modules (connection, model, symbol, etc.)
│   ├── proto/        # Generated protobuf stubs
│   └── release-digests.json  # Pinned service digests, synced from clients/
├── tests/            # Test suite
├── scripts/          # Release helpers (version check, latency measurement)
├── pyproject.toml    # Package metadata and build configuration
├── README.md         # Package documentation
└── INSTALL.md        # This file
```

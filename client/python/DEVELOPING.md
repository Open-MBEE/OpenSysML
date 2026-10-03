# Developing the Python client

The package targets Python 3.10 and later. From the repository root, install
the checkout with its development dependencies:

```bash
pip install -e "client/python/[dev]"
```

To exercise private-service tests, build the service binary with
`make build-grpc`. The client starts a private child when it needs one; tests
that require a service skip when none is available unless
`OPENSYSML_REQUIRE_SERVICE=1` is set.

## Tests

```bash
pytest client/python/tests/
pytest -m integration client/python/tests/
OPENSYSML_REQUIRE_SERVICE=1 pytest client/python/tests/
```

Run the Python API reference coverage test from the package directory:

```bash
cd client/python
pytest tests/test_api_reference.py -q
```

Lifecycle tests inspect child processes through the development-only
`psutil` dependency. It is not required at runtime.

## Generated files

Regenerate the committed generated-class golden file with a running
`sysml-grpc` service:

```bash
python -m opensysml.generate internal/frontend/repl/testdata/vehicle_package.sysml \
    -o client/python/tests/golden/vehicle_types.py
```

Regenerate protobuf bindings from the repository root:

```bash
pip install grpcio-tools
make python-proto
```

## Release digests

The release pipeline stamps a release's own service digests into the package
before building it: `build-python-package` runs
`pin_release_checksums.py --version "$CIRCLE_TAG" --from-binaries dist/grpc
--table client/python/opensysml/release-digests.json` against the binaries
`build-release-binaries` just built, then fails unless the wheel and sdist pin
all five `sysml-grpc-*` assets for the tag. The committed tables are untouched
by that stamp. After the service assets for a release are published, the same
script downloads them, verifies any `.sha256` sidecar and back-fills their
hashes into the shared release-digest table:

```bash
export GITHUB_TOKEN=...
python client/python/scripts/pin_release_checksums.py --version v0.9.1 --write
python3 scripts/sync-release-digests.py --check
```

The token avoids unauthenticated release-API rate limits. The pinning
operation syncs the generated copies used by the clients; `--check` verifies
the copies and re-hashes every pinned release asset. Do not edit the generated
client copies by hand.

## FMI runner

The optional `fmi` extra installs FMPy and the `opensysml-fmi-runner`
executable:

```bash
pip install "client/python/[fmi]"
```

It is the reference runner for the `tool:fmi` engine, which uses the fmi/1
protocol to simulate co-simulation and model-exchange FMUs. Set
`OPENSYSML_FMI_RUNNER` to its executable when needed. See the
[FMI reference](https://opensysml.org/reference/fmi/).

## Version and package layout

`opensysml/_version.py` is the package version declaration;
`opensysml.__version__` reports the installed distribution. The release
version check compares it with the core release tag. Version tests require
the checkout under test to be the installed distribution, for example through
the editable installation above.

```text
client/python/
├── opensysml/          # Client source and generated protobuf stubs
├── tests/              # Unit, service and integration tests
├── scripts/            # Release, benchmark and developer utilities
├── pyproject.toml      # Package metadata and build configuration
└── README.md           # PyPI package documentation
```

For package installation and the user quickstart, see the
[Python client README](README.md).

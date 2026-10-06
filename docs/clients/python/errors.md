# Errors

Failures a Python caller can act on are subclasses of `opensysml.OpenSysMLError`.
The client translates service status codes at its boundary, so applications
usually do not need to import `grpc` or branch on gRPC status values. If a
service error came from an RPC, the original `grpc.RpcError` is available as
`__cause__`.

| Error | Use it for |
| --- | --- |
| `ConnectionError` | service unavailable, startup failure or binary resolution |
| `ChecksumMismatchError` | downloaded bytes contradict a pinned digest |
| `ManifestSignatureError` | the release checksum manifest's signature is invalid |
| `UnpinnedReleaseError` | no digest is pinned for the requested release |
| `SigstoreUnavailableError` | the `sigstore` package (or one it depends on) is missing, so a signed manifest cannot be checked; names the install |
| `StaleServiceError` | an explicitly reached service reports another release |
| `ModelError` | strict loading found error diagnostics |
| `SymbolNotFoundError` | a requested symbol is absent |
| `FeatureValueError` | a feature value could not be evaluated |
| `ExecutionError` | evaluation, execution or verification could not be answered |
| `WrongKindError` | a valid symbol has the wrong kind for the operation |
| `ConversionError` | the model could not be written in the requested format |
| `EditError` | a model edit was refused |
| `QueryError` | a standard query payload is not valid |
| `DocumentQueryError` | a document-query value cannot be represented |
| `MissingCapabilityError` | the connected service does not support a required call |

The error hierarchy also includes `ServiceError` for unrecognized service
failures, `ModelNotFoundError` for an evicted model, `ModelFileNotFoundError`
for a source path the service cannot read, and `UnsupportedValueError` for a
wire value the client cannot represent.

Two failures can both carry `NOT_FOUND` from the service but need different
remedies: an unreadable source raises `ModelFileNotFoundError`; a model hash
evicted from the bounded service cache raises `ModelNotFoundError`.

A false verification verdict is a result, not an exception. A failed request
or a condition that could not be evaluated is represented separately; see
[Verification and analysis](verification.md).

## Names that shadowed builtins

Neither historical builtin-like name is part of the public API any more:
the module-level evaluation function is `opensysml.evaluate`, and the
execution exception is `opensysml.ExecutionError`. Deprecated aliases remain
available and warn when accessed, but they are excluded from `__all__`.

| Deprecated name | Use instead |
| --- | --- |
| `opensysml.eval` | `opensysml.evaluate` |
| `opensysml.errors.RuntimeError` | `opensysml.ExecutionError` |

Import the package rather than star-importing names that might shadow builtins.
The `Model.eval` and `Connection.eval` methods keep their name; object
attributes do not shadow the module-level builtin.

```python
import opensysml

opensysml.evaluate("1 + 2", file_path="model.sysml")
model.eval("mass", subject="Demo::sedan")
```

"""Exception hierarchy for opensysml.

Every failure a caller can act on is a :class:`OpenSysMLError`, including the ones
the service reports over gRPC: :func:`from_rpc_error` translates a status code
into the class for it at the client boundary and keeps the original
``grpc.RpcError`` as ``__cause__``. A script therefore never has to ``import
grpc`` and switch on status codes to tell a missing file from a dead service.
"""

import builtins
import warnings
from contextlib import contextmanager

import grpc


class OpenSysMLError(Exception):
    """Base class for all opensysml errors."""


class ConnectionError(OpenSysMLError, builtins.ConnectionError):
    """Raised when the client cannot connect to or start the sysml-grpc service.

    Also a built-in :class:`ConnectionError`, for the same reason
    :class:`ExecutionError` is a built-in ``RuntimeError``: the name in a
    traceback promises the built-in, and a script reaching a service over a
    socket writes ``except ConnectionError``.

    Attributes:
        message (str): Error description
        code (grpc.StatusCode): Status the call failed with, when the failure
            came from a call rather than from starting the service
    """

    def __init__(self, message, code=None):
        super().__init__(message)
        self.message = message
        self.code = code


class ChecksumMismatchError(ConnectionError):
    """Raised when a downloaded binary does not match the checksum published for it.

    A :class:`ConnectionError`, since the service cannot be started, but its own
    class so that a possibly tampered download is never handled as a transport
    failure and answered from whatever was cached before.

    Attributes:
        unpinned (bool): False for a real mismatch; see
            :class:`UnpinnedReleaseError`, which is the only one it is True for
    """

    #: A digest contradicts another one, which is what this class means.
    unpinned = False


class UnpinnedReleaseError(ChecksumMismatchError):
    """Raised when this opensysml pins no digest for the release being downloaded.

    Nothing contradicts anything here, so calling it a checksum mismatch named
    the wrong cause: the release is simply not one this opensysml vouches for. A
    :class:`ChecksumMismatchError` still, so an ``except`` clause written before
    this class existed keeps catching it, and a working cached binary may still
    be used because no download is under suspicion.
    """

    #: Told apart from a mismatch by class; the flag is kept for older callers.
    unpinned = True


class UnsignedReleaseError(UnpinnedReleaseError):
    """Raised when a release publishes no signature this opensysml can check.

    An old release published before the pipeline signed its checksum manifest,
    one whose bundle asset is missing or unreadable, or an install without the
    ``sigstore`` dependency: no signature was checked, so the release is one
    this opensysml cannot vouch for, exactly as an unpinned one is.
    """


class ManifestSignatureError(ChecksumMismatchError):
    """Raised when the signature on a release's checksum manifest does not verify.

    A bundle was published and read, and it does not attest that this release
    pipeline produced this manifest — another signer, an expired certificate, or
    a manifest changed after signing. That is evidence, not absence, so it is a
    mismatch: no cached binary answers for it.
    """


class StaleServiceError(ConnectionError):
    """Raised when the service already listening is not the one asked for.

    A service this client did not start is reported rather than stopped, since
    the client cannot assume the process is its own.

    Attributes:
        address (str): Address the mismatched service is listening on
        reason (str): How it differs from the service that was asked for
        remedy (str): What to do about it
        info (ServerInfo): What it reported about itself, or None when it could
            not be asked
    """

    def __init__(self, address, reason, remedy, info=None):
        super().__init__(
            f"the sysml-grpc service already listening on {address} is not the "
            f"one this client asked for: {reason}.\n"
            f"  service: {info.describe() if info is not None else address}\n"
            f"  fix:     {remedy}"
        )
        self.address = address
        self.reason = reason
        self.remedy = remedy
        self.info = info


class UnsupportedValueError(OpenSysMLError):
    """Raised when the service sends a value the wire format cannot represent."""


class FeatureValueError(OpenSysMLError):
    """Raised when a feature value could not be evaluated or was never materialized.

    Attributes:
        feature_name (str): Name of the feature
        message (str): Error description reported by the service
    """

    def __init__(self, feature_name, message):
        super().__init__(f"feature value {feature_name!r}: {message}")
        self.feature_name = feature_name
        self.message = message


class TypeMismatchError(OpenSysMLError):
    """Raised when a feature holds a value of another type than its generated view declares.

    Attributes:
        feature_name (str): Name of the feature
        expected (str): Type the generated class declares
        value: The value actually decoded
    """

    def __init__(self, feature_name, expected, value):
        super().__init__(
            f"feature value {feature_name!r}: expected {expected}, got {value!r}"
        )
        self.feature_name = feature_name
        self.expected = expected
        self.value = value


class InstanceTypeError(OpenSysMLError):
    """Raised when a generated typed view is asked to wrap an instance of another type.

    Attributes:
        expected (str): FQN of the definition the generated class views
        actual (str): FQN the instance reports as its type
    """

    def __init__(self, expected, actual):
        super().__init__(
            f"instance of {actual!r} is not a {expected!r}; call the generated "
            f"class's unchecked(instance) to view it without this check"
        )
        self.expected = expected
        self.actual = actual


class ConversionError(OpenSysMLError):
    """Raised when the service could not write a model in the requested format.

    Attributes:
        message (str): Error description reported by the service
        diagnostics (list): Diagnostic objects behind the failure, when the
            source was notation the parser could not read
    """

    def __init__(self, message, diagnostics=None):
        super().__init__(message)
        self.message = message
        self.diagnostics = diagnostics or []


class ExecutionError(OpenSysMLError, builtins.RuntimeError):
    """Raised when a runtime operation (eval/instantiate/execute/verify) fails.

    Also a built-in :class:`RuntimeError`, so ``except RuntimeError`` catches it
    — which is what the traceback's ``opensysml.errors.RuntimeError`` used to
    promise and not deliver. That old name is a deprecated alias of this class.

    Attributes:
        message (str): Error description
        diagnostics (list): List of Diagnostic objects (if available)
    """

    def __init__(self, message, diagnostics=None):
        super().__init__(message)
        self.message = message
        self.diagnostics = diagnostics or []


class WrongKindError(ExecutionError):
    """Raised when a call named an element of another kind than it asks about.

    Verifying ``Wheel`` as a constraint is a wrong request, not an undecided
    verdict about the model, so it raises as naming no element at all does. An
    :class:`ExecutionError`, since that is what such a failure used to be.
    """


class AnalysisRunError(ExecutionError):
    """Raised when an analysis case could not run to its end.

    An unbound subject, an input with no value, a failing step, or an alternative
    a trade study could not evaluate. What the run computed before it failed is
    kept, so the evaluations that did succeed and the objective it left
    undecided stay inspectable. An :class:`ExecutionError`, since that is what
    such a failure used to be.

    Attributes:
        message (str): Error description
        result (AnalysisResult): The outputs, verdicts and evaluations the run
            made before it failed; each verdict is undecided
        diagnostics (list): List of Diagnostic objects (if available)
    """

    def __init__(self, message, result, diagnostics=None):
        super().__init__(message, diagnostics=diagnostics)
        self.result = result


class ModelError(OpenSysMLError):
    """Raised when a model the service parsed has errors and the caller wanted none.

    Raised by ``load(..., strict=True)``, and by :meth:`Model.raise_for_errors`.

    Attributes:
        message (str): Error description naming the source and error count
        diagnostics (list): The error-severity Diagnostic objects
        model (Model): The model itself, so it stays inspectable after the raise
    """

    def __init__(self, message, diagnostics=None, model=None):
        super().__init__(message)
        self.message = message
        self.diagnostics = diagnostics or []
        self.model = model


class SymbolNotFoundError(OpenSysMLError, KeyError):
    """Raised when a symbol a lookup required is not in the model.

    Also a :class:`KeyError`, since ``model["Vehicle"]`` is a subscript and is
    expected to raise one.

    Attributes:
        name (str): Name that was looked up
        suggestions (list[str]): Names in the model close enough to be typos of it
    """

    def __init__(self, name, suggestions=None):
        self.name = name
        self.suggestions = list(suggestions or [])
        message = f"no symbol named {name!r} in this model"
        if self.suggestions:
            message += "; did you mean " + ", ".join(
                repr(s) for s in self.suggestions
            ) + "?"
        super().__init__(message)

    def __str__(self):
        # KeyError.__str__ reprs its argument, which would quote the whole
        # sentence; the message is already written for a reader.
        return self.args[0]


class EditError(OpenSysMLError):
    """Raised when an edit to a model was refused, and nothing was changed.

    The subclasses name the refusals a caller acts on differently. An edit is
    never a silent no-op: every refusal raises one of these.

    Attributes:
        message (str): Why the edit was refused, in the service's wording
        failure (str): Refusal kind, as the wire enum names it, e.g.
            ``'EDIT_FAILURE_UNKNOWN_TARGET'``
        diagnostics (list): Diagnostic objects behind the refusal — the parse
            errors of an unreadable new value, or the errors the edited notation
            was found to have
        referring_elements (list[str]): For a refused rename, where the
            references it would have broken are made
    """

    def __init__(self, message, failure="", diagnostics=None, referring_elements=None):
        super().__init__(message)
        self.message = message
        self.failure = failure
        self.diagnostics = diagnostics or []
        self.referring_elements = list(referring_elements or [])


class NoEditsError(EditError, builtins.ValueError):
    """Raised when an editor with no operations was applied.

    Applying nothing is a mistake in the caller, not an empty write: the model
    is not re-parsed and no file is written.
    """


class EditTargetError(EditError, builtins.LookupError):
    """Raised when the element an edit names cannot carry that edit.

    Covers a target the model does not declare, one declared outside this
    model's own source, an ambiguous one, a target with no value to set, and one
    with no declared name to rename.
    """


class InvalidEditError(EditError, builtins.ValueError):
    """Raised when the new value or name itself cannot be read.

    A value that does not parse as one expression, or a name that does not lex as
    an identifier, is refused before the model is touched.
    """


class RenameReferencedError(EditError):
    """Raised when a rename would break references to the renamed element.

    Renaming a declaration rewrites its name token only, so a referenced element
    cannot be renamed this way. ``referring_elements`` says where the references
    are made.
    """


class OverlappingEditsError(EditError, builtins.ValueError):
    """Raised when two operations would edit the same bytes of the source."""


class EditResultError(EditError):
    """Raised when the edited notation could not be read back.

    The service re-parses and re-analyses what it edited and returns no content
    if the edit introduced an error, so an unreadable model is never written.
    ``diagnostics`` says what the edit broke.
    """


class OwnerNotFoundError(EditError):
    """Raised when an add-member owner is not declared in the model."""


class OwnerNotNamespaceError(EditError):
    """Raised when an add-member owner cannot contain members."""


class IllegalMemberKindError(InvalidEditError):
    """Raised when a declaration kind is invalid for the source language."""


class MemberNameTakenError(EditError):
    """Raised when an owner already declares the requested member name."""


class DeleteReferencedError(EditError):
    """Raised when a referenced declaration is deleted without cascade."""


class ServiceError(OpenSysMLError):
    """Raised when the service fails a call, translated from its gRPC status.

    The subclasses below name the statuses a caller acts on differently; this
    class itself is raised for every other status, so no failure escapes the
    hierarchy as a bare ``grpc.RpcError``. The original is always ``__cause__``.

    Attributes:
        message (str): Description the service reported
        code (grpc.StatusCode): Status code it failed the call with
    """

    def __init__(self, message, code=None):
        super().__init__(message)
        self.message = message
        self.code = code


class ModelNotFoundError(ServiceError):
    """Raised when the service no longer holds the model a call named.

    Its model cache is bounded, so a model loaded long ago and many models back
    may have been evicted. Load it again.
    """


class ModelFileNotFoundError(ServiceError, builtins.FileNotFoundError):
    """Raised when the service could not read the source file a call named.

    Also a built-in :class:`FileNotFoundError`, because that is what the failure
    is: the path does not name a readable file for the service's own process.
    """


class InvalidRequestError(ServiceError, builtins.ValueError):
    """Raised when the service rejects a request as malformed or unsupported.

    Also a :class:`ValueError`: an argument the service cannot accept is a bad
    argument, whether this client or the service caught it.
    """


class ServiceTimeoutError(ServiceError, builtins.TimeoutError):
    """Raised when a call exceeded its deadline or was cancelled."""


class UnsupportedOperationError(ServiceError):
    """Raised when the connected service does not implement the call at all.

    Capability-aware client operations surface a service-side capability
    refusal as :class:`~opensysml.capabilities.MissingCapabilityError`; this is
    the fallback for an unclassified method or an older service.
    """


#: Status codes that name a distinct failure, and the class each becomes.
#: Anything else becomes a plain :class:`ServiceError`, so a status this client
#: has never seen still arrives inside the hierarchy.
_CODE_ERRORS = {
    grpc.StatusCode.INVALID_ARGUMENT: InvalidRequestError,
    grpc.StatusCode.FAILED_PRECONDITION: InvalidRequestError,
    grpc.StatusCode.OUT_OF_RANGE: InvalidRequestError,
    grpc.StatusCode.UNAVAILABLE: ConnectionError,
    grpc.StatusCode.DEADLINE_EXCEEDED: ServiceTimeoutError,
    grpc.StatusCode.CANCELLED: ServiceTimeoutError,
    grpc.StatusCode.UNIMPLEMENTED: UnsupportedOperationError,
}


def _not_found_error(details, default):
    """Pick the class for a NOT_FOUND status from what the service said.

    The service answers NOT_FOUND both for a source file it could not read and
    for a model hash it no longer holds — different problems with different
    fixes. It says which in its details; ``default`` decides for a message this
    client does not recognize, since the call site knows what it named.
    """
    lowered = details.lower()
    if "file not found" in lowered or "no such file" in lowered:
        return ModelFileNotFoundError
    if "model not found" in lowered:
        return ModelNotFoundError
    if "symbol not found" in lowered:
        return SymbolNotFoundError
    return default


def from_rpc_error(exc, not_found=ModelNotFoundError, unimplemented=None):
    """Translate a ``grpc.RpcError`` into the :class:`OpenSysMLError` for it.

    Args:
        exc (grpc.RpcError): The failure as gRPC reported it.
        not_found (type): Class for a NOT_FOUND whose details name neither a
            file nor a model — the call site knows which it asked about.
        unimplemented (callable): Optional function accepting the service
            details and returning a more specific error for this operation.

    Returns:
        ServiceError: The translated error. Raise it ``from exc`` so the status
        code and the gRPC debug string stay reachable as ``__cause__``.
    """
    # A failed call is a grpc.Call as well as a grpc.RpcError, which is where
    # the status lives; an RpcError raised for anything else carries none.
    if isinstance(exc, grpc.Call):
        code = exc.code()
        details = exc.details() or ""
    else:
        code = None
        details = ""

    if code == grpc.StatusCode.UNIMPLEMENTED and unimplemented is not None:
        return unimplemented(details)
    if code == grpc.StatusCode.NOT_FOUND:
        cls = _not_found_error(details, not_found)
    else:
        cls = _CODE_ERRORS.get(code, ServiceError)

    name = code.name if code is not None else "UNKNOWN"
    message = details or f"the sysml-grpc service failed the call with {name}"
    if cls is SymbolNotFoundError:
        # Its constructor takes the name, which the details end with.
        return SymbolNotFoundError(message.rpartition("symbol not found: ")[2])
    if cls is ConnectionError:
        # ConnectionError is also raised for a service that would not start, so
        # a translated one says the service was reached for and was not there.
        return ConnectionError(
            f"sysml-grpc service unavailable: {message}", code=code
        )
    return cls(message, code=code)


@contextmanager
def translate_rpc_errors(not_found=ModelNotFoundError, unimplemented=None):
    """Re-raise any ``grpc.RpcError`` raised in the block as a OpenSysMLError.

    Wraps a stub call at the client boundary, so gRPC's status codes stop at the
    boundary and callers see this package's hierarchy.

    Args:
        not_found (type): Class for a NOT_FOUND the details do not identify;
            pass :class:`ModelFileNotFoundError` where the call named a path.
        unimplemented (callable): Optional function accepting the service
            details and returning a more specific error for this operation.
    """
    try:
        yield
    except grpc.RpcError as exc:
        raise from_rpc_error(
            exc, not_found=not_found, unimplemented=unimplemented
        ) from exc


#: Names kept for callers of older versions, and what they are now.
_DEPRECATED_NAMES = {"RuntimeError": ExecutionError}


def __getattr__(name):
    """Serve the deprecated ``RuntimeError`` alias, warning about its use.

    It is the same class as :class:`ExecutionError`, so ``except
    opensysml.errors.RuntimeError`` keeps working; it is gone from ``__all__`` so
    that a star-import no longer shadows the built-in with it.
    """
    replacement = _DEPRECATED_NAMES.get(name)
    if replacement is None:
        raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
    warnings.warn(
        f"opensysml.errors.{name} is deprecated; use "
        f"opensysml.errors.{replacement.__name__} instead",
        DeprecationWarning,
        stacklevel=2,
    )
    return replacement


__all__ = [
    "OpenSysMLError",
    "ChecksumMismatchError",
    "ConnectionError",
    "ConversionError",
    "ExecutionError",
    "InstanceTypeError",
    "InvalidRequestError",
    "ModelError",
    "ModelFileNotFoundError",
    "ModelNotFoundError",
    "ManifestSignatureError",
    "ServiceError",
    "FeatureValueError",
    "ServiceTimeoutError",
    "StaleServiceError",
    "SymbolNotFoundError",
    "TypeMismatchError",
    "UnpinnedReleaseError",
    "UnsignedReleaseError",
    "UnsupportedOperationError",
    "UnsupportedValueError",
    "WrongKindError",
    "from_rpc_error",
    "translate_rpc_errors",
]

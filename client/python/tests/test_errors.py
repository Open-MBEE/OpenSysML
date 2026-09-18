"""Tests for translating gRPC failures into the opensysml exception hierarchy."""

import builtins
import warnings

import grpc
import pytest

from opensysml import errors
from opensysml.errors import (
    ConnectionError,
    ExecutionError,
    InvalidRequestError,
    ModelFileNotFoundError,
    ModelNotFoundError,
    OpenSysMLError,
    ServiceError,
    ServiceTimeoutError,
    SymbolNotFoundError,
    UnsupportedOperationError,
    from_rpc_error,
    translate_rpc_errors,
)


class FakeRpcError(grpc.RpcError, grpc.Call):
    """A failed call, as gRPC presents one: an RpcError that is also a Call."""

    def __init__(self, code, details=""):
        self._code = code
        self._details = details

    def code(self):
        return self._code

    def details(self):
        return self._details

    def initial_metadata(self):
        return ()

    def trailing_metadata(self):
        return ()

    def is_active(self):
        return False

    def time_remaining(self):
        return None

    def cancel(self):
        return False

    def add_callback(self, callback):
        return False


class TestStatusTranslation:
    @pytest.mark.parametrize(
        "code,expected",
        [
            (grpc.StatusCode.INVALID_ARGUMENT, InvalidRequestError),
            (grpc.StatusCode.FAILED_PRECONDITION, InvalidRequestError),
            (grpc.StatusCode.OUT_OF_RANGE, InvalidRequestError),
            (grpc.StatusCode.UNAVAILABLE, ConnectionError),
            (grpc.StatusCode.DEADLINE_EXCEEDED, ServiceTimeoutError),
            (grpc.StatusCode.CANCELLED, ServiceTimeoutError),
            (grpc.StatusCode.UNIMPLEMENTED, UnsupportedOperationError),
            (grpc.StatusCode.INTERNAL, ServiceError),
            (grpc.StatusCode.DATA_LOSS, ServiceError),
        ],
    )
    def test_each_status_becomes_the_class_for_it(self, code, expected):
        translated = from_rpc_error(FakeRpcError(code, "the service said so"))
        assert isinstance(translated, expected)
        assert isinstance(translated, OpenSysMLError)
        assert "the service said so" in str(translated)

    def test_a_status_without_details_still_says_which_status(self):
        translated = from_rpc_error(FakeRpcError(grpc.StatusCode.INTERNAL))
        assert "INTERNAL" in str(translated)

    def test_an_rpc_error_carrying_no_status_is_still_a_service_error(self):
        translated = from_rpc_error(grpc.RpcError())
        assert type(translated) is ServiceError
        assert translated.code is None

    def test_an_unreadable_file_is_a_file_not_found(self):
        translated = from_rpc_error(
            FakeRpcError(grpc.StatusCode.NOT_FOUND, "file not found: /tmp/nope.sysml")
        )
        assert isinstance(translated, ModelFileNotFoundError)
        # And is what the failure is, so os-level handling catches it.
        assert isinstance(translated, FileNotFoundError)

    def test_an_evicted_model_is_a_model_not_found(self):
        translated = from_rpc_error(
            FakeRpcError(grpc.StatusCode.NOT_FOUND, "model not found: abc123"),
            not_found=ModelFileNotFoundError,
        )
        assert isinstance(translated, ModelNotFoundError)
        assert not isinstance(translated, ModelFileNotFoundError)

    def test_an_unrecognized_not_found_falls_back_to_what_the_caller_named(self):
        translated = from_rpc_error(
            FakeRpcError(grpc.StatusCode.NOT_FOUND, "nothing there"),
            not_found=ModelFileNotFoundError,
        )
        assert isinstance(translated, ModelFileNotFoundError)
        assert isinstance(
            from_rpc_error(FakeRpcError(grpc.StatusCode.NOT_FOUND, "nothing there")),
            ModelNotFoundError,
        )

    def test_the_status_code_stays_reachable(self):
        translated = from_rpc_error(FakeRpcError(grpc.StatusCode.INTERNAL, "boom"))
        assert translated.code == grpc.StatusCode.INTERNAL

    def test_the_original_failure_is_the_cause(self):
        original = FakeRpcError(grpc.StatusCode.UNAVAILABLE, "connection refused")
        with pytest.raises(ConnectionError) as excinfo:
            with translate_rpc_errors():
                raise original
        assert excinfo.value.__cause__ is original
        # The debug string gRPC produces stays reachable through the cause.
        assert excinfo.value.__cause__.code() == grpc.StatusCode.UNAVAILABLE

    def test_a_call_can_specialize_an_unimplemented_status(self):
        original = FakeRpcError(
            grpc.StatusCode.UNIMPLEMENTED, "capability 'query' is unavailable"
        )
        specialized = InvalidRequestError("query is unavailable")
        with pytest.raises(InvalidRequestError) as excinfo:
            with translate_rpc_errors(
                unimplemented=lambda details: specialized
            ):
                raise original
        assert excinfo.value is specialized
        assert excinfo.value.__cause__ is original

    def test_the_block_passes_other_exceptions_through(self):
        translating = translate_rpc_errors()
        with pytest.raises(ZeroDivisionError):
            with translating:
                raise ZeroDivisionError()


class TestConnectionError:
    def test_it_is_catchable_as_the_builtin_connection_error(self):
        assert issubclass(ConnectionError, builtins.ConnectionError)
        assert issubclass(ConnectionError, OpenSysMLError)

    def test_a_translated_one_keeps_the_status_it_came_from(self):
        translated = from_rpc_error(
            FakeRpcError(grpc.StatusCode.UNAVAILABLE, "connection refused")
        )
        assert isinstance(translated, ConnectionError)
        assert translated.code == grpc.StatusCode.UNAVAILABLE
        assert "connection refused" in str(translated)

    def test_one_raised_while_starting_the_service_carries_no_status(self):
        assert ConnectionError("service failed to start").code is None


class TestExecutionError:
    def test_it_is_catchable_as_the_builtin_runtime_error(self):
        with pytest.raises(RuntimeError):
            raise ExecutionError("evaluation failed")

    def test_it_is_a_opensysml_error(self):
        assert issubclass(ExecutionError, OpenSysMLError)

    def test_it_carries_the_diagnostics_behind_the_failure(self):
        exc = ExecutionError("evaluation failed", diagnostics=["d"])
        assert exc.message == "evaluation failed"
        assert exc.diagnostics == ["d"]

    def test_the_old_name_is_a_deprecated_alias(self):
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always")
            alias = errors.RuntimeError
        assert alias is ExecutionError
        assert len(caught) == 1
        assert issubclass(caught[0].category, DeprecationWarning)
        assert "ExecutionError" in str(caught[0].message)

    def test_the_old_name_is_gone_from_the_star_import_surface(self):
        # Which is the point: `from opensysml.errors import *` no longer shadows
        # the built-in RuntimeError.
        assert "RuntimeError" not in errors.__all__
        namespace = {}
        exec("from opensysml.errors import *", namespace)
        assert "RuntimeError" not in namespace

    def test_an_unknown_attribute_is_still_an_attribute_error(self):
        with pytest.raises(AttributeError):
            errors.NoSuchThing


class TestSymbolNotFoundError:
    def test_it_names_the_symbol(self):
        exc = SymbolNotFoundError("Vehicle")
        assert exc.name == "Vehicle"
        assert "'Vehicle'" in str(exc)

    def test_it_offers_near_names(self):
        exc = SymbolNotFoundError("Vehicl", suggestions=["Vehicle"])
        assert exc.suggestions == ["Vehicle"]
        assert "did you mean 'Vehicle'" in str(exc)

    def test_it_is_a_key_error_because_the_lookup_is_a_subscript(self):
        assert isinstance(SymbolNotFoundError("X"), KeyError)


class TestPackageSurface:
    def test_every_exception_is_catchable_from_the_package(self):
        # A caller catches what the package exports; an exception reachable only
        # under opensysml.errors is one a documented failure cannot be caught by.
        import opensysml

        exceptions = {
            name for name in errors.__all__
            if isinstance(getattr(errors, name), type)
            and issubclass(getattr(errors, name), BaseException)
        }
        missing = sorted(
            name for name in exceptions
            if getattr(opensysml, name, None) is not getattr(errors, name)
        )
        assert missing == []
        assert exceptions <= set(opensysml.__all__)

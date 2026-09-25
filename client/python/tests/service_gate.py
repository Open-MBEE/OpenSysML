"""Whether a sysml-grpc service is there, and what its absence means.

A developer without the binary is meant to skip the tests that need a service.
CI builds the binary and starts one, so there an absent service is the failure
these tests exist to catch, not a reason to pass quietly: exporting
``$OPENSYSML_REQUIRE_SERVICE`` turns every such skip into a failure.
"""

import os
import socket

import grpc
import pytest

#: Set where a service is provided, so a missing one fails instead of skipping.
REQUIRE_SERVICE_ENV = 'OPENSYSML_REQUIRE_SERVICE'


def service_required():
    """Whether the caller promised a service, making its absence a failure."""
    return os.environ.get(REQUIRE_SERVICE_ENV, '').strip().lower() not in (
        '', '0', 'false', 'no'
    )


def skip_or_fail_without_service(reason):
    """Skip for a developer with no service; fail where one was promised.

    Args:
        reason (str): What was missing

    Raises:
        Failed: If $OPENSYSML_REQUIRE_SERVICE is set
        Skipped: Otherwise
    """
    if service_required():
        pytest.fail(
            f"${REQUIRE_SERVICE_ENV} is set, so these tests must run against a "
            f"service, but {reason}"
        )
    pytest.skip(reason)


def fail_if_service_promised(available, address='localhost:50051'):
    """Fail collection when a service was promised but none answers.

    Args:
        available (bool): What the health probe found
        address (str): Where a service was looked for

    Raises:
        Failed: If a service was promised and none answered
    """
    if available or not service_required():
        return
    pytest.fail(
        f"${REQUIRE_SERVICE_ENV} is set, so these tests must run against a "
        f"service, but none answers on {address}",
        pytrace=False,
    )


def is_server_available(host='localhost', port=50051, timeout=2):
    """Whether a sysml-grpc service answers a health probe at an address.

    Args:
        host (str): Service hostname
        port (int): Service port
        timeout (float): RPC timeout in seconds

    Returns:
        bool: True when a service answered
    """
    from opensysml.proto import sysml_pb2, sysml_pb2_grpc

    channel = grpc.insecure_channel(f'{host}:{port}')
    try:
        stub = sysml_pb2_grpc.SysMLServiceStub(channel)
        request = sysml_pb2.DiagnosticsRequest(model_hash="health_check")
        stub.GetDiagnostics(request, timeout=timeout)
        return True
    except grpc.RpcError as e:
        # NOT_FOUND is the answer for an unknown hash, so the service is up.
        return e.code() == grpc.StatusCode.NOT_FOUND
    except Exception:
        return False
    finally:
        channel.close()


def service_binary():
    """Path of a sysml-grpc binary to spawn a service from, or None if there is none.

    Returns:
        str or None: An executable binary, from the cache or a local build
    """
    from opensysml.binary import get_binary_path

    repo_build = os.path.join(
        os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))),
        'bin', 'sysml-grpc',
    )
    for path in (get_binary_path(), repo_build):
        if os.path.exists(path) and os.access(path, os.X_OK):
            return path
    return None


def free_port():
    """A port nothing is listening on, for a service of a test's own."""
    with socket.socket() as sock:
        sock.bind(('localhost', 0))
        return sock.getsockname()[1]

"""Tests for the lifecycle of the private service a connection starts.

A connection that is not pointed at a service of somebody else's starts one of
its own: a child on a port the kernel gave it, reachable only by the process
that started it, and dying with it. These tests drive that against a real
binary, including the ways the starting process can die.

Run with: pytest tests/test_lifecycle.py -v
"""

import os
import signal
import subprocess
import sys
import textwrap
import threading
import time

import psutil
import pytest

import opensysml
from opensysml.binary import ensure_binary, get_binary_path
from opensysml.connection import _private_services
from tests.service_gate import (
    service_binary,
    skip_or_fail_without_service,
)


@pytest.fixture
def private_binary(monkeypatch):
    """Make the service this process starts a binary that is actually here."""
    binary = service_binary()
    if binary is None:
        skip_or_fail_without_service("no sysml-grpc binary is available to spawn")
    monkeypatch.delenv("OPENSYSML_SERVICE", raising=False)
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    monkeypatch.setattr("opensysml.connection.ensure_binary", lambda **kwargs: binary)
    started = dict(_private_services)
    yield binary
    # Only what this test started: another test's connection still holds its own.
    for key, service in list(_private_services.items()):
        if started.get(key) is not service:
            service.stop()
            del _private_services[key]


#: What a holder does before the body a test gives it: connect, and say which
#: service that started, so the test can watch for it once the holder is dead.
_HOLDER_PROLOGUE = """\
import os, sys, time
import opensysml
from opensysml import connection

connection.ensure_binary = lambda **kwargs: BINARY
conn = opensysml.connect()
print(conn._private.process.pid, flush=True)
"""


def _holder_script(body):
    """A script that connects in an interpreter of its own and reports the child.

    Args:
        body (str): What it does once it has printed the service's pid

    Returns:
        str: The program to run
    """
    prologue = _HOLDER_PROLOGUE.replace('BINARY', repr(service_binary()))
    return prologue + textwrap.dedent(body)


def _holder(body, env=None):
    """Run a holder script, and return it with the pid of the service it started.

    Returns:
        tuple[subprocess.Popen, psutil.Process]: The holder, and its service
    """
    process = subprocess.Popen(
        [sys.executable, "-c", _holder_script(body)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        env={**os.environ, **(env or {})},
    )
    line = process.stdout.readline()
    if not line:
        process.wait(timeout=10)
        pytest.fail(f"the holder exited with {process.returncode} without connecting")
    return process, psutil.Process(int(line))


@pytest.mark.integration
class TestBinaryIsAvailable:
    """The binary a private service is started from."""

    def test_binary_exists_or_download(self):
        """Verify binary exists or can be downloaded."""
        binary_path = get_binary_path()
        if os.path.exists(binary_path):
            pytest.skip("Binary already exists - manual install detected")

        try:
            result = ensure_binary()
        except Exception as e:
            pytest.skip(f"Binary download not available yet: {e}")
        assert os.path.exists(result)
        assert os.access(result, os.X_OK)


@pytest.mark.integration
class TestPrivateService:
    """What a connection starts for itself, and who else can reach it."""

    def test_it_listens_on_a_port_the_kernel_gave_it(self, private_binary):
        """No fixed port, and none chosen by this client, so none to collide on."""
        with opensysml.connect() as conn:
            assert conn.port != opensysml.DEFAULT_PORT
            # The kernel's ephemeral range, wherever it happens to start.
            assert conn.port > 1024
            assert conn.host in ("127.0.0.1", "::1")
            assert conn.server_info() is not None
            # The address is the one the service reported, not one probed for.
            assert conn._private.address == f"{conn.host}:{conn.port}"

    def test_the_connections_of_one_interpreter_share_one_service(
        self, private_binary
    ):
        """One child per interpreter, so its connections share a parse cache."""
        with opensysml.connect() as first, opensysml.connect() as second:
            assert second.port == first.port
            assert second._private is first._private
            assert first._private.refs == 2

    def test_the_last_connection_released_stops_it(self, private_binary):
        """It lives as long as something in this process is using it."""
        first = opensysml.connect()
        service = psutil.Process(first._private.process.pid)
        second = opensysml.connect()

        first.close()
        assert service.is_running()
        # Closing twice releases one hold, not two.
        first.close()
        assert service.is_running()

        second.close()
        _wait_gone(service)
        assert _is_gone(service)

    def test_threads_opening_and_closing_connections_share_one_service(
        self, private_binary
    ):
        """Concurrent holds are counted, so none of them loses its service."""
        held = opensysml.connect()
        service = held._private
        errors = []

        def churn():
            # Checked rather than asserted: an AssertionError raised here would be
            # caught below as if it were a connection failure.
            try:
                for _ in range(20):
                    with opensysml.connect() as conn:
                        if conn._private is not service:
                            errors.append(AssertionError("a thread got a second service"))
                            return
                        if conn.server_info() is None:
                            errors.append(AssertionError("the service reported no info"))
                            return
            except Exception as failure:  # reported rather than lost in the thread
                errors.append(failure)

        workers = [threading.Thread(target=churn) for _ in range(8)]
        for worker in workers:
            worker.start()
        for worker in workers:
            worker.join(timeout=60)

        assert errors == []
        assert service.refs == 1
        assert held.server_info() is not None
        held.close()

    def test_a_connection_made_after_it_stopped_starts_another(self, private_binary):
        """Nothing outside this process could have been using the stopped one."""
        with opensysml.connect() as conn:
            first = conn._private.process.pid
        with opensysml.connect() as conn:
            assert conn._private.process.pid != first
            assert conn.server_info() is not None

    def test_a_service_that_crashed_is_replaced(self, private_binary):
        """A dead child is started again rather than reported to the caller."""
        conn = opensysml.connect()
        crashed = conn._private.process
        psutil.Process(crashed.pid).kill()
        crashed.wait(timeout=10)

        with opensysml.connect() as replacement:
            started = replacement._private.process
            assert started.pid != crashed.pid
            assert replacement.server_info() is not None
            conn.close()
            # Closing a connection to the crashed one spares its replacement.
            assert replacement.server_info() is not None
        assert started.wait(timeout=10) is not None

    def test_it_records_nothing_outside_this_process(self, private_binary):
        """Nothing outside this process may use it, so nothing is written down."""
        state = os.path.expanduser("~/.opensysml")
        with opensysml.connect():
            written = os.listdir(state) if os.path.isdir(state) else []
            assert not [name for name in written if name.endswith(".pid")]
            assert not [name for name in written if name.endswith(".lock")]

    def test_a_connection_refused_at_connect_time_leaves_nothing_running(
        self, private_binary
    ):
        """A connection never returned holds nothing, so nothing is left behind."""
        with pytest.raises(opensysml.MissingCapabilityError):
            opensysml.connect(require_capabilities=["no-such-capability"])

        assert not _private_services
        assert not _service_processes()

    def test_module_level_calls_share_the_one_service(
        self, private_binary, tmp_path, monkeypatch
    ):
        """The convenience API is one connection, so it is one service."""
        monkeypatch.setattr(opensysml, "_default_connection", None)
        monkeypatch.setattr(opensysml, "_default_connection_params", None)
        first = tmp_path / "one.sysml"
        first.write_text("package One { part P1; }")
        second = tmp_path / "two.sysml"
        second.write_text("package Two { part P2; }")

        try:
            assert opensysml.load(str(first)).hash != opensysml.load(str(second)).hash
            assert len(_service_processes()) == 1
        finally:
            # monkeypatch puts the module back; only the service needs closing.
            opensysml._default_connection.close()


@pytest.mark.integration
class TestNoOrphans:
    """The child may not outlive the process that started it, however it dies.

    The mechanism is a pipe: the child holds its read end as stdin, this process
    holds the write end and never writes to it. Whatever ends this process closes
    every descriptor it held, the child reads end of file and exits — so this
    needs no cleanup to run, which is what makes it survive SIGKILL. Waitable on
    Linux and macOS alike, since both close descriptors on exit; on Windows the
    same holds for the handle a Popen keeps, which the kernel closes with the
    process.
    """

    def test_a_parent_killed_with_sigkill_leaves_no_service(self, private_binary):
        """The case no cleanup path can cover: nothing of the parent runs."""
        holder, service = _holder("time.sleep(60)")
        try:
            holder.send_signal(signal.SIGKILL)
            holder.wait(timeout=10)
            _wait_gone(service)
            assert _is_gone(service)
        finally:
            _kill_if_running(holder, service)

    def test_an_interpreter_that_dies_abnormally_leaves_no_service(
        self, private_binary
    ):
        """os._exit runs no atexit handler, and still ends the service."""
        holder, service = _holder("os._exit(1)")
        try:
            holder.wait(timeout=10)
            assert holder.returncode == 1
            _wait_gone(service)
            assert _is_gone(service)
        finally:
            _kill_if_running(holder, service)

    def test_an_interpreter_that_exits_normally_leaves_no_service(
        self, private_binary
    ):
        """The ordinary exit, where the connection is never closed by hand."""
        holder, service = _holder("sys.exit(0)")
        try:
            holder.wait(timeout=10)
            assert holder.returncode == 0
            _wait_gone(service)
            assert _is_gone(service)
        finally:
            _kill_if_running(holder, service)

    @pytest.mark.skipif(
        not hasattr(os, "fork"), reason="fork() is not available on this platform"
    )
    def test_a_forked_child_neither_stops_nor_inherits_the_service(
        self, private_binary
    ):
        """A fork inherits the pipe but no ownership, and starts its own.

        Its copy of the write end would keep the service alive past its parent's
        death, and closing its connection would stop a service its parent is
        using; it gives up both at the fork.
        """
        holder, service = _holder(
            """\
            pid = os.fork()
            if pid == 0:
                # The fork holds no service, and starts one of its own.
                assert not connection._private_services
                forked = opensysml.connect()
                print(forked._private.process.pid, flush=True)
                forked.close()
                os._exit(0)
            os.waitpid(pid, 0)
            print('parent-survived', flush=True)
            time.sleep(60)
            """
        )
        try:
            forked_pid = int(holder.stdout.readline())
            assert forked_pid != service.pid
            assert holder.stdout.readline().strip() == b"parent-survived"
            # The fork's exit left its parent's service running.
            assert service.is_running()
            assert not psutil.pid_exists(forked_pid) or not _is_service(forked_pid)

            holder.send_signal(signal.SIGKILL)
            holder.wait(timeout=10)
            _wait_gone(service)
            assert _is_gone(service)
        finally:
            _kill_if_running(holder, service)

    def test_the_service_exits_when_the_pipe_it_watches_closes(self, private_binary):
        """The mechanism itself: end of file on stdin is the signal to stop."""
        with opensysml.connect() as conn:
            process = conn._private.process
            process.stdin.close()
            assert process.wait(timeout=10) is not None

    def test_a_disowned_service_is_never_signalled(self, private_binary):
        """A service given up at a fork is left to end on its own.

        Only this Popen's own child is ever signalled, and only while this
        process owns it, so no pid the operating system reused can be.
        """
        with opensysml.connect() as conn:
            service = conn._private
            service.disown()
            service.stop()
            # Ended by the pipe closing, not by a signal from here.
            assert service.process.wait(timeout=10) == 0


def _service_processes():
    """The service children of this process that are still running."""
    return [
        child for child in psutil.Process().children()
        if _is_service(child.pid)
    ]


def _is_service(pid):
    """Whether a pid is a sysml-grpc, for a check that must not guess."""
    try:
        return "sysml-grpc" in " ".join(psutil.Process(pid).cmdline())
    except psutil.Error:
        return False


def _kill_if_running(holder, service):
    """Clean up after a failing test, so no port or process is left held."""
    if holder.poll() is None:
        holder.kill()
        holder.wait(timeout=10)
    if not _is_gone(service):
        service.kill()
        _wait_gone(service)


def _is_gone(process):
    """Whether a process has exited, counting one nobody has reaped yet.

    A service whose parent was killed is reparented, and stays a zombie until
    whatever inherited it reaps it; it is not running either way.

    Args:
        process (psutil.Process): The process to look at

    Returns:
        bool: True when it is no longer executing
    """
    try:
        return not process.is_running() or process.status() == psutil.STATUS_ZOMBIE
    except psutil.NoSuchProcess:
        return True


def _wait_gone(process, timeout=10):
    """Wait for a process to exit, so an assertion is not made on the race."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and not _is_gone(process):
        time.sleep(0.05)

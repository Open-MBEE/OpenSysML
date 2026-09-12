"""Connection class for communicating with sysml-grpc service."""

import atexit
import grpc
import os
import queue
import subprocess
import threading
import warnings
from collections import deque
from typing import Deque, Dict, Optional
from opensysml.proto import sysml_pb2, sysml_pb2_grpc
from opensysml.model import Model
from opensysml.binary import ensure_binary, resolve_latest_version
from opensysml.capabilities import (
    CAPABILITY_APPLY_EDITS,
    CAPABILITY_AUTHORING, CAPABILITY_INLINE_LANGUAGE,
    CAPABILITY_STRICT_CONFORMANCE,
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_CONVERT,
    CAPABILITY_DOCUMENT_QUERY,
    CAPABILITY_ENGINES,
    CAPABILITY_EVALUATE_SUBJECT,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_INFINITY_VALUE,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_METAOBJECT_VALUES,
    CAPABILITY_QUERY,
    CAPABILITY_RENDER_DOCUMENT,
    CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE,
    CAPABILITY_SET_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
    ServerInfo,
    mismatch_reason,
    require,
    upgrade_remedy,
)
from opensysml.conversion import (
    Conversion,
    EXPERIMENTAL_NOTICE,
    ExperimentalFeatureWarning,
    is_experimental,
)
from opensysml.diagnostic import Diagnostic
from opensysml.document import build_bindings, result_of as document_result
from opensysml.edit import error_for_failure, failure_name, result_of
from opensysml.enumeration import EnumLiteral
from opensysml.exploration import Exploration, Outcome
from opensysml.errors import (
    AnalysisRunError,
    ConnectionError,
    ConversionError,
    ExecutionError,
    ModelFileNotFoundError,
    ModelNotFoundError,
    StaleServiceError,
    SymbolNotFoundError,
    UnsupportedValueError,
    WrongKindError,
    from_rpc_error,
    translate_rpc_errors,
)
from opensysml.query import build_query, elements_of
from opensysml.values import (
    Array,
    Function,
    InstanceRef,
    MeasurementRef,
    Metaobject,
    Quantity,
    SetValue,
    TensorQuantity,
    Vector,
    VectorQuantity,
    _Infinity,
    value_to_python,
)
from opensysml.engines import ENGINE_AUTO, EngineInfo, Standing
from opensysml.verdict import (
    AnalysisResult, CalcResult, CaseEvaluation, SweepRow, SweepTable, Verdict,
    VerificationVerdict,
)


#: Port the service listens on when a caller names none.
DEFAULT_PORT = 50051

#: A required release not looked up yet, distinct from 'none required'.
_UNRESOLVED = object()

#: Address of an externally managed service to connect to, as ``host:port``.
#: Naming one here is the opt-in for a caller who cannot pass host and port.
SERVICE_ENV = 'OPENSYSML_SERVICE'

#: Options of every channel this client opens: only identity, so no service
#: compresses a response, which grpcio can hand to the parser still compressed.
CHANNEL_OPTIONS = (
    ('grpc.compression_enabled_algorithms_bitset', 1 << grpc.Compression.NoCompression),
)

#: Seconds a private child is given to report the address it bound.
START_TIMEOUT = 2.5

#: Seconds a private child is given to exit on its own before it is killed.
STOP_TIMEOUT = 5.0

#: Lines of a private child's stderr kept, so a failure to start can quote it.
_STDERR_LINES_KEPT = 20

#: Seconds a failed child's log is waited for, so an error can quote all of it.
STDERR_DRAIN_TIMEOUT = 0.5

#: Private services this interpreter started, keyed by the release required of
#: them, each counting the connections holding it. One child per interpreter per
#: requirement: its connections share the parse cache they would otherwise each
#: pay for, and no other process can reach it, so none can stop it either.
_private_services: Dict[Optional[str], '_PrivateService'] = {}

#: Held to start, join or release a private service, since connections of one
#: interpreter share them and may be opened and closed by different threads.
_private_services_lock = threading.RLock()


def open_channel(address):
    """Open an insecure channel to ``address`` with this client's options.

    Args:
        address (str): The service's ``host:port``

    Returns:
        grpc.Channel: The channel, to be closed by the caller
    """
    return grpc.insecure_channel(address, options=CHANNEL_OPTIONS)


def split_target(host, port=None):
    """Split a ``host:port`` string written as the host into host and port.

    ``connect("localhost:50123")`` names an address, not a hostname, so it is
    read as one rather than building ``localhost:50123:50051`` and reporting the
    service unreachable at an address nobody asked for.

    Args:
        host (str): Hostname, or a ``host:port`` address
        port (int, optional): Port; None is no port given, so an address's own
            port stands and a plain hostname gets DEFAULT_PORT

    Returns:
        tuple[str, int]: The host and port to connect to

    Raises:
        ValueError: If the address's port is not a number, or disagrees with a
            port also given
    """
    if not isinstance(host, str) or ':' not in host:
        return host, DEFAULT_PORT if port is None else port

    # A bare IPv6 address has colons of its own; only a bracketed one, or a
    # single colon, names a port.
    if host.startswith('['):
        closing = host.find(']')
        if closing == -1 or not host[closing + 1:].startswith(':'):
            return host, DEFAULT_PORT if port is None else port
        name, written = host[:closing + 1], host[closing + 2:]
    elif host.count(':') > 1:
        return host, DEFAULT_PORT if port is None else port
    else:
        name, written = host.split(':', 1)

    if not written.isdigit():
        raise ValueError(
            f"host={host!r} names no port this client can read; pass "
            f"host and port separately, as connect({name!r}, <port>)"
        )
    embedded = int(written)
    if port is not None and port != embedded:
        raise ValueError(
            f"host={host!r} and port={port} name different ports; give the "
            f"port once"
        )
    return name, embedded


def _failure_of(message, failure_reason, diagnostics):
    """Build the error for a failure the service classified.

    The kind is read from the response's typed reason, so a wrong request and a
    condition that could not be evaluated are told apart without reading the
    message text.
    """
    if failure_reason == sysml_pb2.FAILURE_REASON_WRONG_KIND:
        return WrongKindError(message, diagnostics=diagnostics)
    return ExecutionError(message, diagnostics=diagnostics)


def _value_or_unsupported(pb_value, resolve_instance):
    """Read a wire value, standing an UnsupportedValueError in for one with no form."""
    try:
        return value_to_python(pb_value, resolve_instance)
    except UnsupportedValueError as exc:
        return exc


def _evaluations_of(response, resolve_instance):
    """Read the case evaluations of a response, empty for a service without them.

    An argument or result the wire format cannot represent is reported as an
    UnsupportedValueError in its place, so one such value does not discard the
    evaluation or the run it belongs to.
    """
    evaluations = []
    for pb in getattr(response, "evaluations", ()):
        arguments = [_value_or_unsupported(arg, resolve_instance) for arg in pb.arguments]
        result = None
        if not pb.error and pb.HasField("result"):
            result = _value_or_unsupported(pb.result, resolve_instance)
        evaluations.append(CaseEvaluation(
            pb.function_id, arguments, result=result, error=pb.error,
            selected=pb.selected, tied=pb.tied,
        ))
    return evaluations


def _explores(schedule):
    """Whether a schedule spelling names the exploring policy, options or not."""
    return bool(schedule) and (schedule == "explore" or schedule.startswith("explore:"))


def _explore_engine(engine):
    """Whether an engine selection is the explore engine, a synonym of the exploring schedule."""
    return engine == "explore"


def _engine_field(engine):
    """The engine field as sent: empty for auto, which every service reads as such."""
    if not engine or engine == ENGINE_AUTO:
        return ""
    return engine


def _refuse_exploring(schedule, method):
    """Refuse an exploring schedule on a method answering one run's result."""
    if _explores(schedule):
        raise ValueError(
            f"schedule {schedule!r} answers with every outcome, not one run's "
            f"result: use {method}"
        )


def _require_exploring(schedule):
    """Refuse a schedule that would answer one run's result on a method reading outcomes."""
    if not _explores(schedule):
        raise ValueError(
            f"schedule {schedule!r} answers one run's result, not every outcome: "
            f"spell it 'explore' or 'explore:runs=<n>,depth=<d>'"
        )


def _verifications_of(response):
    """Read the body verdicts of a response, empty for a service without them."""
    return [
        VerificationVerdict(pb) for pb in getattr(response, "verification_verdicts", ())
    ]


def _verifications_for(verifications, pb_verdict):
    """The body verdicts of the requirement this verdict is about.

    A response answering several assertions carries the cases of every
    requirement it covers, so each verdict takes only its own; naming no
    requirement takes none rather than all of them.
    """
    requirement = getattr(pb_verdict, "requirement_id", "")
    if not requirement:
        return []
    return [v for v in verifications if v.requirement_id == requirement]


def _raise_wrong_kind(pb_verdict, diagnostics):
    """Raise when a verdict reports the named element is of another kind.

    Such a verdict is no answer about the model, so it is raised rather than
    returned as an undecided one; every other failure stays in verdict.error.
    """
    if pb_verdict.failure_reason == sysml_pb2.FAILURE_REASON_WRONG_KIND:
        raise WrongKindError(pb_verdict.error, diagnostics=diagnostics)


def named_target(host, port=None):
    """The externally managed service named by the caller or the environment.

    Naming an address is the opt-in to a service this client does not manage;
    with none named, a connection starts a private child of its own instead.

    Args:
        host (str): Hostname, or a ``host:port`` address
        port (int, optional): Port, or None for none given

    Returns:
        tuple[str, int] or None: The host and port named, or None when the
            caller named neither an address nor $OPENSYSML_SERVICE

    Raises:
        ValueError: If the address is unreadable or disagrees with port
    """
    if port is not None or host != 'localhost':
        return split_target(host, port)
    named = os.environ.get(SERVICE_ENV)
    if named:
        return split_target(named)
    return None


class _PrivateService:
    """A sysml-grpc child this interpreter started and only it can reach.

    It is given its port by the kernel, which it reports on stdout, and it holds
    the read end of a pipe whose write end this process keeps open and never
    writes to: however this process dies, the pipe closes, the child reads end of
    file and exits. Nothing about it is recorded outside this process, because
    nothing outside this process may use or stop it.
    """

    def __init__(self, process, binary_path, key):
        self.process = process
        self.binary_path = binary_path
        self.key = key
        self.address = ''
        #: Connections in this process holding it; the last one released stops it.
        self.refs = 0
        #: Set in a child of fork(), which inherits no right to stop it.
        self.disowned = False
        self._stderr: Deque[str] = deque(maxlen=_STDERR_LINES_KEPT)
        #: Held to read or append the log, which one thread does while another reads.
        self._stderr_lock = threading.Lock()
        self._reported: 'queue.Queue[Optional[str]]' = queue.Queue(maxsize=1)
        #: Set at end of file on its stdout, which only its exit closes.
        self._ended = threading.Event()
        self._stderr_reader = self._start_reader(self._read_stderr)
        self._start_reader(self._read_address)

    def alive(self):
        """Whether the child is still there to be used.

        End of file on its stdout is the reliable answer: a wait can still find
        an exited child unreapable for an instant, and report it as running.

        Returns:
            bool: True while it runs
        """
        return not self._ended.is_set() and self.process.poll() is None

    @classmethod
    def start(cls, version, key):
        """Start a private child and wait for the address it bound.

        Args:
            version (str, optional): Release to start, as ensure_binary reads it
            key (str, optional): Release requirement it is held under

        Returns:
            _PrivateService: The started child, listening and reachable

        Raises:
            ConnectionError: If it does not report an address it is serving
        """
        binary_path = ensure_binary(version=version)
        if not os.path.exists(binary_path):
            raise ConnectionError(f"Binary not found after download: {binary_path}")
        process = subprocess.Popen(
            [binary_path, '-port', '0', '-health-port', '0',
             '-report-address', '-exit-with-parent'],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
        )
        service = cls(process, binary_path, key)
        try:
            service.address = service._await_address()
        except BaseException:
            service.stop()
            raise
        return service

    def stop(self):
        """Stop the child, unless a fork() left this process no right to.

        Closing the pipe would end it on its own; it is asked to stop as well so
        a connection closed in a long-lived process does not wait on the read.
        Only this Popen's own child is ever signalled, so no pid the operating
        system has since reused can be.
        """
        if self.disowned:
            return
        self._close_pipe()
        if self.process.poll() is None:
            self.process.terminate()
        try:
            self.process.wait(timeout=STOP_TIMEOUT)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()

    def disown(self):
        """Give up a service inherited by a child of fork().

        The child closes its copy of the write end, so the service still dies
        with the process that started it, and never signals a process it did not
        start itself.
        """
        self.disowned = True
        self._close_pipe()

    def _close_pipe(self):
        """Close the write end whose end of file ends the child."""
        if self.process.stdin is None:
            return
        try:
            self.process.stdin.close()
        except OSError:
            pass

    def _await_address(self):
        """The address the child reports once its listener is bound.

        Returns:
            str: The ``host:port`` to dial

        Raises:
            ConnectionError: If it exits, or reports nothing in START_TIMEOUT
        """
        try:
            reported = self._reported.get(timeout=START_TIMEOUT)
        except queue.Empty:
            raise ConnectionError(
                f"{self.binary_path} did not report a listening address within "
                f"{START_TIMEOUT}s{self._stderr_tail()}"
            )
        if reported is None:
            raise ConnectionError(
                f"{self.binary_path} exited with code {self.process.poll()} "
                f"without serving an address{self._stderr_tail()}"
            )
        return reported

    def _read_address(self):
        """Report the address line, then keep stdout drained.

        A child that exits without reporting closes stdout, which reads as end
        of file: the wait ends on the exit rather than on the timeout.
        """
        line = self.process.stdout.readline().decode('utf-8', 'replace').strip()
        self._reported.put(line or None)
        # Discard the rest of stdout until end of file, keeping nothing.
        deque(self.process.stdout, maxlen=0)
        self._ended.set()

    def _read_stderr(self):
        """Keep the last lines of the child's log, and its stderr drained.

        An undrained pipe fills and blocks the service in a write, so its log is
        read for as long as it runs whether or not anything asks for it.
        """
        for line in self.process.stderr:
            with self._stderr_lock:
                self._stderr.append(line.decode('utf-8', 'replace').rstrip())

    def _start_reader(self, target):
        """Read one of the child's pipes in a thread that cannot outlive exit."""
        thread = threading.Thread(target=target, daemon=True)
        thread.start()
        return thread

    def _stderr_tail(self):
        """What the child last logged, for an error that must explain itself.

        A child that failed has closed stderr, so its reader is given a moment to
        finish: the log is quoted whole rather than as far as it had been read.
        """
        self._stderr_reader.join(timeout=STDERR_DRAIN_TIMEOUT)
        with self._stderr_lock:
            logged = list(self._stderr)
        if not logged:
            return ""
        return "; it logged: " + " | ".join(logged)


def _join_private_service(version, required_release):
    """Hold this interpreter's private service for a release requirement.

    One is started when there is none, or when the last one died: a service that
    crashed is replaced rather than reported, since nothing outside this process
    could have been using it.

    Args:
        version (str, optional): Release to start, as ensure_binary reads it
        required_release (str, optional): Release required of it, which services
            are keyed by, so a connection never joins one of another release

    Returns:
        _PrivateService: A running child with a reference taken on it, which the
            caller releases exactly once
    """
    with _private_services_lock:
        service = _private_services.get(required_release)
        if service is None or not service.alive():
            service = _PrivateService.start(version, required_release)
            _private_services[required_release] = service
        service.refs += 1
        return service


def _disown_private_services():
    """Give up, in a child of fork(), the services its parent started.

    The child inherits copies of the pipes but no ownership: it closes them, so
    each service still dies with the process that started it, and starts one of
    its own if it goes on to connect.
    """
    for service in _private_services.values():
        service.disown()
    _private_services.clear()
    _private_services_lock.release()


if hasattr(os, 'register_at_fork'):
    # The lock is taken across the fork, so a child never inherits it held by a
    # thread that does not exist there, mid-change.
    os.register_at_fork(
        before=_private_services_lock.acquire,
        after_in_parent=_private_services_lock.release,
        after_in_child=_disown_private_services,
    )


class Connection:
    """Manages connection to sysml-grpc service.

    Unless an address is named, a connection joins this interpreter's private
    service, starting it if there is none: a child on a port the kernel chose,
    which no other process can reach and which dies with this one.

    Attributes:
        host (str): Service hostname
        port (int): Service port
    """
    
    def __init__(self, host='localhost', port=None, auto_start=True,
                 version=None, require_capabilities=None):
        """Initialize connection to sysml-grpc service.
        
        Args:
            host (str): Hostname of an externally managed service, or a
                ``host:port`` address naming one. Naming either is the opt-in to
                a service this client does not manage; left unnamed, and with
                $OPENSYSML_SERVICE unset, the connection starts a private child
                instead (default: 'localhost')
            port (int, optional): Port of an externally managed service. None
                names none, so a private child is used unless host names an
                address; auto_start=False without either means the standard port
            auto_start (bool): If True, start a private child when no address is
                named. If False, connect to the address named, or to the
                standard port, and start nothing (default: True)
            version (str, optional): Release tag the service must report, or
                'latest'. Defaults to $OPENSYSML_GRPC_VERSION, the same tag the
                binary cache is checked against; without either, whatever
                release answers is accepted. Private children are held per
                requirement, so a connection never joins one of another release.
                An externally managed service that is not listening yet is
                checked at the first call instead.
            require_capabilities (iterable, optional): Capability names the
                service must report, checked once at connect time rather than
                when the first call needing one is made

        Raises:
            ValueError: If host names a port that is unreadable or disagrees
                with port
            ConnectionError: If a private child cannot be started
            StaleServiceError: If the service reached is another release
            MissingCapabilityError: If the service lacks a required capability
        """
        self._cleaned_up = False
        self._server_info = None
        self._channel = None
        #: The private child this connection holds, or None for one it does not manage.
        self._private = None
        self._version = version or os.environ.get('OPENSYSML_GRPC_VERSION') or None
        self._required_capabilities = frozenset(require_capabilities or ())
        self._resolved_release = _UNRESOLVED
        # An externally managed service that was not listening yet is checked at
        # the first handshake, so connecting stays free of eager I/O.
        self._check_release_on_handshake = False

        named = named_target(host, port)
        if named is None and auto_start:
            self._ensure_service()
        else:
            self.host, self.port = named if named is not None else (host, DEFAULT_PORT)
            self._address = f"{self.host}:{self.port}"
            # Provenance of the service, so an error can name the binary at fault.
            self._origin = (
                f"service at {self._address} (not started by this client)"
            )

        self._channel = open_channel(self._address)
        self._service = sysml_pb2_grpc.SysMLServiceStub(self._channel)
        try:
            if self._private is None:
                self._check_managed_service_release()
            else:
                # The handshake is also the child's readiness check: it answers
                # once it serves the address it reported binding.
                self._raise_if_release_mismatch(self.server_info())
            for capability in sorted(self._required_capabilities):
                require(self.server_info(), capability, upgrade_remedy(capability))
        except BaseException:
            # A refused connection is never returned, so nothing else can
            # release its channel or the reference it took on the service.
            self.close()
            raise
    
    def _check_managed_service_release(self):
        """Check the release of a service somebody else manages.

        Nothing is started or stopped in its place, so a mismatch is reported
        rather than acted on. Such a service may not be listening yet, so one
        that cannot be asked is checked at the first handshake instead.
        """
        if self._required_release() is None:
            return
        info = self._running_service_info()
        if info is None:
            self._check_release_on_handshake = True
            return
        self._raise_if_release_mismatch(info)

    def _raise_if_release_mismatch(self, info):
        """Report a service that is not the release asked for.

        Raises:
            StaleServiceError: If what it reports differs from the requirement
        """
        required = self._required_release()
        if required is None:
            return
        reason = mismatch_reason(info, version=required)
        if reason is None:
            return
        if self._private is not None:
            remedy = (
                f"the binary this client started, {self._private.binary_path}, is "
                f"not {required}: make that release available (its download is "
                f"cached under ~/.opensysml/bin), or accept what is installed by "
                f"passing version=None and unsetting $OPENSYSML_GRPC_VERSION"
            )
        else:
            remedy = (
                f"stop the service listening on {self._address} yourself and let "
                f"this client start a {required} one, or accept what is running "
                f"by passing version=None and unsetting $OPENSYSML_GRPC_VERSION"
            )
        raise StaleServiceError(self._address, reason, remedy, info=info)

    def close(self):
        """Close the gRPC channel and release any hold on a private service."""
        if self._channel:
            self._channel.close()
        self._cleanup_service()
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()
    
    @property
    def _stub(self):
        """The service stub, with any release check still owed done first.

        Every call goes through here, so a service that came up after the client
        was built is checked at whichever call reaches it first.
        """
        if self._check_release_on_handshake:
            self.server_info()
        return self._service

    def server_info(self):
        """Ask the service what it is and what it supports.

        The answer is cached for the life of the connection: a service does not
        change build while a channel is open to it.

        Returns:
            ServerInfo: Reported version and capabilities. ``answered`` is False
                when the service predates the GetServerInfo RPC, in which case
                it claims no capabilities.

        Raises:
            StaleServiceError: If a release was asked for and this first answer
                shows the service is another one
        """
        if self._server_info is None:
            request = sysml_pb2.ServerInfoRequest()
            try:
                response = self._service.GetServerInfo(request)
            except grpc.RpcError as e:
                if e.code() != grpc.StatusCode.UNIMPLEMENTED:
                    raise from_rpc_error(e) from e
                self._server_info = ServerInfo(
                    version='',
                    capabilities=frozenset(),
                    answered=False,
                    origin=self._origin,
                )
            else:
                self._server_info = ServerInfo(
                    version=response.version,
                    capabilities=frozenset(response.capabilities),
                    answered=True,
                    origin=self._origin,
                )
        # Cleared only once the check passes, so a mismatch keeps being reported
        # instead of the connection turning usable after one error.
        if self._check_release_on_handshake:
            self._raise_if_release_mismatch(self._server_info)
            self._check_release_on_handshake = False
        return self._server_info

    def load(self, file_path, strict=False, strict_conformance=False):
        """Load a SysML model from file.
        
        Args:
            file_path (str): Path to .sysml file
            strict (bool): Refuse a model the service reported errors for,
                instead of returning one whose lookups fail later. The
                :class:`~opensysml.errors.ModelError` raised carries the model, so
                its diagnostics stay inspectable.
            strict_conformance (bool): Ask whether the file is conforming SysML v2:
                notation only OpenSysML accepts is reported as an error rather than
                a warning.
        
        Returns:
            Model: Parsed model object
        
        Raises:
            ModelFileNotFoundError: If the service cannot read file_path
            ModelError: If strict and the model has error diagnostics
            ServiceError: If the service fails the call for any other reason
        """
        self._require_strict_conformance(strict_conformance)
        request = sysml_pb2.ParseFileRequest(
            file_path=file_path, strict_conformance=strict_conformance)
        capabilities = (
            (CAPABILITY_STRICT_CONFORMANCE,) if strict_conformance else ()
        )
        with translate_rpc_errors(
            not_found=ModelFileNotFoundError,
            unimplemented=self._capability_refusal(capabilities),
        ):
            response = self._stub.ParseFile(request)
        model = Model(response, self, source_path=file_path)
        if strict:
            model.raise_for_errors()
        return model
    
    def _require_strict_conformance(self, strict_conformance):
        """Refuse a strict-conformance ask a service would silently ignore."""
        if not strict_conformance:
            return
        require(
            self.server_info(),
            CAPABILITY_STRICT_CONFORMANCE,
            upgrade_remedy(CAPABILITY_STRICT_CONFORMANCE),
        )

    def load_from_content(self, content, strict=False, language=None,
                          strict_conformance=False):
        """Load a model from inline SysML content.
        
        Args:
            content (str): SysML source code
            strict (bool): Refuse a model the service reported errors for
            language (str, optional): "sysml" or "kerml"; the language the
                inline content is written in
            strict_conformance (bool): Ask whether the content is conforming
                SysML v2: notation only OpenSysML accepts is an error, not a
                warning
            
        Returns:
            Model: Parsed model object

        Raises:
            ModelError: If strict and the model has error diagnostics
        """
        if language is not None:
            require(
                self.server_info(),
                CAPABILITY_INLINE_LANGUAGE,
                upgrade_remedy(CAPABILITY_INLINE_LANGUAGE),
            )
            if language not in ("sysml", "kerml"):
                raise ValueError("language must be 'sysml' or 'kerml'")
        self._require_strict_conformance(strict_conformance)
        request = sysml_pb2.ParseFileRequest(
            content=content, language=language or "", strict_conformance=strict_conformance)
        capabilities = []
        if language is not None:
            capabilities.append(CAPABILITY_INLINE_LANGUAGE)
        if strict_conformance:
            capabilities.append(CAPABILITY_STRICT_CONFORMANCE)
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(capabilities)
        ):
            response = self._stub.ParseFile(request)
        model = Model(response, self)
        if strict:
            model.raise_for_errors()
        return model
    
    def convert(self, to_format, file_path=None, content=None, model_hash=None,
                from_format='', tolerate_syntax_errors=False):
        """Write a model out in another of the formats OpenSysML writes.

        The source is a loaded model, named by its hash, or one named the way
        :meth:`load` names it: a path the service opens, or content carried
        inline. A hash converts the source the service parsed, so a file edited
        since the load does not change the answer; a path is read afresh.

        Args:
            to_format (str): Format to write: 'sysml', 'kerml', 'text', 'ttl',
                'turtle' or 'rdf'
            file_path (str, optional): Path the service reads the source from
            content (str, optional): Source carried inline
            model_hash (str, optional): Hash of a loaded model, whose parsed
                source is converted
            from_format (str, optional): Format to read the source as; inferred
                from file_path's extension when omitted, notation for a
                model_hash, and required for inline content
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it, reporting its syntax errors as
                the result's diagnostics. Notation to notation only: every other
                direction builds a graph, where unreadable declarations would go
                missing silently.

        Returns:
            Conversion: The converted model, the formats used and any tolerated
                syntax errors

        Warns:
            ExperimentalFeatureWarning: If either format is RDF, whose mapping is
                experimental — see ``docs/reference/rdf-mapping.md``

        Raises:
            ValueError: If other than one of file_path, content and model_hash
                is given
            MissingCapabilityError: If the service cannot convert
            ConversionError: If the model could not be written in that format
            InvalidRequestError: If a format is unknown
            ModelFileNotFoundError: If the named file cannot be read
            ModelNotFoundError: If the model is no longer cached
        """
        given = [
            name
            for name, value in (
                ('file_path', file_path),
                ('content', content),
                ('model_hash', model_hash),
            )
            if value is not None
        ]
        if len(given) != 1:
            raise ValueError(
                "Provide exactly one of file_path, content or model_hash; got "
                + (", ".join(given) if given else "none")
            )
        require(
            self.server_info(),
            CAPABILITY_CONVERT,
            upgrade_remedy(CAPABILITY_CONVERT),
        )

        request = sysml_pb2.ConvertRequest(
            to_format=to_format,
            from_format=from_format,
            tolerate_syntax_errors=tolerate_syntax_errors,
        )
        if file_path is not None:
            request.file_path = file_path
        elif content is not None:
            request.content = content
        else:
            request.model_hash = model_hash

        not_found = (
            ModelFileNotFoundError if file_path is not None else ModelNotFoundError
        )
        with translate_rpc_errors(
            not_found=not_found,
            unimplemented=self._capability_refusal((CAPABILITY_CONVERT,)),
        ):
            response = self._stub.Convert(request)
        # Judged from the response, so an inferred format counts and a service
        # too old to mark the conversion is still read as experimental.
        experimental = response.experimental or is_experimental(
            response.from_format, response.to_format
        )
        notice = response.experimental_notice or (
            EXPERIMENTAL_NOTICE if experimental else ""
        )
        if experimental:
            # Warned before the error is raised: a refusal is the mapping's
            # experimental behavior, not a reason to say nothing about it.
            warnings.warn(notice, ExperimentalFeatureWarning, stacklevel=2)
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ConversionError(response.error, diagnostics=diagnostics)
        return Conversion(
            content=response.content,
            from_format=response.from_format,
            to_format=response.to_format,
            diagnostics=diagnostics,
            experimental=experimental,
            experimental_notice=notice,
        )

    def apply_edits(self, model_hash, operations):
        """Apply source-preserving edits to a loaded model.

        The operations are performed on the source the service parsed, so what
        comes back is that notation with the edited spans replaced and every
        other byte — comments, blank lines, indentation — unchanged. The service
        re-parses and re-analyses the result and refuses to return content the
        parser could not read back.

        Args:
            model_hash (str): Hash of the model to edit
            operations (list[tuple]): ``('set_value', target, value)`` and
                ``('rename', target, new_name)`` tuples, as
                :class:`~opensysml.edit.Editor` collects them

        Returns:
            EditResult: The edited notation and what each operation changed

        Raises:
            ValueError: If an operation names no kind this client knows
            EditError: If the service refused the edit; the subclass names why,
                including :class:`NoEditsError` for no operations at all
            MissingCapabilityError: If the service cannot apply edits
            ModelNotFoundError: If the model is no longer cached
        """
        info = self.server_info()
        require(info, CAPABILITY_APPLY_EDITS, upgrade_remedy(CAPABILITY_APPLY_EDITS))
        request = sysml_pb2.ApplyEditsRequest(model_hash=model_hash)
        requests_authoring = False
        for operation_data in operations:
            operation = request.operations.add()
            kind = operation_data[0]
            if kind == 'set_value':
                _, target, text = operation_data
                operation.set_value.target = target
                operation.set_value.value = text
            elif kind == 'rename':
                _, target, text = operation_data
                operation.rename.target = target
                operation.rename.new_name = text
            elif kind == 'add_member':
                if len(operation_data) != 8:
                    raise ValueError(
                        "malformed add_member operation: expected 8 fields"
                    )
                _, owner, member_kind, name, type_name, multiplicity, value, specializes = operation_data
                require(info, CAPABILITY_AUTHORING, upgrade_remedy(CAPABILITY_AUTHORING))
                requests_authoring = True
                add = operation.add_member
                add.owner, add.kind, add.name = owner, member_kind, name
                add.type, add.multiplicity, add.value = type_name, multiplicity, value
                add.specializes.extend(specializes)
            elif kind == 'delete':
                if len(operation_data) != 3 or not isinstance(operation_data[2], bool):
                    raise ValueError(
                        "malformed delete operation: expected target and bool cascade"
                    )
                _, target, cascade = operation_data
                require(info, CAPABILITY_AUTHORING, upgrade_remedy(CAPABILITY_AUTHORING))
                requests_authoring = True
                operation.delete.target, operation.delete.cascade = target, cascade
            else:
                raise ValueError(
                    f"unknown edit operation {kind!r}: expected set_value, rename, add_member or delete"
                )

        requested_capabilities = [CAPABILITY_APPLY_EDITS]
        if requests_authoring:
            requested_capabilities.append(CAPABILITY_AUTHORING)
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(requested_capabilities)
        ):
            response = self._stub.ApplyEdits(request)
        if response.error:
            raise error_for_failure(
                failure_name(response.failure),
                response.error,
                diagnostics=[Diagnostic(d) for d in response.diagnostics],
                referring_elements=list(response.referring_elements),
            )
        return result_of(response)

    def query(self, model_hash, payload=None, scope=None, select=None, where=None):
        """Run a SysML v2 API & Services Query over a loaded model.

        The query is the standard's JSON object, so a cookbook payload works
        verbatim, or the same thing as keywords. See :mod:`opensysml.query`.

        Args:
            model_hash (str): Hash of the model to query
            payload (dict, optional): The standard's ``Query`` object
            scope (list, optional): Elements to consider; empty is the whole model
            select (list, optional): Properties to report; empty reports every one
            where (dict, optional): Constraint to filter by

        Returns:
            list[QueryElement]: The elements selected, in declaration order

        Raises:
            QueryError: If the query is not one the standard's model describes
            MissingCapabilityError: If the service cannot query
            InvalidRequestError: If a property or scope is unknown to the service
            ModelNotFoundError: If the model is no longer cached
        """
        require(
            self.server_info(),
            CAPABILITY_QUERY,
            upgrade_remedy(CAPABILITY_QUERY),
        )
        request = sysml_pb2.QueryRequest(
            model_hash=model_hash,
            query=build_query(payload, scope=scope, select=select, where=where),
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal((CAPABILITY_QUERY,))
        ):
            response = self._stub.Query(request)
        return elements_of(response)

    def run_document_query(self, model_hash, query_id, bindings=None):
        """Run a named document query and answer its typed rows.

        The query is the model's own — a calc def specializing
        ``DocumentQueries::Query`` — not the standard's Query object that
        :meth:`query` evaluates. See :mod:`opensysml.document`.

        Args:
            model_hash (str): Hash of the model holding the query
            query_id (str): Qualified name of the document query
            bindings (Mapping, optional): Parameter name to a value or list of
                values; an :class:`~opensysml.document.ElementRef` binds a
                model element

        Returns:
            DocumentQueryResult: Projected columns and typed rows, in the
            engine's deterministic order

        Raises:
            MissingCapabilityError: If the service cannot run document queries
            InvalidRequestError: If the query is not one, or a binding is wrong
            SymbolNotFoundError: If the model does not declare the query
            ModelNotFoundError: If the model is no longer cached
        """
        require(
            self.server_info(),
            CAPABILITY_DOCUMENT_QUERY,
            upgrade_remedy(CAPABILITY_DOCUMENT_QUERY),
        )
        request = sysml_pb2.RunDocumentQueryRequest(
            model_hash=model_hash,
            query_id=query_id,
            bindings=build_bindings(bindings),
        )
        with translate_rpc_errors(
            not_found=SymbolNotFoundError,
            unimplemented=self._capability_refusal((CAPABILITY_DOCUMENT_QUERY,)),
        ):
            response = self._stub.RunDocumentQuery(request)
        return document_result(response)

    def render_document(self, model_hash, document_id):
        """Render a named document to Markdown.

        The document is the model's own — a part def specializing
        ``DocumentQueries::Document`` — whose queries are bound in the model.

        Args:
            model_hash (str): Hash of the model holding the document
            document_id (str): Qualified name of the document

        Returns:
            str: The rendered Markdown

        Raises:
            MissingCapabilityError: If the service cannot render documents
            InvalidRequestError: If the symbol named is not a document
            SymbolNotFoundError: If the model does not declare the document
            ModelNotFoundError: If the model is no longer cached
        """
        require(
            self.server_info(),
            CAPABILITY_RENDER_DOCUMENT,
            upgrade_remedy(CAPABILITY_RENDER_DOCUMENT),
        )
        request = sysml_pb2.RenderDocumentRequest(
            model_hash=model_hash,
            document_id=document_id,
        )
        with translate_rpc_errors(
            not_found=SymbolNotFoundError,
            unimplemented=self._capability_refusal((CAPABILITY_RENDER_DOCUMENT,)),
        ):
            response = self._stub.RenderDocument(request)
        return response.markdown

    def get_symbol(self, model_hash, symbol_id):
        """Fetch symbol by ID from cached model.
        
        Args:
            model_hash (str): Model content hash
            symbol_id (str): Fully-qualified symbol ID
        
        Returns:
            sysml_pb2.SymbolInfo or None: Symbol protobuf, or None if not found
        """
        request = sysml_pb2.GetSymbolRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
        )
        with translate_rpc_errors():
            response = self._stub.GetSymbol(request)
        
        if response.error:
            # Symbol not found or other error
            return None
        
        return response.symbol
    
    def eval(
        self,
        expression,
        model_hash,
        context_symbol_id=None,
        subject_symbol_id=None,
    ):
        """Evaluate a SysML expression.
        
        Args:
            expression (str): SysML expression (e.g., "2 + 2")
            model_hash (str): Hash from ParseFile response
            context_symbol_id (str, optional): Symbol FQN for context scope
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against, so a feature reads that
                object's value rather than the declared default
            
        Returns:
            Value from expression (int, float, bool, str, Instance, etc.)
            
        Raises:
            ExecutionError: If evaluation fails
            ModelNotFoundError: If the service no longer holds the model
            UnsupportedValueError: If the result cannot be represented on the wire
        """
        if subject_symbol_id:
            # A service that ignores the subject would answer with the declared
            # default, which is indistinguishable from the object's own value.
            require(
                self.server_info(),
                CAPABILITY_EVALUATE_SUBJECT,
                upgrade_remedy(CAPABILITY_EVALUATE_SUBJECT),
            )

        req = sysml_pb2.EvaluateRequest(
            model_hash=model_hash,
            expression=expression,
            context_symbol_id=context_symbol_id or "",
            subject_symbol_id=subject_symbol_id or "",
        )
        
        capabilities = (
            (CAPABILITY_EVALUATE_SUBJECT,) if subject_symbol_id else ()
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(capabilities)
        ):
            response = self._stub.Evaluate(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        # Convert protobuf Value to Python type
        return self._value_to_python(response.result)
    
    def instantiate(self, symbol_id, model_hash):
        """Instantiate a part/usage symbol.
        
        Args:
            symbol_id (str): FQN of part/usage to instantiate
            model_hash (str): Hash from ParseFile response
            
        Returns:
            Instance object
            
        Raises:
            ExecutionError: If instantiation fails
            ModelNotFoundError: If the service no longer holds the model
            MissingCapabilityError: If the service predates ``feature_values``
        """
        from opensysml.instance import Instance

        self._require_feature_values()
        req = sysml_pb2.InstantiateRequest(
            model_hash=model_hash,
            symbol_id=symbol_id
        )
        
        with translate_rpc_errors():
            response = self._stub.Instantiate(req)
        
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        graph = {inst.id: inst for inst in response.instances}
        return Instance(response.instance, graph)
    
    def execute_action(self, action_symbol_id, model_hash, inputs=None,
                       schedule=None):
        """Execute an action definition.
        
        Args:
            action_symbol_id (str): FQN of action def
            model_hash (str): Hash from ParseFile response
            inputs (dict, optional): Input parameter name → value
            schedule (str, optional): Scheduling policy the run resolves its
                choice points under — ``"declared"``, ``"reverse"`` (the
                default) or ``"seed:<n>"`` — spelled as ``sysml -schedule``
                spells it. ``"explore"`` answers with every outcome rather than
                one run's, so it belongs to :meth:`explore_action`
            
        Returns:
            dict: Output parameter name → value; an output the wire format cannot
                represent is reported as an UnsupportedValueError in its place,
                so one such output does not discard the rest
            
        Raises:
            ValueError: If the schedule explores
            ExecutionError: If execution fails
            ModelNotFoundError: If the service no longer holds the model
            MissingCapabilityError: If an input holds a ``complex`` and the
                service predates ``complex_values``, an :class:`~opensysml.values.Array`,
                :class:`~opensysml.values.Vector` or :class:`~opensysml.values.VectorQuantity`
                and the service predates ``structured_values``, a
                :class:`~opensysml.values.MeasurementRef` and the service predates
                ``measurement_refs``, a :class:`~opensysml.values.Function`
                and the service predates ``function_values``, a
                :class:`~opensysml.values.SetValue` and the service predates
                ``set_values``, a :class:`~opensysml.values.TensorQuantity`
                and the service predates ``tensor_values``, a
                :class:`~opensysml.values.Metaobject` and the service predates
                ``metaobject_values``, or a schedule is given and the service
                predates ``schedule``; nothing is sent
            InvalidRequestError: If the schedule names no policy
        """
        _refuse_exploring(schedule, "explore_action")
        response = self._execute_action(action_symbol_id, model_hash, inputs, schedule)
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        return self._values_to_python(response.outputs)

    def explore_action(self, action_symbol_id, model_hash, inputs=None,
                       schedule="explore"):
        """Run an action once per valid order of its choice points, within a budget.

        The service replays the action from the start, taking a different
        alternative at some choice point each time, until every order within
        the budget has run. Runs agreeing on their outputs are one outcome; a
        run that failed is an outcome of its own rather than an error of the
        exploration.

        Args:
            action_symbol_id (str): FQN of action def
            model_hash (str): Hash from ParseFile response
            inputs (dict, optional): Input parameter name → value
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``, bounding the runs made and the
                choice points one run resolves; the service documents the
                defaults

        Returns:
            Exploration: Every distinct outcome reached and how the exploration
                ended; incomplete, naming the budget, when a budget was hit

        Raises:
            ValueError: If the schedule does not explore
            ExecutionError: If the action could not be explored at all — an
                unknown action, an input of the wrong kind
            ModelNotFoundError: If the service no longer holds the model
            MissingCapabilityError: If the service predates ``schedule_explore``,
                or an input needs a value capability it lacks; nothing is sent
            InvalidRequestError: If the schedule's options are malformed
        """
        _require_exploring(schedule)
        response = self._execute_action(action_symbol_id, model_hash, inputs, schedule)
        return self._exploration_of(response)

    def _execute_action(self, action_symbol_id, model_hash, inputs, schedule):
        """Send an ExecuteAction request, the schedule's capabilities checked first."""
        pb_inputs = {name: self._python_to_value(val) for name, val in (inputs or {}).items()}
        self._require_schedule(schedule)
        req = sysml_pb2.ExecuteActionRequest(
            model_hash=model_hash,
            action_symbol_id=action_symbol_id,
            inputs=pb_inputs,
            schedule=schedule or "",
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(self._schedule_capabilities(schedule))
        ):
            return self._stub.ExecuteAction(req)
    
    def execute_state(self, state_machine_symbol_id, model_hash, events=None,
                      schedule=None):
        """Execute a state machine.
        
        Args:
            state_machine_symbol_id (str): FQN of state machine def
            model_hash (str): Hash from ParseFile response
            events (list, optional): Event names to process
            schedule (str, optional): Scheduling policy the run resolves its
                choice points under, as for :meth:`execute_action`;
                ``"explore"`` belongs to :meth:`explore_state`
            
        Returns:
            dict: {'states_visited': [...], 'final_context': {...}, 'final_time': float};
                a context value the wire format cannot represent is reported as
                an UnsupportedValueError in its place; ``final_time`` is the
                run's simulation clock when it ended, in seconds, the instant
                of its last time-triggered transition — 0.0 from a service
                that predates ``final_time``
            
        Raises:
            ValueError: If the schedule explores
            ExecutionError: If execution fails
            ModelNotFoundError: If the service no longer holds the model
            MissingCapabilityError: If a schedule is given and the service
                predates ``schedule``; nothing is sent
            InvalidRequestError: If the schedule names no policy
        """
        _refuse_exploring(schedule, "explore_state")
        response = self._execute_state(state_machine_symbol_id, model_hash, events, schedule)
        if response.error:
            wrapped_diags = [Diagnostic(d) for d in response.diagnostics]
            raise ExecutionError(response.error, diagnostics=wrapped_diags)
        
        return {
            'states_visited': list(response.states_visited),
            'final_context': self._values_to_python(response.final_context),
            'final_time': response.final_time,
        }

    def explore_state(self, state_machine_symbol_id, model_hash, events=None,
                      schedule="explore"):
        """Run a state machine over the events once per valid order of its choice points.

        Runs agreeing on the state they rest in, the states they entered and
        the values they hold are one outcome; see :meth:`explore_action`.

        Args:
            state_machine_symbol_id (str): FQN of state machine def
            model_hash (str): Hash from ParseFile response
            events (list, optional): Event names to process
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``

        Returns:
            Exploration: Every distinct outcome reached and how the exploration
                ended

        Raises:
            ValueError: If the schedule does not explore
            ExecutionError: If the machine could not be explored at all
            ModelNotFoundError: If the service no longer holds the model
            MissingCapabilityError: If the service predates ``schedule_explore``;
                nothing is sent
            InvalidRequestError: If the schedule's options are malformed
        """
        _require_exploring(schedule)
        response = self._execute_state(state_machine_symbol_id, model_hash, events, schedule)
        return self._exploration_of(response)

    def _execute_state(self, state_machine_symbol_id, model_hash, events, schedule):
        """Send an ExecuteState request, the schedule's capabilities checked first."""
        self._require_schedule(schedule)
        req = sysml_pb2.ExecuteStateRequest(
            model_hash=model_hash,
            state_machine_symbol_id=state_machine_symbol_id,
            events=events or [],
            schedule=schedule or "",
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(self._schedule_capabilities(schedule))
        ):
            return self._stub.ExecuteState(req)

    def _exploration_of(self, response, failure_reason=None):
        """Read the outcomes and status of an explored run, or raise its failure to run at all."""
        if response.error:
            diagnostics = [Diagnostic(d) for d in response.diagnostics]
            raise _failure_of(response.error, failure_reason, diagnostics)
        status = response.exploration
        return Exploration(
            [self._outcome_of(pb) for pb in response.outcomes],
            complete=status.complete,
            runs=status.runs,
            budgets_hit=status.budgets_hit,
            runs_budget=status.runs_budget,
            depth_budget=status.depth_budget,
        )

    def _outcome_of(self, pb):
        """Read one wire outcome, an unsupported value kept as its error."""
        return Outcome(
            self._values_to_python(pb.outputs),
            final_state=pb.final_state,
            states_visited=pb.states_visited,
            error=pb.error,
            linearizations=pb.linearizations,
            witness=pb.witness,
            diagnostics=[Diagnostic(d) for d in pb.diagnostics],
        )
    
    def list_engines(self):
        """List the analysis engines the service answers with, as ``sysml -engines`` does.

        Returns:
            list[EngineInfo]: One per engine, in name order, with the strength
                it may claim, the questions it answers and whether it can run

        Raises:
            MissingCapabilityError: If the service predates ``engines``;
                nothing is sent
        """
        self._require_engines()
        with translate_rpc_errors(
            unimplemented=self._capability_refusal((CAPABILITY_ENGINES,))
        ):
            response = self._stub.ListEngines(sysml_pb2.ListEnginesRequest())
        return [EngineInfo.of(pb) for pb in response.engines]

    def verify_constraint(self, symbol_id, model_hash, subject_symbol_id=None, engine=None):
        """Ask whether a constraint holds, as the REPL's ``%constraint`` does.

        Args:
            symbol_id (str): FQN of the constraint definition or usage
            model_hash (str): Hash from ParseFile response
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against, so the verdict is about
                concrete values rather than declared defaults
            engine (str, optional): The engine to ask, as ``sysml -engine``
                spells it: ``"auto"`` (the default) for the strongest covering
                engine, ``"all"`` for every covering one composed, or one by
                name, whose refusal is then the answer

        Returns:
            Verdict: The answer. A condition that evaluated to false is that
                answer, not an exception; a failure to evaluate is reported as
                ``verdict.error``.

        Raises:
            WrongKindError: If symbol_id names an element that is not a
                constraint, which is a wrong request rather than a verdict
            ExecutionError: If the request could not be answered at all — an
                unknown symbol, a subject that could not be instantiated
            MissingCapabilityError: If the service cannot verify, or an engine
                is given and the service predates ``engines``; nothing is sent
            InvalidRequestError: If the engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        self._require_engine(engine)
        request = sysml_pb2.VerifyConstraintRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject_symbol_id or "",
            engine=_engine_field(engine),
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(
                (CAPABILITY_VERIFICATION,) + self._engine_capabilities(engine)
            )
        ):
            response = self._stub.VerifyConstraint(request)
        return self._verdict_of(response)

    def verify_requirement(self, symbol_id, model_hash, subject_symbol_id=None, engine=None):
        """Ask whether a requirement is satisfied, as ``%requirement`` does.

        Args:
            symbol_id (str): FQN of the requirement definition or usage
            model_hash (str): Hash from ParseFile response
            subject_symbol_id (str, optional): FQN of a part/usage to
                instantiate and evaluate against
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            Verdict: The answer

        Raises:
            WrongKindError: If symbol_id names an element that is not a
                requirement
            ExecutionError: If the request could not be answered at all
            MissingCapabilityError: If the service cannot verify, or an engine
                is given and the service predates ``engines``; nothing is sent
            InvalidRequestError: If the engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        self._require_engine(engine)
        request = sysml_pb2.VerifyRequirementRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject_symbol_id or "",
            engine=_engine_field(engine),
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(
                (CAPABILITY_VERIFICATION,) + self._engine_capabilities(engine)
            )
        ):
            response = self._stub.VerifyRequirement(request)
        return self._verdict_of(response)

    def verify_satisfaction(self, model_hash, symbol_id=None, engine=None):
        """Ask whether the model's satisfaction assertions hold, as ``%satisfy`` does.

        Each assertion is evaluated against an object of its subject, built for
        the call, so a verdict is about the values that subject holds.

        Args:
            model_hash (str): Hash from ParseFile response
            symbol_id (str, optional): FQN limiting evaluation to the assertions
                stated within that element, or to that element itself when it is
                a named satisfaction assertion. Omitted evaluates every
                assertion the model states.
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            list[Verdict]: One verdict per assertion, in declaration order. A
                model stating no assertion gives an empty list. Each verdict's
                ``verifications`` are the cases verifying its own requirement,
                so a call covering several requirements does not mix them.

        Raises:
            WrongKindError: If symbol_id names an element that can state no
                satisfaction assertion
            ExecutionError: If the request could not be answered at all
            MissingCapabilityError: If the service cannot verify, or an engine
                is given and the service predates ``engines``; nothing is sent
            InvalidRequestError: If the engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        self._require_engine(engine)
        request = sysml_pb2.VerifySatisfactionRequest(
            model_hash=model_hash,
            symbol_id=symbol_id or "",
            engine=_engine_field(engine),
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(
                (CAPABILITY_VERIFICATION,) + self._engine_capabilities(engine)
            )
        ):
            response = self._stub.VerifySatisfaction(request)

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise _failure_of(
                response.error, response.failure_reason, diagnostics
            )
        for pb_verdict in response.verdicts:
            _raise_wrong_kind(pb_verdict, diagnostics)
        instances = self._instances_of(response)
        verifications = _verifications_of(response)
        return [
            Verdict(
                pb_verdict,
                instances=instances,
                diagnostics=diagnostics,
                verifications=_verifications_for(verifications, pb_verdict),
            )
            for pb_verdict in response.verdicts
        ]

    def calc(self, symbol_id, model_hash, arguments=None, engine=None):
        """Invoke a calculation, as the REPL's ``%calc`` does.

        Arguments are bound positionally. A calc usage named with no arguments
        binds its inputs from its own members and reports every output feature
        it computes (SysML 7.17).

        Args:
            symbol_id (str): FQN of the calc definition or usage
            model_hash (str): Hash from ParseFile response
            arguments (list, optional): Positional arguments, as Python values
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            CalcResult: The value an invocation returned, or the output features
                a calc usage computed

        Raises:
            WrongKindError: If symbol_id names an element that is not a calc
            ExecutionError: If the calculation could not be evaluated
            MissingCapabilityError: If the service cannot verify, or an
                argument holds a ``complex`` and the service predates
                ``complex_values``, an array, vector or vector quantity and
                the service predates ``structured_values``, a measurement
                reference and the service predates ``measurement_refs``, a
                function and the service predates ``function_values``, a set
                and the service predates ``set_values``, a tensor quantity
                and the service predates ``tensor_values``, a metaobject and
                the service predates ``metaobject_values``, or an engine is
                given and the service predates ``engines``; nothing is sent
            InvalidRequestError: If the engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        self._require_engine(engine)
        request = sysml_pb2.EvaluateCalcRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            arguments=[self._python_to_value(arg) for arg in (arguments or [])],
            engine=_engine_field(engine),
        )
        with translate_rpc_errors(
            unimplemented=self._capability_refusal((
                CAPABILITY_VERIFICATION,
                CAPABILITY_COMPLEX_VALUES,
                CAPABILITY_STRUCTURED_VALUES,
                CAPABILITY_MEASUREMENT_REFS,
                CAPABILITY_FUNCTION_VALUES,
                CAPABILITY_SET_VALUES,
                CAPABILITY_TENSOR_VALUES,
                CAPABILITY_METAOBJECT_VALUES,
            ) + self._engine_capabilities(engine))
        ):
            response = self._stub.EvaluateCalc(request)

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise _failure_of(
                response.error, response.failure_reason, diagnostics
            )

        outputs = {}
        for output in response.outputs:
            try:
                outputs[output.name] = self._value_to_python(output.value)
            except UnsupportedValueError as exc:
                outputs[output.name] = exc
        value = None
        if not outputs and response.HasField('result'):
            value = self._value_to_python(response.result)
        return CalcResult(value, outputs, diagnostics=diagnostics, standing=Standing.of(response))

    def run_analysis(self, symbol_id, model_hash, subject=None, arguments=None,
                     named_arguments=None, schedule=None, engine=None):
        """Run an analysis case, as the REPL's ``%analysis`` does.

        The subject named is instantiated and bound as the case's subject; a
        usage that binds its own subject needs none. Positional arguments bind
        the case's ``in`` parameters in declaration order, the subject excluded;
        named arguments bind them by name. Its objective and each ``assert
        constraint`` in its body are then checked against what it computed
        (SysML 7.22).

        Args:
            symbol_id (str): FQN of the analysis case definition or usage
            model_hash (str): Hash from ParseFile response
            subject (str, optional): FQN of a part/usage to instantiate and run
                the case on
            arguments (list, optional): Positional arguments, as Python values
            named_arguments (dict, optional): Arguments by parameter name
            schedule (str, optional): Scheduling policy the actions the case
                performs resolve their choice points under, as for
                :meth:`execute_action`; ``"explore"`` belongs to
                :meth:`explore_analysis`
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`; ``"explore"`` is the exploring
                schedule by another name and belongs to :meth:`explore_analysis`

        Returns:
            AnalysisResult: The outputs the case computed and the verdict of
                its objective and assertions, with each evaluation a trade
                study made of an alternative

        Raises:
            ValueError: If the schedule or the engine explores
            WrongKindError: If symbol_id names an element that is not an
                analysis case
            AnalysisRunError: If the case could not run to its end and left
                something to inspect — the evaluations made before an
                alternative failed, an objective the failure left undecided;
                carries it as :attr:`~opensysml.errors.AnalysisRunError.result`
            ExecutionError: If the request was refused before the run — an
                unknown symbol — or the failure left nothing to report
            MissingCapabilityError: If the service cannot verify, or an
                argument holds a ``complex`` and the service predates
                ``complex_values``, an array, vector or vector quantity and
                the service predates ``structured_values``, a set and the
                service predates ``set_values``, a tensor quantity and the
                service predates ``tensor_values``, a metaobject and the
                service predates ``metaobject_values``, a schedule is given and
                the service predates ``schedule``, or an engine is given and
                the service predates ``engines``; nothing is sent
            InvalidRequestError: If the schedule names no policy, or the
                engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        _refuse_exploring(schedule, "explore_analysis")
        if _explore_engine(engine):
            raise ValueError(
                "engine 'explore' answers with every outcome, not one run's "
                "result: use explore_analysis"
            )
        response = self._run_analysis(
            symbol_id, model_hash, subject, arguments, named_arguments, schedule, engine
        )

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        # A request refused before the run, or a failure leaving nothing to report, has no partial result.
        if response.error and not (
            response.outputs or response.verdicts or response.evaluations or response.instances
        ):
            raise _failure_of(response.error, response.failure_reason, diagnostics)

        instances = self._instances_of(response)
        by_id = {inst.id: inst for inst in instances}
        # An object the response carries is read as it; one it does not stays an id.
        resolve = lambda instance_id: by_id.get(instance_id, instance_id)  # noqa: E731
        outputs = {}
        for output in response.outputs:
            try:
                outputs[output.name] = value_to_python(output.value, resolve)
            except UnsupportedValueError as exc:
                outputs[output.name] = exc
        verdicts = [
            Verdict(pb_verdict, instances=instances, diagnostics=diagnostics)
            for pb_verdict in response.verdicts
        ]
        result = AnalysisResult(
            outputs,
            verdicts,
            instances=instances,
            diagnostics=diagnostics,
            verifications=_verifications_of(response),
            evaluations=_evaluations_of(response, resolve),
            standing=Standing.of(response),
        )
        if response.error:
            raise AnalysisRunError(response.error, result, diagnostics=diagnostics)
        return result

    def explore_analysis(self, symbol_id, model_hash, subject=None, arguments=None,
                         named_arguments=None, schedule="explore"):
        """Run an analysis case once per valid order of the choice points its actions meet.

        Runs agreeing on the case's outputs and on its objective and assertion
        verdicts are one outcome; a verdict is reported among the outcome's
        outputs as ``objective <name>`` or ``assertion <name>``, and what a
        verification case's body answered as ``verdict <case>``. See
        :meth:`explore_action`.

        Args:
            symbol_id (str): FQN of the analysis case definition or usage
            model_hash (str): Hash from ParseFile response
            subject (str, optional): FQN of a part/usage to instantiate and run
                the case on
            arguments (list, optional): Positional arguments, as Python values
            named_arguments (dict, optional): Arguments by parameter name
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``

        Returns:
            Exploration: Every distinct outcome reached and how the exploration
                ended

        Raises:
            ValueError: If the schedule does not explore
            WrongKindError: If symbol_id names an element that is not an
                analysis case
            ExecutionError: If the case could not be explored at all — an
                unbound subject, an argument of the wrong kind
            MissingCapabilityError: If the service cannot verify, predates
                ``schedule_explore`` or lacks a value capability an argument
                needs; nothing is sent
            InvalidRequestError: If the schedule's options are malformed
            ModelNotFoundError: If the service no longer holds the model
        """
        _require_exploring(schedule)
        response = self._run_analysis(
            symbol_id, model_hash, subject, arguments, named_arguments, schedule, None
        )
        return self._exploration_of(response, response.failure_reason)

    def _run_analysis(self, symbol_id, model_hash, subject, arguments,
                      named_arguments, schedule, engine):
        """Send a RunAnalysis request, its capabilities checked first."""
        self._require_verification()
        self._require_schedule(schedule)
        self._require_engine(engine)
        request = sysml_pb2.RunAnalysisRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject or "",
            arguments=[self._python_to_value(arg) for arg in (arguments or [])],
            schedule=schedule or "",
            engine=_engine_field(engine),
        )
        for name, arg in (named_arguments or {}).items():
            request.named_arguments[name].CopyFrom(self._python_to_value(arg))
        with translate_rpc_errors(
            unimplemented=self._capability_refusal([
                CAPABILITY_VERIFICATION,
                CAPABILITY_COMPLEX_VALUES,
                CAPABILITY_STRUCTURED_VALUES,
                CAPABILITY_MEASUREMENT_REFS,
                CAPABILITY_SET_VALUES,
                CAPABILITY_TENSOR_VALUES,
                CAPABILITY_METAOBJECT_VALUES,
                *self._schedule_capabilities(schedule),
                *self._engine_capabilities(engine),
            ])
        ):
            return self._stub.RunAnalysis(request)

    def run_sweep(self, symbol_id, model_hash, ranges, subject=None,
                  arguments=None, named_arguments=None, samples=0, seed=0, engine=None):
        """Run an analysis case or calc once per row of a parameter sweep.

        Every row is an ordinary run of that target with the swept parameters
        bound to the row's values and the other arguments as given, so nothing
        about how one run executes changes. Several ranges make one row per
        point of their cartesian product, the first varying slowest. Passing
        ``samples`` draws that many rows uniformly from each range instead of
        stepping through it, in draw order, from ``seed``: the same seed draws
        the same table. A run that failed is a row carrying its error.

        Args:
            symbol_id (str): FQN of the analysis case or calc
            model_hash (str): Hash from ParseFile response
            ranges (dict): Range per swept parameter, as
                ``{"speed": (0, 10, 2)}`` or ``{"speed": (0.0, 10.0)}`` where
                the rows are drawn; a range between whole numbers steps by one
                where it states no step, one with a fractional endpoint must
                state one; the values are typed by the parameter they bind
            subject (str, optional): FQN of a part/usage to instantiate and run
                an analysis case on
            arguments (list, optional): Positional arguments every row binds
            named_arguments (dict, optional): Arguments by name every row binds
            samples (int, optional): Rows to draw rather than step through
            seed (int, optional): Seed the draws are taken from
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            SweepTable: One row per run, in the order the runs were made

        Raises:
            WrongKindError: If symbol_id names neither an analysis case nor a
                calc
            ExecutionError: If no run followed from the request — a parameter
                the target does not declare, a range no values follow from, a
                sample count of none, more runs than the service's budget, an
                engine that does not answer sweeps
            MissingCapabilityError: If the service cannot verify, or an engine
                is given and the service predates ``engines``; nothing is sent
            InvalidRequestError: If the engine names none the service registers
            ModelNotFoundError: If the service no longer holds the model
        """
        self._require_verification()
        self._require_engine(engine)
        request = sysml_pb2.RunSweepRequest(
            model_hash=model_hash,
            symbol_id=symbol_id,
            subject_symbol_id=subject or "",
            arguments=[self._python_to_value(arg) for arg in (arguments or [])],
            samples=samples,
            seed=seed,
            engine=_engine_field(engine),
        )
        for name, arg in (named_arguments or {}).items():
            request.named_arguments[name].CopyFrom(self._python_to_value(arg))
        for name, bounds in (ranges or {}).items():
            request.ranges.append(self._sweep_range(name, bounds))
        with translate_rpc_errors(
            unimplemented=self._capability_refusal(
                (CAPABILITY_VERIFICATION, CAPABILITY_COMPLEX_VALUES, CAPABILITY_STRUCTURED_VALUES)
                + self._engine_capabilities(engine)
            )
        ):
            response = self._stub.RunSweep(request)

        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise _failure_of(
                response.error, response.failure_reason, diagnostics
            )
        instances = self._instances_of(response)
        rows = [
            self._sweep_row(row, instances, diagnostics)
            for row in response.rows
        ]
        return SweepTable(
            rows, list(response.parameters), sampled=response.sampled,
            seed=response.seed, instances=instances, diagnostics=diagnostics,
            standing=Standing.of(response),
        )

    def _sweep_range(self, name, bounds):
        """One parameter's range as the wire states it: two endpoints, or three
        where the range states the step it advances by."""
        if len(bounds) not in (2, 3):
            raise InvalidRequestError(
                f"range of {name} takes (from, to) or (from, to, step), "
                f"not {len(bounds)} value(s)"
            )
        pb_range = sysml_pb2.SweepRange(parameter=name)
        pb_range.start.CopyFrom(self._python_to_value(bounds[0]))
        pb_range.end.CopyFrom(self._python_to_value(bounds[1]))
        if len(bounds) == 3:
            pb_range.step.CopyFrom(self._python_to_value(bounds[2]))
        return pb_range

    def _sweep_row(self, row, instances, diagnostics):
        """One run of a sweep as Python values."""
        by_id = {inst.id: inst for inst in instances}
        resolve = lambda instance_id: by_id.get(instance_id, instance_id)  # noqa: E731
        inputs = {}
        outputs = {}
        for target, source in ((inputs, row.inputs), (outputs, row.outputs)):
            for entry in source:
                try:
                    target[entry.name] = value_to_python(entry.value, resolve)
                except UnsupportedValueError as exc:
                    target[entry.name] = exc
        verdicts = [
            Verdict(pb_verdict, instances=instances, diagnostics=diagnostics)
            for pb_verdict in row.verdicts
        ]
        return SweepRow(
            inputs, outputs, verdicts, row.elapsed_micros / 1e6,
            error=row.error, evaluations=_evaluations_of(row, resolve),
        )

    def _require_verification(self):
        """Refuse a verification the connected service does not implement."""
        require(
            self.server_info(),
            CAPABILITY_VERIFICATION,
            upgrade_remedy(CAPABILITY_VERIFICATION),
        )

    def _require_engines(self):
        """Refuse to ask about engines a service without ``engines`` does not list."""
        require(self.server_info(), CAPABILITY_ENGINES, upgrade_remedy(CAPABILITY_ENGINES))

    def _require_engine(self, engine):
        """Refuse to send an engine selection a service without ``engines`` would run under auto."""
        for capability in self._engine_capabilities(engine):
            require(self.server_info(), capability, upgrade_remedy(capability))

    @staticmethod
    def _engine_capabilities(engine):
        """The capabilities an engine selection needs of the service: none for auto."""
        if not _engine_field(engine):
            return ()
        if _explore_engine(engine):
            return (CAPABILITY_ENGINES, CAPABILITY_SCHEDULE_EXPLORE)
        return (CAPABILITY_ENGINES,)

    def _capability_refusal(self, capabilities):
        """Translate a capability-gated UNIMPLEMENTED into the preflight error."""
        capabilities = tuple(capabilities)
        if not capabilities:
            return None

        def refused(details):
            capability = next(
                (name for name in capabilities if name in details),
                capabilities[0],
            )
            return MissingCapabilityError(
                capability,
                self.server_info(),
                upgrade_remedy(capability),
            )

        return refused

    def _verdict_of(self, response):
        """Wrap a single-verdict verification response, raising its failure."""
        diagnostics = [Diagnostic(d) for d in response.diagnostics]
        if response.error:
            raise ExecutionError(response.error, diagnostics=diagnostics)
        _raise_wrong_kind(response.verdict, diagnostics)
        return Verdict(
            response.verdict,
            instances=self._instances_of(response),
            diagnostics=diagnostics,
            verifications=_verifications_of(response),
        )

    def _require_complex_values(self):
        """Refuse to send a complex a service without ``complex_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_COMPLEX_VALUES,
            upgrade_remedy(CAPABILITY_COMPLEX_VALUES),
        )

    def _require_structured_values(self):
        """Refuse to send an array, vector or vector quantity a service without ``structured_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_STRUCTURED_VALUES,
            upgrade_remedy(CAPABILITY_STRUCTURED_VALUES),
        )

    def _require_measurement_refs(self):
        """Refuse to send a measurement reference a service without ``measurement_refs`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_MEASUREMENT_REFS,
            upgrade_remedy(CAPABILITY_MEASUREMENT_REFS),
        )

    def _require_function_values(self):
        """Refuse to send a function a service without ``function_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_FUNCTION_VALUES,
            upgrade_remedy(CAPABILITY_FUNCTION_VALUES),
        )

    def _require_metaobject_values(self):
        """Refuse to send a metaobject a service without ``metaobject_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_METAOBJECT_VALUES,
            upgrade_remedy(CAPABILITY_METAOBJECT_VALUES),
        )

    def _require_infinity_value(self):
        """Refuse to send the unbounded value ``*`` a service without ``infinity_value`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_INFINITY_VALUE,
            upgrade_remedy(CAPABILITY_INFINITY_VALUE),
        )

    def _require_schedule(self, schedule):
        """Refuse to send a schedule a service without ``schedule`` would run under the default."""
        for capability in self._schedule_capabilities(schedule):
            require(self.server_info(), capability, upgrade_remedy(capability))

    @staticmethod
    def _schedule_capabilities(schedule):
        """The capabilities a schedule spelling needs of the service: none for the default."""
        capabilities = []
        if schedule:
            capabilities.append(CAPABILITY_SCHEDULE)
        if _explores(schedule):
            capabilities.append(CAPABILITY_SCHEDULE_EXPLORE)
        return capabilities

    def _require_set_values(self):
        """Refuse to send a set a service without ``set_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_SET_VALUES,
            upgrade_remedy(CAPABILITY_SET_VALUES),
        )

    def _require_tensor_values(self):
        """Refuse to send a tensor quantity a service without ``tensor_values`` would read as null."""
        require(
            self.server_info(),
            CAPABILITY_TENSOR_VALUES,
            upgrade_remedy(CAPABILITY_TENSOR_VALUES),
        )

    def _require_feature_values(self):
        """Refuse instances from a service that populates only the removed `slots` field."""
        require(
            self.server_info(),
            CAPABILITY_FEATURE_VALUES,
            upgrade_remedy(CAPABILITY_FEATURE_VALUES),
        )

    def _instances_of(self, response):
        """Wrap the instance graph a verification returned, roots first."""
        from opensysml.instance import Instance

        if response.instances:
            self._require_feature_values()
        graph = {inst.id: inst for inst in response.instances}
        wrappers = {}
        return [
            Instance(pb_inst, graph, _wrappers=wrappers)
            for pb_inst in response.instances
        ]

    def _python_to_value(self, py_value):
        """Convert Python type to protobuf Value."""
        from opensysml.instance import Instance
        
        if isinstance(py_value, bool):
            return sysml_pb2.Value(bool_value=py_value)
        elif isinstance(py_value, InstanceRef):
            return sysml_pb2.Value(instance_id=py_value.id)
        elif isinstance(py_value, int):
            return sysml_pb2.Value(int_value=py_value)
        elif isinstance(py_value, float):
            return sysml_pb2.Value(real_value=py_value)
        elif isinstance(py_value, complex):
            self._require_complex_values()
            return sysml_pb2.Value(complex=sysml_pb2.Complex(
                real=py_value.real, imaginary=py_value.imag,
            ))
        elif isinstance(py_value, str):
            return sysml_pb2.Value(string_value=py_value)
        elif py_value is None:
            return sysml_pb2.Value(null="")
        elif isinstance(py_value, Instance):
            return sysml_pb2.Value(instance_id=py_value.id)
        elif isinstance(py_value, Quantity):
            return sysml_pb2.Value(quantity=py_value.to_pb())
        elif isinstance(py_value, _Infinity):
            self._require_infinity_value()
            return sysml_pb2.Value(infinity=True)
        elif isinstance(py_value, MeasurementRef):
            self._require_measurement_refs()
            return sysml_pb2.Value(measurement_ref=py_value.to_pb())
        elif isinstance(py_value, Function):
            self._require_function_values()
            return sysml_pb2.Value(function=py_value.to_pb())
        elif isinstance(py_value, Metaobject):
            self._require_metaobject_values()
            return sysml_pb2.Value(metaobject=py_value.to_pb())
        elif isinstance(py_value, Array):
            self._require_structured_values()
            return sysml_pb2.Value(array=py_value.to_pb(self._python_to_value))
        elif isinstance(py_value, Vector):
            self._require_structured_values()
            return sysml_pb2.Value(vector=py_value.to_pb())
        elif isinstance(py_value, VectorQuantity):
            self._require_structured_values()
            return sysml_pb2.Value(vector_quantity=py_value.to_pb())
        elif isinstance(py_value, (SetValue, set, frozenset)):
            self._require_set_values()
            return sysml_pb2.Value(set=SetValue(py_value).to_pb(self._python_to_value))
        elif isinstance(py_value, TensorQuantity):
            self._require_tensor_values()
            return sysml_pb2.Value(tensor_quantity=py_value.to_pb())
        elif isinstance(py_value, EnumLiteral):
            return sysml_pb2.Value(enum_literal=sysml_pb2.EnumLiteral(
                literal_id=py_value.literal_id,
                enumeration_id=py_value.enumeration_id,
                name=py_value.name,
            ))
        elif isinstance(py_value, list):
            elements = [self._python_to_value(v) for v in py_value]
            return sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=elements))
        else:
            raise ValueError(f"Unsupported Python type: {type(py_value)}")
    
    def _value_to_python(self, pb_value):
        """Convert protobuf Value to Python type.

        Instance references outside an Instantiate response are returned as an
        :class:`InstanceRef`; there is no instance graph to resolve them against.
        """
        return value_to_python(pb_value)

    def _values_to_python(self, pb_values):
        """Convert a name → Value map, keeping an unsupported value as its error.

        Mirrors Instance.feature_values: one value the wire format cannot represent must
        not discard the entries around it.
        """
        result = {}
        for name, pb_value in pb_values.items():
            try:
                result[name] = self._value_to_python(pb_value)
            except UnsupportedValueError as exc:
                result[name] = exc
        return result
    
    def _running_service_info(self, timeout=5.0):
        """Ask the service already listening what it is, over a channel of its own.

        Args:
            timeout (float): RPC timeout in seconds

        Returns:
            ServerInfo or None: What it reported; ``answered`` is False when it
                predates the handshake, which is itself an answer, and None when
                the call failed, so nothing was learned
        """
        channel = open_channel(self._address)
        try:
            stub = sysml_pb2_grpc.SysMLServiceStub(channel)
            response = stub.GetServerInfo(
                sysml_pb2.ServerInfoRequest(), timeout=timeout
            )
        except grpc.RpcError as e:
            if e.code() != grpc.StatusCode.UNIMPLEMENTED:
                return None
            return ServerInfo(
                version='',
                capabilities=frozenset(),
                answered=False,
                origin=self._origin,
            )
        finally:
            channel.close()
        return ServerInfo(
            version=response.version,
            capabilities=frozenset(response.capabilities),
            answered=True,
            origin=self._origin,
        )

    def _required_release(self):
        """Release tag the service must report, or None if none is required.

        An unresolvable 'latest' requires nothing, as for the binary cache. The
        answer is resolved once, so every check of a connection uses the same one.
        """
        if self._resolved_release is _UNRESOLVED:
            if self._version != 'latest':
                self._resolved_release = self._version
            else:
                try:
                    self._resolved_release = resolve_latest_version()
                except ConnectionError:
                    self._resolved_release = None
        return self._resolved_release

    def _ensure_service(self):
        """Join this interpreter's private service, starting it if there is none.

        The child is given its port by the kernel and reports it, so no free port
        is chosen here and then competed for, and there is no fixed port to
        collide on. The last connection to release it stops it; so does the exit
        of this process, however it exits.

        Raises:
            ConnectionError: If a child cannot be started, or does not serve
        """
        service = _join_private_service(self._version, self._required_release())
        self._private = service
        self.host, self.port = split_target(service.address)
        self._address = service.address
        self._origin = f"{service.binary_path}, started by this client"
        atexit.register(self._cleanup_service)

    def _cleanup_service(self):
        """Release this connection's hold, stopping the child if it was the last.

        The hold is dropped before the child is stopped, so a second call cannot
        stop it twice, and a connection that holds nothing stops nothing.
        """
        if self._cleaned_up or self._private is None:
            return
        self._cleaned_up = True
        service, self._private = self._private, None
        with _private_services_lock:
            service.refs -= 1
            if service.refs > 0:
                return
            if _private_services.get(service.key) is service:
                del _private_services[service.key]
        service.stop()

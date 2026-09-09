"""What a connected sysml-grpc service can do, and how a client requires it.

The service is released separately from this client, so a client that needs a
feature must ask for it rather than infer it. It asks by *capability name*, not
by version: version strings of forks and development builds are not ordered
against each other (a source build reports ``dev``), while a capability name is
an exact contract the service either reports or does not.

A service too old to answer ``GetServerInfo`` fails the call with
``UNIMPLEMENTED``, which is itself an answer — :data:`ServerInfo.answered` is
then ``False``, and no capability is claimed.
"""

from dataclasses import dataclass
from typing import FrozenSet, Iterable, Optional

from opensysml.binary import get_binary_path
from opensysml.errors import OpenSysMLError

#: Static type facts on ``SymbolInfo`` — ``type_info``, ``multiplicity`` and
#: ``specializations``. Typed code generation requires this: without it every
#: feature looks untyped, which is indistinguishable from a feature that is
#: genuinely untyped.
CAPABILITY_TYPE_FACTS = "type_facts"

#: The ``Convert`` RPC, which writes a model back out as SysML notation or RDF
#: Turtle. Without it the service refuses conversion with ``UNIMPLEMENTED``.
CAPABILITY_CONVERT = "convert"

#: The verification RPCs — ``VerifyConstraint``, ``VerifyRequirement``,
#: ``VerifySatisfaction`` and ``EvaluateCalc`` — which answer the questions the
#: REPL's ``%constraint``, ``%requirement``, ``%satisfy`` and ``%calc`` answer.
#: Without it the service refuses those calls with ``UNIMPLEMENTED``.
CAPABILITY_VERIFICATION = "verification"

#: The ``Query`` RPC, which evaluates a SysML v2 API & Services ``Query`` over a
#: loaded model. Without it the service refuses queries with ``UNIMPLEMENTED``.
CAPABILITY_QUERY = "query"

#: The ``RunDocumentQuery`` RPC, which runs a named document query and answers
#: typed rows. Without it the service refuses with ``UNIMPLEMENTED``.
CAPABILITY_DOCUMENT_QUERY = "document_query"

#: The ``RenderDocument`` RPC, which renders a named document to Markdown.
#: Without it the service refuses with ``UNIMPLEMENTED``.
CAPABILITY_RENDER_DOCUMENT = "render_document"

#: An enumeration literal as ``Value.enum_literal``. Without it a literal is
#: reported as an unsupported null, which is indistinguishable from a value the
#: service could not evaluate.
CAPABILITY_ENUM_VALUES = "enum_values"

#: Evaluating an expression against an instantiated subject. Without it the
#: service refuses a request that names a subject with ``UNIMPLEMENTED``.
CAPABILITY_EVALUATE_SUBJECT = "evaluate_subject"

#: Populated ``SymbolInfo.attributes``. Without it the attribute set is empty,
#: which is indistinguishable from an element that has no attributes.
CAPABILITY_SYMBOL_ATTRIBUTES = "symbol_attributes"

#: A valueless feature of a value type as ``Value.unset``, read as
#: :data:`opensysml.UNSET`. Without it the empty object such a feature materializes
#: crosses as an instance id, which is indistinguishable from an object of a
#: class that declares no features.
CAPABILITY_UNSET_VALUE = "unset_value"

#: An object's values as ``Instance.feature_values``, which replaced the pre-0.1.0
#: ``Instance.slots``. Without it every instance arrives with no values at all,
#: which is indistinguishable from an object whose features are all unset.
CAPABILITY_FEATURE_VALUES = "feature_values"

#: The ``ApplyEdits`` RPC, which edits a loaded model's own source and hands back
#: the edited notation. Without it the service refuses edits with
#: ``UNIMPLEMENTED``.
CAPABILITY_APPLY_EDITS = "apply_edits"
#: Source-preserving add-member and delete authoring operations.
CAPABILITY_AUTHORING = "authoring"
#: Declares the language of inline content passed to ``ParseFile``.
CAPABILITY_INLINE_LANGUAGE = "inline_language"
#: ``ParseFileRequest.strict_conformance``, which asks whether the source is
#: conforming SysML v2 rather than accepting the OpenSysML notation extensions.
#: Without it the service refuses a request that sets the field.
CAPABILITY_STRICT_CONFORMANCE = "strict_conformance"

#: A complex number as ``Value.complex``, read as a Python ``complex``. Without it
#: the service sends an unsupported null naming the value, which is an error.
CAPABILITY_COMPLEX_VALUES = "complex_values"

#: An array, a vector and a vector quantity as ``Value.array``, ``Value.vector``
#: and ``Value.vector_quantity``, read as :class:`~opensysml.values.Array`,
#: :class:`~opensysml.values.Vector` and :class:`~opensysml.values.VectorQuantity`.
#: Without it the service sends an unsupported null naming the value, which is
#: an error, and refuses one sent to it with ``UNIMPLEMENTED``.
CAPABILITY_STRUCTURED_VALUES = "structured_values"

#: A bare measurement reference — a unit with no magnitude, ``SI::m`` or
#: ``m / s`` — as ``Value.measurement_ref``, read as
#: :class:`~opensysml.values.MeasurementRef`. Without it the service sends an
#: unsupported null naming the unit, which is an error, and refuses one sent to
#: it with ``UNIMPLEMENTED``.
CAPABILITY_MEASUREMENT_REFS = "measurement_refs"

#: A calc held as a value — a calc definition, or a calc usage with an input no
#: read could supply — as ``Value.function``, named by its declaration and read
#: as :class:`~opensysml.values.Function`. Without it the service sends an
#: unsupported null naming the calc, which is an error, and refuses one sent to
#: it with ``UNIMPLEMENTED``.
CAPABILITY_FUNCTION_VALUES = "function_values"

#: A unique, unordered collection — a ``Collections::Set``'s elements — as
#: ``Value.set``, read as :class:`~opensysml.values.SetValue` with each element
#: once in canonical order. Without it the service sends an unsupported null
#: naming the value, which is an error, and refuses one sent to it with
#: ``UNIMPLEMENTED``.
CAPABILITY_SET_VALUES = "set_values"

#: A tensor quantity of any rank as ``Value.tensor_quantity``, read as
#: :class:`~opensysml.values.TensorQuantity`. Without it the service sends an
#: unsupported null naming the value, which is an error, and refuses one sent to
#: it with ``UNIMPLEMENTED``.
CAPABILITY_TENSOR_VALUES = "tensor_values"

#: What the body of a verification case answered, as the
#: ``verification_verdicts`` of a requirement, satisfaction or analysis
#: response, read as :class:`~opensysml.verdict.VerificationVerdict`. Without it
#: the service reports satisfaction verdicts alone and refuses to run a
#: verification case.
CAPABILITY_VERIFICATION_VERDICTS = "verification_verdicts"

#: The unbounded value ``*`` as ``Value.infinity``, read as
#: :data:`~opensysml.values.INFINITY`. Without it the service sends an
#: unsupported null naming it, which is an error, and refuses one sent to it
#: with ``UNIMPLEMENTED``.
CAPABILITY_INFINITY_VALUE = "infinity_value"

#: ``Diagnostic.code`` populated, read as :attr:`~opensysml.diagnostic.Diagnostic.code`,
#: so an empty code is a finding none was assigned. Without it every code is empty.
CAPABILITY_DIAGNOSTIC_CODES = "diagnostic_codes"

#: The ``schedule`` field of an action, state or analysis run, naming the
#: scheduling policy its choice points are resolved under: ``declared``,
#: ``reverse`` (the default), ``seed:<n>`` or ``explore``. Without it the service
#: would drop the field and run under the default, so the client refuses to send one.
CAPABILITY_SCHEDULE = "schedule"

#: Each application an analysis run made of one of the case's calcs as a value —
#: a trade study's evaluation of each alternative — as ``RunAnalysisResponse.evaluations``,
#: read as :class:`~opensysml.verdict.CaseEvaluation`, and what a failed run
#: computed kept beside its error. Without it a run reports no evaluation and a
#: failed run its error alone.
CAPABILITY_CASE_EVALUATIONS = "case_evaluations"

#: The ``explore[:runs=<n>,depth=<d>]`` schedule, under which a run answers with
#: every distinct outcome as ``outcomes`` and an ``exploration`` status instead
#: of one run's result. Without it the service refuses the schedule with
#: ``UNIMPLEMENTED``, so the client refuses to send one.
CAPABILITY_SCHEDULE_EXPLORE = "schedule_explore"

#: ``final_time`` populated on an action or state run's response: the run's
#: simulation clock when it ended, in seconds, read as the ``final_time`` of
#: :meth:`~opensysml.connection.Connection.execute_state`. Without it the
#: field is 0 whatever the run waited on.
CAPABILITY_FINAL_TIME = "final_time"


@dataclass(frozen=True)
class ServerInfo:
    """Self-description of the service a :class:`~opensysml.connection.Connection` talks to.

    Attributes:
        version: Build version the service reports, informational only. Empty
            when the service is too old to answer.
        capabilities: Capability names the service reports.
        answered: Whether the service answered the handshake at all. ``False``
            means it predates ``GetServerInfo``.
        origin: Human-readable provenance of the service — the binary path this
            client started, or the address it connected to. Used to name the
            offending binary in an error message.
    """

    version: str
    capabilities: FrozenSet[str]
    answered: bool
    origin: str

    def has(self, capability: str) -> bool:
        """Whether the service reports ``capability``."""
        return capability in self.capabilities

    def describe(self) -> str:
        """One-line description of the service, for an error message."""
        version = self.version or "unknown"
        if not self.answered:
            return (
                f"{self.origin} (version unknown: too old to answer GetServerInfo, "
                f"so it predates every capability)"
            )
        reported = ", ".join(sorted(self.capabilities)) or "none"
        return f"{self.origin} (version {version}, capabilities: {reported})"


class MissingCapabilityError(OpenSysMLError):
    """Raised when the connected service cannot supply a required capability.

    Attributes:
        capability (str): Capability name that was required
        info (ServerInfo): What the service reported about itself
    """

    def __init__(self, capability: str, info: ServerInfo, remedy: str):
        super().__init__(
            f"the sysml-grpc service does not support the {capability!r} capability, "
            f"which this operation requires.\n"
            f"  service: {info.describe()}\n"
            f"  fix:     {remedy}"
        )
        self.capability = capability
        self.info = info


def upgrade_remedy(capability: str) -> str:
    """Remedy for a service lacking ``capability``, naming both routes to one that has it."""
    try:
        cached = f"cached at {get_binary_path()}"
    except OpenSysMLError:
        # A platform with no release build of its own still gets the advice.
        cached = "cached locally"
    return (
        f"run a sysml-grpc whose GetServerInfo reports {capability!r}: set "
        f"$OPENSYSML_GRPC_VERSION to a release that has it, which replaces the binary "
        f"{cached} when that is another release, or build one with `make build-grpc` "
        f"and start it yourself"
    )


def mismatch_reason(
    info: ServerInfo, version: Optional[str] = None,
    capabilities: Iterable[str] = (),
) -> Optional[str]:
    """Why ``info`` is not the service that was asked for, or ``None`` when it is.

    A release is compared as an exact tag, since version strings are not ordered:
    a build that cannot be shown to be the one asked for is a mismatch.

    Args:
        info: What the running service reported about itself
        version: Release tag the client asks for, or ``None`` to ask for none
        capabilities: Capability names the client asks for

    Returns:
        How the service differs from what was asked for, or ``None``
    """
    reasons = []
    if version is not None:
        if not info.answered:
            reasons.append(
                f"it did not answer GetServerInfo, so it cannot be shown to be "
                f"the {version} that was asked for"
            )
        elif info.version != version:
            reasons.append(
                f"it reports version {info.version or 'unknown'}, but "
                f"{version} was asked for"
            )
    missing = sorted(c for c in capabilities if not info.has(c))
    if missing:
        named = ", ".join(repr(c) for c in missing)
        noun = "capabilities" if len(missing) > 1 else "capability"
        reasons.append(f"it does not report the {named} {noun} this client requires")
    return "; ".join(reasons) or None


def require(info: ServerInfo, capability: str, remedy: str) -> None:
    """Raise :class:`MissingCapabilityError` unless ``info`` reports ``capability``."""
    if not info.has(capability):
        raise MissingCapabilityError(capability, info, remedy)

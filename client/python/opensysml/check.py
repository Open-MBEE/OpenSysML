"""The answer to ``CheckBehavior``: a behavior searched over every schedule.

:meth:`~opensysml.Model.check_action` and :meth:`~opensysml.Model.check_state_machine`
answer a :class:`CheckResult`, the report ``sysml -check-property`` prints with
every violation, divergence and final outcome witnessed as a replay schedule.
"""

from dataclasses import dataclass, field

from opensysml.engines import Standing

#: The verdicts a check answers.
VERDICT_HOLDS = "holds"
VERDICT_FAILS = "fails"
VERDICT_UNDECIDED = "undecided"


@dataclass(frozen=True)
class CheckWitness:
    """One schedule replayed: pass ``schedule`` as a run's schedule to follow it.

    Attributes:
        schedule: The replay schedule policy
        draws: The draws the schedule took
        choices: The choices it made at each choice point
        inputs: The inputs the symbolic engine chose, as ``name = value``
        file_name: The witness file's name when one was asked for, else empty
        file_content: That file's content
    """

    schedule: str = ""
    draws: tuple = field(default_factory=tuple)
    choices: tuple = field(default_factory=tuple)
    inputs: tuple = field(default_factory=tuple)
    file_name: str = ""
    file_content: str = ""

    @classmethod
    def of(cls, pb):
        if pb is None:
            return None
        return cls(pb.schedule, tuple(pb.draws), tuple(pb.choices), tuple(pb.inputs),
                   pb.file_name, pb.file_content)


@dataclass(frozen=True)
class CheckViolation:
    """One violation witnessed.

    Attributes:
        kind: ``property``, ``deadlock`` or ``failure``
        property: The property false at the state, for a property violation
        error: The failure's message, for a failure
        depth: Moves from the start to the violating state
        probability: The schedule's probability under a weighted scheduler, else 0
        witness: The schedule reaching it
        text: The violation as the report prints it
    """

    kind: str
    property: str = ""
    error: str = ""
    depth: int = 0
    probability: float = 0.0
    witness: CheckWitness | None = None
    text: str = ""

    @classmethod
    def of(cls, pb):
        return cls(pb.kind, pb.property, pb.error, int(pb.depth), pb.probability,
                   CheckWitness.of(pb.witness) if pb.HasField("witness") else None, pb.text)


@dataclass(frozen=True)
class CheckDivergentValue:
    """One final value a feature reaches and the schedule reaching it."""

    value: str
    witness: CheckWitness | None = None

    @classmethod
    def of(cls, pb):
        return cls(pb.value, CheckWitness.of(pb.witness) if pb.HasField("witness") else None)


@dataclass(frozen=True)
class CheckDivergence:
    """A feature whose final value the schedule decides."""

    feature: str
    values: tuple = field(default_factory=tuple)

    @classmethod
    def of(cls, pb):
        return cls(pb.feature, tuple(CheckDivergentValue.of(v) for v in pb.values))


@dataclass(frozen=True)
class CheckFinal:
    """One final outcome of an exhaustive search."""

    outcome: str
    values: dict = field(default_factory=dict)
    witness: CheckWitness | None = None

    @classmethod
    def of(cls, pb):
        return cls(pb.outcome, dict(pb.values),
                   CheckWitness.of(pb.witness) if pb.HasField("witness") else None)


@dataclass(frozen=True)
class CheckSearch:
    """The explicit-state search's statistics."""

    states: int = 0
    moves: int = 0
    max_depth: int = 0
    bounds_hit: tuple = field(default_factory=tuple)
    horizon: float = 0.0
    horizon_set: bool = False
    mass_bounded: bool = False
    notes: tuple = field(default_factory=tuple)
    scope: tuple = field(default_factory=tuple)
    status: str = ""

    @classmethod
    def of(cls, pb):
        return cls(int(pb.states), int(pb.moves), int(pb.max_depth), tuple(pb.bounds_hit), pb.horizon,
                   pb.horizon_set, pb.mass_bounded, tuple(pb.notes), tuple(pb.scope), pb.status)


@dataclass(frozen=True)
class CheckResult:
    """What a check answered.

    Attributes:
        verdict: ``holds``, ``fails`` or ``undecided``
        lines: The report as ``sysml -check-property`` prints it
        search: The explicit-state search's statistics, None for a symbolic answer
        violations: Each property, deadlock or failure witnessed
        divergences: Each feature whose final value the schedule decided
        finals: Each final outcome of an exhaustive search
        claim: The symbolic engine's claim
        free_inputs: The inputs it ranged over
        assumptions: What it assumed
        witness: Its witness; ``contrast`` its counter-witness
        standing: The engine, its strength and the bounds it ran under
        reason: Why the verdict is undecided, when it is
    """

    verdict: str
    lines: tuple = field(default_factory=tuple)
    search: CheckSearch | None = None
    violations: tuple = field(default_factory=tuple)
    divergences: tuple = field(default_factory=tuple)
    finals: tuple = field(default_factory=tuple)
    claim: str = ""
    free_inputs: tuple = field(default_factory=tuple)
    assumptions: tuple = field(default_factory=tuple)
    witness: CheckWitness | None = None
    contrast: CheckWitness | None = None
    standing: Standing = field(default_factory=Standing)
    reason: str = ""

    @property
    def holds(self):
        """Whether every schedule kept every property."""
        return self.verdict == VERDICT_HOLDS

    def __str__(self):
        return "\n".join(self.lines) if self.lines else self.verdict


def check_result(response):
    """Build a :class:`CheckResult` from a ``CheckBehaviorResponse``."""
    return CheckResult(
        verdict=response.verdict,
        lines=tuple(response.lines),
        search=CheckSearch.of(response.search) if response.HasField("search") else None,
        violations=tuple(CheckViolation.of(v) for v in response.violations),
        divergences=tuple(CheckDivergence.of(d) for d in response.divergences),
        finals=tuple(CheckFinal.of(f) for f in response.finals),
        claim=response.claim,
        free_inputs=tuple(response.free_inputs),
        assumptions=tuple(response.assumptions),
        witness=CheckWitness.of(response.witness) if response.HasField("witness") else None,
        contrast=CheckWitness.of(response.contrast) if response.HasField("contrast") else None,
        standing=Standing.of(response),
        reason=response.reason,
    )

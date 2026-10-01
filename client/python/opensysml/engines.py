"""The analysis engines the service answers with, and the standing of an answer.

Every verdict, calc result, analysis result and sweep table names the engine
that answered it, the strength of the evidence it earned and the bounds it ran
under; a service predating the ``engines`` capability reports none of them.
Selecting an engine is spelled as ``sysml -engine`` is: ``"auto"`` (or none)
for the strongest covering engine, ``"all"`` for every covering one composed,
or one engine by name.
"""

from dataclasses import dataclass, field

#: Strengths, weakest first, as the service spells them.
STRENGTH_NOT_COVERED = "not covered"
STRENGTH_OBSERVED = "observed"
STRENGTH_WITNESSED = "witnessed"
STRENGTH_BOUNDED = "bounded"
STRENGTH_PROVED = "proved"

#: Engine selections that name no single engine.
ENGINE_AUTO = "auto"
ENGINE_ALL = "all"


@dataclass(frozen=True)
class Bound:
    """One limit an engine ran under.

    Attributes:
        name: What the limit was on: ``steps``, ``runs``, ``depth``, ``elements``
        limit: The limit itself
        reached: Whether the engine stopped at it, which lowered its strength
    """

    name: str
    limit: int
    reached: bool = False

    def __str__(self):
        mark = " reached" if self.reached else ""
        return f"{self.name} {self.limit}{mark}"


@dataclass(frozen=True)
class Standing:
    """How far an answer can be trusted: who answered, how strongly, within what.

    Attributes:
        engine: Name of the engine that answered; empty when the service
            predates ``engines`` or no engine covered the question
        strength: One of the ``STRENGTH_*`` spellings; empty when the service
            predates ``engines``
        bounds: The bounds the engine ran under, each marked when it stopped
            at it
    """

    engine: str = ""
    strength: str = ""
    bounds: tuple = field(default_factory=tuple)

    @classmethod
    def of(cls, pb):
        """The standing a response or verdict message carries."""
        return cls(
            engine=pb.engine,
            strength=pb.strength,
            bounds=tuple(Bound(b.name, b.limit, b.reached) for b in pb.bounds),
        )

    @property
    def reported(self):
        """Whether the service reported a standing at all."""
        return bool(self.strength)

    @property
    def reached(self):
        """The bounds the engine stopped at."""
        return [b for b in self.bounds if b.reached]

    def explain(self):
        """One phrase: ``observed by run``, ``bounded by explore (runs 64 reached)``."""
        if not self.reported:
            return ""
        line = self.strength
        if self.engine:
            line += f" by {self.engine}"
        reached = self.reached
        if reached:
            line += " (" + ", ".join(str(b) for b in reached) + ")"
        return line

    def __str__(self):
        return self.explain()


@dataclass(frozen=True)
class EngineInfo:
    """One engine the service registers, as :meth:`~opensysml.Connection.list_engines` reports it.

    Attributes:
        name: The engine's name, the spelling ``engine=`` selects it by
        authority: The strongest evidence it may claim, one of ``STRENGTH_*``
        answers: The question kinds it answers
        bounds: The bounds it runs under
        process: The external process it needs, empty for an in-process engine
        process_found: Where that process was found, empty when it was not
        ready: Whether it can run here
        unavailable: Why it cannot, when it cannot
        kind: ``built-in``, ``tool`` or ``engine`` (one registered from a manifest)
        protocol: How it is spoken to: ``-`` for a built-in engine; for a tool,
            ``object`` for the one-JSON-object exchange or ``argv+<stdin>`` when the
            entry has an ``invocation`` block, with a ``/<reply format>`` suffix
            such as ``argv+none/csv`` when the ``reply`` block reads another format
            (the format alone when the entry has no ``invocation``);
            ``<transport>/<protocol>`` such as ``stdio/1`` for an engine entry
        source: The manifest file an external engine was read from, empty for a built-in one
        command: The resolved command of an external engine, empty for a built-in one
        version: The version its manifest declares, empty for a built-in one
        served: Whether this service runs it; an external engine is listed unserved until
            the service is started with ``-serve-external-engines``
    """

    name: str
    authority: str
    answers: tuple
    bounds: tuple = field(default_factory=tuple)
    process: str = ""
    process_found: str = ""
    ready: bool = True
    unavailable: str = ""
    kind: str = ""
    protocol: str = ""
    source: str = ""
    command: str = ""
    version: str = ""
    served: bool = False

    @classmethod
    def of(cls, pb):
        """An engine as the wire describes it."""
        return cls(
            name=pb.name,
            authority=pb.authority,
            answers=tuple(pb.answers),
            bounds=tuple(pb.bounds),
            process=pb.process,
            process_found=pb.process_found,
            ready=pb.ready,
            unavailable=pb.unavailable,
            kind=pb.kind,
            protocol=pb.protocol,
            source=pb.source,
            command=pb.command,
            version=pb.version,
            served=pb.served,
        )

    def explain(self):
        """One line naming the engine, its kind, its authority, its questions and its status."""
        status = "ready" if self.ready else f"unavailable: {self.unavailable}"
        if self.kind == "engine" and not self.served:
            status += "; not served by this service"
        kind = f" ({self.kind}, {self.protocol})" if self.kind in ("engine", "tool") else ""
        return f"{self.name}{kind}: {self.authority}, answers {', '.join(self.answers)}; {status}"

    def __str__(self):
        return self.explain()

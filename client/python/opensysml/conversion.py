"""Writing a model back out, in SysML notation or RDF Turtle, and migrating a
SysML v1 model in.

The service converts and migrates; this module is the client's side of both.
The formats are the ones OpenSysML can write, named as the ``sysml`` CLI's
``-from``/``-convert``/``-migrate`` name them, so a script and a command line
agree.

A SysML v1 model is migrated, not converted: every element lands in the
:class:`MigrationReport` as mapped, approximated, unmapped or skipped, so
:meth:`~opensysml.connection.Connection.convert` refuses v1 input with the help
that names :meth:`~opensysml.connection.Connection.migrate`.

A round trip is defined on the model, not on the bytes: notation written back
out is re-indented from the original source, and a trip through Turtle returns
an equivalent model rather than identical text. ``docs/reference/rdf-mapping.md`` states
what survives.
"""

import os
from dataclasses import dataclass, field
from typing import Dict, List

#: SysML v2 / KerML textual notation. ``kerml`` and ``text`` name it too.
FORMAT_SYSML = "sysml"
#: RDF in Turtle syntax. ``turtle`` and ``rdf`` name it too.
FORMAT_TURTLE = "ttl"
#: The OMG API's JSON element form, the same RDF mapping spelled differently.
#: ``json`` names it too.
FORMAT_API_JSON = "api-json"

#: Names the service canonicalizes each format to, so a reported format can be
#: told apart without repeating the alias table.
_TURTLE_NAMES = frozenset({"ttl", "turtle", "rdf"})
_API_JSON_NAMES = frozenset({"api-json", "json"})

#: Names of the SysML v1 input the service migrates, an experimental mapping too.
_XMI_NAMES = frozenset({"xmi", "uml", "mdzip"})

#: Extensions that name a SysML v1 model: UML XMI, an Eclipse UML2 file and a
#: Cameo/MagicDraw project archive.
_V1_EXTENSIONS = frozenset({".xmi", ".uml", ".mdzip"})

#: Why a v1 model is refused by ``convert``, in the words every surface uses.
MIGRATED_NOT_CONVERTED = (
    "is a SysML v1 model, which is migrated, not converted: every element is "
    "mapped, approximated or left unmapped and reported element by element"
)


def is_v1(from_format):
    """Report whether ``from_format`` names a SysML v1 model: xmi, uml or mdzip."""
    return from_format in _XMI_NAMES


def path_is_v1(path):
    """Report whether ``path``'s extension names a SysML v1 model."""
    return os.path.splitext(path)[1].lower() in _V1_EXTENSIONS

#: The fallback wording, for a service too old to send its own notice: the RDF
#: mapping's status is a property of the mapping, not of the service.
EXPERIMENTAL_NOTICE = (
    "RDF conversion \u2014 Turtle and the API's JSON element form alike \u2014 is "
    "experimental: the mapping covers model structure and the behavior its "
    "bodies state, refuses what it cannot write back, and its vocabulary may "
    "change without a compatibility path; see "
    "docs/reference/rdf-mapping.md \u00a7 Status"
)


class ExperimentalFeatureWarning(UserWarning):
    """Warns that a conversion went through an experimental mapping.

    Raised as a warning rather than an error: the conversion did happen. Silence
    it with :func:`warnings.simplefilter` on this class, which no stable feature
    warns with, so silencing it cannot hide anything else.
    """


def is_experimental(from_format, to_format):
    """Report whether a conversion between these formats uses an experimental mapping.

    Args:
        from_format (str): Format read, as the service reports it.
        to_format (str): Format written, as the service reports it.

    Returns:
        bool: True when either side is RDF or the API's JSON element form, or
        the input is SysML v1 XMI, which is migrated. Notation to notation is
        stable.
    """
    return (
        from_format in _TURTLE_NAMES
        or to_format in _TURTLE_NAMES
        or from_format in _API_JSON_NAMES
        or to_format in _API_JSON_NAMES
        or from_format in _XMI_NAMES
    )


#: Extensions the exporter's FormatOfPath knows, so a path names the same format
#: here as it does to `sysml -convert` and `%save`.
_EXTENSIONS = {
    ".sysml": FORMAT_SYSML,
    ".kerml": FORMAT_SYSML,
    ".ttl": FORMAT_TURTLE,
    ".turtle": FORMAT_TURTLE,
    ".json": FORMAT_API_JSON,
}


def format_of_path(path):
    """Infer the format to write ``path`` as, from its extension.

    Args:
        path (str): File name or path.

    Returns:
        str: Format name, as :func:`~opensysml.connection.Connection.convert` takes it.

    Raises:
        ValueError: If the extension names no format this client can write.
    """
    ext = os.path.splitext(path)[1].lower()
    try:
        return _EXTENSIONS[ext]
    except KeyError:
        known = ", ".join(sorted(_EXTENSIONS))
        raise ValueError(
            f"cannot tell the format to write {path!r} as: expected one of {known}, "
            f"or pass to_format explicitly"
        ) from None


@dataclass(frozen=True)
class Conversion:
    """A model written out in one of the formats OpenSysML writes.

    Attributes:
        content: The converted model. ``str(conversion)`` is this text, so a
            conversion can be written or compared directly.
        from_format: Format the source was read as. Reported even when it was
            inferred, so a caller learns what the inference decided.
        to_format: Format ``content`` is written in.
        diagnostics: Syntax errors the service tolerated under
            ``tolerate_syntax_errors``. Empty otherwise: a conversion that
            failed raises instead.
        experimental: True when the conversion went through the RDF mapping,
            which is experimental. A notation conversion is stable.
        experimental_notice: What is experimental about it, in the service's own
            wording. Empty when ``experimental`` is False.
    """

    content: str
    from_format: str
    to_format: str
    diagnostics: List[object] = field(default_factory=list)
    experimental: bool = False
    experimental_notice: str = ""

    def __str__(self):
        return self.content

    def __len__(self):
        return len(self.content)

    def write(self, path):
        """Write the converted model to ``path``.

        The bytes written are the ones the service returned: ``newline=""`` turns
        off the translation text mode would otherwise apply to every line ending.

        Args:
            path (str): File to write, created or truncated.

        Returns:
            str: The path written, for chaining.
        """
        with open(path, "w", encoding="utf-8", newline="") as handle:
            handle.write(self.content)
        return path


@dataclass(frozen=True)
class MigrationEntry:
    """One SysML v1 element's verdict in a migration.

    Attributes:
        id: The element's ``xmi:id``.
        kind: Its v1 metaclass, with its applied stereotypes.
        name: Its qualified name in the v1 model.
        target: The v2 element it was written as, when it was written.
        verdict: ``mapped``, ``approximated``, ``unmapped`` or ``skipped``.
        note: Why the verdict is what it is, in the migrator's words.
    """

    id: str
    kind: str
    name: str
    target: str
    verdict: str
    note: str


@dataclass(frozen=True)
class MigrationReport:
    """The account a migration gives of itself: what became of every element.

    The summary and the four counts always come back. ``entries`` and ``text``
    come back when :meth:`~opensysml.connection.Connection.migrate` is asked
    for the report; ``text`` is what ``sysml -migrate -migration-report`` writes.

    Attributes:
        source: The v1 model migrated, as the service named it.
        exporter: The tool that exported it, as its XMI says.
        summary: The one-line account, ``migrated N element(s): … mapped, …
            approximated, … unmapped (… skipped …)``.
        mapped: Elements with a faithful v2 form.
        approximated: Elements written in a v2 form that is not quite theirs.
        unmapped: Elements with no v2 form, left out and reported.
        skipped: Elements the migration does not consider: profile, library
            and notation-only content, and elements nothing refers to.
        entries: Every element's verdict, when the report was asked for.
        text: The report as ``-migration-report`` writes it, when asked for.
    """

    source: str
    exporter: str
    summary: str
    mapped: int
    approximated: int
    unmapped: int
    skipped: int
    entries: List[MigrationEntry] = field(default_factory=list)
    text: str = ""

    def __str__(self):
        return self.text or self.summary

    def by_verdict(self, verdict):
        """The entries with ``verdict``: mapped, approximated, unmapped or skipped."""
        return [entry for entry in self.entries if entry.verdict == verdict]


@dataclass(frozen=True)
class Migration:
    """A SysML v1 model migrated to one of the formats OpenSysML writes.

    Attributes:
        content: The migrated model. ``str(migration)`` is this text.
        from_format: The v1 form read, canonically ``xmi``: ``uml`` and
            ``mdzip`` name the same reader.
        to_format: Format ``content`` is written in.
        report: What became of every element. Never None: the summary and the
            counts come back with every migration.
        results: The JSON index of the result snapshots the v1 tool stored, as
            ``-migration-results`` writes it, when asked for; else ``""``.
        files: Image files the model's diagrams embed, by the relative path
            ``content`` refers to them with, when there are any.
        experimental: Always True: the migration is experimental.
        experimental_notice: What is experimental about it, in the service's own
            wording.
    """

    content: str
    from_format: str
    to_format: str
    report: MigrationReport
    results: str = ""
    files: Dict[str, bytes] = field(default_factory=dict)
    experimental: bool = True
    experimental_notice: str = ""

    def __str__(self):
        return self.content

    def __len__(self):
        return len(self.content)

    def write(self, path):
        """Write the migrated model to ``path``, and its image files beside it.

        The files are written at their relative paths under ``path``'s
        directory, where the migrated model refers to them, as ``sysml -migrate
        -o`` writes them.

        Args:
            path (str): File to write, created or truncated.

        Returns:
            str: The path written, for chaining.
        """
        with open(path, "w", encoding="utf-8", newline="") as handle:
            handle.write(self.content)
        base = os.path.dirname(path)
        for relative, data in self.files.items():
            target = os.path.join(base, *relative.split("/"))
            os.makedirs(os.path.dirname(target) or ".", exist_ok=True)
            with open(target, "wb") as handle:
                handle.write(data)
        return path


def migration_report_of(message):
    """Build a :class:`MigrationReport` from a ``MigrationReport`` protobuf message."""
    return MigrationReport(
        source=message.source,
        exporter=message.exporter,
        summary=message.summary,
        mapped=message.mapped,
        approximated=message.approximated,
        unmapped=message.unmapped,
        skipped=message.skipped,
        entries=[
            MigrationEntry(
                id=entry.id,
                kind=entry.kind,
                name=entry.name,
                target=entry.target,
                verdict=entry.verdict,
                note=entry.note,
            )
            for entry in message.entries
        ],
        text=message.text,
    )

"""Writing a model back out, in SysML notation or RDF Turtle.

The service converts; this module is the client's side of it. The formats are
the two OpenSysML can write, named as the ``sysml`` CLI's ``-from``/``-to``
name them, so a script and a command line agree.

A round trip is defined on the model, not on the bytes: notation written back
out is re-indented from the original source, and a trip through Turtle returns
an equivalent model rather than identical text. ``docs/reference/rdf-mapping.md`` states
what survives.
"""

import os
from dataclasses import dataclass, field
from typing import List

#: SysML v2 / KerML textual notation. ``kerml`` and ``text`` name it too.
FORMAT_SYSML = "sysml"
#: RDF in Turtle syntax. ``turtle`` and ``rdf`` name it too.
FORMAT_TURTLE = "ttl"

#: Names the service canonicalizes each format to, so a reported format can be
#: told apart without repeating the alias table.
_TURTLE_NAMES = frozenset({"ttl", "turtle", "rdf"})

#: Names of the SysML v1 input the service migrates, an experimental mapping too.
_XMI_NAMES = frozenset({"xmi", "uml"})

#: The fallback wording, for a service too old to send its own notice: the RDF
#: mapping's status is a property of the mapping, not of the service.
EXPERIMENTAL_NOTICE = (
    "RDF conversion is experimental: the mapping covers model structure and the "
    "behavior its bodies state, refuses what it cannot write back, and its "
    "vocabulary may change without a compatibility path; see "
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
        bool: True when either side is RDF or the input is SysML v1 XMI, which
        is migrated. Notation to notation is stable.
    """
    return (
        from_format in _TURTLE_NAMES
        or to_format in _TURTLE_NAMES
        or from_format in _XMI_NAMES
    )


#: Extensions the exporter's FormatOfPath knows, so a path names the same format
#: here as it does to `sysml -convert` and `%save`.
_EXTENSIONS = {
    ".sysml": FORMAT_SYSML,
    ".kerml": FORMAT_SYSML,
    ".ttl": FORMAT_TURTLE,
    ".turtle": FORMAT_TURTLE,
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

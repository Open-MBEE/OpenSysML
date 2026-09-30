"""The documents of a model parsed from several sources at once.

:meth:`Connection.parse_sources <opensysml.connection.Connection.parse_sources>`
takes a sequence of documents, each a file on the service's disk or inline
content under a name, and parses them together as one model. A document is
written as one of:

* a path (``str`` or ``os.PathLike``) — a file the service reads, its extension
  saying whether it is SysML or KerML;
* a ``(name, content)`` pair — inline source, reported in diagnostics and
  indexed under ``name``;
* a :class:`SourceDocument`, the explicit form, which is also how inline
  content declares a language other than SysML.
"""

import os
from dataclasses import dataclass
from typing import Iterable, List, Optional, Tuple, Union

from opensysml.proto import sysml_pb2

#: Languages inline content may be declared in.
LANGUAGES = ("sysml", "kerml")


@dataclass(frozen=True)
class SourceDocument:
    """One document of a model parsed from several.

    Exactly one of ``path`` and ``content`` is set. Build one with
    :meth:`file` or :meth:`inline` rather than the constructor.

    Attributes:
        path (str, optional): File the service reads. Its extension says which
            notation it is; it is also the name diagnostics report.
        content (str, optional): Inline model source, parsed under ``name``.
        name (str): What diagnostics call inline content and what the model
            indexes it under; two documents of one model may not share a name.
            A file is named by its path, so this is empty for one.
        language (str, optional): Notation of inline content, ``"sysml"`` (the
            default when None) or ``"kerml"``. Needs the ``inline_language``
            capability when set.
    """

    path: Optional[str] = None
    content: Optional[str] = None
    name: str = ""
    language: Optional[str] = None

    def __post_init__(self) -> None:
        if not isinstance(self.name, str):
            raise TypeError(f"name must be str, not {type(self.name).__name__}")
        for field, value in (
            ("path", self.path), ("content", self.content), ("language", self.language)
        ):
            if value is not None and not isinstance(value, str):
                raise TypeError(f"{field} must be str, not {type(value).__name__}")
        if (self.path is None) == (self.content is None):
            raise ValueError(
                "a SourceDocument is either a file path or inline content, not both"
            )
        if self.path is not None:
            if not self.path:
                raise ValueError("a file document needs a path")
            if self.name:
                raise ValueError(
                    "a file document is named by its path; name applies to inline content"
                )
            if self.language is not None:
                raise ValueError(
                    "a file's extension says which language it is; "
                    "language applies to inline content"
                )
        else:
            if not self.name:
                raise ValueError(
                    "inline content needs a name; diagnostics report it under that name"
                )
            if self.language is not None and self.language not in LANGUAGES:
                raise ValueError("language must be 'sysml' or 'kerml'")

    @classmethod
    def file(cls, path: "Union[str, os.PathLike[str]]") -> "SourceDocument":
        """A file the service reads, named by its path.

        Args:
            path (str or os.PathLike): Path to the .sysml or .kerml file, on the
                machine the service runs on
        """
        return cls(path=os.fspath(path))

    @classmethod
    def inline(
        cls, name: str, content: str, language: Optional[str] = None
    ) -> "SourceDocument":
        """Inline content, reported and indexed under ``name``.

        Args:
            name (str): Name diagnostics report the content under, such as
                ``"lib.sysml"``
            content (str): The model source
            language (str, optional): ``"sysml"`` or ``"kerml"``; SysML when
                omitted
        """
        return cls(content=content, name=name, language=language)

    @property
    def document_name(self) -> str:
        """The name diagnostics report this document under: the path or name."""
        return self.path if self.path is not None else self.name

    def to_pb(self) -> sysml_pb2.SourceDocument:
        """The ``SourceDocument`` message that sends this document."""
        if self.path is not None:
            return sysml_pb2.SourceDocument(file_path=self.path)
        return sysml_pb2.SourceDocument(
            content=self.content, name=self.name, language=self.language or ""
        )


#: What :meth:`Connection.parse_sources` accepts for one document.
Source = Union[str, "os.PathLike[str]", Tuple[str, str], SourceDocument]


def source_documents(documents: Iterable[Source]) -> List[SourceDocument]:
    """The :class:`SourceDocument` for each of ``documents``.

    Args:
        documents: Paths, ``(name, content)`` pairs and SourceDocuments, in the
            order the model reads them

    Returns:
        list[SourceDocument]: One per document, in order

    Raises:
        ValueError: If there are no documents, two share a name, one is of none
            of the accepted forms, or ``documents`` is itself one document
            rather than a sequence of them
    """
    if isinstance(documents, (str, bytes, os.PathLike, SourceDocument)) or (
        _is_named_content(documents)
    ):
        raise ValueError(
            "parse_sources takes a sequence of documents; write one document as "
            "[document]"
        )
    result: List[SourceDocument] = []
    for position, document in enumerate(documents):
        if isinstance(document, SourceDocument):
            result.append(document)
        elif isinstance(document, (str, os.PathLike)):
            result.append(SourceDocument.file(document))
        elif _is_named_content(document):
            name, content = document
            result.append(SourceDocument.inline(name, content))
        else:
            raise ValueError(
                f"document {position} is {type(document).__name__}; expected a "
                "path, a (name, content) pair or a SourceDocument"
            )
    if not result:
        raise ValueError("parse_sources needs at least one document")
    seen = set()
    for document in result:
        name = document.document_name
        if name in seen:
            raise ValueError(f"two documents are named {name!r}")
        seen.add(name)
    return result


def _is_named_content(document: object) -> bool:
    return (
        isinstance(document, tuple)
        and len(document) == 2
        and all(isinstance(part, str) for part in document)
    )


__all__ = ["LANGUAGES", "Source", "SourceDocument", "source_documents"]

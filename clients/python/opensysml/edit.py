"""Changing a loaded model and writing it back, with its layout intact.

An edit is described, not typed out: an :class:`Editor` collects operations
naming elements the way a read result names them, and :meth:`Editor.apply` has
the service perform them on the source it parsed. The service edits the bytes the
operations reach and nothing else, so comments, blank lines and indentation
outside an edited span come back unchanged, and it re-parses what it edited
before returning it.

Operations include setting a feature's value, renaming a declaration, adding a
member, deleting a declaration, and moving one into another namespace.
Renaming rewrites the declaration's name token only and is refused for an
element that is referenced — see :class:`~opensysml.errors.RenameReferencedError`.
"""

from dataclasses import dataclass, field
from typing import List

from opensysml.conversion import Conversion, FORMAT_SYSML
from opensysml.proto import sysml_pb2
from opensysml.errors import (
    EditError,
    EditResultError,
    EditTargetError,
    InvalidEditError,
    NoEditsError,
    OverlappingEditsError,
    RenameReferencedError,
    OwnerNotFoundError,
    OwnerNotNamespaceError,
    IllegalMemberKindError,
    MemberNameTakenError,
    DeleteReferencedError,
    OwnerInsideTargetError,
    MoveReferencedError,
    ReferencedElsewhereError,
    Referrer,
)

#: Refusal kinds, as the wire enum names them, and the error each raises. A kind
#: this client has not seen raises the base :class:`EditError`, so no refusal
#: escapes the hierarchy.
_FAILURE_ERRORS = {
    "EDIT_FAILURE_NO_OPERATIONS": NoEditsError,
    "EDIT_FAILURE_UNKNOWN_TARGET": EditTargetError,
    "EDIT_FAILURE_AMBIGUOUS_TARGET": EditTargetError,
    "EDIT_FAILURE_NOT_VALUED": EditTargetError,
    "EDIT_FAILURE_NOT_NAMED": EditTargetError,
    "EDIT_FAILURE_INVALID_VALUE": InvalidEditError,
    "EDIT_FAILURE_INVALID_NAME": InvalidEditError,
    "EDIT_FAILURE_RENAME_REFERENCED": RenameReferencedError,
    "EDIT_FAILURE_OVERLAPPING_EDITS": OverlappingEditsError,
    "EDIT_FAILURE_RESULT_INVALID": EditResultError,
    "EDIT_FAILURE_OWNER_UNKNOWN": OwnerNotFoundError,
    "EDIT_FAILURE_OWNER_NOT_NAMESPACE": OwnerNotNamespaceError,
    "EDIT_FAILURE_ILLEGAL_KIND": IllegalMemberKindError,
    "EDIT_FAILURE_MEMBER_NAME_TAKEN": MemberNameTakenError,
    "EDIT_FAILURE_DELETE_REFERENCED": DeleteReferencedError,
    "EDIT_FAILURE_OWNER_INSIDE_TARGET": OwnerInsideTargetError,
    "EDIT_FAILURE_MOVE_REFERENCED": MoveReferencedError,
    "EDIT_FAILURE_REFERENCED_ELSEWHERE": ReferencedElsewhereError,
}


def failure_name(failure):
    """Name a refusal kind, including one this client's enum has no name for.

    proto3 enums are open, so a newer service can refuse an edit for a reason
    this build has never heard of; that must still raise an :class:`EditError`
    rather than fail on the enum lookup.

    Args:
        failure (int): EditFailure number as the service sent it

    Returns:
        str: The enum's name, or a name naming the unknown number
    """
    try:
        return sysml_pb2.EditFailure.Name(failure)
    except ValueError:
        return f"EDIT_FAILURE_{failure}"


def error_for_failure(failure, message, diagnostics=None, referring_elements=None,
                      referrers=None):
    """Build the error a refusal kind names.

    Args:
        failure (str): Refusal kind, as the wire enum names it
        message (str): Why the edit was refused
        diagnostics (list, optional): Diagnostics behind the refusal
        referring_elements (list, optional): Referrers of a refused rename
        referrers (list[Referrer], optional): The same referrers, each with
            the document declaring it

    Returns:
        EditError: The typed refusal, ready to raise
    """
    cls = _FAILURE_ERRORS.get(failure, EditError)
    return cls(
        message,
        failure=failure,
        diagnostics=diagnostics,
        referring_elements=referring_elements,
        referrers=referrers,
    )


@dataclass(frozen=True)
class AppliedEdit:
    """One byte range an operation replaced in the source it saw.

    Attributes:
        operation_index: Position of the operation in the editor, so an applied
            edit is matched back to what asked for it.
        target: Element edited, by the id it was named with.
        offset: Byte offset where the replacement starts.
        length: Number of bytes replaced. Zero for a value added to a feature
            that had none: nothing was replaced, text was inserted.
        old_text: The bytes that were there.
        new_text: What replaced them.
        document: The document the bytes belong to, named as the parse named
            it; the model's one document for a model loaded from a file.
    """

    operation_index: int
    target: str
    offset: int
    length: int
    old_text: str
    new_text: str
    document: str = ""

    def __str__(self):
        return f"{self.target}: {self.old_text!r} -> {self.new_text!r}"


@dataclass(frozen=True)
class EditedDocument:
    """The edited notation of one document of the model.

    Attributes:
        name: The document's name as the parse named it: the file path of a
            loaded file, or the name inline content was loaded under.
        content: The edited notation, byte-identical to the source outside the
            edited spans.
    """

    name: str
    content: str

    def __str__(self):
        return self.content


@dataclass(frozen=True)
class EditResult(Conversion):
    """The edited notation, as a :class:`~opensysml.conversion.Conversion`.

    ``str(result)`` is the edited text and ``result.save(path)`` writes it, so an
    edit is written the way a conversion is. ``content`` is the notation of a
    model of one document, which is every model this client loads; a model of
    several documents, edited through the service directly by a request that
    accepts documents, answers with its rewritten documents in ``documents``
    and an empty ``content``. A request not accepting them is refused on such
    a model, as every request was before ``documents`` existed.

    Attributes:
        applied: What each operation changed, grouped by document in the order
            ``documents`` lists them and in source order within a document.
        documents: The edited notation of every document the edits rewrote,
            the edited document first: one entry for a model of one document.
            Empty from a service without the ``edit_documents`` capability,
            which answers ``content`` alone.
    """

    applied: List[AppliedEdit] = field(default_factory=list)
    documents: List[EditedDocument] = field(default_factory=list)

    def save(self, path):
        """Write the edited model to ``path``.

        Args:
            path (str): File to write, created or truncated

        Returns:
            str: The path written, for chaining
        """
        return self.write(path)


class Editor:
    """Operations to perform on a loaded model, and the call that performs them.

    Collected client-side and applied in one call, so the service edits and
    validates the model once. Every operation names its element by the id a read
    reports (:attr:`Symbol.id`), or by the :class:`~opensysml.symbol.Symbol` itself.

    An editor is applied once: it describes an edit of the model it was made
    from, and the edited model is a different model. Build another editor from
    the reloaded model to edit again.

    Example:
        >>> edit = model.edit()
        >>> edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
        >>> edit.apply().save("spacecraft.sysml")
        'spacecraft.sysml'
    """

    def __init__(self, model_hash, connection):
        """Initialize an editor over a loaded model.

        Args:
            model_hash (str): Hash of the model to edit
            connection: Connection the model was loaded over
        """
        self._model_hash = model_hash
        self._connection = connection
        self._operations = []
        self._applied = False

    @property
    def operations(self):
        """The operations collected so far, in the order they were added."""
        return list(self._operations)

    @property
    def applied(self):
        """Whether this editor has been applied."""
        return self._applied

    def __len__(self):
        return len(self._operations)

    def __bool__(self):
        # An editor with no operations is falsy, so `if edit:` asks what it reads
        # as: whether there is anything to apply.
        return bool(self._operations)

    def set_value(self, target, value):
        """Set the value expression of one of the model's features.

        Replaces an existing ``= <expr>``, or adds one before the terminating
        semicolon when the feature has none.

        Args:
            target (str or Symbol): Feature to edit, by FQN/id or symbol
            value (str): The new value, as SysML notation for one expression,
                e.g. ``"1050.0[SI::kg]"``, ``'"flight-2"'`` or ``"unitMass * 2"``

        Returns:
            Editor: self, so operations can be chained

        Raises:
            TypeError: If value is not a string: the notation is what is sent,
                and guessing notation for a Python object would guess its type
        """
        if not isinstance(value, str):
            raise TypeError(
                f"value must be SysML notation for an expression, not "
                f"{type(value).__name__}: write it as it should read in the file"
            )
        self._add(("set_value", _target_id(target), value))
        return self

    def rename(self, target, new_name):
        """Rename one of the model's declarations.

        Rewrites the declaration's name token and every reference to it in the
        model's source, including qualified names, alias targets and imports. A
        rename that would make another name mean the renamed element, or make
        this one mean something else, is refused with
        :class:`~opensysml.errors.InvalidEditError`.

        Args:
            target (str or Symbol): Declaration to rename, by FQN/id or symbol
            new_name (str): The new name, as it should read in the file

        Returns:
            Editor: self, so operations can be chained

        Raises:
            TypeError: If new_name is not a string
        """
        if not isinstance(new_name, str):
            raise TypeError(
                f"new_name must be a name, not {type(new_name).__name__}"
            )
        self._add(("rename", _target_id(target), new_name))
        return self

    def add_member(self, owner, kind, name, type=None, multiplicity=None,
                   value=None, specializes=None):
        """Add one declaration, using strings for all SysML/KerML notation."""
        for label, text in (("kind", kind), ("name", name), ("type", type),
                            ("multiplicity", multiplicity), ("value", value)):
            if text is not None and not isinstance(text, str):
                raise TypeError(
                    f"{label} must be notation text, not "
                    f"{text.__class__.__name__}"
                )
        owner = owner if isinstance(owner, str) else _target_id(owner)
        if specializes is None:
            specializes = []
        if isinstance(specializes, str) or not all(isinstance(x, str) for x in specializes):
            raise TypeError("specializes must be a sequence of notation strings")
        self._add(("add_member", owner, kind, name, type or "", multiplicity or "",
                   value or "", list(specializes)))
        return self

    def delete(self, target, cascade=False):
        """Delete a declaration, optionally removing declarations that refer to it."""
        if not isinstance(cascade, bool):
            raise TypeError("cascade must be bool")
        self._add(("delete", _target_id(target), cascade))
        return self

    def move(self, target, owner):
        """Move a declaration into another namespace of the same document.

        The declaration is carried with its body and owned comments to where
        :meth:`add_member` would insert it, and references to it are respelled
        so they still reach it. A move that would leave a reference no spelling
        restores is refused with :class:`~opensysml.errors.MoveReferencedError`.

        Args:
            target (str or Symbol): Declaration to move, by FQN/id or symbol
            owner (str or Symbol): Namespace to receive it; ``""`` is the
                document root

        Returns:
            Editor: self, so operations can be chained
        """
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(("move", _target_id(target), owner))
        return self

    def apply(self):
        """Have the service perform these operations and return the edited model.

        The service edits the source it parsed, re-parses and re-analyses the
        result, and refuses to return content the parser could not read back.

        Returns:
            EditResult: The edited notation, and what each operation changed

        Raises:
            NoEditsError: If no operation was added
            EditError: If the service refused the edit; the subclass names why
            MissingCapabilityError: If the service cannot apply edits
            ModelNotFoundError: If the service no longer holds this model
            RuntimeError: If this editor was already applied
        """
        if self._applied:
            raise RuntimeError(
                "this editor has already been applied: it describes an edit of "
                "the model it was made from, so build another editor from the "
                "edited model rather than applying this one twice"
            )
        if not self._operations:
            raise NoEditsError(
                "this editor has no operations: add an edit before "
                "applying it",
                failure="EDIT_FAILURE_NO_OPERATIONS",
            )
        result = self._connection.apply_edits(self._model_hash, self._operations)
        self._applied = True
        return result

    def _add(self, operation):
        if self._applied:
            raise RuntimeError(
                "this editor has already been applied: build another editor from "
                "the edited model to edit further"
            )
        self._operations.append(operation)

    def __repr__(self):
        return (
            f"Editor(model_hash={self._model_hash!r}, "
            f"operations={len(self._operations)}, applied={self._applied})"
        )

    def add_package(self, owner, name, **kwargs):
        """Add a ``package`` declaration."""
        return self.add_member(owner, "package", name, **kwargs)

    def add_part_def(self, owner, name, **kwargs):
        """Add a ``part def`` declaration."""
        return self.add_member(owner, "part def", name, **kwargs)

    def add_part(self, owner, name, **kwargs):
        """Add a ``part`` declaration."""
        return self.add_member(owner, "part", name, **kwargs)

    def add_attribute_def(self, owner, name, **kwargs):
        """Add an ``attribute def`` declaration."""
        return self.add_member(owner, "attribute def", name, **kwargs)

    def add_attribute(self, owner, name, **kwargs):
        """Add an ``attribute`` declaration."""
        return self.add_member(owner, "attribute", name, **kwargs)

    def add_item_def(self, owner, name, **kwargs):
        """Add an ``item def`` declaration."""
        return self.add_member(owner, "item def", name, **kwargs)

    def add_item(self, owner, name, **kwargs):
        """Add an ``item`` declaration."""
        return self.add_member(owner, "item", name, **kwargs)

    def add_port_def(self, owner, name, **kwargs):
        """Add a ``port def`` declaration."""
        return self.add_member(owner, "port def", name, **kwargs)

    def add_port(self, owner, name, **kwargs):
        """Add a ``port`` declaration."""
        return self.add_member(owner, "port", name, **kwargs)

    def add_class(self, owner, name, **kwargs):
        """Add a ``class`` declaration."""
        return self.add_member(owner, "class", name, **kwargs)

    def add_struct(self, owner, name, **kwargs):
        """Add a ``struct`` declaration."""
        return self.add_member(owner, "struct", name, **kwargs)

    def add_datatype(self, owner, name, **kwargs):
        """Add a ``datatype`` declaration."""
        return self.add_member(owner, "datatype", name, **kwargs)

    def add_classifier(self, owner, name, **kwargs):
        """Add a ``classifier`` declaration."""
        return self.add_member(owner, "classifier", name, **kwargs)

    def add_feature(self, owner, name, **kwargs):
        """Add a ``feature`` declaration."""
        return self.add_member(owner, "feature", name, **kwargs)

    def add_assoc(self, owner, name, **kwargs):
        """Add an ``assoc`` declaration."""
        return self.add_member(owner, "assoc", name, **kwargs)

    def add_behavior(self, owner, name, **kwargs):
        """Add a ``behavior`` declaration."""
        return self.add_member(owner, "behavior", name, **kwargs)

    def add_function(self, owner, name, **kwargs):
        """Add a ``function`` declaration."""
        return self.add_member(owner, "function", name, **kwargs)

    def add_predicate(self, owner, name, **kwargs):
        """Add a ``predicate`` declaration."""
        return self.add_member(owner, "predicate", name, **kwargs)

    def add_interaction(self, owner, name, **kwargs):
        """Add an ``interaction`` declaration."""
        return self.add_member(owner, "interaction", name, **kwargs)

    def add_metaclass(self, owner, name, **kwargs):
        """Add a ``metaclass`` declaration."""
        return self.add_member(owner, "metaclass", name, **kwargs)

    def add_calc_def(self, owner, name, **kwargs):
        """Add a ``calc def`` declaration."""
        return self.add_member(owner, "calc def", name, **kwargs)

    def add_calc(self, owner, name, **kwargs):
        """Add a ``calc`` declaration."""
        return self.add_member(owner, "calc", name, **kwargs)


def _target_id(target):
    """The id an operation names its element by, from an id or a Symbol."""
    if isinstance(target, str):
        return target
    ident = getattr(target, "id", None)
    if isinstance(ident, str) and ident:
        return ident
    raise TypeError(
        f"target must be a symbol id (FQN) or a Symbol, not "
        f"{type(target).__name__}"
    )


def result_of(response, applied_source=FORMAT_SYSML):
    """Read an ``ApplyEditsResponse`` as an :class:`EditResult`.

    Args:
        response: sysml_pb2.ApplyEditsResponse protobuf message
        applied_source (str): Format the content is written in

    Returns:
        EditResult: The edited notation and what changed
    """
    return EditResult(
        content=response.content,
        from_format=applied_source,
        to_format=applied_source,
        applied=[
            AppliedEdit(
                operation_index=a.operation_index,
                target=a.target,
                offset=a.offset,
                length=a.length,
                old_text=a.old_text,
                new_text=a.new_text,
                document=a.document,
            )
            for a in response.applied
        ],
        documents=[
            EditedDocument(name=d.name, content=d.content)
            for d in response.documents
        ],
    )


def referrers_of(response):
    """Read an ``ApplyEditsResponse``'s referrers as :class:`Referrer` objects.

    Args:
        response: sysml_pb2.ApplyEditsResponse protobuf message

    Returns:
        list[Referrer]: Each referring declaration with its document
    """
    return [Referrer(name=r.name, document=r.document) for r in response.referrers]

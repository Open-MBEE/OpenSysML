"""Changing a loaded model and writing it back, with its layout intact.

An edit is described, not typed out: an :class:`Editor` collects operations
naming elements the way a read result names them, and :meth:`Editor.apply` has
the service perform them on the source it parsed. The service edits the bytes the
operations reach and nothing else, so comments, blank lines and indentation
outside an edited span come back unchanged, and it re-parses what it edited
before returning it.

Operations include setting a feature's value, renaming a declaration, adding a
member, connection, transition or documentation, deleting a declaration, and
moving one into another namespace.
Renaming rewrites the declaration's name token only and is refused for an
element that is referenced — see :class:`~opensysml.errors.RenameReferencedError`.
"""

from collections.abc import Sequence
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


def _sequence_text(label, value, optional=False):
    if value is None and optional:
        return
    if not isinstance(value, str):
        raise TypeError(f"{label} must be notation text, not {type(value).__name__}")


def _sequence_tuple(owner, keyword, ref="", member_kind="", member_name="",
                    type_name="", after="", **fields):
    operation = (
        "add_sequence", owner, keyword, ref, member_kind, member_name,
        type_name, after,
    )
    return operation + (fields,) if fields else operation


def _sequence_options(after=None, multiplicity=None):
    _sequence_text("multiplicity", multiplicity, optional=True)
    _sequence_text("after", after, optional=True)
    return after or "", multiplicity


def _sequence_keyword_options(keyword, options):
    if options[1] is not None and keyword != "then":
        raise ValueError("multiplicity requires then=True")


def _sequence_statement(owner, keyword, member_kind="", *, ref="", member_name="",
                        type_name="", options=None, after=None, multiplicity=None,
                        **fields):
    if options is None:
        options = _sequence_options(after, multiplicity)
    after, multiplicity = options
    if multiplicity is not None:
        fields["multiplicity"] = multiplicity
    return _sequence_tuple(
        owner, keyword, ref, member_kind, member_name, type_name or "",
        after, **fields,
    )


class Body:
    """Chainable action-body items for nested ``if`` and loop statements.

    The first ordinary statement is written without ``then`` by default, and
    later ordinary statements use it. Empty then/loop bodies emit ``{ }``.
    An empty else branch performs nothing, so an empty ``else_body`` is
    equivalent to omitting ``else``.

    The statement forms follow SysML.xtext:1368 ActionNodeMember, 1607 ActionBodyParameter,
    1442 AcceptNode, 1499 SendNode, 1535 AssignmentNode, 1596 IfNode,
    1615 WhileLoopNode, 1624 ForLoopNode, 1641 TerminateNode and formal/2026-03-02.

    Example:
        >>> editor.add_while(
        ...     "Demo::A", "count < 2", Body().add_assign("count", "count + 1")
        ... )
    """

    def __init__(self):
        self._operations = []

    @property
    def operations(self):
        """The body items collected so far."""
        return list(self._operations)

    def _keyword(self, then):
        if then is None:
            return "" if not self._operations else "then"
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool or None, not {type(then).__name__}")
        return "then" if then else ""

    def _add_statement(self, kind, *, then=None, type_name="", **fields):
        options = _sequence_options(
            fields.pop("after", None), fields.pop("multiplicity", None)
        )
        keyword = self._keyword(then)
        _sequence_keyword_options(keyword, options)
        self._operations.append(_sequence_statement(
            "", keyword, kind, type_name=type_name, options=options, **fields
        ))
        return self

    def add_first(self, ref):
        """Emit ``first <ref>;`` (SysML.xtext:1384 InitialNodeMember; formal/2026-03-02).

        Args:
            ref: The node the body starts at.

        Raises:
            TypeError: If ``ref`` is not notation text.
        """
        _sequence_text("ref", ref)
        self._operations.append(_sequence_statement("", "first", ref=ref))
        return self

    def add_then(self, ref=None, action=None, type=None, kind="action",
                 multiplicity=None):
        """Emit ``then [m] <ref>;`` or ``then [m] <kind> <action> : <type>;`` (SysML.xtext:878, 887, 1703 TargetSuccession; formal/2026-03-02).

        Args:
            ref: The target reference, mutually exclusive with ``action``.
            action: The member name, mutually exclusive with ``ref``.
            type: Optional type of the declared action member.
            kind: Declared member kind, defaulting to ``"action"``.
            multiplicity: Optional bracketed source-end multiplicity ``[m]``.

        Raises:
            TypeError: If a notation argument is not a string.
            ValueError: If the arguments do not describe exactly one form.
        """
        for label, text in (("ref", ref), ("action", action), ("type", type),
                            ("kind", kind), ("multiplicity", multiplicity)):
            _sequence_text(label, text, optional=True)
        options = ("", multiplicity)
        if (ref is None) == (action is None):
            raise ValueError("exactly one of ref and action is required")
        if ref is not None:
            if type is not None or (kind is not None and kind != "action"):
                raise ValueError("a then reference takes no type or kind")
            self._operations.append(_sequence_statement(
                "", "then", ref=ref, options=options
            ))
        else:
            self._operations.append(_sequence_statement(
                "", "then", kind or "action", member_name=action,
                type_name=type or "", options=options,
            ))
        return self

    def add_action(self, name=None, type=None, kind="action"):
        """Emit ``<kind> <name> : <type>;`` (SysML.xtext:1368 ActionNodeMember; formal/2026-03-02).

        Args:
            name: Optional declared action name.
            type: Optional action type.
            kind: Body item kind, defaulting to ``"action"``.

        Raises:
            TypeError: If a notation argument is not a string.
        """
        for label, text in (("name", name), ("type", type), ("kind", kind)):
            _sequence_text(label, text, optional=True)
        self._operations.append(_sequence_statement(
            "", "", kind or "action", member_name=name or "",
            type_name=type or "",
        ))
        return self

    def add_accept(self, payload, type=None, via=None, *, then=None, multiplicity=None):
        """Emit ``accept <payload> [: <type>] [via <via>];`` or ``then [m] accept ...;`` (SysML.xtext:1442 AcceptNode; formal/2026-03-02).

        Args:
            payload: Payload parameter or trigger notation.
            type: Optional accepted payload type.
            via: Optional port expression.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("payload", payload)
        _sequence_text("type", type, optional=True)
        _sequence_text("via", via, optional=True)
        return self._add_statement(
            "accept", then=then, type_name=type or "",
            parameter=payload, via=via or "",
            multiplicity=multiplicity,
        )

    def add_send(self, payload, to=None, via=None, *, then=None, multiplicity=None):
        """Emit ``send <payload> [via <via>] [to <to>];`` or ``then [m] send ...;`` (SysML.xtext:1499 SendNode; formal/2026-03-02).

        Args:
            payload: Payload expression.
            to: Optional receiver expression.
            via: Optional port expression.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("payload", payload)
        _sequence_text("to", to, optional=True)
        _sequence_text("via", via, optional=True)
        return self._add_statement(
            "send", then=then, value=payload, target=to or "",
            via=via or "",
            multiplicity=multiplicity,
        )

    def add_assign(self, target, value, *, then=None, multiplicity=None):
        """Emit ``assign <target> := <value>;`` or ``then [m] assign ...;`` (SysML.xtext:1535 AssignmentNode; formal/2026-03-02).

        Args:
            target: Feature reference to assign.
            value: Assigned expression.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("target", target)
        _sequence_text("value", value)
        return self._add_statement(
            "assign", then=then, target=target, value=value,
            multiplicity=multiplicity,
        )

    def add_if(self, condition, body, else_body=None, *, then=None, multiplicity=None):
        """Emit ``if <condition> { <body> } [else { <else_body> }]`` or ``then [m] if ...`` (SysML.xtext:1596 IfNode; formal/2026-03-02).

        Args:
            condition: Boolean condition expression.
            body: Then-branch :class:`Body`.
            else_body: Optional else-branch :class:`Body`; an empty body means
                no else branch.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string or body is not a
                :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("condition", condition)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        if else_body is not None and not isinstance(else_body, Body):
            raise TypeError(f"else_body must be Body or None, not {type(else_body).__name__}")
        return self._add_statement(
            "if", then=then, condition=condition,
            body=body.operations,
            else_body=else_body.operations if else_body and else_body.operations else [],
            multiplicity=multiplicity,
        )

    def add_while(self, condition, body, until=None, *, then=None, multiplicity=None):
        """Emit ``while <condition> { <body> } [until <until>];`` or ``then [m] while ...`` (SysML.xtext:1615 WhileLoopNode; formal/2026-03-02).

        Args:
            condition: Loop condition expression.
            body: Loop-body :class:`Body`.
            until: Optional post-condition expression.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string or ``body`` is not
                a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("condition", condition)
        _sequence_text("until", until, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        return self._add_statement(
            "while", then=then, condition=condition,
            until=until or "", body=body.operations,
            multiplicity=multiplicity,
        )

    def add_loop(self, body, until=None, *, then=None, multiplicity=None):
        """Emit ``loop { <body> } [until <until>];`` or ``then [m] loop ...`` (SysML.xtext:1615 WhileLoopNode; formal/2026-03-02).

        Args:
            body: Loop-body :class:`Body`.
            until: Optional post-condition expression.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string or ``body`` is not
                a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("until", until, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        return self._add_statement(
            "loop", then=then, until=until or "",
            body=body.operations,
            multiplicity=multiplicity,
        )

    def add_for(self, variable, collection, body, type=None, *, then=None, multiplicity=None):
        """Emit ``for <variable> [: <type>] in <collection> { <body> }`` or ``then [m] for ...`` (SysML.xtext:1624 ForLoopNode; formal/2026-03-02).

        Args:
            variable: Loop variable name.
            collection: Collection expression.
            body: Loop-body :class:`Body`.
            type: Optional loop-variable type.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string or ``body`` is not
                a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("variable", variable)
        _sequence_text("collection", collection)
        _sequence_text("type", type, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        return self._add_statement(
            "for", then=then, parameter=variable,
            value=collection, type_name=type or "", body=body.operations,
            multiplicity=multiplicity,
        )

    def add_terminate(self, occurrence=None, *, then=None, multiplicity=None):
        """Emit ``terminate [<occurrence>];`` or ``then [m] terminate ...;`` (SysML.xtext:1641 TerminateNode; formal/2026-03-02).

        Args:
            occurrence: Optional occurrence to terminate.
            then: Whether to prefix the item with ``then``; ``None`` chooses
                plain for the first item and ``then`` thereafter.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.

        Raises:
            TypeError: If a notation argument is not a string.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("occurrence", occurrence, optional=True)
        return self._add_statement(
            "terminate", then=then, value=occurrence or "",
            multiplicity=multiplicity,
        )

    def add_guarded_then(self, guard, ref):
        """Emit ``if <guard> then <ref>;`` (SysML.xtext:1708 GuardedTargetSuccession; formal/2026-03-02).

        Args:
            guard: Guard expression.
            ref: Target reference.

        Raises:
            TypeError: If ``guard`` or ``ref`` is not notation text.
        """
        _sequence_text("guard", guard)
        _sequence_text("ref", ref)
        self._operations.append(_sequence_statement(
            "", "if", ref=ref, condition=guard,
        ))
        return self

    def add_else(self, ref):
        """Emit ``else <ref>;`` (SysML.xtext:1714 DefaultTargetSuccession; formal/2026-03-02).

        Args:
            ref: Target reference.

        Raises:
            TypeError: If ``ref`` is not notation text.
        """
        _sequence_text("ref", ref)
        self._operations.append(_sequence_statement("", "else", ref=ref))
        return self


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
                   value=None, specializes=None, abstract=False, redefines=None,
                   default=False, direction=None, expression=None, doc=None):
        """Add one declaration, using strings for all SysML/KerML notation.

        ``expression`` writes a body expression for kinds whose bodies admit
        one: a constraint condition or a calc, case, analysis, verification, or
        use-case result expression.
        ``doc`` is plain documentation text, written as the new declaration's
        first body member ``doc /* ... */``; it reads back unchanged as
        ``Documentation.body``, and may not contain ``*/`` or a carriage return.
        """
        if not isinstance(kind, str):
            raise TypeError(f"kind must be notation text, not {kind.__class__.__name__}")
        for label, text in (("kind", kind), ("type", type),
                            ("multiplicity", multiplicity), ("value", value),
                            ("expression", expression)):
            if text is not None and not isinstance(text, str):
                raise TypeError(
                    f"{label} must be notation text, not "
                    f"{text.__class__.__name__}"
                )
        if not isinstance(name, str):
            raise TypeError(f"name must be notation text, not {name.__class__.__name__}")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        specializes = _notation_references("specializes", specializes)
        redefines = _notation_references("redefines", redefines)
        if not isinstance(abstract, bool):
            raise TypeError("abstract must be bool")
        if not isinstance(default, bool):
            raise TypeError("default must be bool")
        if direction is not None and not isinstance(direction, str):
            raise TypeError(f"direction must be notation text, not {direction.__class__.__name__}")
        if doc is not None and not isinstance(doc, str):
            raise TypeError(f"doc must be text, not {doc.__class__.__name__}")
        base = ("add_member", owner, kind, name, type or "", multiplicity or "",
                value or "", list(specializes))
        if (
            abstract
            or redefines
            or default
            or direction is not None
            or kind in ("ref", "return")
            or kind == ""
            or expression is not None
            or doc
        ):
            base += (abstract, list(redefines), default, direction or "")
        if expression is not None or doc:
            base += (expression or "",)
        if doc:
            base += (doc,)
        self._add(base)
        return self

    def add_documentation(self, target, body, name=None, locale=None, replace=False):
        """Add ``doc /* body */`` as the first body member of a declaration.

        A declaration ended by ``;`` is given a body. One that already owns
        documentation is refused unless ``replace`` is true, which rewrites the
        one it owns (and is refused if it owns several).

        Args:
            target (str or Symbol): Declaration to document, by FQN/id or symbol
            body (str): Plain documentation text, exactly as
                ``Documentation.body`` reads back (whitespace included); it may
                not contain ``*/``, which closes a comment, or a carriage return
            name (str): Optional documentation name, ``doc name /* ... */``
            locale (str): Optional locale, ``doc locale "en" /* ... */``
            replace (bool): Rewrite the target's documentation instead of
                refusing when it has one

        Returns:
            Editor: self, so operations can be chained
        """
        if not isinstance(body, str):
            raise TypeError(f"body must be text, not {body.__class__.__name__}")
        for label, text in (("name", name), ("locale", locale)):
            if text is not None and not isinstance(text, str):
                raise TypeError(f"{label} must be text, not {text.__class__.__name__}")
        if not isinstance(replace, bool):
            raise TypeError(f"replace must be a bool, not {replace.__class__.__name__}")
        self._add((
            "add_documentation", _target_id(target), body, name or "", locale or "", replace,
        ))
        return self

    def add_comment(self, owner, body, name=None, about=None, locale=None):
        """Add ``comment [name] [about a, b] [locale "..."] /* body */`` to a body.

        The comment goes where a new member of ``owner`` goes; a declaration
        ended by ``;`` is given a body, and an empty owner is the document root.

        Args:
            owner (str or Symbol): Namespace receiving the comment, by FQN/id
                or symbol; ``""`` for the top level
            body (str): Plain comment text, exactly as ``Comment.body`` reads
                back (whitespace included); it may not contain ``*/``, which
                closes a comment, or a carriage return
            name (str): Optional comment name, ``comment name /* ... */``
            about (list[str or Symbol]): Optional annotated elements, each a
                qualified name (or symbol) resolved from ``owner``
            locale (str): Optional locale, ``comment locale "en" /* ... */``

        Returns:
            Editor: self, so operations can be chained
        """
        if not isinstance(body, str):
            raise TypeError(f"body must be text, not {body.__class__.__name__}")
        for label, text in (("name", name), ("locale", locale)):
            if text is not None and not isinstance(text, str):
                raise TypeError(f"{label} must be text, not {text.__class__.__name__}")
        if isinstance(about, str):
            raise TypeError("about must be a sequence of names or symbols, not one name")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        about = tuple(_target_id(element) for element in about or ())
        self._add(("add_comment", owner, body, name or "", about, locale or ""))
        return self

    def add_note(self, target, text):
        """Write the line note ``// text`` on its own line above a declaration.

        A note is lexical trivia, not a model element: no query, export or
        ``to_api_json()`` sees it, but the edited text keeps it, and later
        edits that move or delete ``target`` carry it along.

        Args:
            target (str or Symbol): Declaration the note precedes, by FQN/id or
                symbol
            text (str): One line of note text; it may not contain a line break

        Returns:
            Editor: self, so operations can be chained
        """
        if not isinstance(text, str):
            raise TypeError(f"text must be text, not {text.__class__.__name__}")
        if "\n" in text or "\r" in text:
            raise ValueError("a note is one line: its text may not contain a line break")
        self._add(("add_note", _target_id(target), text))
        return self

    def add_satisfy(self, owner, requirement, by=None, asserted=False, negated=False):
        """Add a ``satisfy`` usage to a body that admits behavior usages."""
        if not isinstance(requirement, str):
            raise TypeError(
                f"requirement must be notation text, not {type(requirement).__name__}"
            )
        if by is not None and not isinstance(by, str):
            raise TypeError(f"by must be notation text, not {type(by).__name__}")
        if not isinstance(asserted, bool) or not isinstance(negated, bool):
            raise TypeError("asserted and negated must be bool")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(("add_satisfy", owner, requirement, by or "", asserted, negated))
        return self

    def add_requirement_constraint(self, owner, kind, expression, name=None):
        """Add a ``require`` or ``assume`` constraint to a requirement-like body."""
        for label, text in (("kind", kind), ("expression", expression)):
            if not isinstance(text, str):
                raise TypeError(f"{label} must be notation text, not {type(text).__name__}")
        if name is not None and not isinstance(name, str):
            raise TypeError(f"name must be notation text, not {type(name).__name__}")
        if name is None:
            name = ""
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(("add_requirement_constraint", owner, kind, expression, name))
        return self

    def add_transition(self, owner, source, target, name=None, trigger=None,
                       guard=None, effect=None):
        """Add a transition with optional trigger, guard and effect clauses."""
        if not isinstance(source, str):
            raise TypeError(f"source must be notation text, not {source.__class__.__name__}")
        if not isinstance(target, str):
            raise TypeError(f"target must be notation text, not {target.__class__.__name__}")
        for label, text in (("name", name), ("trigger", trigger),
                            ("guard", guard), ("effect", effect)):
            if text is not None and not isinstance(text, str):
                raise TypeError(f"{label} must be notation text, not {text.__class__.__name__}")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add((
            "add_transition", owner, name or "", source, target,
            trigger or "", guard or "", effect or "", False,
        ))
        return self

    def add_entry_transition(self, owner, target):
        """Add an entry transition to target in a state body."""
        if not isinstance(target, str):
            raise TypeError(f"target must be notation text, not {target.__class__.__name__}")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(("add_transition", owner, "", "", target, "", "", "", True))
        return self

    def add_first(self, owner, ref, after=None):
        """Emit ``first <ref>;`` (SysML.xtext:1384 InitialNodeMember; formal/2026-03-02).

        Args:
            owner: The action body, by qualified name or Symbol.
            ref: The node the `first` sequences from, a feature reference such
                as ``start`` or the name of a member the body declares.
            after: Optional name of the body member the `first` follows;
                by default it is appended at the end of the body.

        Raises:
            TypeError: If ``ref`` is not notation text or ``after`` is not a
                member name.
        """
        if not isinstance(ref, str):
            raise TypeError(f"ref must be notation text, not {ref.__class__.__name__}")
        if after is not None and not isinstance(after, str):
            raise TypeError(f"after must be a member name, not {after.__class__.__name__}")
        options = (after or "", None)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(owner, "first", ref=ref, options=options))
        return self

    def add_then(self, owner, ref=None, action=None, type=None, after=None,
                 kind="action", multiplicity=None):
        """Emit ``then [m] <ref>;`` or ``then [m] <kind> <name> : <type>;`` (SysML.xtext:878, 887, 1703 TargetSuccession; formal/2026-03-02).

        The member is sequenced after the member before it.

        Args:
            owner: The action body, by qualified name or Symbol.
            ref: The node the `then` sequences to — a feature reference such
                as ``done`` or the name of a member the body declares.
            action: The name of the member the `then` declares, written with
                `kind` (default ``"action"``) and an optional `type`. Exactly
                one of `ref` and `action` is required.
            type: Optional typing target of the declared member.
            after: Optional name of the body member the `then` follows: the
                new member sequences from it, and a `then` that previously
                followed it now sequences from the new member.
            kind: The usage kind the `then` declares: ``"action"``,
                ``"perform action"``, ``"state"``, ``"merge"``, ``"decide"``,
                ``"join"`` or ``"fork"``.
            multiplicity: Optional bracketed source-end multiplicity ``[m]``.

        Raises:
            TypeError: If a text argument is not a string.
            ValueError: If both or neither of `ref` and `action` is given, or
                `ref` comes with declaration fields, or the multiplicity is
                not valid for the requested form.
        """
        for label, text in (("ref", ref), ("action", action), ("type", type),
                            ("after", after), ("kind", kind),
                            ("multiplicity", multiplicity)):
            if text is not None and not isinstance(text, str):
                raise TypeError(f"{label} must be notation text, not {text.__class__.__name__}")
        options = (after or "", multiplicity)
        if (ref is None) == (action is None):
            raise ValueError("exactly one of ref and action is required")
        if ref is not None and (type is not None or
                                (kind is not None and kind != "action")):
            raise ValueError("a then reference takes no type or kind")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        if ref is not None:
            self._add(_sequence_statement(
                owner, "then", ref=ref, options=options,
            ))
        else:
            self._add(_sequence_statement(
                owner, "then", kind if kind is not None else "action",
                member_name=action, type_name=type or "", options=options,
            ))
        return self

    def add_accept(self, owner, payload, type=None, via=None, *, then=True,
                   multiplicity=None, after=None):
        """Emit ``then [m] accept <payload> [: <type>] [via <via>];`` (SysML.xtext:1442 AcceptNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            payload: Payload parameter or trigger notation.
            type: Optional accepted payload type.
            via: Optional port expression.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string or ``then`` is not
                a bool.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("payload", payload)
        _sequence_text("type", type, optional=True)
        _sequence_text("via", via, optional=True)
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "accept",
            type_name=type or "", options=options,
            parameter=payload, via=via or "",
        ))
        return self

    def add_send(self, owner, payload, to=None, via=None, *, then=True,
                 multiplicity=None, after=None):
        """Emit ``then [m] send <payload> [via <via>] [to <to>];`` (SysML.xtext:1499 SendNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            payload: Payload expression.
            to: Optional receiver expression.
            via: Optional port expression.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string or ``then`` is not
                a bool.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("payload", payload)
        _sequence_text("to", to, optional=True)
        _sequence_text("via", via, optional=True)
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "send",
            options=options,
            value=payload, target=to or "", via=via or "",
        ))
        return self

    def add_assign(self, owner, target, value, *, then=True, multiplicity=None,
                   after=None):
        """Emit ``then [m] assign <target> := <value>;`` (SysML.xtext:1535 AssignmentNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            target: Feature reference to assign.
            value: Assigned expression.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string or ``then`` is not
                a bool.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("target", target)
        _sequence_text("value", value)
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "assign",
            options=options,
            target=target, value=value,
        ))
        return self

    def add_if(self, owner, condition, body, else_body=None, *, then=True,
               multiplicity=None, after=None):
        """Emit ``then [m] if <condition> { <body> } [else { <else_body> }]`` (SysML.xtext:1596 IfNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix. An
        empty else body is equivalent to omitting ``else``.

        Args:
            owner: The action body, by qualified name or Symbol.
            condition: Boolean condition expression.
            body: Then-branch :class:`Body`.
            else_body: Optional else-branch :class:`Body`.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string, ``then`` is not a
                bool, or a branch is not a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("condition", condition)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        if else_body is not None and not isinstance(else_body, Body):
            raise TypeError(f"else_body must be Body or None, not {else_body.__class__.__name__}")
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "if",
            options=options,
            condition=condition, body=body.operations,
            else_body=else_body.operations if else_body and else_body.operations else [],
        ))
        return self

    def add_while(self, owner, condition, body, until=None, *, then=True,
                  multiplicity=None, after=None):
        """Emit ``then [m] while <condition> { <body> } [until <until>];`` (SysML.xtext:1615 WhileLoopNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            condition: Loop condition expression.
            body: Loop-body :class:`Body`.
            until: Optional post-condition expression.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string, ``then`` is not a
                bool, or ``body`` is not a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("condition", condition)
        _sequence_text("until", until, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "while",
            options=options,
            condition=condition, until=until or "",
            body=body.operations,
        ))
        return self

    def add_loop(self, owner, body, until=None, *, then=True,
                 multiplicity=None, after=None):
        """Emit ``then [m] loop { <body> } [until <until>];`` (SysML.xtext:1615 WhileLoopNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            body: Loop-body :class:`Body`.
            until: Optional post-condition expression.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string, ``then`` is not a
                bool, or ``body`` is not a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("until", until, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "loop",
            options=options,
            until=until or "", body=body.operations,
        ))
        return self

    def add_for(self, owner, variable, collection, body, type=None, *, then=True,
                multiplicity=None, after=None):
        """Emit ``then [m] for <variable> [: <type>] in <collection> { <body> }`` (SysML.xtext:1624 ForLoopNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            variable: Loop variable name.
            collection: Collection expression.
            body: Loop-body :class:`Body`.
            type: Optional loop-variable type.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string, ``then`` is not a
                bool, or ``body`` is not a :class:`Body`.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("variable", variable)
        _sequence_text("collection", collection)
        _sequence_text("type", type, optional=True)
        if not isinstance(body, Body):
            raise TypeError(f"body must be Body, not {body.__class__.__name__}")
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "for",
            type_name=type or "", options=options,
            parameter=variable, value=collection, body=body.operations,
        ))
        return self

    def add_terminate(self, owner, occurrence=None, *, then=True,
                      multiplicity=None, after=None):
        """Emit ``then [m] terminate [<occurrence>];`` (SysML.xtext:1641 TerminateNode; formal/2026-03-02).

        Without ``then`` this emits the same notation without that prefix.

        Args:
            owner: The action body, by qualified name or Symbol.
            occurrence: Optional occurrence to terminate.
            then: Whether to prefix the statement with ``then``.
            multiplicity: Optional source-end multiplicity, emitted with ``then``.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string or ``then`` is not
                a bool.
            ValueError: If ``multiplicity`` is used without ``then``.
        """
        _sequence_text("occurrence", occurrence, optional=True)
        options = _sequence_options(after, multiplicity)
        if not isinstance(then, bool):
            raise TypeError(f"then must be bool, not {then.__class__.__name__}")
        _sequence_keyword_options("then" if then else "", options)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "then" if then else "", "terminate",
            options=options,
            value=occurrence or "",
        ))
        return self

    def add_guarded_then(self, owner, guard, ref, *, after=None):
        """Emit ``if <guard> then <ref>;`` (SysML.xtext:1708 GuardedTargetSuccession; formal/2026-03-02).

        Args:
            owner: The action body, by qualified name or Symbol.
            guard: Guard expression.
            ref: Target reference.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string.
        """
        _sequence_text("guard", guard)
        _sequence_text("ref", ref)
        options = _sequence_options(after)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(
            owner, "if", ref=ref, options=options, condition=guard,
        ))
        return self

    def add_else(self, owner, ref, *, after=None):
        """Emit ``else <ref>;`` (SysML.xtext:1714 DefaultTargetSuccession; formal/2026-03-02).

        Args:
            owner: The action body, by qualified name or Symbol.
            ref: Target reference.
            after: Optional body member to insert after.

        Raises:
            TypeError: If a notation argument is not a string.
        """
        _sequence_text("ref", ref)
        options = _sequence_options(after)
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(_sequence_statement(owner, "else", ref=ref, options=options))
        return self

    def add_import(self, owner, target, visibility=None, recursive=False,
                   all=False, filter=None):
        """Add an import declaration to a namespace body or the document root.

        ``target`` is the imported qualified name, optionally ``$::``-rooted:
        ``A::B`` for a membership import, ``A::*`` for a namespace import.
        ``visibility`` is ``private``,
        ``public`` or ``protected``; ``None`` writes ``private``, the indicator
        the grammar requires and the one legal in every body including the
        document root. ``recursive`` writes ``::**``, ``all`` writes
        ``import all``, and ``filter`` is one expression string or a list or
        tuple of them, each written ``[<expression>]``.
        """
        if not isinstance(target, str):
            raise TypeError(f"target must be notation text, not {target.__class__.__name__}")
        if visibility is not None and not isinstance(visibility, str):
            raise TypeError(
                f"visibility must be notation text or None, not {visibility.__class__.__name__}")
        if not isinstance(recursive, bool):
            raise TypeError(f"recursive must be a bool, not {recursive.__class__.__name__}")
        if not isinstance(all, bool):
            raise TypeError(f"all must be a bool, not {all.__class__.__name__}")
        if filter is None:
            filters = ()
        elif isinstance(filter, str):
            filters = (filter,)
        elif isinstance(filter, (list, tuple)):
            filters = tuple(filter)
        else:
            raise TypeError(
                f"filter must be notation text or a list of it, not {filter.__class__.__name__}")
        for index, expression in enumerate(filters):
            if not isinstance(expression, str):
                raise TypeError(f"filter[{index}] must be notation text")
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add((
            "add_import", owner, visibility or "", target, recursive, all, filters,
        ))
        return self

    def add_require_constraint(self, owner, expression, name=None):
        """Add a ``require constraint`` to a requirement-like body."""
        return self.add_requirement_constraint(owner, "require", expression, name)

    def add_assume_constraint(self, owner, expression, name=None):
        """Add an ``assume constraint`` to a requirement-like body."""
        return self.add_requirement_constraint(owner, "assume", expression, name)

    def add_connection(self, owner, kind, from_, to, name=None, type=None):
        """Add a connection-like usage between two feature references."""
        for label, text in (("kind", kind), ("from_", from_), ("to", to)):
            if not isinstance(text, str):
                raise TypeError(
                    f"{label} must be notation text, not {text.__class__.__name__}"
                )
        for label, text in (("name", name), ("type", type)):
            if text is not None and not isinstance(text, str):
                raise TypeError(
                    f"{label} must be notation text, not {text.__class__.__name__}"
                )
        owner = owner if isinstance(owner, str) else _target_id(owner)
        self._add(("add_connection", owner, kind, from_, to, name or "", type or ""))
        return self

    def add_allocation(self, owner, from_, to, **kwargs):
        """Add an ``allocation ... allocate from_ to to`` usage."""
        return self.add_connection(owner, "allocation", from_, to, **kwargs)

    def add_flow(self, owner, from_, to, **kwargs):
        """Add a ``flow ... from from_ to to`` usage."""
        return self.add_connection(owner, "flow", from_, to, **kwargs)

    def add_succession(self, owner, from_, to, **kwargs):
        """Add a ``succession ... first from_ then to`` usage."""
        return self.add_connection(owner, "succession", from_, to, **kwargs)

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

    def add_calc_def(
        self, owner, name, inputs=None, return_type=None, return_expression=None,
        expression=None, **kwargs
    ):
        """Add a ``calc def`` with input parameters and an optional result.

        ``return_expression`` requires ``return_type`` and is bound to that
        result parameter; it does not write a ``return <expr>;`` statement.
        ``expression`` writes the calculation body's result expression.
        """
        inputs = _parameter_pairs(inputs, "inputs")
        _optional_text(return_type, "return_type")
        _optional_text(return_expression, "return_expression")
        _optional_text(expression, "expression")
        if expression is not None and return_expression is not None:
            raise ValueError("expression and return_expression both bind the result; give one")
        if return_expression is not None and not return_type:
            raise ValueError("return_expression requires return_type")
        owner = _owner_id(owner)
        self.add_member(owner, "calc def", name, expression=expression, **kwargs)
        qualified_name = name if owner == "" else owner + "::" + name
        for parameter_name, parameter_type in inputs:
            self.add_parameter(
                qualified_name, "in", parameter_name, type=parameter_type
            )
        if return_type is not None or return_expression is not None:
            self.add_return(
                qualified_name, type=return_type, value=return_expression
            )
        return self

    def add_calc(
        self, owner, name, inputs=None, return_type=None, return_expression=None,
        expression=None, **kwargs
    ):
        """Add a ``calc`` with input parameters and an optional result.

        ``return_expression`` requires ``return_type`` and is bound to that
        result parameter; it does not write a ``return <expr>;`` statement.
        ``expression`` writes the calculation body's result expression.
        """
        inputs = _parameter_pairs(inputs, "inputs")
        _optional_text(return_type, "return_type")
        _optional_text(return_expression, "return_expression")
        _optional_text(expression, "expression")
        if expression is not None and return_expression is not None:
            raise ValueError("expression and return_expression both bind the result; give one")
        if return_expression is not None and not return_type:
            raise ValueError("return_expression requires return_type")
        owner = _owner_id(owner)
        self.add_member(owner, "calc", name, expression=expression, **kwargs)
        qualified_name = name if owner == "" else owner + "::" + name
        for parameter_name, parameter_type in inputs:
            self.add_parameter(
                qualified_name, "in", parameter_name, type=parameter_type
            )
        if return_type is not None or return_expression is not None:
            self.add_return(
                qualified_name, type=return_type, value=return_expression
            )
        return self

    def add_parameter(self, owner, direction, name, type=None, kind=None, **kwargs):
        """Add a directional parameter usage.

        By default the usage spells without a kind keyword — ``in x : T;``, an
        implicit directed usage. An explicit `kind` is passed through as today:
        ``kind="ref"`` writes ``in ref x : T;``, which this runtime reads as a
        performer-bound reference parameter.
        """
        return self.add_member(
            owner, kind or "", name, type=type, direction=direction, **kwargs
        )

    def add_return(self, owner, name="", **kwargs):
        """Add a return parameter member."""
        return self.add_member(owner, "return", name, **kwargs)

    def add_action_def(self, owner, name, inputs=None, outputs=None, **kwargs):
        """Add an ``action def`` with input and output parameters."""
        inputs = _parameter_pairs(inputs, "inputs")
        outputs = _parameter_pairs(outputs, "outputs")
        owner = _owner_id(owner)
        self.add_member(owner, "action def", name, **kwargs)
        qualified_name = name if owner == "" else owner + "::" + name
        for parameter_name, parameter_type in inputs:
            self.add_parameter(
                qualified_name, "in", parameter_name, type=parameter_type
            )
        for parameter_name, parameter_type in outputs:
            self.add_parameter(
                qualified_name, "out", parameter_name, type=parameter_type
            )
        return self

    def add_action(self, owner, name, inputs=None, outputs=None, **kwargs):
        """Add an ``action`` with input and output parameters."""
        inputs = _parameter_pairs(inputs, "inputs")
        outputs = _parameter_pairs(outputs, "outputs")
        owner = _owner_id(owner)
        self.add_member(owner, "action", name, **kwargs)
        qualified_name = name if owner == "" else owner + "::" + name
        for parameter_name, parameter_type in inputs:
            self.add_parameter(
                qualified_name, "in", parameter_name, type=parameter_type
            )
        for parameter_name, parameter_type in outputs:
            self.add_parameter(
                qualified_name, "out", parameter_name, type=parameter_type
            )
        return self

    def add_perform_action(self, owner, name, type=None, **kwargs):
        """Add a ``perform action name : Type`` usage (SysML v2 7.17.6)."""
        return self.add_member(owner, "perform action", name, type=type, **kwargs)

    def add_perform(self, owner, action, doc=None):
        """Add a ``perform <action>;`` usage naming an existing action usage;
        the member is named by the action it references."""
        return self.add_member(owner, "perform", action, doc=doc)

    def add_exhibit_state(self, owner, name, type=None):
        """Add an ``exhibit state`` usage."""
        return self.add_member(owner, "exhibit state", name, type=type)

    def add_exhibit(self, owner, state):
        """Add an ``exhibit <state>;`` usage named by its referenced state."""
        return self.add_member(owner, "exhibit", state)

    def add_state_action(self, owner, kind, name, type=None):
        """Add an ``entry``, ``do`` or ``exit`` action to a state body."""
        if not isinstance(kind, str):
            raise TypeError(
                f"kind must be notation text, not {kind.__class__.__name__}"
            )
        if kind not in ("entry", "do", "exit"):
            raise ValueError("kind must be 'entry', 'do' or 'exit'")
        return self.add_member(owner, f"{kind} action", name, type=type)

    def add_state_def(self, owner, name, **kwargs):
        """Add a ``state def`` declaration."""
        return self.add_member(owner, "state def", name, **kwargs)

    def add_state(self, owner, name, **kwargs):
        """Add a ``state`` declaration."""
        return self.add_member(owner, "state", name, **kwargs)

    def add_constraint_def(self, owner, name, expression=None, **kwargs):
        """Add a ``constraint def``; ``expression=`` writes ``{ … }`` while
        ``value=`` writes a feature value with ``= …``."""
        return self.add_member(
            owner, "constraint def", name, expression=expression, **kwargs
        )

    def add_constraint(self, owner, name, expression=None, **kwargs):
        """Add a ``constraint``; ``expression=`` writes ``{ … }`` while
        ``value=`` writes a feature value with ``= …``."""
        return self.add_member(
            owner, "constraint", name, expression=expression, **kwargs
        )

    def add_assert_constraint(
        self, owner, name=None, type=None, expression=None, negated=False
    ):
        """Add an asserted constraint usage, optionally negated."""
        if not isinstance(negated, bool):
            raise TypeError("negated must be bool")
        return self.add_member(
            owner,
            "assert not constraint" if negated else "assert constraint",
            "" if name is None else name,
            type=type,
            expression=expression,
        )

    def add_assert(self, owner, ref, negated=False):
        """Add an anonymous ``assert`` usage; the assertion is not named by ref."""
        if not isinstance(ref, str):
            raise TypeError(f"ref must be notation text, not {type(ref).__name__}")
        if not isinstance(negated, bool):
            raise TypeError("negated must be bool")
        return self.add_member(owner, "assert not" if negated else "assert", ref)

    def add_requirement_def(self, owner, name, **kwargs):
        """Add a ``requirement def`` declaration."""
        return self.add_member(owner, "requirement def", name, **kwargs)

    def add_requirement(self, owner, name, **kwargs):
        """Add a ``requirement`` declaration."""
        return self.add_member(owner, "requirement", name, **kwargs)


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


def _owner_id(owner):
    return owner if isinstance(owner, str) else _target_id(owner)


def _parameter_pairs(parameters, argument):
    if parameters is None:
        return []
    if not isinstance(parameters, list):
        raise TypeError(
            f"{argument} must be a list of 2-tuples of strings, "
            f"not {parameters.__class__.__name__}"
        )
    pairs = []
    for index, pair in enumerate(parameters):
        if not isinstance(pair, tuple) or len(pair) != 2:
            raise TypeError(f"{argument}[{index}] must be a 2-tuple of strings")
        if not all(isinstance(value, str) for value in pair):
            raise TypeError(f"{argument}[{index}] name and type must be strings")
        pairs.append(pair)
    return pairs


def _optional_text(value, argument):
    if value is not None and not isinstance(value, str):
        raise TypeError(
            f"{argument} must be notation text or None, "
            f"not {value.__class__.__name__}"
        )


def _notation_references(label, values):
    """Normalize one feature-reference string or a sequence of them."""
    if values is None:
        return []
    if isinstance(values, str):
        values = [values]
    elif not isinstance(values, Sequence):
        raise TypeError(f"{label} must be a notation string or sequence of strings")
    references = list(values)
    if not all(isinstance(reference, str) for reference in references):
        raise TypeError(f"{label} must contain only notation strings")
    return references


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

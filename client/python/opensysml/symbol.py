"""Symbol class for navigating the semantic model."""

from typing import Any, Dict, FrozenSet, Iterable, List, Optional, Protocol, Tuple, TYPE_CHECKING, Union

from opensysml.errors import UnresolvedChildError
from opensysml.capabilities import (
    CAPABILITY_SYMBOL_ATTRIBUTES,
    CAPABILITY_SYMBOL_MEMBERS,
    require,
    upgrade_remedy,
)
from opensysml.typefacts import (
    AttributeFacts,
    Multiplicity,
    Specialization,
    SymbolFacts,
    TypeFacts,
)

if TYPE_CHECKING:
    from opensysml.proto import sysml_pb2

# Kind strings emitted by the service (internal/semantic/symbols: symbolKindNames).
# Matched case-insensitively so older PascalCase producers still work.
ATTRIBUTE_KINDS = frozenset({"attributedef", "attributeusage", "referenceusage"})
PART_KINDS = frozenset({"partdef", "partusage"})
PORT_KINDS = frozenset({"portdef", "portusage"})
BINDING_KINDS = frozenset({"bindingusage"})
CONNECTION_KINDS = frozenset({
    "connectiondef", "connectionusage", "bindingusage", "interfacedef",
    "interfaceusage", "flowusage", "allocationdef", "allocationusage",
})

KindFilter = Union[str, Iterable[str], None]


class Members(List["Symbol"]):
    """Member symbols, with how many standard-library members were left out.

    A list, so it is used as one; ``withheld_library`` says how many members
    inherited from standard-library content the service withheld, so their
    absence is stated rather than silent.
    """

    def __init__(self, members: Iterable["Symbol"] = (), withheld_library: int = 0):
        super().__init__(members)
        self.withheld_library = withheld_library


class SymbolSource(Protocol):
    """What a symbol needs of the connection it navigates through."""

    def get_symbol(self, model_hash: str, symbol_id: str, include_library_members: bool = False) -> Any:
        """The ``SymbolInfo`` for an id, or None when the model has no such symbol."""

    def server_info(self) -> Any:
        """What the service reports about itself, for a capability check."""


def _type_text(symbol, attribute_facts) -> Optional[str]:
    """Name a member's type: its own resolved type, else what the service resolved
    for it as an attribute."""
    facts = symbol.type_facts
    if facts is not None:
        text = facts.resolved_id or facts.declared or facts.primitive
        if text:
            return text
    if attribute_facts is not None and attribute_facts.type:
        return attribute_facts.type
    return None


def _multiplicity_text(symbol) -> Optional[str]:
    """Render a member's declared multiplicity as ``lower..upper``, or None."""
    multiplicity = symbol.multiplicity
    if multiplicity is None:
        return None
    return f"{multiplicity.lower}..{multiplicity.upper}"


class Symbol:
    """Represents a symbol in the SysML model (definition or usage).
    
    Wraps a SymbolInfo protobuf message and provides lazy navigation
    through the symbol tree via an optional RPC client.
    
    Attributes:
        id: Unique symbol identifier (fully-qualified name)
        name: Simple name of the symbol
        kind: SysML element kind (e.g., "partDef", "attributeUsage")
    """
    
    def __init__(self, pb_symbol: "sysml_pb2.SymbolInfo", client: Optional[SymbolSource],
                 model_hash: str, inherited_from: Optional[str] = None):
        """Initialize Symbol from SymbolInfo protobuf.
        
        Args:
            pb_symbol: SymbolInfo protobuf message
            client: Optional RPC client for lazy loading children
            model_hash: Hash of the model this symbol belongs to
            inherited_from: FQN of the type declaring this symbol when it was
                reached as an inherited member of another; None otherwise
        """
        self._pb = pb_symbol
        self._client = client
        self._model_hash = model_hash
        self._inherited_from = inherited_from
        self._members_cache: Dict[bool, Members] = {}
        self._own_cache: Optional[Members] = None
        self._walked_cache: Optional[Members] = None

    @property
    def inherited(self) -> bool:
        """True when this symbol was reached as a member another symbol
        inherits rather than declares; False for a symbol looked up directly."""
        return self._inherited_from is not None

    @property
    def inherited_from(self) -> Optional[str]:
        """FQN of the type declaring this symbol when it was reached as an
        inherited member, None otherwise."""
        return self._inherited_from

    @property
    def withheld_library_members(self) -> int:
        """Return how many standard-library-inherited members were withheld.

        :meth:`members` leaves out the members an element inherits from
        standard-library content (the dozens every part has through
        ``Parts::Part`` and ``Base::Anything``) unless asked for them; this says
        how many, so their absence is stated rather than silent.
        """
        return self._pb.withheld_library_members
    
    @property
    def id(self) -> str:
        """Return symbol ID (fully-qualified name)."""
        return self._pb.id
    
    @property
    def name(self) -> str:
        """Return simple name."""
        return self._pb.name
    
    @property
    def kind(self) -> str:
        """Return SysML element kind."""
        return self._pb.kind
    
    @property
    def metadata(self) -> dict:
        """Return copy of symbol metadata dictionary.

        Keys present only when they hold: ``visibility``, ``type``,
        ``multiplicity``, ``direction``, ``specializes``, ``abstract``,
        ``variation`` (a variation definition or usage, or an enumeration
        definition; always ``abstract`` too) and ``variant`` (a variant usage
        or an enumerated value), each flag ``"true"``.
        """
        return dict(self._pb.metadata)
    
    @property
    def type_facts(self) -> Optional[TypeFacts]:
        """Return the symbol's static type, or None when it carries no type."""
        if not self._pb.HasField("type_info"):
            return None
        return TypeFacts.from_pb(self._pb.type_info)

    @property
    def multiplicity(self) -> Optional[Multiplicity]:
        """Return the declared multiplicity range, or None when undeclared."""
        if not self._pb.HasField("multiplicity"):
            return None
        return Multiplicity.from_pb(self._pb.multiplicity)

    @property
    def specializations(self) -> List[Specialization]:
        """Return all generalization edges declared on this symbol."""
        return [Specialization.from_pb(spec) for spec in self._pb.specializations]

    @property
    def withheld_library_attributes(self) -> int:
        """Return how many standard-library-inherited attributes were withheld.

        The service reports a model's own and non-library-inherited attributes;
        this says how many of the metamodel frame's it left out, so their absence
        is stated rather than silent.
        """
        return self._pb.withheld_library_attributes

    def attribute_facts(self) -> List[AttributeFacts]:
        """Return every attribute this symbol has, own and inherited, with its
        resolved type, constant default value and unit. Attributes inherited from
        standard-library content are withheld and counted by
        :attr:`withheld_library_attributes`.

        This is the attribute set the service resolved, so unlike
        :meth:`attributes` it needs no further RPC and reports each attribute's
        default value.

        Raises:
            MissingCapabilityError: The service does not report
                ``symbol_attributes``, so it answers with an empty set that
                cannot be told from an element having no attributes.
        """
        self._require_resolved_attributes()
        return self._attribute_facts()

    def _attribute_facts(self) -> List[AttributeFacts]:
        """Return the attribute facts the service reported, whatever they are."""
        return [AttributeFacts.from_pb(attr) for attr in self._pb.attributes]

    def _require_resolved_attributes(self) -> None:
        """Name a service whose attribute set is empty because it predates the feature."""
        if self._client is None:
            return
        require(
            self._client.server_info(),
            CAPABILITY_SYMBOL_ATTRIBUTES,
            upgrade_remedy(CAPABILITY_SYMBOL_ATTRIBUTES),
        )

    def facts(self) -> SymbolFacts:
        """Return this symbol's static facts, detached from the protobuf message.

        ``attributes`` carries what the service reported, which is nothing from
        one predating the attribute set; :meth:`attribute_facts` names such a
        service instead.
        """
        return SymbolFacts(
            id=self.id,
            name=self.name,
            kind=self.kind,
            type=self.type_facts,
            multiplicity=self.multiplicity,
            specializations=tuple(self.specializations),
            attributes=tuple(self._attribute_facts()),
            withheld_library_attributes=self.withheld_library_attributes,
        )

    def members(self, kind: KindFilter = None, inherited: bool = False,
                library: bool = False) -> Members:
        """Return this symbol's members, as the service enumerates them.

        Own members come first in declaration order, then — with ``inherited`` —
        the members inherited through specialization and typing, in inheritance
        order, as KerML ``Type::member`` lists them: a redefining member hides
        the one it redefines and a private member of a supertype is not
        inherited. Each inherited member reports :attr:`inherited` and
        :attr:`inherited_from`. Members inherited from standard-library content
        are withheld and counted in the result's ``withheld_library`` unless
        ``library`` is set, which fetches them too.

        Args:
            kind: Kind name or names to keep (``"partUsage"``, ``PART_KINDS``),
                matched case-insensitively; None keeps every kind
            inherited: Include inherited members, not only owned ones
            library: With ``inherited``, include the members inherited from
                standard-library content too; an owned member is never
                withheld, so without ``inherited`` this changes nothing

        Returns:
            :class:`Members`, a list of Symbol objects; empty if no client is set

        Raises:
            MissingCapabilityError: ``inherited`` was asked of a service without
                ``symbol_members``, which cannot enumerate inherited members
        """
        if self._client is None:
            return Members()
        if inherited:
            require(
                self._client.server_info(),
                CAPABILITY_SYMBOL_MEMBERS,
                upgrade_remedy(CAPABILITY_SYMBOL_MEMBERS),
            )
            found = self._all_members(library)
        else:
            found = self._own_members()
        kinds = _kind_set(kind)
        if kinds is None:
            return found
        return Members((m for m in found if m.kind.lower() in kinds), found.withheld_library)

    def properties(self, *names):
        """Read this element's metamodel properties by their specification names.

        :meth:`Model.element_properties` for this symbol: ``names`` are
        specification names (``"inheritedFeature"``, ``"isComposite"``); none
        reads every property the element form carries. See
        :mod:`opensysml.element_properties`.

        Returns:
            ElementProperties: The element and one entry per property

        Raises:
            MissingCapabilityError: If the service lacks ``element_properties``
            ValueError: If no client is set
        """
        if self._client is None:
            raise ValueError("this symbol has no client to read its properties from")
        return self._client.get_element_properties(self._model_hash, self.id, names)

    def property(self, name):
        """Read one metamodel property of this element by its specification name.

        Returns:
            ElementProperty: Its values, or why it has none
        """
        return self.properties(name)[name]

    def _own_members(self) -> Members:
        """Return the owned members, from ``members`` where the service
        enumerates them and from ``child_ids`` otherwise."""
        if self._own_cache is None:
            if self._pb.members:
                every = self._all_members(False)
                self._own_cache = Members(
                    (m for m in every if not m.inherited), every.withheld_library
                )
            else:
                self._own_cache = Members(
                    self._fetch_all((child_id, None) for child_id in self._pb.child_ids),
                    self._pb.withheld_library_members,
                )
        return self._own_cache

    def _all_members(self, library: bool) -> Members:
        """Return every member, own then inherited, fetching the standard-library
        ones too when ``library``; cached per choice."""
        cached = self._members_cache.get(library)
        if cached is not None:
            return cached
        pb = self._pb
        if library and pb.withheld_library_members:
            refetched = self._client.get_symbol(self._model_hash, self.id, include_library_members=True)
            if refetched is not None:
                pb = refetched
        symbols = self._fetch_all(
            (info.id, info.inherited_from if info.inherited else None) for info in pb.members
        )
        result = Members(symbols, pb.withheld_library_members)
        self._members_cache[library] = result
        return result

    def _fetch_all(self, ids: Iterable[Tuple[str, Optional[str]]]) -> List["Symbol"]:
        """Resolve every listed member id through the client, raising
        :class:`UnresolvedChildError` naming the ids the service does not resolve
        rather than dropping them."""
        symbols: List["Symbol"] = []
        unresolved: List[str] = []
        for symbol_id, inherited_from in ids:
            info = self._client.get_symbol(self._model_hash, symbol_id)
            if info is None:
                unresolved.append(symbol_id)
                continue
            symbols.append(Symbol(info, self._client, self._model_hash, inherited_from))
        if unresolved:
            raise UnresolvedChildError(self.id, unresolved)
        return symbols

    def children(self, inherited: bool = False, library: bool = False) -> Members:
        """Return this symbol's member symbols of every kind.

        By default only the members this symbol itself declares (its owned
        members, the service's ``child_ids``); ``inherited=True`` adds the
        members inherited through specialization and typing, with redefined and
        private members of the supertypes left out, and ``library=True`` the
        members inherited from standard-library content, otherwise withheld and
        counted in the result's ``withheld_library``. See :meth:`members`.

        An unnamed member (``bind a = b;``, ``connect a to b;``, ``part : P;``,
        ``@M { … }`` or a ``#M`` prefix) is listed under the positional id the
        query API and the export give it, such as ``Pkg::Owner::@3``.

        Lazily fetches each member using the RPC client if available, and
        caches the results. Returns an empty list if no client is set.

        Returns:
            :class:`Members`, a list of Symbol objects

        Raises:
            UnresolvedChildError: the service listed a member it then could not
                resolve, so the members cannot be read completely
        """
        return self.members(None, inherited, library)

    def attributes(self, inherited: bool = True, library: bool = False) -> Members:
        """Return attribute members: ``attributeDef``, ``attributeUsage`` and
        ``referenceUsage``.

        Unlike the other member methods this includes inherited attributes by
        default, the set :meth:`attribute_facts` and
        :attr:`withheld_library_attributes` describe; ``inherited=False`` keeps
        the declared ones only. Attributes inherited from standard-library
        content are withheld unless ``library`` is set. See :meth:`members`.

        A service without ``symbol_members`` cannot enumerate inherited members,
        so against one this method still follows the declared specializations
        itself, as it did before the service enumerated members: the inherited
        attributes it finds are ordered as :meth:`attribute_facts` reports them,
        carry :attr:`inherited_from`, and the result counts
        :attr:`withheld_library_attributes`; ``library=True`` still needs the
        capability, since such a service cannot list library content.

        Returns:
            :class:`Members`, a list of attribute Symbol objects
        """
        if (
            inherited
            and not library
            and self._client is not None
            and not self._client.server_info().has(CAPABILITY_SYMBOL_MEMBERS)
        ):
            return self._walked_attributes()
        return self.members(ATTRIBUTE_KINDS, inherited, library)

    def _walked_attributes(self) -> Members:
        """Return own and inherited attributes by following the declared
        specializations client-side, for a service that cannot enumerate
        inherited members; cached."""
        if self._walked_cache is not None:
            return self._walked_cache
        declared = [m for m in self._own_members() if m.kind.lower() in ATTRIBUTE_KINDS]
        by_name = {attr.name: attr for attr in declared}
        for inherited in self._inherited_attributes(set()):
            by_name.setdefault(inherited.name, inherited)

        reported = [attr.name for attr in self._pb.attributes]
        ordered = [by_name[name] for name in reported if name in by_name]
        # A declared attribute the service did not report — an unnamed one, or an
        # older service — is still this symbol's, so it is kept.
        ordered += [attr for attr in declared if attr.name not in reported]
        self._walked_cache = Members(ordered, self._pb.withheld_library_attributes)
        return self._walked_cache

    def _inherited_attributes(self, visited: set) -> List["Symbol"]:
        """Return the attributes the supertypes declare, nearest first, each
        marked as inherited from the type declaring it. Every generalization
        edge is followed, typing included: ``part car : Car`` has the attributes
        ``Car`` declares."""
        if self._client is None or self.id in visited:
            return []
        visited.add(self.id)

        result: List["Symbol"] = []
        for spec in self.specializations:
            if not spec.target_id or spec.target_id in visited:
                continue
            info = self._client.get_symbol(self._model_hash, spec.target_id)
            if info is None:
                continue
            supertype = Symbol(info, self._client, self._model_hash)
            result += [
                Symbol(attr._pb, self._client, self._model_hash, supertype.id)
                for attr in supertype._own_members()
                if attr.kind.lower() in ATTRIBUTE_KINDS
            ]
            result += supertype._inherited_attributes(visited)
        return result

    def parts(self, inherited: bool = False, library: bool = False) -> Members:
        """Return part members: ``partDef`` and ``partUsage``, own by default
        and the inherited ones with ``inherited=True``. See :meth:`members`."""
        return self.members(PART_KINDS, inherited, library)

    def ports(self, inherited: bool = False, library: bool = False) -> Members:
        """Return port members: ``portDef`` and ``portUsage``, own by default
        and the inherited ones with ``inherited=True``. See :meth:`members`."""
        return self.members(PORT_KINDS, inherited, library)

    def bindings(self, inherited: bool = False, library: bool = False) -> Members:
        """Return binding members (``bindingUsage``), own by default and the
        inherited ones with ``inherited=True``. See :meth:`members`."""
        return self.members(BINDING_KINDS, inherited, library)

    def connections(self, inherited: bool = False, library: bool = False) -> Members:
        """Return connector members: connection, binding, interface, flow and
        allocation definitions and usages, own by default and the inherited
        ones with ``inherited=True``. See :meth:`members`."""
        return self.members(CONNECTION_KINDS, inherited, library)

    def get_attr(self, name: str) -> Optional["Symbol"]:
        """Get attribute by name.
        
        Args:
            name: Attribute name to search for
            
        Returns:
            Symbol if found, None otherwise
        """
        for attr in self.attributes():
            if attr.name == name:
                return attr
        return None
    
    def to_dataframe(self):
        """Convert this symbol's members to a pandas DataFrame.

        One row per member, own then inherited (:meth:`members` with
        ``inherited=True``, standard-library members withheld), with columns:
        name, kind, id, type, multiplicity, value, unit, inherited. ``value`` and ``unit`` are an attribute's default where the
        service resolved a constant one, and are None where it did not.

        Returns:
            pandas.DataFrame with this symbol's members

        Raises:
            ImportError: If pandas is not installed
            MissingCapabilityError: The service does not report
                ``symbol_attributes``, so no default could be reported, or not
                ``symbol_members``, so inherited members cannot be listed
        """
        self._require_resolved_attributes()
        try:
            import pandas as pd
        except ImportError:
            raise ImportError(
                "pandas is required for to_dataframe(). "
                "Install with: pip install pandas"
            )

        rows = list(self.children(inherited=True))

        columns = ['name', 'kind', 'id', 'type', 'multiplicity', 'value', 'unit', 'inherited']
        if not rows:
            return pd.DataFrame(columns=columns)

        # Facts are named, so they are keyed by the attribute id that bears the
        # name: a member of another kind sharing it is not that attribute.
        by_name = {attr.name: attr for attr in self.attribute_facts()}
        attribute_names = {attr.id: attr.name for attr in rows if attr.kind.lower() in ATTRIBUTE_KINDS}
        row_facts = [by_name.get(attribute_names.get(row.id, "")) for row in rows]
        data = {
            'name': [row.name for row in rows],
            'kind': [row.kind for row in rows],
            'id': [row.id for row in rows],
            'type': [_type_text(row, facts) for row, facts in zip(rows, row_facts)],
            'multiplicity': [_multiplicity_text(row) for row in rows],
            'value': [facts.value if facts else None for facts in row_facts],
            'unit': [(facts.unit or None) if facts else None for facts in row_facts],
            'inherited': [row.inherited for row in rows],
        }

        return pd.DataFrame(data, columns=columns)
    
    def __str__(self) -> str:
        """Return human-readable string representation."""
        return f"{self.name} ({self.kind})"
    
    def __repr__(self) -> str:
        """Return developer-friendly string representation."""
        return f"Symbol(id={self.id!r}, kind={self.kind!r})"
    
    def _repr_html_(self) -> str:
        """IPython rich display: formatted definition."""
        from html import escape
        
        html = [
            '<div style="font-family: monospace; padding: 10px; border: 1px solid #ddd; background: #f9f9f9;">',
            f'<h4 style="margin-top: 0;">{escape(self.name)}</h4>',
            f'<p><strong>Kind:</strong> <code>{escape(self.kind)}</code></p>',
            f'<p><strong>ID:</strong> <code>{escape(self.id)}</code></p>',
        ]
        
        # Show metadata if present
        if self.metadata:
            html.extend(['<p><strong>Metadata:</strong></p>', '<ul style="margin: 5px 0;">'])
            for key, value in self.metadata.items():
                html.append(f'<li><code>{escape(key)}</code>: {escape(str(value))}</li>')
            html.append('</ul>')
        
        # Show children summary
        children = self.children()
        if children:
            html.append(f'<p><strong>Children:</strong> {len(children)} symbol(s)</p>')
            
            # Group by kind
            by_kind: Dict[str, List["Symbol"]] = {}
            for child in children:
                by_kind.setdefault(child.kind, []).append(child)
            
            html.append('<ul style="margin: 5px 0;">')
            for kind, items in sorted(by_kind.items()):
                names = ', '.join(escape(item.name) for item in items[:5])
                if len(items) > 5:
                    names += f', ... (+{len(items)-5} more)'
                html.append(f'<li>{escape(kind)}: {names}</li>')
            html.append('</ul>')
        else:
            html.append('<p><em>No children</em></p>')
        
        html.append('</div>')
        return ''.join(html)


def _kind_set(kind: KindFilter) -> Optional[FrozenSet[str]]:
    """Normalize a kind filter to lower-case names, None for every kind."""
    if kind is None:
        return None
    if isinstance(kind, str):
        return frozenset({kind.lower()})
    return frozenset(k.lower() for k in kind)

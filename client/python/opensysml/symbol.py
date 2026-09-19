"""Symbol class for navigating the semantic model."""

from typing import Any, Dict, List, Optional, Protocol, TYPE_CHECKING

from opensysml.capabilities import (
    CAPABILITY_SYMBOL_ATTRIBUTES,
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

# Kind strings emitted by the service (internal/core/symbols: symbolKindNames).
# Matched case-insensitively so older PascalCase producers still work.
ATTRIBUTE_KINDS = frozenset({"attributedef", "attributeusage"})
PART_KINDS = frozenset({"partdef", "partusage"})


class SymbolSource(Protocol):
    """What a symbol needs of the connection it navigates through."""

    def get_symbol(self, model_hash: str, symbol_id: str) -> Any:
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
                 model_hash: str):
        """Initialize Symbol from SymbolInfo protobuf.
        
        Args:
            pb_symbol: SymbolInfo protobuf message
            client: Optional RPC client for lazy loading children
            model_hash: Hash of the model this symbol belongs to
        """
        self._pb = pb_symbol
        self._client = client
        self._model_hash = model_hash
        self._children_cache: Optional[List["Symbol"]] = None
        self._attributes_cache: Optional[List["Symbol"]] = None
    
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
        """Return copy of symbol metadata dictionary."""
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

    def children(self) -> List["Symbol"]:
        """Return all child symbols.
        
        Lazily fetches child symbols using the RPC client if available.
        Caches results after first call.
        Returns empty list if no client is set.
        
        Returns:
            List of child Symbol objects
        """
        if self._children_cache is not None:
            return self._children_cache
        
        if self._client is None:
            self._children_cache = []
            return self._children_cache
        
        result = []
        for child_id in self._pb.child_ids:
            child_info = self._client.get_symbol(self._model_hash, child_id)
            if child_info is not None:
                result.append(Symbol(child_info, self._client, self._model_hash))
        
        self._children_cache = result
        return result
    
    def attributes(self) -> List["Symbol"]:
        """Return the attribute symbols this symbol has, own and inherited.

        Ordered as the service reports the attribute set, so an attribute
        inherited from a supertype is included and a redefinition masks the
        declaration it redefines. An inherited attribute is fetched from the
        supertype that declares it, so it carries its own facts.

        Returns:
            List of attribute Symbol objects
        """
        if self._attributes_cache is not None:
            return self._attributes_cache

        declared = [child for child in self.children() if child.kind.lower() in ATTRIBUTE_KINDS]
        by_name = {child.name: child for child in declared}
        for inherited in self._inherited_attributes(set()):
            by_name.setdefault(inherited.name, inherited)

        reported = [attr.name for attr in self._pb.attributes]
        ordered = [by_name[name] for name in reported if name in by_name]
        # A declared attribute the service did not report — an unnamed one, or an
        # older service — is still this symbol's, so it is kept.
        ordered += [child for child in declared if child.name not in reported]

        self._attributes_cache = ordered
        return ordered

    def _inherited_attributes(self, visited: set) -> List["Symbol"]:
        """Return the attribute symbols this symbol's supertypes declare, nearest first.

        Every generalization edge is followed, typing included: a usage written
        as ``part car : Car`` has the attributes ``Car`` declares, which is what
        the service reports for it.
        """
        if self._client is None or self.id in visited:
            return []
        visited.add(self.id)

        result = []
        for spec in self.specializations:
            if not spec.target_id or spec.target_id in visited:
                continue
            info = self._client.get_symbol(self._model_hash, spec.target_id)
            if info is None:
                continue
            supertype = Symbol(info, self._client, self._model_hash)
            result += [
                child for child in supertype.children()
                if child.kind.lower() in ATTRIBUTE_KINDS
            ]
            result += supertype._inherited_attributes(visited)
        return result
    
    def parts(self) -> List["Symbol"]:
        """Return child symbols that are part definitions or usages.
        
        Returns:
            List of part Symbol objects
        """
        return [child for child in self.children() if child.kind.lower() in PART_KINDS]
    
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

        One row per child, plus one per attribute inherited from a supertype,
        with columns: name, kind, id, type, multiplicity, value, unit,
        inherited. ``value`` and ``unit`` are an attribute's default where the
        service resolved a constant one, and are None where it did not.

        Returns:
            pandas.DataFrame with this symbol's members

        Raises:
            ImportError: If pandas is not installed
            MissingCapabilityError: The service does not report
                ``symbol_attributes``, so no default could be reported
        """
        self._require_resolved_attributes()
        try:
            import pandas as pd
        except ImportError:
            raise ImportError(
                "pandas is required for to_dataframe(). "
                "Install with: pip install pandas"
            )

        attributes = self.attributes()
        rows = list(self.children())
        child_ids = {child.id for child in rows}
        inherited_ids = {attr.id for attr in attributes if attr.id not in child_ids}
        rows += [attr for attr in attributes if attr.id in inherited_ids]

        columns = ['name', 'kind', 'id', 'type', 'multiplicity', 'value', 'unit', 'inherited']
        if not rows:
            return pd.DataFrame(columns=columns)

        # Facts are named, so they are keyed by the attribute id that bears the
        # name: a member of another kind sharing it is not that attribute.
        by_name = {attr.name: attr for attr in self.attribute_facts()}
        attribute_names = {attr.id: attr.name for attr in attributes}
        row_facts = [by_name.get(attribute_names.get(row.id, "")) for row in rows]
        data = {
            'name': [row.name for row in rows],
            'kind': [row.kind for row in rows],
            'id': [row.id for row in rows],
            'type': [_type_text(row, facts) for row, facts in zip(rows, row_facts)],
            'multiplicity': [_multiplicity_text(row) for row in rows],
            'value': [facts.value if facts else None for facts in row_facts],
            'unit': [(facts.unit or None) if facts else None for facts in row_facts],
            'inherited': [row.id in inherited_ids for row in rows],
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
        
        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ddd; background: #f9f9f9;">']
        html.append(f'<h4 style="margin-top: 0;">{escape(self.name)}</h4>')
        html.append(f'<p><strong>Kind:</strong> <code>{escape(self.kind)}</code></p>')
        html.append(f'<p><strong>ID:</strong> <code>{escape(self.id)}</code></p>')
        
        # Show metadata if present
        if self.metadata:
            html.append('<p><strong>Metadata:</strong></p>')
            html.append('<ul style="margin: 5px 0;">')
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

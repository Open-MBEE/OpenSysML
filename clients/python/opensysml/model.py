"""Model class wrapping parsed SysML model."""

import difflib

from opensysml.capabilities import CAPABILITY_QUERY
from opensysml.symbol import Symbol
from opensysml.conversion import FORMAT_SYSML, FORMAT_TURTLE, format_of_path
from opensysml.diagnostic import Diagnostic
from opensysml.edit import Editor
from opensysml.errors import ModelError, SymbolNotFoundError
from opensysml.query import TYPE_PRIMITIVE_CONSTRAINT

#: Severity the service reports for a diagnostic that makes a model unusable.
_SEVERITY_ERROR = "error"

#: Query properties: an element's id, its effective name (what ``Symbol.name``
#: reports) and the id of the element owning it.
_PROPERTY_ID = "@id"
_PROPERTY_NAME = "name"
_PROPERTY_OWNER = "owner"


class Model:
    """Represents a parsed SysML model.
    
    Attributes:
        hash (str): Model content hash (for cache lookups)
        root (Symbol): Root symbol of the model
        diagnostics (list[Diagnostic]): Parse diagnostics (errors/warnings)
    """
    
    def __init__(self, pb_response, client, source_path=None):
        """Initialize Model from protobuf ParseFileResponse.
        
        Args:
            pb_response: sysml_pb2.ParseFileResponse protobuf message
            client: Client instance for symbol navigation
            source_path (str, optional): Path the model was loaded from
        """
        self._pb = pb_response
        self._client = client
        self._source_path = source_path
        self._hash = pb_response.model_hash
        self._root = Symbol(pb_response.root, client, self._hash)
        self._diagnostics = [
            Diagnostic(pb_diag) for pb_diag in pb_response.diagnostics
        ]
    
    @property
    def hash(self):
        """Get model content hash."""
        return self._hash
    
    @property
    def connection(self):
        """Get the connection this model was loaded over."""
        return self._client
    
    @property
    def root(self):
        """Get root symbol."""
        return self._root
    
    @property
    def diagnostics(self):
        """Get list of diagnostics."""
        return self._diagnostics

    @property
    def errors(self):
        """The error-severity diagnostics, which are what makes a model unusable.

        Returns:
            list[Diagnostic]: Diagnostics of severity 'error', in report order
        """
        return [
            d for d in self._diagnostics
            if (d.severity or "").lower() == _SEVERITY_ERROR
        ]

    @property
    def ok(self):
        """Whether the service parsed and analysed this model without errors.

        A model with errors is still returned and still navigable — that is how
        a tool reports every problem at once — but its symbols may be missing or
        unresolved, so lookups on it fail later. Test this, or load with
        ``strict=True``, before treating a model as the model that was written.

        Returns:
            bool: True when no diagnostic has error severity
        """
        return not self.errors

    def raise_for_errors(self):
        """Raise :class:`~opensysml.errors.ModelError` unless :attr:`ok`.

        Returns:
            Model: self, so a call can be chained onto a load

        Raises:
            ModelError: If the model has error diagnostics. It carries them as
                ``diagnostics`` and this model as ``model``.
        """
        errors = self.errors
        if not errors:
            return self
        where = self._source_path or "the model"
        summary = "; ".join(str(d) for d in errors[:3])
        if len(errors) > 3:
            summary += f"; ... and {len(errors) - 3} more"
        raise ModelError(
            f"{where} has {len(errors)} error(s): {summary}",
            diagnostics=errors,
            model=self,
        )
    
    @property
    def source_path(self):
        """Path this model was loaded from, or None if it was loaded inline."""
        return self._source_path

    def convert(self, to_format, tolerate_syntax_errors=False):
        """Write this model out in one of the formats OpenSysML writes.

        Converts the source this model was parsed from, not the file as it
        stands now, so what is written is the model that was inspected: notation
        keeps its comments and lexemes, re-indented, while Turtle carries what
        the model declares. See ``docs/reference/rdf-mapping.md``.

        The service holds that source in its model cache, which is bounded, so a
        model loaded long ago and many models back may have been evicted; load it
        again, or convert its path through :meth:`Connection.convert`.

        Args:
            to_format (str): 'sysml', 'kerml', 'text', 'ttl', 'turtle' or 'rdf'
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it

        Returns:
            Conversion: The converted model; ``str()`` of it is the text

        Warns:
            ExperimentalFeatureWarning: If the format is RDF, whose mapping is
                experimental — see ``docs/reference/rdf-mapping.md``

        Raises:
            ConversionError: If the model could not be written in that format
            MissingCapabilityError: If the service cannot convert
            grpc.RpcError: If the service no longer holds this model
        """
        return self.connection.convert(
            to_format,
            model_hash=self._hash,
            # A model came from ParseFile, which reads notation and nothing else.
            from_format=FORMAT_SYSML,
            tolerate_syntax_errors=tolerate_syntax_errors,
        )

    def to_sysml(self, tolerate_syntax_errors=False):
        """Write this model out as SysML textual notation.

        Returns:
            Conversion: The notation; ``str()`` of it is the text
        """
        return self.convert(FORMAT_SYSML, tolerate_syntax_errors=tolerate_syntax_errors)

    def to_turtle(self):
        """Write this model out as an RDF graph in Turtle syntax.

        The RDF mapping is experimental: it covers model structure and the
        behavior its bodies state, refuses what it cannot write back, and warns
        with :class:`ExperimentalFeatureWarning`.

        Returns:
            Conversion: The Turtle; ``str()`` of it is the text
        """
        return self.convert(FORMAT_TURTLE)

    def save(self, path, to_format=None, tolerate_syntax_errors=False):
        """Write this model to ``path``, in the format its extension names.

        Args:
            path (str): File to write, created or truncated
            to_format (str, optional): Format to write, overriding the extension
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it

        Returns:
            Conversion: What was written

        Raises:
            ValueError: If no to_format was given and the extension names none
            ConversionError: If the model could not be written in that format
        """
        conversion = self.convert(
            to_format or format_of_path(path),
            tolerate_syntax_errors=tolerate_syntax_errors,
        )
        conversion.write(path)
        return conversion

    def edit(self):
        """Start an edit of this model, to be applied in one call.

        The editor collects operations naming elements by the ids this model
        reports, and :meth:`Editor.apply` has the service perform them on the
        source it parsed: the edited spans are replaced and every other byte,
        comments and layout included, comes back unchanged.

        The service holds that source in its bounded model cache, so a model
        loaded long ago may have been evicted; load it again to edit it.

        Returns:
            Editor: The editor, empty. Applying an empty one is an error.

        Example:
            >>> edit = model.edit()
            >>> edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
            >>> edit.apply().save("spacecraft.sysml")
            'spacecraft.sysml'
        """
        return Editor(self._hash, self.connection)

    def query(self, payload=None, scope=None, select=None, where=None):
        """Run a SysML v2 API & Services Query over this model.

        Takes the standard's ``Query`` JSON, so a payload written for the
        standard's API works verbatim, or the same thing as keywords. The query
        model has no graph traversal: "everything under this element" is a
        ``scope``, not a constraint. See ``docs/reference/api.md``.

        Args:
            payload (dict, optional): The standard's ``Query`` object
            scope (list, optional): Elements to consider, by qualified name;
                empty considers the whole loaded model
            select (list, optional): Properties to report; empty reports every one
            where (dict, optional): Constraint to filter by

        Returns:
            list[QueryElement]: The elements selected, in declaration order

        Raises:
            QueryError: If the query is not one the standard's model describes
            MissingCapabilityError: If the service cannot query
            InvalidRequestError: If a property or scope is unknown to the service
            ModelNotFoundError: If the service no longer holds this model

        Example:
            >>> model.query({"@type": "Query", "where": {
            ...     "@type": "PrimitiveConstraint",
            ...     "operator": "=", "property": "@type", "value": ["PartUsage"]}})
            [Demo::vehicle (PartUsage)]
        """
        return self.connection.query(
            self._hash, payload, scope=scope, select=select, where=where,
        )

    def run_document_query(self, query_id, bindings=None):
        """Run one of this model's named document queries.

        The query is the model's own — a calc def specializing
        ``DocumentQueries::Query`` — not the standard's Query object
        :meth:`query` evaluates. See :mod:`opensysml.document`.

        Args:
            query_id (str): Qualified name of the document query
            bindings (Mapping, optional): Parameter name to a value or list of
                values; an :class:`~opensysml.document.ElementRef` binds a
                model element

        Returns:
            DocumentQueryResult: Projected columns and typed rows, in the
            engine's deterministic order

        Raises:
            MissingCapabilityError: If the service cannot run document queries
            InvalidRequestError: If the query is not one, or a binding is wrong
            SymbolNotFoundError: If this model does not declare the query
            ModelNotFoundError: If the service no longer holds this model

        Example:
            >>> from opensysml.document import ElementRef
            >>> result = model.run_document_query(
            ...     "Observatory::SubsystemTable",
            ...     bindings={"root": ElementRef("Observatory::telescope")})
            >>> result.columns
            ('name', 'mass')
        """
        return self.connection.run_document_query(
            self._hash, query_id, bindings=bindings,
        )

    def render_document(self, document_id):
        """Render one of this model's named documents to Markdown.

        The document is a part def specializing ``DocumentQueries::Document``,
        whose queries are bound in the model.

        Args:
            document_id (str): Qualified name of the document

        Returns:
            str: The rendered Markdown

        Raises:
            MissingCapabilityError: If the service cannot render documents
            InvalidRequestError: If the symbol named is not a document
            SymbolNotFoundError: If this model does not declare the document
            ModelNotFoundError: If the service no longer holds this model

        Example:
            >>> markdown = model.render_document("Observatory::MassReport")
            >>> markdown.splitlines()[0]
            '# Telescope Mass Report'
        """
        return self.connection.render_document(self._hash, document_id)

    def find(self, name):
        """Find symbol by short name or fully-qualified name.

        A symbol's own ``id`` is accepted as well as its short name, so the
        identifier a symbol reports can be round-tripped back into ``find``.
        Several symbols may share a short name; the outermost wins, and among
        those the one declared first. A name declared in the model wins over a
        library symbol whose id it is, as ``Base`` is both a library package and
        a common name. Lookups are answered from the service's index, in one or
        two round trips whatever the size of the model.

        Args:
            name (str): Short name ("Vehicle") or FQN ("Demo::Vehicle")

        Returns:
            Symbol or None: The matching symbol, or None if not found. Use
            ``model[name]`` where a missing symbol is a failure, so it is
            reported as one instead of as an AttributeError on None.
        """
        if self.root.name == name or self.root.id == name:
            return self.root
        if "::" in name:
            return self._symbol_by_id(name) or self._symbol_named(name)
        return self._symbol_named(name) or self._symbol_by_id(name)

    def get(self, fqn):
        """Get symbol by fully-qualified name (e.g., "Demo::Vehicle").

        Args:
            fqn (str): Fully-qualified name to look up

        Returns:
            Symbol or None: Matching symbol, or None if not found
        """
        if self.root.id == fqn:
            return self.root
        return self._symbol_by_id(fqn)

    def _symbol_by_id(self, fqn):
        """The symbol whose id is ``fqn``, fetched in one call, or None.

        The service resolves a qualified name the way the notation does, through
        imports and aliases, so it may answer with a symbol of another id. Only
        the symbol that carries this id is what an id lookup names.
        """
        info = self._client.get_symbol(self._hash, fqn)
        if info is None or info.id != fqn:
            return None
        return Symbol(info, self._client, self._hash)

    def _symbol_named(self, name):
        """The outermost, first-declared symbol whose short name is ``name``.

        A service that can query answers which elements carry a name in one
        call. Without that, or when an erroring model may hold a name the query
        cannot see (one taken from an unresolved redefinition), the tree is
        walked symbol by symbol.
        """
        if not self._client.server_info().has(CAPABILITY_QUERY):
            return self._walk_to(name)
        named = self._query(_PROPERTY_NAME, name, select=[_PROPERTY_OWNER])
        for fqn in self._outermost_first(named):
            symbol = self._symbol_by_id(fqn)
            if symbol is not None:
                return symbol
        return None if self.ok else self._walk_to(name)

    def _query(self, prop, value, select):
        """The elements whose ``prop`` is ``value`` (any of them, given a list)."""
        return self._client.query(
            self._hash,
            select=select,
            where={
                "@type": TYPE_PRIMITIVE_CONSTRAINT,
                "operator": "=",
                "property": prop,
                "value": value,
            },
        )

    def _outermost_first(self, elements):
        """The ids of ``elements`` by nesting depth, declaration order among equals.

        Depth is counted up the owner chain rather than from the id's ``::``
        segments, which a quoted name may contain itself. Each hop up costs one
        call, and only when the name is shared.
        """
        ids = [element.id for element in elements]
        if len(ids) < 2:
            return ids
        owner = {self.root.id: ""}
        owner.update((element.id, element.get(_PROPERTY_OWNER, "")) for element in elements)
        unknown = {fqn for fqn in owner.values() if fqn and fqn not in owner}
        while unknown:
            for element in self._query(_PROPERTY_ID, sorted(unknown), select=[_PROPERTY_OWNER]):
                owner[element.id] = element.get(_PROPERTY_OWNER, "")
            for fqn in unknown:
                owner.setdefault(fqn, "")
            unknown = {fqn for fqn in owner.values() if fqn and fqn not in owner}

        def depth(fqn):
            hops = 0
            while owner.get(fqn):
                fqn = owner[fqn]
                hops += 1
            return hops

        return sorted(ids, key=depth)

    def _walk_to(self, name):
        """The first symbol named ``name`` in breadth-first order, or None."""
        return next((s for s in self._walk() if s.name == name), None)

    def _walk(self):
        """Every symbol below the root, breadth-first, one call per symbol."""
        queue = [self.root]
        while queue:
            current = queue.pop(0)
            for child in current.children():
                yield child
                queue.append(child)

    def eval(self, expression, context_symbol_id=None, subject=None):
        """Evaluate a SysML expression against this model.

        Args:
            expression (str): SysML expression (e.g., "1 + 1")
            context_symbol_id (str, optional): FQN of the symbol whose scope the
                expression's names resolve in
            subject (str, optional): FQN of a part/usage to instantiate and
                evaluate against, as ``%eval`` does after ``%instantiate``, so a
                feature reads that object's value rather than the declared
                default. Without a context the subject also names the scope.

        Returns:
            The evaluated value, as a Python value

        Raises:
            ExecutionError: If the expression could not be evaluated, or the
                subject is unknown or could not be instantiated
            ModelNotFoundError: If the service no longer holds this model
            UnsupportedValueError: If the result cannot be represented on the wire

        Example:
            >>> model.eval("1 + 1")
            2
            >>> model.eval("mass", subject="Demo::car")
            1600.0
        """
        return self._client.eval(
            expression,
            self._hash,
            context_symbol_id=context_symbol_id,
            subject_symbol_id=subject,
        )

    def instantiate(self, symbol_id):
        """Build an object of one of this model's parts or usages.

        Args:
            symbol_id (str): FQN of the part/usage to instantiate

        Returns:
            Instance: The object built, with its feature values and nested objects

        Raises:
            ExecutionError: If the element could not be instantiated
            ModelNotFoundError: If the service no longer holds this model

        Example:
            >>> model.instantiate("Demo::Vehicle").mass
            1500.0
        """
        return self._client.instantiate(symbol_id, self._hash)

    def execute_action(self, action_symbol_id, inputs=None, schedule=None):
        """Execute one of this model's actions.

        Args:
            action_symbol_id (str): FQN of the action definition or usage
            inputs (dict, optional): Input parameter name → Python value
            schedule (str, optional): Scheduling policy the run resolves its
                choice points under — ``"declared"``, ``"reverse"`` (the
                default) or ``"seed:<n>"``; ``"explore"`` belongs to
                :meth:`explore_action`

        Returns:
            dict: Output parameter name → value; an output the wire format
                cannot represent is reported as an UnsupportedValueError in its
                place, so one such output does not discard the rest

        Raises:
            ValueError: If the schedule explores
            ExecutionError: If the action could not be executed
            ModelNotFoundError: If the service no longer holds this model
            MissingCapabilityError: If a schedule is given and the service
                predates ``schedule``
            InvalidRequestError: If the schedule names no policy
        """
        return self._client.execute_action(
            action_symbol_id, self._hash, inputs=inputs, schedule=schedule
        )

    def explore_action(self, action_symbol_id, inputs=None, schedule="explore"):
        """Run one of this model's actions once per valid order of its choice points.

        Args:
            action_symbol_id (str): FQN of the action definition or usage
            inputs (dict, optional): Input parameter name → Python value
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``, bounding the runs made and
                the choice points one run resolves

        Returns:
            Exploration: Every distinct outcome reached, each with the number
                of orders reaching it and the choices of one, and how the
                exploration ended

        Raises:
            ValueError: If the schedule does not explore
            ExecutionError: If the action could not be explored at all
            ModelNotFoundError: If the service no longer holds this model
            MissingCapabilityError: If the service predates ``schedule_explore``
            InvalidRequestError: If the schedule's options are malformed
        """
        return self._client.explore_action(
            action_symbol_id, self._hash, inputs=inputs, schedule=schedule
        )

    def execute_state(self, state_machine_symbol_id, events=None, schedule=None):
        """Execute one of this model's state machines.

        Args:
            state_machine_symbol_id (str): FQN of the state machine definition
                or usage
            events (list, optional): Event names to process, in order
            schedule (str, optional): Scheduling policy the run resolves its
                choice points under, as for :meth:`execute_action`;
                ``"explore"`` belongs to :meth:`explore_state`

        Returns:
            dict: {'states_visited': [...], 'final_context': {...}, 'final_time': float};
                a context value the wire format cannot represent is reported as
                an UnsupportedValueError in its place; ``final_time`` is the
                run's simulation clock when it ended, in seconds

        Raises:
            ValueError: If the schedule explores
            ExecutionError: If the state machine could not be executed
            ModelNotFoundError: If the service no longer holds this model
            MissingCapabilityError: If a schedule is given and the service
                predates ``schedule``
            InvalidRequestError: If the schedule names no policy
        """
        return self._client.execute_state(
            state_machine_symbol_id, self._hash, events=events, schedule=schedule
        )

    def explore_state(self, state_machine_symbol_id, events=None, schedule="explore"):
        """Run one of this model's state machines once per valid order of its choice points.

        Args:
            state_machine_symbol_id (str): FQN of the state machine definition
                or usage
            events (list, optional): Event names to process, in order
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``

        Returns:
            Exploration: Every distinct outcome reached — the state rested in,
                the states entered and the values held — and how the
                exploration ended

        Raises:
            ValueError: If the schedule does not explore
            ExecutionError: If the state machine could not be explored at all
            ModelNotFoundError: If the service no longer holds this model
            MissingCapabilityError: If the service predates ``schedule_explore``
            InvalidRequestError: If the schedule's options are malformed
        """
        return self._client.explore_state(
            state_machine_symbol_id, self._hash, events=events, schedule=schedule
        )

    def verify_constraint(self, symbol_id, subject=None, engine=None):
        """Ask whether one of this model's constraints holds.

        Args:
            symbol_id (str): FQN of the constraint definition or usage
            subject (str, optional): FQN of a part/usage to instantiate and
                evaluate against, so the verdict is about concrete values
            engine (str, optional): The engine to ask: ``"auto"`` (the
                default), ``"all"``, or one by name, as
                :meth:`~opensysml.Connection.verify_constraint` takes it

        Returns:
            Verdict: The answer; false is the model's answer, not an exception

        Raises:
            WrongKindError: If symbol_id names an element that is not a
                constraint
            ExecutionError: If the request could not be answered at all
        """
        return self._client.verify_constraint(
            symbol_id, self._hash, subject_symbol_id=subject, engine=engine
        )

    def verify_requirement(self, symbol_id, subject=None, engine=None):
        """Ask whether one of this model's requirements is satisfied.

        Args:
            symbol_id (str): FQN of the requirement definition or usage
            subject (str, optional): FQN of a part/usage to instantiate and
                evaluate against
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            Verdict: The answer

        Raises:
            WrongKindError: If symbol_id names an element that is not a
                requirement
            ExecutionError: If the request could not be answered at all
        """
        return self._client.verify_requirement(
            symbol_id, self._hash, subject_symbol_id=subject, engine=engine
        )

    def verify_satisfaction(self, symbol_id=None, engine=None):
        """Ask whether this model's satisfaction assertions hold.

        This is the scriptable form of "does this model satisfy its
        requirements?": every ``assert satisfy ... by ...`` the model states,
        each evaluated against an object of its subject.

        Args:
            symbol_id (str, optional): FQN limiting evaluation to the assertions
                stated within that element, or to that element itself when it is
                a named satisfaction assertion
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            list[Verdict]: One verdict per assertion, in declaration order. An
                element stating none gives an empty list.

        Raises:
            WrongKindError: If symbol_id names an element that can state no
                satisfaction assertion
            ExecutionError: If the request could not be answered at all
        """
        return self._client.verify_satisfaction(self._hash, symbol_id=symbol_id, engine=engine)

    def satisfied(self, symbol_id=None):
        """Whether every satisfaction assertion evaluated holds.

        A model stating no assertion is trivially satisfied, so read this
        together with :meth:`verify_satisfaction` where that matters. An
        assertion that could not be evaluated is not a holding one.

        Args:
            symbol_id (str, optional): FQN limiting evaluation, as in
                :meth:`verify_satisfaction`

        Returns:
            bool: True when no assertion fails
        """
        return all(v.holds for v in self.verify_satisfaction(symbol_id))

    def calc(self, symbol_id, arguments=None, engine=None):
        """Invoke one of this model's calculations.

        Args:
            symbol_id (str): FQN of the calc definition or usage
            arguments (list, optional): Positional arguments, as Python values
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            CalcResult: The value returned, or the outputs a calc usage computed

        Raises:
            WrongKindError: If symbol_id names an element that is not a calc
            ExecutionError: If the calculation could not be evaluated
        """
        return self._client.calc(symbol_id, self._hash, arguments=arguments, engine=engine)

    def run_analysis(self, symbol_id, subject=None, arguments=None,
                     named_arguments=None, schedule=None, engine=None):
        """Run one of this model's analysis cases.

        Args:
            symbol_id (str): FQN of the analysis case definition or usage
            subject (str, optional): FQN of a part/usage to instantiate and run
                the case on; a usage binding its own subject needs none
            arguments (list, optional): Positional arguments for the case's
                ``in`` parameters, as Python values
            named_arguments (dict, optional): Arguments by parameter name
            schedule (str, optional): Scheduling policy the actions the case
                performs resolve their choice points under, as for
                :meth:`execute_action`; ``"explore"`` belongs to
                :meth:`explore_analysis`
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`; ``"explore"`` belongs to
                :meth:`explore_analysis`

        Returns:
            AnalysisResult: The outputs the case computed and the verdict of
                its objective and assertions

        Raises:
            ValueError: If the schedule or the engine explores
            WrongKindError: If symbol_id names an element that is not an
                analysis case
            ExecutionError: If the case could not run
            InvalidRequestError: If the schedule names no policy, or the
                engine names none the service registers
        """
        return self._client.run_analysis(
            symbol_id, self._hash, subject=subject, arguments=arguments,
            named_arguments=named_arguments, schedule=schedule, engine=engine,
        )

    def explore_analysis(self, symbol_id, subject=None, arguments=None,
                         named_arguments=None, schedule="explore"):
        """Run one of this model's analysis cases once per valid order of its actions' choice points.

        Args:
            symbol_id (str): FQN of the analysis case definition or usage
            subject (str, optional): FQN of a part/usage to instantiate and run
                the case on
            arguments (list, optional): Positional arguments, as Python values
            named_arguments (dict, optional): Arguments by parameter name
            schedule (str, optional): ``"explore"`` or
                ``"explore:runs=<n>,depth=<d>"``

        Returns:
            Exploration: Every distinct outcome reached — the case's outputs and
                its objective and assertion verdicts — and how the exploration
                ended

        Raises:
            ValueError: If the schedule does not explore
            WrongKindError: If symbol_id names an element that is not an
                analysis case
            ExecutionError: If the case could not be explored at all
            MissingCapabilityError: If the service predates ``schedule_explore``
            InvalidRequestError: If the schedule's options are malformed
        """
        return self._client.explore_analysis(
            symbol_id, self._hash, subject=subject, arguments=arguments,
            named_arguments=named_arguments, schedule=schedule,
        )

    def run_sweep(self, symbol_id, ranges, subject=None, arguments=None,
                  named_arguments=None, samples=0, seed=0, engine=None):
        """Run one of this model's analysis cases or calcs once per swept row.

        Args:
            symbol_id (str): FQN of the analysis case or calc
            ranges (dict): Range per swept parameter, ``{"speed": (0, 10, 2)}``
            subject (str, optional): FQN of a part/usage to instantiate and run
                an analysis case on
            arguments (list, optional): Positional arguments every row binds
            named_arguments (dict, optional): Arguments by name every row binds
            samples (int, optional): Rows to draw rather than step through
            seed (int, optional): Seed the draws are taken from
            engine (str, optional): The engine to ask, as for
                :meth:`verify_constraint`

        Returns:
            SweepTable: One row per run, in the order the runs were made

        Raises:
            WrongKindError: If symbol_id names neither an analysis case nor a calc
            ExecutionError: If no run followed from the request
        """
        return self._client.run_sweep(
            symbol_id, self._hash, ranges, subject=subject,
            arguments=arguments, named_arguments=named_arguments,
            samples=samples, seed=seed, engine=engine,
        )

    def __getitem__(self, name):
        """Look a symbol up by short name or FQN, raising when there is none.

        The raising counterpart of :meth:`find`: ``model["Vehicle"].attributes()``
        names the symbol that is missing, where ``find`` would return None and
        fail as an AttributeError on it one call later.

        Args:
            name (str): Short name ("Vehicle") or FQN ("Demo::Vehicle")

        Returns:
            Symbol: The matching symbol

        Raises:
            SymbolNotFoundError: If the model declares no such symbol. Also a
                KeyError, and it names the closest declared names.
        """
        symbol = self.find(name)
        if symbol is None:
            raise SymbolNotFoundError(name, self._near_names(name))
        return symbol

    def __contains__(self, name):
        """Whether a short name or FQN names a symbol in this model."""
        return self.find(name) is not None

    def _near_names(self, name):
        """Declared names close enough to ``name`` to be what was meant.

        Both short names and FQNs are candidates, since either is accepted by a
        lookup and either may have been mistyped.
        """
        if self._client.server_info().has(CAPABILITY_QUERY):
            declared = [
                (element.id, element.get(_PROPERTY_NAME, ""))
                for element in self._client.query(self._hash, select=[_PROPERTY_NAME])
            ]
        else:
            declared = [(child.id, child.name) for child in self._walk()]
        candidates = []
        for fqn, short_name in declared:
            if short_name:
                candidates.append(short_name)
            if fqn and fqn != short_name:
                candidates.append(fqn)
        return difflib.get_close_matches(name, candidates, n=3)

    def __str__(self):
        """String representation: 'Model: name (kind)'."""
        return f"Model: {self.root.name} ({self.root.kind})"
    
    def __repr__(self):
        """Detailed representation."""
        diag_count = len(self.diagnostics)
        return (
            f"Model(hash={self.hash!r}, root={self.root.name!r}, "
            f"diagnostics={diag_count})"
        )
    
    def _repr_html_(self):
        """IPython rich display: tree view + diagnostic summary."""
        from html import escape
        
        # Count diagnostics by severity
        errors = sum(1 for d in self.diagnostics if d.severity == 'error')
        warnings = sum(1 for d in self.diagnostics if d.severity == 'warning')
        
        # Build HTML
        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ccc;">']
        html.append(f'<h3>Model: {escape(self.root.name)}</h3>')
        html.append(f'<p><strong>Hash:</strong> <code>{escape(self.hash[:12])}...</code></p>')
        html.append(f'<p><strong>Root Kind:</strong> {escape(self.root.kind)}</p>')
        
        # Diagnostic summary
        if self.diagnostics:
            html.append('<p><strong>Diagnostics:</strong>')
            if errors:
                html.append(f' <span style="color: red;">{errors} error(s)</span>')
            if warnings:
                html.append(f' <span style="color: orange;">{warnings} warning(s)</span>')
            html.append('</p>')
            
            # Show first 5 diagnostics
            html.append('<ul style="margin-top: 5px;">')
            for diag in self.diagnostics[:5]:
                color = 'red' if diag.severity == 'error' else 'orange'
                html.append(f'<li style="color: {color};">{escape(diag.message)} (line {diag.span.start_line})</li>')
            if len(self.diagnostics) > 5:
                html.append(f'<li>... and {len(self.diagnostics) - 5} more</li>')
            html.append('</ul>')
        else:
            html.append('<p style="color: green;"><strong>✓</strong> No diagnostics</p>')
        
        html.append('</div>')
        return ''.join(html)

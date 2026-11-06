"""Instance class wrapping runtime-materialized objects."""

from opensysml.errors import FeatureValueError
from opensysml.values import UNSET, InstanceRef, feature_value_to_python, value_to_python


class Instance:
    """Represents a runtime instance of a part/usage.

    Feature values are exposed as Python values: numbers, strings, booleans,
    lists and nested Instance objects. The protobuf messages remain reachable
    through ``get_feature()`` and ``raw_features``.

    A feature is also an attribute (``inst.engine``), but a real attribute
    always wins: a feature named like one of the class's own members —
    ``id``, ``type_symbol_id``, ``features``, ``owner``, ``feature``,
    ``feature_symbol_id``, ``index``, ``path``, ``reached_by``, ``paths``,
    ``get`` or any other method — is read by name instead, ``inst["path"]``
    or ``inst.get("path")``.

    Attributes:
        id (int): Unique instance identifier
        type_symbol_id (str): FQN of the def/usage this instantiates
        features (dict): Feature name → Python value
    """

    def __init__(self, pb_instance, graph=None, _wrappers=None, _connection=None, _model_hash=None):
        """Initialize from protobuf Instance message.

        Args:
            pb_instance: sysml_pb2.Instance protobuf message
            graph: optional {instance_id: sysml_pb2.Instance} of reachable
                instances, as returned by InstantiateResponse.instances
            _wrappers: internal cache shared by all instances of one graph
            _connection: the connection that instantiated the graph, which
                ``paths()`` asks for every path to the object
            _model_hash: the model the graph was instantiated in
        """
        self._pb = pb_instance
        self._connection = _connection
        self._model_hash = _model_hash
        # The graph is shared by every instance of one response, never mutated.
        if not graph:
            self._graph = {pb_instance.id: pb_instance}
        elif pb_instance.id in graph:
            self._graph = graph
        else:
            self._graph = {**graph, pb_instance.id: pb_instance}
        self._wrappers = _wrappers if _wrappers is not None else {}
        self._wrappers[pb_instance.id] = self

    @property
    def id(self):
        """Get instance ID."""
        return self._pb.id

    @property
    def type_symbol_id(self):
        """Get type symbol FQN."""
        return self._pb.type_symbol_id

    @property
    def _values(self):
        """The protobuf feature values."""
        return self._pb.feature_values

    @property
    def features(self):
        """Get all feature values as {feature_name: Python value}.

        A feature whose value failed to evaluate or was never materialized is
        reported as a FeatureValueError object instead of raising, so the whole
        instance stays inspectable; attribute or item access on it raises.
        """
        result = {}
        for name, pb_value in self._values.items():
            try:
                result[name] = feature_value_to_python(name, pb_value, self._resolve_instance)
            except FeatureValueError as exc:
                result[name] = exc
        return result

    @property
    def raw_features(self):
        """Get all feature values as {feature_name: sysml_pb2.FeatureValue}."""
        return dict(self._values)

    def get_feature(self, feature_name):
        """Get the raw protobuf feature value for a feature.

        Args:
            feature_name (str): Name of feature

        Returns:
            sysml_pb2.FeatureValue or None if not found
        """
        return self._values.get(feature_name)

    def get(self, feature_name, default=None):
        """Get a feature's Python value, or default if the feature does not exist.

        Raises:
            FeatureValueError: If the feature exists but failed to evaluate.
        """
        pb_value = self._values.get(feature_name)
        if pb_value is None:
            return default
        return feature_value_to_python(feature_name, pb_value, self._resolve_instance)

    def _resolve_instance(self, instance_id):
        """Resolve an instance id to an Instance, or an InstanceRef if unreachable."""
        wrapper = self._wrappers.get(instance_id)
        if wrapper is not None:
            return wrapper
        pb_child = self._graph.get(instance_id)
        if pb_child is None:
            return InstanceRef(instance_id)
        return Instance(pb_child, self._graph, self._wrappers, self._connection, self._model_hash)

    @property
    def reached_by(self):
        """The ``InstanceReach`` message saying how the traversal that produced
        this graph reached the object, or ``None`` for the root (and from a
        service without the ``instance_paths`` capability)."""
        if not self._pb.HasField("reached_by"):
            return None
        return self._pb.reached_by

    @property
    def owner(self):
        """The Instance holding this one along the path the traversal reached
        it by, or ``None`` for the root.

        One object is often a value of several features (a port and the
        connector end referencing it are one object); this is the holder the
        traversal came through, and ``paths()`` lists every path to it.
        """
        reach = self.reached_by
        if reach is None:
            return None
        return self._resolve_instance(reach.instance_id)

    @property
    def feature(self):
        """The name of the owner's feature this object is a value of, or
        ``None`` for the root. ``feature_symbol_id`` is its qualified name."""
        reach = self.reached_by
        return reach.feature_name if reach is not None else None

    @property
    def feature_symbol_id(self):
        """The qualified name of the feature ``feature`` names, as declared."""
        reach = self.reached_by
        return reach.feature_symbol_id if reach is not None else None

    @property
    def index(self):
        """The position among the values of a multi-valued feature, counted
        from 1 as an object path indexes them (``wheels[2]``); 0 for a
        single-valued feature, ``None`` for the root."""
        reach = self.reached_by
        return reach.index if reach is not None else None

    @property
    def path(self):
        """The path from the root the traversal reached this object by: the
        feature names, indexed within a multi-valued feature, joined by dots
        (``powertrain.engine.fuelIn``, ``wheels[2].hub``); ``""`` for the root.
        """
        reach = self.reached_by
        return reach.path if reach is not None else ""

    def paths(self):
        """Every root-relative path that reaches this object, aliases through
        connector ends and reference usages included, as InstancePath objects
        in traversal order: the roots the service holds in id order and, from
        each, depth-first over the features of an object in name order. The
        list is an InstancePaths, whose ``truncated`` says whether the service
        bounded its search short of every path.

        Raises:
            RuntimeError: the instance was not produced by a connection
            MissingCapabilityError: the service predates ``instance_paths``
        """
        if self._connection is None or self._model_hash is None:
            raise RuntimeError("paths() needs the connection that instantiated the object")
        return self._connection.instance_paths(self.id, self._model_hash)

    def __getattr__(self, name):
        """Expose feature values as attributes. Real attributes always win."""
        if name.startswith('_'):
            raise AttributeError(name)
        values = self._values if self.__dict__.get('_pb') is not None else {}
        if name not in values:
            raise AttributeError(
                f"{type(self).__name__!r} object has no attribute or feature {name!r}"
            )
        return feature_value_to_python(name, values[name], self._resolve_instance)

    def __getitem__(self, feature_name):
        """Expose feature values by name; raises KeyError for an unknown feature."""
        pb_value = self._values.get(feature_name)
        if pb_value is None:
            raise KeyError(feature_name)
        return feature_value_to_python(feature_name, pb_value, self._resolve_instance)

    def __contains__(self, feature_name):
        return feature_name in self._values

    def __dir__(self):
        return sorted(set(super().__dir__()) | set(self._values))

    def __str__(self):
        return f"Instance(id={self.id}, type={self.type_symbol_id})"

    def __repr__(self):
        return f"Instance(id={self.id}, type={self.type_symbol_id!r}, features={len(self._values)})"

    def _repr_html_(self):
        """IPython rich display: feature-value table."""
        from html import escape

        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ddd;">']
        html.append(f'<h4 style="margin-top: 0;">Instance #{self.id}</h4>')
        html.append(f'<p><strong>Type:</strong> <code>{escape(self.type_symbol_id)}</code></p>')

        raw_features = self.raw_features
        if raw_features:
            html.extend([
                '<table style="border-collapse: collapse; width: 100%; margin-top: 10px;">',
                '<thead><tr style="background: #f0f0f0;">',
                '<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Feature</th>',
                '<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Value</th>',
                '<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Materialized</th>',
                '</tr></thead><tbody>',
            ])

            for feature_name, pb_value in raw_features.items():
                html.append('<tr>')
                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;"><code>{escape(feature_name)}</code></td>')

                # Format value
                if pb_value.error:
                    value_str = f'<em>error: {escape(pb_value.error)}</em>'
                elif pb_value.HasField('value'):
                    value_str = self._format_value(pb_value.value)
                elif pb_value.values:
                    value_str = '[' + ', '.join(self._format_value(v) for v in pb_value.values) + ']'
                else:
                    value_str = '<em>None</em>'

                html.extend([
                    f'<td style="border: 1px solid #ccc; padding: 5px;">{value_str}</td>',
                    f'<td style="border: 1px solid #ccc; padding: 5px;">{"✓" if pb_value.materialized else ""}</td>',
                    '</tr>',
                ])

            html.append('</tbody></table>')
        else:
            html.append('<p><em>No feature values</em></p>')

        html.append('</div>')
        return ''.join(html)

    def _format_value(self, pb_value):
        """Format a protobuf Value for HTML display."""
        from html import escape

        kind = pb_value.WhichOneof('kind')
        if kind == 'string_value':
            return f'"{escape(pb_value.string_value)}"'
        if kind == 'instance_id':
            return f'Instance#{pb_value.instance_id}'
        if kind == 'null':
            return escape(pb_value.null) if pb_value.null else 'null'
        if kind == 'unset':
            return f'<em>{escape(str(UNSET))}</em>'
        try:
            return escape(str(value_to_python(pb_value)))
        except Exception:  # pragma: no cover - display must not fail
            return '<em>unknown</em>'


class InstancePaths(list):
    """The root-relative paths to one object, a list of InstancePath in
    traversal order, and whether the service cut the list short.

    Attributes:
        truncated (bool): the service bounded its search — more paths than it
            lists, or deeper ones, may reach the object
    """

    def __init__(self, paths=(), truncated=False):
        super().__init__(paths)
        self.truncated = truncated

    def __repr__(self):
        return "InstancePaths(%s%s)" % (list.__repr__(self), ", truncated=True" if self.truncated else "")


class InstancePath:
    """One root-relative path to an object, as InstancePaths answers it.

    Attributes:
        root_id (int): the root object the path starts from
        root_symbol_id (str): the qualified name the root was instantiated as,
            ``""`` when the name no longer binds it and ``#<id>`` addresses it
        path (str): the steps as an object path (``powertrain.tank.fuelOut``),
            ``""`` when the object is the root
        steps (list): PathStep per feature walked
    """

    def __init__(self, pb_path):
        self._pb = pb_path
        self.root_id = pb_path.root.instance_id
        self.root_symbol_id = pb_path.root.symbol_id
        self.path = pb_path.path
        self.steps = [PathStep(step) for step in pb_path.steps]

    def __str__(self):
        root = self.root_symbol_id or "#%d" % self.root_id
        return root + "." + self.path if self.path else root

    def __repr__(self):
        return "InstancePath(%r)" % str(self)

    def __eq__(self, other):
        return isinstance(other, InstancePath) and self._pb == other._pb

    def __hash__(self):
        return hash(str(self))


class PathStep:
    """One feature walked along an InstancePath.

    Attributes:
        feature_name (str): the holder's feature
        feature_symbol_id (str): the qualified name of the feature as declared
        index (int): position within a multi-valued feature from 1; 0 for a
            single-valued one
        instance_id (int): the object the step reaches
    """

    def __init__(self, pb_step):
        self.feature_name = pb_step.feature_name
        self.feature_symbol_id = pb_step.feature_symbol_id
        self.index = pb_step.index
        self.instance_id = pb_step.instance_id

    def __repr__(self):
        suffix = "[%d]" % self.index if self.index else ""
        return "PathStep(%s%s -> #%d)" % (self.feature_name, suffix, self.instance_id)

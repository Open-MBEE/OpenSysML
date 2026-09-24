"""Instance class wrapping runtime-materialized objects."""

from opensysml.errors import FeatureValueError
from opensysml.values import UNSET, InstanceRef, feature_value_to_python, value_to_python


class Instance:
    """Represents a runtime instance of a part/usage.

    Feature values are exposed as Python values: numbers, strings, booleans,
    lists and nested Instance objects. The protobuf messages remain reachable
    through ``get_feature()`` and ``raw_features``.

    Attributes:
        id (int): Unique instance identifier
        type_symbol_id (str): FQN of the def/usage this instantiates
        features (dict): Feature name → Python value
    """

    def __init__(self, pb_instance, graph=None, _wrappers=None):
        """Initialize from protobuf Instance message.

        Args:
            pb_instance: sysml_pb2.Instance protobuf message
            graph: optional {instance_id: sysml_pb2.Instance} of reachable
                instances, as returned by InstantiateResponse.instances
            _wrappers: internal cache shared by all instances of one graph
        """
        self._pb = pb_instance
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
        return Instance(pb_child, self._graph, self._wrappers)

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
            html.append('<table style="border-collapse: collapse; width: 100%; margin-top: 10px;">')
            html.append('<thead><tr style="background: #f0f0f0;">')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Feature</th>')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Value</th>')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Materialized</th>')
            html.append('</tr></thead><tbody>')

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

                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;">{value_str}</td>')
                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;">{"✓" if pb_value.materialized else ""}</td>')
                html.append('</tr>')

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

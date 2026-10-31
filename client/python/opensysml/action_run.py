"""What one execution of an action produced."""


class ActionOutputs(dict):
    """An action run's output parameters by name, and its performer's attributes.

    The mapping holds the output parameters alone, as it always has; an output the
    wire format cannot represent is an UnsupportedValueError in its place.

    Attributes:
        performer (dict): The attributes the object the action ran on holds when
            the run ends, keyed as an explored :class:`~opensysml.Outcome`'s
            ``outputs`` key them (``"this.level"``); empty without a performer
        order_dependent (dict): Output name → how many of the run's unresolved
            choice points (ones whose alternatives write what another reads or
            writes) touch it: the value then stands only up to the order the run
            took, and :meth:`~opensysml.connection.Connection.explore_action`
            shows the set. An output nothing order-dependent wrote is absent;
            empty from a service without the ``order_dependence`` capability.
        trace (tuple[DocumentEvent, ...]): The run's records when requested
            with ``trace=True``: each token step, wait, accept, send, decision,
            assignment and the completion, in order
        trace_dropped (int): Oldest trace records the service discarded
    """

    def __init__(self, outputs=(), performer=None, order_dependent=None, trace=(), trace_dropped=0):
        super().__init__(outputs)
        self.performer = dict(performer or {})
        self.order_dependent = dict(order_dependent or {})
        self.trace = tuple(trace)
        self.trace_dropped = trace_dropped

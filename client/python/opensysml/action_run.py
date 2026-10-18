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
    """

    def __init__(self, outputs=(), performer=None, order_dependent=None):
        super().__init__(outputs)
        self.performer = dict(performer or {})
        self.order_dependent = dict(order_dependent or {})

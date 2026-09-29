"""What one execution of an action produced."""


class ActionOutputs(dict):
    """An action run's output parameters by name, and its performer's attributes.

    The mapping holds the output parameters alone, as it always has; an output the
    wire format cannot represent is an UnsupportedValueError in its place.

    Attributes:
        performer (dict): The attributes the object the action ran on holds when
            the run ends, keyed as an explored :class:`~opensysml.Outcome`'s
            ``outputs`` key them (``"this.level"``); empty without a performer
    """

    def __init__(self, outputs=(), performer=None):
        super().__init__(outputs)
        self.performer = dict(performer or {})

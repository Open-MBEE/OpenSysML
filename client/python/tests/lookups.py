"""Stand-ins for ``get_symbol``, which names a symbol by ``name``, by ``id`` or by one legacy string."""


def answering(answer):
    """A ``get_symbol`` that hands ``answer`` the string asked for, whichever kind it was stated as."""

    def get_symbol(model_hash, symbol_id=None, *, name=None, id=None, **options):
        return answer(model_hash, asked_of(symbol_id, name, id), **options)

    return get_symbol


def asked_of(symbol_id, name, id):
    """The one string of a lookup."""
    return next(value for value in (symbol_id, name, id) if value is not None)


def asked(call):
    """The string a recorded ``get_symbol`` call asked for."""
    return asked_of(call.args[1] if len(call.args) > 1 else None, call.kwargs.get("name"), call.kwargs.get("id"))

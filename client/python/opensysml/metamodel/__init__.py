"""Generated metamodel classes and a reader for exported JSON documents."""

# pyright: reportUnsupportedDunderAll=false

from ._generated import *
from ._generated import __all__ as _generated_all
from ._reader import ElementGraph, read_json
from ._runtime import (
    Many,
    MalformedDocument,
    MalformedValue,
    MetamodelError,
    NotSupplied,
    Opt,
    UnknownJSONKey,
    UnresolvedReference,
)

__all__ = [
    *_generated_all,
    "ElementGraph",
    "Many",
    "MalformedDocument",
    "MalformedValue",
    "MetamodelError",
    "NotSupplied",
    "Opt",
    "UnknownJSONKey",
    "UnresolvedReference",
    "read_json",
]

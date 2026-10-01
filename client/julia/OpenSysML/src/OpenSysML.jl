module OpenSysML

using HTTP
using JSON

export Connection, Model, Diagnostic, Instance, InstanceRef, Quantity,
       EnumLiteral, Unset, Infinity, FunctionRef, Metaobject, Undetermined,
       ConnectError, TransportError, DiagnosticError,
       connect, external, private, call, server_info, has_capability,
       parse_file, parse_source, parse_sources, diagnostics, symbol,
       evaluate, instantiate, execute_action, execute_state, query,
       decode_value, encode_value, resolve_binary

include("errors.jl")
include("values.jl")
include("connection.jl")
include("model.jl")

end

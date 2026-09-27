struct Diagnostic
    severity::String
    message::String
    file::String
    line::Int
    col::Int
    code::String
end

function Diagnostic(d::AbstractDict)
    span = get(d, "span", Dict{String,Any}())
    Diagnostic(get(d, "severity", ""), get(d, "message", ""),
               get(span, "file", ""), get(span, "startLine", 0), get(span, "startCol", 0),
               get(d, "code", ""))
end

struct ConnectError <: Exception
    code::String
    message::String
    http_status::Int
end

function Base.showerror(io::IO, e::ConnectError)
    print(io, "ConnectError($(e.code), HTTP $(e.http_status)): $(e.message)")
end

struct TransportError <: Exception
    message::String
end

Base.showerror(io::IO, e::TransportError) = print(io, "TransportError: $(e.message)")

struct DiagnosticError <: Exception
    message::String
    diagnostics::Vector{Diagnostic}
end

function Base.showerror(io::IO, e::DiagnosticError)
    print(io, "DiagnosticError: $(e.message)")
    for d in e.diagnostics
        print(io, "\n  $(d.severity): $(d.message)")
    end
end

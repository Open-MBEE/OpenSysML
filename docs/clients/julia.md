# Julia

`OpenSysML` is a Julia client that calls the service using Connect-JSON over
`HTTP.jl`. It provides model parsing, evaluation, execution, verification and
typed query results without generated protobuf code.

## Install

Julia 1.10 or later is required. The package is not in the Julia General
registry; develop it from a checkout:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
```

## First model

Create a connection, parse the shared Demo model and check error diagnostics
before evaluating the sedan's mass:

```julia
using OpenSysML

source = """
package Demo {
  part def Vehicle {
    attribute mass default = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1800.0;
  }
}
"""

conn = connect()
try
    model = parse_source(conn, source; name="demo.sysml")
    if !isok(model)
        foreach(println, diagnostics(model))
        error("The model has errors.")
    end

    mass = evaluate(model, "mass"; subject="Demo::sedan")
    println(mass)
finally
    close(conn)
end
```

```text
1800.0
```

Julia's API also includes multi-document parsing, model values, behavior and
state execution, verification, analysis, editing, conversion and queries.

## Service source

Julia starts a private service by default, but a missing binary does not
trigger a release download unless `version=` or
`OPENSYSML_GRPC_VERSION` explicitly requests one. See the
[service-binary reference](../reference/clients.md#providing-the-service-binary)
for the available binary sources and release settings.

Parse errors are available from `diagnostics(model)`; requests that cannot be
completed raise typed client errors. The README documents the resolver's
digest behavior and the service capabilities the client uses.

## Next steps

- [Julia API reference](../reference/julia-api.md)
- [Client README and examples](https://github.com/Open-MBEE/OpenSysML/blob/main/client/julia/OpenSysML/README.md)
- [Shared SysML model examples](https://github.com/Open-MBEE/OpenSysML/tree/main/examples)

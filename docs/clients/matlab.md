# MATLAB and GNU Octave

The `+opensysml` client calls the service using Connect-JSON. It is source
distributed for MATLAB R2019b+ and GNU Octave 7+; use it when model workflows
are already built around either environment.

## Install

Add the package directory to the MATLAB or Octave path:

```matlab
addpath('client/matlab')
```

MATLAB uses `matlab.net.http`; Octave uses a `curl` subprocess.

## First model

`parseSource` accepts inline SysML. Check the parsed model before evaluating
the mass of the `Demo::sedan` subject:

```matlab
addpath('client/matlab')

source = sprintf(['package Demo {\n' ...
    '  part def Vehicle {\n' ...
    '    attribute mass default = 1500.0;\n' ...
    '  }\n' ...
    '  part sedan : Vehicle {\n' ...
    '    attribute :>> mass = 1800.0;\n' ...
    '  }\n' ...
    '}']);

conn = opensysml.connect();
cleanup = onCleanup(@() conn.close());
model = opensysml.parseSource(conn, source, 'name', 'demo.sysml');
if ~model.ok()
    disp(opensysml.diagnostics(model))
    error('The model has errors.')
end

mass = opensysml.evaluate(model, 'mass', 'subject', 'Demo::sedan');
fprintf('%.1f\n', mass);
```

`Connection` can start a private child or connect to an external address;
`Model` also supports symbol lookup, instances, verification, execution,
analysis, editing, conversion and document queries.

## Service source

MATLAB never downloads a service binary. Build or install `sysml-grpc` and
provide it through `OPENSYSML_GRPC_BINARY`, the shared cache or `$PATH`; see
the [service-binary reference](../reference/clients.md#providing-the-service-binary).
In Octave builds without Java, connect to an external service instead of
starting a child.

Parse diagnostics are available from the model, and failures use typed
`MException` identifiers under the `opensysml:*` namespace.

## Next steps

- [MATLAB API reference](../reference/matlab-api.md)
- [Client README and tests](https://github.com/Open-MBEE/OpenSysML/blob/main/client/matlab/README.md)
- [Shared SysML model examples](https://github.com/Open-MBEE/OpenSysML/tree/main/examples)

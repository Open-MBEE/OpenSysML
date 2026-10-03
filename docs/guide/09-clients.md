# 9. From your own program

Everything the REPL and editors do is available to a program. There are seven ways to reach the
engine: the Go API calls it in process, and six clients call the `sysml-grpc` service. Clients that
start a service own a private child by default; a browser client connects to an explicit service.
The
[client-library reference](../reference/clients.md) compares their coverage, transports, lifecycle
and distribution.

| Surface | Reaches the engine by | Distribution | API reference |
| --- | --- | --- | --- |
| Go, `client/opensysml` | In process, or Connect to a service | With the core `v*` tags | [Go packages](../reference/api.md) |
| Python, `opensysml` | gRPC, private child or named service | PyPI | [Python API](../reference/python-api.md) |
| Node/TypeScript, `@openmbee/opensysml` | Connect, Node or browser | npm, with per-platform service packages | [Node API](../reference/node-api.md) |
| Java, `org.openmbee:opensysml` | Connect over the JDK HTTP client | Checkout; not on Maven Central | [Java API](../reference/java-api.md) |
| Rust, `opensysml` | Blocking Connect client | crates.io | [Rust API](../reference/rust-api.md) |
| Julia, `OpenSysML` | Connect-JSON over `HTTP.jl` | Checkout; not in Julia General | [Julia API](../reference/julia-api.md) |
| MATLAB, `+opensysml` | Connect-JSON over MATLAB HTTP or Octave `curl` | Source files added to the path | [MATLAB API](../reference/matlab-api.md) |

The Python, Node and Rust packages are published to their language registries. Java is installed
from a checkout; Julia is developed from a checkout; MATLAB is source distributed. The
[client-library reference](../reference/clients.md) explains how to choose among them and what each
surface covers.

## The same task, seven ways

Each example reads the same SysML model, checks parse diagnostics, and evaluates the redefined mass
of `Demo::sedan`.

```sysml
// vehicle.sysml
package Demo {
  part def Vehicle {
    attribute mass default = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1800.0;
  }
}
```

### Install it

=== "Go"

    ```sh
    go get github.com/Open-MBEE/OpenSysML@latest
    ```

=== "Python"

    ```sh
    python -m pip install opensysml
    ```

=== "Node"

    ```sh
    npm install @openmbee/opensysml
    ```

=== "Java"

    ```sh
    make build-grpc
    mvn -f client/java/pom.xml install
    ```

    The artifact is not on Maven Central.

=== "Rust"

    ```toml
    [dependencies]
    opensysml = "0.9"
    ```

=== "Julia"

    ```julia
    using Pkg
    Pkg.develop(path = "client/julia/OpenSysML")
    ```

    Julia 1.10 or later; the package is not in General.

=== "MATLAB"

    ```matlab
    addpath('client/matlab')
    ```

    MATLAB R2019b+ or GNU Octave 7+.

### Parse, check and evaluate

=== "Go"

    ```go
    client, err := opensysml.New()
    if err != nil { return err }
    defer client.Close()

    model, err := client.ParseFile(ctx, "vehicle.sysml")
    if err != nil { return err }
    if errors := model.Errors(); len(errors) != 0 {
        return fmt.Errorf("model diagnostics: %v", errors)
    }
    mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
    if err != nil { return err }
    fmt.Println(mass)
    ```

=== "Python"

    ```python
    import opensysml

    model = opensysml.load("vehicle.sysml")
    if not model.ok:
        for diagnostic in model.diagnostics:
            print(diagnostic)
        raise ValueError("The model has errors.")

    print(model.eval("mass", subject="Demo::sedan"))  # 1800.0
    ```

=== "Node"

    ```ts
    import { load } from "@openmbee/opensysml";

    const model = await load("vehicle.sysml");
    try {
      if (model.hasErrors) throw new Error(JSON.stringify(model.diagnostics));
      const mass = await model.eval("mass", { subject: "Demo::sedan" });
      if (mass.kind !== "real") throw new Error(`Expected real, got ${mass.kind}`);
      console.log(mass.value.toFixed(1)); // 1800.0
    } finally {
      await model.close();
    }
    ```

=== "Java"

    ```java
    try (Connection connection = Connection.open()) {
      Model model = connection.load(Path.of("vehicle.sysml"));
      if (!model.ok()) {
        model.diagnostics().forEach(System.err::println);
        throw new IllegalStateException("The model has errors.");
      }
      Value value = model.evalWithSubject("mass", "Demo::sedan");
      System.out.printf("%.1f%n", ((Value.RealValue) value).value()); // 1800.0
    }
    ```

=== "Rust"

    ```rust
    let model = opensysml::load("vehicle.sysml")?;
    if !model.ok() {
        for diagnostic in model.diagnostics() { eprintln!("{diagnostic}"); }
        return Err("The model has errors.".into());
    }
    let result = model.evaluate(
        "mass",
        &opensysml::EvalOptions {
            subject: Some("Demo::sedan".into()),
            ..Default::default()
        },
    )?;
    if let opensysml::Value::Real(mass) = result.result {
        println!("{mass:.1}"); // 1800.0
    }
    ```

=== "Julia"

    ```julia
    using OpenSysML

    conn = connect()
    try
        model = parse_file(conn, "vehicle.sysml")
        if !isok(model)
            foreach(println, diagnostics(model))
            error("The model has errors.")
        end
        println(evaluate(model, "mass"; subject="Demo::sedan"))  # 1800.0
    finally
        close(conn)
    end
    ```

=== "MATLAB"

    ```matlab
    conn = opensysml.connect();
    cleanup = onCleanup(@() conn.close());
    model = opensysml.parseFile(conn, 'vehicle.sysml');
    if ~model.ok()
        disp(opensysml.diagnostics(model))
        error('The model has errors.')
    end
    mass = opensysml.evaluate(model, 'mass', 'subject', 'Demo::sedan');
    fprintf('%.1f\n', mass);  % 1800.0
    ```

The same engine parses the model in every client. A parse that reports errors still returns
diagnostics; the examples check those before using the result. A false constraint verdict is an
answer about the model, not an exception. For service resolution and error details, see the
[client-library reference](../reference/clients.md).

## From Go

The [client index](../clients.md) links to the Go overview and walkthrough. The
[Go package reference](../reference/api.md) documents the exported in-process surface.

## From Python

The [Python client guide](../clients/python/index.md) is split by task, from models and diagnostics through
verification, editing, queries and service setup. See also the [Python API reference](../reference/python-api.md).

## From Node or a browser

The [Node client guide](../clients/node.md) covers npm installation, typed values and browser
connections. See the [Node API reference](../reference/node-api.md) for the full surface.

## From Java

The [Java client guide](../clients/java.md) covers checkout installation, parsing and service
connections. See the [Java API reference](../reference/java-api.md) for types and exceptions.

## From Rust

The [Rust client guide](../clients/rust.md) covers the crates.io package and a first model. See the
[Rust API reference](../reference/rust-api.md) for the blocking typed API.

## From Julia

The [Julia client guide](../clients/julia.md) covers checkout installation and Connect-JSON usage.
See the [Julia API reference](../reference/julia-api.md) for the full package surface.

## From MATLAB

The [MATLAB client guide](../clients/matlab.md) covers adding the source package to the path and
connecting from MATLAB or Octave. See the [MATLAB API reference](../reference/matlab-api.md) for
the complete surface.

---

Next: [10. Troubleshooting](10-troubleshooting.md).

# Java

The Java client is for JVM applications that call the `sysml-grpc` service
without taking ownership of their host application's runtime or transport.
Its typed immutable results are useful in desktop tooling, servers and
Eclipse-based integrations.

## Install

The artifact is not on Maven Central. Build and install it into the local
Maven repository from a checkout; JDK 17 is the baseline:

```bash
make build-grpc
mvn -f client/java/pom.xml install
```

Then add `org.openmbee:opensysml` at the matching client version to the
consumer's `pom.xml`.

## First model

`Connection.parse` accepts inline SysML. Check the model diagnostics, then
evaluate the subject's redefined mass:

```java
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Value;

public class Demo {
  public static void main(String[] args) throws Exception {
    String source = """
        package Demo {
          part def Vehicle {
            attribute mass default = 1500.0;
          }
          part sedan : Vehicle {
            attribute :>> mass = 1800.0;
          }
        }
        """;

    try (Connection connection = Connection.open()) {
      Model model = connection.parse(source);
      if (!model.ok()) {
        for (Diagnostic diagnostic : model.diagnostics()) System.err.println(diagnostic);
        throw new IllegalStateException("The model has errors.");
      }

      Value value = model.evalWithSubject("mass", "Demo::sedan");
      if (!(value instanceof Value.RealValue real)) {
        throw new IllegalStateException("Expected a real value.");
      }
      System.out.printf("%.1f%n", real.value());
    }
  }
}
```

```text
1800.0
```

`Connection.open()` owns the private child it starts and closes it with the
connection. A connection can instead target a service owned by the
application or another process. The client also provides symbol lookup,
instantiation, verification, analysis, execution, queries and source edits.

## Service source

The Java client starts a private service by default and can also connect to an
explicitly managed service. See the
[service-binary reference](../reference/clients.md#providing-the-service-binary)
for the binary lookup order, version requirements and verification options.

Invalid models expose their parse diagnostics through `Model`; unsupported
service capabilities raise a typed exception before the client makes a
capability-gated call.

## Next steps

- [Java API reference](../reference/java-api.md)
- [Client README and examples](https://github.com/Open-MBEE/OpenSysML/blob/main/client/java/README.md)
- [Shared SysML model examples](https://github.com/Open-MBEE/OpenSysML/tree/main/examples)

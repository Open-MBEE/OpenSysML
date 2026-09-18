package org.openmbee.opensysml;

import java.nio.file.Path;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/** Opens a connection, reports the service's pid, and then waits to be killed. */
public final class OrphanSafetyChild {

  private OrphanSafetyChild() {}

  /**
   * Runs the child JVM.
   *
   * @param args the service binary, and {@code halt} to leave by {@code Runtime.halt} instead
   * @throws Exception if the connection could not be opened
   */
  public static void main(String[] args) throws Exception {
    Connection connection =
        Connection.open(ConnectionOptions.builder().binaryPath(Path.of(args[0])).build());
    // Prove the service answers before anyone kills this JVM.
    connection.parse("package Demo { part def Vehicle; }");
    System.out.println(connection.ownedService().orElseThrow().pid());
    System.out.flush();
    if (args.length > 1 && args[1].equals("halt")) {
      Runtime.getRuntime().halt(9);
    }
    new CountDownLatch(1).await(10, TimeUnit.MINUTES);
  }
}

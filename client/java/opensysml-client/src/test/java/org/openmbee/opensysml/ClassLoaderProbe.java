package org.openmbee.opensysml;

import java.nio.file.Path;

/** Runs a whole connection lifecycle, for a copy of the client loaded by a throwaway classloader. */
public final class ClassLoaderProbe {

  private ClassLoaderProbe() {}

  /**
   * Opens a connection, uses it, closes it and stops the service this copy of the client owns.
   *
   * @param binary the service binary
   * @return the pid of the service this copy started
   */
  public static long run(String binary) {
    long pid;
    try (Connection connection =
        Connection.open(ConnectionOptions.builder().binaryPath(Path.of(binary)).build())) {
      pid = connection.ownedService().orElseThrow().pid();
      connection.parse("package Demo { part def Vehicle; }");
    }
    Connection.stopSharedServices();
    return pid;
  }
}

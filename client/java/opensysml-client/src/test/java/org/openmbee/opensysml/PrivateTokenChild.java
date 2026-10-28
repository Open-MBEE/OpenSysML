package org.openmbee.opensysml;

import java.nio.file.Path;

/**
 * Started with {@code OPENSYSML_GRPC_TOKEN} set, opens a private service and then reaches it with
 * another token: a child that inherited the variable would refuse the second call.
 */
public final class PrivateTokenChild {

  private PrivateTokenChild() {}

  /**
   * Runs the child JVM.
   *
   * @param args the service binary
   * @throws Exception if either connection could not be opened
   */
  public static void main(String[] args) throws Exception {
    try (Connection owner =
        Connection.open(ConnectionOptions.builder().binaryPath(Path.of(args[0])).build())) {
      int colon = owner.address().lastIndexOf(':');
      String host = owner.address().substring(0, colon).replace("[", "").replace("]", "");
      int port = Integer.parseInt(owner.address().substring(colon + 1));
      try (Connection other =
          Connection.open(
              ConnectionOptions.builder().service(host, port).token("another-token").build())) {
        // Opening already asked the service for its handshake; a call proves it keeps serving.
        other.parse("package Demo { part def Vehicle; }");
        System.out.println("served");
        System.out.flush();
      }
    }
  }
}

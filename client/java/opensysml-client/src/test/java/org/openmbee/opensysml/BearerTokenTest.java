package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

/** A service that requires a bearer token admits the right one and refuses the rest. */
class BearerTokenTest {

  private static final String TOKEN = "java-token_1";

  @Test
  void theRightTokenIsAdmittedAndTheRestRefused() throws IOException, InterruptedException {
    Process service =
        new ProcessBuilder(
                ServiceBinary.required().toString(),
                "-port", "0", "-health-port", "0", "-report-address", "-token", TOKEN)
            .redirectError(ProcessBuilder.Redirect.DISCARD)
            .start();
    try {
      String address;
      try (BufferedReader reader =
          new BufferedReader(
              new InputStreamReader(service.getInputStream(), StandardCharsets.UTF_8))) {
        address = reader.readLine();
      }
      int colon = address.lastIndexOf(':');
      String host = address.substring(0, colon).replace("[", "").replace("]", "");
      int port = Integer.parseInt(address.substring(colon + 1));

      try (Connection admitted =
          Connection.open(ConnectionOptions.builder().service(host, port).token(TOKEN).build())) {
        // Opening already asked the service for its handshake, so the token was admitted.
        assertNotNull(admitted);
      }
      for (ConnectionOptions.Builder refused :
          new ConnectionOptions.Builder[] {
            ConnectionOptions.builder().service(host, port),
            ConnectionOptions.builder().service(host, port).token(TOKEN + "x"),
          }) {
        ConnectionOptions options = refused.build();
        ServiceException e = assertThrows(ServiceException.class, () -> Connection.open(options));
        assertEquals(StatusCode.UNAUTHENTICATED, e.status());
      }
    } finally {
      service.destroyForcibly().waitFor();
    }
  }

  @Test
  void aPrivateChildDoesNotInheritTheEnvironmentToken() throws IOException, InterruptedException {
    ProcessBuilder builder =
        new ProcessBuilder(
            ProcessHandle.current().info().command().orElseThrow(),
            "-cp",
            System.getProperty("java.class.path"),
            PrivateTokenChild.class.getName(),
            ServiceBinary.required().toString());
    builder.environment().put(ConnectionOptions.TOKEN_ENV, TOKEN);
    builder.redirectError(ProcessBuilder.Redirect.INHERIT);
    Process jvm = builder.start();
    String version;
    try (BufferedReader reader =
        new BufferedReader(new InputStreamReader(jvm.getInputStream(), StandardCharsets.UTF_8))) {
      version = reader.readLine();
    }
    assertTrue(jvm.waitFor(60, TimeUnit.SECONDS), "the child JVM did not end");
    assertEquals(0, jvm.exitValue(), "the child JVM failed; its stderr is above");
    assertNotNull(version);
    assertFalse(version.isBlank());
  }
}

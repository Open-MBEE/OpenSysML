package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
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
        ServiceException e =
            assertThrows(ServiceException.class, () -> Connection.open(refused.build()));
        assertEquals(StatusCode.UNAUTHENTICATED, e.status());
      }
    } finally {
      service.destroyForcibly().waitFor();
    }
  }
}

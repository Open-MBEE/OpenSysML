package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.Optional;
import org.junit.jupiter.api.Test;

class ConnectionTest {

  @Test
  void builtAgainstDefaultDoesNotRequireAServiceVersion() {
    assertEquals(
        Optional.empty(),
        Connection.requiredRelease(ConnectionOptions.defaults(), name -> null));
  }

  @Test
  void anEnvironmentReleaseIsStillRequired() {
    assertEquals(
        Optional.of("v0.3.0"),
        Connection.requiredRelease(
            ConnectionOptions.defaults(),
            name -> ConnectionOptions.VERSION_ENV.equals(name) ? " v0.3.0 " : null));
  }
}

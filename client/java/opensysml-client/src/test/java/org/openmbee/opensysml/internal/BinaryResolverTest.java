package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.ServiceStartException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** What the client will and will not execute. */
class BinaryResolverTest {

  @Test
  void aPinnedDigestIsCheckedBeforeAnythingIsExecuted(@TempDir Path directory) throws Exception {
    Path binary = executable(directory, "#!/bin/sh\nexit 0\n");
    ConnectionOptions wrong =
        ConnectionOptions.builder()
            .binaryPath(binary)
            .expectedBinarySha256("0".repeat(64))
            .build();
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> BinaryResolver.resolve(wrong));
    assertTrue(refused.getMessage().contains("it was not started"));

    ConnectionOptions right =
        ConnectionOptions.builder().binaryPath(binary).expectedBinarySha256(sha256(binary)).build();
    assertEquals(binary, BinaryResolver.resolve(right));
  }

  @Test
  void aDigestIsCaseInsensitiveHex(@TempDir Path directory) throws Exception {
    Path binary = executable(directory, "#!/bin/sh\nexit 0\n");
    ConnectionOptions options =
        ConnectionOptions.builder()
            .binaryPath(binary)
            .expectedBinarySha256(sha256(binary).toUpperCase(java.util.Locale.ROOT))
            .build();
    assertEquals(binary, BinaryResolver.resolve(options));
  }

  @Test
  void aNamedBinaryThatIsNotExecutableIsRefused(@TempDir Path directory) throws Exception {
    Path notExecutable = Files.writeString(directory.resolve("sysml-grpc"), "text");
    ConnectionOptions options = ConnectionOptions.builder().binaryPath(notExecutable).build();
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> BinaryResolver.resolve(options));
    assertTrue(refused.getMessage().contains("no executable sysml-grpc"));
  }

  @Test
  void aMissingBinaryNamesEverywhereItLooked(@TempDir Path directory) {
    ConnectionOptions options =
        ConnectionOptions.builder().binaryPath(directory.resolve("absent")).build();
    assertThrows(ServiceStartException.class, () -> BinaryResolver.resolve(options));
  }

  private static Path executable(Path directory, String script) throws Exception {
    Path binary = Files.writeString(directory.resolve("sysml-grpc"), script);
    assertTrue(binary.toFile().setExecutable(true));
    return binary;
  }

  private static String sha256(Path file) throws Exception {
    return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(Files.readAllBytes(file)));
  }
}

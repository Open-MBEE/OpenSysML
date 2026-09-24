package org.openmbee.opensysml.cameo;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import org.junit.jupiter.api.Assumptions;

/** Locates the repository's built sysml-grpc, the way the Java client's own integration tests do. */
final class ServiceBinary {
  static final String REQUIRE_PROPERTY = "opensysml.requireService";

  private ServiceBinary() {}

  static Path repository() {
    Path directory = Path.of("").toAbsolutePath();
    while (directory != null && !Files.isRegularFile(directory.resolve("go.mod"))) directory = directory.getParent();
    if (directory == null) throw new IllegalStateException("repository root (go.mod) not found");
    return directory;
  }

  static Path required() {
    Path binary = repository().resolve("bin").resolve("sysml-grpc");
    Optional<Path> found = Files.isExecutable(binary) ? Optional.of(binary) : Optional.empty();
    if (found.isEmpty() && !Boolean.getBoolean(REQUIRE_PROPERTY)) {
      Assumptions.abort("bin/sysml-grpc is not built; run `make build-grpc`");
    }
    return found.orElseThrow(() -> new IllegalStateException(
        "bin/sysml-grpc is not built and -D" + REQUIRE_PROPERTY + " requires it"));
  }
}

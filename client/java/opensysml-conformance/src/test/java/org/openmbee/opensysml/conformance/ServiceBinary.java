package org.openmbee.opensysml.conformance;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import org.junit.jupiter.api.Assumptions;

/** Finds the {@code sysml-grpc} the suite runs against. */
final class ServiceBinary {

  /** Set in CI so an absent binary fails the build rather than skipping the suite. */
  static final String REQUIRE_PROPERTY = "opensysml.requireService";

  private ServiceBinary() {}

  /** The repository's built binary, skipping the test when it has not been built. */
  static Path required() {
    Optional<Path> binary = find();
    if (binary.isEmpty() && !Boolean.getBoolean(REQUIRE_PROPERTY)) {
      Assumptions.abort("bin/sysml-grpc is not built; run `make build-grpc`");
    }
    return binary.orElseThrow(
        () ->
            new IllegalStateException(
                "bin/sysml-grpc is not built and -D" + REQUIRE_PROPERTY + " requires it"));
  }

  private static Optional<Path> find() {
    for (Path directory = Path.of("").toAbsolutePath();
        directory != null;
        directory = directory.getParent()) {
      if (Files.isRegularFile(directory.resolve("go.mod"))) {
        Path binary = directory.resolve("bin").resolve("sysml-grpc");
        return Files.isExecutable(binary) ? Optional.of(binary) : Optional.empty();
      }
    }
    return Optional.empty();
  }
}

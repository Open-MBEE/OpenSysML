package org.openmbee.opensysml;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import org.junit.jupiter.api.Assumptions;

/** Finds the {@code sysml-grpc} the integration tests start. */
final class ServiceBinary {

  /** Set in CI so an absent binary fails the build rather than skipping the tests. */
  static final String REQUIRE_PROPERTY = "opensysml.requireService";

  private ServiceBinary() {}

  /**
   * The repository's built binary, if it is there.
   *
   * @return the binary path
   */
  static Optional<Path> find() {
    return repositoryRoot()
        .map(root -> root.resolve("bin").resolve("sysml-grpc"))
        .filter(Files::isExecutable);
  }

  /**
   * The repository's root, the nearest ancestor of the working directory that holds a {@code
   * go.mod}, if there is one.
   *
   * @return the root
   */
  static Optional<Path> repositoryRoot() {
    Path directory = Path.of("").toAbsolutePath();
    while (directory != null) {
      if (Files.isRegularFile(directory.resolve("go.mod"))) {
        return Optional.of(directory);
      }
      directory = directory.getParent();
    }
    return Optional.empty();
  }

  /**
   * The binary, skipping the test when it has not been built unless CI requires it.
   *
   * @return the binary path
   */
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

  /**
   * Options that start a private service from the built binary.
   *
   * @return a builder naming the binary
   */
  static ConnectionOptions.Builder options() {
    return ConnectionOptions.builder().binaryPath(required());
  }
}

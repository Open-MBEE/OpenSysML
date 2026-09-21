package org.openmbee.opensysml.syson;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;

import org.junit.jupiter.api.Assumptions;

final class ServiceBinary {
    static final String REQUIRE_PROPERTY = "opensysml.requireService";

    private ServiceBinary() {
    }

    static Optional<Path> find() {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            if (Files.isRegularFile(directory.resolve("go.mod"))) {
                Path binary = directory.resolve("bin").resolve("sysml-grpc");
                return Files.isExecutable(binary) ? Optional.of(binary) : Optional.empty();
            }
            directory = directory.getParent();
        }
        return Optional.empty();
    }

    static Path required() {
        Optional<Path> binary = find();
        if (binary.isEmpty() && !Boolean.getBoolean(REQUIRE_PROPERTY)) {
            Assumptions.abort("bin/sysml-grpc is not built; run `make build-grpc`");
        }
        return binary.orElseThrow(() -> new IllegalStateException(
                "bin/sysml-grpc is not built and -D" + REQUIRE_PROPERTY + " requires it"));
    }
}

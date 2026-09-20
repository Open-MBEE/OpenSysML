package org.openmbee.opensysml.cameo.engine;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Supplier;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.cameo.bin.HostBinary;
import org.openmbee.opensysml.cameo.results.ResultsMapper;
import org.openmbee.opensysml.cameo.results.RunResult;

/** Runs one operation at a time over a lazily opened, reused connection to sysml-grpc. */
public final class Engine implements AutoCloseable {
  private static final Duration CANCEL_POLL = Duration.ofMillis(250);

  private final Supplier<ConnectionOptions> options;
  private Connection connection;

  public Engine(Supplier<ConnectionOptions> options) {
    this.options = Objects.requireNonNull(options);
  }

  public Engine(Path pluginDirectory) {
    this(() -> optionsFor(pluginDirectory));
  }

  static ConnectionOptions optionsFor(Path pluginDirectory) {
    Path binary = HostBinary.locate(pluginDirectory);
    ConnectionOptions.Builder builder =
        ConnectionOptions.builder().binaryPath(binary).requestTimeout(Duration.ofHours(1));
    Path digests = pluginDirectory.resolve("bin").resolve(HostBinary.DIGESTS_FILE);
    if (Files.isRegularFile(digests)) {
      try {
        String asset = binary.getFileName().toString();
        Files.readAllLines(digests).stream()
            .filter(line -> line.endsWith("  " + asset))
            .findFirst()
            .map(line -> line.substring(0, line.indexOf("  ")))
            .ifPresent(builder::expectedBinarySha256);
      } catch (IOException exception) {
        throw new IllegalStateException("cannot read service digest manifest", exception);
      }
    }
    return builder.build();
  }

  private synchronized Connection connection() {
    if (connection == null) connection = Connection.open(options.get());
    return connection;
  }

  private synchronized void dropConnection() {
    if (connection != null) {
      connection.close();
      connection = null;
    }
  }

  public RunResult run(RunRequest request, Cancellation cancellation) {
    Objects.requireNonNull(request);
    Objects.requireNonNull(cancellation);
    if (cancellation.requested()) return RunResult.cancelled(request);
    long started = System.nanoTime();
    Thread watcher = new Thread(() -> watch(cancellation), "opensysml-cancel-watcher");
    watcher.setDaemon(true);
    watcher.start();
    try {
      Connection current = connection();
      Model model = current.parseSources(request.source().sources(current));
      List<Diagnostic> setup = new ArrayList<>(request.source().exportDiagnostics());
      setup.addAll(model.parseDiagnostics());
      RunResult result = dispatch(request, model).withLeadingDiagnostics(setup);
      return cancellation.requested() ? RunResult.cancelled(request) : result.withElapsed(elapsed(started));
    } catch (RuntimeException exception) {
      if (cancellation.requested()) return RunResult.cancelled(request);
      return RunResult.error(request, exception, elapsed(started));
    } finally {
      watcher.interrupt();
    }
  }

  private void watch(Cancellation cancellation) {
    while (!cancellation.requested()) {
      try {
        Thread.sleep(CANCEL_POLL.toMillis());
      } catch (InterruptedException interrupted) {
        return;
      }
    }
    dropConnection();
  }

  private static RunResult dispatch(RunRequest request, Model model) {
    Duration elapsed = Duration.ZERO;
    String subject = request.subjectQualifiedName();
    return switch (request.operation()) {
      case INSTANTIATE -> ResultsMapper.map(request, model.instantiate(subject), elapsed);
      case EXECUTE_ACTION -> ResultsMapper.map(request, model.executeAction(subject), elapsed);
      case EXECUTE_STATE -> ResultsMapper.map(request, model.executeState(subject, List.of()), elapsed);
      case EVALUATE_CALC ->
          ResultsMapper.map(request, model.evaluateCalc(subject, request.calcArgs()), elapsed);
      case RUN_ANALYSIS -> ResultsMapper.map(request, model.runAnalysis(subject), elapsed);
      case VERIFY -> verify(request, model);
    };
  }

  /** Picks the verification RPC from the symbol's kind so the user need not know it. */
  private static RunResult verify(RunRequest request, Model model) {
    Duration elapsed = Duration.ZERO;
    String subject = request.subjectQualifiedName();
    Optional<Symbol> symbol = model.findSymbol(subject);
    String kind = symbol.map(Symbol::kind).orElse("");
    if (kind.startsWith("satisfyRequirement")) {
      return ResultsMapper.map(request, model.verifySatisfaction(subject), elapsed);
    }
    if (kind.startsWith("requirement")) {
      return ResultsMapper.map(request, model.verifyRequirement(subject), elapsed);
    }
    return ResultsMapper.map(request, model.verifyConstraint(subject), elapsed);
  }

  private static Duration elapsed(long started) {
    return Duration.ofNanos(System.nanoTime() - started);
  }

  @Override
  public void close() {
    dropConnection();
  }
}

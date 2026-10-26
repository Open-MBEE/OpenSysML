package org.openmbee.opensysml.mdk.engine;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Predicate;
import java.util.function.Supplier;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.StatusCode;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.mdk.bin.HostBinary;
import org.openmbee.opensysml.mdk.export.ModelExport;
import org.openmbee.opensysml.mdk.results.ResultsMapper;
import org.openmbee.opensysml.mdk.results.RunResult;
import org.openmbee.opensysml.mdk.source.ModelSource;

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
    Thread watcher = null;
    Connection current = null;
    try {
      current = connection();
      watcher = watcher(cancellation, current);
      Model model = current.parseSources(request.source().sources(current));
      List<Diagnostic> setup = new ArrayList<>(request.source().exportDiagnostics());
      setup.addAll(model.parseDiagnostics());
      RunResult result = dispatch(request, model)
          .withOutcomeNames(request.source()::elementName)
          .withLeadingDiagnostics(setup);
      return cancellation.requested() ? RunResult.cancelled(request) : result.withElapsed(elapsed(started));
    } catch (RuntimeException exception) {
      if (cancellation.requested()) return RunResult.cancelled(request);
      if (current != null && isUnavailable(exception)) dropConnection(current);
      return RunResult.error(request, exception, elapsed(started));
    } finally {
      stop(watcher);
    }
  }

  /**
   * Exports the model text of {@code source} to {@code destination} — a v1 archive is migrated,
   * a v2 project's text is written as the tool exported it — with its files and migration report
   * beside it, and parses the text so the result carries the service's diagnostics on it. Files
   * the export would replace are put to {@code replaceExisting} before anything is written.
   *
   * @return the export, or empty when cancelled or {@code replaceExisting} declined
   * @throws RuntimeException when the service or the file system fails; the caller reports it
   */
  public Optional<ModelExport.Result> export(
      ModelSource source, Path destination, Predicate<List<Path>> replaceExisting, Cancellation cancellation) {
    Objects.requireNonNull(source);
    Objects.requireNonNull(destination);
    Objects.requireNonNull(replaceExisting);
    Objects.requireNonNull(cancellation);
    if (cancellation.requested()) return Optional.empty();
    Thread watcher = null;
    Connection current = null;
    try {
      current = connection();
      watcher = watcher(cancellation, current);
      List<SourceDocument> documents = named(source.sources(current), destination);
      Model model = current.parseSources(documents);
      if (cancellation.requested()) return Optional.empty();
      List<Diagnostic> diagnostics = new ArrayList<>(source.exportDiagnostics());
      diagnostics.addAll(model.parseDiagnostics());
      return ModelExport.write(
          destination, documents, source.files(), source.reportText(), diagnostics, replaceExisting);
    } catch (RuntimeException exception) {
      if (cancellation.requested()) return Optional.empty();
      if (current != null && isUnavailable(exception)) dropConnection(current);
      throw exception;
    } finally {
      stop(watcher);
    }
  }

  /** The first document under the destination's file name, so the diagnostics name the file the user saved. */
  private static List<SourceDocument> named(List<SourceDocument> documents, Path destination) {
    if (documents.isEmpty()) return documents;
    List<SourceDocument> named = new ArrayList<>(documents);
    named.set(0, documents.get(0).withName(destination.getFileName().toString()));
    return List.copyOf(named);
  }

  private Thread watcher(Cancellation cancellation, Connection current) {
    Thread watcher = new Thread(() -> watch(cancellation, current), "opensysml-cancel-watcher");
    watcher.setDaemon(true);
    watcher.start();
    return watcher;
  }

  private static void stop(Thread watcher) {
    if (watcher == null) return;
    watcher.interrupt();
    try {
      watcher.join();
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
    }
  }

  private static boolean isUnavailable(Throwable failure) {
    for (Throwable cause = failure; cause != null; cause = cause.getCause()) {
      if (cause instanceof ServiceException service
          && service.status() == StatusCode.UNAVAILABLE) {
        return true;
      }
    }
    return false;
  }

  private synchronized void dropConnection(Connection expected) {
    if (connection == expected) {
      expected.close();
      connection = null;
    }
  }

  private void watch(Cancellation cancellation, Connection expected) {
    while (!cancellation.requested()) {
      try {
        Thread.sleep(CANCEL_POLL.toMillis());
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        return;
      }
    }
    dropConnection(expected);
  }

  private static RunResult dispatch(RunRequest request, Model model) {
    Duration elapsed = Duration.ZERO;
    String subject = request.source().subjectName(request.subjectQualifiedName());
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
    String subject = request.source().subjectName(request.subjectQualifiedName());
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

package org.openmbee.opensysml.conformance;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.Encoding;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Optional;
import java.util.regex.Pattern;

/**
 * Runs the shared conformance suite through the Java client.
 *
 * <p>Every scenario is made as a call on the client's public API, so a report says what the client
 * itself reads out of an answer and not only what the service put on the wire. The RPCs v1 does not
 * cover are skipped, and the count is in the report.
 */
public final class Main {

  private static final PrintStream STDERR =
      new PrintStream(new FileOutputStream(FileDescriptor.err), true, StandardCharsets.UTF_8);
  private static final PrintStream STDOUT =
      new PrintStream(new FileOutputStream(FileDescriptor.out), true, StandardCharsets.UTF_8);

  private Main() {}

  /** What the runner was asked to do. */
  record Options(
      Path directory,
      Optional<Path> binary,
      Optional<String> service,
      Optional<Path> report,
      Optional<Pattern> filter,
      List<String> protocols,
      Mutations mutation,
      boolean allowSkips,
      boolean verbose) {}

  private static final String USAGE =
      """
      usage: conformance [options]

        -dir <path>        conformance directory (default: the repository's conformance/)
        -binary <path>     sysml-grpc to start privately (default: the client's own discovery)
        -service <addr>    host:port of a service already running, instead of a private one
        -report <path>     write the machine-readable report here
        -run <regexp>      only scenarios whose id matches
        -protocols <list>  connect,connect-json (default: connect)
        -mutate <name>     corrupt every answer: perturb-reals, truncate-lists, rewrite-strings
        -allow-skips       do not fail the run for skipped scenarios
        -v                 print each normalized answer
      """;

  /**
   * Runs the suite and exits non-zero if anything failed.
   *
   * @param args the flags above
   */
  public static void main(String[] args) {
    Options options;
    try {
      options = parse(args);
    } catch (IllegalArgumentException e) {
      STDERR.println(e.getMessage());
      STDERR.print(USAGE);
      System.exit(2);
      return;
    }
    System.exit(run(options, STDOUT));
  }

  /** Runs the suite as the command line asked, returning the exit status. */
  static int run(Options options, PrintStream out) {
    List<Scenario> scenarios = Scenarios.load(options.directory().resolve("scenarios"));
    Path fixtures = options.directory().resolve("fixtures");
    if (!Files.isDirectory(fixtures)) {
      throw new IllegalArgumentException("no fixtures directory at " + fixtures);
    }

    Report report = new Report();
    for (String protocol : options.protocols()) {
      Encoding encoding = protocol.equals("connect-json") ? Encoding.JSON : Encoding.PROTOBUF;
      try (Connection connection = open(options, encoding)) {
        report.service = connection.address();
        Runner runner =
            new Runner(
                protocol, connection, fixtures, out, options.verbose(), options.mutation());
        report.add(runner.runAll(scenarios, options.filter()));
      }
    }

    out.printf(
        "%ntotal %d scenarios: %d passed, %d failed, %d skipped, %d in error%n",
        report.total, report.passed, report.failed, report.skipped, report.errored);
    options.report().ifPresent(path -> write(path, report));

    if (report.failed > 0 || report.errored > 0) {
      return 1;
    }
    if (report.skipped > 0 && !options.allowSkips()) {
      out.printf("%d scenarios were skipped; pass -allow-skips to accept that%n", report.skipped);
      return 1;
    }
    return 0;
  }

  private static Connection open(Options options, Encoding encoding) {
    ConnectionOptions.Builder builder = ConnectionOptions.builder().encoding(encoding);
    options.binary().ifPresent(builder::binaryPath);
    options
        .service()
        .ifPresent(
            address -> {
              int separator = address.lastIndexOf(':');
              if (separator < 0) {
                throw new IllegalArgumentException("-service wants host:port, got " + address);
              }
              builder.service(
                  address.substring(0, separator),
                  Integer.parseInt(address.substring(separator + 1)));
            });
    return Connection.open(builder.build());
  }

  private static void write(Path path, Report report) {
    Gson gson = new GsonBuilder().setPrettyPrinting().create();
    try {
      Path parent = path.toAbsolutePath().getParent();
      if (parent != null) {
        Files.createDirectories(parent);
      }
      Files.writeString(path, gson.toJson(report) + "\n", StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new UncheckedIOException("writing " + path, e);
    }
  }

  /** Parses the command line; an {@link IllegalArgumentException} is a usage error. */
  static Options parse(String[] args) {
    Path directory = null;
    Path binary = null;
    String service = null;
    Path report = null;
    Pattern filter = null;
    List<String> protocols = List.of("connect");
    Mutations mutation = Mutations.NONE;
    boolean allowSkips = false;
    boolean verbose = false;

    int index = 0;
    while (index < args.length) {
      String flag = args[index++];
      switch (flag) {
        case "-dir" -> directory = Path.of(value(args, index++, flag));
        case "-binary" -> binary = Path.of(value(args, index++, flag));
        case "-service" -> service = value(args, index++, flag);
        case "-report" -> report = Path.of(value(args, index++, flag));
        case "-run" -> filter = Pattern.compile(value(args, index++, flag));
        case "-protocols" -> protocols = protocols(value(args, index++, flag));
        case "-mutate" -> mutation = Mutations.of(value(args, index++, flag));
        case "-allow-skips" -> allowSkips = true;
        case "-v" -> verbose = true;
        case "-h", "-help", "--help" -> {
          STDOUT.print(USAGE);
          System.exit(0);
        }
        default -> throw new IllegalArgumentException("unknown flag " + flag);
      }
    }

    return new Options(
        directory == null ? conformanceDirectory() : directory,
        Optional.ofNullable(binary),
        Optional.ofNullable(service),
        Optional.ofNullable(report),
        Optional.ofNullable(filter),
        protocols,
        mutation,
        allowSkips,
        verbose);
  }

  private static List<String> protocols(String value) {
    List<String> protocols = new ArrayList<>();
    for (String protocol : value.split(",", -1)) {
      String name = protocol.trim().toLowerCase(Locale.ROOT);
      switch (name) {
        case "connect", "connect-json" -> protocols.add(name);
        case "grpc" ->
            throw new IllegalArgumentException(
                "this client speaks the Connect protocol only; use cmd/conformance for grpc");
        default -> throw new IllegalArgumentException("unknown protocol " + protocol);
      }
    }
    if (protocols.isEmpty()) {
      throw new IllegalArgumentException("-protocols names none");
    }
    return List.copyOf(protocols);
  }

  private static String value(String[] args, int index, String flag) {
    if (index >= args.length) {
      throw new IllegalArgumentException(flag + " wants a value");
    }
    return args[index];
  }

  /** The repository's conformance directory, found by walking up from the working directory. */
  static Path conformanceDirectory() {
    for (Path directory = Path.of("").toAbsolutePath();
        directory != null;
        directory = directory.getParent()) {
      Path candidate = directory.resolve("conformance");
      if (Files.isDirectory(candidate.resolve("scenarios"))) {
        return candidate;
      }
    }
    throw new IllegalArgumentException("no conformance/scenarios above the working directory");
  }
}

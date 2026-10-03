package org.openmbee.opensysml;

import com.google.protobuf.ByteString;
import com.google.protobuf.Message;
import org.openmbee.opensysml.internal.BinaryDownloader;
import org.openmbee.opensysml.internal.ConnectTransport;
import org.openmbee.opensysml.internal.PrivateService;
import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.internal.ServiceRegistry;
import org.openmbee.opensysml.proto.ConvertRequest;
import org.openmbee.opensysml.proto.ConvertResponse;
import org.openmbee.opensysml.proto.ListEnginesRequest;
import org.openmbee.opensysml.proto.MigrateRequest;
import org.openmbee.opensysml.proto.MigrateResponse;
import org.openmbee.opensysml.proto.ListEnginesResponse;
import org.openmbee.opensysml.proto.ParseFileRequest;
import org.openmbee.opensysml.proto.ParseFileResponse;
import org.openmbee.opensysml.proto.ParseSourcesRequest;
import org.openmbee.opensysml.proto.ParseSourcesResponse;
import org.openmbee.opensysml.proto.ServerInfoRequest;
import org.openmbee.opensysml.proto.ServerInfoResponse;
import java.nio.file.Path;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * A connection to a {@code sysml-grpc} service, and the entry point of this client.
 *
 * <p>Two lifecycle modes, chosen by {@link ConnectionOptions}:
 *
 * <ul>
 *   <li><b>Private service.</b> The default. The connection starts {@code sysml-grpc} as a child of
 *       this JVM on a port the kernel chose, and every connection made by the classloader that
 *       loaded this class shares that child, so they share its parse cache. The last connection to
 *       close stops it. The child cannot be orphaned: see {@link PrivateService}.
 *   <li><b>External service.</b> Opt in by naming a host and port, or by setting
 *       {@code $OPENSYSML_SERVICE} to {@code host:port}. Closing such a connection leaves the
 *       service running.
 * </ul>
 *
 * <p>Thread-safe: a connection may be shared by any number of threads, since a call carries all of
 * its own state and {@code java.net.http} is itself thread-safe. {@link #close()} is idempotent.
 *
 * <p>Every failure is unchecked; see {@link OpenSysMLException} for the one exception rule.
 *
 * <pre>{@code
 * try (Connection connection = Connection.open()) {
 *   Model model = connection.load(Path.of("vehicle.sysml"));
 *   Value mass = model.evalInContext("mass", "Demo::Vehicle");
 * }
 * }</pre>
 */
public final class Connection implements AutoCloseable {
  private static final String NAME_OPTIONS = "options";
  private static final String NAME_CONTENT = "content";

  private final ConnectTransport transport;
  private final String address;
  private final Optional<PrivateService> ownedService;
  private final Capabilities capabilities;
  private final AtomicBoolean closed = new AtomicBoolean();
  private boolean answeredServerInfo = true;

  private Connection(
      ConnectTransport transport, String address, Optional<PrivateService> ownedService) {
    this.transport = transport;
    this.address = address;
    this.ownedService = ownedService;
    this.capabilities = readCapabilities();
  }

  /** A connection standing on these capabilities, for tests of what a call checks first. */
  Connection(ConnectTransport transport, Capabilities capabilities) {
    this.transport = transport;
    this.address = "";
    this.ownedService = Optional.empty();
    this.capabilities = capabilities;
  }

  /**
   * Opens a connection with the default options: a private service, protobuf bodies.
   *
   * @return an open connection the caller must close
   */
  public static Connection open() {
    return open(ConnectionOptions.defaults());
  }

  /**
   * Opens a connection.
   *
   * @param options how to reach the service
   * @return an open connection the caller must close
   * @throws ServiceStartException if no service was named and none could be started
   * @throws TransportException if the service could not be reached
   */
  public static Connection open(ConnectionOptions options) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    Optional<String> external = externalAddress(options);
    if (external.isPresent()) {
      String address = external.get();
      ConnectTransport transport =
          new ConnectTransport(address, options.encoding(), options.requestTimeout());
      Connection connection;
      try {
        connection = new Connection(transport, address, Optional.empty());
      } catch (RuntimeException e) {
        transport.close();
        throw e;
      }
      try {
        connection.requireOptions(options);
      } catch (RuntimeException e) {
        connection.close();
        throw e;
      }
      return connection;
    }
    if (!options.autoStart()) {
      throw new ServiceStartException(
          "no service was named and autoStart is false; name one with "
              + "ConnectionOptions.service(host, port) or $"
              + ConnectionOptions.SERVICE_ENV);
    }
    PrivateService service = ServiceRegistry.acquire(options);
    ConnectTransport transport =
        new ConnectTransport(service.address(), options.encoding(), options.requestTimeout());
    Connection connection;
    try {
      connection = new Connection(transport, service.address(), Optional.of(service));
    } catch (RuntimeException e) {
      transport.close();
      ServiceRegistry.release(service);
      throw e;
    }
    try {
      connection.requireOptions(options);
    } catch (RuntimeException e) {
      connection.close();
      throw e;
    }
    return connection;
  }

  private void requireOptions(ConnectionOptions options) {
    requiredRelease(options).ifPresent(this::requireRelease);
    new TreeSet<>(options.requiredCapabilities()).forEach(capabilities::require);
  }

  private static Optional<String> requiredRelease(ConnectionOptions options) {
    Optional<String> asked = BinaryDownloader.versionAskedFor(options);
    if (asked.isEmpty() || !asked.get().equals("latest")) {
      return asked;
    }
    try {
      return Optional.of(
          BinaryDownloader.production()
              .resolveLatestVersion(BinaryDownloader.githubRepo(options)));
    } catch (ServiceStartException e) {
      return Optional.empty();
    }
  }

  private void requireRelease(String release) {
    String reason;
    if (!answeredServerInfo) {
      reason =
          "it did not answer GetServerInfo, so it cannot be shown to be the "
              + release
              + " that was asked for";
    } else if (!capabilities.serviceVersion().equals(release)) {
      reason =
          "it reports version "
              + (capabilities.serviceVersion().isEmpty()
                  ? "unknown"
                  : capabilities.serviceVersion())
              + ", but "
              + release
              + " was asked for";
    } else {
      return;
    }
    String remedy =
        ownedService.isPresent()
            ? "the binary this client started, "
                + ownedService.get().binary()
                + ", is not "
                + release
                + ": make that release available (its download is cached under"
                + " ~/.opensysml/bin), or accept what is installed by naming no"
                + " ConnectionOptions.downloadVersion and unsetting $"
                + ConnectionOptions.VERSION_ENV
            : "stop the service listening on "
                + address
                + " yourself and let this client start a "
                + release
                + " one, or accept what is running by naming no"
                + " ConnectionOptions.downloadVersion and unsetting $"
                + ConnectionOptions.VERSION_ENV;
    throw new StaleServiceException(address, reason, remedy, capabilities.serviceVersion());
  }

  /**
   * Stops every private service this classloader's connections share, whether or not connections
   * still hold them. For a host application unloading the client, where nothing else will run.
   *
   * @return how many services were stopped
   */
  public static int stopSharedServices() {
    return ServiceRegistry.stopAll();
  }

  /**
   * What the service says it can do, read once when the connection opened.
   *
   * <p>Negotiate on these names. The service does not answer {@code UNIMPLEMENTED} for a capability
   * it lacks, so a call is not a test for one.
   *
   * @return the capabilities and the service's version string
   */
  public Capabilities capabilities() {
    return capabilities;
  }

  /**
   * The analysis engines the service answers with, in name order: what each answers, how strongly
   * it can, and whether it can run here. Name one with {@link Model#withEngine(String)}.
   *
   * @return the engines
   * @throws CapabilityException if the service does not advertise {@code engines}
   */
  public List<EngineInfo> listEngines() {
    checkOpen();
    capabilities.require(Capabilities.ENGINES);
    ListEnginesResponse response =
        call(
            "ListEngines",
            ListEnginesRequest.getDefaultInstance(),
            ListEnginesResponse.getDefaultInstance());
    return Protos.engines(response.getEnginesList());
  }

  /**
   * The {@code host:port} this connection talks to.
   *
   * @return the address
   */
  public String address() {
    return address;
  }

  /**
   * Whether closing this connection can stop the service.
   *
   * @return {@code true} for a private service, {@code false} for an external one
   */
  public boolean ownsService() {
    return ownedService.isPresent();
  }

  /**
   * Parses a file the service can read, and caches it there.
   *
   * <p>The path is interpreted by the service, which for a private child is this machine. It is not
   * read here, so a path that is not there is the service's {@link StatusCode#NOT_FOUND}.
   *
   * @param file the source to parse
   * @return the parsed model
   * @throws ModelException if the source could not be parsed at all
   * @throws ServiceException if the service could not read it
   */
  public Model load(Path file) {
    Objects.requireNonNull(file, "file");
    return parsed(
        ParseFileRequest.newBuilder().setFilePath(file.toString()).build(), List.of(file.toString()));
  }

  /**
   * Parses a file, with options.
   *
   * @param file the source to parse
   * @param options notation and how strictly to judge it
   * @return the parsed model
   */
  public Model load(Path file, ParseOptions options) {
    Objects.requireNonNull(file, "file");
    return parsed(
        request(options).setFilePath(file.toString()).build(), List.of(file.toString()));
  }

  /**
   * Parses notation given inline.
   *
   * @param content SysML notation
   * @return the parsed model
   * @throws ModelException if the source could not be parsed at all
   */
  public Model parse(String content) {
    Objects.requireNonNull(content, NAME_CONTENT);
    return parsed(ParseFileRequest.newBuilder().setContent(content).build(), List.of());
  }

  /**
   * Parses notation given inline, with options.
   *
   * @param content notation in {@code options}' language
   * @param options notation and how strictly to judge it
   * @return the parsed model
   */
  public Model parse(String content, ParseOptions options) {
    Objects.requireNonNull(content, NAME_CONTENT);
    return parsed(request(options).setContent(content).build(), List.of());
  }

  /**
   * Parses several source documents as one model, which every later call then names by one hash.
   *
   * <p>Documents are parsed in the order given, each {@link SourceDocument} naming a file or
   * inline notation and, for inline notation, the name other documents import it by. A document's
   * own {@link SourceDocument#language()} is sent when it names one; it is not inferred from the
   * options.
   *
   * @param documents the documents to parse, in order
   * @return the parsed model, its {@link Model#roots()} naming one root per document
   * @throws ModelException if the documents could not be parsed at all
   * @throws ServiceException if the request names no documents or two documents by one name
   * @throws CapabilityException if the service does not advertise {@code parse_sources}, or an
   *     inline document names a language and it does not advertise {@code inline_language}
   */
  public Model parseSources(List<SourceDocument> documents) {
    return parseSources(documents, ParseOptions.defaults());
  }

  /**
   * Parses several source documents as one model, with options.
   *
   * @param documents the documents to parse, in order
   * @param options how strictly to judge the notation; its language does not apply to the
   *     documents, which name their own
   * @return the parsed model, its {@link Model#roots()} naming one root per document
   * @throws ModelException if the documents could not be parsed at all
   * @throws ServiceException if the request names no documents or two documents by one name
   * @throws CapabilityException if the service does not advertise {@code parse_sources}, an inline
   *     document names a language and it does not advertise {@code inline_language}, or strict
   *     conformance is asked and it does not advertise {@code strict_conformance}
   */
  public Model parseSources(List<SourceDocument> documents, ParseOptions options) {
    Objects.requireNonNull(documents, "documents");
    Objects.requireNonNull(options, NAME_OPTIONS);
    capabilities.require(Capabilities.PARSE_SOURCES);
    if (options.strictConformance()) {
      capabilities.require(Capabilities.STRICT_CONFORMANCE);
    }
    ParseSourcesRequest.Builder request =
        ParseSourcesRequest.newBuilder().setStrictConformance(options.strictConformance());
    List<String> names = new java.util.ArrayList<>(documents.size());
    for (SourceDocument document : documents) {
      if (document.language().isPresent()) {
        capabilities.require(Capabilities.INLINE_LANGUAGE);
      }
      request.addDocuments(Protos.proto(document));
      names.add(document.file().map(Path::toString).orElseGet(() -> document.name().orElse("")));
    }
    ParseSourcesResponse response =
        call("ParseSources", request.build(), ParseSourcesResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    List<Symbol> roots = response.getRootsList().stream().map(Protos::symbol).toList();
    return new Model(this, response.getModelHash(), roots, diagnostics, names);
  }

  /**
   * Converts notation given inline into another format.
   *
   * <p>A SysML v1 model ({@code "xmi"}, {@code "uml"} or {@code "mdzip"}) is refused: it is
   * migrated, not converted, and {@link #migrate(byte[], String, MigrationOptions)} accounts for
   * every element on the way.
   *
   * @param content the notation to convert, which must name its format in {@code options} since
   *     inline content has no extension to infer it from
   * @param toFormat the format to write, named as the service names formats ({@code "sysml"},
   *     {@code "kerml"}, {@code "ttl"}, {@code "api-json"}, …)
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convert(String content, String toFormat) {
    return convert(content, toFormat, ConversionOptions.defaults());
  }

  /**
   * Converts notation given inline into another format, with options.
   *
   * @param content the notation to convert
   * @param toFormat the format to write
   * @param options the source format and whether unreadable notation is written back anyway
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the source format could not be inferred or the request was
   *     refused, {@link StatusCode#INVALID_ARGUMENT} naming {@code migrate} when the source is a
   *     SysML v1 model
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convert(String content, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(content, NAME_CONTENT);
    return converted(ConvertRequest.newBuilder().setContent(content), toFormat, options);
  }

  /**
   * Converts a file into another format, its format inferred from its extension.
   *
   * <p>A SysML v1 model — a {@code .xmi}, {@code .uml} or {@code .mdzip} file — is refused: it is
   * migrated, not converted, and {@link #migrateFile(Path, String, MigrationOptions)} accounts for
   * every element on the way.
   *
   * @param file the source to convert
   * @param toFormat the format to write
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the service could not read the file
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convertFile(Path file, String toFormat) {
    return convertFile(file, toFormat, ConversionOptions.defaults());
  }

  /**
   * Converts a file into another format, with options.
   *
   * @param file the source to convert
   * @param toFormat the format to write
   * @param options the source format, which overrides the file's extension, and whether unreadable
   *     notation is written back anyway
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the service could not read the file, or {@link
   *     StatusCode#INVALID_ARGUMENT} naming {@code migrateFile} when the file is a SysML v1 model
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convertFile(Path file, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(file, "file");
    Objects.requireNonNull(options, NAME_OPTIONS);
    if (options.fromFormat().isEmpty() && isV1File(file)) {
      throw notMigrated(file.toString(), "call migrateFile with the same file");
    }
    return converted(
        ConvertRequest.newBuilder().setFilePath(file.toString()), toFormat, options);
  }

  /**
   * Migrates a SysML v1 model given inline to SysML v2, accounting for every element.
   *
   * <p>Migration is ledgered, not lossless: every element lands in the {@link Migration#report()}
   * as mapped, approximated, unmapped or skipped. The summary and counts come back with every
   * migration; {@link MigrationOptions#withReport(boolean)} asks for every element's verdict. This
   * is what {@code sysml Model.mdzip -migrate sysml} does.
   *
   * @param content the v1 model's bytes — bytes, since a {@code .mdzip} archive is binary — which
   *     must name their form in {@code options} since inline content has no extension
   * @param toFormat the format to write, named as the service names formats ({@code "sysml"},
   *     {@code "kerml"}, {@code "ttl"}, {@code "api-json"}, …)
   * @param options the v1 form, and what the migration is asked for beyond the model
   * @return the migration, carrying the model, its report and what else was asked for
   * @throws MigrationException if the v1 model could not be migrated at all; an element without a
   *     v2 form is reported, not thrown
   * @throws ServiceException if the request was refused: {@link StatusCode#INVALID_ARGUMENT} when
   *     {@code options} name a v2 format, which is converted, not migrated, or no form at all
   * @throws CapabilityException if the service does not advertise {@code migrate}
   */
  public Migration migrate(byte[] content, String toFormat, MigrationOptions options) {
    Objects.requireNonNull(content, NAME_CONTENT);
    return migrated(
        MigrateRequest.newBuilder().setContent(ByteString.copyFrom(content)),
        "inline content",
        toFormat,
        options);
  }

  /**
   * Migrates a SysML v1 model the service reads from a file, its form inferred from its extension.
   *
   * @param file the {@code .xmi}, {@code .uml} or {@code .mdzip} file to migrate
   * @param toFormat the format to write
   * @return the migration, carrying the model and its report
   * @throws MigrationException if the v1 model could not be migrated at all; an element without a
   *     v2 form is reported, not thrown
   * @throws ServiceException if the service could not read the file, or {@link
   *     StatusCode#INVALID_ARGUMENT} when its extension names a v2 format, which is converted, not
   *     migrated
   * @throws CapabilityException if the service does not advertise {@code migrate}
   * @see #migrateFile(Path, String, MigrationOptions)
   */
  public Migration migrateFile(Path file, String toFormat) {
    return migrateFile(file, toFormat, MigrationOptions.defaults());
  }

  /**
   * Migrates a SysML v1 model the service reads from a file, with options.
   *
   * <p>Migration is ledgered, not lossless: every element lands in the {@link Migration#report()}
   * as mapped, approximated, unmapped or skipped. This is what {@code sysml Model.mdzip -migrate
   * sysml -migration-report Model.report.txt} does, with {@link
   * MigrationOptions#withReport(boolean)} standing for the report flag.
   *
   * @param file the file to migrate
   * @param toFormat the format to write
   * @param options the v1 form, which overrides the file's extension, and what the migration is
   *     asked for beyond the model
   * @return the migration, carrying the model, its report and what else was asked for
   * @throws MigrationException if the v1 model could not be migrated at all; an element without a
   *     v2 form is reported, not thrown
   * @throws ServiceException if the service could not read the file, or {@link
   *     StatusCode#INVALID_ARGUMENT} when the source is a v2 format, which is converted, not
   *     migrated
   * @throws CapabilityException if the service does not advertise {@code migrate}
   */
  public Migration migrateFile(Path file, String toFormat, MigrationOptions options) {
    Objects.requireNonNull(file, "file");
    return migrated(
        MigrateRequest.newBuilder().setFilePath(file.toString()), file.toString(), toFormat, options);
  }

  /**
   * A handle on a model the service parsed already, named by its hash.
   *
   * <p>For a host that kept a hash across connections. Nothing is called here, so a hash the
   * service does not hold is reported by the first call made on the model, not by this one, and the
   * handle carries neither a root nor the diagnostics of that parse.
   *
   * @param modelHash a hash a parse returned
   * @return a handle on that model
   */
  public Model model(String modelHash) {
    checkOpen();
    Objects.requireNonNull(modelHash, "modelHash");
    if (modelHash.isBlank()) {
      throw new IllegalArgumentException("modelHash must not be blank");
    }
    return new Model(this, modelHash, List.of(), List.of());
  }

  /**
   * Closes the connection, and the private service when this was the last connection holding it.
   * Idempotent; an external service is never stopped.
   */
  @Override
  public void close() {
    if (!closed.compareAndSet(false, true)) {
      return;
    }
    try {
      transport.close();
    } finally {
      ownedService.ifPresent(ServiceRegistry::release);
    }
  }

  /** The private service this connection holds, for a lifecycle test. */
  Optional<PrivateService> ownedService() {
    return ownedService;
  }

  /** Calls the service. Package-private: generated messages are not part of the public surface. */
  <T extends Message> T call(String method, Message request, T responseDefault) {
    checkOpen();
    return transport.call(method, request, responseDefault);
  }

  private Model parsed(ParseFileRequest request, List<String> documents) {
    if (request.getStrictConformance()) {
      capabilities.require(Capabilities.STRICT_CONFORMANCE);
    }
    ParseFileResponse response = call("ParseFile", request, ParseFileResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    List<Symbol> roots =
        response.hasRoot() ? List.of(Protos.symbol(response.getRoot())) : List.of();
    return new Model(this, response.getModelHash(), roots, diagnostics, documents);
  }

  private Conversion converted(
      ConvertRequest.Builder request, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(toFormat, "toFormat");
    Objects.requireNonNull(options, NAME_OPTIONS);
    if (options.fromFormat().filter(Connection::isV1Format).isPresent()) {
      String name = request.hasFilePath() ? request.getFilePath() : "the source";
      throw notMigrated(name, "call migrate with the same source");
    }
    capabilities.require(Capabilities.CONVERT);
    request.setToFormat(toFormat).setTolerateSyntaxErrors(options.tolerateSyntaxErrors());
    options.fromFormat().ifPresent(request::setFromFormat);
    options.idForm().ifPresent(request::setIdForm);
    ConvertResponse response =
        call("Convert", request.build(), ConvertResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ConversionException(response.getError(), diagnostics);
    }
    return Protos.conversion(response);
  }

  private Migration migrated(
      MigrateRequest.Builder request, String name, String toFormat, MigrationOptions options) {
    Objects.requireNonNull(toFormat, "toFormat");
    Objects.requireNonNull(options, NAME_OPTIONS);
    options
        .fromFormat()
        .filter(from -> !isV1Format(from))
        .ifPresent(
            from -> {
              throw new ServiceException(
                  StatusCode.INVALID_ARGUMENT,
                  name
                      + " is "
                      + from
                      + " input, which is converted, not migrated: only a SysML v1 model (xmi,"
                      + " uml or mdzip) is migrated; call convert with the same source");
            });
    capabilities.require(Capabilities.MIGRATE);
    request
        .setToFormat(toFormat)
        .setReport(options.report())
        .setResults(options.results())
        .setImageBaseUrl(options.imageBaseUrl())
        .setStrict(options.strict());
    options.fromFormat().ifPresent(request::setFromFormat);
    options.layoutFile().ifPresent(layout -> request.setLayoutPath(layout.toString()));
    options.layoutContent().ifPresent(request::setLayoutContent);
    MigrateResponse response =
        call("Migrate", request.build(), MigrateResponse.getDefaultInstance());
    if (!response.getError().isEmpty()) {
      throw new MigrationException(response.getError());
    }
    return Protos.migration(response);
  }

  /** Why a v1 model is refused by a conversion, in the words every surface uses. */
  static final String MIGRATED_NOT_CONVERTED =
      "is a SysML v1 model, which is migrated, not converted: every element is mapped,"
          + " approximated or left unmapped and reported element by element";

  private static ServiceException notMigrated(String name, String remedy) {
    return new ServiceException(
        StatusCode.INVALID_ARGUMENT, name + " " + MIGRATED_NOT_CONVERTED + "; " + remedy);
  }

  private static boolean isV1Format(String format) {
    return switch (format.strip().toLowerCase(Locale.ROOT)) {
      case "xmi", "uml", "mdzip" -> true;
      default -> false;
    };
  }

  private static boolean isV1File(Path file) {
    String name = file.getFileName() == null ? "" : file.getFileName().toString();
    int dot = name.lastIndexOf('.');
    return dot >= 0 && isV1Format(name.substring(dot + 1).toLowerCase(Locale.ROOT));
  }

  private static ParseFileRequest.Builder request(ParseOptions options) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    return ParseFileRequest.newBuilder()
        .setLanguage(options.language().wireName())
        .setStrictConformance(options.strictConformance());
  }

  private Capabilities readCapabilities() {
    ServerInfoResponse info;
    try {
      info =
          transport.call(
              "GetServerInfo",
              ServerInfoRequest.getDefaultInstance(),
              ServerInfoResponse.getDefaultInstance());
    } catch (ServiceException e) {
      if (e.status() != StatusCode.UNIMPLEMENTED) {
        throw e;
      }
      answeredServerInfo = false;
      return new Capabilities("", Set.of());
    }
    Set<String> names = new LinkedHashSet<>(info.getCapabilitiesList());
    return new Capabilities(info.getVersion(), names);
  }

  private static Optional<String> externalAddress(ConnectionOptions options) {
    Optional<String> host = options.host();
    if (host.isPresent()) {
      return Optional.of(host.get() + ":" + options.port());
    }
    String named = System.getenv(ConnectionOptions.SERVICE_ENV);
    if (named == null || named.isBlank()) {
      return Optional.empty();
    }
    String address = named.trim();
    int separator = address.lastIndexOf(':');
    if (separator <= 0 || separator == address.length() - 1) {
      throw new IllegalArgumentException(
          "$" + ConnectionOptions.SERVICE_ENV + " must be host:port, not " + named);
    }
    try {
      Integer.parseInt(address.substring(separator + 1));
    } catch (NumberFormatException e) {
      throw new IllegalArgumentException(
          "$" + ConnectionOptions.SERVICE_ENV + " must be host:port, not " + named, e);
    }
    return Optional.of(address);
  }

  private void checkOpen() {
    if (closed.get()) {
      throw new IllegalStateException("this connection is closed");
    }
  }
}

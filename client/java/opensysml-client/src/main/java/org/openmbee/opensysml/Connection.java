package org.openmbee.opensysml;

import com.google.protobuf.Message;
import org.openmbee.opensysml.internal.ConnectTransport;
import org.openmbee.opensysml.internal.PrivateService;
import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.internal.ServiceRegistry;
import org.openmbee.opensysml.proto.ConvertRequest;
import org.openmbee.opensysml.proto.ConvertResponse;
import org.openmbee.opensysml.proto.ListEnginesRequest;
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
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
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

  private final ConnectTransport transport;
  private final String address;
  private final Optional<PrivateService> ownedService;
  private final Capabilities capabilities;
  private final AtomicBoolean closed = new AtomicBoolean();

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
    Objects.requireNonNull(options, "options");
    Optional<String> external = externalAddress(options);
    if (external.isPresent()) {
      String address = external.get();
      ConnectTransport transport =
          new ConnectTransport(address, options.encoding(), options.requestTimeout());
      try {
        return new Connection(transport, address, Optional.empty());
      } catch (RuntimeException e) {
        transport.close();
        throw e;
      }
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
    try {
      return new Connection(transport, service.address(), Optional.of(service));
    } catch (RuntimeException e) {
      transport.close();
      ServiceRegistry.release(service);
      throw e;
    }
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
    return parsed(ParseFileRequest.newBuilder().setFilePath(file.toString()).build());
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
    return parsed(request(options).setFilePath(file.toString()).build());
  }

  /**
   * Parses notation given inline.
   *
   * @param content SysML notation
   * @return the parsed model
   * @throws ModelException if the source could not be parsed at all
   */
  public Model parse(String content) {
    Objects.requireNonNull(content, "content");
    return parsed(ParseFileRequest.newBuilder().setContent(content).build());
  }

  /**
   * Parses notation given inline, with options.
   *
   * @param content notation in {@code options}' language
   * @param options notation and how strictly to judge it
   * @return the parsed model
   */
  public Model parse(String content, ParseOptions options) {
    Objects.requireNonNull(content, "content");
    return parsed(request(options).setContent(content).build());
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
    Objects.requireNonNull(options, "options");
    capabilities.require(Capabilities.PARSE_SOURCES);
    if (options.strictConformance()) {
      capabilities.require(Capabilities.STRICT_CONFORMANCE);
    }
    ParseSourcesRequest.Builder request =
        ParseSourcesRequest.newBuilder().setStrictConformance(options.strictConformance());
    for (SourceDocument document : documents) {
      if (document.language().isPresent()) {
        capabilities.require(Capabilities.INLINE_LANGUAGE);
      }
      request.addDocuments(Protos.proto(document));
    }
    ParseSourcesResponse response =
        call("ParseSources", request.build(), ParseSourcesResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    List<Symbol> roots = new java.util.ArrayList<>(response.getRootsCount());
    for (org.openmbee.opensysml.proto.SymbolInfo root : response.getRootsList()) {
      roots.add(Protos.symbol(root));
    }
    return new Model(this, response.getModelHash(), roots, diagnostics);
  }

  /**
   * Converts notation given inline into another format.
   *
   * @param content the notation to convert, which must name its format in {@code options} since
   *     inline content has no extension to infer it from
   * @param toFormat the format to write, named as the service names formats ({@code "sysml"},
   *     {@code "kerml"}, {@code "ttl"}, …)
   * @return the conversion, carrying the text and the formats used
   * @throws ModelException if the conversion failed; its diagnostics say why
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
   * @throws ModelException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the source format could not be inferred or the request was
   *     refused
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convert(String content, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(content, "content");
    return converted(ConvertRequest.newBuilder().setContent(content), toFormat, options);
  }

  /**
   * Converts a file into another format, its format inferred from its extension.
   *
   * @param file the source to convert
   * @param toFormat the format to write
   * @return the conversion, carrying the text and the formats used
   * @throws ModelException if the conversion failed; its diagnostics say why
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
   * @throws ModelException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the service could not read the file
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convertFile(Path file, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(file, "file");
    return converted(
        ConvertRequest.newBuilder().setFilePath(file.toString()), toFormat, options);
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

  private Model parsed(ParseFileRequest request) {
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
    return new Model(this, response.getModelHash(), roots, diagnostics);
  }

  private Conversion converted(
      ConvertRequest.Builder request, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(toFormat, "toFormat");
    Objects.requireNonNull(options, "options");
    capabilities.require(Capabilities.CONVERT);
    request.setToFormat(toFormat).setTolerateSyntaxErrors(options.tolerateSyntaxErrors());
    options.fromFormat().ifPresent(request::setFromFormat);
    ConvertResponse response =
        call("Convert", request.build(), ConvertResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    return Protos.conversion(response);
  }

  private static ParseFileRequest.Builder request(ParseOptions options) {
    Objects.requireNonNull(options, "options");
    return ParseFileRequest.newBuilder()
        .setLanguage(options.language().wireName())
        .setStrictConformance(options.strictConformance());
  }

  private Capabilities readCapabilities() {
    ServerInfoResponse info =
        transport.call(
            "GetServerInfo",
            ServerInfoRequest.getDefaultInstance(),
            ServerInfoResponse.getDefaultInstance());
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

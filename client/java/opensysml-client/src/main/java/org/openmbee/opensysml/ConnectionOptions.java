package org.openmbee.opensysml;

import java.nio.file.Path;
import java.time.Duration;
import java.util.Objects;
import java.util.Optional;

/**
 * How a {@link Connection} reaches a service: private child by default, an external service when
 * one is named.
 *
 * <p>Immutable; build one with {@link #builder()}.
 */
public final class ConnectionOptions {

  /** Names an external service as {@code host:port}, as the Python client reads it too. */
  public static final String SERVICE_ENV = "OPENSYSML_SERVICE";

  /** Names the service binary a private child is started from. */
  public static final String BINARY_ENV = "OPENSYSML_GRPC_BINARY";

  /** Names the release to download when no binary is installed, or {@code latest}. */
  public static final String VERSION_ENV = "OPENSYSML_GRPC_VERSION";

  /** Names the repository releases are downloaded from, as {@code owner/repo}. */
  public static final String REPO_ENV = "OPENSYSML_GITHUB_REPO";

  /**
   * Set to the repository whose unpinned downloads may be accepted ({@code 1} for any), which is
   * same-origin trust: the checksum then comes from whoever served the binary.
   */
  public static final String ALLOW_UNPINNED_ENV = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD";

  private final Optional<String> host;
  private final int port;
  private final boolean autoStart;
  private final Optional<Path> binaryPath;
  private final Optional<String> expectedBinarySha256;
  private final Optional<String> downloadVersion;
  private final Optional<String> githubRepo;
  private final boolean allowUnpinnedDownload;
  private final Encoding encoding;
  private final Duration requestTimeout;
  private final Duration startupTimeout;
  private final boolean isolatedService;

  private ConnectionOptions(Builder builder) {
    this.host = Optional.ofNullable(builder.host);
    this.port = builder.port;
    this.autoStart = builder.autoStart;
    this.binaryPath = Optional.ofNullable(builder.binaryPath);
    this.expectedBinarySha256 = Optional.ofNullable(builder.expectedBinarySha256);
    this.downloadVersion = Optional.ofNullable(builder.downloadVersion);
    this.githubRepo = Optional.ofNullable(builder.githubRepo);
    this.allowUnpinnedDownload = builder.allowUnpinnedDownload;
    this.encoding = builder.encoding;
    this.requestTimeout = builder.requestTimeout;
    this.startupTimeout = builder.startupTimeout;
    this.isolatedService = builder.isolatedService;
  }

  /**
   * Options with every default: a private child, protobuf bodies.
   *
   * @return the default options
   */
  public static ConnectionOptions defaults() {
    return builder().build();
  }

  /**
   * A builder for options.
   *
   * @return a new builder
   */
  public static Builder builder() {
    return new Builder();
  }

  /**
   * Host of an external service, absent when the connection starts a private child.
   *
   * @return the host
   */
  public Optional<String> host() {
    return host;
  }

  /**
   * Port of an external service, or 0 when none is named.
   *
   * @return the port
   */
  public int port() {
    return port;
  }

  /**
   * Whether a private child may be started when no external service is named.
   *
   * @return {@code true} when starting one is allowed
   */
  public boolean autoStart() {
    return autoStart;
  }

  /**
   * The service binary to start, absent when it is discovered instead.
   *
   * @return the binary path
   */
  public Optional<Path> binaryPath() {
    return binaryPath;
  }

  /**
   * SHA-256 the binary must have before it is executed, absent when none is required.
   *
   * @return the expected hex digest
   */
  public Optional<String> expectedBinarySha256() {
    return expectedBinarySha256;
  }

  /**
   * The release a missing binary is downloaded from, absent when {@code $OPENSYSML_GRPC_VERSION}
   * decides and nothing is downloaded without it.
   *
   * @return a release tag, or {@code latest}
   */
  public Optional<String> downloadVersion() {
    return downloadVersion;
  }

  /**
   * The repository releases are downloaded from, absent when {@code $OPENSYSML_GITHUB_REPO} or the
   * default decides.
   *
   * @return owner/repo
   */
  public Optional<String> githubRepo() {
    return githubRepo;
  }

  /**
   * Whether a release this client pins no digest for, and whose checksum manifest carries no
   * signature that verifies, may be installed against the checksum served beside it.
   *
   * @return {@code true} when same-origin trust was accepted here
   */
  public boolean allowUnpinnedDownload() {
    return allowUnpinnedDownload;
  }

  /**
   * The body encoding calls use.
   *
   * @return the encoding
   */
  public Encoding encoding() {
    return encoding;
  }

  /**
   * How long one call may take.
   *
   * @return the per-call timeout
   */
  public Duration requestTimeout() {
    return requestTimeout;
  }

  /**
   * How long a private child has to report its address.
   *
   * @return the startup timeout
   */
  public Duration startupTimeout() {
    return startupTimeout;
  }

  /**
   * Whether this connection gets a child of its own rather than the shared one.
   *
   * @return {@code true} when the child is not shared
   */
  public boolean isolatedService() {
    return isolatedService;
  }

  /** Builds {@link ConnectionOptions}. */
  public static final class Builder {

    private String host;
    private int port;
    private boolean autoStart = true;
    private Path binaryPath;
    private String expectedBinarySha256;
    private String downloadVersion;
    private String githubRepo;
    private boolean allowUnpinnedDownload;
    private Encoding encoding = Encoding.PROTOBUF;
    private Duration requestTimeout = Duration.ofSeconds(60);
    private Duration startupTimeout = Duration.ofSeconds(30);
    private boolean isolatedService;

    private Builder() {}

    /**
     * Connects to a service this client did not start, and leaves it running on close.
     *
     * @param host host it listens on
     * @param port port it listens on
     * @return this builder
     */
    public Builder service(String host, int port) {
      this.host = Objects.requireNonNull(host, "host");
      if (port <= 0 || port > 65535) {
        throw new IllegalArgumentException("port out of range: " + port);
      }
      this.port = port;
      return this;
    }

    /**
     * Whether a private child may be started. False without a named service is an error at
     * {@link Connection#open(ConnectionOptions)}, rather than a surprise child.
     *
     * @param autoStart {@code false} to refuse starting one
     * @return this builder
     */
    public Builder autoStart(boolean autoStart) {
      this.autoStart = autoStart;
      return this;
    }

    /**
     * The service binary to start a private child from.
     *
     * @param binaryPath path of an executable {@code sysml-grpc}
     * @return this builder
     */
    public Builder binaryPath(Path binaryPath) {
      this.binaryPath = Objects.requireNonNull(binaryPath, "binaryPath");
      return this;
    }

    /**
     * Requires the binary to have this SHA-256 before it is executed.
     *
     * @param hexDigest lower- or upper-case hex SHA-256
     * @return this builder
     */
    public Builder expectedBinarySha256(String hexDigest) {
      Objects.requireNonNull(hexDigest, "hexDigest");
      if (!hexDigest.matches("(?i)[0-9a-f]{64}")) {
        throw new IllegalArgumentException("not a SHA-256 hex digest: " + hexDigest);
      }
      this.expectedBinarySha256 = hexDigest;
      return this;
    }

    /**
     * The release to download when no binary is installed, or when the cached one is another
     * release. Without one, {@code $OPENSYSML_GRPC_VERSION} decides, and nothing is downloaded
     * without either.
     *
     * @param downloadVersion a release tag (e.g. {@code v0.3.0}), or {@code latest}
     * @return this builder
     */
    public Builder downloadVersion(String downloadVersion) {
      Objects.requireNonNull(downloadVersion, "downloadVersion");
      if (downloadVersion.isBlank()) {
        throw new IllegalArgumentException("downloadVersion must name a release or 'latest'");
      }
      this.downloadVersion = downloadVersion.trim();
      return this;
    }

    /**
     * The repository releases are downloaded from, for a fork publishing its own builds. Without
     * one, {@code $OPENSYSML_GITHUB_REPO} decides, then {@code Open-MBEE/OpenSysML}.
     *
     * @param githubRepo owner/repo
     * @return this builder
     */
    public Builder githubRepo(String githubRepo) {
      Objects.requireNonNull(githubRepo, "githubRepo");
      // The two names end up as path segments of a release URL, so nothing that can leave them.
      if (!githubRepo.matches("[A-Za-z0-9._-]{1,128}/[A-Za-z0-9._-]{1,128}")
          || githubRepo.matches("\\.+/.*|.*/\\.+")) {
        throw new IllegalArgumentException("not an owner/repo: " + githubRepo);
      }
      this.githubRepo = githubRepo;
      return this;
    }

    /**
     * Accepts the checksum a release serves beside a binary this client pins no digest for and
     * whose checksum manifest carries no signature that verifies. That is same-origin trust: it
     * detects corruption, not a compromised release. {@code $OPENSYSML_ALLOW_UNPINNED_DOWNLOAD}
     * grants the same thing from the environment.
     *
     * @param allowUnpinnedDownload {@code true} to accept it
     * @return this builder
     */
    public Builder allowUnpinnedDownload(boolean allowUnpinnedDownload) {
      this.allowUnpinnedDownload = allowUnpinnedDownload;
      return this;
    }

    /**
     * The body encoding to use. Protobuf unless there is a reason.
     *
     * @param encoding the encoding
     * @return this builder
     */
    public Builder encoding(Encoding encoding) {
      this.encoding = Objects.requireNonNull(encoding, "encoding");
      return this;
    }

    /**
     * How long one call may take.
     *
     * @param requestTimeout a positive duration
     * @return this builder
     */
    public Builder requestTimeout(Duration requestTimeout) {
      this.requestTimeout = requirePositive(requestTimeout, "requestTimeout");
      return this;
    }

    /**
     * How long a private child has to report its address.
     *
     * @param startupTimeout a positive duration
     * @return this builder
     */
    public Builder startupTimeout(Duration startupTimeout) {
      this.startupTimeout = requirePositive(startupTimeout, "startupTimeout");
      return this;
    }

    /**
     * Gives this connection a child of its own, for a host that must not let one tenant's models
     * sit in another tenant's parse cache.
     *
     * @param isolatedService {@code true} to start an unshared child
     * @return this builder
     */
    public Builder isolatedService(boolean isolatedService) {
      this.isolatedService = isolatedService;
      return this;
    }

    /**
     * Builds the options.
     *
     * @return immutable options
     */
    public ConnectionOptions build() {
      return new ConnectionOptions(this);
    }

    private static Duration requirePositive(Duration duration, String name) {
      Objects.requireNonNull(duration, name);
      if (duration.isZero() || duration.isNegative()) {
        throw new IllegalArgumentException(name + " must be positive: " + duration);
      }
      return duration;
    }
  }
}

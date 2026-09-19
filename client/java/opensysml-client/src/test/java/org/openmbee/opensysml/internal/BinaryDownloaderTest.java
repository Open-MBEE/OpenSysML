package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.ChecksumMismatchException;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.ManifestSignatureException;
import org.openmbee.opensysml.ServiceStartException;
import org.openmbee.opensysml.UnpinnedReleaseException;
import java.io.IOException;
import java.net.URISyntaxException;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.function.UnaryOperator;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** What a download is verified against, and what a failed one leaves behind. */
class BinaryDownloaderTest {

  private static final ConnectionOptions OPTIONS = ConnectionOptions.defaults();
  private static final String REPO = "Open-MBEE/OpenSysML";
  private static final String VERSION = "v9.9.9";
  private static final String ASSET = "sysml-grpc-linux-amd64";
  private static final byte[] BINARY = "#!/bin/sh\nexit 0\n".getBytes(StandardCharsets.UTF_8);

  @TempDir Path home;

  private ReleaseServer releases;
  private List<String> warnings;
  private Map<String, String> environment;

  @BeforeEach
  void serveRelease() throws IOException {
    releases = new ReleaseServer();
    warnings = new ArrayList<>();
    environment = new java.util.HashMap<>();
  }

  @AfterEach
  void stopServing() {
    releases.close();
  }

  private Path cache() {
    return home.resolve("bin").resolve("sysml-grpc");
  }

  private BinaryDownloader.Builder downloader() {
    return new BinaryDownloader.Builder()
        .downloadBaseUrl(releases.downloadBaseUrl())
        .apiBaseUrl(releases.apiBaseUrl())
        .binaryPath(cache())
        .asset(ASSET)
        .pins(ReleaseDigests.of(Map.of()))
        .environment((UnaryOperator<String>) environment::get)
        .warnings(warnings::add);
  }

  private static ReleaseDigests pinning(String digest) {
    return ReleaseDigests.of(Map.of(REPO, Map.of(VERSION, Map.of(ASSET, digest))));
  }

  /** The temporary files a download left in the cache directory, which must be none. */
  private List<String> leftBehind() throws IOException {
    try (var entries = Files.list(cache().getParent())) {
      return entries
          .map(path -> path.getFileName().toString())
          .filter(
            name ->
                !name.equals("sysml-grpc")
                    && !name.equals("sysml-grpc.json")
                    && !name.equals("sysml-grpc.lock"))
          .sorted()
          .toList();
    }
  }

  /** A recorded signed release, from the one copy of it the Python tests also read. */
  private static Path fixture(String name) {
    try {
      Path testClasses =
          Path.of(
              BinaryDownloaderTest.class
                  .getProtectionDomain()
                  .getCodeSource()
                  .getLocation()
                  .toURI());
      return testClasses
          .resolve("../../../../python/tests/fixtures/signed_release/" + name)
          .normalize();
    } catch (URISyntaxException e) {
      throw new IllegalStateException(e);
    }
  }

  @Test
  void installsAReleaseItPinsTheDigestOf() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    Path installed = downloader.downloadBinary(VERSION, OPTIONS);

    assertEquals(cache(), installed);
    assertArrayEqualsBytes(BINARY, Files.readAllBytes(installed));
    assertTrue(Files.isExecutable(installed), "the installed binary must be executable");
    assertEquals(List.of(), warnings);
    // The Python client reads this record, so it is the shape that client writes.
    assertEquals(
        Map.of(
            "version", VERSION,
            "sha256", ReleaseServer.sha256(BINARY),
            "repo", REPO),
        readMetadata(downloader));
    assertEquals(Optional.of(VERSION), downloader.cachedRelease(REPO));
  }

  @Test
  void resolvesTheLatestReleaseThroughTheApi() {
    releases.latest(REPO, VERSION).publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    assertEquals(VERSION, downloader.resolveLatestVersion(REPO));
    downloader.downloadBinary("latest", OPTIONS);
    assertEquals(Optional.of(VERSION), downloader.cachedRelease(REPO));
  }

  @Test
  void refusesAServedChecksumThatDisagreesWithThePin() throws IOException {
    releases.publish(REPO, VERSION, ASSET, "someone else's binary".getBytes(StandardCharsets.UTF_8));
    Path cached = installedBinary("cached");
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    ChecksumMismatchException refused =
        assertThrows(
            ChecksumMismatchException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains("this client pins"), refused.getMessage());
    assertEquals("cached", Files.readString(cached));
    assertFalse(
        releases.requested().contains("/" + REPO + "/releases/download/" + VERSION + "/" + ASSET),
        "a download contradicting the pin must not be fetched at all");
  }

  @Test
  void refusesABodyThatDoesNotHashToItsChecksum() throws IOException {
    // A sidecar agreeing with the pin over a body that does not: a truncated or swapped download.
    releases.put(REPO, VERSION, ASSET, "truncated".getBytes(StandardCharsets.UTF_8));
    releases.put(
        REPO,
        VERSION,
        ASSET + ".sha256",
        (ReleaseServer.sha256(BINARY) + "  " + ASSET + "\n").getBytes(StandardCharsets.UTF_8));
    Path cached = installedBinary("cached");
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    ChecksumMismatchException refused =
        assertThrows(
            ChecksumMismatchException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains("hashes to"), refused.getMessage());
    assertEquals("cached", Files.readString(cached));
    assertEquals(
        List.of(),
        leftBehind(),
        "the unverified download must not be left behind");
  }

  @Test
  void refusesAReleaseNothingVouchesFor() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().build();

    UnpinnedReleaseException refused =
        assertThrows(
            UnpinnedReleaseException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains(VERSION), refused.getMessage());
    assertTrue(refused.getMessage().contains("pins no SHA-256 digest"), refused.getMessage());
    assertFalse(Files.exists(cache()), "nothing may be installed for a release nothing vouches for");
    assertFalse(
        releases.requested().contains("/" + REPO + "/releases/download/" + VERSION + "/" + ASSET),
        "the binary must not be fetched before it is known what to verify it against");
  }

  @Test
  void acceptsSameOriginTrustWhenItIsGrantedForAnyRepository() {
    environment.put(ConnectionOptions.ALLOW_UNPINNED_ENV, "1");
    releases.publish(REPO, VERSION, ASSET, BINARY);

    downloader().build().downloadBinary(VERSION, OPTIONS);

    assertTrue(Files.exists(cache()));
    assertEquals(1, warnings.size(), warnings.toString());
    assertTrue(warnings.get(0).contains("pins no digest"), warnings.get(0));
    assertTrue(warnings.get(0).contains("compromised release"), warnings.get(0));
  }

  @Test
  void acceptsSameOriginTrustWhenItIsGrantedForThisRepository() {
    environment.put(ConnectionOptions.ALLOW_UNPINNED_ENV, "a-fork/OpenSysML, " + REPO);
    releases.publish(REPO, VERSION, ASSET, BINARY);

    downloader().build().downloadBinary(VERSION, OPTIONS);

    assertTrue(Files.exists(cache()));
    assertEquals(1, warnings.size(), warnings.toString());
  }

  @Test
  void refusesSameOriginTrustGrantedForAnotherRepository() {
    environment.put(ConnectionOptions.ALLOW_UNPINNED_ENV, "a-fork/OpenSysML");
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().build();

    assertThrows(
        UnpinnedReleaseException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));
  }

  @Test
  void acceptsSameOriginTrustGrantedInTheOptions() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions allowed = ConnectionOptions.builder().allowUnpinnedDownload(true).build();

    downloader().build().downloadBinary(VERSION, allowed);

    assertTrue(Files.exists(cache()));
  }

  @Test
  void takesTheDigestFromASignedManifestWhenNothingIsPinned() throws IOException {
    byte[] signed = Files.readAllBytes(fixture(ASSET));
    publishSignedManifest("SHA256SUMS.txt.bundle");
    releases.publish(REPO, VERSION, ASSET, signed);

    Path installed =
        downloader().trustedRoot(fixture("trusted_root.json")).build()
            .downloadBinary(VERSION, OPTIONS);

    assertArrayEqualsBytes(signed, Files.readAllBytes(installed));
    assertEquals(List.of(), warnings, "a signed manifest is a verified digest, not a fallback");
  }

  @Test
  void refusesAManifestSignedByAnotherIdentityEvenWhenSameOriginTrustIsGranted()
      throws IOException {
    environment.put(ConnectionOptions.ALLOW_UNPINNED_ENV, "1");
    publishSignedManifest("SHA256SUMS.txt.other-identity.bundle");
    releases.publish(REPO, VERSION, ASSET, Files.readAllBytes(fixture(ASSET)));
    BinaryDownloader downloader = downloader().trustedRoot(fixture("trusted_root.json")).build();

    ManifestSignatureException refused =
        assertThrows(
            ManifestSignatureException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains("does not verify"), refused.getMessage());
    assertFalse(Files.exists(cache()), "a signature that fails is never fallen back from");
  }

  @Test
  void refusesAManifestSignedWithAnExpiredCertificate() throws IOException {
    publishSignedManifest("SHA256SUMS.txt.expired.bundle");
    releases.publish(REPO, VERSION, ASSET, Files.readAllBytes(fixture(ASSET)));
    BinaryDownloader downloader = downloader().trustedRoot(fixture("trusted_root.json")).build();

    assertThrows(
        ManifestSignatureException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));
  }

  @Test
  void refusesAManifestChangedAfterItWasSigned() throws IOException {
    publishSignedManifest("SHA256SUMS.txt.bundle");
    releases.put(
        REPO,
        VERSION,
        SignedManifest.MANIFEST_ASSET,
        (ReleaseServer.sha256(BINARY) + "  " + ASSET + "\n").getBytes(StandardCharsets.UTF_8));
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().trustedRoot(fixture("trusted_root.json")).build();

    assertThrows(
        ManifestSignatureException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));
  }

  @Test
  void aPinIsWhatTheDownloadMustHash() {
    String pinned = "ab".repeat(32);
    String other = "cd".repeat(32);
    BinaryDownloader downloader = downloader().pins(pinning(pinned)).build();
    ConnectionOptions allowed = ConnectionOptions.builder().allowUnpinnedDownload(true).build();

    assertEquals(
        pinned, downloader.expectedDigest(VERSION, ASSET, pinned, REPO, pinned, null, OPTIONS));
    // A signed manifest disagreeing with a pin is not a reason to prefer either.
    ChecksumMismatchException contradicted =
        assertThrows(
            ChecksumMismatchException.class,
            () -> downloader.expectedDigest(VERSION, ASSET, pinned, REPO, other, null, allowed));
    assertTrue(contradicted.getMessage().contains("signed"), contradicted.getMessage());
    // Nor is same-origin trust, granted or not.
    assertThrows(
        ChecksumMismatchException.class,
        () -> downloader.expectedDigest(VERSION, ASSET, other, REPO, null, null, allowed));
  }

  @Test
  void replacesACacheOfAnotherRelease() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();
    installedBinary("older release");
    Files.writeString(
        downloader.metadataPath(),
        "{\"version\": \"v0.0.1\", \"sha256\": \""
            + ReleaseServer.sha256("older release".getBytes(StandardCharsets.UTF_8))
            + "\", \"repo\": \""
            + REPO
            + "\"}");

    Optional<String> stale = downloader.staleCacheReason(VERSION, REPO);
    assertTrue(stale.isPresent());
    assertTrue(stale.get().contains("v0.0.1"), stale.get());

    downloader.downloadBinary(VERSION, OPTIONS);
    assertArrayEqualsBytes(BINARY, Files.readAllBytes(cache()));
  }

  @Test
  void keepsTheCacheWhenTheReplacementCannotBeDownloaded() throws IOException {
    Path cached = installedBinary("cached");
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    // The release publishes no asset at all: a transport failure, which installs nothing.
    ServiceStartException failed =
        assertThrows(
            ServiceStartException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(failed.getMessage().contains("404"), failed.getMessage());
    assertEquals("cached", Files.readString(cached));
  }

  @Test
  void aCacheItCannotVouchForIsNotTheReleaseItRecords() throws IOException {
    BinaryDownloader downloader = downloader().build();
    installedBinary("swapped in by hand");
    Files.writeString(
        downloader.metadataPath(),
        "{\"version\": \"" + VERSION + "\", \"sha256\": \"" + ReleaseServer.sha256(BINARY)
            + "\", \"repo\": \"" + REPO + "\"}");

    assertEquals(Optional.empty(), downloader.cachedRelease(REPO));
    assertTrue(downloader.staleCacheReason(VERSION, REPO).isPresent());
  }

  @Test
  void aCacheFromAnotherRepositoryDoesNotAnswerForThisOne() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();
    downloader.downloadBinary(VERSION, OPTIONS);

    assertEquals(Optional.of(VERSION), downloader.cachedRelease(REPO));
    // Forks publish the same tags, so the fork's v9.9.9 is not this one.
    assertEquals(Optional.empty(), downloader.cachedRelease("a-fork/OpenSysML"));
    assertTrue(
        downloader.staleCacheReason(VERSION, "a-fork/OpenSysML").orElseThrow()
            .contains("downloaded from " + REPO));
  }

  @Test
  void readsWhereADownloadIsAskedForFrom() {
    ConnectionOptions asked = ConnectionOptions.builder().downloadVersion("v1.2.3").build();
    assertEquals(
        Optional.of("v1.2.3"), BinaryDownloader.versionAskedFor(asked, environment::get));
    assertEquals(
        Optional.empty(), BinaryDownloader.versionAskedFor(OPTIONS, environment::get));

    environment.put(ConnectionOptions.VERSION_ENV, "v0.3.0");
    assertEquals(
        Optional.of("v0.3.0"), BinaryDownloader.versionAskedFor(OPTIONS, environment::get));
    assertEquals(
        Optional.of("v1.2.3"),
        BinaryDownloader.versionAskedFor(asked, environment::get),
        "the caller outranks the environment");

    assertEquals(REPO, BinaryDownloader.githubRepo(OPTIONS, environment::get));
    environment.put(ConnectionOptions.REPO_ENV, "a-fork/OpenSysML");
    assertEquals("a-fork/OpenSysML", BinaryDownloader.githubRepo(OPTIONS, environment::get));
    assertEquals(
        "another/fork",
        BinaryDownloader.githubRepo(
            ConnectionOptions.builder().githubRepo("another/fork").build(), environment::get));
  }

  @Test
  void buildsReleaseUrls() {
    BinaryDownloader downloader = downloader().build();
    assertEquals(
        releases.downloadBaseUrl() + "/" + REPO + "/releases/download/" + VERSION + "/" + ASSET,
        downloader.releaseDownloadUrl(VERSION, ASSET, REPO));
  }

  @Test
  void refusesARepositoryTheEnvironmentNamesThatIsNotOne() {
    environment.put(ConnectionOptions.REPO_ENV, "http://elsewhere.example/evil");

    ServiceStartException refused =
        assertThrows(
            ServiceStartException.class,
            () -> BinaryDownloader.githubRepo(OPTIONS, environment::get));

    assertTrue(refused.getMessage().contains(ConnectionOptions.REPO_ENV), refused.getMessage());
  }

  @Test
  void refusesRepositoriesAndTagsThatWouldLeaveTheReleaseUrl() {
    BinaryDownloader downloader = downloader().build();

    assertThrows(
        ServiceStartException.class,
        () -> downloader.releaseDownloadUrl(VERSION, ASSET, "Open-MBEE/../../evil"));
    assertThrows(
        ServiceStartException.class, () -> downloader.releaseDownloadUrl("..", ASSET, REPO));
    assertThrows(
        ServiceStartException.class,
        () -> downloader.releaseDownloadUrl("v1?x=/../y", ASSET, REPO));
    assertThrows(ServiceStartException.class, () -> downloader.resolveLatestVersion("evil.example/"
        + "a/b"));
  }

  @Test
  void installsOnceWhenSeveralThreadsDownloadTheSameRelease() throws Exception {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    List<Callable<Path>> downloads = new ArrayList<>();
    for (int i = 0; i < 8; i++) {
      downloads.add(() -> downloader.downloadBinary(VERSION, OPTIONS));
    }
    ExecutorService threads = Executors.newFixedThreadPool(downloads.size());
    try {
      for (Future<Path> download : threads.invokeAll(downloads)) {
        assertEquals(cache(), download.get(), "every concurrent download must install the release");
      }
    } finally {
      threads.shutdownNow();
    }

    assertArrayEqualsBytes(BINARY, Files.readAllBytes(cache()));
    assertEquals(List.of(), leftBehind(), "concurrent downloads must not leave temporary files");
    assertEquals(Optional.of(VERSION), downloader.cachedRelease(REPO));
  }

  @Test
  void refusesADownloadTooLargeToBeARelease() throws Exception {
    releases.put(REPO, VERSION, ASSET, BINARY).endless(REPO, VERSION, ASSET + ".sha256");
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();

    ServiceStartException refused =
        assertThrows(
            ServiceStartException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains("larger than"), refused.getMessage());
    assertFalse(Files.exists(cache()), "an oversized response must install nothing");
    assertEquals(List.of(), leftBehind(), "a refused download must leave no temporary file");
  }

  @Test
  void givesUpOnAReleaseThatStopsSendingItsBody() throws Exception {
    releases.publish(REPO, VERSION, ASSET, BINARY).stalling(REPO, VERSION, ASSET);
    BinaryDownloader downloader =
        downloader()
            .pins(pinning(ReleaseServer.sha256(BINARY)))
            .stallTimeout(Duration.ofMillis(300))
            .build();

    ServiceStartException refused =
        assertThrows(
            ServiceStartException.class, () -> downloader.downloadBinary(VERSION, OPTIONS));

    assertTrue(refused.getMessage().contains("stopped sending"), refused.getMessage());
    assertFalse(Files.exists(cache()), "a stalled download must install nothing");
    assertEquals(List.of(), leftBehind(), "a stalled download must leave no temporary file");
  }

  @Test
  void waitsForACacheLockAnotherCopyOfThisClientHoldsInThisJvm() throws Exception {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    BinaryDownloader downloader = downloader().pins(pinning(ReleaseServer.sha256(BINARY))).build();
    Files.createDirectories(cache().getParent());
    Path lock = cache().resolveSibling("sysml-grpc.lock");

    // A shaded or separately loaded copy of this class locks the same file without sharing this
    // one's monitors, which is refused rather than queued.
    try (FileChannel channel =
            FileChannel.open(lock, StandardOpenOption.CREATE, StandardOpenOption.WRITE);
        FileLock held = channel.lock()) {
      ExecutorService threads = Executors.newSingleThreadExecutor();
      try {
        Future<Path> download =
            threads.submit(() -> downloader.downloadBinary(VERSION, OPTIONS));
        assertThrows(TimeoutException.class, () -> download.get(500, TimeUnit.MILLISECONDS));
        held.release();
        assertArrayEqualsBytes(BINARY, Files.readAllBytes(download.get(30, TimeUnit.SECONDS)));
      } finally {
        threads.shutdownNow();
      }
    }
  }

  private void publishSignedManifest(String bundle) throws IOException {
    releases.put(
        REPO,
        VERSION,
        SignedManifest.MANIFEST_ASSET,
        Files.readAllBytes(fixture(SignedManifest.MANIFEST_ASSET)));
    releases.put(
        REPO, VERSION, SignedManifest.BUNDLE_ASSET, Files.readAllBytes(fixture(bundle)));
  }

  private Path installedBinary(String content) throws IOException {
    Files.createDirectories(cache().getParent());
    Files.writeString(cache(), content);
    cache().toFile().setExecutable(true, true);
    return cache();
  }

  private Map<String, String> readMetadata(BinaryDownloader downloader) throws IOException {
    Object recorded = Json.parse(Files.readString(downloader.metadataPath()));
    return Map.of(
        "version", Json.stringMember(recorded, "version"),
        "sha256", Json.stringMember(recorded, "sha256"),
        "repo", Json.stringMember(recorded, "repo"));
  }

  private static void assertArrayEqualsBytes(byte[] expected, byte[] actual) {
    assertEquals(
        new String(expected, StandardCharsets.UTF_8), new String(actual, StandardCharsets.UTF_8));
  }
}

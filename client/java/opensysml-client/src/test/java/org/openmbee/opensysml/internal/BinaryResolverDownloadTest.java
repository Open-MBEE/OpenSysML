package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.ChecksumMismatchException;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.ServiceStartException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** When resolving a binary downloads one, and when it uses what is already installed. */
class BinaryResolverDownloadTest {

  private static final String REPO = "Open-MBEE/OpenSysML";
  private static final String VERSION = "v9.9.9";
  private static final String ASSET = "sysml-grpc-linux-amd64";
  private static final byte[] BINARY = "#!/bin/sh\nexit 0\n".getBytes(StandardCharsets.UTF_8);

  @TempDir Path home;

  private ReleaseServer releases;

  @BeforeEach
  void serveRelease() throws IOException {
    releases = new ReleaseServer();
  }

  @AfterEach
  void stopServing() {
    releases.close();
  }

  private Path cache() {
    return home.resolve("bin").resolve("sysml-grpc");
  }

  private BinaryDownloader downloader() {
    return downloader(Map.of(VERSION, Map.of(ASSET, digest())));
  }

  private BinaryDownloader downloader(Map<String, Map<String, String>> pins) {
    return new BinaryDownloader.Builder()
        .downloadBaseUrl(releases.downloadBaseUrl())
        .apiBaseUrl(releases.apiBaseUrl())
        .binaryPath(cache())
        .asset(ASSET)
        .environment(name -> null)
        .pins(ReleaseDigests.of(Map.of(REPO, pins)))
        .build();
  }

  private static String digest() {
    return ReleaseServer.sha256(BINARY);
  }

  @Test
  void downloadsTheReleaseAskedForWhenNothingIsInstalled() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    Path resolved = BinaryResolver.resolve(options, downloader());

    assertTrue(Files.isExecutable(resolved));
    assertArrayEquals(BINARY, Files.readAllBytes(resolved));
    assertArrayEquals(BINARY, Files.readAllBytes(cache()));
  }

  @Test
  void usesTheCacheWhenItIsTheReleaseAskedFor() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();
    BinaryDownloader downloader = downloader();
    BinaryResolver.resolve(options, downloader);
    int fetched = releases.requested().size();

    Path resolved = BinaryResolver.resolve(options, downloader);

    assertArrayEquals(BINARY, Files.readAllBytes(resolved));
    assertEquals(fetched, releases.requested().size(), "a cache of that release is not re-fetched");
    assertTrue(Files.exists(downloader.metadataPath()));
  }

  @Test
  void refusesToStartWithoutABinaryWhenNoReleaseIsAskedFor() {
    ConnectionOptions options = ConnectionOptions.builder().binaryPath(cache()).build();
    BinaryDownloader downloader = downloader();
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> BinaryResolver.resolve(options, downloader));
    assertTrue(refused.getMessage().contains("no executable sysml-grpc"), refused.getMessage());
  }

  @Test
  void aCallersPinnedDigestIsCheckedOnWhatWasDownloaded() {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    ConnectionOptions options =
        ConnectionOptions.builder()
            .downloadVersion(VERSION)
            .expectedBinarySha256("ab".repeat(32))
            .build();

    BinaryDownloader downloader = downloader();
    assertThrows(
        ChecksumMismatchException.class, () -> BinaryResolver.resolve(options, downloader));
  }

  @Test
  void keepsAWorkingBinaryWhenTheReleaseAskedForCannotBeDownloaded() throws IOException {
    Files.createDirectories(cache().getParent());
    Files.writeString(cache(), "installed by hand");
    cache().toFile().setExecutable(true, true);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    Path resolved = BinaryResolver.resolve(options, downloader());

    assertEquals("installed by hand", Files.readString(resolved));
  }

  @Test
  void losesNothingWhenADownloadIsRefusedAsTamperedWith() throws IOException {
    releases.publish(REPO, VERSION, ASSET, "another binary".getBytes(StandardCharsets.UTF_8));
    Files.createDirectories(cache().getParent());
    Files.writeString(cache(), "installed by hand");
    cache().toFile().setExecutable(true, true);
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(VERSION).build();

    // A download contradicting a pin is evidence, so it is not answered from the cache.
    BinaryDownloader downloader = downloader();
    assertThrows(
        ChecksumMismatchException.class, () -> BinaryResolver.resolve(options, downloader));
    assertEquals("installed by hand", Files.readString(cache()));
    try (var entries = Files.list(cache().getParent())) {
      assertEquals(
          List.of("sysml-grpc", "sysml-grpc.lock"),
          entries.map(path -> path.getFileName().toString()).sorted().toList(),
          "the refused download must leave nothing behind");
    }
  }

  @Test
  void startsTheReleaseAskedForEvenWhenAnotherIsInstalledOverTheCache() throws IOException {
    releases.publish(REPO, VERSION, ASSET, BINARY);
    byte[] older = "#!/bin/sh\nexit 1\n".getBytes(StandardCharsets.UTF_8);
    String olderVersion = "v9.9.8";
    releases.publish(REPO, olderVersion, ASSET, older);
    BinaryDownloader downloader =
        downloader(
            Map.of(
                VERSION,
                Map.of(ASSET, digest()),
                olderVersion,
                Map.of(ASSET, ReleaseServer.sha256(older))));

    Path resolved =
        BinaryResolver.resolve(
            ConnectionOptions.builder().downloadVersion(VERSION).build(), downloader);
    BinaryResolver.resolve(
        ConnectionOptions.builder().downloadVersion(olderVersion).build(), downloader);

    assertArrayEquals(older, Files.readAllBytes(cache()), "the cache is the release asked for last");
    assertArrayEquals(
        BINARY,
        Files.readAllBytes(resolved),
        "what the first connection starts is the release it asked for");
  }

  @Test
  void twoReleasesAskedForAtOnceLeaveTheCacheAsOneOfThem() throws Exception {
    byte[] older = "#!/bin/sh\nexit 1\n".getBytes(StandardCharsets.UTF_8);
    String olderVersion = "v9.9.8";
    releases.publish(REPO, VERSION, ASSET, BINARY);
    releases.publish(REPO, olderVersion, ASSET, older);
    BinaryDownloader downloader =
        downloader(
            Map.of(
                VERSION,
                Map.of(ASSET, digest()),
                olderVersion,
                Map.of(ASSET, ReleaseServer.sha256(older))));

    ExecutorService threads = Executors.newFixedThreadPool(2);
    try {
      for (Future<Path> resolved :
          threads.invokeAll(
              List.of(
                  resolving(VERSION, downloader),
                  resolving(olderVersion, downloader),
                  resolving(VERSION, downloader),
                  resolving(olderVersion, downloader)))) {
        assertTrue(Files.isExecutable(resolved.get()));
      }
    } finally {
      threads.shutdownNow();
    }

    // Whichever release won the cache, its bytes and its recorded release are the same one.
    String cached = downloader.cachedRelease(REPO).orElseThrow();
    assertArrayEquals(
        cached.equals(VERSION) ? BINARY : older,
        Files.readAllBytes(cache()),
        "the cache must hold the release its metadata records");
  }

  private Callable<Path> resolving(String version, BinaryDownloader downloader) {
    ConnectionOptions options = ConnectionOptions.builder().downloadVersion(version).build();
    byte[] wanted = version.equals(VERSION) ? BINARY : "#!/bin/sh\nexit 1\n"
        .getBytes(StandardCharsets.UTF_8);
    return () -> {
      Path resolved = BinaryResolver.resolve(options, downloader);
      assertArrayEquals(
          wanted, Files.readAllBytes(resolved), "each connection starts the release it asked for");
      return resolved;
    };
  }
}

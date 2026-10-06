package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.ServiceStartException;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.TreeSet;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;

/** Which asset a platform downloads, what the shipped pins say, and what a manifest lists. */
class ReleaseAssetsTest {

  /** The repository the jar's own release-digests.json pins. */
  private static final String SHIPPED_REPO = "Open-MBEE/OpenSysML";

  /** The assets every release publishes, which a complete pin names all of. */
  private static final List<String> SERVICE_ASSETS =
      List.of(
          "sysml-grpc-darwin-amd64",
          "sysml-grpc-darwin-arm64",
          "sysml-grpc-linux-amd64",
          "sysml-grpc-linux-arm64",
          "sysml-grpc-windows-amd64.exe");

  /**
   * The release job stamps the resource it packages and names the tag here, so the suite proves
   * the jar about to ship pins its own release.
   */
  private static final String EXPECT_PINNED_RELEASE_ENV = "OPENSYSML_EXPECT_PINNED_RELEASE";

  @Test
  void mapsPlatformsOntoTheAssetsReleasesPublish() {
    assertEquals("sysml-grpc-linux-amd64", ReleasePlatform.assetName("linux", "amd64"));
    assertEquals("sysml-grpc-linux-arm64", ReleasePlatform.assetName("linux", "arm64"));
    assertEquals("sysml-grpc-darwin-amd64", ReleasePlatform.assetName("darwin", "amd64"));
    assertEquals("sysml-grpc-darwin-arm64", ReleasePlatform.assetName("darwin", "arm64"));
    assertEquals("sysml-grpc-windows-amd64.exe", ReleasePlatform.assetName("windows", "amd64"));
  }

  @Test
  void refusesAPlatformNoReleaseIsBuiltFor() {
    ServiceStartException refused =
        assertThrows(
            ServiceStartException.class, () -> ReleasePlatform.assetName("windows", "arm64"));
    assertTrue(refused.getMessage().contains("windows-arm64"), refused.getMessage());
    assertThrows(ServiceStartException.class, () -> ReleasePlatform.assetName("plan9", "amd64"));
  }

  @Test
  void namesTheAssetThisMachineWouldDownload() {
    assertEquals(
        "sysml-grpc-" + ReleasePlatform.goos() + "-" + ReleasePlatform.goarch()
            + (ReleasePlatform.isWindows() ? ".exe" : ""),
        ReleasePlatform.assetName());
    assertTrue(ReleasePlatform.cachedBinary().endsWith(ReleasePlatform.cachedBinaryName()));
    assertTrue(ReleasePlatform.cachedBinary().toString().contains(".opensysml"));
  }

  @Test
  void shipsThePinnedDigestsInTheJar() {
    ReleaseDigests shipped = ReleaseDigests.shipped();
    assertFalse(shipped.isEmpty(), "the release-digests.json resource must be on the classpath");
    assertEquals(
        Optional.empty(),
        shipped.pin("Open-MBEE/OpenSysML", "v0.0.0-never-released", "sysml-grpc-linux-amd64"));
    Optional<String> pinned =
        shipped.pin("Open-MBEE/OpenSysML", "v0.3.0", "sysml-grpc-linux-amd64");
    assertTrue(pinned.isPresent(), "v0.3.0 must be pinned");
    assertTrue(pinned.get().matches("[0-9a-f]{64}"), pinned.get());
  }

  @Test
  void pinsEveryServiceAssetOfEveryReleaseItShips() throws IOException {
    Map<?, ?> releases = shippedReleases();
    assertFalse(releases.isEmpty(), "the resource pins no release of " + SHIPPED_REPO);
    ReleaseDigests shipped = ReleaseDigests.shipped();
    for (Map.Entry<?, ?> release : releases.entrySet()) {
      String version = (String) release.getKey();
      assertTrue(
          version.matches("v\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z.]+)?"),
          version + " is not a release tag");
      assertTrue(release.getValue() instanceof Map<?, ?>, version + " pins no assets");
      Map<?, ?> assets = (Map<?, ?>) release.getValue();
      assertEquals(
          new TreeSet<>(SERVICE_ASSETS),
          new TreeSet<Object>(assets.keySet()),
          version + " does not pin exactly the five service assets");
      for (String asset : SERVICE_ASSETS) {
        Optional<String> pinned = shipped.pin(SHIPPED_REPO, version, asset);
        assertEquals(Optional.of(assets.get(asset)), pinned, asset + " of " + version);
        assertTrue(pinned.get().matches("[0-9a-f]{64}"), pinned.get());
      }
    }
  }

  @Test
  @EnabledIfEnvironmentVariable(named = EXPECT_PINNED_RELEASE_ENV, matches = ".+")
  void pinsTheReleaseBeingPublished() {
    String tag = System.getenv(EXPECT_PINNED_RELEASE_ENV);
    assertTrue(
        tag.matches("v\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z.]+)?"),
        "$" + EXPECT_PINNED_RELEASE_ENV + "=" + tag + " is not a tag");
    ReleaseDigests shipped = ReleaseDigests.shipped();
    for (String asset : SERVICE_ASSETS) {
      Optional<String> pinned = shipped.pin(SHIPPED_REPO, tag, asset);
      assertTrue(pinned.isPresent(), "the jar does not pin " + asset + " of " + tag);
      assertTrue(pinned.get().matches("[0-9a-f]{64}"), pinned.get());
    }
  }

  /** The releases of {@link #SHIPPED_REPO} the resource on the classpath lists, as parsed JSON. */
  private static Map<?, ?> shippedReleases() throws IOException {
    try (InputStream in = ReleaseDigests.class.getResourceAsStream(ReleaseDigests.RESOURCE)) {
      assertTrue(in != null, ReleaseDigests.RESOURCE + " must be on the classpath");
      Object table = Json.parse(new String(in.readAllBytes(), StandardCharsets.UTF_8));
      assertTrue(table instanceof Map<?, ?>, "the resource is not a JSON object");
      Object releases = ((Map<?, ?>) table).get(SHIPPED_REPO);
      assertTrue(releases instanceof Map<?, ?>, "the resource pins nothing for " + SHIPPED_REPO);
      return (Map<?, ?>) releases;
    }
  }

  @Test
  void readsTheDigestAManifestListsForAnAsset() {
    byte[] manifest =
        ("ab".repeat(32)
                + "  opensysml-linux-amd64.tar.gz\n"
                + "cd".repeat(32)
                + " *sysml-grpc-linux-amd64\n"
                + "not-a-digest  sysml-grpc-darwin-arm64\n")
            .getBytes(StandardCharsets.UTF_8);

    assertEquals(
        Optional.of("cd".repeat(32)),
        SignedManifest.digestFor(manifest, "sysml-grpc-linux-amd64"));
    assertEquals(
        Optional.empty(),
        SignedManifest.digestFor(manifest, "sysml-grpc-darwin-arm64"),
        "a line that is not a SHA-256 covers nothing");
    assertEquals(Optional.empty(), SignedManifest.digestFor(manifest, "sysml-grpc-linux-arm64"));
  }

  @Test
  void pinsTheIdentityTheReleasePipelineSignsWith() {
    SignedManifest.ReleaseSigner signer =
        SignedManifest.signerFor("Open-MBEE/OpenSysML").orElseThrow();
    assertEquals(
        "https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62", signer.issuer());
    String subject =
        signer.project() + "/pipeline-definitions/c3d1a44e-6cb7-4a2f-8f60-2b1d0e3f9a15";
    assertTrue(subject.matches(signer.subjectPattern()), signer.subjectPattern());
    assertFalse(
        (subject + "/attacker").matches(signer.subjectPattern()),
        "the subject must match whole, not as a prefix");
    assertFalse(
        (signer.project() + "/pipeline-definitions/not-a-uuid").matches(signer.subjectPattern()));
    assertEquals(Optional.empty(), SignedManifest.signerFor("a-fork/OpenSysML"));
  }
}

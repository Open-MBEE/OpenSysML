package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.ServiceStartException;
import java.nio.charset.StandardCharsets;
import java.util.Optional;
import org.junit.jupiter.api.Test;

/** Which asset a platform downloads, what the shipped pins say, and what a manifest lists. */
class ReleaseAssetsTest {

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

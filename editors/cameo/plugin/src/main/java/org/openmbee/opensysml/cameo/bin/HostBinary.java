package org.openmbee.opensysml.cameo.bin;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Locale;
import java.util.Optional;

/** Locates the sysml-grpc binary staged for the host platform under the plugin's bin/ folder. */
public final class HostBinary {
  public static final String DIGESTS_FILE = "DIGESTS";

  private HostBinary() {}

  public static Optional<String> assetName() {
    return assetName(System.getProperty("os.name"), System.getProperty("os.arch"));
  }

  public static Optional<String> assetName(String osName, String osArch) {
    String os = osName.toLowerCase(Locale.ROOT);
    String platform;
    if (os.contains("win")) {
      platform = "windows";
    } else if (os.contains("mac") || os.contains("darwin")) {
      platform = "darwin";
    } else if (os.contains("linux")) {
      platform = "linux";
    } else {
      return Optional.empty();
    }
    String architecture = osArch.toLowerCase(Locale.ROOT);
    String arch;
    if (architecture.equals("amd64") || architecture.equals("x86_64") || architecture.equals("x64")) {
      arch = "amd64";
    } else if (architecture.equals("aarch64") || architecture.equals("arm64")) {
      arch = "arm64";
    } else {
      return Optional.empty();
    }
    return Optional.of("sysml-grpc-" + platform + "-" + arch + (platform.equals("windows") ? ".exe" : ""));
  }

  public static boolean exists(Path pluginDir) {
    return assetName().map(asset -> Files.isRegularFile(pluginDir.resolve("bin").resolve(asset))).orElse(false);
  }

  public static Path locate(Path pluginDir) {
    String asset = assetName().orElseThrow(
        () -> new IllegalStateException("unsupported host: " + System.getProperty("os.name")
            + "/" + System.getProperty("os.arch")));
    Path path = pluginDir.resolve("bin").resolve(asset);
    if (!Files.isRegularFile(path)) {
      throw new IllegalStateException("missing OpenSysML service binary: " + path);
    }
    path.toFile().setExecutable(true, false);
    return path;
  }
}

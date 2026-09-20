package org.openmbee.opensysml.cameo.bin;

import java.nio.file.Files;
import java.nio.file.Path;

/** Locates the sysml-grpc binary staged for the host platform under the plugin's bin/ folder. */
public final class HostBinary {
  public static final String DIGESTS_FILE = "DIGESTS";

  private HostBinary() {}

  public static String assetName() {
    return assetName(System.getProperty("os.name"), System.getProperty("os.arch"));
  }

  public static String assetName(String osName, String osArch) {
    String os = osName.toLowerCase();
    String platform = os.contains("win") ? "windows" : os.contains("mac") || os.contains("darwin") ? "darwin" : "linux";
    String architecture = osArch.toLowerCase();
    String arch = architecture.equals("aarch64") || architecture.equals("arm64") ? "arm64" : "amd64";
    return "sysml-grpc-" + platform + "-" + arch + (platform.equals("windows") ? ".exe" : "");
  }

  public static boolean exists(Path pluginDir) {
    return Files.isRegularFile(pluginDir.resolve("bin").resolve(assetName()));
  }

  public static Path locate(Path pluginDir) {
    Path path = pluginDir.resolve("bin").resolve(assetName());
    if (!Files.isRegularFile(path)) {
      throw new IllegalStateException("missing OpenSysML service binary: " + path);
    }
    path.toFile().setExecutable(true, false);
    return path;
  }
}

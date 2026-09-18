package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.ServiceStartException;
import java.nio.file.Path;
import java.util.Locale;
import java.util.Set;

/**
 * The release asset built for this machine, and where a download of it is cached.
 *
 * <p>Asset names carry Go's {@code GOOS-GOARCH}, so {@code os.name} and {@code os.arch} are mapped
 * onto those; a pair no release publishes a binary for is refused by name rather than fetched as a
 * 404.
 */
public final class ReleasePlatform {

  private static final String WINDOWS = "windows";

  /** Every {@code goos-goarch} a release publishes a {@code sysml-grpc} binary for. */
  private static final Set<String> PUBLISHED =
      Set.of("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", WINDOWS + "-amd64");

  private ReleasePlatform() {}

  /**
   * The asset name a release publishes the service binary for this machine under.
   *
   * @return e.g. {@code sysml-grpc-linux-amd64}
   * @throws ServiceStartException if no release is built for this operating system and architecture
   */
  public static String assetName() {
    return assetName(goos(), goarch());
  }

  /**
   * The asset name a release publishes the service binary for a platform under.
   *
   * @param goos {@code linux}, {@code darwin} or {@code windows}
   * @param goarch {@code amd64} or {@code arm64}
   * @return e.g. {@code sysml-grpc-windows-amd64.exe}
   * @throws ServiceStartException if no release is built for that pair
   */
  public static String assetName(String goos, String goarch) {
    String pair = goos + "-" + goarch;
    if (!PUBLISHED.contains(pair)) {
      throw new ServiceStartException(
          "no sysml-grpc release is published for "
              + pair
              + "; build one with `make build` and name it with ConnectionOptions.binaryPath()");
    }
    String name = "sysml-grpc-" + pair;
    return WINDOWS.equals(goos) ? name + ".exe" : name;
  }

  /**
   * Go's name for this operating system.
   *
   * @return {@code linux}, {@code darwin} or {@code windows}
   * @throws ServiceStartException if it is none of those
   */
  public static String goos() {
    String name = System.getProperty("os.name", "").toLowerCase(Locale.ROOT);
    if (name.startsWith("linux")) {
      return "linux";
    }
    if (name.startsWith("mac") || name.startsWith("darwin")) {
      return "darwin";
    }
    if (name.startsWith(WINDOWS)) {
      return WINDOWS;
    }
    throw new ServiceStartException("unsupported operating system: " + name);
  }

  /**
   * Go's name for this architecture.
   *
   * @return {@code amd64} or {@code arm64}
   * @throws ServiceStartException if it is neither
   */
  public static String goarch() {
    String arch = System.getProperty("os.arch", "").toLowerCase(Locale.ROOT);
    if (arch.equals("x86_64") || arch.equals("amd64")) {
      return "amd64";
    }
    if (arch.equals("aarch64") || arch.equals("arm64")) {
      return "arm64";
    }
    throw new ServiceStartException("unsupported architecture: " + arch);
  }

  /**
   * Whether this is Windows, where the cached binary keeps its {@code .exe}.
   *
   * @return {@code true} on Windows
   */
  public static boolean isWindows() {
    return System.getProperty("os.name", "").toLowerCase(Locale.ROOT).startsWith(WINDOWS);
  }

  /**
   * Where a downloaded binary is cached, shared with the Python client.
   *
   * @return {@code ~/.opensysml/bin/sysml-grpc}, or {@code sysml-grpc.exe} on Windows
   */
  public static Path cachedBinary() {
    return Path.of(System.getProperty("user.home"), ".opensysml", "bin", cachedBinaryName());
  }

  /**
   * The file name of the cached binary.
   *
   * @return {@code sysml-grpc}, or {@code sysml-grpc.exe} on Windows
   */
  public static String cachedBinaryName() {
    return isWindows() ? "sysml-grpc.exe" : "sysml-grpc";
  }
}

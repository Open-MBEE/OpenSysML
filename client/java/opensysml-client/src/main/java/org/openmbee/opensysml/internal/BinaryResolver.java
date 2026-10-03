package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.ChecksumMismatchException;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.ServiceStartException;
import org.openmbee.opensysml.UnpinnedReleaseException;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Optional;
import java.util.function.UnaryOperator;

/**
 * Finds the {@code sysml-grpc} binary a private service is started from, downloading a release
 * when one is needed and nothing installed answers for it.
 *
 * <p>An operator-installed binary is used as found. A missing cache is downloaded from the
 * requested release or the release this client was built against, and verified before install:
 * against the digest this client pins for that release asset, else against the digest in the
 * release's sigstore-signed {@code SHA256SUMS.txt}, and otherwise refused. See
 * {@link BinaryDownloader}. A caller-pinned digest from
 * {@link ConnectionOptions.Builder#expectedBinarySha256(String)} is checked whichever binary is
 * resolved, before it is executed.
 */
public final class BinaryResolver {

  private static final System.Logger LOG = System.getLogger(BinaryResolver.class.getName());

  private BinaryResolver() {}

  /**
   * The binary to start, from the options, the environment, the user's cache, {@code PATH}, or a
   * release downloaded into the cache.
   *
   * @param options how the connection was configured
   * @return an existing, executable binary
   * @throws ServiceStartException if none was found, or a pinned digest does not match
   */
  public static Path resolve(ConnectionOptions options) {
    return resolve(options, BinaryDownloader.production());
  }

  /**
   * The binary to start, downloading through a given downloader.
   *
   * @param options how the connection was configured
   * @param downloader where a release is downloaded from and cached
   * @return an existing, executable binary
   * @throws ServiceStartException if none was found, or a pinned digest does not match
   */
  static Path resolve(ConnectionOptions options, BinaryDownloader downloader) {
    return resolve(options, downloader, System::getenv);
  }

  static Path resolve(
      ConnectionOptions options,
      BinaryDownloader downloader,
      UnaryOperator<String> environment) {
    Optional<Path> named = options.binaryPath();
    if (named.isPresent()) {
      Path path = named.get();
      if (!isExecutable(path)) {
        throw new ServiceStartException("no executable sysml-grpc at " + path);
      }
      return verified(path, options);
    }
    String namedEnvironment = environment.apply(ConnectionOptions.BINARY_ENV);
    if (namedEnvironment != null && !namedEnvironment.isBlank()) {
      Path path = Path.of(namedEnvironment.trim());
      if (isExecutable(path)) {
        return verified(path, options);
      }
    }

    boolean implicit = !BinaryDownloader.versionWasExplicit(options, environment);
    Optional<String> version = BinaryDownloader.versionAskedFor(options, environment);
    Path cache = downloader.binaryPath();
    if (version.isEmpty()) {
      List<String> looked = new ArrayList<>();
      for (Path candidate : candidates(cache, environment)) {
        looked.add(candidate.toString());
        if (isExecutable(candidate)) {
          return verified(candidate, options);
        }
      }
      throw new ServiceStartException(
          "no sysml-grpc binary found; build one with `make build`, install one with the "
              + "opensysml Python client, ask for a release with "
              + "ConnectionOptions.downloadVersion() or $"
              + ConnectionOptions.VERSION_ENV
              + ", or name a binary with ConnectionOptions.binaryPath() or $"
              + ConnectionOptions.BINARY_ENV
              + ". Looked at: "
              + String.join(", ", looked));
    }
    Path resolved;
    try {
      resolved =
          downloaded(options, downloader, version.get(), cache, implicit, environment);
    } catch (UnpinnedReleaseException e) {
      if (!implicit) {
        throw e;
      }
      throw unavailable(version.get(), e);
    } catch (ChecksumMismatchException e) {
      throw e;
    } catch (ServiceStartException e) {
      if (!implicit) {
        throw e;
      }
      throw unavailable(version.get(), e);
    }
    return verified(resolved, options);
  }

  /**
   * The cached binary when it is the release asked for, and otherwise the downloaded one. A
   * download that fails leaves a working binary in place rather than losing it.
   */
  private static Path downloaded(
      ConnectionOptions options,
      BinaryDownloader downloader,
      String version,
      Path cache,
      boolean implicit,
      UnaryOperator<String> environment) {
    // Held over the whole decision: two connections asking for different releases would otherwise
    // each find the cache stale and install over the other. What is started is the binary under
    // its own digest, so a release installed over the cache afterwards is not the one that runs.
    return downloader.withCacheLock(
        () -> {
          Path chosen = install(options, downloader, version, cache, implicit, environment);
          return chosen.equals(downloader.binaryPath()) ? downloader.stableBinary() : chosen;
        });
  }

  private static Path install(
      ConnectionOptions options,
      BinaryDownloader downloader,
      String version,
      Path cache,
      boolean implicit,
      UnaryOperator<String> environment) {
    String repo = BinaryDownloader.githubRepo(options);
    Path installed = null;
    if (isExecutable(cache)) {
      if (implicit && !Files.exists(downloader.metadataPath())) {
        return cache;
      }
      Optional<String> stale = downloader.staleCacheReason(version, repo);
      if (stale.isEmpty()) {
        return cache;
      }
      installed = cache;
      warn("replacing the cached sysml-grpc: " + stale.get() + ". Downloading " + version + ".");
    }
    if (installed == null) {
      for (Path candidate : candidates(cache, environment)) {
        if (isExecutable(candidate)) {
          installed = candidate;
          break;
        }
      }
    }

    try {
      return downloader.downloadBinary(version, options);
    } catch (ChecksumMismatchException e) {
      // A release nothing vouches for contradicts nothing, so a working binary stands; a
      // download that may have been tampered with is never answered from the cache.
      if (!(e instanceof UnpinnedReleaseException) || installed == null) {
        throw e;
      }
      warn(
          "keeping the sysml-grpc at "
              + installed
              + ": "
              + version
              + " was not downloaded ("
              + e.getMessage()
              + "). It may be an older release than asked for.");
      return installed;
    } catch (ServiceStartException e) {
      if (installed == null) {
        throw e;
      }
      // A release with no binary to fetch is no reason to lose a working one.
      warn(
          "keeping the sysml-grpc at "
              + installed
              + ": "
              + version
              + " could not be downloaded ("
              + e.getMessage()
              + "). It may be an older release than asked for.");
      return installed;
    }
  }

  private static List<Path> candidates(Path cache, UnaryOperator<String> environment) {
    List<Path> candidates = new ArrayList<>();
    String named = environment.apply(ConnectionOptions.BINARY_ENV);
    if (named != null && !named.isBlank()) {
      candidates.add(Path.of(named.trim()));
    }
    // Where the Python client installs its verified download, so the two share one binary.
    candidates.add(cache);
    String executable = ReleasePlatform.cachedBinaryName();
    String path = environment.apply("PATH");
    if (path != null) {
      for (String entry : path.split(File.pathSeparator)) {
        if (entry.isBlank()) {
          continue;
        }
        try {
          candidates.add(Path.of(entry, executable));
        } catch (InvalidPathException e) {
          // A PATH entry that is not a path is not a place to look.
        }
      }
    }
    return candidates;
  }

  private static void warn(String message) {
    LOG.log(System.Logger.Level.WARNING, message);
  }

  private static ServiceStartException unavailable(String version, Throwable cause) {
    String detail = cause.getMessage();
    String unreleased =
        detail != null && detail.contains("404") ? " This may be an unreleased checkout." : "";
    return new ServiceStartException(
        "Could not download sysml-grpc release "
            + version
            + ": "
            + detail
            + ". Install sysml-grpc on $PATH, set $"
            + ConnectionOptions.BINARY_ENV
            + ", or select another release with $"
            + ConnectionOptions.VERSION_ENV
            + " or ConnectionOptions.downloadVersion()."
            + unreleased,
        cause);
  }

  private static Path verified(Path binary, ConnectionOptions options) {
    Optional<String> expected = options.expectedBinarySha256();
    if (expected.isEmpty()) {
      return binary;
    }
    String actual = sha256(binary);
    if (!actual.equalsIgnoreCase(expected.get())) {
      throw new ChecksumMismatchException(
          "sysml-grpc at "
              + binary
              + " has SHA-256 "
              + actual
              + ", but "
              + expected.get()
              + " was required; it was not started");
    }
    return binary;
  }

  private static String sha256(Path binary) {
    try {
      MessageDigest digest = MessageDigest.getInstance("SHA-256");
      try (InputStream in = Files.newInputStream(binary)) {
        byte[] buffer = new byte[65536];
        int read;
        while ((read = in.read(buffer)) > 0) {
          digest.update(buffer, 0, read);
        }
      }
      StringBuilder hex = new StringBuilder(64);
      for (byte b : digest.digest()) {
        hex.append(String.format(Locale.ROOT, "%02x", b));
      }
      return hex.toString();
    } catch (IOException | NoSuchAlgorithmException e) {
      throw new ServiceStartException("sysml-grpc at " + binary + " could not be hashed", e);
    }
  }

  private static boolean isExecutable(Path path) {
    return Files.isRegularFile(path) && Files.isExecutable(path);
  }
}

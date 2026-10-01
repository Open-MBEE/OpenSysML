package org.openmbee.opensysml.cameo.tools;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public final class BinaryStager {
  private static final String DEFAULT_BASE = "https://github.com";
  private static final String REPOSITORY = "Open-MBEE/OpenSysML";
  private static final Pattern DIGEST =
      Pattern.compile("\"([^\"]+)\"\\s*:\\s*\"([0-9a-fA-F]{64})\"");

  private BinaryStager() {}

  public static void main(String[] args) throws Exception {
    if (args.length < 2) {
      throw new IllegalArgumentException("usage: BinaryStager <version> <outDir> [--base-url URL] [--digests path]");
    }
    String version = args[0];
    Path output = Path.of(args[1]);
    String base = DEFAULT_BASE;
    Path digests = null;
    int i = 2;
    while (i < args.length) {
      String option = args[i];
      if ("--base-url".equals(option) || "--digests".equals(option)) {
        if (i + 1 >= args.length) {
          throw new IllegalArgumentException("option requires a value: " + option);
        }
        if ("--base-url".equals(option)) base = args[i + 1];
        else digests = Path.of(args[i + 1]);
        i += 2;
      } else {
        throw new IllegalArgumentException("unknown option: " + option);
      }
    }
    stage(version, output, base, digests);
  }

  public static Map<String, String> stage(String version, Path output, String base, Path digestFile)
      throws IOException, InterruptedException {
    if (!version.matches("v\\d+\\.\\d+\\.\\d+")) {
      throw new IllegalArgumentException("version must be a release tag such as v1.2.3");
    }
    String json =
        digestFile == null
            ? new String(
                BinaryStager.class.getResourceAsStream("/release-digests.json").readAllBytes(),
                StandardCharsets.UTF_8)
            : Files.readString(digestFile);
    Map<String, String> pinned = parseDigests(json, version);
    if (pinned.isEmpty()) throw new IllegalArgumentException("version is not pinned: " + version);
    Files.createDirectories(output);
    Map<String, String> written = new LinkedHashMap<>();
    HttpClient client = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NORMAL).build();
    for (String asset : assets()) {
      String expected = pinned.get(asset);
      if (expected == null) throw new IllegalArgumentException("asset is not pinned: " + asset);
      URI uri = URI.create(base + "/" + REPOSITORY + "/releases/download/" + version + "/" + asset);
      HttpResponse<byte[]> response =
          client.send(HttpRequest.newBuilder(uri).GET().build(), HttpResponse.BodyHandlers.ofByteArray());
      if (response.statusCode() / 100 != 2) throw new IOException("download failed: " + response.statusCode() + " for " + uri);
      String actual = sha256(response.body());
      if (!expected.equalsIgnoreCase(actual)) throw new IOException("digest mismatch for " + asset);
      Files.write(output.resolve(asset), response.body());
      written.put(asset, actual);
    }
    StringBuilder manifest = new StringBuilder();
    written.forEach((asset, digest) -> manifest.append(digest).append("  ").append(asset).append('\n'));
    Files.writeString(output.resolve("DIGESTS"), manifest.toString());
    return Map.copyOf(written);
  }

  static String[] assets() {
    return new String[] {
      "sysml-grpc-linux-amd64", "sysml-grpc-linux-arm64", "sysml-grpc-darwin-amd64",
      "sysml-grpc-darwin-arm64", "sysml-grpc-windows-amd64.exe"
    };
  }

  static Map<String, String> parseDigests(String json, String version) {
    Map<String, String> values = new LinkedHashMap<>();
    int start = json.indexOf("\"" + version + "\"");
    if (start < 0) return values;
    int end = json.indexOf('}', start);
    if (end < 0) end = json.length();
    Matcher matcher = DIGEST.matcher(json.substring(start, end));
    while (matcher.find()) values.put(matcher.group(1), matcher.group(2).toLowerCase());
    return values;
  }

  static String sha256(byte[] bytes) {
    try {
      return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes));
    } catch (Exception exception) {
      throw new IllegalStateException(exception);
    }
  }
}

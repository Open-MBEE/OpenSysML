package org.openmbee.opensysml.internal;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/** A GitHub releases API and download host serving fixtures, so no test reaches the network. */
final class ReleaseServer implements AutoCloseable {

  /** How long a stalling asset holds its exchange open, longer than any test waits on it. */
  static final Duration STALL = Duration.ofSeconds(5);

  private final HttpServer server;
  private final Map<String, byte[]> assets = new HashMap<>();
  private final List<String> requested = new ArrayList<>();
  private final Set<String> endless = ConcurrentHashMap.newKeySet();
  private final Set<String> stalling = ConcurrentHashMap.newKeySet();
  /** Released when the server stops, so a stalled exchange does not outlive its test. */
  private final CountDownLatch stopped = new CountDownLatch(1);

  ReleaseServer() throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext("/", this::answer);
    server.start();
  }

  /** Publishes an asset of a release, and the {@code .sha256} sidecar the release serves for it. */
  ReleaseServer publish(String githubRepo, String version, String asset, byte[] content) {
    put(githubRepo, version, asset, content);
    put(
        githubRepo,
        version,
        asset + ".sha256",
        (sha256(content) + "  " + asset + "\n").getBytes(StandardCharsets.UTF_8));
    return this;
  }

  /** Publishes an asset with no sidecar, or with one that disagrees with it. */
  ReleaseServer put(String githubRepo, String version, String asset, byte[] content) {
    assets.put("/" + githubRepo + "/releases/download/" + version + "/" + asset, content);
    return this;
  }

  /** Serves an asset that never ends, as a hostile or broken origin would. */
  ReleaseServer endless(String githubRepo, String version, String asset) {
    endless.add("/" + githubRepo + "/releases/download/" + version + "/" + asset);
    return this;
  }

  /** Answers an asset with headers and then nothing, as a hung origin does. */
  ReleaseServer stalling(String githubRepo, String version, String asset) {
    stalling.add("/" + githubRepo + "/releases/download/" + version + "/" + asset);
    return this;
  }

  /** Answers the releases API with a tag, as {@code latest} resolution reads it. */
  ReleaseServer latest(String githubRepo, String tag) {
    assets.put(
        "/repos/" + githubRepo + "/releases/latest",
        ("{\"tag_name\": \"" + tag + "\", \"name\": \"" + tag + "\"}")
            .getBytes(StandardCharsets.UTF_8));
    return this;
  }

  /** Every path asked for, so a test can assert what was and was not fetched. */
  List<String> requested() {
    return requested;
  }

  String downloadBaseUrl() {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }

  String apiBaseUrl() {
    return downloadBaseUrl();
  }

  static String sha256(byte[] content) {
    try {
      return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(content));
    } catch (NoSuchAlgorithmException e) {
      throw new IllegalStateException(e);
    }
  }

  private void answer(HttpExchange exchange) throws IOException {
    String path = exchange.getRequestURI().getPath();
    synchronized (requested) {
      requested.add(path);
    }
    if (endless.contains(path)) {
      exchange.sendResponseHeaders(200, 0);
      try (OutputStream body = exchange.getResponseBody()) {
        byte[] chunk = new byte[64 * 1024];
        while (true) {
          body.write(chunk);
        }
      } catch (IOException gaveUp) {
        // The client gave up on a body it will not read to the end, which is the point.
      }
      return;
    }
    if (stalling.contains(path)) {
      exchange.sendResponseHeaders(200, 0);
      try (OutputStream body = exchange.getResponseBody()) {
        body.flush();
        stopped.await(STALL.toMillis(), TimeUnit.MILLISECONDS);
      } catch (IOException | InterruptedException interrupted) {
        Thread.currentThread().interrupt();
      }
      return;
    }
    byte[] content = assets.get(path);
    if (content == null) {
      exchange.sendResponseHeaders(404, -1);
      exchange.close();
      return;
    }
    exchange.sendResponseHeaders(200, content.length);
    try (OutputStream body = exchange.getResponseBody()) {
      body.write(content);
    }
  }

  @Override
  public void close() {
    stopped.countDown();
    server.stop(0);
  }
}

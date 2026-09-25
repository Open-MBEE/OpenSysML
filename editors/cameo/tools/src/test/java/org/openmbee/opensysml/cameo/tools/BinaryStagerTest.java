package org.openmbee.opensysml.cameo.tools;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import org.junit.jupiter.api.Test;

class BinaryStagerTest {
  @Test
  void stagesAllAssetsWithGoodDigests() throws IOException, InterruptedException {
    byte[] payload = "service".getBytes();
    String digest = BinaryStager.sha256(payload);
    StringBuilder json = new StringBuilder("{\"v1.2.3\":{");
    for (int i = 0; i < BinaryStager.assets().length; i++) {
      if (i > 0) json.append(',');
      json.append('"').append(BinaryStager.assets()[i]).append("\":\"").append(digest).append('"');
    }
    json.append("}}");
    HttpServer server = server(payload);
    try {
      Path pin = Files.createTempFile("digests", ".json");
      Files.writeString(pin, json);
      Map<String, String> written =
          BinaryStager.stage("v1.2.3", Files.createTempDirectory("staged"), base(server), pin);
      assertEquals(5, written.size());
    } finally {
      server.stop(0);
    }
  }

  @Test
  void rejectsTamperedAsset() throws IOException, InterruptedException {
    byte[] payload = "service".getBytes();
    String json = "{\"v1.2.3\":{\"sysml-grpc-linux-amd64\":\"" + BinaryStager.sha256("different".getBytes()) + "\"}}";
    HttpServer server = server(payload);
    try {
      Path pin = Files.createTempFile("digests", ".json");
      Files.writeString(pin, json);
      Path staged = Files.createTempDirectory("staged");
      assertThrows(Exception.class, () -> BinaryStager.stage("v1.2.3", staged, base(server), pin));
    } finally {
      server.stop(0);
    }
  }

  @Test
  void rejectsUnpinnedVersion() throws IOException {
    Path pin = Files.createTempFile("digests", ".json");
    Files.writeString(pin, "{\"v1.2.3\":{}}");
    Path staged = Files.createTempDirectory("staged");
    assertThrows(IllegalArgumentException.class, () -> BinaryStager.stage("v9.9.9", staged, "http://127.0.0.1", pin));
  }

  private static HttpServer server(byte[] payload) throws IOException {
    HttpServer server = HttpServer.create(new InetSocketAddress(0), 0);
    server.createContext("/", exchange -> {
      String path = exchange.getRequestURI().getPath();
      if (!path.startsWith("/Open-MBEE/OpenSysML/releases/download/v1.2.3/sysml-grpc-")) {
        exchange.sendResponseHeaders(404, -1);
        exchange.close();
        return;
      }
      exchange.sendResponseHeaders(200, payload.length);
      exchange.getResponseBody().write(payload);
      exchange.close();
    });
    server.start();
    return server;
  }

  private static String base(HttpServer server) {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }
}

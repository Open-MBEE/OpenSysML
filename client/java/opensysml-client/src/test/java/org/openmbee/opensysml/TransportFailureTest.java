package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpTimeoutException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Base64;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

/** What a caller is told when the answer is not the service's: a stall, a refusal, or a body. */
class TransportFailureTest {

  @Test
  void aCallThatOutlivesItsTimeoutIsADeadlineRatherThanAnUnreachableService() throws Exception {
    // A service that is reached and answers nothing must not look like one that is down: a caller
    // retrying UNAVAILABLE would repeat a call the service may still be running.
    try (Stub stalling =
        Stub.serving(
            exchange -> {
              drain(exchange);
              stall(Duration.ofSeconds(30));
            })) {
      ConnectionOptions options =
          ConnectionOptions.builder()
              .service("127.0.0.1", stalling.port())
              .requestTimeout(Duration.ofMillis(300))
              .build();
      TransportException failed =
          assertThrows(TransportException.class, () -> Connection.open(options));
      assertEquals(StatusCode.DEADLINE_EXCEEDED, failed.status());
      assertInstanceOf(HttpTimeoutException.class, failed.getCause());
      assertTrue(failed.getMessage().contains("did not answer"), failed.getMessage());
    }
  }

  @Test
  void aConnectErrorBodyNamesTheStatusTheCallerSees() throws Exception {
    try (Stub refusing =
        Stub.serving(
            exchange -> {
              drain(exchange);
              respond(
                  exchange,
                  404,
                  "application/json",
                  "{\"code\":\"not_found\",\"message\":\"no such model\"}");
            })) {
      ConnectionOptions options =
          ConnectionOptions.builder().service("127.0.0.1", refusing.port()).build();
      ServiceException refused = assertThrows(ServiceException.class, () -> Connection.open(options));
      assertEquals(StatusCode.NOT_FOUND, refused.status(), "the body's code outranks the HTTP status");
      assertTrue(refused.getMessage().contains("no such model"), refused.getMessage());
    }
  }

  @Test
  void anAnswerThatIsNotTheServicesIsATransportFailureRatherThanACorruptModel() throws Exception {
    try (Stub babbling =
        Stub.serving(
            exchange -> {
              drain(exchange);
              respond(exchange, 200, "text/html", "<html>a proxy answered</html>");
            })) {
      ConnectionOptions options =
          ConnectionOptions.builder().service("127.0.0.1", babbling.port()).build();
      TransportException failed =
          assertThrows(TransportException.class, () -> Connection.open(options));
      assertEquals(StatusCode.UNAVAILABLE, failed.status());
      assertTrue(failed.getMessage().contains("could not be decoded"), failed.getMessage());
    }
  }

  @Test
  void aQueryOutcomeDetailBecomesATypedException() throws Exception {
    // The detail is the wire message in base64, which Connect writes without padding; a null or a
    // detail of another type beside it is not the outcome.
    org.openmbee.opensysml.proto.QueryOutcome outcome =
        org.openmbee.opensysml.proto.QueryOutcome.newBuilder()
            .setKind("pathError")
            .setStep("Feature(mass)")
            .addFailures(
                org.openmbee.opensysml.proto.QueryFailure.newBuilder()
                    .setStep("Feature(mass)")
                    .setExpected("a feature named mass on the element")
                    .setFrontier(
                        org.openmbee.opensysml.proto.QueryFrontier.newBuilder()
                            .setAt("Lander::lander::cabinEngine")
                            .addFeatures(
                                org.openmbee.opensysml.proto.QueryFrontierFeature.newBuilder()
                                    .setName("length")
                                    .setType("LengthValue"))
                            .addKinds("connect")
                            .setNearest("length")))
            .build();
    String value = Base64.getEncoder().withoutPadding().encodeToString(outcome.toByteArray());
    String body =
        "{\"code\":\"failed_precondition\",\"message\":\"query reads feature mass\","
            + "\"details\":[null,{\"type\":\"google.protobuf.Empty\",\"value\":\"\"},"
            + "{\"type\":\"sysml.QueryOutcome\",\"value\":\""
            + value
            + "\"}]}";
    try (Stub refusing =
        Stub.serving(
            exchange -> {
              drain(exchange);
              respond(exchange, 412, "application/json", body);
            })) {
      ConnectionOptions options =
          ConnectionOptions.builder().service("127.0.0.1", refusing.port()).build();
      QueryOutcomeException refused =
          assertThrows(QueryOutcomeException.class, () -> Connection.open(options));
      assertEquals(StatusCode.FAILED_PRECONDITION, refused.status());
      assertTrue(refused.serviceMessage().endsWith("query reads feature mass"), refused.getMessage());
      assertEquals(
          new QueryOutcome(
              "pathError",
              "Feature(mass)",
              List.of(
                  new QueryOutcome.Failure(
                      "Feature(mass)",
                      "a feature named mass on the element",
                      new QueryOutcome.Frontier(
                          "Lander::lander::cabinEngine",
                          List.of(),
                          List.of(new QueryOutcome.Frontier.Feature("length", "LengthValue")),
                          List.of("connect"),
                          "length"))),
              List.of(),
              List.of()),
          refused.outcome());
    }
  }

  @Test
  void anOutcomeDetailThatWillNotDecodeLeavesTheStatusToTheCaller() throws Exception {
    // The status is the service's answer; a detail this client cannot read does not hide it.
    String body =
        "{\"code\":\"failed_precondition\",\"message\":\"query reads feature mass\","
            + "\"details\":[{\"type\":\"sysml.QueryOutcome\",\"value\":\"!!\"}]}";
    try (Stub refusing =
        Stub.serving(
            exchange -> {
              drain(exchange);
              respond(exchange, 412, "application/json", body);
            })) {
      ConnectionOptions options =
          ConnectionOptions.builder().service("127.0.0.1", refusing.port()).build();
      ServiceException refused = assertThrows(ServiceException.class, () -> Connection.open(options));
      assertEquals(ServiceException.class, refused.getClass());
      assertEquals(StatusCode.FAILED_PRECONDITION, refused.status());
      assertTrue(refused.getMessage().contains("query reads feature mass"), refused.getMessage());
    }
  }

  private static void drain(HttpExchange exchange) throws IOException {
    try (InputStream body = exchange.getRequestBody()) {
      body.readAllBytes();
    }
  }

  private static void respond(HttpExchange exchange, int status, String contentType, String body)
      throws IOException {
    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
    exchange.getResponseHeaders().add("Content-Type", contentType);
    exchange.sendResponseHeaders(status, bytes.length);
    try (OutputStream out = exchange.getResponseBody()) {
      out.write(bytes);
    }
  }

  /** Holds the exchange open without answering, as a hung service does. */
  private static void stall(Duration duration) {
    try {
      new CountDownLatch(1).await(duration.toMillis(), TimeUnit.MILLISECONDS);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
    }
  }

  /** An HTTP server standing in for the service, answering whatever a test needs it to. */
  private static final class Stub implements AutoCloseable {

    private final HttpServer server;

    private Stub(HttpServer server) {
      this.server = server;
    }

    static Stub serving(HttpHandler handler) throws IOException {
      HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
      server.createContext("/", handler);
      server.start();
      return new Stub(server);
    }

    int port() {
      return server.getAddress().getPort();
    }

    @Override
    public void close() {
      server.stop(0);
    }
  }
}

package org.openmbee.opensysml.internal;

import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Message;
import org.openmbee.opensysml.Encoding;
import org.openmbee.opensysml.StatusCode;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.TransportException;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpConnectTimeoutException;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Unary calls over the Connect protocol: a POST to {@code /sysml.SysMLService/<Method>} carrying a
 * protobuf (or JSON) body.
 *
 * <p>Built on {@code java.net.http}, so the client needs no HTTP stack of its own and no Netty:
 * nothing here can conflict with a host application's own transitive gRPC or Netty version.
 */
public final class ConnectTransport implements AutoCloseable {

  private static final String SERVICE = "sysml.SysMLService";

  private final String baseUrl;
  private final Encoding encoding;
  private final Duration requestTimeout;
  private final ExecutorService executor;
  private final HttpClient http;

  /**
   * Opens a transport against a listening service.
   *
   * @param address the service's {@code host:port}
   * @param encoding body encoding to send and accept
   * @param requestTimeout how long one call may take
   */
  public ConnectTransport(String address, Encoding encoding, Duration requestTimeout) {
    this.baseUrl = "http://" + Objects.requireNonNull(address, "address");
    this.encoding = Objects.requireNonNull(encoding, "encoding");
    this.requestTimeout = Objects.requireNonNull(requestTimeout, "requestTimeout");
    if (encoding == Encoding.JSON) {
      JsonBodies.requireOnClasspath();
    }
    // Daemon threads of our own, so an unloaded classloader leaves nothing running
    // and a JVM shutdown is never held up by the client.
    this.executor =
        Executors.newCachedThreadPool(
            runnable -> {
              Thread thread = new Thread(runnable, "opensysml-http");
              thread.setDaemon(true);
              return thread;
            });
    this.http =
        HttpClient.newBuilder()
            .executor(executor)
            .connectTimeout(requestTimeout)
            .version(HttpClient.Version.HTTP_1_1)
            .build();
  }

  /**
   * Calls one method of the service.
   *
   * @param <T> the response message type
   * @param method bare method name ({@code "Evaluate"})
   * @param request the request message
   * @param responseDefault default instance of the response message
   * @return the answer
   * @throws ServiceException if the call was refused
   * @throws TransportException if the service could not be reached, the call outlived its timeout,
   *     or the answer was unreadable
   */
  public <T extends Message> T call(String method, Message request, T responseDefault) {
    Objects.requireNonNull(method, "method");
    Objects.requireNonNull(request, "request");
    Objects.requireNonNull(responseDefault, "responseDefault");

    byte[] body =
        encoding == Encoding.JSON
            ? JsonBodies.serialize(request).getBytes(StandardCharsets.UTF_8)
            : request.toByteArray();
    HttpRequest httpRequest =
        HttpRequest.newBuilder(URI.create(baseUrl + "/" + SERVICE + "/" + method))
            .timeout(requestTimeout)
            .header("Content-Type", encoding.contentType())
            .header("Connect-Protocol-Version", "1")
            .POST(HttpRequest.BodyPublishers.ofByteArray(body))
            .build();

    HttpResponse<byte[]> response;
    try {
      response = http.send(httpRequest, HttpResponse.BodyHandlers.ofByteArray());
    } catch (HttpConnectTimeoutException e) {
      throw new TransportException(method + " could not be called at " + baseUrl, e);
    } catch (HttpTimeoutException e) {
      // The service was reached and may still be running the call, so this is not UNAVAILABLE.
      throw new TransportException(
          StatusCode.DEADLINE_EXCEEDED,
          method + " did not answer within " + requestTimeout + " at " + baseUrl,
          e);
    } catch (IOException e) {
      throw new TransportException(method + " could not be called at " + baseUrl, e);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      throw new TransportException(method + " was interrupted", e);
    }

    if (response.statusCode() / 100 != 2) {
      throw refusal(method, response);
    }
    return decode(method, response.body(), responseDefault);
  }

  private <T extends Message> T decode(String method, byte[] body, T responseDefault) {
    try {
      if (encoding == Encoding.JSON) {
        return JsonBodies.parse(new String(body, StandardCharsets.UTF_8), responseDefault);
      }
      // The parser of a message's own default instance answers that message type.
      @SuppressWarnings("unchecked")
      T response = (T) responseDefault.getParserForType().parseFrom(body);
      return response;
    } catch (InvalidProtocolBufferException e) {
      throw new TransportException(method + " answered a body that could not be decoded", e);
    }
  }

  private ServiceException refusal(String method, HttpResponse<byte[]> response) {
    String text = new String(response.body(), StandardCharsets.UTF_8);
    StatusCode status = StatusCode.fromHttpStatus(response.statusCode());
    String message = text;
    try {
      Object error = Json.parse(text);
      String code = Json.stringMember(error, "code");
      if (!code.isEmpty()) {
        status = StatusCode.fromConnectCode(code);
      }
      String reported = Json.stringMember(error, "message");
      if (!reported.isEmpty()) {
        message = reported;
      }
    } catch (IllegalArgumentException e) {
      // Not a Connect error body; the HTTP status and the text are the whole answer.
    }
    return new ServiceException(status, method + ": " + message);
  }

  /** Releases the threads this transport owns. The service, if any, is not touched. */
  @Override
  public void close() {
    executor.shutdownNow();
  }
}

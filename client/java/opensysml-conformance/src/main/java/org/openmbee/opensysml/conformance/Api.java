package org.openmbee.opensysml.conformance;

import com.google.protobuf.Message;
import org.openmbee.opensysml.Capabilities;
import org.openmbee.opensysml.CapabilityException;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Language;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.ModelException;
import org.openmbee.opensysml.ParseOptions;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.DiagnosticsRequest;
import org.openmbee.opensysml.proto.DiagnosticsResponse;
import org.openmbee.opensysml.proto.EvaluateRequest;
import org.openmbee.opensysml.proto.EvaluateResponse;
import org.openmbee.opensysml.proto.GetSymbolRequest;
import org.openmbee.opensysml.proto.InstantiateRequest;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.ParseFileRequest;
import org.openmbee.opensysml.proto.ParseFileResponse;
import org.openmbee.opensysml.proto.ServerInfoRequest;
import org.openmbee.opensysml.proto.ServerInfoResponse;
import org.openmbee.opensysml.proto.SymbolResponse;
import org.openmbee.opensysml.proto.Sysml;
import java.nio.file.Path;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;

/**
 * Makes a scenario's call through the client's public API and writes the answer back as the
 * response message the scenario is stated against. An RPC v1 does not cover is reported as such
 * rather than called over the transport, which is what the skipped count in a report counts.
 */
final class Api {

  private static final String RPC_EVALUATE = "Evaluate";
  private static final String RPC_GET_DIAGNOSTICS = "GetDiagnostics";
  private static final String RPC_GET_SERVER_INFO = "GetServerInfo";
  private static final String RPC_GET_SYMBOL = "GetSymbol";
  private static final String RPC_INSTANTIATE = "Instantiate";
  private static final String RPC_PARSE_FILE = "ParseFile";

  /** The RPCs the v1 API covers, and so the ones a scenario can be run through it. */
  static final Set<String> COVERED =
      Set.of(
          RPC_GET_SERVER_INFO,
          RPC_PARSE_FILE,
          RPC_GET_SYMBOL,
          RPC_GET_DIAGNOSTICS,
          RPC_EVALUATE,
          RPC_INSTANTIATE);

  private final Connection connection;

  Api(Connection connection) {
    this.connection = connection;
  }

  /** What a call answered: a response, a refusal carrying a status, or nothing the API can do. */
  sealed interface Answer {
    /** The service answered. */
    record Answered(Message response) implements Answer {}

    /** The call was refused with a status. */
    record Refused(String status, String message) implements Answer {}

    /** The v1 API cannot make this call, so the scenario is skipped. */
    record Unsupported(String reason) implements Answer {}
  }

  /**
   * An empty request of the method's input type, which the scenario's protobuf JSON is merged into.
   *
   * @param method the bare method name
   * @return a builder of that method's request
   */
  static Message.Builder request(String method) {
    return switch (method) {
      case RPC_GET_SERVER_INFO -> ServerInfoRequest.newBuilder();
      case RPC_PARSE_FILE -> ParseFileRequest.newBuilder();
      case RPC_GET_SYMBOL -> GetSymbolRequest.newBuilder();
      case RPC_GET_DIAGNOSTICS -> DiagnosticsRequest.newBuilder();
      case RPC_EVALUATE -> EvaluateRequest.newBuilder();
      case RPC_INSTANTIATE -> InstantiateRequest.newBuilder();
      default -> throw new IllegalArgumentException("no request type for " + method);
    };
  }

  /**
   * Whether the service declares an RPC at all, so a scenario naming one that does not exist is an
   * error in the suite rather than a skip.
   *
   * @param method the bare method name
   * @return whether {@code sysml.SysMLService} declares it
   */
  static boolean declared(String method) {
    return Sysml.getDescriptor().getServices().stream()
        .filter(service -> service.getFullName().equals("sysml.SysMLService"))
        .anyMatch(service -> service.findMethodByName(method) != null);
  }

  /**
   * Makes one call.
   *
   * @param method the bare method name
   * @param request the request the scenario named
   * @return what it answered
   */
  Answer call(String method, Message request) {
    if (!COVERED.contains(method)) {
      return new Answer.Unsupported("the v1 API does not cover " + method);
    }
    try {
      return new Answer.Answered(
          switch (method) {
            case RPC_GET_SERVER_INFO -> serverInfo();
            case RPC_PARSE_FILE -> parse((ParseFileRequest) request);
            case RPC_GET_SYMBOL -> symbol((GetSymbolRequest) request);
            case RPC_GET_DIAGNOSTICS -> diagnostics((DiagnosticsRequest) request);
            case RPC_EVALUATE -> evaluate((EvaluateRequest) request);
            case RPC_INSTANTIATE -> instantiate((InstantiateRequest) request);
            default -> throw new IllegalStateException(method);
          });
    } catch (Unsupported e) {
      return new Answer.Unsupported(e.getMessage());
    } catch (ServiceException e) {
      return new Answer.Refused(e.status().name(), e.serviceMessage());
    } catch (CapabilityException e) {
      // The client refuses a call whose behaviour the service does not advertise, since the
      // service would ignore the field rather than answer UNIMPLEMENTED.
      return new Answer.Refused("UNIMPLEMENTED", e.getMessage());
    }
  }

  /** A call the v1 API has no way to make, thrown where the request is read. */
  private static final class Unsupported extends RuntimeException {
    private static final long serialVersionUID = 1L;

    Unsupported(String reason) {
      super(reason);
    }
  }

  private ServerInfoResponse serverInfo() {
    Capabilities capabilities = connection.capabilities();
    return ServerInfoResponse.newBuilder()
        .setVersion(capabilities.serviceVersion())
        .addAllCapabilities(new TreeSet<>(capabilities.names()))
        .build();
  }

  private ParseFileResponse parse(ParseFileRequest request) {
    ParseOptions options =
        new ParseOptions(Language.fromWireName(request.getLanguage()), request.getStrictConformance());
    Model model =
        switch (request.getSourceCase()) {
          case FILE_PATH -> connection.load(Path.of(request.getFilePath()), options);
          case CONTENT -> connection.parse(request.getContent(), options);
          case SOURCE_NOT_SET ->
              throw new Unsupported(
                  "the v1 API always names a source, so it cannot send a request naming none");
        };
    ParseFileResponse.Builder response =
        ParseFileResponse.newBuilder()
            .setModelHash(model.hash())
            .addAllDiagnostics(Rendering.diagnostics(model.parseDiagnostics()));
    model.root().ifPresent(root -> response.setRoot(Rendering.symbol(root)));
    return response.build();
  }

  private SymbolResponse symbol(GetSymbolRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      Symbol symbol = model.symbol(request.getSymbolId());
      return SymbolResponse.newBuilder().setSymbol(Rendering.symbol(symbol)).build();
    } catch (ModelException e) {
      return SymbolResponse.newBuilder().setError(e.getMessage()).build();
    }
  }

  private DiagnosticsResponse diagnostics(DiagnosticsRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      return DiagnosticsResponse.newBuilder()
          .addAllDiagnostics(Rendering.diagnostics(model.diagnostics()))
          .build();
    } catch (ModelException e) {
      return DiagnosticsResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private EvaluateResponse evaluate(EvaluateRequest request) {
    Model model = connection.model(request.getModelHash());
    boolean hasContext = !request.getContextSymbolId().isEmpty();
    boolean hasSubject = !request.getSubjectSymbolId().isEmpty();
    if (hasContext && hasSubject) {
      throw new Unsupported("the v1 API evaluates in a context or against a subject, not both");
    }
    try {
      Value value;
      if (hasSubject) {
        value = model.evalWithSubject(request.getExpression(), request.getSubjectSymbolId());
      } else if (hasContext) {
        value = model.evalInContext(request.getExpression(), request.getContextSymbolId());
      } else {
        value = model.eval(request.getExpression());
      }
      return EvaluateResponse.newBuilder().setResult(Rendering.value(value)).build();
    } catch (ModelException e) {
      return EvaluateResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private InstantiateResponse instantiate(InstantiateRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      Instantiation instantiation = model.instantiate(request.getSymbolId());
      List<org.openmbee.opensysml.proto.Instance> reachable =
          instantiation.reachable().stream().map(Rendering::instance).toList();
      return InstantiateResponse.newBuilder()
          .setInstance(Rendering.instance(instantiation.root()))
          .addAllInstances(reachable)
          .addAllDiagnostics(Rendering.diagnostics(instantiation.diagnostics()))
          .build();
    } catch (ModelException e) {
      return InstantiateResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }
}

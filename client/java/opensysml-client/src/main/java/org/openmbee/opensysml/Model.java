package org.openmbee.opensysml;

import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.proto.DiagnosticsRequest;
import org.openmbee.opensysml.proto.DiagnosticsResponse;
import org.openmbee.opensysml.proto.EvaluateRequest;
import org.openmbee.opensysml.proto.EvaluateResponse;
import org.openmbee.opensysml.proto.GetSymbolRequest;
import org.openmbee.opensysml.proto.InstantiateRequest;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.SymbolResponse;
import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A model the service has parsed, named by the hash every later call carries.
 *
 * <p>Obtained from {@link Connection#load(java.nio.file.Path)}, {@link Connection#parse(String)} or
 * {@link Connection#model(String)}. Immutable and thread-safe; it holds no state of its own beyond
 * the hash and what the parse reported.
 */
public final class Model {

  private final Connection connection;
  private final String hash;
  private final Optional<Symbol> root;
  private final List<Diagnostic> parseDiagnostics;

  Model(
      Connection connection,
      String hash,
      Optional<Symbol> root,
      List<Diagnostic> parseDiagnostics) {
    this.connection = connection;
    this.hash = hash;
    this.root = root;
    this.parseDiagnostics = List.copyOf(parseDiagnostics);
  }

  /**
   * The hash the service knows this model by.
   *
   * @return the model hash
   */
  public String hash() {
    return hash;
  }

  /**
   * The connection this model is read over.
   *
   * @return the connection
   */
  public Connection connection() {
    return connection;
  }

  /**
   * The root namespace of the parse, absent for a model addressed by hash alone.
   *
   * @return the root symbol
   */
  public Optional<Symbol> root() {
    return root;
  }

  /**
   * What the parse that produced this model reported. Empty for a model addressed by hash alone,
   * where {@link #diagnostics()} asks the service instead.
   *
   * @return the diagnostics of the parse, in order
   */
  public List<Diagnostic> parseDiagnostics() {
    return parseDiagnostics;
  }

  /**
   * Asks the service for this model's diagnostics.
   *
   * @return the diagnostics, in order
   * @throws ServiceException if the service does not hold this model
   * @throws ModelException if the service reported a failure in its answer
   */
  public List<Diagnostic> diagnostics() {
    DiagnosticsResponse response =
        connection.call(
            "GetDiagnostics",
            DiagnosticsRequest.newBuilder().setModelHash(hash).build(),
            DiagnosticsResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    return diagnostics;
  }

  /**
   * A symbol by qualified name.
   *
   * @param symbolId a qualified name, such as {@code "Demo::Vehicle"}
   * @return the symbol
   * @throws ModelException if the model declares no such symbol
   * @throws ServiceException if the service does not hold this model
   */
  public Symbol symbol(String symbolId) {
    SymbolResponse response = symbolResponse(symbolId);
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), List.of());
    }
    return Protos.symbol(response.getSymbol());
  }

  /**
   * A symbol by qualified name, absent when the model declares no such symbol.
   *
   * @param symbolId a qualified name
   * @return the symbol, or empty
   * @throws ServiceException if the service does not hold this model
   */
  public Optional<Symbol> findSymbol(String symbolId) {
    SymbolResponse response = symbolResponse(symbolId);
    return response.getError().isEmpty()
        ? Optional.of(Protos.symbol(response.getSymbol()))
        : Optional.empty();
  }

  /**
   * Evaluates an expression against the model's declarations.
   *
   * @param expression a SysML expression, such as {@code "2 + 2"}
   * @return what it evaluated to
   * @throws ModelException if the expression could not be evaluated
   * @throws ServiceException if the service does not hold this model
   */
  public Value eval(String expression) {
    return evaluated(request(expression).build());
  }

  /**
   * Evaluates an expression in the scope of a symbol, so its features are in scope and a feature
   * reads the declared default.
   *
   * @param expression a SysML expression
   * @param contextSymbolId qualified name of the scope to evaluate in
   * @return what it evaluated to
   */
  public Value evalInContext(String expression, String contextSymbolId) {
    Objects.requireNonNull(contextSymbolId, "contextSymbolId");
    return evaluated(request(expression).setContextSymbolId(contextSymbolId).build());
  }

  /**
   * Evaluates an expression against an object of a symbol, so a feature reads that object's value
   * rather than the declared default.
   *
   * @param expression a SysML expression
   * @param subjectSymbolId qualified name of the symbol to instantiate and evaluate against
   * @return what it evaluated to
   * @throws CapabilityException if the service does not advertise {@code evaluate_subject}, which it
   *     would otherwise ignore rather than refuse
   */
  public Value evalWithSubject(String expression, String subjectSymbolId) {
    Objects.requireNonNull(subjectSymbolId, "subjectSymbolId");
    connection.capabilities().require(Capabilities.EVALUATE_SUBJECT);
    return evaluated(request(expression).setSubjectSymbolId(subjectSymbolId).build());
  }

  /**
   * Builds an object of a part definition or usage, and everything reachable from it.
   *
   * @param symbolId qualified name of the definition or usage to instantiate
   * @return the object built
   * @throws ModelException if it could not be built
   * @throws ServiceException if the service does not hold this model
   */
  public Instantiation instantiate(String symbolId) {
    Objects.requireNonNull(symbolId, "symbolId");
    InstantiateResponse response =
        connection.call(
            "Instantiate",
            InstantiateRequest.newBuilder().setModelHash(hash).setSymbolId(symbolId).build(),
            InstantiateResponse.getDefaultInstance());
    if (!response.getError().isEmpty()) {
      throw new ModelException(
          response.getError(), Protos.diagnostics(response.getDiagnosticsList()));
    }
    return Protos.instantiation(response);
  }

  private SymbolResponse symbolResponse(String symbolId) {
    Objects.requireNonNull(symbolId, "symbolId");
    return connection.call(
        "GetSymbol",
        GetSymbolRequest.newBuilder().setModelHash(hash).setSymbolId(symbolId).build(),
        SymbolResponse.getDefaultInstance());
  }

  private EvaluateRequest.Builder request(String expression) {
    Objects.requireNonNull(expression, "expression");
    return EvaluateRequest.newBuilder().setModelHash(hash).setExpression(expression);
  }

  private Value evaluated(EvaluateRequest request) {
    EvaluateResponse response =
        connection.call("Evaluate", request, EvaluateResponse.getDefaultInstance());
    List<Diagnostic> diagnostics = Protos.diagnostics(response.getDiagnosticsList());
    if (!response.getError().isEmpty()) {
      throw new ModelException(response.getError(), diagnostics);
    }
    return Protos.value(response.getResult())
        .orElseThrow(
            () ->
                new ModelException(
                    "the service answered neither a value nor a failure for "
                        + request.getExpression(),
                    diagnostics));
  }
}

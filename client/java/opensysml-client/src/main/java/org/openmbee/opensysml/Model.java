package org.openmbee.opensysml;

import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.proto.ApplyEditsRequest;
import org.openmbee.opensysml.proto.ApplyEditsResponse;
import org.openmbee.opensysml.proto.ConvertRequest;
import org.openmbee.opensysml.proto.ConvertResponse;
import org.openmbee.opensysml.proto.DiagnosticsRequest;
import org.openmbee.opensysml.proto.DiagnosticsResponse;
import org.openmbee.opensysml.proto.EvaluateCalcRequest;
import org.openmbee.opensysml.proto.EvaluateCalcResponse;
import org.openmbee.opensysml.proto.EvaluateRequest;
import org.openmbee.opensysml.proto.EvaluateResponse;
import org.openmbee.opensysml.proto.ExecuteActionRequest;
import org.openmbee.opensysml.proto.ExecuteActionResponse;
import org.openmbee.opensysml.proto.ExecuteStateRequest;
import org.openmbee.opensysml.proto.ExecuteStateResponse;
import org.openmbee.opensysml.proto.GetSymbolRequest;
import org.openmbee.opensysml.proto.InstantiateRequest;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.QueryRequest;
import org.openmbee.opensysml.proto.QueryResponse;
import org.openmbee.opensysml.proto.RenderDocumentRequest;
import org.openmbee.opensysml.proto.RenderDocumentResponse;
import org.openmbee.opensysml.proto.RenderViewRequest;
import org.openmbee.opensysml.proto.RenderViewResponse;
import org.openmbee.opensysml.proto.RunAnalysisRequest;
import org.openmbee.opensysml.proto.RunAnalysisResponse;
import org.openmbee.opensysml.proto.RunDocumentQueryRequest;
import org.openmbee.opensysml.proto.RunDocumentQueryResponse;
import org.openmbee.opensysml.proto.RunSweepRequest;
import org.openmbee.opensysml.proto.RunSweepResponse;
import org.openmbee.opensysml.proto.SymbolResponse;
import org.openmbee.opensysml.proto.ValidateInstanceRequest;
import org.openmbee.opensysml.proto.ValidateInstanceResponse;
import org.openmbee.opensysml.proto.VerifyConstraintRequest;
import org.openmbee.opensysml.proto.VerifyConstraintResponse;
import org.openmbee.opensysml.proto.VerifyRequirementRequest;
import org.openmbee.opensysml.proto.VerifyRequirementResponse;
import org.openmbee.opensysml.proto.VerifySatisfactionRequest;
import org.openmbee.opensysml.proto.VerifySatisfactionResponse;
import java.nio.file.Path;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import java.util.TreeSet;

/**
 * A model the service has parsed, named by the hash every later call carries.
 *
 * <p>Obtained from {@link Connection#load(java.nio.file.Path)}, {@link Connection#parse(String)},
 * {@link Connection#parseSources(List)} or {@link Connection#model(String)}. Immutable and thread-safe; it holds no state of its own beyond
 * the hash, what the parse reported and the engine {@link #withEngine(String)} named.
 *
 * <p>Reading a model is {@link #eval}, {@link #symbol} and {@link #instantiate}. Running it is
 * {@link #executeAction} and {@link #executeState} for one run under a schedule, {@link
 * #exploreAction} and {@link #exploreState} for every run. Checking it is {@link
 * #verifyConstraint}, {@link #verifyRequirement}, {@link #verifySatisfaction} and {@link
 * #validateInstance}, which answer verdicts: a condition the model answers false about is a
 * {@link Verdict} that does not hold, not an exception. {@link #evaluateCalc} and {@link
 * #runAnalysis} compute, and {@link #query} selects elements.
 *
 * <p>A model also edits and projects itself: {@link #applyEdits(List, EditOptions)} applies
 * element-level edits to its source text and answers the text they produce, {@link
 * #convert(String, ConversionOptions)} rewrites it in another format, {@link #runSweep(String,
 * List, SweepOptions)} runs a case, a verification or an analysis once per combination of its
 * swept parameters, {@link #runDocumentQuery(String, Map)} projects its elements through a
 * document query, and {@link #renderDocument(String, DocumentForm)} renders a Markdown or HTML
 * view over it. An
 * apply-edits refusal is an {@link EditException}, a {@link ModelException} carrying the refusal's
 * {@link EditFailure} and referrers.
 *
 * <p>Every call answers what the service reported or throws: {@link ModelException} when the
 * service answered but reported a failure about the model, {@link ServiceException} when it refused
 * the call, {@link CapabilityException} before anything is sent when the service does not advertise
 * what the call needs.
 */
public final class Model {
  private static final String EXPLORE = "explore";
  private static final String NAME_SUBJECT_SYMBOL_ID = "subjectSymbolId";
  private static final String NAME_SYMBOL_ID = "symbolId";
  private static final String NAME_OPTIONS = "options";
  private static final String PROPERTY_ID = "@id";
  private static final String PROPERTY_NAME = "name";
  private static final String PROPERTY_OWNER = "owner";

  private final Connection connection;
  private final String hash;
  private final List<Symbol> roots;
  private final List<Diagnostic> parseDiagnostics;
  private final List<String> documents;
  private final Optional<String> engine;

  Model(Connection connection, String hash, List<Symbol> roots, List<Diagnostic> parseDiagnostics) {
    this(connection, hash, roots, parseDiagnostics, List.of(), Optional.empty());
  }

  Model(
      Connection connection,
      String hash,
      List<Symbol> roots,
      List<Diagnostic> parseDiagnostics,
      List<String> documents) {
    this(connection, hash, roots, parseDiagnostics, documents, Optional.empty());
  }

  private Model(
      Connection connection,
      String hash,
      List<Symbol> roots,
      List<Diagnostic> parseDiagnostics,
      List<String> documents,
      Optional<String> engine) {
    this.connection = connection;
    this.hash = hash;
    this.roots = List.copyOf(roots);
    this.parseDiagnostics = List.copyOf(parseDiagnostics);
    this.documents = List.copyOf(documents);
    this.engine = engine;
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
   * The model's outermost elements, in document order: one per document a {@link
   * Connection#parseSources(List)} model carries, the one root a one-document model or a handle
   * addressed by hash declares, none for an empty model.
   *
   * @return the roots, never {@code null}
   */
  public List<Symbol> roots() {
    return roots;
  }

  /**
   * The root namespace of the parse: the first of {@link #roots()}, which is the only one for a
   * one-document model.
   *
   * @return the root symbol, absent for an empty model or a model addressed by hash alone
   */
  public Optional<Symbol> root() {
    return roots.isEmpty() ? Optional.empty() : Optional.of(roots.get(0));
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
   * The name of each document, in document order, as diagnostics report it: each path or name a
   * {@link Connection#parseSources(List)} model was given, the path a loaded file was read from,
   * none for inline notation or a model addressed by hash alone.
   *
   * @return the document names
   */
  public List<String> documents() {
    return documents;
  }

  /**
   * The error-severity diagnostics of the parse, which are what makes a model unusable.
   *
   * @return the errors among {@link #parseDiagnostics()}, in report order
   */
  public List<Diagnostic> errors() {
    return parseDiagnostics.stream()
        .filter(diagnostic -> diagnostic.severity() == Diagnostic.Severity.ERROR)
        .toList();
  }

  /**
   * Whether the parse reported no errors. A model with errors is still navigable, but its symbols
   * may be missing or unresolved, so check this before treating it as the model that was written.
   *
   * @return {@code true} when no parse diagnostic has error severity
   */
  public boolean ok() {
    return errors().isEmpty();
  }

  /**
   * Requires a parse without errors, for chaining onto a load.
   *
   * @return this model
   * @throws ModelException if the parse reported errors; it carries them as its diagnostics
   */
  public Model requireNoErrors() {
    List<Diagnostic> errors = errors();
    if (errors.isEmpty()) {
      return this;
    }
    String where = documents.size() == 1 ? documents.get(0) : "the model";
    StringBuilder summary = new StringBuilder();
    for (int i = 0; i < Math.min(3, errors.size()); i++) {
      summary.append(i == 0 ? "" : "; ").append(errors.get(i).message());
    }
    if (errors.size() > 3) {
      summary.append("; ... and ").append(errors.size() - 3).append(" more");
    }
    throw new ModelException(
        where + " has " + errors.size() + " error(s): " + summary, errors);
  }

  /**
   * The engine this model's verifications, calculations and analyses are put to.
   *
   * @return the engine, absent when the service chooses
   */
  public Optional<String> engine() {
    return engine;
  }

  /**
   * The same model, its verifications, calculations and analyses put to one engine: an engine's
   * name as {@link Connection#listEngines()} reports it, {@link Standing#ENGINE_ALL} for every
   * engine that covers the question, or {@link Standing#ENGINE_AUTO} to leave the choice to the
   * service. A name the service does not register fails the call with {@link
   * StatusCode#INVALID_ARGUMENT}. The {@code "explore"} engine answers a question with every
   * outcome rather than one run's, so it needs {@code schedule_explore} beside {@code engines}.
   *
   * @param engine the engine
   * @return a model bound to it
   * @throws CapabilityException if the service does not advertise {@code engines}, which it would
   *     otherwise ignore rather than refuse, or the engine is {@code "explore"} and the service
   *     does not advertise {@code schedule_explore}
   */
  public Model withEngine(String engine) {
    Objects.requireNonNull(engine, "engine");
    connection.capabilities().require(Capabilities.ENGINES);
    if (engine.equals(EXPLORE)) {
      connection.capabilities().require(Capabilities.SCHEDULE_EXPLORE);
    }
    return new Model(connection, hash, roots, parseDiagnostics, documents, Optional.of(engine));
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
   * A symbol by short name or qualified name, answered from the service's index.
   *
   * <p>A symbol's own {@link Symbol#id()} is accepted as well as its short name. Several symbols
   * may share a short name: the outermost wins, and among those the one declared first. A name
   * declared in the model wins over a library symbol whose id it is.
   *
   * @param name a short name ({@code "Vehicle"}) or a qualified one ({@code "Demo::Vehicle"})
   * @return the symbol, or empty
   * @throws CapabilityException if the model has no roots to walk, as an adopted one, and the
   *     service lacks {@code query}
   * @throws ServiceException if the service does not hold this model
   */
  public Optional<Symbol> find(String name) {
    Objects.requireNonNull(name, "name");
    for (Symbol root : roots) {
      if (root.name().equals(name) || root.id().equals(name)) {
        return Optional.of(root);
      }
    }
    if (name.contains("::")) {
      Optional<Symbol> byId = symbolWithId(name);
      return byId.isPresent() ? byId : symbolNamed(name);
    }
    Optional<Symbol> named = symbolNamed(name);
    return named.isPresent() ? named : symbolWithId(name);
  }

  /**
   * A symbol by its {@link Symbol#id()} only, absent when no symbol carries that id.
   *
   * @param id a qualified name
   * @return the symbol, or empty
   * @throws ServiceException if the service does not hold this model
   */
  public Optional<Symbol> get(String id) {
    Objects.requireNonNull(id, "id");
    for (Symbol root : roots) {
      if (root.id().equals(id)) {
        return Optional.of(root);
      }
    }
    return symbolWithId(id);
  }

  /**
   * The raising counterpart of {@link #find(String)}.
   *
   * @param name a short name or a qualified one
   * @return the symbol
   * @throws SymbolNotFoundException if the model declares no such symbol, naming near names
   * @throws ServiceException if the service does not hold this model
   */
  public Symbol lookup(String name) {
    return find(name).orElseThrow(() -> new SymbolNotFoundException(name, nearNames(name)));
  }

  /**
   * Whether a short name or qualified name names a symbol in this model.
   *
   * @param name a short name or a qualified one
   * @return {@code true} when {@link #find(String)} finds it
   * @throws ServiceException if the service does not hold this model
   */
  public boolean contains(String name) {
    return find(name).isPresent();
  }

  private List<String> nearNames(String name) {
    Map<String, String> declared = new LinkedHashMap<>();
    if (connection.capabilities().has(Capabilities.QUERY)) {
      for (QueryElement element : query(Query.all().withSelect(List.of(PROPERTY_NAME)))) {
        declared.put(element.id(), element.properties().getOrDefault(PROPERTY_NAME, ""));
      }
    } else {
      walk(symbol -> {
        declared.put(symbol.id(), symbol.name());
        return false;
      });
    }
    List<String> candidates = new ArrayList<>();
    declared.forEach(
        (id, shortName) -> {
          if (!shortName.isEmpty()) {
            candidates.add(shortName);
          }
          if (!id.isEmpty() && !id.equals(shortName)) {
            candidates.add(id);
          }
        });
    return NearNames.closest(name, candidates, 3);
  }

  private Optional<Symbol> symbolWithId(String id) {
    return findSymbol(id).filter(symbol -> symbol.id().equals(id));
  }

  private Optional<Symbol> symbolNamed(String name) {
    if (!connection.capabilities().has(Capabilities.QUERY)) {
      return walkTo(name);
    }
    List<QueryElement> named = queryProperty(PROPERTY_NAME, List.of(name));
    List<String> ids = new ArrayList<>(named.stream().map(QueryElement::id).toList());
    if (ids.size() > 1) {
      Map<String, Integer> depth = depths(named);
      ids.sort(Comparator.comparing(depth::get));
    }
    if (!ok()) {
      Optional<Symbol> walked = walkTo(name);
      if (walked.isPresent()) {
        return walked;
      }
    }
    for (String id : ids) {
      Optional<Symbol> symbol = symbolWithId(id);
      if (symbol.isPresent()) {
        return symbol;
      }
    }
    return Optional.empty();
  }

  private List<QueryElement> queryProperty(String property, List<String> values) {
    return query(
        Query.all()
            .withSelect(List.of(PROPERTY_OWNER))
            .where(Condition.equalTo(property, values)));
  }

  private Map<String, Integer> depths(List<QueryElement> elements) {
    Map<String, String> owner = new HashMap<>();
    roots.forEach(root -> owner.put(root.id(), ""));
    elements.forEach(e -> owner.put(e.id(), e.properties().getOrDefault(PROPERTY_OWNER, "")));
    Set<String> unknown = unknownOwners(owner);
    while (!unknown.isEmpty()) {
      for (QueryElement element : queryProperty(PROPERTY_ID, new ArrayList<>(new TreeSet<>(unknown)))) {
        owner.put(element.id(), element.properties().getOrDefault(PROPERTY_OWNER, ""));
      }
      unknown.forEach(id -> owner.putIfAbsent(id, ""));
      unknown = unknownOwners(owner);
    }
    Set<String> rootIds = new HashSet<>();
    roots.forEach(root -> rootIds.add(root.id()));
    Map<String, Integer> depth = new HashMap<>();
    for (QueryElement element : elements) {
      String id = element.id();
      int hops = 0;
      while (!owner.getOrDefault(id, "").isEmpty()) {
        id = owner.get(id);
        hops++;
      }
      depth.put(element.id(), rootIds.contains(id) ? hops : hops + 1);
    }
    return depth;
  }

  private static Set<String> unknownOwners(Map<String, String> owner) {
    return owner.values().stream()
        .filter(id -> !id.isEmpty() && !owner.containsKey(id))
        .collect(Collectors.toSet());
  }

  private Optional<Symbol> walkTo(String name) {
    Symbol[] found = new Symbol[1];
    walk(
        symbol -> {
          if (symbol.name().equals(name)) {
            found[0] = symbol;
            return true;
          }
          return false;
        });
    return Optional.ofNullable(found[0]);
  }

  private void walk(Predicate<Symbol> stop) {
    if (roots.isEmpty()) {
      connection.capabilities().require(Capabilities.QUERY);
    }
    ArrayDeque<Symbol> queue = new ArrayDeque<>(roots);
    Set<String> seen = new HashSet<>();
    while (!queue.isEmpty()) {
      Symbol current = queue.removeFirst();
      for (String childId : current.childIds()) {
        Optional<Symbol> child = seen.add(childId) ? findSymbol(childId) : Optional.empty();
        if (child.isPresent()) {
          if (stop.test(child.get())) {
            return;
          }
          queue.addLast(child.get());
        }
      }
    }
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
    Objects.requireNonNull(subjectSymbolId, NAME_SUBJECT_SYMBOL_ID);
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
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
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

  /**
   * Executes an action once, under the service's default schedule and outside any object.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @return the outputs it produced
   * @throws ModelException if the action could not be executed
   * @throws ServiceException if the service does not hold this model
   */
  public ActionRun executeAction(String actionSymbolId) {
    return executeAction(actionSymbolId, Map.of(), ExecutionOptions.defaults());
  }

  /**
   * Executes an action once, with its input parameters bound.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @param inputs values for its input parameters, by name
   * @return the outputs it produced
   * @throws ModelException if the action could not be executed
   * @throws ServiceException if the service does not hold this model
   */
  public ActionRun executeAction(String actionSymbolId, Map<String, Value> inputs) {
    return executeAction(actionSymbolId, inputs, ExecutionOptions.defaults());
  }

  /**
   * Executes an action once, under a schedule and on an object.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @param inputs values for its input parameters, by name
   * @param options the schedule the run resolves its choice points under and the object performing
   *     it; an exploring schedule belongs to {@link #exploreAction}
   * @return the outputs it produced
   * @throws IllegalArgumentException if the schedule explores or a state trace is requested
   * @throws ModelException if the action could not be executed
   * @throws ServiceException if the service does not hold this model, or the schedule names no
   *     policy
   * @throws CapabilityException if a schedule is named and the service does not advertise {@code
   *     schedule}, or a performer and it does not advertise {@code performer}
   */
  public ActionRun executeAction(
      String actionSymbolId, Map<String, Value> inputs, ExecutionOptions options) {
    ExecuteActionResponse response = executeAction(actionSymbolId, inputs, options, false);
    return Protos.actionRun(response, connection.capabilities().has(Capabilities.FINAL_TIME));
  }

  /**
   * Runs an action once per valid order of its choice points, within the {@code explore} budget.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @return every distinct outcome reached, and how the search ended
   * @throws ModelException if the action could not be explored at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code schedule_explore}
   */
  public Exploration exploreAction(String actionSymbolId) {
    return exploreAction(actionSymbolId, Map.of(), ExecutionOptions.defaults());
  }

  /**
   * Runs an action once per valid order of its choice points, within a budget.
   *
   * <p>Runs agreeing on their outputs are one {@link Outcome}; a run failing under some order is an
   * outcome whose error is set, not a failure of the call.
   *
   * @param actionSymbolId qualified name of the action definition or usage
   * @param inputs values for its input parameters, by name
   * @param options the exploring schedule ({@code "explore"}, the default when none is named, or
   *     {@code "explore:runs=<n>,depth=<d>"}) and the object performing each run, made anew for it
   * @return every distinct outcome reached, and how the search ended
   * @throws IllegalArgumentException if the schedule does not explore or a state trace is requested
   * @throws ModelException if the action could not be explored at all
   * @throws ServiceException if the service does not hold this model, or the schedule's budget is
   *     malformed
   * @throws CapabilityException if the service does not advertise {@code schedule_explore}, or a
   *     performer is named and it does not advertise {@code performer}
   */
  public Exploration exploreAction(
      String actionSymbolId, Map<String, Value> inputs, ExecutionOptions options) {
    ExecuteActionResponse response = executeAction(actionSymbolId, inputs, options, true);
    return Protos.exploration(response.getOutcomesList(), response.getExploration());
  }

  private ExecuteActionResponse executeAction(
      String actionSymbolId, Map<String, Value> inputs, ExecutionOptions options, boolean explore) {
    Objects.requireNonNull(actionSymbolId, "actionSymbolId");
    Objects.requireNonNull(inputs, "inputs");
    Objects.requireNonNull(options, NAME_OPTIONS);
    if (options.trace()) {
      throw new IllegalArgumentException("a state trace is only valid for a state run");
    }
    ExecuteActionRequest.Builder request =
        ExecuteActionRequest.newBuilder()
            .setModelHash(hash)
            .setActionSymbolId(actionSymbolId)
            .putAllInputs(Protos.protos(inputs, connection.capabilities()))
            .setSchedule(schedule(options, explore));
    options.performer().ifPresent(request::setPerformerSymbolId);
    ExecuteActionResponse response =
        connection.call("ExecuteAction", request.build(), ExecuteActionResponse.getDefaultInstance());
    failed(response.getError(), FailureReason.UNSPECIFIED, response.getDiagnosticsList());
    return response;
  }

  /**
   * Executes a state machine once, under the service's default schedule and outside any object.
   *
   * @param stateMachineSymbolId qualified name of the state definition or usage
   * @param events the events to send it, in order
   * @return the states it visited and its final context
   * @throws ModelException if the machine could not be executed; a traced failure carries its
   *     partial trace and dropped-record count
   * @throws ServiceException if the service does not hold this model
   */
  public StateRun executeState(String stateMachineSymbolId, List<String> events) {
    return executeState(stateMachineSymbolId, events, ExecutionOptions.defaults());
  }

  /**
   * Executes a state machine once, under a schedule and on an object.
   *
   * @param stateMachineSymbolId qualified name of the state definition or usage
   * @param events the events to send it, in order
   * @param options the schedule and performer, as for {@link #executeAction(String, Map,
   *     ExecutionOptions)}
   * @return the states it visited and its final context
   * @throws IllegalArgumentException if the schedule explores
   * @throws ModelException if the machine could not be executed
   * @throws ServiceException if the service does not hold this model, or the schedule names no
   *     policy
   * @throws CapabilityException if a schedule is named and the service does not advertise {@code
   *     schedule}, or a performer and it does not advertise {@code performer}
   */
  public StateRun executeState(
      String stateMachineSymbolId, List<String> events, ExecutionOptions options) {
    ExecuteStateResponse response = executeState(stateMachineSymbolId, events, options, false);
    return Protos.stateRun(response, connection.capabilities().has(Capabilities.FINAL_TIME));
  }

  /**
   * Runs a state machine once per valid order of its choice points, within the {@code explore}
   * budget.
   *
   * @param stateMachineSymbolId qualified name of the state definition or usage
   * @param events the events to send it, in order
   * @return every distinct outcome reached, and how the search ended
   * @throws ModelException if the machine could not be explored at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code schedule_explore}
   */
  public Exploration exploreState(String stateMachineSymbolId, List<String> events) {
    return exploreState(stateMachineSymbolId, events, ExecutionOptions.defaults());
  }

  /**
   * Runs a state machine once per valid order of its choice points, within a budget.
   *
   * @param stateMachineSymbolId qualified name of the state definition or usage
   * @param events the events to send it, in order
   * @param options the exploring schedule and performer, as for {@link #exploreAction(String, Map,
   *     ExecutionOptions)}
   * @return every distinct outcome reached, and how the search ended
   * @throws IllegalArgumentException if the schedule does not explore
   * @throws ModelException if the machine could not be explored at all
   * @throws ServiceException if the service does not hold this model, or the schedule's budget is
   *     malformed
   * @throws CapabilityException if the service does not advertise {@code schedule_explore}, or a
   *     performer is named and it does not advertise {@code performer}
   */
  public Exploration exploreState(
      String stateMachineSymbolId, List<String> events, ExecutionOptions options) {
    ExecuteStateResponse response = executeState(stateMachineSymbolId, events, options, true);
    return Protos.exploration(response.getOutcomesList(), response.getExploration());
  }

  private ExecuteStateResponse executeState(
      String stateMachineSymbolId, List<String> events, ExecutionOptions options, boolean explore) {
    Objects.requireNonNull(stateMachineSymbolId, "stateMachineSymbolId");
    Objects.requireNonNull(events, "events");
    Objects.requireNonNull(options, NAME_OPTIONS);
    if (options.trace() && explore) {
      throw new IllegalArgumentException("a trace describes one run, not an exploration");
    }
    if (options.trace()) {
      connection.capabilities().require(Capabilities.STATE_TRACE);
    }
    ExecuteStateRequest.Builder request =
        ExecuteStateRequest.newBuilder()
            .setModelHash(hash)
            .setStateMachineSymbolId(stateMachineSymbolId)
            .addAllEvents(events)
            .setSchedule(schedule(options, explore));
    options.performer().ifPresent(request::setPerformerSymbolId);
    request.setTrace(options.trace());
    ExecuteStateResponse response =
        connection.call("ExecuteState", request.build(), ExecuteStateResponse.getDefaultInstance());
    if (!response.getError().isEmpty()) {
      throw new ModelException(
          response.getError(),
          FailureReason.UNSPECIFIED,
          Protos.diagnostics(response.getDiagnosticsList()),
          Protos.documentEvents(response.getTraceList()),
          response.getTraceDropped());
    }
    return response;
  }

  /**
   * Verifies a constraint against the model's declared values.
   *
   * @param symbolId qualified name of the constraint definition or usage
   * @return its verdict, which does not hold when the model answers false and is undecided when
   *     the constraint could not be evaluated
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Verification verifyConstraint(String symbolId) {
    return verifyConstraint(symbolId, Optional.empty(), VerifyOptions.defaults());
  }

  /**
   * Verifies a constraint against an object's values.
   *
   * @param symbolId qualified name of the constraint definition or usage
   * @param subjectSymbolId qualified name of the part definition or usage to instantiate and verify
   *     the constraint on
   * @return its verdict, about that object
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Verification verifyConstraint(String symbolId, String subjectSymbolId) {
    Objects.requireNonNull(subjectSymbolId, NAME_SUBJECT_SYMBOL_ID);
    return verifyConstraint(symbolId, Optional.of(subjectSymbolId), VerifyOptions.defaults());
  }

  /**
   * Answers a question about a constraint against the model's declared values.
   *
   * @param symbolId qualified name of the constraint definition or usage
   * @param options the question to ask: an evaluation (the default), {@code holds} or
   *     {@code satisfiable}, as {@link VerifyOptions} spells them
   * @return its verdict, its status the answer the question got
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}, or the
   *     question is not an evaluation and the service does not advertise
   *     {@code verification_questions}
   */
  public Verification verifyConstraint(String symbolId, VerifyOptions options) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifyConstraint(symbolId, Optional.empty(), options);
  }

  /**
   * Answers a question about a constraint against an object's values.
   *
   * @param symbolId qualified name of the constraint definition or usage
   * @param subjectSymbolId qualified name of the part definition or usage to instantiate and verify
   *     the constraint on
   * @param options the question to ask, as {@link VerifyOptions} spells it
   * @return its verdict, about that object
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification} or, for a
   *     question that is not an evaluation, {@code verification_questions}
   */
  public Verification verifyConstraint(
      String symbolId, String subjectSymbolId, VerifyOptions options) {
    Objects.requireNonNull(subjectSymbolId, NAME_SUBJECT_SYMBOL_ID);
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifyConstraint(symbolId, Optional.of(subjectSymbolId), options);
  }

  private Verification verifyConstraint(
      String symbolId, Optional<String> subjectSymbolId, VerifyOptions options) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    connection.capabilities().require(Capabilities.VERIFICATION);
    requireQuestion(options);
    VerifyConstraintRequest.Builder request =
        VerifyConstraintRequest.newBuilder().setModelHash(hash).setSymbolId(symbolId);
    subjectSymbolId.ifPresent(request::setSubjectSymbolId);
    engine.ifPresent(request::setEngine);
    questionField(options).ifPresent(request::setQuestion);
    VerifyConstraintResponse response =
        connection.call(
            "VerifyConstraint", request.build(), VerifyConstraintResponse.getDefaultInstance());
    failed(response.getError(), FailureReason.UNSPECIFIED, response.getDiagnosticsList());
    return Protos.verification(response);
  }

  /**
   * Verifies a requirement's constraints against the model's declared values.
   *
   * @param symbolId qualified name of the requirement definition or usage
   * @return its verdict, with what the verification cases verifying it answered
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Verification verifyRequirement(String symbolId) {
    return verifyRequirement(symbolId, Optional.empty(), VerifyOptions.defaults());
  }

  /**
   * Verifies a requirement's constraints against an object's values.
   *
   * @param symbolId qualified name of the requirement definition or usage
   * @param subjectSymbolId qualified name of the part definition or usage to instantiate and verify
   *     the requirement on
   * @return its verdict, about that object
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Verification verifyRequirement(String symbolId, String subjectSymbolId) {
    Objects.requireNonNull(subjectSymbolId, NAME_SUBJECT_SYMBOL_ID);
    return verifyRequirement(symbolId, Optional.of(subjectSymbolId), VerifyOptions.defaults());
  }

  /**
   * Answers a question about a requirement's constraints against the model's declared values.
   *
   * @param symbolId qualified name of the requirement definition or usage
   * @param options the question to ask, as {@link VerifyOptions} spells it
   * @return its verdict, with what the verification cases verifying it answered
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification} or, for a
   *     question that is not an evaluation, {@code verification_questions}
   */
  public Verification verifyRequirement(String symbolId, VerifyOptions options) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifyRequirement(symbolId, Optional.empty(), options);
  }

  /**
   * Answers a question about a requirement's constraints against an object's values.
   *
   * @param symbolId qualified name of the requirement definition or usage
   * @param subjectSymbolId qualified name of the part definition or usage to instantiate and verify
   *     the requirement on
   * @param options the question to ask, as {@link VerifyOptions} spells it
   * @return its verdict, about that object
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification} or, for a
   *     question that is not an evaluation, {@code verification_questions}
   */
  public Verification verifyRequirement(
      String symbolId, String subjectSymbolId, VerifyOptions options) {
    Objects.requireNonNull(subjectSymbolId, NAME_SUBJECT_SYMBOL_ID);
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifyRequirement(symbolId, Optional.of(subjectSymbolId), options);
  }

  private Verification verifyRequirement(
      String symbolId, Optional<String> subjectSymbolId, VerifyOptions options) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    connection.capabilities().require(Capabilities.VERIFICATION);
    requireQuestion(options);
    VerifyRequirementRequest.Builder request =
        VerifyRequirementRequest.newBuilder().setModelHash(hash).setSymbolId(symbolId);
    subjectSymbolId.ifPresent(request::setSubjectSymbolId);
    engine.ifPresent(request::setEngine);
    questionField(options).ifPresent(request::setQuestion);
    VerifyRequirementResponse response =
        connection.call(
            "VerifyRequirement", request.build(), VerifyRequirementResponse.getDefaultInstance());
    failed(response.getError(), FailureReason.UNSPECIFIED, response.getDiagnosticsList());
    return Protos.verification(response);
  }

  /**
   * Evaluates every {@code satisfy} assertion in the model.
   *
   * @return one verdict per assertion, in declaration order
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Satisfaction verifySatisfaction() {
    return verifySatisfaction(Optional.empty(), VerifyOptions.defaults());
  }

  /**
   * Answers a question about every {@code satisfy} assertion in the model.
   *
   * @param options the question to ask, as {@link VerifyOptions} spells it
   * @return one verdict per assertion, in declaration order
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification} or, for a
   *     question that is not an evaluation, {@code verification_questions}
   */
  public Satisfaction verifySatisfaction(VerifyOptions options) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifySatisfaction(Optional.empty(), options);
  }

  /**
   * Evaluates the {@code satisfy} assertions within one element.
   *
   * @param scopeSymbolId qualified name of the package, definition or usage whose assertions are
   *     evaluated
   * @return one verdict per assertion, in declaration order
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Satisfaction verifySatisfaction(String scopeSymbolId) {
    Objects.requireNonNull(scopeSymbolId, "scopeSymbolId");
    return verifySatisfaction(Optional.of(scopeSymbolId), VerifyOptions.defaults());
  }

  /**
   * Answers a question about the {@code satisfy} assertions within one element.
   *
   * @param scopeSymbolId qualified name of the package, definition or usage whose assertions are
   *     asked about
   * @param options the question to ask, as {@link VerifyOptions} spells it
   * @return one verdict per assertion, in declaration order
   * @throws ModelException if the service could not answer at all
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification} or, for a
   *     question that is not an evaluation, {@code verification_questions}
   */
  public Satisfaction verifySatisfaction(String scopeSymbolId, VerifyOptions options) {
    Objects.requireNonNull(scopeSymbolId, "scopeSymbolId");
    Objects.requireNonNull(options, NAME_OPTIONS);
    return verifySatisfaction(Optional.of(scopeSymbolId), options);
  }

  private Satisfaction verifySatisfaction(Optional<String> scopeSymbolId, VerifyOptions options) {
    connection.capabilities().require(Capabilities.VERIFICATION);
    requireQuestion(options);
    VerifySatisfactionRequest.Builder request =
        VerifySatisfactionRequest.newBuilder().setModelHash(hash);
    scopeSymbolId.ifPresent(request::setSymbolId);
    engine.ifPresent(request::setEngine);
    questionField(options).ifPresent(request::setQuestion);
    VerifySatisfactionResponse response =
        connection.call(
            "VerifySatisfaction", request.build(), VerifySatisfactionResponse.getDefaultInstance());
    failed(response.getError(), response.getFailureReason(), response.getDiagnosticsList());
    return Protos.satisfaction(response);
  }

  /**
   * Checks every assertion about an object of a part definition or usage, and about the objects it
   * holds, against their values.
   *
   * @param symbolId qualified name of the part definition or usage to instantiate and validate
   * @return one verdict per assertion, and one for the object as a whole
   * @throws ModelException if the service could not answer at all: an unknown symbol, or one that
   *     declares no object
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Validation validateInstance(String symbolId) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    connection.capabilities().require(Capabilities.VERIFICATION);
    ValidateInstanceRequest.Builder request =
        ValidateInstanceRequest.newBuilder().setModelHash(hash).setSymbolId(symbolId);
    engine.ifPresent(request::setEngine);
    ValidateInstanceResponse response =
        connection.call(
            "ValidateInstance", request.build(), ValidateInstanceResponse.getDefaultInstance());
    failed(response.getError(), response.getFailureReason(), response.getDiagnosticsList());
    return Protos.validation(response);
  }

  /**
   * Evaluates a calc.
   *
   * @param symbolId qualified name of the calc definition or usage
   * @param arguments values for its {@code in} parameters, in declaration order; empty evaluates a
   *     usage from its own members
   * @return what it computed
   * @throws ModelException if the calc could not be evaluated, its {@link
   *     ModelException#failureReason()} saying why
   * @throws ServiceException if the service does not hold this model
   */
  public Calculation evaluateCalc(String symbolId, List<Value> arguments) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    Objects.requireNonNull(arguments, "arguments");
    EvaluateCalcRequest.Builder request =
        EvaluateCalcRequest.newBuilder()
            .setModelHash(hash)
            .setSymbolId(symbolId)
            .addAllArguments(Protos.protos(arguments, connection.capabilities()));
    engine.ifPresent(request::setEngine);
    EvaluateCalcResponse response =
        connection.call("EvaluateCalc", request.build(), EvaluateCalcResponse.getDefaultInstance());
    failed(response.getError(), response.getFailureReason(), response.getDiagnosticsList());
    return Protos.calculation(response);
  }

  /**
   * Runs an analysis case usage that binds its own subject.
   *
   * @param symbolId qualified name of the analysis case usage
   * @return what it computed and the verdicts of its objective and assertions
   * @throws AnalysisException if the case could not run to its end but left something to inspect
   * @throws ModelException if the request was refused before the run, or the failure left nothing to
   *     report; its {@link ModelException#failureReason()} says why
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Analysis runAnalysis(String symbolId) {
    return runAnalysis(symbolId, AnalysisOptions.defaults());
  }

  /**
   * Runs an analysis case once, on a subject and with its parameters bound.
   *
   * <p>The subject named is instantiated and bound as the case's subject; positional arguments
   * bind its {@code in} parameters in declaration order, the subject excluded, and named arguments
   * bind them by name. Its objective and each {@code assert constraint} in its body are then checked
   * against what it computed.
   *
   * @param symbolId qualified name of the analysis case definition or usage
   * @param options the subject, arguments and schedule; an exploring schedule belongs to {@link
   *     #exploreAnalysis}
   * @return what it computed and the verdicts of its objective and assertions
   * @throws IllegalArgumentException if the schedule or the engine explores
   * @throws AnalysisException if the case could not run to its end but left something to inspect
   * @throws ModelException if the request was refused before the run, or the failure left nothing to
   *     report; its {@link ModelException#failureReason()} says why
   * @throws ServiceException if the service does not hold this model, or the schedule names no
   *     policy
   * @throws CapabilityException if the service does not advertise {@code verification}, or a
   *     schedule is named and it does not advertise {@code schedule}
   */
  public Analysis runAnalysis(String symbolId, AnalysisOptions options) {
    RunAnalysisResponse response = runAnalysis(symbolId, options, false);
    Analysis analysis = Protos.analysis(response);
    if (response.getError().isEmpty()) {
      return analysis;
    }
    FailureReason reason = Protos.failureReason(response.getFailureReason());
    if (analysis.outputs().isEmpty()
        && analysis.verdicts().isEmpty()
        && analysis.evaluations().isEmpty()
        && analysis.instances().isEmpty()) {
      throw new ModelException(response.getError(), reason, analysis.diagnostics());
    }
    throw new AnalysisException(response.getError(), reason, analysis.diagnostics(), analysis);
  }

  /**
   * Runs an analysis case once per valid order of the choice points its actions meet.
   *
   * <p>Runs agreeing on the case's outputs and verdicts are one {@link Outcome}; a verdict is
   * reported among the outcome's outputs as {@code "objective <name>"} or {@code "assertion
   * <name>"}.
   *
   * @param symbolId qualified name of the analysis case definition or usage
   * @param options the subject, arguments and exploring schedule ({@code "explore"}, the default
   *     when none is named, or {@code "explore:runs=<n>,depth=<d>"})
   * @return every distinct outcome reached, and how the search ended
   * @throws IllegalArgumentException if the schedule does not explore
   * @throws ModelException if the case could not be explored at all
   * @throws ServiceException if the service does not hold this model, or the schedule's budget is
   *     malformed
   * @throws CapabilityException if the service does not advertise {@code verification} or {@code
   *     schedule_explore}
   */
  public Exploration exploreAnalysis(String symbolId, AnalysisOptions options) {
    RunAnalysisResponse response = runAnalysis(symbolId, options, true);
    failed(response.getError(), response.getFailureReason(), response.getDiagnosticsList());
    return Protos.exploration(response.getOutcomesList(), response.getExploration());
  }

  private RunAnalysisResponse runAnalysis(
      String symbolId, AnalysisOptions options, boolean explore) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    Objects.requireNonNull(options, NAME_OPTIONS);
    connection.capabilities().require(Capabilities.VERIFICATION);
    if (!explore && engine.isPresent() && engine.orElseThrow().equals(EXPLORE)) {
      throw new IllegalArgumentException(
          "engine explore answers every outcome; use exploreAnalysis");
    }
    RunAnalysisRequest.Builder request =
        RunAnalysisRequest.newBuilder()
            .setModelHash(hash)
            .setSymbolId(symbolId)
            .addAllArguments(Protos.protos(options.arguments(), connection.capabilities()))
            .putAllNamedArguments(Protos.protos(options.namedArguments(), connection.capabilities()))
            .setSchedule(schedule(options.schedule(), options.explores(), explore));
    options.subject().ifPresent(request::setSubjectSymbolId);
    engine.ifPresent(request::setEngine);
    return connection.call("RunAnalysis", request.build(), RunAnalysisResponse.getDefaultInstance());
  }

  /**
   * Selects elements of the model.
   *
   * @param query the elements considered, the properties reported and the filter applied
   * @return the elements selected, each with the properties asked for
   * @throws ServiceException if the service does not hold this model, or the query names an unknown
   *     scope or property, or leaves a comparison's operator unset
   * @throws CapabilityException if the service does not advertise {@code query}
   */
  public List<QueryElement> query(Query query) {
    Objects.requireNonNull(query, "query");
    connection.capabilities().require(Capabilities.QUERY);
    return selected(QueryRequest.newBuilder().setModelHash(hash).setQuery(Protos.proto(query)));
  }

  /**
   * Selects elements of the model with an OSLC query string, such as {@code
   * "oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name"}.
   *
   * @param oslcQuery the query, as OSLC Query Syntax spells it
   * @return the elements selected, each with the properties asked for
   * @throws ServiceException if the service does not hold this model, or the query does not parse
   * @throws CapabilityException if the service does not advertise {@code oslc_query}
   */
  public List<QueryElement> queryOslc(String oslcQuery) {
    Objects.requireNonNull(oslcQuery, "oslcQuery");
    connection.capabilities().require(Capabilities.OSLC_QUERY);
    return selected(QueryRequest.newBuilder().setModelHash(hash).setOslcQuery(oslcQuery));
  }

  private List<QueryElement> selected(QueryRequest.Builder request) {
    QueryResponse response =
        connection.call("Query", request.build(), QueryResponse.getDefaultInstance());
    return Protos.queryElements(response.getElementsList());
  }

  /**
   * Rewrites the model in another format.
   *
   * @param toFormat the format to write, named as the service names formats ({@code "sysml"},
   *     {@code "kerml"}, {@code "ttl"}, {@code "api-json"}, …)
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convert(String toFormat) {
    return convert(toFormat, ConversionOptions.defaults());
  }

  /**
   * Rewrites the model in another format, with options.
   *
   * @param toFormat the format to write
   * @param options the source format and whether unreadable notation is written back anyway
   * @return the conversion, carrying the text and the formats used
   * @throws ConversionException if the conversion failed; its diagnostics say why
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code convert}
   */
  public Conversion convert(String toFormat, ConversionOptions options) {
    Objects.requireNonNull(toFormat, "toFormat");
    Objects.requireNonNull(options, NAME_OPTIONS);
    connection.capabilities().require(Capabilities.CONVERT);
    ConvertRequest.Builder request =
        ConvertRequest.newBuilder()
            .setModelHash(hash)
            .setToFormat(toFormat)
            .setTolerateSyntaxErrors(options.tolerateSyntaxErrors());
    options.fromFormat().ifPresent(request::setFromFormat);
    options.idForm().ifPresent(request::setIdForm);
    ConvertResponse response =
        connection.call("Convert", request.build(), ConvertResponse.getDefaultInstance());
    if (!response.getError().isEmpty()) {
      throw new ConversionException(
          response.getError(), Protos.diagnostics(response.getDiagnosticsList()));
    }
    return Protos.conversion(response);
  }

  /**
   * Writes the model to a file, in the format its extension names ({@link Conversion#formatOf}).
   *
   * @param path the file, created or truncated
   * @return the conversion that was written
   * @throws IllegalArgumentException if the extension names no format
   * @throws ConversionException if the model could not be written in that format
   * @throws java.io.UncheckedIOException if the file cannot be written
   */
  public Conversion save(Path path) {
    return save(path, Conversion.formatOf(path), ConversionOptions.defaults());
  }

  /**
   * Writes the model to a file in a named format, with options.
   *
   * @param path the file, created or truncated
   * @param toFormat the format to write
   * @param options the conversion options
   * @return the conversion that was written
   * @throws ConversionException if the model could not be written in that format
   * @throws java.io.UncheckedIOException if the file cannot be written
   */
  public Conversion save(Path path, String toFormat, ConversionOptions options) {
    Objects.requireNonNull(path, "path");
    Conversion conversion = convert(toFormat, options);
    conversion.write(path);
    return conversion;
  }

  /**
   * Starts an edit of this model, applied in one call by {@link Editor#apply()}.
   *
   * @return an empty editor
   */
  public Editor edit() {
    return new Editor(this);
  }

  /**
   * Applies edits to the model's source text, answering the text they produce.
   *
   * <p>An empty batch is refused in band, as {@link EditFailure#NO_OPERATIONS}.
   *
   * @param edits the edits, applied in order as one batch
   * @return the rewritten text, which edits applied where, and the new documents' content when the
   *     batch accepted them
   * @throws EditException if the service refused the batch; its {@link EditException#failure()}
   *     says why and its {@link EditException#referrers()} name what references the target
   * @throws ServiceException if the request itself was rejected
   * @throws CapabilityException if the service does not advertise {@code apply_edits}, or an edit
   *     is an {@link Edit.AddMember}, {@link Edit.AddConnection}, {@link Edit.Delete} or
   *     {@link Edit.Move} and it does not advertise {@code authoring}; {@link Edit.AddConnection}
   *     also requires {@code connection_authoring}; a modifier or a {@code ref}/{@code return}
   *     member requires {@code member_modifiers}, {@link Edit.AddSatisfy} requires
   *     {@code satisfy_authoring}, and {@link Edit.AddRequirementConstraint} requires
   *     {@code requirement_constraint_authoring}; an {@link Edit.AddSequence} carrying an
   *     action-body statement or source multiplicity requires
   *     {@code action_body_statement_authoring}
   */
  public EditResult applyEdits(List<Edit> edits) {
    return applyEdits(edits, EditOptions.defaults());
  }

  /**
   * Applies edits to the model's source text, with options.
   *
   * @param edits the edits, applied in order as one batch
   * @param options the document the edits target and whether the answer's documents are read;
   *     {@link EditOptions#defaults()} accepts them
   * @return the rewritten text, which edits applied where, and the new documents' content when the
   *     batch accepted them
   * @throws EditException if the service refused the batch
   * @throws ServiceException if the request itself was rejected, or a document is named that no
   *     document of the model has
   * @throws CapabilityException if the service does not advertise {@code apply_edits}, an edit
   *     writes a declaration and it does not advertise {@code authoring}, or a document is named
   *     and it does not advertise {@code edit_documents}; {@link Edit.AddConnection} also requires
   *     {@code connection_authoring}; an {@link Edit.AddSequence} carrying an action-body
   *     statement or source multiplicity requires {@code action_body_statement_authoring}
   */
  public EditResult applyEdits(List<Edit> edits, EditOptions options) {
    Objects.requireNonNull(edits, "edits");
    Objects.requireNonNull(options, NAME_OPTIONS);
    connection.capabilities().require(Capabilities.APPLY_EDITS);
    Set<String> required = new HashSet<>();
    for (Edit edit : edits) {
      editCapabilities(edit, required);
    }
    for (String capability : EDIT_CAPABILITY_ORDER) {
      if (required.contains(capability)) {
        connection.capabilities().require(capability);
      }
    }
    options.document().ifPresent(document -> connection.capabilities().require(Capabilities.EDIT_DOCUMENTS));
    ApplyEditsRequest.Builder request =
        ApplyEditsRequest.newBuilder()
            .setModelHash(hash)
            .addAllOperations(Protos.edits(edits))
            .setAcceptDocuments(options.acceptDocuments());
    options.document().ifPresent(request::setDocument);
    boolean adopted = roots.isEmpty() && documents.isEmpty();
    Optional<Boolean> several =
        adopted ? Optional.empty() : Optional.of(Math.max(roots.size(), documents.size()) > 1);
    ApplyEditsResponse response;
    if (adopted
        && options.acceptDocuments()
        && connection.capabilities().has(Capabilities.EDIT_DOCUMENTS)) {
      try {
        response = applyEdits(request.setAcceptDocuments(false));
        several = Optional.of(false);
      } catch (ServiceException refused) {
        if (refused.status() != StatusCode.FAILED_PRECONDITION) {
          throw refused;
        }
        response = applyEdits(request.setAcceptDocuments(true));
        several = Optional.of(true);
      }
    } else {
      response = applyEdits(request);
    }
    EditResult result = Protos.editResult(response);
    if (several.isEmpty() || several.get().booleanValue() == result.severalDocuments()) {
      return result;
    }
    return new EditResult(
        result.content(), result.applied(), result.documents(), result.diagnostics(), several.get());
  }

  private ApplyEditsResponse applyEdits(ApplyEditsRequest.Builder request) {
    ApplyEditsResponse response =
        connection.call("ApplyEdits", request.build(), ApplyEditsResponse.getDefaultInstance());
    if (!response.getError().isEmpty()) {
      throw new EditException(
          response.getError(),
          Protos.editFailure(response.getFailure()),
          Protos.editFailureName(response.getFailure(), response.getFailureValue()),
          Protos.diagnostics(response.getDiagnosticsList()),
          response.getReferringElementsList(),
          Protos.referrers(response.getReferrersList()));
    }
    return response;
  }

  /** The order the capabilities an edit request needs are checked in. */
  private static final List<String> EDIT_CAPABILITY_ORDER =
      List.of(
          Capabilities.AUTHORING,
          Capabilities.CONNECTION_AUTHORING,
          Capabilities.MEMBER_MODIFIERS,
          Capabilities.SATISFY_AUTHORING,
          Capabilities.REQUIREMENT_CONSTRAINT_AUTHORING,
          Capabilities.TRANSITION_AUTHORING,
          Capabilities.VERIFICATION_OBJECTIVE_AUTHORING,
          Capabilities.METADATA_AUTHORING,
          Capabilities.METADATA_PREFIX_AUTHORING,
          Capabilities.SEQUENCE_AUTHORING,
          Capabilities.ACTION_BODY_STATEMENT_AUTHORING,
          Capabilities.IMPLICIT_PARAMETERS,
          Capabilities.CONSTRAINT_BODY_AUTHORING,
          Capabilities.STATE_ACTION_AUTHORING,
          Capabilities.IMPORT_AUTHORING,
          Capabilities.DOCUMENTATION_AUTHORING,
          Capabilities.COMMENT_AUTHORING);

  private static final List<String> STATE_ACTION_KINDS =
      List.of("exhibit state", "exhibit", "entry action", "do action", "exit action");

  private static final List<String> ASSERTED_CONSTRAINT_KINDS =
      List.of("assert", "assert not", "assert constraint", "assert not constraint");

  /** Adds to {@code required} the capabilities applying {@code edit} needs the service to advertise. */
  private static void editCapabilities(Edit edit, Set<String> required) {
    if (edit instanceof Edit.AddMember addMember) {
      addMemberCapabilities(addMember, required);
    } else if (edit instanceof Edit.AddConnection) {
      required.add(Capabilities.CONNECTION_AUTHORING);
    } else if (edit instanceof Edit.AddSatisfy) {
      required.add(Capabilities.SATISFY_AUTHORING);
    } else if (edit instanceof Edit.AddRequirementConstraint) {
      required.add(Capabilities.REQUIREMENT_CONSTRAINT_AUTHORING);
    } else if (edit instanceof Edit.AddTransition) {
      required.add(Capabilities.TRANSITION_AUTHORING);
    } else if (edit instanceof Edit.AddVerify) {
      required.add(Capabilities.VERIFICATION_OBJECTIVE_AUTHORING);
    } else if (edit instanceof Edit.AddMetadata) {
      required.add(Capabilities.METADATA_AUTHORING);
    } else if (edit instanceof Edit.AddMetadataPrefix) {
      required.add(Capabilities.METADATA_PREFIX_AUTHORING);
    } else if (edit instanceof Edit.AddSequence addSequence) {
      required.add(Capabilities.SEQUENCE_AUTHORING);
      if (addSequence.requiresActionBodyStatementAuthoring()) {
        required.add(Capabilities.ACTION_BODY_STATEMENT_AUTHORING);
      }
    } else if (edit instanceof Edit.AddImport) {
      required.add(Capabilities.IMPORT_AUTHORING);
    } else if (edit instanceof Edit.AddDocumentation) {
      required.add(Capabilities.DOCUMENTATION_AUTHORING);
    } else if (edit instanceof Edit.AddComment || edit instanceof Edit.AddNote) {
      required.add(Capabilities.COMMENT_AUTHORING);
    } else if (!(edit instanceof Edit.Delete) && !(edit instanceof Edit.Move)) {
      return;
    }
    required.add(Capabilities.AUTHORING);
  }

  private static void addMemberCapabilities(Edit.AddMember addMember, Set<String> required) {
    String kind = addMember.kind();
    if (addMember.isAbstract()
        || !addMember.redefines().isEmpty()
        || addMember.isDefault()
        || !addMember.direction().isEmpty()
        || kind.equals("ref")
        || kind.equals("return")) {
      required.add(Capabilities.MEMBER_MODIFIERS);
    }
    if (kind.isEmpty()) {
      required.add(Capabilities.IMPLICIT_PARAMETERS);
    }
    if (addMember.bodyExpression().filter(e -> !e.isEmpty()).isPresent()
        || ASSERTED_CONSTRAINT_KINDS.contains(kind)) {
      required.add(Capabilities.CONSTRAINT_BODY_AUTHORING);
    }
    if (STATE_ACTION_KINDS.contains(kind)) {
      required.add(Capabilities.STATE_ACTION_AUTHORING);
    }
    if (kind.equals("objective") && addMember.name().isEmpty()) {
      required.add(Capabilities.VERIFICATION_OBJECTIVE_AUTHORING);
    }
    if (!addMember.metadataPrefixes().isEmpty()) {
      required.add(Capabilities.METADATA_AUTHORING);
    }
    if (!addMember.doc().isEmpty()) {
      required.add(Capabilities.DOCUMENTATION_AUTHORING);
    }
  }

  /**
   * Runs a case or a calc once per combination of its swept parameters.
   *
   * @param symbolId qualified name of the analysis case or calc, definition or usage
   * @param ranges the swept parameters, several of which make one row per point of their cartesian
   *     product, the first varying slowest; empty sweeps the target's declared values
   * @return the table of rows, in the deterministic order they ran
   * @throws ModelException if the sweep itself failed — the target unreadable or not one that
   *     sweeps; one combination's failure is that {@link SweepRow}'s error, not the sweep's
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Sweep runSweep(String symbolId, List<SweepRange> ranges) {
    return runSweep(symbolId, ranges, SweepOptions.defaults());
  }

  /**
   * Runs a case or a calc once per combination of its swept parameters, on a subject and with
   * parameters bound.
   *
   * @param symbolId qualified name of the analysis case or calc, definition or usage
   * @param ranges the swept parameters
   * @param options the subject, the arguments every row binds, and the sampling ({@code samples}
   *     draws rows uniformly and needs {@code seed}; zero steps through each range)
   * @return the table of rows, in the deterministic order they ran
   * @throws ModelException if the sweep itself failed; one combination's failure is that {@link
   *     SweepRow}'s error, not the sweep's
   * @throws ServiceException if the service does not hold this model
   * @throws CapabilityException if the service does not advertise {@code verification}
   */
  public Sweep runSweep(String symbolId, List<SweepRange> ranges, SweepOptions options) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
    Objects.requireNonNull(ranges, "ranges");
    Objects.requireNonNull(options, NAME_OPTIONS);
    connection.capabilities().require(Capabilities.VERIFICATION);
    RunSweepRequest.Builder request =
        RunSweepRequest.newBuilder()
            .setModelHash(hash)
            .setSymbolId(symbolId)
            .addAllArguments(Protos.protos(options.arguments(), connection.capabilities()))
            .putAllNamedArguments(Protos.protos(options.namedArguments(), connection.capabilities()))
            .setSamples(options.samples())
            .setSeed(options.seed());
    options.subject().ifPresent(request::setSubjectSymbolId);
    engine.ifPresent(request::setEngine);
    for (SweepRange range : ranges) {
      request.addRanges(Protos.proto(range));
    }
    RunSweepResponse response =
        connection.call("RunSweep", request.build(), RunSweepResponse.getDefaultInstance());
    failed(response.getError(), response.getFailureReason(), response.getDiagnosticsList());
    return Protos.sweep(response);
  }

  /**
   * Runs a document query declared in the model, answering its columns and typed rows.
   *
   * @param queryId qualified name of the document query
   * @return the projected columns and rows
   * @throws ServiceException if the model declares no such query or it declares something else
   * @throws CapabilityException if the service does not advertise {@code document_query}
   */
  public DocumentQueryResult runDocumentQuery(String queryId) {
    return runDocumentQuery(queryId, Map.of());
  }

  /**
   * Runs a document query with its entry parameters bound.
   *
   * <p>Each binding names a {@link DocumentValue}: an {@link DocumentValue.ElementRef} binds an
   * element by qualified name, an {@link DocumentValue.ObjectRef} an object the service holds by
   * path or id, and a literal its value. A parameter bound by several values is a nonscalar
   * binding.
   *
   * @param queryId qualified name of the document query
   * @param bindings the entry parameters' values, by parameter name
   * @return the projected columns and rows
   * @throws ServiceException if the model declares no such query, it declares something else, or a
   *     binding is refused
   * @throws CapabilityException if the service does not advertise {@code document_query}
   */
  public DocumentQueryResult runDocumentQuery(
      String queryId, Map<String, List<DocumentValue>> bindings) {
    Objects.requireNonNull(queryId, "queryId");
    Objects.requireNonNull(bindings, "bindings");
    connection.capabilities().require(Capabilities.DOCUMENT_QUERY);
    RunDocumentQueryRequest.Builder request =
        RunDocumentQueryRequest.newBuilder().setModelHash(hash).setQueryId(queryId);
    bindings.forEach(
        (parameter, values) ->
            request.addBindings(
                org.openmbee.opensysml.proto.DocumentQueryBinding.newBuilder()
                    .setParameter(parameter)
                    .addAllValues(
                        values.stream().map(Protos::proto).toList())));
    if (request.getBindingsList().stream().anyMatch(Protos::holdsBigInt)) {
      connection.capabilities().require(Capabilities.BIG_INT_VALUES);
    }
    if (!connection.capabilities().has(Capabilities.RATIONAL_VALUES)) {
      for (int i = 0; i < request.getBindingsCount(); i++) {
        request.setBindings(i, Protos.rationalsAsReals(request.getBindings(i)));
      }
    }
    if (request.getBindingsList().stream().anyMatch(Protos::holdsRational)) {
      connection.capabilities().require(Capabilities.RATIONAL_VALUES);
    }
    RunDocumentQueryResponse response =
        connection.call(
            "RunDocumentQuery", request.build(), RunDocumentQueryResponse.getDefaultInstance());
    return Protos.documentQueryResult(response);
  }

  /**
   * Renders a document declared in the model to Markdown.
   *
   * @param documentId qualified name of the document
   * @return the rendered document, byte-for-byte what the command line writes
   * @throws ServiceException if the model declares no such document or it declares something else
   * @throws CapabilityException if the service does not advertise {@code render_document}
   */
  public RenderedDocument renderDocument(String documentId) {
    return renderDocument(documentId, DocumentForm.MARKDOWN);
  }

  /**
   * Renders a document declared in the model to Markdown or HTML.
   *
   * @param documentId qualified name of the document
   * @param form the form to render it in
   * @return the rendered document, byte-for-byte what the command line writes in that form
   * @throws ServiceException if the model declares no such document or it declares something else
   * @throws CapabilityException if the service does not advertise {@code render_document}, or
   *     {@code render_document_html} when HTML is asked for
   */
  public RenderedDocument renderDocument(String documentId, DocumentForm form) {
    Objects.requireNonNull(documentId, "documentId");
    Objects.requireNonNull(form, "form");
    connection.capabilities().require(Capabilities.RENDER_DOCUMENT);
    if (form == DocumentForm.HTML) {
      connection.capabilities().require(Capabilities.RENDER_DOCUMENT_HTML);
    }
    RenderDocumentResponse response =
        connection.call(
            "RenderDocument",
            RenderDocumentRequest.newBuilder()
                .setModelHash(hash)
                .setDocumentId(documentId)
                .setForm(form == DocumentForm.MARKDOWN ? "" : form.wireName())
                .build(),
            RenderDocumentResponse.getDefaultInstance());
    return new RenderedDocument(
        form, form == DocumentForm.HTML ? response.getHtml() : response.getMarkdown());
  }

  /**
   * Renders a named or targeted pseudo-view as machine-readable diagram data.
   *
   * @param viewName qualified view name or {@code #<kind>:<qualified name>} pseudo-view
   * @return every field the view renderer produced
   * @throws CapabilityException if the service does not advertise {@code render_view}
   * @throws ServiceException if the view cannot be selected or rendered
   */
  public RenderedView renderView(String viewName) {
    return renderView(viewName, RenderViewPorts.MINIMAL);
  }

  /** Renders a view with an explicit port selection. */
  public RenderedView renderView(String viewName, RenderViewPorts ports) {
    Objects.requireNonNull(viewName, "viewName");
    Objects.requireNonNull(ports, "ports");
    connection.capabilities().require(Capabilities.RENDER_VIEW);
    RenderViewResponse response =
        connection.call(
            "RenderView",
            RenderViewRequest.newBuilder()
                .setModelHash(hash)
                .setView(viewName)
                .setPorts(ports.wireName())
                .build(),
            RenderViewResponse.getDefaultInstance());
    return RenderedView.from(response);
  }

  private String schedule(ExecutionOptions options, boolean explore) {
    Objects.requireNonNull(options, NAME_OPTIONS);
    if (options.performer().isPresent()) {
      connection.capabilities().require(Capabilities.PERFORMER);
    }
    return schedule(options.schedule(), options.explores(), explore);
  }

  private String schedule(Optional<String> schedule, boolean explores, boolean explore) {
    if (explore) {
      if (schedule.isPresent() && !explores) {
        throw new IllegalArgumentException(
            "schedule " + schedule.orElseThrow() + " runs once; an exploration takes explore");
      }
      connection.capabilities().require(Capabilities.SCHEDULE_EXPLORE);
      return schedule.orElse(EXPLORE);
    }
    if (explores) {
      throw new IllegalArgumentException(
          "schedule " + schedule.orElseThrow() + " answers every outcome; use the explore call");
    }
    if (schedule.isPresent()) {
      connection.capabilities().require(Capabilities.SCHEDULE);
    }
    return schedule.orElse("");
  }

  private static void failed(
      String error,
      org.openmbee.opensysml.proto.FailureReason reason,
      List<org.openmbee.opensysml.proto.Diagnostic> diagnostics) {
    failed(error, Protos.failureReason(reason), diagnostics);
  }

  private static void failed(
      String error, FailureReason reason, List<org.openmbee.opensysml.proto.Diagnostic> diagnostics) {
    if (!error.isEmpty()) {
      throw new ModelException(error, reason, Protos.diagnostics(diagnostics));
    }
  }

  // The question as sent: absent for an evaluation, which every service reads as such.
  private static Optional<String> questionField(VerifyOptions options) {
    if (options.question().isEmpty()
        || options.question().equals(VerifyOptions.QUESTION_EVALUATE)) {
      return Optional.empty();
    }
    return Optional.of(options.question());
  }

  // Refuse to send a question a service without verification_questions would evaluate instead.
  private void requireQuestion(VerifyOptions options) {
    questionField(options)
        .ifPresent(question -> connection.capabilities().require(Capabilities.VERIFICATION_QUESTIONS));
  }

  private SymbolResponse symbolResponse(String symbolId) {
    Objects.requireNonNull(symbolId, NAME_SYMBOL_ID);
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

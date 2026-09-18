package org.openmbee.opensysml.conformance;

import com.google.protobuf.Message;
import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.AnalysisException;
import org.openmbee.opensysml.AnalysisOptions;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.Capabilities;
import org.openmbee.opensysml.CapabilityException;
import org.openmbee.opensysml.Condition;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ExecutionOptions;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Language;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.ModelException;
import org.openmbee.opensysml.ParseOptions;
import org.openmbee.opensysml.Query;
import org.openmbee.opensysml.QueryElement;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.proto.CompositeConstraint;
import org.openmbee.opensysml.proto.Constraint;
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
import org.openmbee.opensysml.proto.ListEnginesRequest;
import org.openmbee.opensysml.proto.ListEnginesResponse;
import org.openmbee.opensysml.proto.ParseFileRequest;
import org.openmbee.opensysml.proto.ParseFileResponse;
import org.openmbee.opensysml.proto.PrimitiveConstraint;
import org.openmbee.opensysml.proto.QueryRequest;
import org.openmbee.opensysml.proto.QueryResponse;
import org.openmbee.opensysml.proto.RunAnalysisRequest;
import org.openmbee.opensysml.proto.RunAnalysisResponse;
import org.openmbee.opensysml.proto.ServerInfoRequest;
import org.openmbee.opensysml.proto.ServerInfoResponse;
import org.openmbee.opensysml.proto.SymbolResponse;
import org.openmbee.opensysml.proto.Sysml;
import org.openmbee.opensysml.proto.ValidateInstanceRequest;
import org.openmbee.opensysml.proto.ValidateInstanceResponse;
import org.openmbee.opensysml.proto.VerifyConstraintRequest;
import org.openmbee.opensysml.proto.VerifyConstraintResponse;
import org.openmbee.opensysml.proto.VerifyRequirementRequest;
import org.openmbee.opensysml.proto.VerifyRequirementResponse;
import org.openmbee.opensysml.proto.VerifySatisfactionRequest;
import org.openmbee.opensysml.proto.VerifySatisfactionResponse;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.TreeSet;

/**
 * Makes a scenario's call through the client's public API and writes the answer back as the
 * response message the scenario is stated against. An RPC the API does not cover is reported as such
 * rather than called over the transport, which is what the skipped count in a report counts.
 */
final class Api {

  private static final String RPC_EVALUATE = "Evaluate";
  private static final String RPC_EVALUATE_CALC = "EvaluateCalc";
  private static final String RPC_EXECUTE_ACTION = "ExecuteAction";
  private static final String RPC_EXECUTE_STATE = "ExecuteState";
  private static final String RPC_GET_DIAGNOSTICS = "GetDiagnostics";
  private static final String RPC_GET_SERVER_INFO = "GetServerInfo";
  private static final String RPC_GET_SYMBOL = "GetSymbol";
  private static final String RPC_INSTANTIATE = "Instantiate";
  private static final String RPC_LIST_ENGINES = "ListEngines";
  private static final String RPC_PARSE_FILE = "ParseFile";
  private static final String RPC_QUERY = "Query";
  private static final String RPC_RUN_ANALYSIS = "RunAnalysis";
  private static final String RPC_VALIDATE_INSTANCE = "ValidateInstance";
  private static final String RPC_VERIFY_CONSTRAINT = "VerifyConstraint";
  private static final String RPC_VERIFY_REQUIREMENT = "VerifyRequirement";
  private static final String RPC_VERIFY_SATISFACTION = "VerifySatisfaction";

  /** The RPCs the public API covers, and so the ones a scenario can be run through it. */
  static final Set<String> COVERED =
      Set.of(
          RPC_GET_SERVER_INFO,
          RPC_LIST_ENGINES,
          RPC_PARSE_FILE,
          RPC_GET_SYMBOL,
          RPC_GET_DIAGNOSTICS,
          RPC_EVALUATE,
          RPC_INSTANTIATE,
          RPC_EXECUTE_ACTION,
          RPC_EXECUTE_STATE,
          RPC_VERIFY_CONSTRAINT,
          RPC_VERIFY_REQUIREMENT,
          RPC_VERIFY_SATISFACTION,
          RPC_VALIDATE_INSTANCE,
          RPC_EVALUATE_CALC,
          RPC_RUN_ANALYSIS,
          RPC_QUERY);

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

    /** The public API cannot make this call, so the scenario is skipped. */
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
      case RPC_LIST_ENGINES -> ListEnginesRequest.newBuilder();
      case RPC_PARSE_FILE -> ParseFileRequest.newBuilder();
      case RPC_GET_SYMBOL -> GetSymbolRequest.newBuilder();
      case RPC_GET_DIAGNOSTICS -> DiagnosticsRequest.newBuilder();
      case RPC_EVALUATE -> EvaluateRequest.newBuilder();
      case RPC_INSTANTIATE -> InstantiateRequest.newBuilder();
      case RPC_EXECUTE_ACTION -> ExecuteActionRequest.newBuilder();
      case RPC_EXECUTE_STATE -> ExecuteStateRequest.newBuilder();
      case RPC_VERIFY_CONSTRAINT -> VerifyConstraintRequest.newBuilder();
      case RPC_VERIFY_REQUIREMENT -> VerifyRequirementRequest.newBuilder();
      case RPC_VERIFY_SATISFACTION -> VerifySatisfactionRequest.newBuilder();
      case RPC_VALIDATE_INSTANCE -> ValidateInstanceRequest.newBuilder();
      case RPC_EVALUATE_CALC -> EvaluateCalcRequest.newBuilder();
      case RPC_RUN_ANALYSIS -> RunAnalysisRequest.newBuilder();
      case RPC_QUERY -> QueryRequest.newBuilder();
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
      return new Answer.Unsupported("the public API does not cover " + method);
    }
    try {
      return new Answer.Answered(
          switch (method) {
            case RPC_GET_SERVER_INFO -> serverInfo();
            case RPC_LIST_ENGINES -> listEngines();
            case RPC_PARSE_FILE -> parse((ParseFileRequest) request);
            case RPC_GET_SYMBOL -> symbol((GetSymbolRequest) request);
            case RPC_GET_DIAGNOSTICS -> diagnostics((DiagnosticsRequest) request);
            case RPC_EVALUATE -> evaluate((EvaluateRequest) request);
            case RPC_INSTANTIATE -> instantiate((InstantiateRequest) request);
            case RPC_EXECUTE_ACTION -> executeAction((ExecuteActionRequest) request);
            case RPC_EXECUTE_STATE -> executeState((ExecuteStateRequest) request);
            case RPC_VERIFY_CONSTRAINT -> verifyConstraint((VerifyConstraintRequest) request);
            case RPC_VERIFY_REQUIREMENT -> verifyRequirement((VerifyRequirementRequest) request);
            case RPC_VERIFY_SATISFACTION ->
                verifySatisfaction((VerifySatisfactionRequest) request);
            case RPC_VALIDATE_INSTANCE -> validateInstance((ValidateInstanceRequest) request);
            case RPC_EVALUATE_CALC -> evaluateCalc((EvaluateCalcRequest) request);
            case RPC_RUN_ANALYSIS -> runAnalysis((RunAnalysisRequest) request);
            case RPC_QUERY -> query((QueryRequest) request);
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

  /** A call the public API has no way to make, thrown where the request is read. */
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

  private ListEnginesResponse listEngines() {
    return ListEnginesResponse.newBuilder()
        .addAllEngines(Rendering.engines(connection.listEngines()))
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
                  "the public API always names a source, so it cannot send a request naming none");
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
      throw new Unsupported("the public API evaluates in a context or against a subject, not both");
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

  private ExecuteActionResponse executeAction(ExecuteActionRequest request) {
    Model model = connection.model(request.getModelHash());
    ExecutionOptions options = execution(request.getSchedule(), request.getPerformerSymbolId());
    Map<String, Value> inputs = values(request.getInputsMap());
    try {
      if (options.explores()) {
        Exploration exploration = model.exploreAction(request.getActionSymbolId(), inputs, options);
        return ExecuteActionResponse.newBuilder()
            .addAllOutcomes(Rendering.outcomes(exploration))
            .setExploration(Rendering.exploration(exploration))
            .build();
      }
      ActionRun run = model.executeAction(request.getActionSymbolId(), inputs, options);
      ExecuteActionResponse.Builder response =
          ExecuteActionResponse.newBuilder()
              .putAllOutputs(Rendering.values(run.outputs()))
              .addAllDiagnostics(Rendering.diagnostics(run.diagnostics()));
      run.finalTime().ifPresent(response::setFinalTime);
      return response.build();
    } catch (ModelException e) {
      return ExecuteActionResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private ExecuteStateResponse executeState(ExecuteStateRequest request) {
    Model model = connection.model(request.getModelHash());
    ExecutionOptions options = execution(request.getSchedule(), request.getPerformerSymbolId());
    try {
      if (options.explores()) {
        Exploration exploration =
            model.exploreState(
                request.getStateMachineSymbolId(), request.getEventsList(), options);
        return ExecuteStateResponse.newBuilder()
            .addAllOutcomes(Rendering.outcomes(exploration))
            .setExploration(Rendering.exploration(exploration))
            .build();
      }
      StateRun run =
          model.executeState(request.getStateMachineSymbolId(), request.getEventsList(), options);
      ExecuteStateResponse.Builder response =
          ExecuteStateResponse.newBuilder()
              .addAllStatesVisited(run.statesVisited())
              .putAllFinalContext(Rendering.values(run.finalContext()))
              .addAllDiagnostics(Rendering.diagnostics(run.diagnostics()));
      run.finalTime().ifPresent(response::setFinalTime);
      return response.build();
    } catch (ModelException e) {
      return ExecuteStateResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private VerifyConstraintResponse verifyConstraint(VerifyConstraintRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    try {
      Verification verification =
          request.getSubjectSymbolId().isEmpty()
              ? model.verifyConstraint(request.getSymbolId())
              : model.verifyConstraint(request.getSymbolId(), request.getSubjectSymbolId());
      return VerifyConstraintResponse.newBuilder()
          .setVerdict(Rendering.verdict(verification.verdict()))
          .addAllInstances(Rendering.instances(verification.instances()))
          .addAllDiagnostics(Rendering.diagnostics(verification.diagnostics()))
          .build();
    } catch (ModelException e) {
      return VerifyConstraintResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private VerifyRequirementResponse verifyRequirement(VerifyRequirementRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    try {
      Verification verification =
          request.getSubjectSymbolId().isEmpty()
              ? model.verifyRequirement(request.getSymbolId())
              : model.verifyRequirement(request.getSymbolId(), request.getSubjectSymbolId());
      return VerifyRequirementResponse.newBuilder()
          .setVerdict(Rendering.verdict(verification.verdict()))
          .addAllInstances(Rendering.instances(verification.instances()))
          .addAllDiagnostics(Rendering.diagnostics(verification.diagnostics()))
          .addAllVerificationVerdicts(
              Rendering.verificationVerdicts(verification.verifications()))
          .build();
    } catch (ModelException e) {
      return VerifyRequirementResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private VerifySatisfactionResponse verifySatisfaction(VerifySatisfactionRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    try {
      Satisfaction satisfaction =
          request.getSymbolId().isEmpty()
              ? model.verifySatisfaction()
              : model.verifySatisfaction(request.getSymbolId());
      return VerifySatisfactionResponse.newBuilder()
          .addAllVerdicts(Rendering.verdicts(satisfaction.verdicts()))
          .addAllInstances(Rendering.instances(satisfaction.instances()))
          .addAllDiagnostics(Rendering.diagnostics(satisfaction.diagnostics()))
          .addAllVerificationVerdicts(
              Rendering.verificationVerdicts(satisfaction.verifications()))
          .build();
    } catch (ModelException e) {
      return VerifySatisfactionResponse.newBuilder()
          .setError(e.getMessage())
          .setFailureReason(Rendering.failureReason(e.failureReason()))
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private ValidateInstanceResponse validateInstance(ValidateInstanceRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    try {
      Validation validation = model.validateInstance(request.getSymbolId());
      return ValidateInstanceResponse.newBuilder()
          .setSummary(Rendering.verdict(validation.summary()))
          .addAllVerdicts(Rendering.verdicts(validation.verdicts()))
          .addAllInstances(Rendering.instances(validation.instances()))
          .addAllDiagnostics(Rendering.diagnostics(validation.diagnostics()))
          .addAllVerificationVerdicts(Rendering.verificationVerdicts(validation.verifications()))
          .setBounded(validation.bounded())
          .build();
    } catch (ModelException e) {
      return ValidateInstanceResponse.newBuilder()
          .setError(e.getMessage())
          .setFailureReason(Rendering.failureReason(e.failureReason()))
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private EvaluateCalcResponse evaluateCalc(EvaluateCalcRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    try {
      Calculation calculation =
          model.evaluateCalc(request.getSymbolId(), values(request.getArgumentsList()));
      EvaluateCalcResponse.Builder response =
          EvaluateCalcResponse.newBuilder()
              .addAllOutputs(Rendering.outputs(calculation.outputs()))
              .addAllDiagnostics(Rendering.diagnostics(calculation.diagnostics()))
              .setEngine(calculation.standing().engine())
              .setStrength(calculation.standing().strength())
              .addAllBounds(Rendering.bounds(calculation.standing()));
      calculation.result().ifPresent(result -> response.setResult(Rendering.value(result)));
      return response.build();
    } catch (ModelException e) {
      return EvaluateCalcResponse.newBuilder()
          .setError(e.getMessage())
          .setFailureReason(Rendering.failureReason(e.failureReason()))
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private RunAnalysisResponse runAnalysis(RunAnalysisRequest request) {
    Model model = engine(connection.model(request.getModelHash()), request.getEngine());
    AnalysisOptions options =
        AnalysisOptions.defaults()
            .withArguments(values(request.getArgumentsList()))
            .withNamedArguments(values(request.getNamedArgumentsMap()));
    if (!request.getSubjectSymbolId().isEmpty()) {
      options = options.withSubject(request.getSubjectSymbolId());
    }
    if (!request.getSchedule().isEmpty()) {
      options = options.withSchedule(request.getSchedule());
    }
    try {
      if (options.explores()) {
        Exploration exploration = model.exploreAnalysis(request.getSymbolId(), options);
        return RunAnalysisResponse.newBuilder()
            .addAllOutcomes(Rendering.outcomes(exploration))
            .setExploration(Rendering.exploration(exploration))
            .build();
      }
      return analysis(model.runAnalysis(request.getSymbolId(), options)).build();
    } catch (AnalysisException e) {
      return e.partial()
          .map(Api::analysis)
          .orElseGet(
              () ->
                  RunAnalysisResponse.newBuilder()
                      .addAllDiagnostics(Rendering.diagnostics(e.diagnostics())))
          .setError(e.getMessage())
          .setFailureReason(Rendering.failureReason(e.failureReason()))
          .build();
    } catch (ModelException e) {
      return RunAnalysisResponse.newBuilder()
          .setError(e.getMessage())
          .setFailureReason(Rendering.failureReason(e.failureReason()))
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private static RunAnalysisResponse.Builder analysis(Analysis analysis) {
    return RunAnalysisResponse.newBuilder()
        .addAllOutputs(Rendering.outputs(analysis.outputs()))
        .addAllVerdicts(Rendering.verdicts(analysis.verdicts()))
        .addAllInstances(Rendering.instances(analysis.instances()))
        .addAllDiagnostics(Rendering.diagnostics(analysis.diagnostics()))
        .addAllVerificationVerdicts(Rendering.verificationVerdicts(analysis.verifications()))
        .addAllEvaluations(Rendering.evaluations(analysis.evaluations()))
        .setEngine(analysis.standing().engine())
        .setStrength(analysis.standing().strength())
        .addAllBounds(Rendering.bounds(analysis.standing()));
  }

  private QueryResponse query(QueryRequest request) {
    Model model = connection.model(request.getModelHash());
    if (request.hasQuery() && !request.getOslcQuery().isEmpty()) {
      throw new Unsupported("the public API sends a structured query or an OSLC one, not both");
    }
    List<QueryElement> elements =
        request.getOslcQuery().isEmpty()
            ? model.query(query(request.getQuery()))
            : model.queryOslc(request.getOslcQuery());
    return QueryResponse.newBuilder().addAllElements(Rendering.elements(elements)).build();
  }

  private static Query query(org.openmbee.opensysml.proto.Query query) {
    Query built = Query.all().withScope(query.getScopeList()).withSelect(query.getSelectList());
    return query.hasWhere() ? built.where(condition(query.getWhere())) : built;
  }

  private static Condition condition(Constraint constraint) {
    return switch (constraint.getConstraintCase()) {
      case PRIMITIVE -> {
        PrimitiveConstraint primitive = constraint.getPrimitive();
        Condition.Comparison comparison =
            switch (primitive.getOperator()) {
              case PRIMITIVE_OPERATOR_EQUAL ->
                  Condition.equal(primitive.getProperty(), primitive.getValueList());
              case PRIMITIVE_OPERATOR_GREATER ->
                  Condition.greater(primitive.getProperty(), soleValue(primitive));
              case PRIMITIVE_OPERATOR_LESS ->
                  Condition.less(primitive.getProperty(), soleValue(primitive));
              case PRIMITIVE_OPERATOR_UNSPECIFIED, UNRECOGNIZED ->
                  throw new Unsupported(
                      "the public API cannot send a comparison with no operator");
            };
        yield primitive.getInverse() ? comparison.negated() : comparison;
      }
      case COMPOSITE -> {
        CompositeConstraint composite = constraint.getComposite();
        List<Condition> conditions = composite.getConstraintList().stream().map(Api::condition).toList();
        yield switch (composite.getOperator()) {
          case COMPOSITE_OPERATOR_AND -> Condition.all(conditions);
          case COMPOSITE_OPERATOR_OR -> Condition.any(conditions);
          case COMPOSITE_OPERATOR_UNSPECIFIED, UNRECOGNIZED ->
              throw new Unsupported("the public API cannot send a combination with no operator");
        };
      }
      case CONSTRAINT_NOT_SET ->
          throw new Unsupported("the public API cannot send an empty condition");
    };
  }

  private static String soleValue(PrimitiveConstraint primitive) {
    if (primitive.getValueCount() != 1) {
      throw new Unsupported("the public API compares an ordering against one value");
    }
    return primitive.getValue(0);
  }

  private static ExecutionOptions execution(String schedule, String performer) {
    ExecutionOptions options = ExecutionOptions.defaults();
    if (!schedule.isEmpty()) {
      options = options.withSchedule(schedule);
    }
    if (!performer.isEmpty()) {
      options = options.withPerformer(performer);
    }
    return options;
  }

  private static Model engine(Model model, String engine) {
    return engine.isEmpty() ? model : model.withEngine(engine);
  }

  private static List<Value> values(List<org.openmbee.opensysml.proto.Value> values) {
    List<Value> read = new ArrayList<>(values.size());
    for (org.openmbee.opensysml.proto.Value value : values) {
      read.add(value(value));
    }
    return read;
  }

  private static Map<String, Value> values(Map<String, org.openmbee.opensysml.proto.Value> values) {
    Map<String, Value> read = new LinkedHashMap<>();
    values.forEach((name, value) -> read.put(name, value(value)));
    return read;
  }

  private static Value value(org.openmbee.opensysml.proto.Value value) {
    Optional<Value> read;
    try {
      read = Protos.value(value);
    } catch (TransportException malformed) {
      throw new Unsupported("the public API cannot send a value the client would refuse to read");
    }
    return read.orElseThrow(
        () -> new Unsupported("the public API cannot send a value of a kind it does not know"));
  }
}

package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.openmbee.mdk.magic.TestRequests.request;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.UnaryOperator;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.actions.QueryArguments;
import org.openmbee.mdk.magic.annotations.AnnotationPlan;
import org.openmbee.mdk.magic.annotations.AnnotationPlanner;
import org.openmbee.mdk.magic.bridge.DocGenBridge;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.QueryParameter;
import org.openmbee.mdk.magic.engine.QueryPlan;
import org.openmbee.mdk.magic.engine.QueryResolver;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.identity.IdentityIndex;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.model.SimpleElement;
import org.openmbee.mdk.magic.results.QueryAnswer;
import org.openmbee.mdk.magic.results.QueryFailure;
import org.openmbee.mdk.magic.results.ResultsMapper;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.mdk.magic.source.V2TextualSource;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.DocumentQueryResult;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.QueryOutcome;
import org.openmbee.opensysml.QueryOutcomeException;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.StatusCode;

class QueryRunTest {
  private static final Duration ELAPSED = Duration.ofMillis(7);
  private static final QueryParameter ROOT =
      new QueryParameter("Q", "root", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, false, false);
  private static final QueryParameter KIND =
      new QueryParameter("Q", "kind", "ScalarValues::String", QueryParameter.Kind.STRING, false, true);
  private static final QueryParameter LIMITS =
      new QueryParameter("Q", "limits", "ScalarValues::Integer", QueryParameter.Kind.INTEGER, true, false);
  private static final QueryPlan PLAN = new QueryPlan(List.of("Q"), List.of(ROOT, KIND, LIMITS));

  private static DocumentRow row(String element, List<List<DocumentValue>> cells) {
    return new DocumentRow(new DocumentValue.ElementRef(element, "PartUsage"), cells,
        Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty());
  }

  private static DocumentQueryResult parts() {
    return new DocumentQueryResult(List.of("qualifiedName", "mass", "owner"), List.of(
        row("A::a", List.of(
            List.of(new DocumentValue.StringValue("A::a")),
            List.of(new DocumentValue.RealValue(1.5)),
            List.of(new DocumentValue.ElementRef("A", "Package")))),
        row("A::b", List.of(
            List.of(new DocumentValue.StringValue("A::b")),
            List.of(),
            List.of(new DocumentValue.ElementRef("A", "Package"), new DocumentValue.ElementRef("A::a", "PartUsage"))))));
  }

  @Test
  void rowsBecomeATableWhoseCellsNameElements() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::Q"),
        List.of(QueryAnswer.rows("A::Q", parts())), ELAPSED);
    assertEquals(Status.PASSED, result.status());
    assertEquals(ELAPSED, result.elapsed());
    assertEquals(1, result.tables().size());
    RunResult.Table table = result.tables().get(0);
    assertEquals("A::Q", table.query());
    assertEquals(List.of("qualifiedName", "mass", "owner"), table.columns());
    assertEquals("A::a", table.rows().get(0).elementId());
    assertEquals(new RunResult.Cell("A::a", "A::a"), table.rows().get(0).cells().get(0));
    assertEquals(new RunResult.Cell("1.5", null), table.rows().get(0).cells().get(1));
    assertEquals(new RunResult.Cell("A", "A"), table.rows().get(0).cells().get(2));
    assertEquals(new RunResult.Cell("", null), table.rows().get(1).cells().get(1));
    assertEquals(new RunResult.Cell("A, A::a", null), table.rows().get(1).cells().get(2), "a cell of many names no one element");
    assertEquals(List.of(new RunResult.Outcome("A::Q", "2 rows of qualifiedName, mass, owner", Status.PASSED, "A::Q")),
        result.outcomes(), "the query is the subject, so it gets the one outcome");
  }

  @Test
  void noRowsIsAnInformationalSuccess() {
    DocumentQueryResult empty = new DocumentQueryResult(List.of("qualifiedName"), List.of());
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::Q"),
        List.of(QueryAnswer.rows("A::Q", empty)), ELAPSED);
    assertEquals(Status.INFO, result.status());
    assertEquals("no rows", result.outcomes().get(0).detail());
    assertEquals(0, result.rowCount());
  }

  @Test
  void aFailedQueryIsAnErrorOutcomeCarryingTheServiceMessage() {
    ServiceException failure = new ServiceException(StatusCode.INVALID_ARGUMENT, "query A::Q names no element A::Nope");
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::doc"),
        List.of(QueryAnswer.rows("A::Q", parts()), QueryAnswer.failed("A::R", failure)), ELAPSED);
    assertEquals(Status.ERROR, result.status());
    assertEquals(List.of(
        new RunResult.Outcome("A::doc", "2 queries", Status.ERROR, "A::doc"),
        new RunResult.Outcome("A::Q", "2 rows of qualifiedName, mass, owner", Status.PASSED, "A::Q"),
        new RunResult.Outcome("A::R", "query A::Q names no element A::Nope", Status.ERROR, "A::R")),
        result.outcomes());
    assertEquals(1, result.tables().size(), "the query that answered still shows its rows");
  }

  private static QueryOutcomeException pathError(QueryOutcome.Failure... failures) {
    return new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query A::R reads feature mass of A::a, which has no such feature",
        new QueryOutcome(QueryOutcome.PATH_ERROR, "Feature(mass)", List.of(failures), List.of(), List.of()));
  }

  private static QueryOutcome.Failure missingMass(String at) {
    return new QueryOutcome.Failure("Feature(mass)", "a feature named mass on the element",
        new QueryOutcome.Frontier(at, List.of(), List.of(), List.of(), ""));
  }

  @Test
  void aReportedPathErrorIsAFailedOutcomePerFailureAtItsFrontierElement() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::doc"), List.of(
        QueryAnswer.rows("A::Q", parts()),
        QueryAnswer.failed("A::R", pathError(missingMass("A::a"), missingMass("A::b")))), ELAPSED);
    assertEquals(Status.FAILED, result.status(), "the query ran and reported; it was not refused");
    assertEquals(List.of(
        new RunResult.Outcome("A::doc", "2 queries", Status.FAILED, "A::doc"),
        new RunResult.Outcome("A::Q", "2 rows of qualifiedName, mass, owner", Status.PASSED, "A::Q"),
        new RunResult.Outcome("A::R", "query A::R reads feature mass of A::a, which has no such feature", Status.FAILED, "A::R"),
        new RunResult.Outcome("A::R", "path error at Feature(mass): expected a feature named mass on the element; at A::a", Status.FAILED, "A::a"),
        new RunResult.Outcome("A::R", "path error at Feature(mass): expected a feature named mass on the element; at A::b", Status.FAILED, "A::b")),
        result.outcomes());
    assertEquals(1, result.tables().size(), "the query that answered still shows its rows");
  }

  @Test
  void aFrontierListsItsCandidatesAndTheNearestSpelling() {
    QueryOutcome.Failure misspelled = new QueryOutcome.Failure(
        "Named(A::a::cabni)", "exactly one element with that qualified name",
        new QueryOutcome.Frontier("A::a",
            List.of(new QueryOutcome.Frontier.Member("cabin", "PartUsage"), new QueryOutcome.Frontier.Member("engine", "PartUsage")),
            List.of(new QueryOutcome.Frontier.Feature("mass", "MassValue"), new QueryOutcome.Frontier.Feature("name", "")),
            List.of("connect", "satisfy"), "cabin"));
    assertEquals(
        "path error at Named(A::a::cabni): expected exactly one element with that qualified name; at A::a;"
            + " members: [cabin (PartUsage), engine (PartUsage)]; features: [mass : MassValue, name];"
            + " kinds: [connect, satisfy]; nearest: cabin",
        QueryFailure.outcomes("A::R", pathError(misspelled), UnaryOperator.identity()).get(1).detail());
    QueryOutcome.Failure nowhere = new QueryOutcome.Failure("Feature(colour)", "a property of the metamodel",
        new QueryOutcome.Frontier("", List.of(), List.of(new QueryOutcome.Frontier.Feature("color", "")), List.of(), "color"));
    RunResult.Outcome unplaced = QueryFailure.outcomes("A::R", pathError(nowhere), UnaryOperator.identity()).get(1);
    assertEquals("path error at Feature(colour): expected a property of the metamodel; features: [color]; nearest: color",
        unplaced.detail());
    assertNull(unplaced.elementId(), "a path that stopped at no element annotates none");
  }

  @Test
  void whatTheServiceCutIsCountedAfterWhatItSent() {
    QueryOutcome.Failure cut = new QueryOutcome.Failure("Related(haul)", "a relationship kind of the metamodel",
        new QueryOutcome.Frontier("A::a",
            List.of(new QueryOutcome.Frontier.Member("cab", "PartUsage")), List.of(), List.of("connect", "satisfy"), "",
            23, 301, 2));
    QueryOutcomeException failure = new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query A::R has no kind haul",
        new QueryOutcome(QueryOutcome.PATH_ERROR, "Related(haul)", List.of(cut, missingMass("A::b")), List.of(), List.of(), 11, 0, 0));
    List<RunResult.Outcome> outcomes = QueryFailure.outcomes("A::R", failure, UnaryOperator.identity());
    assertEquals(
        "path error at Related(haul): expected a relationship kind of the metamodel; at A::a;"
            + " members: [cab (PartUsage), … and 22 more]; features: [… and 301 more]; kinds: [connect, satisfy]",
        outcomes.get(1).detail());
    assertEquals(new RunResult.Outcome("A::R", "… and 9 more path errors at Related(haul)", Status.FAILED, null),
        outcomes.get(3), "the failures left out are counted, and annotate nothing");
    assertEquals(4, outcomes.size());
    QueryOutcomeException one = new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query A::R has no kind haul",
        new QueryOutcome(QueryOutcome.PATH_ERROR, "Related(haul)", List.of(cut), List.of(), List.of(), 2, 0, 0));
    assertEquals("… and 1 more path error at Related(haul)", QueryFailure.outcomes("A::R", one, UnaryOperator.identity()).get(2).detail());
  }

  @Test
  void aModelErrorIsAFailedOutcomePerFindingThenPerPathFailure() {
    QueryOutcome outcome = new QueryOutcome(QueryOutcome.MODEL_ERROR, "Named(A::part)", List.of(),
        List.of(new QueryOutcome.Finding("Named(A::part)", "A", "ambiguousName", "A::part (declared 2 times)")),
        List.of(missingMass("A::b")), 0, 4, 3);
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::R"), List.of(QueryAnswer.failed("A::R",
        new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query A::R names 2 elements A::part", outcome))), ELAPSED);
    assertEquals(Status.FAILED, result.status());
    assertEquals(List.of(
        new RunResult.Outcome("A::R", "query A::R names 2 elements A::part", Status.FAILED, "A::R"),
        new RunResult.Outcome("A::R", "model error at Named(A::part): A violates ambiguousName: A::part (declared 2 times)", Status.FAILED, "A"),
        new RunResult.Outcome("A::R", "… and 3 more model errors at Named(A::part)", Status.FAILED, null),
        new RunResult.Outcome("A::R", "path error at Feature(mass): expected a feature named mass on the element; at A::b", Status.FAILED, "A::b"),
        new RunResult.Outcome("A::R", "… and 2 more path errors at Named(A::part)", Status.FAILED, null)),
        result.outcomes());
  }

  @Test
  void aRefusalOutranksAReportedFailureAndAnOutcomeOfAnotherKindIsARefusal() {
    ServiceException refused = new ServiceException(StatusCode.INVALID_ARGUMENT, "query A::S requires binding root");
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::doc"), List.of(
        QueryAnswer.failed("A::R", pathError(missingMass("A::a"))), QueryAnswer.failed("A::S", refused)), ELAPSED);
    assertEquals(Status.ERROR, result.status());
    assertEquals(new RunResult.Outcome("A::S", "query A::S requires binding root", Status.ERROR, "A::S"),
        result.outcomes().get(result.outcomes().size() - 1));
    QueryOutcomeException unknown = new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query A::R timed out",
        new QueryOutcome("budgetError", "Descendants", List.of(missingMass("A::a")), List.of(), List.of()));
    assertEquals(List.of(new RunResult.Outcome("A::R", "query A::R timed out", Status.ERROR, "A::R")),
        QueryFailure.outcomes("A::R", unknown, UnaryOperator.identity()), "a kind this plugin does not know is not read as a path error");
  }

  /** A source whose service names carry a root the host's names lack, as a migrated project's do. */
  private static final ModelSource ROOTED = new ModelSource() {
    @Override
    public ModelPath path() {
      return ModelPath.V1_MDZIP;
    }

    @Override
    public List<SourceDocument> sources(Connection connection) {
      return List.of();
    }

    @Override
    public List<Diagnostic> exportDiagnostics() {
      return List.of();
    }

    @Override
    public String subjectName(String qualifiedName) {
      return "Model::" + qualifiedName;
    }

    @Override
    public String elementName(String subjectName) {
      return subjectName.startsWith("Model::") ? subjectName.substring("Model::".length()) : subjectName;
    }
  };

  @Test
  void aReportedFailureAnnotatesTheElementsItIsAtUnderTheirHostNames() {
    QueryOutcome outcome = new QueryOutcome(QueryOutcome.MODEL_ERROR, "Feature(mass)", List.of(),
        List.of(new QueryOutcome.Finding("Feature(mass)", "Model::A::b", "multiplicity", "2 values")),
        List.of(missingMass("Model::A::a"), missingMass("Model::A::gone")));
    RunResult result = ResultsMapper.map(new RunRequest(Operation.RUN_QUERY, ROOTED, "A::R", List.of()),
        List.of(QueryAnswer.failed("Model::A::R",
            new QueryOutcomeException(StatusCode.FAILED_PRECONDITION, "query Model::A::R reads mass", outcome))), ELAPSED)
        .withOutcomeNames(ROOTED::elementName);
    assertEquals(List.of(
        new RunResult.Outcome("A::R", "query Model::A::R reads mass", Status.FAILED, "A::R"),
        new RunResult.Outcome("A::R", "model error at Feature(mass): A::b violates multiplicity: 2 values", Status.FAILED, "A::b"),
        new RunResult.Outcome("A::R", "path error at Feature(mass): expected a feature named mass on the element; at A::a", Status.FAILED, "A::a"),
        new RunResult.Outcome("A::R", "path error at Feature(mass): expected a feature named mass on the element; at A::gone", Status.FAILED, "A::gone")),
        result.outcomes(), "the elements are named as the host names them; the service's own words stay");
    var index = IdentityIndex.of(List.of(
        new SimpleElement("1", "A::a", "a", "a"), new SimpleElement("2", "A::b", "b", "b"),
        new SimpleElement("3", "A::R", "R", "query")));
    List<AnnotationPlan> plans = AnnotationPlanner.plan(result, index);
    assertEquals(List.of("query", "b", "a"), plans.stream().map(plan -> plan.target().handle()).toList(),
        "an element the host does not know is left out: " + plans);
    assertTrue(plans.stream().allMatch(plan -> plan.severity() == AnnotationPlan.Severity.ERROR), plans.toString());
    assertEquals("model error at Feature(mass): A::b violates multiplicity: 2 values", plans.get(1).text());
    assertEquals("Run query", plans.get(1).kind());
  }

  @Test
  void aReportedFailureCrossesTheBridgeAsItsOutcomes() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::R"),
        List.of(QueryAnswer.failed("A::R", pathError(missingMass("A::a")))), ELAPSED);
    Map<String, Object> flat = DocGenBridge.flatten(result);
    assertEquals("FAILED", flat.get("status"));
    assertEquals(List.of(
        Map.of("label", "A::R", "detail", "query A::R reads feature mass of A::a, which has no such feature",
            "status", "FAILED", "elementId", "A::R"),
        Map.of("label", "A::R", "detail", "path error at Feature(mass): expected a feature named mass on the element; at A::a",
            "status", "FAILED", "elementId", "A::a")),
        flat.get("outcomes"));
    assertEquals(List.of(), flat.get("tables"));
  }

  @Test
  void rendersEveryDocumentValue() {
    assertEquals("A::a", ResultsMapper.render(new DocumentValue.ElementRef("A::a", "")));
    assertEquals("#3", ResultsMapper.render(new DocumentValue.ObjectRef(3, "", Optional.empty())));
    assertEquals("A::a.wheels[2]", ResultsMapper.render(new DocumentValue.ObjectRef(3, "A::a.wheels[2]", Optional.empty())));
    assertEquals("7", ResultsMapper.render(new DocumentValue.IntegerValue(7)));
    assertEquals("12345678901234567890", ResultsMapper.render(
        new DocumentValue.BigIntegerValue(new java.math.BigInteger("12345678901234567890"))));
    assertEquals("true", ResultsMapper.render(new DocumentValue.BooleanValue(true)));
    assertEquals("*", ResultsMapper.render(new DocumentValue.InfinityValue()));
    assertEquals("text", ResultsMapper.render(new DocumentValue.StringValue("text")));
  }

  @Test
  void renamingFollowsTableElementsAndTheCellsNamingThem() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::Q"),
        List.of(QueryAnswer.rows("Model::A::Q", parts())), ELAPSED)
        .withOutcomeNames(name -> name.startsWith("A") ? "Root::" + name : name);
    RunResult.Table table = result.tables().get(0);
    assertEquals("Root::A::a", table.rows().get(0).elementId());
    assertEquals(new RunResult.Cell("Root::A::a", "Root::A::a"), table.rows().get(0).cells().get(0));
    assertEquals(new RunResult.Cell("1.5", null), table.rows().get(0).cells().get(1));
    assertEquals("Model::A::Q", table.query(), "names the rename leaves alone stay");
  }

  @Test
  void rowsAnnotateTheElementsTheyWereComputedFor() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::Q"),
        List.of(QueryAnswer.rows("A::Q", parts())), ELAPSED);
    var index = IdentityIndex.of(List.of(
        new SimpleElement("1", "A::a", "a", "a"), new SimpleElement("2", "A::b", "b", "b")));
    List<AnnotationPlan> plans = AnnotationPlanner.plan(result, index);
    assertEquals(2, plans.size(), plans.toString());
    assertEquals("a", plans.get(0).target().handle());
    assertEquals(AnnotationPlan.Severity.INFO, plans.get(0).severity());
    assertEquals("A::Q: qualifiedName = A::a, mass = 1.5, owner = A", plans.get(0).text());
    assertEquals("b", plans.get(1).target().handle());
  }

  @Test
  void flattensTablesIntoJdkValues() {
    RunResult result = ResultsMapper.map(request(Operation.RUN_QUERY, "A::Q"),
        List.of(QueryAnswer.rows("A::Q", parts())), ELAPSED);
    Map<String, Object> flat = DocGenBridge.flatten(result);
    List<?> tables = (List<?>) flat.get("tables");
    Map<?, ?> table = (Map<?, ?>) tables.get(0);
    assertEquals("A::Q", table.get("query"));
    assertEquals(List.of("qualifiedName", "mass", "owner"), table.get("columns"));
    List<?> rows = (List<?>) table.get("rows");
    Map<?, ?> first = (Map<?, ?>) rows.get(0);
    assertEquals("A::a", first.get("elementId"));
    List<?> cells = (List<?>) first.get("cells");
    assertEquals(Map.of("text", "A::a", "elementId", "A::a"), cells.get(0));
    assertEquals(Map.of("text", "1.5"), cells.get(1));
    Map<?, ?> second = (Map<?, ?>) rows.get(1);
    assertEquals(Map.of("text", "A, A::a"), ((List<?>) second.get("cells")).get(2));
  }

  @Test
  void parsesNamedAndPositionalBindingsByParameterType() {
    Map<String, List<DocumentValue>> named = QueryArguments.parse(
        "kind = \"Part; Definition\"; root = Demo::'Vehicle Design'::Car; limits = 1, 22", PLAN);
    assertEquals(List.of(new DocumentValue.StringValue("Part; Definition")), named.get("kind"));
    assertEquals(List.of(new DocumentValue.ElementRef("Demo::Vehicle Design::Car", "")), named.get("root"));
    assertEquals(List.of(new DocumentValue.IntegerValue(1), new DocumentValue.IntegerValue(22)), named.get("limits"));
    Map<String, List<DocumentValue>> positional = QueryArguments.parse("Demo::Car; PartUsage", PLAN);
    assertEquals(List.of("root", "kind"), List.copyOf(positional.keySet()));
    assertEquals(List.of(new DocumentValue.StringValue("PartUsage")), positional.get("kind"));
    assertTrue(QueryArguments.parse("  ", PLAN).isEmpty());
  }

  @Test
  void rejectsBindingsTheQueryDoesNotDeclareOrCannotRead() {
    assertEquals("no parameter named depth; the query declares root, kind, limits",
        assertThrows(IllegalArgumentException.class, () -> QueryArguments.parse("depth = 2", PLAN)).getMessage());
    assertEquals("argument 4 has no parameter; the query declares root, kind, limits",
        assertThrows(IllegalArgumentException.class, () -> QueryArguments.parse("a; b; 1; d", PLAN)).getMessage());
    assertEquals("parameter limits takes ScalarValues::Integer, not many",
        assertThrows(IllegalArgumentException.class, () -> QueryArguments.parse("limits = many", PLAN)).getMessage());
    assertEquals("parameter root takes one value, not 2",
        assertThrows(IllegalArgumentException.class, () -> QueryArguments.parse("root = A, B", PLAN)).getMessage());
    QueryParameter flag = new QueryParameter("Q", "flag", "ScalarValues::Boolean", QueryParameter.Kind.BOOLEAN, false, false);
    QueryParameter ratio = new QueryParameter("Q", "ratio", "ScalarValues::Real", QueryParameter.Kind.REAL, false, false);
    QueryPlan scalars = new QueryPlan(List.of("Q"), List.of(flag, ratio));
    assertEquals(List.of(new DocumentValue.BooleanValue(true), new DocumentValue.RealValue(2.0)),
        List.of(QueryArguments.parse("true; 2", scalars).get("flag").get(0), QueryArguments.parse("true; 2", scalars).get("ratio").get(0)));
    assertThrows(IllegalArgumentException.class, () -> QueryArguments.parse("yes", scalars));
  }

  @Test
  void parameterKindsFollowThePrimitiveAndThePlanKnowsItsSoleElement() {
    assertEquals(QueryParameter.Kind.ELEMENT, QueryParameter.kindOf(""));
    assertEquals(QueryParameter.Kind.STRING, QueryParameter.kindOf("String"));
    assertEquals(QueryParameter.Kind.INTEGER, QueryParameter.kindOf("ScalarValues::Natural"));
    assertEquals(QueryParameter.Kind.REAL, QueryParameter.kindOf("Rational"));
    assertEquals(QueryParameter.Kind.BOOLEAN, QueryParameter.kindOf("Boolean"));
    assertEquals(Optional.of(ROOT), PLAN.soleElementParameter());
    QueryParameter other = new QueryParameter("Q", "scope", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, true, false);
    assertEquals(Optional.empty(), new QueryPlan(List.of("Q"), List.of(ROOT, other)).soleElementParameter());
    QueryParameter valued = new QueryParameter("Q", "scope", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, true, true);
    assertEquals(Optional.of(ROOT), new QueryPlan(List.of("Q"), List.of(ROOT, valued)).soleElementParameter(),
        "an element parameter with a value of its own needs no default");
    assertThrows(IllegalArgumentException.class, () -> new QueryPlan(List.of(), List.of()));
  }

  @Test
  void parametersSharedByQueriesAreNamedByTheirQuery() {
    QueryParameter first = new QueryParameter("A::Q", "root", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, false, false);
    QueryParameter second = new QueryParameter("A::R", "root", "ScalarValues::String", QueryParameter.Kind.STRING, false, false);
    QueryPlan plan = new QueryPlan(List.of("A::Q", "A::R"), List.of(first, second, KIND));
    assertEquals("A::Q::root", plan.key(first));
    assertEquals("A::R::root", plan.key(second));
    assertEquals("kind", plan.key(KIND));
    assertEquals(Optional.of(second), plan.parameter("A::R::root"));
    assertEquals(Optional.of(KIND), plan.parameter("kind"));
    assertEquals(Optional.empty(), plan.parameter("depth"));
    assertEquals("root is a parameter of several queries; name it as one of A::Q::root, A::R::root",
        assertThrows(IllegalArgumentException.class, () -> plan.parameter("root")).getMessage());
    Map<String, List<DocumentValue>> named = QueryArguments.parse("A::R::root = \"x\"; A::Q::root = Demo::a", plan);
    assertEquals(List.of(new DocumentValue.StringValue("x")), named.get("A::R::root"));
    assertEquals(List.of(new DocumentValue.ElementRef("Demo::a", "")), named.get("A::Q::root"));
    assertEquals(List.of("A::Q::root", "A::R::root"), List.copyOf(QueryArguments.parse("Demo::a; x", plan).keySet()),
        "positional bindings take the key the dialog and the engine use");
  }

  @Test
  void migrationTargetsDropTheirKeywordsAndQuotes() {
    assertEquals("Model::Fleet Documents::Fleet Brief Document",
        QueryResolver.targetName("part def Model::'Fleet Documents'::'Fleet Brief Document'"));
    assertEquals("Model::Fleet Documents::Fleet Handbook Document::Introduction::table",
        QueryResolver.targetName("part Model::'Fleet Documents'::'Fleet Handbook Document'::Introduction::table"));
    assertEquals("Collect Owned Elements", QueryResolver.targetName("'Collect Owned Elements'"));
    assertEquals("Model::Fleet Viewpoints::Notes", QueryResolver.targetName("Model::'Fleet Viewpoints'::Notes"));
  }

  @Test
  void requestsCarryBindingsForTheRun() {
    RunRequest plain = new RunRequest(Operation.RUN_QUERY, new V2TextualSource("package Demo;"), "Demo::Q", List.of());
    assertTrue(plain.queryBindings().isEmpty());
    RunRequest bound = plain.withQueryBindings(Map.of("root", List.of(new DocumentValue.ElementRef("Demo", ""))));
    assertEquals(List.of(new DocumentValue.ElementRef("Demo", "")), bound.queryBindings().get("root"));
    assertNull(bound.queryBindings().get("kind"));
    assertEquals("Demo::Q", bound.subjectQualifiedName());
  }
}

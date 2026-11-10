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
import org.openmbee.mdk.magic.model.SimpleElement;
import org.openmbee.mdk.magic.results.QueryAnswer;
import org.openmbee.mdk.magic.results.ResultsMapper;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.mdk.magic.source.V2TextualSource;
import org.openmbee.opensysml.DocumentQueryResult;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.ServiceException;
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

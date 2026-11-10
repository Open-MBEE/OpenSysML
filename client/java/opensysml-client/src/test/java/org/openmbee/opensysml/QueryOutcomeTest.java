package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Path;
import java.util.Arrays;
import java.util.List;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

/**
 * The structured outcome a failed query carries, decoded from a real service over the QVTS
 * examples: each record is the one the CLI's {@code -json} prints for the same query.
 */
class QueryOutcomeTest {

  private static Connection connection;
  private static Path examples;

  @BeforeAll
  static void open() {
    connection = Connection.open(ServiceBinary.options().build());
    examples = ServiceBinary.repositoryRoot().orElseThrow().resolve("examples").resolve("qvts");
  }

  @AfterAll
  static void close() {
    if (connection != null) {
      connection.close();
    }
  }

  private static Model lander(String... files) {
    return connection.parseSources(
        Arrays.stream(files).map(examples::resolve).map(SourceDocument::file).toList());
  }

  @Test
  void aPathErrorNamesEachFailingElementAndItsFrontier() {
    Model model = lander("lander.sysml", "queries.sysml");
    QueryOutcomeException failed =
        assertThrows(
            QueryOutcomeException.class,
            () -> model.runDocumentQuery("QVTSDemo::MissingMassProject"));

    assertEquals(StatusCode.FAILED_PRECONDITION, failed.status());
    assertTrue(
        failed.serviceMessage().contains("reads feature mass of Lander::lander::cabinEngine"),
        failed.serviceMessage());
    QueryOutcome outcome = failed.outcome();
    assertTrue(outcome.isPathError());
    assertEquals(QueryOutcome.PATH_ERROR, outcome.kind());
    assertEquals("Feature(mass)", outcome.step());
    assertEquals(
        List.of(
            new QueryOutcome.Failure(
                "Feature(mass)",
                "a feature named mass on the element",
                new QueryOutcome.Frontier(
                    "Lander::lander::cabinEngine", List.of(), List.of(), List.of(), "")),
            new QueryOutcome.Failure(
                "Feature(mass)",
                "a feature named mass on the element",
                new QueryOutcome.Frontier(
                    "Lander::lander::engineBay", List.of(), List.of(), List.of(), ""))),
        outcome.failures());
    assertEquals(List.of(), outcome.findings());
    assertEquals(List.of(), outcome.pathFailures());
  }

  @Test
  void aMisspelledNameListsTheMembersThereAndTheNearestSpelling() {
    Model model = lander("lander.sysml", "queries.sysml");
    QueryOutcomeException failed =
        assertThrows(
            QueryOutcomeException.class,
            () -> model.runDocumentQuery("QVTSDemo::MisspelledName"));

    QueryOutcome outcome = failed.outcome();
    assertEquals("Named(Lander::lander::cabni)", outcome.step());
    assertEquals(1, outcome.failures().size(), outcome.toString());
    QueryOutcome.Failure failure = outcome.failures().get(0);
    assertEquals("Named(Lander::lander::cabni)", failure.step());
    assertEquals("exactly one element with that qualified name", failure.expected());
    assertEquals(
        new QueryOutcome.Frontier(
            "Lander::lander",
            List.of(
                new QueryOutcome.Frontier.Member("cabin", "PartUsage"),
                new QueryOutcome.Frontier.Member("cabinEngine", "ConnectionUsage"),
                new QueryOutcome.Frontier.Member("engine", "PartUsage"),
                new QueryOutcome.Frontier.Member("engineBay", "ConnectionUsage"),
                new QueryOutcome.Frontier.Member("scienceBay", "PartUsage")),
            List.of(),
            List.of(),
            "cabin"),
        failure.frontier());
  }

  @Test
  void aModelErrorNamesTheConstraintTheModelBreaks() {
    Model model = lander("lander.sysml", "duplicates.sysml", "queries.sysml");
    QueryOutcomeException failed =
        assertThrows(
            QueryOutcomeException.class, () -> model.runDocumentQuery("QVTSDemo::AmbiguousPart"));

    assertEquals(StatusCode.FAILED_PRECONDITION, failed.status());
    assertTrue(
        failed.serviceMessage().contains("Duplicates::repeated (declared 2 times)"),
        failed.serviceMessage());
    QueryOutcome outcome = failed.outcome();
    assertTrue(outcome.isModelError());
    assertEquals(QueryOutcome.MODEL_ERROR, outcome.kind());
    assertEquals("Named(Duplicates::repeated)", outcome.step());
    assertEquals(List.of(), outcome.failures());
    assertEquals(
        List.of(
            new QueryOutcome.Finding(
                "Named(Duplicates::repeated)",
                "Duplicates",
                "ambiguousName",
                "Duplicates::repeated (declared 2 times)")),
        outcome.findings());
    assertEquals(List.of(), outcome.pathFailures());
  }

  @Test
  void aQueryNamingAnUnknownPropertyCarriesTheSameOutcome() {
    Model model = lander("lander.sysml");
    QueryOutcomeException failed =
        assertThrows(
            QueryOutcomeException.class, () -> model.queryOslc("oslc.where=sysml:nmae=\"lander\""));

    assertEquals(StatusCode.INVALID_ARGUMENT, failed.status());
    QueryOutcome outcome = failed.outcome();
    assertTrue(outcome.isPathError());
    assertEquals("Feature(sysml:nmae)", outcome.step());
    assertEquals(1, outcome.failures().size(), outcome.toString());
    QueryOutcome.Failure failure = outcome.failures().get(0);
    assertEquals("a property of the metamodel", failure.expected());
    assertEquals("", failure.frontier().at(), "a translator refusal resolved nothing");
    assertEquals("sysml:name", failure.frontier().nearest());
    assertTrue(
        failure.frontier().features().contains(new QueryOutcome.Frontier.Feature("sysml:name", "")),
        failure.frontier().features().toString());
  }
}

package org.openmbee.opensysml.conformance;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonPrimitive;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Message;
import com.google.protobuf.util.JsonFormat;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.ModelException;
import org.openmbee.opensysml.OpenSysMLException;
import org.openmbee.opensysml.ParseOptions;
import java.io.IOException;
import java.io.PrintStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.TreeSet;
import java.util.function.UnaryOperator;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Runs scenarios against one service through one encoding of the Connect protocol. */
final class Runner {

  private static final String DETAIL_FORMAT = "       %s%n";
  private static final Pattern FIXTURE_REFERENCE = Pattern.compile("^\\$\\{fixture:([^}]+)}$");

  private final String protocol;
  private final Connection connection;
  private final Api api;
  private final Path fixtures;
  private final List<String> capabilities;
  private final Map<Scenario.Fixture, String> models = new HashMap<>();
  private final PrintStream out;
  private final boolean verbose;
  private final UnaryOperator<Message> mutation;

  Runner(
      String protocol,
      Connection connection,
      Path fixtures,
      PrintStream out,
      boolean verbose,
      UnaryOperator<Message> mutation) {
    this.protocol = protocol;
    this.connection = connection;
    this.api = new Api(connection);
    this.fixtures = fixtures;
    this.capabilities = List.copyOf(new TreeSet<>(connection.capabilities().names()));
    this.out = out;
    this.verbose = verbose;
    this.mutation = mutation;
  }

  /**
   * Runs every scenario whose id matches, reporting as it goes.
   *
   * @param scenarios the scenarios to run
   * @param filter an id pattern, or empty for all of them
   * @return this protocol's summary
   */
  Report.Summary runAll(List<Scenario> scenarios, Optional<Pattern> filter) {
    Report.Summary summary = new Report.Summary(protocol, connection.address(), capabilities);
    for (Scenario scenario : scenarios) {
      if (filter.isPresent() && !filter.get().matcher(scenario.id()).find()) {
        continue;
      }
      Report.Result result = run(scenario);
      summary.results.add(result);
      summary.total++;
      switch (result.outcome) {
        case "pass" -> summary.passed++;
        case "fail" -> summary.failed++;
        case "skip" -> summary.skipped++;
        default -> summary.errored++;
      }
      report(result);
    }
    out.printf(
        "%n[%s] %d scenarios: %d passed, %d failed, %d skipped, %d in error%n",
        protocol, summary.total, summary.passed, summary.failed, summary.skipped, summary.errored);
    return summary;
  }

  private void report(Report.Result result) {
    String mark =
        switch (result.outcome) {
          case "pass" -> "PASS";
          case "fail" -> "FAIL";
          case "skip" -> "SKIP";
          default -> "ERR ";
        };
    out.printf("%s %-46s %s%n", mark, result.id, result.status);
    if (result.reason != null) {
      out.printf(DETAIL_FORMAT, result.reason);
    }
    if (result.failures != null) {
      result.failures.forEach(failure -> out.printf(DETAIL_FORMAT, failure));
    }
  }

  /** Runs one scenario: parses the model it names, makes the call, compares the answer. */
  private Report.Result run(Scenario scenario) {
    long started = System.nanoTime();
    Report.Result result = new Report.Result(scenario.id(), scenario.method());
    try {
      execute(scenario, result);
    } catch (RuntimeException e) {
      errored(result, e.toString());
    }
    result.durationMs = (System.nanoTime() - started) / 1_000_000.0;
    return result;
  }

  private void execute(Scenario scenario, Report.Result result) {
    if (!Api.declared(scenario.method())) {
      errored(result, "sysml.SysMLService has no RPC " + scenario.method());
      return;
    }

    if (!Api.COVERED.contains(scenario.method())) {
      result.outcome = "skip";
      result.status = "-";
      result.reason = "the public API does not cover " + scenario.method();
      return;
    }

    Scenario.Expect expect = scenario.expect();
    List<String> missing = missingCapabilities(scenario);
    if (!missing.isEmpty()) {
      Optional<Scenario.Expect> withoutCapability = scenario.expectWithoutCapability();
      if (withoutCapability.isEmpty()) {
        result.outcome = "skip";
        result.status = "-";
        result.reason = "the service does not report " + String.join(", ", missing);
        return;
      }
      expect = withoutCapability.get();
      result.reason =
          "the service does not report "
              + String.join(", ", missing)
              + ", so the without-capability expectation applies";
    }

    String modelHash = "";
    Optional<Scenario.Fixture> model = scenario.model();
    if (model.isPresent()) {
      try {
        modelHash = modelHash(model.get());
      } catch (OpenSysMLException | IllegalStateException e) {
        errored(result, e.getMessage());
        return;
      }
    }

    Message request;
    try {
      request = request(scenario, modelHash);
    } catch (RuntimeException e) {
      errored(result, e.getMessage());
      return;
    }

    Api.Answer answer = api.call(scenario.method(), request);
    if (answer instanceof Api.Answer.Unsupported unsupported) {
      result.outcome = "skip";
      result.status = "-";
      result.reason = unsupported.reason();
      return;
    }
    if (answer instanceof Api.Answer.Refused refused) {
      checkRefusal(expect, refused, result);
      return;
    }

    Message response = mutation.apply(((Api.Answer.Answered) answer).response());
    if (!expect.wantStatus().equals("OK")) {
      result.outcome = "fail";
      result.failures = List.of("the call succeeded, want status " + expect.wantStatus());
      return;
    }
    Map<String, Object> normalized = new Normalizer(modelHash).normalize(response);
    if (verbose) {
      out.printf(DETAIL_FORMAT, Checks.render(normalized));
    }
    List<String> failures = Checks.check(expect, normalized);
    if (!failures.isEmpty()) {
      result.outcome = "fail";
      result.failures = failures;
    }
  }

  private void checkRefusal(
      Scenario.Expect expect, Api.Answer.Refused refused, Report.Result result) {
    result.status = refused.status();
    if (!expect.wantStatus().equalsIgnoreCase(refused.status())) {
      result.outcome = "fail";
      result.failures =
          List.of(
              "status: "
                  + refused.status()
                  + " ("
                  + refused.message()
                  + "), want "
                  + expect.wantStatus());
      return;
    }
    if (!expect.statusMessageContains().isEmpty()
        && !refused.message().contains(expect.statusMessageContains())) {
      result.outcome = "fail";
      result.failures =
          List.of(
              "status message \""
                  + refused.message()
                  + "\" does not contain \""
                  + expect.statusMessageContains()
                  + "\"");
    }
  }

  /** The capabilities a scenario needs that the service does not report. */
  private List<String> missingCapabilities(Scenario scenario) {
    List<String> missing = new ArrayList<>();
    for (String capability : scenario.requiresCapabilities()) {
      if (!capabilities.contains(capability)) {
        missing.add(capability);
      }
    }
    return missing;
  }

  /** Builds the call's message from the scenario's protobuf JSON, with the placeholders resolved. */
  private Message request(Scenario scenario, String modelHash) {
    Message.Builder builder = Api.request(scenario.method());
    if (scenario.request().isEmpty()) {
      return builder.build();
    }
    JsonElement resolved = resolve(scenario.request(), modelHash);
    try {
      JsonFormat.parser().merge(resolved.toString(), builder);
    } catch (InvalidProtocolBufferException e) {
      throw new IllegalArgumentException(
          "request does not fit " + builder.getDescriptorForType().getFullName() + ": " + e.getMessage(),
          e);
    }
    return builder.build();
  }

  /** Replaces {@code ${model_hash}} and {@code ${fixture:<path>}} in a request. */
  private JsonElement resolve(JsonElement tree, String modelHash) {
    if (tree instanceof JsonObject object) {
      JsonObject resolved = new JsonObject();
      object.entrySet().forEach(entry -> resolved.add(entry.getKey(), resolve(entry.getValue(), modelHash)));
      return resolved;
    }
    if (tree instanceof JsonArray array) {
      JsonArray resolved = new JsonArray();
      array.forEach(item -> resolved.add(resolve(item, modelHash)));
      return resolved;
    }
    if (tree instanceof JsonPrimitive primitive && primitive.isString()) {
      String text = primitive.getAsString();
      if (text.equals(Normalizer.MODEL_HASH)) {
        if (modelHash.isEmpty()) {
          throw new IllegalArgumentException(
              "the request names " + Normalizer.MODEL_HASH + " but the scenario declares no model");
        }
        return new JsonPrimitive(modelHash);
      }
      Matcher reference = FIXTURE_REFERENCE.matcher(text);
      if (reference.matches()) {
        return new JsonPrimitive(fixture(reference.group(1)));
      }
    }
    return tree;
  }

  /** A fixture's source, refusing a name that reaches outside the fixtures directory. */
  private String fixture(String name) {
    Path path = fixtures.resolve(name).normalize();
    if (!path.startsWith(fixtures.normalize())) {
      throw new IllegalArgumentException("fixture " + name + " is outside " + fixtures);
    }
    try {
      return Files.readString(path, StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new UncheckedIOException("reading fixture " + name, e);
    }
  }

  /** Parses a scenario's model once per run; a model that does not parse clean is a suite error. */
  private String modelHash(Scenario.Fixture fixture) {
    String cached = models.get(fixture);
    if (cached != null) {
      return cached;
    }
    ParseOptions options =
        new ParseOptions(
            org.openmbee.opensysml.Language.fromWireName(fixture.language()), fixture.strictConformance());
    Model model;
    try {
      if (!fixture.fixtures().isEmpty()) {
        List<org.openmbee.opensysml.SourceDocument> documents =
            fixture.fixtures().stream()
                .map(name -> org.openmbee.opensysml.SourceDocument.inline(name, fixture(name)))
                .toList();
        model = connection.parseSources(documents, options);
      } else {
        model = connection.parse(fixture(fixture.fixture()), options);
      }
    } catch (ModelException e) {
      throw new IllegalStateException("parsing fixture " + fixture.fixture() + ": " + e.getMessage(), e);
    }
    for (Diagnostic diagnostic : model.parseDiagnostics()) {
      if (diagnostic.severity() == Diagnostic.Severity.ERROR) {
        throw new IllegalStateException(
            "fixture " + fixture.fixture() + " does not parse clean: " + diagnostic.message());
      }
    }
    if (model.hash().isEmpty()) {
      throw new IllegalStateException("parsing fixture " + fixture.fixture() + " returned no model hash");
    }
    models.put(fixture, model.hash());
    return model.hash();
  }

  private static void errored(Report.Result result, String reason) {
    result.outcome = "error";
    result.status = "-";
    result.reason = reason;
  }
}

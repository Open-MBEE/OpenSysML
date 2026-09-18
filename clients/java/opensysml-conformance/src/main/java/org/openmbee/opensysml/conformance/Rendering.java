package org.openmbee.opensysml.conformance;

import org.openmbee.opensysml.CaseEvaluation;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.EngineInfo;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Outcome;
import org.openmbee.opensysml.QueryElement;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.Bound;
import org.openmbee.opensysml.proto.CalcOutput;
import org.openmbee.opensysml.proto.ExplorationStatus;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.MultiplicityInfo;
import org.openmbee.opensysml.proto.QueryResultElement;
import org.openmbee.opensysml.proto.Span;
import org.openmbee.opensysml.proto.Specialization;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.TypeInfo;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Writes the client's public types back into the generated messages the scenarios are stated
 * against. Comparing that rendering, rather than the transport's answer, is what makes a scenario a
 * test of the client's own reading of a response and not only of the service.
 */
final class Rendering {

  private Rendering() {}

  /**
   * A value.
   *
   * @param value the immutable value
   * @return the generated value
   */
  static org.openmbee.opensysml.proto.Value value(Value value) {
    return Protos.proto(value);
  }

  /**
   * Values by name.
   *
   * @param values the immutable values by name
   * @return the generated values by name
   */
  static Map<String, org.openmbee.opensysml.proto.Value> values(Map<String, Value> values) {
    return Protos.protos(values);
  }

  /**
   * Named outputs, in order.
   *
   * @param outputs the immutable outputs by name
   * @return the generated outputs
   */
  static List<CalcOutput> outputs(Map<String, Value> outputs) {
    List<CalcOutput> rendered = new ArrayList<>(outputs.size());
    outputs.forEach(
        (name, value) ->
            rendered.add(CalcOutput.newBuilder().setName(name).setValue(value(value)).build()));
    return rendered;
  }

  /**
   * Instances.
   *
   * @param instances the immutable instances
   * @return the generated instances, in order
   */
  static List<org.openmbee.opensysml.proto.Instance> instances(List<Instance> instances) {
    return instances.stream().map(Rendering::instance).toList();
  }

  /**
   * The bounds of a standing.
   *
   * @param standing the immutable standing
   * @return the generated bounds, in order
   */
  static List<Bound> bounds(Standing standing) {
    return standing.bounds().stream()
        .map(
            bound ->
                Bound.newBuilder()
                    .setName(bound.name())
                    .setLimit(bound.limit())
                    .setReached(bound.reached())
                    .build())
        .toList();
  }

  /**
   * A verdict.
   *
   * @param verdict the immutable verdict
   * @return the generated verdict
   */
  static org.openmbee.opensysml.proto.Verdict verdict(Verdict verdict) {
    org.openmbee.opensysml.proto.Verdict.Builder builder =
        org.openmbee.opensysml.proto.Verdict.newBuilder()
            .setKind(verdict.kind())
            .setElement(verdict.element())
            .setHolds(verdict.holds())
            .setFailureReason(failureReason(verdict.failureReason()))
            .setEngine(verdict.standing().engine())
            .setStrength(verdict.standing().strength())
            .addAllBounds(bounds(verdict.standing()));
    verdict.elementId().ifPresent(builder::setElementId);
    verdict.condition().ifPresent(builder::setCondition);
    verdict.instanceId().ifPresent(builder::setInstanceId);
    verdict.instanceTypeId().ifPresent(builder::setInstanceTypeId);
    verdict.error().ifPresent(builder::setError);
    verdict.requirementId().ifPresent(builder::setRequirementId);
    verdict.instancePath().ifPresent(builder::setInstancePath);
    return builder.build();
  }

  /**
   * A failure reason.
   *
   * @param reason the immutable reason
   * @return the generated reason
   * @throws IllegalStateException for a reason the client read off the wire without knowing it,
   *     which has no rendering the scenario could be compared against
   */
  static org.openmbee.opensysml.proto.FailureReason failureReason(FailureReason reason) {
    return switch (reason) {
      case UNSPECIFIED -> org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_UNSPECIFIED;
      case EVALUATION -> org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_EVALUATION;
      case WRONG_KIND -> org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_WRONG_KIND;
      case AMBIGUOUS_SUBJECT ->
          org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_AMBIGUOUS_SUBJECT;
      case UNKNOWN -> throw new IllegalStateException("no rendering for an unknown failure reason");
    };
  }

  /**
   * Verdicts.
   *
   * @param verdicts the immutable verdicts
   * @return the generated verdicts, in order
   */
  static List<org.openmbee.opensysml.proto.Verdict> verdicts(List<Verdict> verdicts) {
    return verdicts.stream().map(Rendering::verdict).toList();
  }

  /**
   * The body verdicts of verification cases.
   *
   * @param verdicts the immutable verdicts
   * @return the generated verdicts, in order
   */
  static List<org.openmbee.opensysml.proto.VerificationVerdict> verificationVerdicts(
      List<VerificationVerdict> verdicts) {
    List<org.openmbee.opensysml.proto.VerificationVerdict> rendered =
        new ArrayList<>(verdicts.size());
    for (VerificationVerdict verdict : verdicts) {
      org.openmbee.opensysml.proto.VerificationVerdict.Builder builder =
          org.openmbee.opensysml.proto.VerificationVerdict.newBuilder()
              .setCaseId(verdict.caseId())
              .setKind(verdict.kind())
              .setSubcase(verdict.subcase());
      verdict.detail().ifPresent(builder::setDetail);
      verdict.requirementId().ifPresent(builder::setRequirementId);
      rendered.add(builder.build());
    }
    return rendered;
  }

  /**
   * Case evaluations.
   *
   * @param evaluations the immutable evaluations
   * @return the generated evaluations, in order
   */
  static List<org.openmbee.opensysml.proto.CaseEvaluation> evaluations(
      List<CaseEvaluation> evaluations) {
    List<org.openmbee.opensysml.proto.CaseEvaluation> rendered =
        new ArrayList<>(evaluations.size());
    for (CaseEvaluation evaluation : evaluations) {
      org.openmbee.opensysml.proto.CaseEvaluation.Builder builder =
          org.openmbee.opensysml.proto.CaseEvaluation.newBuilder()
              .setFunctionId(evaluation.functionId())
              .setSelected(evaluation.selected())
              .setTied(evaluation.tied());
      evaluation.arguments().forEach(argument -> builder.addArguments(value(argument)));
      evaluation.result().ifPresent(result -> builder.setResult(value(result)));
      evaluation.error().ifPresent(builder::setError);
      rendered.add(builder.build());
    }
    return rendered;
  }

  /**
   * The outcomes of an exploration.
   *
   * @param exploration the immutable exploration
   * @return the generated outcomes, in order
   */
  static List<org.openmbee.opensysml.proto.Outcome> outcomes(Exploration exploration) {
    List<org.openmbee.opensysml.proto.Outcome> rendered =
        new ArrayList<>(exploration.outcomes().size());
    for (Outcome outcome : exploration.outcomes()) {
      org.openmbee.opensysml.proto.Outcome.Builder builder =
          org.openmbee.opensysml.proto.Outcome.newBuilder()
              .putAllOutputs(values(outcome.outputs()))
              .addAllStatesVisited(outcome.statesVisited())
              .setLinearizations(outcome.linearizations())
              .addAllWitness(outcome.witness())
              .addAllDiagnostics(diagnostics(outcome.diagnostics()));
      outcome.finalState().ifPresent(builder::setFinalState);
      outcome.error().ifPresent(builder::setError);
      rendered.add(builder.build());
    }
    return rendered;
  }

  /**
   * How an exploration ended.
   *
   * @param exploration the immutable exploration
   * @return the generated status
   */
  static ExplorationStatus exploration(Exploration exploration) {
    return ExplorationStatus.newBuilder()
        .setComplete(exploration.complete())
        .setRuns(exploration.runs())
        .addAllBudgetsHit(exploration.budgetsHit())
        .setRunsBudget(exploration.runsBudget())
        .setDepthBudget(exploration.depthBudget())
        .build();
  }

  /**
   * The elements a query selected.
   *
   * @param elements the immutable elements
   * @return the generated elements, in order
   */
  static List<QueryResultElement> elements(List<QueryElement> elements) {
    return elements.stream()
        .map(
            element ->
                QueryResultElement.newBuilder()
                    .setId(element.id())
                    .setType(element.type())
                    .putAllProperties(element.properties())
                    .build())
        .toList();
  }

  /**
   * Engines.
   *
   * @param engines the immutable descriptions
   * @return the generated descriptions, in order
   */
  static List<org.openmbee.opensysml.proto.EngineInfo> engines(List<EngineInfo> engines) {
    return engines.stream()
        .map(
            engine ->
                org.openmbee.opensysml.proto.EngineInfo.newBuilder()
                    .setName(engine.name())
                    .setAuthority(engine.authority())
                    .addAllAnswers(engine.answers())
                    .addAllBounds(engine.bounds())
                    .setProcess(engine.process())
                    .setProcessFound(engine.processFound())
                    .setReady(engine.ready())
                    .setUnavailable(engine.unavailable())
                    .setKind(engine.kind())
                    .setProtocol(engine.protocol())
                    .setSource(engine.source())
                    .setCommand(engine.command())
                    .setVersion(engine.version())
                    .setServed(engine.served())
                    .build())
        .toList();
  }

  /**
   * Diagnostics.
   *
   * @param diagnostics the immutable diagnostics
   * @return the generated diagnostics, in order
   */
  static List<org.openmbee.opensysml.proto.Diagnostic> diagnostics(List<Diagnostic> diagnostics) {
    return diagnostics.stream().map(Rendering::diagnostic).toList();
  }

  private static org.openmbee.opensysml.proto.Diagnostic diagnostic(Diagnostic diagnostic) {
    org.openmbee.opensysml.proto.Diagnostic.Builder builder =
        org.openmbee.opensysml.proto.Diagnostic.newBuilder()
            .setSeverity(diagnostic.severity().wireName())
            .setMessage(diagnostic.message())
            .setCode(diagnostic.code());
    diagnostic
        .span()
        .ifPresent(
            span ->
                builder.setSpan(
                    Span.newBuilder()
                        .setFile(span.file())
                        .setStartLine(span.startLine())
                        .setStartCol(span.startColumn())
                        .setEndLine(span.endLine())
                        .setEndCol(span.endColumn())));
    return builder.build();
  }

  /**
   * A symbol.
   *
   * @param symbol the immutable symbol
   * @return the generated symbol
   */
  static SymbolInfo symbol(Symbol symbol) {
    SymbolInfo.Builder builder =
        SymbolInfo.newBuilder()
            .setId(symbol.id())
            .setName(symbol.name())
            .setKind(symbol.kind())
            .putAllMetadata(symbol.metadata())
            .addAllChildIds(symbol.childIds())
            .setWithheldLibraryAttributes(symbol.withheldLibraryAttributes());
    for (Symbol.Attribute attribute : symbol.attributes()) {
      AttributeInfo.Builder rendered =
          AttributeInfo.newBuilder().setName(attribute.name()).setType(attribute.type());
      attribute.value().ifPresent(value -> rendered.setValue(value(value)));
      attribute.unit().ifPresent(rendered::setUnit);
      builder.addAttributes(rendered);
    }
    for (Symbol.Specialization specialization : symbol.specializations()) {
      Specialization.Builder rendered =
          Specialization.newBuilder()
              .setKind(specialization.kind())
              .setDeclared(specialization.declared());
      specialization.targetId().ifPresent(rendered::setTargetId);
      specialization.targetKind().ifPresent(rendered::setTargetKind);
      builder.addSpecializations(rendered);
    }
    symbol
        .typeFacts()
        .ifPresent(
            facts -> {
              TypeInfo.Builder rendered = TypeInfo.newBuilder().setQuantity(facts.quantity());
              facts.declared().ifPresent(rendered::setDeclared);
              facts.resolvedId().ifPresent(rendered::setResolvedId);
              facts.resolvedKind().ifPresent(rendered::setResolvedKind);
              facts.primitive().ifPresent(rendered::setPrimitive);
              facts.primitiveSource().ifPresent(rendered::setPrimitiveSource);
              facts.unit().ifPresent(rendered::setUnit);
              builder.setTypeInfo(rendered);
            });
    symbol
        .multiplicity()
        .ifPresent(
            multiplicity -> {
              MultiplicityInfo.Builder rendered = MultiplicityInfo.newBuilder();
              multiplicity.lower().ifPresent(rendered::setLower);
              multiplicity.upper().ifPresent(rendered::setUpper);
              builder.setMultiplicity(rendered);
            });
    return builder.build();
  }

  /**
   * An instance.
   *
   * @param instance the immutable instance
   * @return the generated instance
   */
  static org.openmbee.opensysml.proto.Instance instance(Instance instance) {
    org.openmbee.opensysml.proto.Instance.Builder builder =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(instance.id())
            .setTypeSymbolId(instance.typeSymbolId());
    for (Map.Entry<String, Instance.FeatureValue> entry : instance.featureValues().entrySet()) {
      Instance.FeatureValue featureValue = entry.getValue();
      FeatureValue.Builder rendered =
          FeatureValue.newBuilder()
              .setFeatureName(featureValue.featureName())
              .setMaterialized(featureValue.materialized());
      featureValue.value().ifPresent(value -> rendered.setValue(value(value)));
      featureValue.values().forEach(value -> rendered.addValues(value(value)));
      featureValue.error().ifPresent(rendered::setError);
      builder.putFeatureValues(entry.getKey(), rendered.build());
    }
    return builder.build();
  }
}

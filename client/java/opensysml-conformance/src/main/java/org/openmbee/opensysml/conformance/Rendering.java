package org.openmbee.opensysml.conformance;

import org.openmbee.opensysml.CaseEvaluation;
import org.openmbee.opensysml.CheckResult;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.EngineInfo;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Outcome;
import org.openmbee.opensysml.QueryElement;
import org.openmbee.opensysml.RenderedView;
import org.openmbee.opensysml.Recording;
import org.openmbee.opensysml.Runs;
import org.openmbee.opensysml.SelfCheck;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.ToolDryRun;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.WitnessAssignment;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.opensysml.internal.Protos;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.Bound;
import org.openmbee.opensysml.proto.CheckBehaviorResponse;
import org.openmbee.opensysml.proto.CheckDivergence;
import org.openmbee.opensysml.proto.CheckDivergentValue;
import org.openmbee.opensysml.proto.CheckFinal;
import org.openmbee.opensysml.proto.CheckSearch;
import org.openmbee.opensysml.proto.CheckViolation;
import org.openmbee.opensysml.proto.CheckWitness;
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

  /** A {@code CheckBehaviorResponse} carrying a check result as the service would answer it. */
  static CheckBehaviorResponse checkResult(CheckResult result) {
    CheckBehaviorResponse.Builder response =
        CheckBehaviorResponse.newBuilder()
            .setVerdict(result.verdict())
            .addAllLines(result.lines())
            .addAllViolations(result.violations().stream().map(Rendering::checkViolation).toList())
            .addAllDivergences(
                result.divergences().stream().map(Rendering::checkDivergence).toList())
            .addAllFinals(result.finals().stream().map(Rendering::checkFinal).toList())
            .setClaim(result.claim())
            .addAllFreeInputs(result.freeInputs())
            .addAllAssumptions(result.assumptions())
            .setEngine(result.standing().engine())
            .setStrength(result.standing().strength())
            .addAllBounds(bounds(result.standing()))
            .setReason(result.reason());
    result.search().ifPresent(s -> response.setSearch(checkSearch(s)));
    result.witness().ifPresent(w -> response.setWitness(checkWitness(w)));
    result.contrast().ifPresent(w -> response.setContrast(checkWitness(w)));
    return response.build();
  }

  private static CheckSearch checkSearch(CheckResult.Search s) {
    return CheckSearch.newBuilder()
        .setStates(s.states())
        .setMoves(s.moves())
        .setMaxDepth(s.maxDepth())
        .addAllBoundsHit(s.boundsHit())
        .setHorizon(s.horizon())
        .setHorizonSet(s.horizonSet())
        .setMassBounded(s.massBounded())
        .addAllNotes(s.notes())
        .addAllScope(s.scope())
        .setStatus(s.status())
        .build();
  }

  private static CheckWitness checkWitness(CheckResult.Witness w) {
    return CheckWitness.newBuilder()
        .setSchedule(w.schedule())
        .addAllDraws(w.draws())
        .addAllChoices(w.choices())
        .addAllInputs(w.inputs())
        .setFileName(w.fileName())
        .setFileContent(w.fileContent())
        .build();
  }

  private static CheckViolation checkViolation(CheckResult.Violation v) {
    CheckViolation.Builder violation =
        CheckViolation.newBuilder()
            .setKind(v.kind())
            .setProperty(v.property())
            .setError(v.error())
            .setDepth(v.depth())
            .setProbability(v.probability())
            .setText(v.text());
    v.witness().ifPresent(w -> violation.setWitness(checkWitness(w)));
    return violation.build();
  }

  private static CheckDivergence checkDivergence(CheckResult.Divergence d) {
    CheckDivergence.Builder divergence = CheckDivergence.newBuilder().setFeature(d.feature());
    for (CheckResult.DivergentValue v : d.values()) {
      CheckDivergentValue.Builder value = CheckDivergentValue.newBuilder().setValue(v.value());
      v.witness().ifPresent(w -> value.setWitness(checkWitness(w)));
      divergence.addValues(value);
    }
    return divergence.build();
  }

  private static CheckFinal checkFinal(CheckResult.Final f) {
    CheckFinal.Builder fin = CheckFinal.newBuilder().setOutcome(f.outcome()).putAllValues(f.values());
    f.witness().ifPresent(w -> fin.setWitness(checkWitness(w)));
    return fin.build();
  }

  static org.openmbee.opensysml.proto.ToolDryRun toolDryRun(ToolDryRun preview) {
    org.openmbee.opensysml.proto.ToolDryRun.Builder proto =
        org.openmbee.opensysml.proto.ToolDryRun.newBuilder()
            .setAction(preview.action())
            .setTool(preview.tool())
            .setVersion(preview.version())
            .setManifest(preview.manifest())
            .setExecutable(preview.executable())
            .setProtocol(preview.protocol())
            .addAllArgs(preview.args())
            .addAllEnv(preview.env())
            .setCwd(preview.cwd())
            .setStdin(preview.stdin())
            .setStdinData(preview.stdinData())
            .setOutputDir(preview.outputDir())
            .setReplySource(preview.replySource())
            .addAllLines(preview.lines());
    preview
        .inputFile()
        .ifPresent(
            file ->
                proto.setInputFile(
                    org.openmbee.opensysml.proto.ToolDryRunFile.newBuilder()
                        .setName(file.name())
                        .setFormat(file.format())
                        .setData(file.data())));
    for (ToolDryRun.Input input : preview.inputs()) {
      proto.addInputs(
          org.openmbee.opensysml.proto.ToolDryRunInput.newBuilder()
              .setVariable(input.variable())
              .setParameter(input.parameter())
              .setValue(input.value())
              .setUnit(input.unit()));
    }
    for (ToolDryRun.Output output : preview.outputs()) {
      proto.addOutputs(
          org.openmbee.opensysml.proto.ToolDryRunOutput.newBuilder()
              .setVariable(output.variable())
              .setParameter(output.parameter()));
    }
    return proto.build();
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
    verdict.question().ifPresent(builder::setQuestion);
    verdict.status().ifPresent(builder::setStatus);
    verdict.witness().forEach(assignment -> builder.addWitness(witness(assignment)));
    return builder.build();
  }

  /**
   * A witness assignment.
   *
   * @param assignment the immutable assignment
   * @return the generated assignment
   */
  static org.openmbee.opensysml.proto.WitnessAssignment witness(WitnessAssignment assignment) {
    org.openmbee.opensysml.proto.WitnessAssignment.Builder builder =
        org.openmbee.opensysml.proto.WitnessAssignment.newBuilder()
            .setFeature(assignment.feature())
            .setUnit(assignment.unit())
            .setExact(assignment.exact());
    assignment.value().ifPresent(value -> builder.setValue(value(value)));
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
      case UNDECIDED -> org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_UNDECIDED;
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

  /**
   * A conversion's answer.
   *
   * @param conversion the immutable conversion
   * @return the generated answer
   */
  static org.openmbee.opensysml.proto.ConvertResponse conversion(
      org.openmbee.opensysml.Conversion conversion) {
    return org.openmbee.opensysml.proto.ConvertResponse.newBuilder()
        .setContent(conversion.content())
        .setFromFormat(conversion.fromFormat())
        .setToFormat(conversion.toFormat())
        .addAllDiagnostics(diagnostics(conversion.diagnostics()))
        .setExperimental(conversion.experimental())
        .setExperimentalNotice(conversion.experimentalNotice())
        .build();
  }

  /**
   * A migration, as the generated answer the wire gives.
   *
   * @param migration the public migration
   * @return the generated answer
   */
  static org.openmbee.opensysml.proto.MigrateResponse migration(
      org.openmbee.opensysml.Migration migration) {
    org.openmbee.opensysml.MigrationReport report = migration.report();
    org.openmbee.opensysml.proto.MigrationReport.Builder rendered =
        org.openmbee.opensysml.proto.MigrationReport.newBuilder()
            .setSource(report.source())
            .setExporter(report.exporter())
            .setSummary(report.summary())
            .setMapped(report.mapped())
            .setApproximated(report.approximated())
            .setUnmapped(report.unmapped())
            .setSkipped(report.skipped())
            .setText(report.text());
    for (org.openmbee.opensysml.MigrationEntry entry : report.entries()) {
      rendered.addEntries(
          org.openmbee.opensysml.proto.MigrationEntry.newBuilder()
              .setId(entry.id())
              .setKind(entry.kind())
              .setName(entry.name())
              .setTarget(entry.target())
              .setVerdict(entry.verdict())
              .setNote(entry.note()));
    }
    org.openmbee.opensysml.proto.MigrateResponse.Builder response =
        org.openmbee.opensysml.proto.MigrateResponse.newBuilder()
            .setContent(migration.content())
            .setFromFormat(migration.fromFormat())
            .setToFormat(migration.toFormat())
            .setExperimental(true)
            .setExperimentalNotice(migration.experimentalNotice())
            .setReport(rendered)
            .setResults(migration.results());
    for (Map.Entry<String, byte[]> file : migration.files().entrySet()) {
      response.addFiles(
          org.openmbee.opensysml.proto.MigrationFile.newBuilder()
              .setPath(file.getKey())
              .setContent(com.google.protobuf.ByteString.copyFrom(file.getValue())));
    }
    return response.build();
  }

  /**
   * The edits a batch applied.
   *
   * @param applied the immutable edits
   * @return the generated edits, in order
   */
  static List<org.openmbee.opensysml.proto.AppliedEdit> appliedEdits(
      List<org.openmbee.opensysml.AppliedEdit> applied) {
    return applied.stream()
        .map(
            edit ->
                org.openmbee.opensysml.proto.AppliedEdit.newBuilder()
                    .setOperationIndex(edit.operationIndex())
                    .setTarget(edit.target())
                    .setOffset(edit.offset())
                    .setLength(edit.length())
                    .setOldText(edit.oldText())
                    .setNewText(edit.newText())
                    .setDocument(edit.document())
                    .build())
        .toList();
  }

  /**
   * The documents a batch rewrote.
   *
   * @param documents the immutable documents
   * @return the generated documents, in order
   */
  static List<org.openmbee.opensysml.proto.EditedDocument> editedDocuments(
      List<org.openmbee.opensysml.EditedDocument> documents) {
    return documents.stream()
        .map(
            document ->
                org.openmbee.opensysml.proto.EditedDocument.newBuilder()
                    .setName(document.name())
                    .setContent(document.content())
                    .build())
        .toList();
  }

  /**
   * The referrers a refused batch named.
   *
   * @param referrers the immutable referrers
   * @return the generated referrers, in order
   */
  static List<org.openmbee.opensysml.proto.Referrer> referrers(
      List<org.openmbee.opensysml.Referrer> referrers) {
    return referrers.stream()
        .map(
            referrer ->
                org.openmbee.opensysml.proto.Referrer.newBuilder()
                    .setName(referrer.name())
                    .setDocument(referrer.document())
                    .build())
        .toList();
  }

  /**
   * A sweep's answer: its table and what ran it.
   *
   * @param sweep the immutable table
   * @return the generated answer
   */
  static org.openmbee.opensysml.proto.RunSweepResponse sweep(org.openmbee.opensysml.Sweep sweep) {
    List<org.openmbee.opensysml.proto.SweepRow> rows =
        sweep.rows().stream().map(Rendering::sweepRow).toList();
    return org.openmbee.opensysml.proto.RunSweepResponse.newBuilder()
        .addAllRows(rows)
        .addAllParameters(sweep.parameters())
        .setSampled(sweep.sampled())
        .setSeed(sweep.seed())
        .addAllInstances(instances(sweep.instances()))
        .addAllDiagnostics(diagnostics(sweep.diagnostics()))
        .setEngine(sweep.standing().engine())
        .setStrength(sweep.standing().strength())
        .addAllBounds(bounds(sweep.standing()))
        .build();
  }

  private static org.openmbee.opensysml.proto.SweepRow sweepRow(org.openmbee.opensysml.SweepRow row) {
    return org.openmbee.opensysml.proto.SweepRow.newBuilder()
        .addAllInputs(outputs(row.inputs()))
        .addAllOutputs(outputs(row.outputs()))
        .addAllVerdicts(verdicts(row.verdicts()))
        .setElapsedMicros(row.elapsed().toNanos() / 1000)
        .setError(row.error())
        .setFailureReason(failureReason(row.failureReason()))
        .addAllEvaluations(evaluations(row.evaluations()))
        .build();
  }

  static org.openmbee.opensysml.proto.RunRunsResponse runs(Runs runs) {
    org.openmbee.opensysml.proto.RunRunsResponse.Builder response =
        org.openmbee.opensysml.proto.RunRunsResponse.newBuilder()
            .addAllRows(runs.rows().stream().map(Rendering::sweepRow).toList())
            .addAllObservables(runs.observables())
            .addAllDistributions(runs.distributions().stream().map(Rendering::distribution).toList())
            .addAllOutputs(outputs(runs.outputs()))
            .addAllVerdicts(verdicts(runs.verdicts()))
            .setUnconcluded(runs.unconcluded())
            .addAllInstances(instances(runs.instances()))
            .addAllDiagnostics(diagnostics(runs.diagnostics()))
            .setEngine(runs.standing().engine())
            .setStrength(runs.standing().strength())
            .addAllBounds(bounds(runs.standing()));
    runs.seed().ifPresent(response::setSeed);
    return response.build();
  }

  private static org.openmbee.opensysml.proto.RunDistribution distribution(Runs.Distribution d) {
    org.openmbee.opensysml.proto.RunDistribution.Builder distribution =
        org.openmbee.opensysml.proto.RunDistribution.newBuilder()
            .setName(d.name())
            .setUnit(d.unit())
            .setCount(d.count())
            .setNumeric(d.numeric())
            .setDeviation(d.deviation());
    d.min().ifPresent(v -> distribution.setMin(value(v)));
    d.max().ifPresent(v -> distribution.setMax(value(v)));
    d.mean().ifPresent(v -> distribution.setMean(value(v)));
    for (Runs.Percentile p : d.percentiles()) {
      distribution.addPercentiles(
          org.openmbee.opensysml.proto.RunPercentile.newBuilder()
              .setPercentile(p.percentile())
              .setValue(value(p.value())));
    }
    for (Runs.HistogramBin b : d.histogram()) {
      distribution.addHistogram(
          org.openmbee.opensysml.proto.RunHistogramBin.newBuilder()
              .setLow(value(b.low()))
              .setHigh(value(b.high()))
              .setCount(b.count()));
    }
    for (Runs.CountedValue v : d.values()) {
      distribution.addValues(
          org.openmbee.opensysml.proto.RunCountedValue.newBuilder()
              .setValue(value(v.value()))
              .setCount(v.count()));
    }
    return distribution.build();
  }

  static org.openmbee.opensysml.proto.SelfCheckResponse selfCheck(SelfCheck check) {
    org.openmbee.opensysml.proto.SelfCheckResponse.Builder response =
        org.openmbee.opensysml.proto.SelfCheckResponse.newBuilder()
            .setStatus(check.status())
            .setElements(check.elements())
            .setChecks(check.checks())
            .setViolations(check.violations())
            .setUnevaluated(check.unevaluated())
            .setEvaluationErrors(check.evaluationErrors())
            .setSummary(check.summary())
            .addAllDiagnostics(diagnostics(check.diagnostics()));
    for (SelfCheck.Finding f : check.findings()) {
      response.addFindings(
          org.openmbee.opensysml.proto.SelfCheckFinding.newBuilder()
              .setStatus(f.status())
              .setSubject(f.subject())
              .setLocation(f.location())
              .setConstraint(f.constraint())
              .setElement(f.element())
              .setMessage(f.message())
              .setLine(f.line()));
    }
    check
        .applications()
        .forEach(
            (constraint, count) ->
                response.addApplications(
                    org.openmbee.opensysml.proto.SelfCheckApplication.newBuilder()
                        .setConstraint(constraint)
                        .setCount(count)));
    return response.build();
  }

  /** Everything of a recording but its single run, which the caller renders. */
  static org.openmbee.opensysml.proto.RecordRunResponse.Builder recording(Recording recording) {
    org.openmbee.opensysml.proto.RecordRunResponse.Builder response =
        org.openmbee.opensysml.proto.RecordRunResponse.newBuilder()
            .setPackage(recording.packageName())
            .setDefinition(recording.definition())
            .addAllRecords(recording.records())
            .setSource(recording.source())
            .setContent(recording.content())
            .addAllDocuments(editedDocuments(recording.documents()))
            .setSkipped(recording.skipped())
            .addAllReasons(recording.reasons())
            .addAllDiagnostics(diagnostics(recording.diagnostics()))
            .setModelHash(recording.modelHash());
    recording.sweep().ifPresent(sweep -> response.setSweep(sweep(sweep)));
    recording.sample().ifPresent(sample -> response.setSample(runs(sample)));
    return response;
  }

  /**
   * A document query's answer: its columns, and each row's element put back in the kind its
   * {@link org.openmbee.opensysml.DocumentRow} carries it as.
   *
   * @param result the immutable result
   * @return the generated answer
   */
  static org.openmbee.opensysml.proto.RunDocumentQueryResponse documentQueryResult(
      org.openmbee.opensysml.DocumentQueryResult result) {
    org.openmbee.opensysml.proto.RunDocumentQueryResponse.Builder response =
        org.openmbee.opensysml.proto.RunDocumentQueryResponse.newBuilder();
    result
        .columns()
        .forEach(
            name ->
                response.addColumns(
                    org.openmbee.opensysml.proto.DocumentQueryColumn.newBuilder().setName(name)));
    for (org.openmbee.opensysml.DocumentRow row : result.rows()) {
      org.openmbee.opensysml.proto.DocumentQueryRow.Builder rendered =
          org.openmbee.opensysml.proto.DocumentQueryRow.newBuilder()
              .setElement(rowElement(row));
      for (List<org.openmbee.opensysml.DocumentValue> cell : row.cells()) {
        rendered.addCells(
            org.openmbee.opensysml.proto.DocumentQueryCell.newBuilder()
                .addAllValues(cell.stream().map(Protos::proto).toList()));
      }
      response.addRows(rendered);
    }
    return response.build();
  }

  static org.openmbee.opensysml.proto.RenderViewResponse renderedView(RenderedView view) {
    org.openmbee.opensysml.proto.RenderViewResponse.Builder response =
        org.openmbee.opensysml.proto.RenderViewResponse.newBuilder()
            .setView(view.view())
            .setKind(view.kind())
            .setStated(view.stated())
            .addAllColumns(view.columns())
            .addAllNotices(view.notices())
            .addAllShown(view.shown());
    view.canvas()
        .ifPresent(
            canvas ->
                response.setCanvas(
                    org.openmbee.opensysml.proto.RenderCanvas.newBuilder()
                        .setUnit(canvas.unit())
                        .setWidth(canvas.width())
                        .setHeight(canvas.height())
                        .setHasSize(canvas.hasSize())));
    for (RenderedView.Node node : view.nodes()) {
      org.openmbee.opensysml.proto.RenderNode.Builder rendered =
          org.openmbee.opensysml.proto.RenderNode.newBuilder()
              .setId(node.id())
              .setKind(node.kind())
              .setName(node.name())
              .setNameSynthesized(node.nameSynthesized())
              .setType(node.type())
              .setDetail(node.detail())
              .setText(node.text())
              .setStandIn(node.standIn())
              .setParent(node.parent());
      node.ports()
          .forEach(
              port ->
                  rendered.addPorts(
                      org.openmbee.opensysml.proto.RenderPort.newBuilder()
                          .setId(port.id())
                          .setName(port.name())
                          .setType(port.type())
                          .setDirection(port.direction())));
      node.origin().ifPresent(span -> rendered.setOrigin(span(span)));
      node.geometry()
          .ifPresent(
              geometry ->
                  rendered.setGeometry(
                      org.openmbee.opensysml.proto.RenderGeometry.newBuilder()
                          .setX(geometry.x())
                          .setY(geometry.y())
                          .setWidth(geometry.width())
                          .setHeight(geometry.height())
                          .setHasSize(geometry.hasSize())
                          .setCollapsed(geometry.collapsed())
                          .setUnplaced(geometry.unplaced())));
      node.style().ifPresent(style -> rendered.setStyle(style(style)));
      response.addNodes(rendered);
    }
    for (RenderedView.Edge edge : view.edges()) {
      org.openmbee.opensysml.proto.RenderEdge.Builder rendered =
          org.openmbee.opensysml.proto.RenderEdge.newBuilder()
              .setFrom(edge.from())
              .setTo(edge.to())
              .setFromPort(edge.fromPort())
              .setToPort(edge.toPort())
              .setLabel(edge.label())
              .setName(edge.name())
              .setKind(edge.kind());
      edge.origin().ifPresent(span -> rendered.setOrigin(span(span)));
      edge.route()
          .forEach(
              point ->
                  rendered.addRoute(
                      org.openmbee.opensysml.proto.RenderPoint.newBuilder()
                          .setX(point.x())
                          .setY(point.y())));
      edge.style().ifPresent(style -> rendered.setStyle(style(style)));
      response.addEdges(rendered);
    }
    view.rows()
        .forEach(
            row -> {
              org.openmbee.opensysml.proto.RenderRow.Builder rendered =
                  org.openmbee.opensysml.proto.RenderRow.newBuilder().addAllCells(row.cells());
              row.origin().ifPresent(span -> rendered.setOrigin(span(span)));
              response.addRows(rendered);
            });
    view.notes()
        .forEach(
            note -> {
              org.openmbee.opensysml.proto.RenderNote.Builder rendered =
                  org.openmbee.opensysml.proto.RenderNote.newBuilder()
                      .setText(note.text())
                      .setAnchor(note.anchor())
                      .setEdgeFrom(note.edgeFrom())
                      .setEdgeTo(note.edgeTo())
                      .setX(note.x())
                      .setY(note.y())
                      .setWidth(note.width())
                      .setHeight(note.height())
                      .setHasSize(note.hasSize());
              note.origin().ifPresent(span -> rendered.setOrigin(span(span)));
              response.addNotes(rendered);
            });
    return response.build();
  }

  private static org.openmbee.opensysml.proto.Span span(RenderedView.Span span) {
    return org.openmbee.opensysml.proto.Span.newBuilder()
        .setFile(span.file())
        .setStartLine(span.startLine())
        .setStartCol(span.startCol())
        .setEndLine(span.endLine())
        .setEndCol(span.endCol())
        .build();
  }

  private static org.openmbee.opensysml.proto.RenderStyle style(RenderedView.Style style) {
    return org.openmbee.opensysml.proto.RenderStyle.newBuilder()
        .setFill(style.fill())
        .setLine(style.line())
        .setText(style.text())
        .setFont(style.font())
        .setFontSize(style.fontSize())
        .setBold(style.bold())
        .setItalic(style.italic())
        .build();
  }

  private static org.openmbee.opensysml.proto.DocumentValue rowElement(
      org.openmbee.opensysml.DocumentRow row) {
    if (row.verdict().isPresent()) {
      return Protos.proto(row.verdict().orElseThrow());
    }
    if (row.state().isPresent()) {
      return Protos.proto(row.state().orElseThrow());
    }
    if (row.event().isPresent()) {
      return Protos.proto(row.event().orElseThrow());
    }
    if (row.object().isPresent()) {
      return Protos.proto(row.object().orElseThrow());
    }
    return Protos.proto(row.element());
  }
}
